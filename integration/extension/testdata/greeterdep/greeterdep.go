// Package greeterdep is a fixture for the out-of-process dependency test: an
// extension that declares a dependency on the greeter point and calls it during
// its own Init. It runs out of process, so the call goes back to the daemon over
// the callback channel and is routed to the real greeter provider.
package greeterdep

import (
	"context"
	"fmt"

	"github.com/moby/moby/v2/internal/extensions"
	greeterv0 "github.com/moby/moby/v2/internal/extensions/example/greeter/v0"
)

// ID is the extension id; the binary is named after it.
const ID = "org.mobyproject.example.greeterdep.v1"

// initialize calls the greeter dependency and verifies the reply, so a
// successful init proves the cross-process dependency call round-tripped.
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

// Extension declares a point dependency on the greeter and uses it at Init.
var Extension = extensions.New(extensions.Declaration{
	ID:           ID,
	Dependencies: []extensions.Dependency{greeterv0.Point.Dependency()},
	Init:         initialize,
})
