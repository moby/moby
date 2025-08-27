package client

// NetworkDisconnectOptions represents the data to be used to disconnect a container
// from the network.
type NetworkDisconnectOptions struct {
	Container string
	Force     bool
}
