// Command nri serves the NRI bridge as an out-of-process Moby extension. It is
// the same [nri.Extension] the daemon can also register in-process -- only the
// packaging differs, so NRI is location agnostic. The SDK runs the extension's
// lifecycle, so registering it is enough: the bridge's adaptation starts in
// Init and stops in Shutdown.
//
// Its config is delivered over the startup handshake, keyed by id, just as an
// in-process extension receives it -- so the daemon can configure it the same
// way either way. With no config it self-configures from the default NRI plugin
// locations.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/moby/moby/v2/daemon/extproviders/nri"
	createspecpb "github.com/moby/moby/v2/extpoints/createspec/v0/protogen"
	"github.com/moby/moby/v2/internal/extensions/sdk"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	srv := sdk.NewServer()
	if err := srv.Register(nri.Extension, createspecpb.ServerPoint); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := srv.Listen(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
