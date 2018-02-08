package controlapi

import (
	"errors"

	"github.com/docker/docker/pkg/plugingetter"
	"github.com/docker/swarmkit/ca"
	"github.com/docker/swarmkit/manager/drivers"
	"github.com/docker/swarmkit/manager/state/raft"
	"github.com/docker/swarmkit/manager/state/store"
)

var (
	errInvalidArgument = errors.New("invalid argument")
)

// Server is the Cluster API gRPC server.
type Server struct {
	store          *store.MemoryStore
	raft           *raft.Node
	securityConfig *ca.SecurityConfig
	pg             plugingetter.PluginGetter
	dr             *drivers.DriverProvider
}

// NewServer creates a Cluster API server.
func NewServer(store *store.MemoryStore, raft *raft.Node, securityConfig *ca.SecurityConfig, pg plugingetter.PluginGetter, dr *drivers.DriverProvider) *Server {
	return &Server{
		store:          store,
		dr:             dr,
		raft:           raft,
		securityConfig: securityConfig,
		pg:             pg,
	}
}
