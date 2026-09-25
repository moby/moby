package jobs

import (
	"context"
	"sync"

	"github.com/docker/go-events"

	"github.com/moby/swarmkit/v2/api"
	"github.com/moby/swarmkit/v2/log"
	"github.com/moby/swarmkit/v2/manager/orchestrator"
	"github.com/moby/swarmkit/v2/manager/orchestrator/jobs/global"
	"github.com/moby/swarmkit/v2/manager/orchestrator/jobs/replicated"
	"github.com/moby/swarmkit/v2/manager/orchestrator/restart"
	"github.com/moby/swarmkit/v2/manager/orchestrator/taskinit"
	"github.com/moby/swarmkit/v2/manager/state/store"
)

// Reconciler is the type that holds the reconciliation logic for the
// orchestrator. It exists so that the logic of actually reconciling and
// writing to the store is separated from the orchestrator, to make the event
// handling logic in the orchestrator easier to test.
type Reconciler interface {
	taskinit.InitHandler

	ReconcileService(id string) error
}

// Orchestrator is the combined orchestrator controlling both Global and
// Replicated Jobs. Initially, these job types were two separate orchestrators,
// like the Replicated and Global orchestrators. However, it became apparent
// that because of the simplicity of Jobs as compared to Services, one combined
// orchestrator suffices for both job types.
type Orchestrator struct {
	store *store.MemoryStore

	// two reconcilers, one for each service type

	replicatedReconciler Reconciler
	globalReconciler     Reconciler

	// startOnce is a function that stops the orchestrator from being started
	// multiple times.
	startOnce sync.Once

	// restartSupervisor is the component that handles restarting tasks
	restartSupervisor restart.SupervisorInterface

	// stopChan is a channel that is closed to signal the orchestrator to stop
	// running
	stopChan chan struct{}
	// stopOnce is used to ensure that stopChan can only be closed once, just
	// in case some freak accident causes subsequent calls to Stop.
	stopOnce sync.Once
	// doneChan is closed when the orchestrator actually stops running
	doneChan chan struct{}

	// checkTasksFunc is a variable that hold taskinit.CheckTasks, but allows
	// swapping it out in testing.
	checkTasksFunc func(context.Context, *store.MemoryStore, store.ReadTx, taskinit.InitHandler, restart.SupervisorInterface) error

	// the watchChan and watchCancel provide the event stream
	watchChan   chan events.Event
	watchCancel func()
}

func NewOrchestrator(store *store.MemoryStore) *Orchestrator {
	return &Orchestrator{
		store:    store,
		stopChan: make(chan struct{}),
		doneChan: make(chan struct{}),
	}
}

// Run runs the Orchestrator reconciliation loop. It takes a context as an
// argument, but canceling this context will not stop the routine; this context
// is only for passing in logging information. Call Stop to stop the
// Orchestrator
func (o *Orchestrator) Run(ctx context.Context) {
	o.startOnce.Do(func() { o.run(ctx) })
}

// init runs the once-off initialization logic for the orchestrator. This
// includes initializing the sub-components, starting the channel watch, and
// running the initial reconciliation pass. this runs as part of the run
// method, but is broken out for the purpose of testing.
func (o *Orchestrator) init(ctx context.Context) {
	var (
		services []*api.Service
	)

	// there are several components to the Orchestrator that are interfaces
	// designed to be swapped out in testing. in production, these fields will
	// all be unset, and be initialized here. in testing, we will set fakes,
	// and this initialization will be skipped.

	if o.restartSupervisor == nil {
		o.restartSupervisor = restart.NewSupervisor(o.store)
	}

	if o.replicatedReconciler == nil {
		// the cluster might be nil, but that doesn't matter.
		o.replicatedReconciler = replicated.NewReconciler(o.store, o.restartSupervisor)
	}

	if o.globalReconciler == nil {
		o.globalReconciler = global.NewReconciler(o.store, o.restartSupervisor)
	}

	if o.checkTasksFunc == nil {
		o.checkTasksFunc = taskinit.CheckTasks
	}

	o.watchChan, o.watchCancel, _ = store.ViewAndWatch(o.store, func(tx store.ReadTx) error {
		services, _ = store.FindServices(tx, store.All)
		return nil
	})

	// checkTasksFunc is used to resume any in-progress restarts that were
	// interrupted by a leadership change. In other orchestrators, this
	// additionally queues up some tasks to be restarted. However, the jobs
	// orchestrator will make a reconciliation pass across all services
	// immediately after this, and so does not need to restart any tasks; they
	// will be restarted during this pass.
	//
	// we cannot call o.checkTasksFunc inside of store.ViewAndWatch above.
	// despite taking a callback with a ReadTx, it actually performs an Update,
	// which acquires a lock and will result in a deadlock. instead, do
	// o.checkTasksFunc here.
	o.store.View(func(tx store.ReadTx) {
		o.checkTasksFunc(ctx, o.store, tx, o.replicatedReconciler, o.restartSupervisor)
		o.checkTasksFunc(ctx, o.store, tx, o.globalReconciler, o.restartSupervisor)
	})

	for _, service := range services {
		if orchestrator.IsReplicatedJob(service) {
			if err := o.replicatedReconciler.ReconcileService(service.ID); err != nil {
				log.G(ctx).WithField(
					"service.id", service.ID,
				).WithError(err).Error("error reconciling replicated job")
			}
		}

		if orchestrator.IsGlobalJob(service) {
			if err := o.globalReconciler.ReconcileService(service.ID); err != nil {
				log.G(ctx).WithField(
					"service.id", service.ID,
				).WithError(err).Error("error reconciling global job")
			}
		}
	}
}

// run provides the actual meat of the the run operation. The call to run is
// made inside of Run, and is enclosed in a sync.Once to stop this from being
// called multiple times
func (o *Orchestrator) run(ctx context.Context) {
	ctx = log.WithModule(ctx, "orchestrator/jobs")

	// closing doneChan should be the absolute last thing that happens in this
	// method, and so should be the absolute first thing we defer.
	defer close(o.doneChan)

	o.init(ctx)
	defer o.watchCancel()

	for {
		// first, before taking any action, see if we should stop the
		// orchestrator. if both the stop channel and the watch channel are
		// available to read, the channel that gets read is picked at random,
		// but we always want to stop if it's possible.
		select {
		case <-o.stopChan:
			return
		default:
		}

		select {
		case event := <-o.watchChan:
			o.handleEvent(ctx, event)
		case <-o.stopChan:
			// we also need to check for stop in here, in case there are no
			// updates to cause the loop to turn over.
			return
		}
	}
}

// handle event does the logic of handling one event message and calling the
// reconciler as needed. by handling the event logic in this function, we can
// make an end-run around the run-loop and avoid being at the mercy of the go
// scheduler when testing the orchestrator.
func (o *Orchestrator) handleEvent(ctx context.Context, event events.Event) {
	var (
		service *api.Service
		task    *api.Task
	)

	switch ev := event.(type) {
	case api.EventCreateService:
		service = ev.Service
	case api.EventUpdateService:
		service = ev.Service
	case api.EventDeleteService:
		if orchestrator.IsReplicatedJob(ev.Service) || orchestrator.IsGlobalJob(ev.Service) {
			orchestrator.SetServiceTasksRemove(ctx, o.store, ev.Service)
			o.restartSupervisor.ClearServiceHistory(ev.Service.ID)
		}
	case api.EventUpdateTask:
		task = ev.Task
	}

	// if this is a task event, we should check if it means the service
	// should be reconciled.
	if task != nil {
		// only bother with all this if the task has entered a terminal
		// state and we don't want that to have happened.
		if task.Status.State > api.TaskStateRunning && task.DesiredState <= api.TaskStateCompleted {
			o.store.View(func(tx store.ReadTx) {
				// if for any reason the service ID is invalid, then
				// service will just be nil and nothing needs to be
				// done
				service = store.GetService(tx, task.ServiceID)
			})
		}
	}

	if orchestrator.IsReplicatedJob(service) {
		if err := o.replicatedReconciler.ReconcileService(service.ID); err != nil {
			log.G(ctx).WithField(
				"service.id", service.ID,
			).WithError(err).Error("error reconciling replicated job")
		}
	}

	if orchestrator.IsGlobalJob(service) {
		if err := o.globalReconciler.ReconcileService(service.ID); err != nil {
			log.G(ctx).WithField(
				"service.id", service.ID,
			).WithError(err).Error("error reconciling global job")
		}
	}
}

// Stop stops the Orchestrator
func (o *Orchestrator) Stop() {
	// close stopChan inside of the Once so that there can be no races
	// involving multiple attempts to close stopChan.
	o.stopOnce.Do(func() {
		close(o.stopChan)
	})
	// now, we wait for the Orchestrator to stop. this wait is unqualified; we
	// will not return until Orchestrator has stopped successfully.
	<-o.doneChan
}
