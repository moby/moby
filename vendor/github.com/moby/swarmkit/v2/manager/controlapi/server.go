package controlapi

import (
	"errors"

	"github.com/docker/docker/pkg/plugingetter"
	"github.com/moby/swarmkit/v2/ca"
	"github.com/moby/swarmkit/v2/manager/drivers"
	"github.com/moby/swarmkit/v2/manager/state/raft"
	"github.com/moby/swarmkit/v2/manager/state/store"
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
