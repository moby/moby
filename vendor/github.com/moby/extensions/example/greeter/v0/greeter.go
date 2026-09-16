//go:generate go run github.com/moby/extensions/cmd/mobyextgen

// Package greeterv0 is an example service point used to exercise socket
// exposure.
package greeterv0

import (
	"context"

	"github.com/moby/extensions"
)

// Greeter is the example service.
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

var Point = extensions.DefinePoint[Greeter]("org.mobyproject.extension.example.greeter.v0")

// Greet calls the greeter provider.
func Greet(ctx context.Context, resolver extensions.Resolver, req *HelloRequest) (*HelloReply, error) {
	g, err := Point.Single(resolver)
	if err != nil {
		return nil, err
	}
	return g.Greet(ctx, req)
}
