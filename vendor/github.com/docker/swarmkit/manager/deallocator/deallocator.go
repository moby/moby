package deallocator

import (
	"context"
	"sync"

	"github.com/docker/go-events"
	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/log"
	"github.com/docker/swarmkit/manager/state/store"
)

// Deallocator waits for services to fully shutdown (ie no containers left)
// and then proceeds to deallocate service-level resources (e.g. networks),
// and finally services themselves
// in particular, the Deallocator should be the only place where services, or
// service-level resources, are ever deleted!
//
// It’s worth noting that this new component’s role is quite different from
// the task reaper’s: tasks are purely internal to Swarmkit, and their status
// is entirely managed by the system itself. In contrast, the deallocator is
// responsible for safely deleting entities that are directly controlled by the
// user.
//
// NOTE: since networks are the only service-level resources as of now,
// it has been deemed over-engineered to have a generic way to
// handle other types of service-level resources; if we ever start
// having more of those and thus want to reconsider this choice, it
// might be worth having a look at this archived branch, that does
// implement a way of separating the code for the deallocator itself
// from each resource-speficic way of handling it
// https://github.com/docker/swarmkit/compare/a84c01f49091167dd086c26b45dc18b38d52e4d9...wk8:wk8/generic_deallocator#diff-75f4f75eee6a6a7a7268c672203ea0ac
type Deallocator struct {
	store *store.MemoryStore

	// closeOnce ensures that stopChan is closed only once
	closeOnce sync.Once

	// for services that are shutting down, we keep track of how many
	// tasks still exist for them
	services map[string]*serviceWithTaskCounts

	// mainly used for tests, so that we can peek
	// into the DB state in between events
	// the bool notifies whether any DB update was actually performed
	eventChan chan bool

	stopChan chan struct{}
	doneChan chan struct{}
}

// used in our internal state's `services` right above
type serviceWithTaskCounts struct {
	service   *api.Service
	taskCount int
}

// New creates a new deallocator
func New(store *store.MemoryStore) *Deallocator {
	return &Deallocator{
		store:    store,
		services: make(map[string]*serviceWithTaskCounts),

		stopChan: make(chan struct{}),
		doneChan: make(chan struct{}),
	}
}

// Run starts the deallocator, which then starts cleaning up services
// and their resources when relevant (ie when no tasks still exist
// for a given service)
// This is a blocking function
func (deallocator *Deallocator) Run(ctx context.Context) error {
	var (
		allServices []*api.Service
		allNetworks []*api.Network
	)

	eventsChan, _, err := store.ViewAndWatch(deallocator.store,
		func(readTx store.ReadTx) (err error) {
			// look for services that are marked for deletion
			// there's no index on the `PendingDelete` field in the store,
			// so we just iterate over all of them and filter manually
			// this is okay since we only do this at leadership change
			allServices, err = store.FindServices(readTx, store.All)

			if err != nil {
				log.G(ctx).WithError(err).Error("failed to list services in deallocator init")
				return err
			}

			// now we also need to look at all existing service-level networks
			// that may be marked for deletion
			if allNetworks, err = store.FindNetworks(readTx, store.All); err != nil {
				log.G(ctx).WithError(err).Error("failed to list networks in deallocator init")
				return err
			}

			return
		},
		api.EventUpdateTask{},
		api.EventDeleteTask{},
		api.EventUpdateService{},
		api.EventUpdateNetwork{})

	if err != nil {
		// if we have an error here, we can't proceed any further
		log.G(ctx).WithError(err).Error("failed to initialize the deallocator")
		return err
	}

	defer func() {
		// eventsChanCancel()
		close(deallocator.doneChan)
	}()

	anyUpdated := false
	// now let's populate our internal taskCounts
	for _, service := range allServices {
		if updated, _ := deallocator.processService(ctx, service); updated {
			anyUpdated = true
		}
	}

	// and deallocate networks that may be marked for deletion and aren't used any more
	for _, network := range allNetworks {
		if updated, _ := deallocator.processNetwork(ctx, nil, network, nil); updated {
			anyUpdated = true
		}
	}

	// now we just need to wait for events
	deallocator.notifyEventChan(anyUpdated)
	for {
		select {
		case event := <-eventsChan:
			if updated, err := deallocator.handleEvent(ctx, event); err == nil {
				deallocator.notifyEventChan(updated)
			} else {
				log.G(ctx).WithError(err).Errorf("error processing deallocator event %#v", event)
			}
		case <-deallocator.stopChan:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// Stop stops the deallocator's routine and wait for the main loop to exit
// Stop can be called in two cases. One when the manager is
// shutting down, and the other when the manager (the leader) is
// becoming a follower. Since these two instances could race with
// each other, we use closeOnce here to ensure that TaskReaper.Stop()
// is called only once to avoid a panic.
func (deallocator *Deallocator) Stop() {
	deallocator.closeOnce.Do(func() {
		close(deallocator.stopChan)
	})
	<-deallocator.doneChan
}

// always a bno-op, except when running tests tests
// see the comment about `Deallocator`s' `eventChan` field
func (deallocator *Deallocator) notifyEventChan(updated bool) {
	if deallocator.eventChan != nil {
		deallocator.eventChan <- updated
	}
}

// if a service is marked for deletion, this checks whether it's ready to be
// deleted yet, and does it if relevant
func (deallocator *Deallocator) processService(ctx context.Context, service *api.Service) (bool, error) {
	if !service.PendingDelete {
		return false, nil
	}

	var (
		tasks []*api.Task
		err   error
	)

	deallocator.store.View(func(tx store.ReadTx) {
		tasks, err = store.FindTasks(tx, store.ByServiceID(service.ID))
	})

	if err != nil {
		log.G(ctx).WithError(err).Errorf("failed to retrieve the list of tasks for service %v", service.ID)
		// if in doubt, let's proceed to clean up the service anyway
		// better to clean up resources that shouldn't be cleaned up yet
		// than ending up with a service and some resources lost in limbo forever
		return true, deallocator.deallocateService(ctx, service)
	}

	remainingTasks := 0
	for _, task := range tasks {
		if isTaskStillAlive(task) {
			remainingTasks++
		}
	}

	if remainingTasks == 0 {
		// no tasks remaining for this service, we can clean it up
		return true, deallocator.deallocateService(ctx, service)
	}

	deallocator.services[service.ID] = &serviceWithTaskCounts{service: service, taskCount: remainingTasks}
	return false, nil
}

func (deallocator *Deallocator) deallocateService(ctx context.Context, service *api.Service) (err error) {
	err = deallocator.store.Update(func(tx store.Tx) error {
		// first, let's delete the service
		var ignoreServiceID *string
		if err := store.DeleteService(tx, service.ID); err != nil {
			// all errors are just for logging here, we do a best effort at cleaning up everything we can
			log.G(ctx).WithError(err).Errorf("failed to delete service record ID %v", service.ID)
			ignoreServiceID = &service.ID
		}

		// then all of its networks, provided no other service uses them
		spec := service.Spec
		// see https://github.com/docker/swarmkit/blob/e2aafdd3453d2ab103dd97364f79ea6b857f9446/api/specs.proto#L80-L84
		// we really should have a helper function on services to do this...
		networkConfigs := spec.Task.Networks
		if len(networkConfigs) == 0 {
			networkConfigs = spec.Networks
		}
		for _, networkConfig := range networkConfigs {
			if network := store.GetNetwork(tx, networkConfig.Target); network != nil {
				deallocator.processNetwork(ctx, tx, network, ignoreServiceID)
			}
		}

		return nil
	})

	if err != nil {
		log.G(ctx).WithError(err).Errorf("DB error when deallocating service %v", service.ID)
	}
	return
}

// proceeds to deallocate a network if it's pending deletion and there no
// longer are any services using it
// actually deletes the network if it's marked for deletion and no services are
// using it any more (or the only one using it has ID `ignoreServiceID`, if not
// nil - this comes in handy when there's been an error deleting a service)
// This function can be called either when deallocating a whole service, or
// because there was an `EventUpdateNetwork` event - in the former case, the
// transaction will be that of the service deallocation, in the latter it will be nil
func (deallocator *Deallocator) processNetwork(ctx context.Context, tx store.Tx, network *api.Network, ignoreServiceID *string) (updated bool, err error) {
	if !network.PendingDelete {
		return
	}

	updateFunc := func(t store.Tx) error {
		services, err := store.FindServices(t, store.ByReferencedNetworkID(network.ID))

		if err != nil {
			log.G(ctx).WithError(err).Errorf("could not fetch services using network ID %v", network.ID)
			return err
		}

		noMoreServices := len(services) == 0 ||
			len(services) == 1 && ignoreServiceID != nil && services[0].ID == *ignoreServiceID

		if noMoreServices {
			return store.DeleteNetwork(t, network.ID)
		}
		return nil
	}

	if tx == nil {
		err = deallocator.store.Update(updateFunc)
	} else {
		err = updateFunc(tx)
	}

	if err != nil {
		log.G(ctx).WithError(err).Errorf("DB error when deallocating network ID %v", network.ID)
	}
	return
}

// Handles new events, and dispatches to the right method depending on what
// type of event it is.
// The boolean part of the return tuple indicates whether anything was actually
// removed from the store
func (deallocator *Deallocator) handleEvent(ctx context.Context, event events.Event) (bool, error) {
	switch typedEvent := event.(type) {
	case api.EventUpdateTask:
		return deallocator.processTaskEvent(ctx, typedEvent.Task, typedEvent.OldTask)
	case api.EventDeleteTask:
		return deallocator.processTaskEvent(ctx, nil, typedEvent.Task)
	case api.EventUpdateService:
		return deallocator.processService(ctx, typedEvent.Service)
	case api.EventUpdateNetwork:
		return deallocator.processNetwork(ctx, nil, typedEvent.Network, nil)
	default:
		return false, nil
	}
}

// Common logic for handling task update/delete events
// oldTask is the task object as it was before its update or deletion
// newTask is nil for delete events, and the new object for updates
func (deallocator *Deallocator) processTaskEvent(ctx context.Context, newTask, oldTask *api.Task) (bool, error) {
	serviceID := oldTask.ServiceID
	serviceWithCount, present := deallocator.services[serviceID]

	if present && isTaskStillAlive(oldTask) && (newTask == nil || !isTaskStillAlive(newTask)) {
		// this task belongs to a service that's shutting down, and in addition,
		// prior to  its update or deletion it was still alive, but now it's
		// not alive any more, so we decrement the counter of alive tasks for
		// this service

		if serviceWithCount.taskCount <= 1 {
			delete(deallocator.services, serviceID)
			return deallocator.processService(ctx, serviceWithCount.service)
		}
		serviceWithCount.taskCount--
	}

	return false, nil
}

// simple helper function to distinguish tasks that are still running
// from ones that are done
func isTaskStillAlive(task *api.Task) bool {
	return task.Status.State <= api.TaskStateRunning
}
