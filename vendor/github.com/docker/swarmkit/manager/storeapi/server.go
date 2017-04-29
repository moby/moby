package storeapi

import (
	"github.com/docker/swarmkit/manager/state/store"
)

// Server is the store API gRPC server.
type Server struct {
	store *store.MemoryStore
}

// NewServer creates a store API server.
func NewServer(store *store.MemoryStore) *Server {
	return &Server{
		store: store,
	}
}
