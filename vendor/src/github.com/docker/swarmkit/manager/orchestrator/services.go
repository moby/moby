package orchestrator

import (
	"sort"

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

func (r *ReplicatedOrchestrator) initCluster(readTx store.ReadTx) error {
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

type tasksByRunningState []*api.Task

func (ts tasksByRunningState) Len() int      { return len(ts) }
func (ts tasksByRunningState) Swap(i, j int) { ts[i], ts[j] = ts[j], ts[i] }

func (ts tasksByRunningState) Less(i, j int) bool {
	return ts[i].Status.State == api.TaskStateRunning && ts[j].Status.State != api.TaskStateRunning
}

type taskWithIndex struct {
	task *api.Task

	// index is a counter that counts this task as the nth instance of
	// the service on its node. This is used for sorting the tasks so that
	// when scaling down we leave tasks more evenly balanced.
	index int
}

type tasksByIndex []taskWithIndex

func (ts tasksByIndex) Len() int      { return len(ts) }
func (ts tasksByIndex) Swap(i, j int) { ts[i], ts[j] = ts[j], ts[i] }

func (ts tasksByIndex) Less(i, j int) bool {
	if ts[i].index < 0 {
		return false
	}
	return ts[i].index < ts[j].index
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
		if t.DesiredState > api.TaskStateNew && t.DesiredState <= api.TaskStateRunning {
			runningTasks = append(runningTasks, t)
			runningInstances[t.Slot] = struct{}{}
		}
	}
	numTasks := len(runningTasks)

	deploy := service.Spec.GetMode().(*api.ServiceSpec_Replicated)
	specifiedInstances := int(deploy.Replicated.Replicas)

	switch {
	case specifiedInstances > numTasks:
		log.G(ctx).Debugf("Service %s was scaled up from %d to %d instances", service.ID, numTasks, specifiedInstances)
		// Update all current tasks then add missing tasks
		r.updater.Update(ctx, r.cluster, service, runningTasks)
		_, err = r.store.Batch(func(batch *store.Batch) error {
			r.addTasks(ctx, batch, service, runningInstances, specifiedInstances-numTasks)
			return nil
		})
		if err != nil {
			log.G(ctx).WithError(err).Errorf("reconcile batch failed")
		}

	case specifiedInstances < numTasks:
		// Update up to N tasks then remove the extra
		log.G(ctx).Debugf("Service %s was scaled down from %d to %d instances", service.ID, numTasks, specifiedInstances)

		// Preferentially remove tasks on the nodes that have the most
		// copies of this service, to leave a more balanced result.

		// First sort tasks such that tasks which are currently running
		// (in terms of observed state) appear before non-running tasks.
		// This will cause us to prefer to remove non-running tasks, all
		// other things being equal in terms of node balance.

		sort.Sort(tasksByRunningState(runningTasks))

		// Assign each task an index that counts it as the nth copy of
		// of the service on its node (1, 2, 3, ...), and sort the
		// tasks by this counter value.

		instancesByNode := make(map[string]int)
		tasksWithIndices := make(tasksByIndex, 0, numTasks)

		for _, t := range runningTasks {
			if t.NodeID != "" {
				instancesByNode[t.NodeID]++
				tasksWithIndices = append(tasksWithIndices, taskWithIndex{task: t, index: instancesByNode[t.NodeID]})
			} else {
				tasksWithIndices = append(tasksWithIndices, taskWithIndex{task: t, index: -1})
			}
		}

		sort.Sort(tasksWithIndices)

		sortedTasks := make([]*api.Task, 0, numTasks)
		for _, t := range tasksWithIndices {
			sortedTasks = append(sortedTasks, t.task)
		}

		r.updater.Update(ctx, r.cluster, service, sortedTasks[:specifiedInstances])
		_, err = r.store.Batch(func(batch *store.Batch) error {
			r.removeTasks(ctx, batch, service, sortedTasks[specifiedInstances:])
			return nil
		})
		if err != nil {
			log.G(ctx).WithError(err).Errorf("reconcile batch failed")
		}

	case specifiedInstances == numTasks:
		// Simple update, no scaling - update all tasks.
		r.updater.Update(ctx, r.cluster, service, runningTasks)
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
			return store.CreateTask(tx, newTask(r.cluster, service, instance))
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
