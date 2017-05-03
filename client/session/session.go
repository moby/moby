package session

import (
	"fmt"
	"net"
	"net/http"
	"regexp"
	"strings"
	"sync"

	"github.com/docker/docker/pkg/stringid"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

// Stream allows moving messages between endpoints
type Stream interface {
	RecvMsg(interface{}) error
	SendMsg(interface{}) error
	Context() context.Context
	// CloseSend() error
}

// HandleFunc is a handler for a request
type HandleFunc func(ctx context.Context, opts map[string][]string, stream *Stream) error

// Dialer returns a connection that can be used by the session
type Dialer func(ctx context.Context, proto string, meta map[string][]string) (net.Conn, error)

// Attachment is a feature exposed to the session
type Attachment interface {
	RegisterHandlers(func(id, method string) error) error
	Handle(ctx context.Context, id, method string, opts map[string][]string, stream Stream) error
}

// Caller can invoke requests on the session
type Caller interface {
	Supports(id, method string) bool
	Call(ctx context.Context, id, method string, opts map[string][]string) (Stream, error)
	Name() string
	SharedKey() string
	Close() error
}

// Handler can respond to requests on the session
type Handler interface {
	RegisterHandlers(func(id, method string)) error
	Handle(id, method string, opts map[string][]string, stream Stream) error
}

// TransportFactory returns a new transport used by the session
type TransportFactory interface {
	NewHandler(ctx context.Context, conn net.Conn) (TransportHandler, error)
	NewCaller(ctx context.Context, conn net.Conn) (TransportCaller, error)
	Name() string
	ProtoName() string
	ToMethodName(id, method string) string
	FromMethodName(method string) (string, string, error)
}

// TransportHandler is handler definition that transport needs to support
type TransportHandler interface {
	Register(id, method string, f HandleFunc) error
	Serve(ctx context.Context) error
}

// TransportCaller is a caller definition that transport needs to support
type TransportCaller interface {
	Call(ctx context.Context, id, method string, opts map[string][]string) (Stream, error)
}

type callbackSelector struct {
	id     string
	method string
}

// Session is a long running connection between client and a daemon
type Session struct {
	uuid      string
	name      string
	sharedKey string
	handlers  map[callbackSelector]HandleFunc
	ctx       context.Context
	cancelCtx func()
	done      chan struct{}
}

// NewSession returns a new long running session
func NewSession(name, sharedKey string) (*Session, error) {
	uuid := stringid.GenerateRandomID()
	s := &Session{
		uuid:      uuid,
		name:      name,
		sharedKey: sharedKey,
		handlers:  make(map[callbackSelector]HandleFunc),
	}
	return s, nil
}

// UUID returns unique identifier for the session
func (s *Session) UUID() string {
	return s.uuid
}

// Allow allows a new feature on the session
func (s *Session) Allow(a Attachment) error {
	return a.RegisterHandlers(func(id, method string) error {
		handler := func(ctx context.Context, opts map[string][]string, stream *Stream) error {
			return a.Handle(ctx, id, method, opts, *stream)
		}
		cbs := callbackSelector{id: id, method: method}
		if _, ok := s.handlers[cbs]; ok {
			return errors.Errorf("handler for %s %s already exists", id, method)
		}
		s.handlers[cbs] = handler
		return nil
	})
}

// Run activates the session
func (s *Session) Run(ctx context.Context, dialer Dialer, tf TransportFactory) error {
	ctx, cancel := context.WithCancel(ctx)
	s.cancelCtx = cancel
	s.done = make(chan struct{})

	defer cancel()
	defer close(s.done)

	meta := make(map[string][]string)
	meta["X-Docker-Expose-Session-UUID"] = []string{s.uuid}
	meta["X-Docker-Expose-Session-Name"] = []string{s.name}
	meta["X-Docker-Expose-Session-SharedKey"] = []string{s.sharedKey}

	for s := range s.handlers {
		k := fmt.Sprintf("X-Docker-Expose-Session-%s-Method", tf.Name())
		meta[k] = append(meta[k], tf.ToMethodName(s.id, s.method))
	}

	conn, err := dialer(ctx, tf.ProtoName(), meta)
	if err != nil {
		return errors.Wrapf(err, "failed to dial %s", tf.ProtoName())
	}

	h, err := tf.NewHandler(ctx, conn)
	if err != nil {
		return err
	}
	for s, f := range s.handlers {
		if err := h.Register(s.id, s.method, f); err != nil {
			return err
		}
	}
	return h.Serve(ctx)
}

// Close closes the session
func (s *Session) Close() error {
	if s.cancelCtx != nil && s.done != nil {
		s.cancelCtx()
		<-s.done
	}
	return nil
}

func (s *Session) context() context.Context {
	return s.ctx
}

func (s *Session) closed() bool {
	select {
	case <-s.context().Done():
		return true
	default:
		return false
	}
}

// Manager is a controller for accessing currently active sessions
type Manager struct {
	tfs      []TransportFactory
	sessions map[string]*Session
	mu       sync.Mutex
	c        *sync.Cond
}

// NewManager returns a new Manager
func NewManager(tfs ...TransportFactory) (*Manager, error) {
	sm := &Manager{
		tfs:      tfs,
		sessions: make(map[string]*Session),
	}
	sm.c = sync.NewCond(&sm.mu)
	return sm, nil
}

// HandleHTTPRequest handles an incoming HTTP request
func (sm *Manager) HandleHTTPRequest(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		return errors.New("handler does not support hijack")
	}

	uuid := r.Header.Get("X-Docker-Expose-Session-UUID")
	name := r.Header.Get("X-Docker-Expose-Session-Name")
	sharedKey := r.Header.Get("X-Docker-Expose-Session-SharedKey")

	proto := r.Header.Get("Upgrade")

	sm.mu.Lock()
	if _, ok := sm.sessions[uuid]; ok {
		return errors.Errorf("session %s already exists", uuid)
	}

	if proto == "" {
		return errors.New("no upgrade proto in request")
	}

	var t TransportFactory
	for _, tf := range sm.tfs {
		if tf.ProtoName() == proto {
			t = tf
			continue
		}
	}

	if t == nil {
		return errors.Errorf("no transport for protocol %s", proto)
	}

	conn, _, err := hijacker.Hijack()
	if err != nil {
		return errors.Wrap(err, "failed to hijack connection")
	}

	// set raw mode
	conn.Write([]byte{})

	fmt.Fprintf(conn, "HTTP/1.1 101 Switching Protocols\r\nConnection: Upgrade\r\nUpgrade: %s\r\n\r\n", proto)

	ctx, cancel := context.WithCancel(ctx)

	caller, err := t.NewCaller(ctx, conn)
	if err != nil {
		return errors.Wrap(err, "failed to create new caller")
	}

	s := &Session{
		uuid:      uuid,
		name:      name,
		sharedKey: sharedKey,
		handlers:  make(map[callbackSelector]HandleFunc),
		ctx:       ctx,
		cancelCtx: cancel,
		done:      make(chan struct{}),
	}

	re := regexp.MustCompile("^X-Docker-Expose-Session-((?i)[a-z0-9_\\.]+)-Method$")

	for k, allv := range r.Header {
		matches := re.FindAllStringSubmatch(k, -1)
		if len(matches) == 1 && len(matches[0]) == 2 {
			if strings.EqualFold(matches[0][1], t.Name()) {
				for _, v := range allv {
					id, method, err := t.FromMethodName(v)
					if err != nil {
						return err
					}
					sel := callbackSelector{id: id, method: method}
					s.handlers[sel] = func(ctx context.Context, opts map[string][]string, stream *Stream) error {
						s, err := caller.Call(ctx, id, method, opts)
						if err != nil {
							return err
						}
						*stream = s
						return nil
					}
				}
			}
		}
	}

	sm.sessions[uuid] = s
	sm.c.Broadcast()
	sm.mu.Unlock()

	defer func() {
		cancel()
		sm.mu.Lock()
		delete(sm.sessions, uuid)
		sm.mu.Unlock()
	}()

	<-s.ctx.Done()
	conn.Close()
	close(s.done)

	return nil
}

// GetSession returns a session by UUID
func (sm *Manager) GetSession(ctx context.Context, uuid string) (context.Context, Caller, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		select {
		case <-ctx.Done():
			sm.c.Broadcast()
		}
	}()

	var session *Session

	sm.mu.Lock()
	for {
		select {
		case <-ctx.Done():
			sm.mu.Unlock()
			return nil, nil, errors.Wrapf(ctx.Err(), "no active session for %s", uuid)
		default:
		}
		var ok bool
		session, ok = sm.sessions[uuid]
		if !ok || session.closed() {
			sm.c.Wait()
			continue
		}
		sm.mu.Unlock()
		break
	}

	sc := &sessionCaller{s: session}
	return session.context(), sc, nil
}

type sessionCaller struct {
	s *Session
}

func (sc *sessionCaller) Name() string {
	return sc.s.name
}

func (sc *sessionCaller) SharedKey() string {
	return sc.s.sharedKey
}

func (sc *sessionCaller) Supports(id, method string) bool {
	if sc.s.closed() {
		return false
	}
	_, ok := sc.s.handlers[callbackSelector{id: id, method: method}]
	return ok
}

func (sc *sessionCaller) Call(ctx context.Context, id, method string, opts map[string][]string) (Stream, error) {
	if !sc.Supports(id, method) {
		return nil, errors.Errorf("method %s not supported on %s", method, id)
	}

	ctx, cancel := context.WithCancel(ctx)
	go func() {
		<-sc.s.context().Done()
		cancel()
	}()

	var stream Stream
	err := sc.s.handlers[callbackSelector{id: id, method: method}](ctx, opts, &stream)
	if err != nil {
		return nil, err
	}
	if stream == nil {
		return nil, errors.Errorf("invalid stream response")
	}
	return stream, nil
}

func (sc *sessionCaller) Close() error {
	// TODO: test this
	return sc.s.Close()
}
