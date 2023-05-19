package grpc // import "github.com/docker/docker/api/server/router/grpc"

import (
	"context"
	"strings"

	"github.com/docker/docker/api/server/router"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/moby/buildkit/util/grpcerrors"
	"github.com/moby/buildkit/util/tracing/detect"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/net/http2"
	"google.golang.org/grpc"
)

func init() {
	// enable in memory recording for grpc traces
	detect.Recorder = detect.NewTraceRecorder()
}

type grpcRouter struct {
	routes     []router.Route
	grpcServer *grpc.Server
	h2Server   *http2.Server
}

var propagators = propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{})

// NewRouter initializes a new grpc http router
func NewRouter(backends ...Backend) router.Router {
	tp, err := detect.TracerProvider()
	if err != nil {
		logrus.WithError(err).Error("failed to detect trace provider")
	}

	opts := []grpc.ServerOption{grpc.UnaryInterceptor(grpcerrors.UnaryServerInterceptor), grpc.StreamInterceptor(grpcerrors.StreamServerInterceptor)}
	if tp != nil {
		streamTracer := otelgrpc.StreamServerInterceptor(otelgrpc.WithTracerProvider(tp), otelgrpc.WithPropagators(propagators))
		unary := grpc_middleware.ChainUnaryServer(unaryInterceptor(tp), grpcerrors.UnaryServerInterceptor)
		stream := grpc_middleware.ChainStreamServer(streamTracer, grpcerrors.StreamServerInterceptor)
		opts = []grpc.ServerOption{grpc.UnaryInterceptor(unary), grpc.StreamInterceptor(stream)}
	}

	r := &grpcRouter{
		h2Server:   &http2.Server{},
		grpcServer: grpc.NewServer(opts...),
	}
	for _, b := range backends {
		b.RegisterGRPC(r.grpcServer)
	}
	r.initRoutes()
	return r
}

// Routes returns the available routers to the session controller
func (gr *grpcRouter) Routes() []router.Route {
	return gr.routes
}

func (gr *grpcRouter) initRoutes() {
	gr.routes = []router.Route{
		router.NewPostRoute("/grpc", gr.serveGRPC),
	}
}

func unaryInterceptor(tp trace.TracerProvider) grpc.UnaryServerInterceptor {
	withTrace := otelgrpc.UnaryServerInterceptor(otelgrpc.WithTracerProvider(tp), otelgrpc.WithPropagators(propagators))

	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		// This method is used by the clients to send their traces to buildkit so they can be included
		// in the daemon trace and stored in the build history record. This method can not be traced because
		// it would cause an infinite loop.
		if strings.HasSuffix(info.FullMethod, "opentelemetry.proto.collector.trace.v1.TraceService/Export") {
			return handler(ctx, req)
		}
		return withTrace(ctx, req, info, handler)
	}
}
