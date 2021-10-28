package session

import (
	"context"
	"net"
	"strings"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/moby/buildkit/identity"
	"github.com/moby/buildkit/util/grpcerrors"
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
	id         string
	name       string
	sharedKey  string
	ctx        context.Context
	cancelCtx  func()
	done       chan struct{}
	grpcServer *grpc.Server
	conn       net.Conn
}

// NewSession returns a new long running session
func NewSession(ctx context.Context, name, sharedKey string) (*Session, error) {
	id := identity.NewID()

	var unary []grpc.UnaryServerInterceptor
	var stream []grpc.StreamServerInterceptor

	serverOpts := []grpc.ServerOption{}

	if span := trace.SpanFromContext(ctx); span.SpanContext().IsValid() {
		unary = append(unary, filterServer(otelgrpc.UnaryServerInterceptor(otelgrpc.WithTracerProvider(span.TracerProvider()), otelgrpc.WithPropagators(propagators))))
		stream = append(stream, otelgrpc.StreamServerInterceptor(otelgrpc.WithTracerProvider(span.TracerProvider()), otelgrpc.WithPropagators(propagators)))
	}

	unary = append(unary, grpcerrors.UnaryServerInterceptor)
	stream = append(stream, grpcerrors.StreamServerInterceptor)

	if len(unary) == 1 {
		serverOpts = append(serverOpts, grpc.UnaryInterceptor(unary[0]))
	} else if len(unary) > 1 {
		serverOpts = append(serverOpts, grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(unary...)))
	}

	if len(stream) == 1 {
		serverOpts = append(serverOpts, grpc.StreamInterceptor(stream[0]))
	} else if len(stream) > 1 {
		serverOpts = append(serverOpts, grpc.StreamInterceptor(grpc_middleware.ChainStreamServer(stream...)))
	}

	s := &Session{
		id:         id,
		name:       name,
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
	s.conn = conn
	serve(ctx, s.grpcServer, conn)
	return nil
}

// Close closes the session
func (s *Session) Close() error {
	if s.cancelCtx != nil && s.done != nil {
		if s.conn != nil {
			s.conn.Close()
		}
		s.grpcServer.Stop()
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

// updates needed in opentelemetry-contrib to avoid this
func filterServer(intercept grpc.UnaryServerInterceptor) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		if strings.HasSuffix(info.FullMethod, "Health/Check") {
			return handler(ctx, req)
		}
		return intercept(ctx, req, info, handler)
	}
}

func filterClient(intercept grpc.UnaryClientInterceptor) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		if strings.HasSuffix(method, "Health/Check") {
			return invoker(ctx, method, req, reply, cc, opts...)
		}
		return intercept(ctx, method, req, reply, cc, invoker, opts...)
	}
}
