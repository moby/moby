// Package greeter is the fixture for the socket-exposure integration test: an
// extension that serves the example greeter gRPC service and opts into having it
// published on the daemon API socket via the service.grpc point. The test then
// calls the greeter over docker.sock with a plain gRPC client.
//
// The greeter service is not an extension point: the extension implements the
// service.grpc point and registers the greeter gRPC service through it. The
// framework serves it -- on the daemon's own server if this extension runs
// in-process, or on the extension's server (proxied by the daemon) out of
// process -- so the same code works either way.
package greeter

import (
	"context"

	servicegrpcv0 "github.com/moby/moby/v2/extpoints/servicegrpc/v0"
	"github.com/moby/moby/v2/internal/extensions"
	greeterv0 "github.com/moby/moby/v2/internal/extensions/example/greeter/v0"
	greeterpb "github.com/moby/moby/v2/internal/extensions/example/greeter/v0/protogen"
	"google.golang.org/grpc"
)

// ID is the extension id; the binary is named after it.
const ID = "org.mobyproject.example.greeter.v1"

type greeter struct{}

func (greeter) Greet(_ context.Context, req *greeterv0.HelloRequest) (*greeterv0.HelloReply, error) {
	return &greeterv0.HelloReply{Message: "hello " + req.Name}, nil
}

// expose implements the service.grpc point: it registers the greeter gRPC
// service under the point the daemon publishes on its API socket.
type expose struct{}

func (expose) RegisterServices(r grpc.ServiceRegistrar) {
	greeterpb.ServerPoint.Register(r, greeter{})
}

// Extension implements only the service.grpc point, so the daemon treats the
// services registered by this provider as socket-exposed services.
var Extension = extensions.New(extensions.Declaration{
	ID:        ID,
	Providers: []extensions.Provider{servicegrpcv0.Point.Provide(expose{})},
})
