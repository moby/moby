package controlapi

import (
	"errors"

	"github.com/docker/swarmkit/ca"
	"github.com/docker/swarmkit/manager/state/raft"
	"github.com/docker/swarmkit/manager/state/store"
)

var (
	errNotImplemented  = errors.New("not implemented")
	errInvalidArgument = errors.New("invalid argument")
)

// Server is the Cluster API gRPC server.
type Server struct {
	store  *store.MemoryStore
	raft   *raft.Node
	rootCA *ca.RootCA
}

// NewServer creates a Cluster API server.
func NewServer(store *store.MemoryStore, raft *raft.Node, rootCA *ca.RootCA) *Server {
	return &Server{
		store:  store,
		raft:   raft,
		rootCA: rootCA,
	}
}
