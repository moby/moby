package reconciler

import (
	"errors"
	"sync"
	"time"

	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"

	"github.com/docker/stacks/pkg/interfaces"
	"github.com/docker/stacks/pkg/reconciler/dispatcher"
	"github.com/docker/stacks/pkg/reconciler/notifier"
	"github.com/docker/stacks/pkg/reconciler/reconciler"
)

const (
	// eventsChanBufferDepth defines the size of the channel buffer for events
	eventsChanBufferDepth = 30
)

// Manager is the main entrypoint for the reconciler package; users of
// the reconciler should instantiate and run a Manager. Manager is the thinnest
// part of the reconciler package, because it is a long-running blocking
// routine, making it difficult to test.
type Manager struct {
	// these Once objects are needed to safely start and stop the Manager in a
	// concurrent environment
	startOnce sync.Once
	stopOnce  sync.Once

	client interfaces.BackendClient

	// stop is a channel that indicates that Stop has been called and the
	// Manager should cease executing
	stop chan struct{}

	d dispatcher.Dispatcher
	r reconciler.Reconciler

	nodeID string
	// notifyCluster is used to signal from JoinCluster and LeaveCluster. It
	// will only ever be read from in one place, we can use a channel instead
	// of a more complicated structure like a Cond.
	notifyCluster chan struct{}
}

// New creates a new Manager, the main entrypoint for the reconciler package,
// along with all of the dependent types
func New(client interfaces.BackendClient) *Manager {
	m := &Manager{
		client: client,
		stop:   make(chan struct{}),
		// notifyCluster is buffered to 1. This means that we can leave a
		// notification in the buffer for the reader to get at any time. When
		// we try to write to the channel, we should do so in a select. If the
		// write cannot proceed, that means there is already a notification in
		// the buffer so there's no need to put another one.
		notifyCluster: make(chan struct{}, 1),
	}

	// create a new Dispatcher and Reconciler, with a NotificationForwarder to
	// put between them
	n := notifier.NewNotificationForwarder()
	m.r = reconciler.New(n, m.client)
	m.d = dispatcher.New(m.r, n)
	return m
}

// Run runs the reconciler package. It is a long-running, blocking routine, and
// should be called inside of a goroutine. When Run stops, it will return nil
// if it stopped cleanly, or an error otherwise. Run may only be called once;
// subsequent calls will return an error. This includes if the Manager is
// stopped for any reason; it may not be restarted once stopped.
func (m *Manager) Run() error {
	var (
		// ran tells us if startOnce executed. ran is false, unless it is set
		// to true in startOnce, which can only be the case if this is the
		// first call
		ran bool
		// err is the result of Run, in a variable so that it can be captured
		// by the closure in the startOnce
		err error
	)
	m.startOnce.Do(func() {
		ran = true
		// start up a loop, where we wait until we become a leader, and then we
		// run the manager, over and over again, until the stop channel is
		// closed
		for {
			m.waitReady()
			err = m.run()
			select {
			case <-m.stop:
				return
			default:
			}
		}
	})

	if !ran {
		// TODO(dperny): return a better error here
		return errors.New("already run")
	}

	return err
}

// JoinCluster notifies the Manager that this node has joined a swarmkit
// cluster.
func (m *Manager) JoinCluster() {
	// try writing to the channel if it's ready. if it's not ready, that means
	// that there is already a notification in the buffer, so we don't need to
	// put one there.
	select {
	case m.notifyCluster <- struct{}{}:
	default:
	}
}

// LeaveCluster notifies the Manager that this node has left a swarmkit cluster
func (m *Manager) LeaveCluster() {
	// same as JoinCluster
	select {
	case m.notifyCluster <- struct{}{}:
	default:
	}
}

// Stop instructs the runner to stop executing. It will cause Run to exit.
// Subsequent calls to Stop after the first have no effect.
func (m *Manager) Stop() {
	m.stopOnce.Do(func() {
		close(m.stop)
	})
}

// waitReady blocks until the node this manager is working on is a swarmkit
// leader. it can be safely re-entered any number of times, and it exits when
// the node has become the leader, or Stop has been called
func (m *Manager) waitReady() {
	// set up a watch for node events
	f := filters.NewArgs(filters.Arg("type", events.NodeEventType))
	_, eventC := m.client.SubscribeToEvents(time.Time{}, time.Time{}, f)
	defer m.client.UnsubscribeFromEvents(eventC)
	for {
		select {
		case <-m.notifyCluster:
			// the result of Info is not a pointer, so there is no need to nil
			// check it. Additionally, NodeID will be empty string if there is
			// no Info object. This handles both the JoinCluster and
			// LeaveCluster cases.
			m.nodeID = m.client.Info().NodeID
			if m.checkLeadership() {
				return
			}
		case ev, ok := <-eventC:
			// if the channel gets cut off for whatever reason, return.
			// TODO(dperny): in the "standalone" case, where this is actually
			// the result of a network socket, this might cause us to start
			// trying to run the manager when it's not supposed to. Luckily,
			// the result of that shouldn't be catastrophic, and anyway
			// standalone isn't a supported production use case.
			if !ok {
				return
			}
			if m.checkNodeEventLeadership(ev) {
				return
			}
		case <-m.stop:
			return
		}
	}
}

// checkLeadership is a helper function that checks if this node is currently
// the leader
func (m *Manager) checkLeadership() bool {
	// we can actually discard the error, because we're only
	// checking for the existence of a ManagerStatus, which the
	// default value of swarm.Node will not include
	n, _ := m.client.GetNode(m.nodeID)
	return n.ManagerStatus != nil && n.ManagerStatus.Leader
}

// checkNodeEventLeadership is a helper function that checks if this node is
// currently a leader, taking an event to decide whether to actually do API
// calls.
func (m *Manager) checkNodeEventLeadership(ev interface{}) bool {
	msg, ok := ev.(events.Message)
	// even though we passed a filter for node events, in the interest
	// of defensive programming, make sure that's really what this is.
	//
	// then, check if this message is an update to THIS node. If it is,
	// we should see if we've become the leader.
	return ok &&
		msg.Type == events.NodeEventType &&
		msg.Actor.ID == m.nodeID &&
		m.checkLeadership()
}

// run is the private method that implements the actual logic of running.
func (m *Manager) run() error {
	// Using the client, get an events channel. SubscribeToEvents takes a
	// couple of Time arguments, but we won't use them right now. Instead,
	// we'll pass a raw time.Time, which is the zero-value. Additionally, to
	// hopefully restrict the firehose a bit, we'll filter events based on
	// scope.
	f := filters.NewArgs(filters.Arg("scope", "swarm"))
	// throw away the first return value, it'll be an empty list anyway and we
	// don't need it.
	_, eventC := m.client.SubscribeToEvents(time.Time{}, time.Time{}, f)
	// make sure we unsubscribe from events when we're done. I think if we
	// don't do this, the channel may leak?
	defer m.client.UnsubscribeFromEvents(eventC)

	// now, we want to make sure that the events channel is buffered, for the
	// benefit of the Dispatcher. The dispatcher is designed such that it
	// processes batches of events all at once without using goroutines, by
	// reading from a channel as long as it's ready and not blocked. By
	// buffering this channel explicitly, we help ensure that this batching
	// proceeds smoothly and ideally, and doesn't end up blocked prematurely
	// while waiting for some other goroutine to make channel writes.
	//
	// We also need to be able to control
	// cancelation of the event stream, to in turn cancel the dispatcher. to do
	// all this, we'll create a channel to forward through, and a goroutine to
	// handle it all

	// dispatcherChan is the channel actually passed to the dispatcher. This
	// channel can ONLY be closed in the below anonymous goroutine.
	dispatcherChan := make(chan interface{}, eventsChanBufferDepth)

	// Use a WaitGroup to handle routine stoppage. This is, a bit cleaner than
	// blocking on a channel close. Also, theoretically, with this pattern we
	// could have multiple dispatchers and reconcilers, but that's an idea for
	// another day.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		// every case where we return from this function should result in the
		// dispatcherChan being closed, so just stick it in a defer.
		defer close(dispatcherChan)
		for {
			select {
			case ev, ok := <-eventC:
				if !ok {
					// TODO(dperny): what happens if we lose this channel
					// without asking for it to be shutdown? right now, this
					// case isn't handled well. We should probably start a new
					// event channel, though.
					return
				}
				// check if we're currently the leader. if we're not, we should
				// return, closing the dispatcher
				if !m.checkNodeEventLeadership(ev) {
					return
				}
				// even though dispatcherChan is buffered, we don't want to
				// block on a send. If something happens and dispatcherChan
				// gets full, we need to be able to bail out of attempting to
				// send to it.
				select {
				case dispatcherChan <- ev:
				case <-m.stop:
					return
				}
			case <-m.notifyCluster:
				if !m.checkLeadership() {
					return
				}
			case <-m.stop:
				return
			}
		}
	}()
	// now, start handling events in the Dispatcher
	err := m.d.HandleEvents(dispatcherChan)
	wg.Wait()

	// return whatever error HandleEvents returned.
	return err
}
