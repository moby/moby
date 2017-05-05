package session

import (
	"fmt"
	"net"
	"net/http"
	"sync"

	"google.golang.org/grpc"

	"strings"

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
	ListStreamInstances(serviceName string) []string
	SupportsStream(serviceName, instanceId, method string) bool
	ConnectToStream(ctx context.Context, serviceName, instanceId, method string, handler MessageStreamHandler) error
}

type MessageStream interface {
	Context() context.Context
	SendMsg(m interface{}) error
	RecvMsg(m interface{}) error
}

type MessageStreamHandler func(MessageStream) error

// Session is a long running connection between client and a daemon
type Session struct {
	uuid           string
	name           string
	sharedKey      string
	ctx            context.Context
	cancelCtx      func()
	done           chan struct{}
	streamServices map[string]*streamServiceDesc
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
type streamServiceDesc struct {
	instances map[string]*streamInstanceDesc
}
type streamInstanceDesc struct {
	methods map[string]MessageStreamHandler
}

// NewSession returns a new long running session
func NewServerSession(name, sharedKey string) (*ServerSession, error) {
	uuid := stringid.GenerateRandomID()
	s := &ServerSession{
		Session: Session{
			uuid:           uuid,
			name:           name,
			sharedKey:      sharedKey,
			streamServices: make(map[string]*streamServiceDesc),
		},
		grpcServer: grpc.NewServer(),
	}
	return s, nil
}

// Allow enable a given service to be reachable trough the grpc session
func (s *ServerSession) Allow(serviceDesc *grpc.ServiceDesc, serviceImpl interface{}) {
	s.grpcServer.RegisterService(serviceDesc, serviceImpl)
}

func (s *ServerSession) AllowStream(serviceName, instanceId, method string, handler MessageStreamHandler) error {
	serviceName = strings.ToLower(serviceName)
	instanceId = strings.ToLower(instanceId)
	method = strings.ToLower(method)
	if strings.Contains(serviceName, ".") {
		return errors.New("serviceName should not contain character'.'")
	}
	svc, ok := s.streamServices[serviceName]
	if !ok {
		svc = &streamServiceDesc{instances: make(map[string]*streamInstanceDesc)}
		s.streamServices[serviceName] = svc
	}
	instance, ok := svc.instances[instanceId]
	if !ok {
		instance = &streamInstanceDesc{methods: make(map[string]MessageStreamHandler)}
		svc.instances[instanceId] = instance
	}
	instance.methods[method] = handler
	return nil
}

// UUID returns unique identifier for the session
func (s *Session) UUID() string {
	return s.uuid
}

func grpcServiceName(serviceName, instanceId string) string {
	return serviceName + "." + instanceId
}
func serviceNameAndInstance(grpcServiceName string) (serviceName, instanceId string) {
	parts := strings.SplitN(grpcServiceName, ".", 2)
	if len(parts) >= 1 {
		serviceName = parts[0]
	}
	if len(parts) >= 2 {
		instanceId = parts[1]
	}
	return
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

	// register low level streams
	for svcName, svcDesc := range s.streamServices {
		meta["X-Docker-Expose-Session-Streamservices"] = append(meta["X-Docker-Expose-Session-Streamservices"], svcName)
		svcInstancesHeader := "X-Docker-Expose-Session-Streaminstances-" + headerize(svcName)
		for instanceId, instanceDesc := range svcDesc.instances {
			grpcSvcDesc := grpc.ServiceDesc{
				ServiceName: grpcServiceName(svcName, instanceId),
				HandlerType: (*interface{})(nil),
				Methods:     make([]grpc.MethodDesc, 0, 0),
			}
			meta[svcInstancesHeader] = append(meta[svcInstancesHeader], instanceId)
			methodsHeader := "X-Docker-Expose-Session-Streammethods-" + headerize(grpcServiceName(svcName, instanceId))
			for methName, methImpl := range instanceDesc.methods {
				meta[methodsHeader] = append(meta[methodsHeader], methName)
				grpcSvcDesc.Streams = append(grpcSvcDesc.Streams,
					grpc.StreamDesc{
						ClientStreams: true,
						ServerStreams: true,
						StreamName:    methName,
						Handler: func(srv interface{}, stream grpc.ServerStream) error {
							return methImpl(stream)
						},
					})
			}
			s.grpcServer.RegisterService(&grpcSvcDesc, (*interface{})(nil))
		}
	}

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
			uuid:           uuid,
			name:           name,
			sharedKey:      sharedKey,
			ctx:            ctx,
			cancelCtx:      cancel,
			done:           make(chan struct{}),
			streamServices: make(map[string]*streamServiceDesc),
		},
		caller:            caller,
		supportedServices: make(map[string]struct{}),
	}

	serviceNames := r.Header["X-Docker-Expose-Session-Services"]
	for _, svcName := range serviceNames {
		s.supportedServices[svcName] = struct{}{}
	}

	streamServiceNames := r.Header["X-Docker-Expose-Session-Streamservices"]
	for _, svcName := range streamServiceNames {
		svc := &streamServiceDesc{instances: make(map[string]*streamInstanceDesc)}
		s.streamServices[svcName] = svc
		instanceNames := r.Header["X-Docker-Expose-Session-Streaminstances-"+headerize(svcName)]
		for _, instanceId := range instanceNames {
			instance := &streamInstanceDesc{methods: make(map[string]MessageStreamHandler)}
			svc.instances[instanceId] = instance
			methNames := r.Header["X-Docker-Expose-Session-Streammethods-"+headerize(grpcServiceName(svcName, instanceId))]
			for _, methName := range methNames {
				instance.methods[methName] = nil
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

func headerize(value string) string {
	parts := strings.Split(value, "-")
	for i := 0; i < len(parts); i++ {
		parts[i] = strings.ToUpper(parts[i][:1]) + strings.ToLower(parts[i][1:])
	}
	return strings.Join(parts, "-")
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

func (sc *sessionCaller) ListStreamInstances(serviceName string) []string {
	serviceName = strings.ToLower(serviceName)
	result := []string{}
	if svc, ok := sc.s.streamServices[serviceName]; ok {
		for name, _ := range svc.instances {
			result = append(result, name)
		}
	}
	return result
}

func (sc *sessionCaller) SupportsStream(serviceName, instanceId, method string) bool {
	serviceName = strings.ToLower(serviceName)
	instanceId = strings.ToLower(instanceId)
	method = strings.ToLower(method)
	svc, ok := sc.s.streamServices[serviceName]
	if !ok {
		return false
	}
	instance, ok := svc.instances[instanceId]
	if !ok {
		return false
	}
	_, ok = instance.methods[method]
	return ok
}

func (sc *sessionCaller) ConnectToStream(ctx context.Context, serviceName, instanceId, method string, handler MessageStreamHandler) error {
	serviceName = strings.ToLower(serviceName)
	instanceId = strings.ToLower(instanceId)
	method = strings.ToLower(method)
	url := "/" + grpcServiceName(serviceName, instanceId) + "/" + method
	desc := &grpc.StreamDesc{
		StreamName:    method,
		Handler:       nil,
		ServerStreams: true,
		ClientStreams: true,
	}
	stream, err := grpc.NewClientStream(ctx, desc, sc.GetGrpcConn(), url)
	if err != nil {
		return err
	}
	defer stream.CloseSend()
	return handler(stream)
}
