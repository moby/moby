package cluster

// Provider provides clustering config details
type Provider interface {
	IsManager() bool
	IsAgent() bool
	GetLocalAddress() string
	GetListenAddress() string
	GetAdvertiseAddress() string
	GetRemoteAddress() string
	ListenClusterEvents() <-chan struct{}
}
