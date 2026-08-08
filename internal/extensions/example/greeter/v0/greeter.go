// Package greeterv0 is an example service point: a plain gRPC service an
// extension can implement and publish on the daemon's API socket via the
// service.grpc point. It exists to exercise socket exposure end to end -- it is
// not a real engine point.
package greeterv0

import (
	"context"

	"github.com/moby/moby/v2/internal/extensions"
)

// Greeter is the example service. An extension implements it; with socket
// exposure, external clients call it on docker.sock exactly as the engine would
// in-daemon.
type Greeter interface {
	Greet(ctx context.Context, req *HelloRequest) (*HelloReply, error)
}

// HelloRequest is the greeting request.
type HelloRequest struct {
	Name string `pb:"1"`
}

// HelloReply is the greeting response.
type HelloReply struct {
	Message string `pb:"1"`
}

// Point is the example greeter service point.
var Point = extensions.DefinePoint[Greeter]("org.mobyproject.extension.example.greeter.v0")

// Greet calls the single greeter provider.
func Greet(ctx context.Context, resolver extensions.Resolver, req *HelloRequest) (*HelloReply, error) {
	g, err := Point.Single(resolver)
	if err != nil {
		return nil, err
	}
	return g.Greet(ctx, req)
}
