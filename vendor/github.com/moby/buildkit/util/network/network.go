package network

// Provider interface for Network
type Provider interface {
	NewInterface() (Interface, error)
	Release(Interface) error
}

// Interface of network for workers
type Interface interface {
	// Set the pid with network interace namespace
	Set(int) error
	// Removes the network interface
	Remove(int) error
}

// NetworkOpts hold network options
type NetworkOpts struct {
	Type          string
	CNIConfigPath string
	CNIPluginPath string
}
