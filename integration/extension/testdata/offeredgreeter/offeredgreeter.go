// Package offeredgreeter is the offered-point integration-test fixture.
package offeredgreeter

import (
	"context"

	"github.com/moby/extensions"
	greeterv0 "github.com/moby/extensions/example/greeter/v0"
	greeterpb "github.com/moby/extensions/example/greeter/v0/protogen"
	servicev0 "github.com/moby/extensions/extpoints/service/v0"
	"github.com/moby/extensions/serverpoint"
)

// ID is the extension id and binary name.
const ID = "org.mobyproject.example.offeredgreeter.v1"

type greeter struct{}

func (greeter) Greet(_ context.Context, req *greeterv0.HelloRequest) (*greeterv0.HelloReply, error) {
	return &greeterv0.HelloReply{Message: "hello " + req.Name}, nil
}

// The point ID matches the private SDK runtime package, but its only service
// is the greeter in a different package.
var point = extensions.DefinePoint[greeterv0.Greeter]("moby.extension.runtime.v1")

var ServerPoint = serverpoint.Registration{Point: point.ID(), Register: greeterpb.ServerPoint.Register}

// Extension offers the greeter for publication.
var Extension = extensions.New(extensions.Declaration{
	ID: ID,
	Providers: []extensions.Provider{
		point.Provide(greeter{}),
		servicev0.Offer(point),
	},
})
