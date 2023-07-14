package grpc // import "github.com/docker/docker/api/server/router/grpc"

import (
	"context"
	"strings"

	"github.com/docker/docker/api/server/router"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/moby/buildkit/util/grpcerrors"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"golang.org/x/net/http2"
	"google.golang.org/grpc"
)

type grpcRouter struct {
	routes     []router.Route
	grpcServer *grpc.Server
	h2Server   *http2.Server
}

// NewRouter initializes a new grpc http router
func NewRouter(backends ...Backend) router.Router {
	unary := grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(unaryInterceptor(), grpcerrors.UnaryServerInterceptor))
	stream := grpc.StreamInterceptor(grpc_middleware.ChainStreamServer(otelgrpc.StreamServerInterceptor(), grpcerrors.StreamServerInterceptor))

	r := &grpcRouter{
		h2Server:   &http2.Server{},
		grpcServer: grpc.NewServer(unary, stream),
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

func unaryInterceptor() grpc.UnaryServerInterceptor {
	withTrace := otelgrpc.UnaryServerInterceptor()

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
