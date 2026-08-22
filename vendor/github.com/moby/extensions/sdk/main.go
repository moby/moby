package sdk

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/moby/extensions"
	"github.com/moby/extensions/serverpoint"
)

// Main runs ext as a standalone out-of-process extension.
//
//	func main() {
//		sdk.Main(
//			myext.Extension,
//			sdk.WithServerPoints(createspecpb.ServerPoint),
//		)
//	}
//
// Use [WithServerPoints] to pass the generated ServerPoint registration for
// every ordinary Point provider.
// The service metadata Point needs no transport registration.
//
// A binary that needs [Server.Depends] or other server setup should build a
// [Server] directly.
func Main(ext extensions.Extension, options ...MainOption) {
	if err := serve(ext, options); err != nil {
		// stdout is reserved for the runtime handshake; the host captures stderr
		// and folds it into its own logs.
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// MainOption configures [Main].
type MainOption interface {
	apply(*mainOptions)
}

type mainOptionFunc func(*mainOptions)

func (f mainOptionFunc) apply(options *mainOptions) {
	f(options)
}

type mainOptions struct {
	points []serverpoint.Registration
}

// WithServerPoints supplies the generated server registration for each ordinary
// Point provided by the extension.
func WithServerPoints(points ...serverpoint.Registration) MainOption {
	points = append([]serverpoint.Registration(nil), points...)
	return mainOptionFunc(func(options *mainOptions) {
		options.points = append(options.points, points...)
	})
}

// serve is Main's body, split out so the signal handler is unregistered on the
// way out rather than skipped by os.Exit.
func serve(ext extensions.Extension, options []MainOption) error {
	// The host stops an extension with SIGTERM on unix; cancelling ctx on it
	// lets the SDK shut the extension down gracefully and exit zero. On Windows
	// the host kills the process instead, so nothing is delivered there.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var config mainOptions
	for _, option := range options {
		if option == nil {
			return errors.New("nil main option")
		}
		option.apply(&config)
	}

	srv := NewServer()
	if err := srv.Register(ext, config.points...); err != nil {
		return err
	}
	return srv.Listen(ctx)
}
