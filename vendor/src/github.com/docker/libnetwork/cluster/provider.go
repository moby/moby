package cluster

// Provider provides clustering config details
type Provider interface {
	IsManager() bool
	IsAgent() bool
	GetLocalAddress() string
	GetAdvertiseAddress() string
	GetRemoteAddress() string
	ListenClusterEvents() <-chan struct{}
	AllocateEndpoint(string, string, []string) (interface{}, interface{}, error)
	DeallocateEndpoint(string) error
}
