package libcontainerd

// Remote on Linux defines the accesspoint to the containerd grpc API.
// Remote on Windows is largely an unimplemented interface as there is
// no remote containerd.
type Remote interface {
	// Client returns a new Client instance connected with given Backend.
	Client(Backend) (Client, error)
	// Cleanup stops containerd if it was started by libcontainerd.
	// Note this is not used on Windows as there is no remote containerd.
	Cleanup()
}

// RemoteOption allows to configure paramters of remotes.
// This is unused on Windows.
type RemoteOption interface {
	Apply(Remote) error
}
