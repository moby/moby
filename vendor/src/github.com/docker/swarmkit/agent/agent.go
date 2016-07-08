package agent

import (
	"fmt"
	"math/rand"
	"reflect"
	"time"

	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/log"
	"golang.org/x/net/context"
)

const (
	initialSessionFailureBackoff = time.Second
	maxSessionFailureBackoff     = 8 * time.Second
)

// Agent implements the primary node functionality for a member of a swarm
// cluster. The primary functionality id to run and report on the status of
// tasks assigned to the node.
type Agent struct {
	config *Config

	// The latest node object state from manager
	// for this node known to the agent.
	node *api.Node

	keys []*api.EncryptionKey

	sessionq chan sessionOperation
	worker   Worker

	started chan struct{}
	ready   chan struct{}
	stopped chan struct{} // requests shutdown
	closed  chan struct{} // only closed in run
	err     error         // read only after closed is closed
}

// New returns a new agent, ready for task dispatch.
func New(config *Config) (*Agent, error) {
	if err := config.validate(); err != nil {
		return nil, err
	}

	a := &Agent{
		config:   config,
		worker:   newWorker(config.DB, config.Executor),
		sessionq: make(chan sessionOperation),
		started:  make(chan struct{}),
		stopped:  make(chan struct{}),
		closed:   make(chan struct{}),
		ready:    make(chan struct{}),
	}

	return a, nil
}

// Start begins execution of the agent in the provided context, if not already
// started.
func (a *Agent) Start(ctx context.Context) error {
	select {
	case <-a.started:
		select {
		case <-a.closed:
			return a.err
		case <-a.stopped:
			return errAgentStopped
		case <-ctx.Done():
			return ctx.Err()
		default:
			return errAgentStarted
		}
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	close(a.started)
	go a.run(ctx)

	return nil
}

// Stop shuts down the agent, blocking until full shutdown. If the agent is not
// started, Stop will block until Started.
func (a *Agent) Stop(ctx context.Context) error {
	select {
	case <-a.started:
		select {
		case <-a.closed:
			return a.err
		case <-a.stopped:
			select {
			case <-a.closed:
				return a.err
			case <-ctx.Done():
				return ctx.Err()
			}
		case <-ctx.Done():
			return ctx.Err()
		default:
			close(a.stopped)
			// recurse and wait for closure
			return a.Stop(ctx)
		}
	case <-ctx.Done():
		return ctx.Err()
	default:
		return errAgentNotStarted
	}
}

// Err returns the error that caused the agent to shutdown or nil. Err blocks
// until the agent is fully shutdown.
func (a *Agent) Err(ctx context.Context) error {
	select {
	case <-a.closed:
		return a.err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Ready returns a channel that will be closed when agent first becomes ready.
func (a *Agent) Ready() <-chan struct{} {
	return a.ready
}

func (a *Agent) run(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer close(a.closed) // full shutdown.

	ctx = log.WithLogger(ctx, log.G(ctx).WithField("module", "agent"))

	log.G(ctx).Debugf("(*Agent).run")
	defer log.G(ctx).Debugf("(*Agent).run exited")

	var (
		backoff    time.Duration
		session    = newSession(ctx, a, backoff) // start the initial session
		registered = session.registered
		ready      = a.ready // first session ready
		sessionq   chan sessionOperation
	)

	if err := a.worker.Init(ctx); err != nil {
		log.G(ctx).WithError(err).Error("worker initialization failed")
		a.err = err
		return // fatal?
	}

	// setup a reliable reporter to call back to us.
	reporter := newStatusReporter(ctx, a)
	defer reporter.Close()

	a.worker.Listen(ctx, reporter)

	for {
		select {
		case operation := <-sessionq:
			operation.response <- operation.fn(session)
		case msg := <-session.tasks:
			if err := a.worker.Assign(ctx, msg.Tasks); err != nil {
				log.G(ctx).WithError(err).Error("task assignment failed")
			}
		case msg := <-session.messages:
			if err := a.handleSessionMessage(ctx, msg); err != nil {
				log.G(ctx).WithError(err).Error("session message handler failed")
			}
		case <-registered:
			log.G(ctx).Debugln("agent: registered")
			if ready != nil {
				close(ready)
			}
			ready = nil
			registered = nil // we only care about this once per session
			backoff = 0      // reset backoff
			sessionq = a.sessionq
		case err := <-session.errs:
			// TODO(stevvooe): This may actually block if a session is closed
			// but no error was sent. Session.close must only be called here
			// for this to work.
			if err != nil {
				log.G(ctx).WithError(err).Error("agent: session failed")
				backoff = initialSessionFailureBackoff + 2*backoff
				if backoff > maxSessionFailureBackoff {
					backoff = maxSessionFailureBackoff
				}
			}

			if err := session.close(); err != nil {
				log.G(ctx).WithError(err).Error("agent: closing session failed")
			}
			sessionq = nil
			// if we're here before <-registered, do nothing for that event
			registered = nil

			// Bounce the connection.
			if a.config.Picker != nil {
				a.config.Picker.Reset()
			}
		case <-session.closed:
			log.G(ctx).Debugf("agent: rebuild session")

			// select a session registration delay from backoff range.
			delay := time.Duration(rand.Int63n(int64(backoff)))
			session = newSession(ctx, a, delay)
			registered = session.registered
			sessionq = a.sessionq
		case <-a.stopped:
			// TODO(stevvooe): Wait on shutdown and cleanup. May need to pump
			// this loop a few times.
			return
		case <-ctx.Done():
			if a.err == nil {
				a.err = ctx.Err()
			}

			return
		}
	}
}

func (a *Agent) handleSessionMessage(ctx context.Context, message *api.SessionMessage) error {
	seen := map[api.Peer]struct{}{}
	for _, manager := range message.Managers {
		if manager.Peer.Addr == "" {
			log.G(ctx).WithField("manager.addr", manager.Peer.Addr).
				Warnf("skipping bad manager address")
			continue
		}

		a.config.Managers.Observe(*manager.Peer, int(manager.Weight))
		seen[*manager.Peer] = struct{}{}
	}

	if message.Node != nil {
		if a.node == nil || !nodesEqual(a.node, message.Node) {
			if a.config.NotifyRoleChange != nil {
				a.config.NotifyRoleChange <- message.Node.Spec.Role
			}
			a.node = message.Node.Copy()
			if err := a.config.Executor.Configure(ctx, a.node); err != nil {
				log.G(ctx).WithError(err).Error("node configure failed")
			}
		}
	}

	// prune managers not in list.
	for peer := range a.config.Managers.Weights() {
		if _, ok := seen[peer]; !ok {
			a.config.Managers.Remove(peer)
		}
	}

	if message.NetworkBootstrapKeys == nil {
		return nil
	}

	for _, key := range message.NetworkBootstrapKeys {
		same := false
		for _, agentKey := range a.keys {
			if agentKey.LamportTime == key.LamportTime {
				same = true
			}
		}
		if !same {
			a.keys = message.NetworkBootstrapKeys
			if err := a.config.Executor.SetNetworkBootstrapKeys(a.keys); err != nil {
				panic(fmt.Errorf("configuring network key failed"))
			}
		}
	}

	return nil
}

type sessionOperation struct {
	fn       func(session *session) error
	response chan error
}

// withSession runs fn with the current session.
func (a *Agent) withSession(ctx context.Context, fn func(session *session) error) error {
	response := make(chan error, 1)
	select {
	case a.sessionq <- sessionOperation{
		fn:       fn,
		response: response,
	}:
		select {
		case err := <-response:
			return err
		case <-a.closed:
			return ErrClosed
		case <-ctx.Done():
			return ctx.Err()
		}
	case <-a.closed:
		return ErrClosed
	case <-ctx.Done():
		return ctx.Err()
	}
}

// UpdateTaskStatus attempts to send a task status update over the current session,
// blocking until the operation is completed.
//
// If an error is returned, the operation should be retried.
func (a *Agent) UpdateTaskStatus(ctx context.Context, taskID string, status *api.TaskStatus) error {
	log.G(ctx).WithField("task.id", taskID).Debugf("(*Agent).UpdateTaskStatus")
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	errs := make(chan error, 1)
	if err := a.withSession(ctx, func(session *session) error {
		go func() {
			err := session.sendTaskStatus(ctx, taskID, status)
			if err != nil {
				if err == errTaskUnknown {
					err = nil // dispatcher no longer cares about this task.
				} else {
					log.G(ctx).WithError(err).Error("sending task status update failed")
				}
			} else {
				log.G(ctx).Debug("task status reported")
			}

			errs <- err
		}()

		return nil
	}); err != nil {
		return err
	}

	select {
	case err := <-errs:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// nodesEqual returns true if the node states are functionaly equal, ignoring status,
// version and other superfluous fields.
//
// This used to decide whether or not to propagate a node update to executor.
func nodesEqual(a, b *api.Node) bool {
	a, b = a.Copy(), b.Copy()

	a.Status, b.Status = api.NodeStatus{}, api.NodeStatus{}
	a.Meta, b.Meta = api.Meta{}, api.Meta{}

	return reflect.DeepEqual(a, b)
}
