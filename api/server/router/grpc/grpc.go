package grpc // import "github.com/docker/docker/api/server/router/grpc"

import (
	"github.com/docker/docker/api/server/router"
	"github.com/moby/buildkit/util/grpcerrors"
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
	r := &grpcRouter{
		h2Server: &http2.Server{},
		grpcServer: grpc.NewServer(
			grpc.UnaryInterceptor(grpcerrors.UnaryServerInterceptor),
			grpc.StreamInterceptor(grpcerrors.StreamServerInterceptor),
		),
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
