package orchestrator

import (
	"github.com/docker/go-events"
	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/log"
	"github.com/docker/swarmkit/manager/state"
	"github.com/docker/swarmkit/manager/state/store"
	"golang.org/x/net/context"
)

// This file provices service-level orchestration. It observes changes to
// services and creates and destroys tasks as necessary to match the service
// specifications. This is different from task-level orchestration, which
// responds to changes in individual tasks (or nodes which run them).

func (r *ReplicatedOrchestrator) initServices(readTx store.ReadTx) error {
	services, err := store.FindServices(readTx, store.All)
	if err != nil {
		return err
	}
	for _, s := range services {
		if isReplicatedService(s) {
			r.reconcileServices[s.ID] = s
		}
	}
	return nil
}

func (r *ReplicatedOrchestrator) handleServiceEvent(ctx context.Context, event events.Event) {
	switch v := event.(type) {
	case state.EventDeleteService:
		if !isReplicatedService(v.Service) {
			return
		}
		deleteServiceTasks(ctx, r.store, v.Service)
		r.restarts.ClearServiceHistory(v.Service.ID)
	case state.EventCreateService:
		if !isReplicatedService(v.Service) {
			return
		}
		r.reconcileServices[v.Service.ID] = v.Service
	case state.EventUpdateService:
		if !isReplicatedService(v.Service) {
			return
		}
		r.reconcileServices[v.Service.ID] = v.Service
	}
}

func (r *ReplicatedOrchestrator) tickServices(ctx context.Context) {
	if len(r.reconcileServices) > 0 {
		for _, s := range r.reconcileServices {
			r.reconcile(ctx, s)
		}
		r.reconcileServices = make(map[string]*api.Service)
	}
}

func (r *ReplicatedOrchestrator) resolveService(ctx context.Context, task *api.Task) *api.Service {
	if task.ServiceID == "" {
		return nil
	}
	var service *api.Service
	r.store.View(func(tx store.ReadTx) {
		service = store.GetService(tx, task.ServiceID)
	})
	return service
}

func (r *ReplicatedOrchestrator) reconcile(ctx context.Context, service *api.Service) {
	var (
		tasks []*api.Task
		err   error
	)
	r.store.View(func(tx store.ReadTx) {
		tasks, err = store.FindTasks(tx, store.ByServiceID(service.ID))
	})
	if err != nil {
		log.G(ctx).WithError(err).Errorf("reconcile failed finding tasks")
		return
	}

	runningTasks := make([]*api.Task, 0, len(tasks))
	runningInstances := make(map[uint64]struct{}) // this could be a bitfield...
	for _, t := range tasks {
		// Technically the check below could just be
		// t.DesiredState <= api.TaskStateRunning, but ignoring tasks
		// with DesiredState == NEW simplifies the drainer unit tests.
		if t.DesiredState >= api.TaskStateReady && t.DesiredState <= api.TaskStateRunning {
			runningTasks = append(runningTasks, t)
			runningInstances[t.Instance] = struct{}{}
		}
	}
	numTasks := len(runningTasks)

	deploy := service.Spec.GetMode().(*api.ServiceSpec_Replicated)
	specifiedInstances := int(deploy.Replicated.Instances)

	// TODO(aaronl): Add support for restart delays.

	_, err = r.store.Batch(func(batch *store.Batch) error {
		switch {
		case specifiedInstances > numTasks:
			log.G(ctx).Debugf("Service %s was scaled up from %d to %d instances", service.ID, numTasks, specifiedInstances)
			// Update all current tasks then add missing tasks
			r.updater.Update(ctx, service, runningTasks)
			r.addTasks(ctx, batch, service, runningInstances, specifiedInstances-numTasks)

		case specifiedInstances < numTasks:
			// Update up to N tasks then remove the extra
			log.G(ctx).Debugf("Service %s was scaled down from %d to %d instances", service.ID, numTasks, specifiedInstances)
			r.updater.Update(ctx, service, runningTasks[:specifiedInstances])
			r.removeTasks(ctx, batch, service, runningTasks[specifiedInstances:])

		case specifiedInstances == numTasks:
			// Simple update, no scaling - update all tasks.
			r.updater.Update(ctx, service, runningTasks)
		}
		return nil
	})

	if err != nil {
		log.G(ctx).WithError(err).Errorf("reconcile batch failed")
	}
}

func (r *ReplicatedOrchestrator) addTasks(ctx context.Context, batch *store.Batch, service *api.Service, runningInstances map[uint64]struct{}, count int) {
	instance := uint64(0)
	for i := 0; i < count; i++ {
		// Find an instance number that is missing a running task
		for {
			instance++
			if _, ok := runningInstances[instance]; !ok {
				break
			}
		}

		err := batch.Update(func(tx store.Tx) error {
			return store.CreateTask(tx, newTask(service, instance))
		})
		if err != nil {
			log.G(ctx).Errorf("Failed to create task: %v", err)
		}
	}
}

func (r *ReplicatedOrchestrator) removeTasks(ctx context.Context, batch *store.Batch, service *api.Service, tasks []*api.Task) {
	for _, t := range tasks {
		err := batch.Update(func(tx store.Tx) error {
			// TODO(aaronl): optimistic update?
			t = store.GetTask(tx, t.ID)
			if t != nil {
				t.DesiredState = api.TaskStateShutdown
				return store.UpdateTask(tx, t)
			}
			return nil
		})
		if err != nil {
			log.G(ctx).WithError(err).Errorf("removing task %s failed", t.ID)
		}
	}
}
