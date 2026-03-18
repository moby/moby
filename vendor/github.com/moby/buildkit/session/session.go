package session

import (
	"context"
	"net"
	"sync"

	"github.com/moby/buildkit/identity"
	"github.com/moby/buildkit/util/grpcerrors"
	"github.com/moby/buildkit/util/tracing"
	"github.com/pkg/errors"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
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

var propagators = propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{})

// Dialer returns a connection that can be used by the session
type Dialer func(ctx context.Context, proto string, meta map[string][]string) (net.Conn, error)

// Attachable defines a feature that can be exposed on a session
type Attachable interface {
	Register(*grpc.Server)
}

// Session is a long running connection between client and a daemon
type Session struct {
	mu          sync.Mutex // synchronizes conn run and close
	id          string
	sharedKey   string
	ctx         context.Context
	cancelCtx   func(error)
	done        chan struct{}
	grpcServer  *grpc.Server
	conn        net.Conn
	closeCalled bool
}

// NewSession returns a new long running session
func NewSession(ctx context.Context, sharedKey string) (*Session, error) {
	id := identity.NewID()

	serverOpts := []grpc.ServerOption{
		grpc.UnaryInterceptor(grpcerrors.UnaryServerInterceptor),
		grpc.StreamInterceptor(grpcerrors.StreamServerInterceptor),
	}

	if span := trace.SpanFromContext(ctx); span.SpanContext().IsValid() {
		statsHandler := tracing.ServerStatsHandler(
			otelgrpc.WithTracerProvider(span.TracerProvider()),
			otelgrpc.WithPropagators(propagators),
		)
		serverOpts = append(serverOpts, grpc.StatsHandler(statsHandler))
	}

	s := &Session{
		id:         id,
		sharedKey:  sharedKey,
		grpcServer: grpc.NewServer(serverOpts...),
	}

	grpc_health_v1.RegisterHealthServer(s.grpcServer, health.NewServer())

	return s, nil
}

// Allow enables a given service to be reachable through the grpc session
func (s *Session) Allow(a Attachable) {
	a.Register(s.grpcServer)
}

// ID returns unique identifier for the session
func (s *Session) ID() string {
	return s.id
}

// Run activates the session
func (s *Session) Run(ctx context.Context, dialer Dialer) error {
	s.mu.Lock()
	if s.closeCalled {
		s.mu.Unlock()
		return nil
	}
	ctx, cancel := context.WithCancelCause(ctx)
	s.cancelCtx = cancel
	s.done = make(chan struct{})

	defer func() { cancel(errors.WithStack(context.Canceled)) }()
	defer close(s.done)

	meta := make(map[string][]string)
	meta[headerSessionID] = []string{s.id}
	meta[headerSessionSharedKey] = []string{s.sharedKey}

	for name, svc := range s.grpcServer.GetServiceInfo() {
		for _, method := range svc.Methods {
			meta[headerSessionMethod] = append(meta[headerSessionMethod], MethodURL(name, method.Name))
		}
	}
	conn, err := dialer(ctx, "h2c", meta)
	if err != nil {
		s.mu.Unlock()
		return errors.Wrap(err, "failed to dial gRPC")
	}
	s.conn = conn
	s.mu.Unlock()
	serve(ctx, s.grpcServer, conn)
	return nil
}

// Close closes the session
func (s *Session) Close() error {
	s.mu.Lock()
	if s.cancelCtx != nil && s.done != nil {
		if s.conn != nil {
			s.conn.Close()
		}
		s.grpcServer.Stop()
		<-s.done
	}
	s.closeCalled = true
	s.mu.Unlock()
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
