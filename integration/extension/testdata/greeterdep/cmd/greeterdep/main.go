// Command greeterdep serves the greeterdep fixture as an out-of-process
// extension that depends on the greeter point and calls it at init.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/moby/moby/v2/integration/extension/testdata/greeterdep"
	greeterpb "github.com/moby/moby/v2/internal/extensions/example/greeter/v0/protogen"
	"github.com/moby/moby/v2/internal/extensions/sdk"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	srv := sdk.NewServer()
	if err := srv.Register(greeterdep.Extension); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	// The greeter point is a dependency: register its client wiring so the
	// resolver Init receives can call it over the callback channel.
	srv.Depends(greeterpb.ClientPoint)
	if err := srv.Listen(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
