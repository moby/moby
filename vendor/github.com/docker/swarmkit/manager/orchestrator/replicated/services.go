package replicated

import (
	"sort"

	"github.com/docker/go-events"
	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/log"
	"github.com/docker/swarmkit/manager/orchestrator"
	"github.com/docker/swarmkit/manager/state"
	"github.com/docker/swarmkit/manager/state/store"
	"golang.org/x/net/context"
)

// This file provices service-level orchestration. It observes changes to
// services and creates and destroys tasks as necessary to match the service
// specifications. This is different from task-level orchestration, which
// responds to changes in individual tasks (or nodes which run them).

func (r *Orchestrator) initCluster(readTx store.ReadTx) error {
	clusters, err := store.FindClusters(readTx, store.ByName("default"))
	if err != nil {
		return err
	}

	if len(clusters) != 1 {
		// we'll just pick it when it is created.
		return nil
	}

	r.cluster = clusters[0]
	return nil
}

func (r *Orchestrator) initServices(readTx store.ReadTx) error {
	services, err := store.FindServices(readTx, store.All)
	if err != nil {
		return err
	}
	for _, s := range services {
		if orchestrator.IsReplicatedService(s) {
			r.reconcileServices[s.ID] = s
		}
	}
	return nil
}

func (r *Orchestrator) handleServiceEvent(ctx context.Context, event events.Event) {
	switch v := event.(type) {
	case state.EventDeleteService:
		if !orchestrator.IsReplicatedService(v.Service) {
			return
		}
		orchestrator.DeleteServiceTasks(ctx, r.store, v.Service)
		r.restarts.ClearServiceHistory(v.Service.ID)
		delete(r.reconcileServices, v.Service.ID)
	case state.EventCreateService:
		if !orchestrator.IsReplicatedService(v.Service) {
			return
		}
		r.reconcileServices[v.Service.ID] = v.Service
	case state.EventUpdateService:
		if !orchestrator.IsReplicatedService(v.Service) {
			return
		}
		r.reconcileServices[v.Service.ID] = v.Service
	}
}

func (r *Orchestrator) tickServices(ctx context.Context) {
	if len(r.reconcileServices) > 0 {
		for _, s := range r.reconcileServices {
			r.reconcile(ctx, s)
		}
		r.reconcileServices = make(map[string]*api.Service)
	}
}

func (r *Orchestrator) resolveService(ctx context.Context, task *api.Task) *api.Service {
	if task.ServiceID == "" {
		return nil
	}
	var service *api.Service
	r.store.View(func(tx store.ReadTx) {
		service = store.GetService(tx, task.ServiceID)
	})
	return service
}

func (r *Orchestrator) reconcile(ctx context.Context, service *api.Service) {
	runningSlots, deadSlots, err := orchestrator.GetRunnableAndDeadSlots(r.store, service.ID)
	if err != nil {
		log.G(ctx).WithError(err).Errorf("reconcile failed finding tasks")
		return
	}

	numSlots := len(runningSlots)

	slotsSlice := make([]orchestrator.Slot, 0, numSlots)
	for _, slot := range runningSlots {
		slotsSlice = append(slotsSlice, slot)
	}

	deploy := service.Spec.GetMode().(*api.ServiceSpec_Replicated)
	specifiedSlots := int(deploy.Replicated.Replicas)

	switch {
	case specifiedSlots > numSlots:
		log.G(ctx).Debugf("Service %s was scaled up from %d to %d instances", service.ID, numSlots, specifiedSlots)
		// Update all current tasks then add missing tasks
		r.updater.Update(ctx, r.cluster, service, slotsSlice)
		_, err = r.store.Batch(func(batch *store.Batch) error {
			r.addTasks(ctx, batch, service, runningSlots, deadSlots, specifiedSlots-numSlots)
			r.deleteTasksMap(ctx, batch, deadSlots)
			return nil
		})
		if err != nil {
			log.G(ctx).WithError(err).Errorf("reconcile batch failed")
		}

	case specifiedSlots < numSlots:
		// Update up to N tasks then remove the extra
		log.G(ctx).Debugf("Service %s was scaled down from %d to %d instances", service.ID, numSlots, specifiedSlots)

		// Preferentially remove tasks on the nodes that have the most
		// copies of this service, to leave a more balanced result.

		// First sort tasks such that tasks which are currently running
		// (in terms of observed state) appear before non-running tasks.
		// This will cause us to prefer to remove non-running tasks, all
		// other things being equal in terms of node balance.

		sort.Sort(slotsByRunningState(slotsSlice))

		// Assign each task an index that counts it as the nth copy of
		// of the service on its node (1, 2, 3, ...), and sort the
		// tasks by this counter value.

		slotsByNode := make(map[string]int)
		slotsWithIndices := make(slotsByIndex, 0, numSlots)

		for _, slot := range slotsSlice {
			if len(slot) == 1 && slot[0].NodeID != "" {
				slotsByNode[slot[0].NodeID]++
				slotsWithIndices = append(slotsWithIndices, slotWithIndex{slot: slot, index: slotsByNode[slot[0].NodeID]})
			} else {
				slotsWithIndices = append(slotsWithIndices, slotWithIndex{slot: slot, index: -1})
			}
		}

		sort.Sort(slotsWithIndices)

		sortedSlots := make([]orchestrator.Slot, 0, numSlots)
		for _, slot := range slotsWithIndices {
			sortedSlots = append(sortedSlots, slot.slot)
		}

		r.updater.Update(ctx, r.cluster, service, sortedSlots[:specifiedSlots])
		_, err = r.store.Batch(func(batch *store.Batch) error {
			r.deleteTasksMap(ctx, batch, deadSlots)
			r.deleteTasks(ctx, batch, sortedSlots[specifiedSlots:])
			return nil
		})
		if err != nil {
			log.G(ctx).WithError(err).Errorf("reconcile batch failed")
		}

	case specifiedSlots == numSlots:
		_, err = r.store.Batch(func(batch *store.Batch) error {
			r.deleteTasksMap(ctx, batch, deadSlots)
			return nil
		})
		if err != nil {
			log.G(ctx).WithError(err).Errorf("reconcile batch failed")
		}
		// Simple update, no scaling - update all tasks.
		r.updater.Update(ctx, r.cluster, service, slotsSlice)
	}
}

func (r *Orchestrator) addTasks(ctx context.Context, batch *store.Batch, service *api.Service, runningSlots map[uint64]orchestrator.Slot, deadSlots map[uint64]orchestrator.Slot, count int) {
	slot := uint64(0)
	for i := 0; i < count; i++ {
		// Find a slot number that is missing a running task
		for {
			slot++
			if _, ok := runningSlots[slot]; !ok {
				break
			}
		}

		delete(deadSlots, slot)
		err := batch.Update(func(tx store.Tx) error {
			return store.CreateTask(tx, orchestrator.NewTask(r.cluster, service, slot, ""))
		})
		if err != nil {
			log.G(ctx).Errorf("Failed to create task: %v", err)
		}
	}
}

func (r *Orchestrator) deleteTasks(ctx context.Context, batch *store.Batch, slots []orchestrator.Slot) {
	for _, slot := range slots {
		for _, t := range slot {
			r.deleteTask(ctx, batch, t)
		}
	}
}

func (r *Orchestrator) deleteTasksMap(ctx context.Context, batch *store.Batch, slots map[uint64]orchestrator.Slot) {
	for _, slot := range slots {
		for _, t := range slot {
			r.deleteTask(ctx, batch, t)
		}
	}
}

func (r *Orchestrator) deleteTask(ctx context.Context, batch *store.Batch, t *api.Task) {
	err := batch.Update(func(tx store.Tx) error {
		return store.DeleteTask(tx, t.ID)
	})
	if err != nil {
		log.G(ctx).WithError(err).Errorf("deleting task %s failed", t.ID)
	}
}
