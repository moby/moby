package experimental

import (
	"context"

	"github.com/tetratelabs/wazero/internal/expctxkeys"
)

// Snapshot holds the execution state at the time of a Snapshotter.Snapshot call.
type Snapshot interface {
	// Restore sets the Wasm execution state to the capture. Because a host function
	// calling this is resetting the pointer to the executation stack, the host function
	// will not be able to return values in the normal way. ret is a slice of values the
	// host function intends to return from the restored function.
	Restore(ret []uint64)
}

// Snapshotter allows host functions to snapshot the WebAssembly execution environment.
type Snapshotter interface {
	// Snapshot captures the current execution state.
	Snapshot() Snapshot
}

// WithSnapshotter enables snapshots.
// Passing the returned context to a exported function invocation enables snapshots,
// and allows host functions to retrieve the Snapshotter using GetSnapshotter.
func WithSnapshotter(ctx context.Context) context.Context {
	return context.WithValue(ctx, expctxkeys.EnableSnapshotterKey{}, struct{}{})
}

// GetSnapshotter gets the Snapshotter from a host function.
// It is only present if WithSnapshotter was called with the function invocation context.
func GetSnapshotter(ctx context.Context) Snapshotter {
	return ctx.Value(expctxkeys.SnapshotterKey{}).(Snapshotter)
}
