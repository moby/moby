package appcontext

import (
	"context"
	"os"
	"os/signal"
	"sync"

	"github.com/moby/buildkit/util/bklog"
	"github.com/pkg/errors"
)

var appContextCache context.Context
var appContextOnce sync.Once

// Context returns a static context that reacts to termination signals of the
// running process. Useful in CLI tools.
func Context() context.Context {
	appContextOnce.Do(func() {
		signals := make(chan os.Signal, 2048)
		signal.Notify(signals, terminationSignals...)

		const exitLimit = 3
		retries := 0

		ctx := context.Background()
		for _, f := range inits {
			ctx = f(ctx) //nolint:fatcontext
		}

		ctx, cancel := context.WithCancelCause(ctx)
		appContextCache = ctx //nolint:fatcontext

		go func() {
			for {
				<-signals
				retries++
				err := errors.Errorf("got %d SIGTERM/SIGINTs, forcing shutdown", retries)
				cancel(err)
				if retries >= exitLimit {
					bklog.G(ctx).Error(err.Error())
					os.Exit(1)
				}
			}
		}()
	})
	return appContextCache
}
