package replicated

import (
	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/manager/orchestrator/restart"
	"github.com/docker/swarmkit/manager/orchestrator/update"
	"github.com/docker/swarmkit/manager/state"
	"github.com/docker/swarmkit/manager/state/store"
	"golang.org/x/net/context"
)

// An Orchestrator runs a reconciliation loop to create and destroy
// tasks as necessary for the replicated services.
type Orchestrator struct {
	store *store.MemoryStore

	reconcileServices map[string]*api.Service
	restartTasks      map[string]struct{}

	// stopChan signals to the state machine to stop running.
	stopChan chan struct{}
	// doneChan is closed when the state machine terminates.
	doneChan chan struct{}

	updater  *update.Supervisor
	restarts *restart.Supervisor

	cluster *api.Cluster // local cluster instance
}

// NewReplicatedOrchestrator creates a new replicated Orchestrator.
func NewReplicatedOrchestrator(store *store.MemoryStore) *Orchestrator {
	restartSupervisor := restart.NewSupervisor(store)
	updater := update.NewSupervisor(store, restartSupervisor)
	return &Orchestrator{
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
func (r *Orchestrator) Run(ctx context.Context) error {
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
func (r *Orchestrator) Stop() {
	close(r.stopChan)
	<-r.doneChan
	r.updater.CancelAll()
	r.restarts.CancelAll()
}

func (r *Orchestrator) tick(ctx context.Context) {
	// tickTasks must be called first, so we respond to task-level changes
	// before performing service reconcillation.
	r.tickTasks(ctx)
	r.tickServices(ctx)
}
