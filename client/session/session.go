package session

import (
	"fmt"
	"net"
	"net/http"
	"sync"

	"google.golang.org/grpc"

	"github.com/docker/docker/pkg/stringid"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

// Dialer returns a connection that can be used by the session
type Dialer func(ctx context.Context, proto string, meta map[string][]string) (net.Conn, error)

// Caller can invoke requests on the session
type Caller interface {
	Supports(serviceName string) bool
	GetGrpcConn() *grpc.ClientConn
	Name() string
	SharedKey() string
	Close() error
}

// Session is a long running connection between client and a daemon
type Session struct {
	uuid      string
	name      string
	sharedKey string
	ctx       context.Context
	cancelCtx func()
	done      chan struct{}
}
type ClientSession struct {
	Session
	caller            *grpcCaller
	supportedServices map[string]struct{}
}
type ServerSession struct {
	Session
	grpcServer *grpc.Server
}

// NewSession returns a new long running session
func NewServerSession(name, sharedKey string) (*ServerSession, error) {
	uuid := stringid.GenerateRandomID()
	s := &ServerSession{
		Session: Session{
			uuid:      uuid,
			name:      name,
			sharedKey: sharedKey,
		},
		grpcServer: grpc.NewServer(),
	}
	return s, nil
}

// Allow enable a given service to be reachable trough the grpc session
func (s *ServerSession) Allow(serviceDesc *grpc.ServiceDesc, serviceImpl interface{}) {
	s.grpcServer.RegisterService(serviceDesc, serviceImpl)
}

// UUID returns unique identifier for the session
func (s *Session) UUID() string {
	return s.uuid
}

// Run activates the session
func (s *ServerSession) Run(ctx context.Context, dialer Dialer) error {
	ctx, cancel := context.WithCancel(ctx)
	s.cancelCtx = cancel
	s.done = make(chan struct{})

	defer cancel()
	defer close(s.done)

	meta := make(map[string][]string)
	meta["X-Docker-Expose-Session-UUID"] = []string{s.uuid}
	meta["X-Docker-Expose-Session-Name"] = []string{s.name}
	meta["X-Docker-Expose-Session-SharedKey"] = []string{s.sharedKey}

	serviceNames := []string{}
	for svc := range s.grpcServer.GetServiceInfo() {
		serviceNames = append(serviceNames, svc)
	}
	meta["X-Docker-Expose-Session-Services"] = serviceNames

	conn, err := dialer(ctx, "gRPC", meta)
	if err != nil {
		return errors.Wrap(err, "failed to dial gRPC")
	}
	serve(ctx, s.grpcServer, conn)
	return nil
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
	sessions map[string]*ClientSession
	mu       sync.Mutex
	c        *sync.Cond
}

// NewManager returns a new Manager
func NewManager() (*Manager, error) {
	sm := &Manager{
		sessions: make(map[string]*ClientSession),
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

	if proto != "gRPC" {
		return errors.Errorf("protocol %s not supported", proto)
	}

	conn, _, err := hijacker.Hijack()
	if err != nil {
		return errors.Wrap(err, "failed to hijack connection")
	}

	// set raw mode
	conn.Write([]byte{})

	fmt.Fprintf(conn, "HTTP/1.1 101 Switching Protocols\r\nConnection: Upgrade\r\nUpgrade: %s\r\n\r\n", proto)

	ctx, cancel := context.WithCancel(ctx)

	caller, err := newCaller(ctx, conn)
	if err != nil {
		return errors.Wrap(err, "failed to create new caller")
	}

	s := &ClientSession{
		Session: Session{
			uuid:      uuid,
			name:      name,
			sharedKey: sharedKey,
			ctx:       ctx,
			cancelCtx: cancel,
			done:      make(chan struct{}),
		},
		caller:            caller,
		supportedServices: make(map[string]struct{}),
	}

	serviceNames := r.Header["X-Docker-Expose-Session-Services"]
	for _, svcName := range serviceNames {
		s.supportedServices[svcName] = struct{}{}
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

	var session *ClientSession

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
	s *ClientSession
}

func (sc *sessionCaller) Name() string {
	return sc.s.name
}

func (sc *sessionCaller) SharedKey() string {
	return sc.s.sharedKey
}

func (sc *sessionCaller) Close() error {
	// TODO: test this
	return sc.s.Close()
}

func (sc *sessionCaller) Supports(serviceName string) bool {
	_, ok := sc.s.supportedServices[serviceName]
	return ok
}
func (sc *sessionCaller) GetGrpcConn() *grpc.ClientConn {
	return sc.s.caller.cc
}
