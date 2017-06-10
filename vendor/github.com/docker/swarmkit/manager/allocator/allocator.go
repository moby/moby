package allocator

import (
	"sync"

	"github.com/docker/docker/pkg/plugingetter"
	"github.com/docker/go-events"
	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/manager/state"
	"github.com/docker/swarmkit/manager/state/store"
	"golang.org/x/net/context"
)

// Allocator controls how the allocation stage in the manager is handled.
type Allocator struct {
	// The manager store.
	store *store.MemoryStore

	// the ballot used to synchronize across all allocators to ensure
	// all of them have completed their respective allocations so that the
	// task can be moved to ALLOCATED state.
	taskBallot *taskBallot

	// context for the network allocator that will be needed by
	// network allocator.
	netCtx *networkContext

	// stopChan signals to the allocator to stop running.
	stopChan chan struct{}
	// doneChan is closed when the allocator is finished running.
	doneChan chan struct{}

	// pluginGetter provides access to docker's plugin inventory.
	pluginGetter plugingetter.PluginGetter
}

// taskBallot controls how the voting for task allocation is
// coordinated b/w different allocators. This the only structure that
// will be written by all allocator goroutines concurrently. Hence the
// mutex.
type taskBallot struct {
	sync.Mutex

	// List of registered voters who have to cast their vote to
	// indicate their allocation complete
	voters []string

	// List of votes collected for every task so far from different voters.
	votes map[string][]string
}

// allocActor controls the various phases in the lifecycle of one kind of allocator.
type allocActor struct {
	// Task voter identity of the allocator.
	taskVoter string

	// Action routine which is called for every event that the
	// allocator received.
	action func(context.Context, events.Event)

	// Init routine which is called during the initialization of
	// the allocator.
	init func(ctx context.Context) error
}

// New returns a new instance of Allocator for use during allocation
// stage of the manager.
func New(store *store.MemoryStore, pg plugingetter.PluginGetter) (*Allocator, error) {
	a := &Allocator{
		store: store,
		taskBallot: &taskBallot{
			votes: make(map[string][]string),
		},
		stopChan:     make(chan struct{}),
		doneChan:     make(chan struct{}),
		pluginGetter: pg,
	}

	return a, nil
}

// Run starts all allocator go-routines and waits for Stop to be called.
func (a *Allocator) Run(ctx context.Context) error {
	// Setup cancel context for all goroutines to use.
	ctx, cancel := context.WithCancel(ctx)
	var (
		wg     sync.WaitGroup
		actors []func() error
	)

	defer func() {
		cancel()
		wg.Wait()
		close(a.doneChan)
	}()

	for _, aa := range []allocActor{
		{
			taskVoter: networkVoter,
			init:      a.doNetworkInit,
			action:    a.doNetworkAlloc,
		},
	} {
		if aa.taskVoter != "" {
			a.registerToVote(aa.taskVoter)
		}

		// Assign a pointer for variable capture
		aaPtr := &aa
		actor := func() error {
			wg.Add(1)
			defer wg.Done()

			// init might return an allocator specific context
			// which is a child of the passed in context to hold
			// allocator specific state
			watch, watchCancel, err := a.init(ctx, aaPtr)
			if err != nil {
				return err
			}

			wg.Add(1)
			go func(watch <-chan events.Event, watchCancel func()) {
				defer func() {
					wg.Done()
					watchCancel()
				}()
				a.run(ctx, *aaPtr, watch)
			}(watch, watchCancel)
			return nil
		}

		actors = append(actors, actor)
	}

	for _, actor := range actors {
		if err := actor(); err != nil {
			return err
		}
	}

	<-a.stopChan
	return nil
}

// Stop stops the allocator
func (a *Allocator) Stop() {
	close(a.stopChan)
	// Wait for all allocator goroutines to truly exit
	<-a.doneChan
}

func (a *Allocator) init(ctx context.Context, aa *allocActor) (<-chan events.Event, func(), error) {
	watch, watchCancel := state.Watch(a.store.WatchQueue(),
		api.EventCreateNetwork{},
		api.EventDeleteNetwork{},
		api.EventCreateService{},
		api.EventUpdateService{},
		api.EventDeleteService{},
		api.EventCreateTask{},
		api.EventUpdateTask{},
		api.EventDeleteTask{},
		api.EventCreateNode{},
		api.EventUpdateNode{},
		api.EventDeleteNode{},
		state.EventCommit{},
	)

	if err := aa.init(ctx); err != nil {
		watchCancel()
		return nil, nil, err
	}

	return watch, watchCancel, nil
}

func (a *Allocator) run(ctx context.Context, aa allocActor, watch <-chan events.Event) {
	for {
		select {
		case ev, ok := <-watch:
			if !ok {
				return
			}

			aa.action(ctx, ev)
		case <-ctx.Done():
			return
		}
	}
}

func (a *Allocator) registerToVote(name string) {
	a.taskBallot.Lock()
	defer a.taskBallot.Unlock()

	a.taskBallot.voters = append(a.taskBallot.voters, name)
}

func (a *Allocator) taskAllocateVote(voter string, id string) bool {
	a.taskBallot.Lock()
	defer a.taskBallot.Unlock()

	// If voter has already voted, return false
	for _, v := range a.taskBallot.votes[id] {
		// check if voter is in x
		if v == voter {
			return false
		}
	}

	a.taskBallot.votes[id] = append(a.taskBallot.votes[id], voter)

	// We haven't gotten enough votes yet
	if len(a.taskBallot.voters) > len(a.taskBallot.votes[id]) {
		return false
	}

nextVoter:
	for _, voter := range a.taskBallot.voters {
		for _, vote := range a.taskBallot.votes[id] {
			if voter == vote {
				continue nextVoter
			}
		}

		// Not every registered voter has registered a vote.
		return false
	}

	return true
}
