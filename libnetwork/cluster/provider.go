package cluster

// Provider provides clustering config details
type Provider interface {
	IsManager() bool
	IsAgent() bool
	GetListenAddress() string
	GetRemoteAddress() string
	ListenClusterEvents() <-chan struct{}
}
