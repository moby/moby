package experimental

import (
	"context"

	"github.com/tetratelabs/wazero/internal/expctxkeys"
)

// CloseNotifier is a notification hook, invoked when a module is closed.
//
// Note: This is experimental progress towards #1197, and likely to change. Do
// not expose this in shared libraries as it can cause version locks.
type CloseNotifier interface {
	// CloseNotify is a notification that occurs *before* an api.Module is
	// closed. `exitCode` is zero on success or in the case there was no exit
	// code.
	//
	// Notes:
	//   - This does not return an error because the module will be closed
	//     unconditionally.
	//   - Do not panic from this function as it doing so could cause resource
	//     leaks.
	//   - While this is only called once per module, if configured for
	//     multiple modules, it will be called for each, e.g. on runtime close.
	CloseNotify(ctx context.Context, exitCode uint32)
}

// ^-- Note: This might need to be a part of the listener or become a part of
// host state implementation. For example, if this is used to implement state
// cleanup for host modules, possibly something like below would be better, as
// it could be implemented in a way that allows concurrent module use.
//
//	// key is like a context key, stateFactory is invoked per instantiate and
//	// is associated with the key (exposed as `Module.State` similar to go
//	// context). Using a key is better than the module name because we can
//	// de-dupe it for host modules that can be instantiated into different
//	// names. Also, you can make the key package private.
//	HostModuleBuilder.WithState(key any, stateFactory func() Cleanup)`
//
// Such a design could work to isolate state only needed for wasip1, for
// example the dirent cache. However, if end users use this for different
// things, we may need separate designs.
//
// In summary, the purpose of this iteration is to identify projects that
// would use something like this, and then we can figure out which way it
// should go.

// CloseNotifyFunc is a convenience for defining inlining a CloseNotifier.
type CloseNotifyFunc func(ctx context.Context, exitCode uint32)

// CloseNotify implements CloseNotifier.CloseNotify.
func (f CloseNotifyFunc) CloseNotify(ctx context.Context, exitCode uint32) {
	f(ctx, exitCode)
}

// WithCloseNotifier registers the given CloseNotifier into the given
// context.Context.
func WithCloseNotifier(ctx context.Context, notifier CloseNotifier) context.Context {
	if notifier != nil {
		return context.WithValue(ctx, expctxkeys.CloseNotifierKey{}, notifier)
	}
	return ctx
}
