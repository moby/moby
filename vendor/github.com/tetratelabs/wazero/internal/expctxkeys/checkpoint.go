package expctxkeys

// EnableSnapshotterKey is a context key to indicate that snapshotting should be enabled.
// The context.Context passed to a exported function invocation should have this key set
// to a non-nil value, and host functions will be able to retrieve it using SnapshotterKey.
type EnableSnapshotterKey struct{}

// SnapshotterKey is a context key to access a Snapshotter from a host function.
// It is only present if EnableSnapshotter was set in the function invocation context.
type SnapshotterKey struct{}
