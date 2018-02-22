package session

import (
	"net"

	"github.com/docker/docker/pkg/stringid"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

const (
	headerSessionID        = "X-Docker-Expose-Session-Uuid"
	headerSessionName      = "X-Docker-Expose-Session-Name"
	headerSessionSharedKey = "X-Docker-Expose-Session-Sharedkey"
	headerSessionMethod    = "X-Docker-Expose-Session-Grpc-Method"
)

// Dialer returns a connection that can be used by the session
type Dialer func(ctx context.Context, proto string, meta map[string][]string) (net.Conn, error)

// Attachable defines a feature that can be expsed on a session
type Attachable interface {
	Register(*grpc.Server)
}

// Session is a long running connection between client and a daemon
type Session struct {
	id         string
	name       string
	sharedKey  string
	ctx        context.Context
	cancelCtx  func()
	done       chan struct{}
	grpcServer *grpc.Server
}

// NewSession returns a new long running session
func NewSession(name, sharedKey string) (*Session, error) {
	id := stringid.GenerateRandomID()
	s := &Session{
		id:         id,
		name:       name,
		sharedKey:  sharedKey,
		grpcServer: grpc.NewServer(),
	}

	grpc_health_v1.RegisterHealthServer(s.grpcServer, health.NewServer())

	return s, nil
}

// Allow enable a given service to be reachable through the grpc session
func (s *Session) Allow(a Attachable) {
	a.Register(s.grpcServer)
}

// ID returns unique identifier for the session
func (s *Session) ID() string {
	return s.id
}

// Run activates the session
func (s *Session) Run(ctx context.Context, dialer Dialer) error {
	ctx, cancel := context.WithCancel(ctx)
	s.cancelCtx = cancel
	s.done = make(chan struct{})

	defer cancel()
	defer close(s.done)

	meta := make(map[string][]string)
	meta[headerSessionID] = []string{s.id}
	meta[headerSessionName] = []string{s.name}
	meta[headerSessionSharedKey] = []string{s.sharedKey}

	for name, svc := range s.grpcServer.GetServiceInfo() {
		for _, method := range svc.Methods {
			meta[headerSessionMethod] = append(meta[headerSessionMethod], MethodURL(name, method.Name))
		}
	}
	conn, err := dialer(ctx, "h2c", meta)
	if err != nil {
		return errors.Wrap(err, "failed to dial gRPC")
	}
	serve(ctx, s.grpcServer, conn)
	return nil
}

// Close closes the session
func (s *Session) Close() error {
	if s.cancelCtx != nil && s.done != nil {
		s.grpcServer.Stop()
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

// MethodURL returns a gRPC method URL for service and method name
func MethodURL(s, m string) string {
	return "/" + s + "/" + m
}
