// Package greeter is the socket-exposure integration-test fixture.
package greeter

import (
	"context"

	servicegrpcv0 "github.com/moby/moby/v2/extpoints/servicegrpc/v0"
	"github.com/moby/moby/v2/internal/extensions"
	greeterv0 "github.com/moby/moby/v2/internal/extensions/example/greeter/v0"
	greeterpb "github.com/moby/moby/v2/internal/extensions/example/greeter/v0/protogen"
	"google.golang.org/grpc"
)

// ID is the extension id and binary name.
const ID = "org.mobyproject.example.greeter.v1"

type greeter struct{}

func (greeter) Greet(_ context.Context, req *greeterv0.HelloRequest) (*greeterv0.HelloReply, error) {
	return &greeterv0.HelloReply{Message: "hello " + req.Name}, nil
}

// expose registers the greeter gRPC service for socket exposure.
type expose struct{}

func (expose) RegisterServices(r grpc.ServiceRegistrar) {
	greeterpb.ServerPoint.Register(r, greeter{})
}

// Extension implements the service.grpc point.
var Extension = extensions.New(extensions.Declaration{
	ID:        ID,
	Providers: []extensions.Provider{servicegrpcv0.Point.Provide(expose{})},
})
