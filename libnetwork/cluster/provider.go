package cluster

import "github.com/docker/libnetwork/types"

// Provider provides clustering config details
type Provider interface {
	IsManager() bool
	IsAgent() bool
	GetListenAddress() string
	GetRemoteAddress() string
	ListenClusterEvents() <-chan struct{}
	GetNetworkKeys() []*types.EncryptionKey
	SetNetworkKeys([]*types.EncryptionKey)
}
