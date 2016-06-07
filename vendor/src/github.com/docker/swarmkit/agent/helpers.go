package agent

import "golang.org/x/net/context"

// runctx blocks until the function exits, closed is closed, or the context is
// cancelled. Call as part os go statement.
func runctx(ctx context.Context, closed chan struct{}, errs chan error, fn func(ctx context.Context) error) {
	select {
	case errs <- fn(ctx):
	case <-closed:
	case <-ctx.Done():
	}
}
