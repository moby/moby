package orchestrator

import (
	"time"

	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/identity"
	"github.com/docker/swarmkit/log"
	"github.com/docker/swarmkit/manager/state"
	"github.com/docker/swarmkit/manager/state/store"
	"github.com/docker/swarmkit/protobuf/ptypes"
	"golang.org/x/net/context"
)

// A ReplicatedOrchestrator runs a reconciliation loop to create and destroy
// tasks as necessary for the replicated services.
type ReplicatedOrchestrator struct {
	store *store.MemoryStore

	reconcileServices map[string]*api.Service
	restartTasks      map[string]struct{}

	// stopChan signals to the state machine to stop running.
	stopChan chan struct{}
	// doneChan is closed when the state machine terminates.
	doneChan chan struct{}

	updater  *UpdateSupervisor
	restarts *RestartSupervisor

	cluster *api.Cluster // local cluster instance
}

// NewReplicatedOrchestrator creates a new ReplicatedOrchestrator.
func NewReplicatedOrchestrator(store *store.MemoryStore) *ReplicatedOrchestrator {
	restartSupervisor := NewRestartSupervisor(store)
	updater := NewUpdateSupervisor(store, restartSupervisor)
	return &ReplicatedOrchestrator{
		store:             store,
		stopChan:          make(chan struct{}),
		doneChan:          make(chan struct{}),
		reconcileServices: make(map[string]*api.Service),
		restartTasks:      make(map[string]struct{}),
		updater:           updater,
		restarts:          restartSupervisor,
	}
}

// Run contains the orchestrator event loop. It runs until Stop is called.
func (r *ReplicatedOrchestrator) Run(ctx context.Context) error {
	defer close(r.doneChan)

	// Watch changes to services and tasks
	queue := r.store.WatchQueue()
	watcher, cancel := queue.Watch()
	defer cancel()

	// Balance existing services and drain initial tasks attached to invalid
	// nodes
	var err error
	r.store.View(func(readTx store.ReadTx) {
		if err = r.initTasks(ctx, readTx); err != nil {
			return
		}

		if err = r.initServices(readTx); err != nil {
			return
		}

		if err = r.initCluster(readTx); err != nil {
			return
		}
	})
	if err != nil {
		return err
	}

	r.tick(ctx)

	for {
		select {
		case event := <-watcher:
			// TODO(stevvooe): Use ctx to limit running time of operation.
			r.handleTaskEvent(ctx, event)
			r.handleServiceEvent(ctx, event)
			switch v := event.(type) {
			case state.EventCommit:
				r.tick(ctx)
			case state.EventUpdateCluster:
				r.cluster = v.Cluster
			}
		case <-r.stopChan:
			return nil
		}
	}
}

// Stop stops the orchestrator.
func (r *ReplicatedOrchestrator) Stop() {
	close(r.stopChan)
	<-r.doneChan
	r.updater.CancelAll()
	r.restarts.CancelAll()
}

func (r *ReplicatedOrchestrator) tick(ctx context.Context) {
	// tickTasks must be called first, so we respond to task-level changes
	// before performing service reconcillation.
	r.tickTasks(ctx)
	r.tickServices(ctx)
}

func newTask(cluster *api.Cluster, service *api.Service, instance uint64) *api.Task {
	var logDriver *api.Driver
	if service.Spec.Task.LogDriver != nil {
		// use the log driver specific to the task, if we have it.
		logDriver = service.Spec.Task.LogDriver
	} else if cluster != nil {
		// pick up the cluster default, if available.
		logDriver = cluster.Spec.TaskDefaults.LogDriver // nil is okay here.
	}

	// NOTE(stevvooe): For now, we don't override the container naming and
	// labeling scheme in the agent. If we decide to do this in the future,
	// they should be overridden here.
	return &api.Task{
		ID:                 identity.NewID(),
		ServiceAnnotations: service.Spec.Annotations,
		Spec:               service.Spec.Task,
		ServiceID:          service.ID,
		Slot:               instance,
		Status: api.TaskStatus{
			State:     api.TaskStateNew,
			Timestamp: ptypes.MustTimestampProto(time.Now()),
			Message:   "created",
		},
		Endpoint: &api.Endpoint{
			Spec: service.Spec.Endpoint.Copy(),
		},
		DesiredState: api.TaskStateRunning,
		LogDriver:    logDriver,
	}
}

// isReplicatedService checks if a service is a replicated service
func isReplicatedService(service *api.Service) bool {
	// service nil validation is required as there are scenarios
	// where service is removed from store
	if service == nil {
		return false
	}
	_, ok := service.Spec.GetMode().(*api.ServiceSpec_Replicated)
	return ok
}

func deleteServiceTasks(ctx context.Context, s *store.MemoryStore, service *api.Service) {
	var (
		tasks []*api.Task
		err   error
	)
	s.View(func(tx store.ReadTx) {
		tasks, err = store.FindTasks(tx, store.ByServiceID(service.ID))
	})
	if err != nil {
		log.G(ctx).WithError(err).Errorf("failed to list tasks")
		return
	}

	_, err = s.Batch(func(batch *store.Batch) error {
		for _, t := range tasks {
			err := batch.Update(func(tx store.Tx) error {
				if err := store.DeleteTask(tx, t.ID); err != nil {
					log.G(ctx).WithError(err).Errorf("failed to delete task")
				}
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		log.G(ctx).WithError(err).Errorf("task search transaction failed")
	}
}

func restartCondition(task *api.Task) api.RestartPolicy_RestartCondition {
	restartCondition := api.RestartOnAny
	if task.Spec.Restart != nil {
		restartCondition = task.Spec.Restart.Condition
	}
	return restartCondition
}
