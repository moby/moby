// Command exthook is a minimal out-of-process extension used by the launcher
// end-to-end test. It provides the echo test point and echoes the request back
// (erroring on an empty message), so the test can assert both the success and
// veto paths across the real stdio handshake and gRPC connection.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/moby/moby/v2/internal/extensions"
	echov1 "github.com/moby/moby/v2/internal/extensions/internal/launcher/echo/v1"
	echopb "github.com/moby/moby/v2/internal/extensions/internal/launcher/echo/v1/protogen"
	"github.com/moby/moby/v2/internal/extensions/sdk"
)

type echo struct{}

func (echo) Echo(_ context.Context, req *echov1.EchoRequest) (*echov1.EchoResponse, error) {
	if req.Message == "" {
		return nil, errors.New("message must not be empty")
	}
	return &echov1.EchoResponse{Message: req.Message}, nil
}

func main() {
	// On unix the launcher stops extensions with SIGTERM; handling it cancels
	// ctx so the SDK shuts down gracefully and the process exits zero. On
	// Windows the launcher kills the process instead.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM)
	defer stop()

	ext := extensions.New(extensions.Declaration{
		ID:        "org.example.exthook.v1",
		Providers: []extensions.Provider{echov1.Point.Provide(echo{})},
	})
	srv := sdk.NewServer()
	if err := srv.Register(ext, echopb.ServerPoint); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := srv.Listen(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
