package controlapi

import (
	"errors"

	"github.com/moby/swarmkit/v2/ca"
	"github.com/moby/swarmkit/v2/manager/allocator/networkallocator"
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
	netvalidator   networkallocator.DriverValidator
	dr             *drivers.DriverProvider
}

// NewServer creates a Cluster API server.
func NewServer(store *store.MemoryStore, raft *raft.Node, securityConfig *ca.SecurityConfig, nv networkallocator.DriverValidator, dr *drivers.DriverProvider) *Server {
	if nv == nil {
		nv = networkallocator.InertProvider{}
	}
	return &Server{
		store:          store,
		dr:             dr,
		raft:           raft,
		securityConfig: securityConfig,
		netvalidator:   nv,
	}
}
