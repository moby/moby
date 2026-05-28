package session

import "context"

// contextWithCaller returns a context that is canceled when either the request
// context is done or the session context is closed.
func contextWithCaller(ctx context.Context, callerCtx context.Context) context.Context {
	ctx, cancel := context.WithCancelCause(ctx)
	context.AfterFunc(callerCtx, func() {
		cause := context.Cause(callerCtx)
		if cause == nil {
			cause = context.Canceled
		}
		cancel(cause)
	})
	return ctx
}
