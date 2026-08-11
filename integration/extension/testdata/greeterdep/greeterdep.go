// Package greeterdep is the out-of-process dependency-test fixture.
package greeterdep

import (
	"context"
	"fmt"

	"github.com/moby/moby/v2/internal/extensions"
	greeterv0 "github.com/moby/moby/v2/internal/extensions/example/greeter/v0"
)

// ID is the extension id and binary name.
const ID = "org.mobyproject.example.greeterdep.v1"

// initialize calls the greeter dependency and verifies the reply.
func initialize(ctx context.Context, _ extensions.Config, r extensions.Resolver) error {
	reply, err := greeterv0.Greet(ctx, r, &greeterv0.HelloRequest{Name: "dep"})
	if err != nil {
		return fmt.Errorf("greeterdep: call greeter dependency: %w", err)
	}
	if reply.Message != "hello dep" {
		return fmt.Errorf("greeterdep: unexpected greeter reply %q", reply.Message)
	}
	return nil
}

// Extension declares and uses a dependency on the greeter point.
var Extension = extensions.New(extensions.Declaration{
	ID:           ID,
	Dependencies: []extensions.Dependency{greeterv0.Point.Dependency()},
	Init:         initialize,
})
