package agent

import (
	"errors"
	"time"

	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/log"
	"github.com/docker/swarmkit/protobuf/ptypes"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

const dispatcherRPCTimeout = 5 * time.Second

var (
	errSessionDisconnect = errors.New("agent: session disconnect") // instructed to disconnect
	errSessionClosed     = errors.New("agent: session closed")
)

// session encapsulates one round of registration with the manager. session
// starts the registration and heartbeat control cycle. Any failure will result
// in a complete shutdown of the session and it must be reestablished.
//
// All communication with the master is done through session.  Changes that
// flow into the agent, such as task assignment, are called back into the
// agent through errs, messages and tasks.
type session struct {
	agent     *Agent
	sessionID string
	session   api.Dispatcher_SessionClient
	errs      chan error
	messages  chan *api.SessionMessage
	tasks     chan *api.TasksMessage

	registered chan struct{} // closed registration
	closed     chan struct{}
}

func newSession(ctx context.Context, agent *Agent, delay time.Duration) *session {
	s := &session{
		agent:      agent,
		errs:       make(chan error),
		messages:   make(chan *api.SessionMessage),
		tasks:      make(chan *api.TasksMessage),
		registered: make(chan struct{}),
		closed:     make(chan struct{}),
	}

	go s.run(ctx, delay)
	return s
}

func (s *session) run(ctx context.Context, delay time.Duration) {
	time.Sleep(delay) // delay before registering.

	if err := s.start(ctx); err != nil {
		select {
		case s.errs <- err:
		case <-s.closed:
		case <-ctx.Done():
		}
		return
	}

	ctx = log.WithLogger(ctx, log.G(ctx).WithField("session.id", s.sessionID))

	go runctx(ctx, s.closed, s.errs, s.heartbeat)
	go runctx(ctx, s.closed, s.errs, s.watch)
	go runctx(ctx, s.closed, s.errs, s.listen)

	close(s.registered)
}

// start begins the session and returns the first SessionMessage.
func (s *session) start(ctx context.Context) error {
	log.G(ctx).Debugf("(*session).start")

	client := api.NewDispatcherClient(s.agent.config.Conn)

	description, err := s.agent.config.Executor.Describe(ctx)
	if err != nil {
		log.G(ctx).WithError(err).WithField("executor", s.agent.config.Executor).
			Errorf("node description unavailable")
		return err
	}
	// Override hostname
	if s.agent.config.Hostname != "" {
		description.Hostname = s.agent.config.Hostname
	}

	errChan := make(chan error, 1)
	var (
		msg    *api.SessionMessage
		stream api.Dispatcher_SessionClient
	)
	// Note: we don't defer cancellation of this context, because the
	// streaming RPC is used after this function returned. We only cancel
	// it in the timeout case to make sure the goroutine completes.
	sessionCtx, cancelSession := context.WithCancel(ctx)

	// Need to run Session in a goroutine since there's no way to set a
	// timeout for an individual Recv call in a stream.
	go func() {
		stream, err = client.Session(sessionCtx, &api.SessionRequest{
			Description: description,
		})
		if err != nil {
			errChan <- err
			return
		}

		msg, err = stream.Recv()
		errChan <- err
	}()

	select {
	case err := <-errChan:
		if err != nil {
			return err
		}
	case <-time.After(dispatcherRPCTimeout):
		cancelSession()
		return errors.New("session initiation timed out")
	}

	s.sessionID = msg.SessionID
	s.session = stream

	return s.handleSessionMessage(ctx, msg)
}

func (s *session) heartbeat(ctx context.Context) error {
	log.G(ctx).Debugf("(*session).heartbeat")
	client := api.NewDispatcherClient(s.agent.config.Conn)
	heartbeat := time.NewTimer(1) // send out a heartbeat right away
	defer heartbeat.Stop()

	for {
		select {
		case <-heartbeat.C:
			heartbeatCtx, cancel := context.WithTimeout(ctx, dispatcherRPCTimeout)
			resp, err := client.Heartbeat(heartbeatCtx, &api.HeartbeatRequest{
				SessionID: s.sessionID,
			})
			cancel()
			if err != nil {
				if grpc.Code(err) == codes.NotFound {
					err = errNodeNotRegistered
				}

				return err
			}

			period, err := ptypes.Duration(&resp.Period)
			if err != nil {
				return err
			}

			heartbeat.Reset(period)
		case <-s.closed:
			return errSessionClosed
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (s *session) listen(ctx context.Context) error {
	defer s.session.CloseSend()
	log.G(ctx).Debugf("(*session).listen")
	for {
		msg, err := s.session.Recv()
		if err != nil {
			return err
		}

		if err := s.handleSessionMessage(ctx, msg); err != nil {
			return err
		}
	}
}

func (s *session) handleSessionMessage(ctx context.Context, msg *api.SessionMessage) error {
	select {
	case s.messages <- msg:
		return nil
	case <-s.closed:
		return errSessionClosed
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *session) watch(ctx context.Context) error {
	log.G(ctx).Debugf("(*session).watch")
	client := api.NewDispatcherClient(s.agent.config.Conn)
	watch, err := client.Tasks(ctx, &api.TasksRequest{
		SessionID: s.sessionID})
	if err != nil {
		return err
	}

	for {
		resp, err := watch.Recv()
		if err != nil {
			return err
		}

		select {
		case s.tasks <- resp:
		case <-s.closed:
			return errSessionClosed
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// sendTaskStatus uses the current session to send the status of a single task.
func (s *session) sendTaskStatus(ctx context.Context, taskID string, status *api.TaskStatus) error {

	client := api.NewDispatcherClient(s.agent.config.Conn)
	if _, err := client.UpdateTaskStatus(ctx, &api.UpdateTaskStatusRequest{
		SessionID: s.sessionID,
		Updates: []*api.UpdateTaskStatusRequest_TaskStatusUpdate{
			{
				TaskID: taskID,
				Status: status,
			},
		},
	}); err != nil {
		// TODO(stevvooe): Dispatcher should not return this error. Status
		// reports for unknown tasks should be ignored.
		if grpc.Code(err) == codes.NotFound {
			return errTaskUnknown
		}

		return err
	}

	return nil
}

func (s *session) sendTaskStatuses(ctx context.Context, updates ...*api.UpdateTaskStatusRequest_TaskStatusUpdate) ([]*api.UpdateTaskStatusRequest_TaskStatusUpdate, error) {
	if len(updates) < 1 {
		return nil, nil
	}

	const batchSize = 1024
	select {
	case <-s.registered:
		select {
		case <-s.closed:
			return updates, ErrClosed
		default:
		}
	case <-s.closed:
		return updates, ErrClosed
	case <-ctx.Done():
		return updates, ctx.Err()
	}

	client := api.NewDispatcherClient(s.agent.config.Conn)
	n := batchSize

	if len(updates) < n {
		n = len(updates)
	}

	if _, err := client.UpdateTaskStatus(ctx, &api.UpdateTaskStatusRequest{
		SessionID: s.sessionID,
		Updates:   updates[:n],
	}); err != nil {
		log.G(ctx).WithError(err).Errorf("failed sending task status batch size of %d", len(updates[:n]))
		return updates, err
	}

	return updates[n:], nil
}

func (s *session) close() error {
	select {
	case <-s.closed:
		return errSessionClosed
	default:
		close(s.closed)
		return nil
	}
}
