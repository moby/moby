package appcontext

import (
	"context"
	"os"
	"os/signal"
	"sync"

	"github.com/moby/buildkit/util/bklog"
	"github.com/pkg/errors"
)

// Context returns a static context that reacts to termination signals of the
// running process. Useful in CLI tools.
func Context() context.Context {
	initContexts()
	return appContext
}

// Shutdown returns a static context that closes when multiple interrupt signals
// have been received to indicate a faster shutdown. Useful in CLI tools.
func Shutdown() context.Context {
	initContexts()
	return shutdownContext
}

var (
	appContext       context.Context
	shutdownContext  context.Context
	initContextsOnce sync.Once
)

func initContexts() {
	initContextsOnce.Do(func() {
		signals := make(chan os.Signal, 2048)
		signal.Notify(signals, terminationSignals...)

		ctx := context.Background()
		for _, f := range inits {
			ctx = f(ctx) //nolint:fatcontext
		}

		ctx, cancel := context.WithCancelCause(ctx)
		appContext = ctx //nolint:fatcontext

		shutdownCtx, shutdownCancel := context.WithCancelCause(context.Background())
		shutdownContext = shutdownCtx

		// We just allow this goroutine to be orphaned since program termination
		// will clean it up.
		go func() {
			<-signals
			err := errors.New("got SIGTERM/SIGINT, forcing shutdown")
			cancel(err)

			<-signals
			err = errors.New("got 2 SIGTERM/SIGINTs, skipping shutdown")
			shutdownCancel(err)

			<-signals
			err = errors.New("got 3 SIGTERM/SIGINTs, forcibly terminating")
			bklog.G(ctx).Fatal(err.Error())
		}()
	})
}
