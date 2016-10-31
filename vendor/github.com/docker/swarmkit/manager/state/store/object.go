package store

import (
	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/manager/state"
	memdb "github.com/hashicorp/go-memdb"
)

// Object is a generic object that can be handled by the store.
type Object interface {
	ID() string               // Get ID
	Meta() api.Meta           // Retrieve metadata
	SetMeta(api.Meta)         // Set metadata
	Copy() Object             // Return a copy of this object
	EventCreate() state.Event // Return a creation event
	EventUpdate() state.Event // Return an update event
	EventDelete() state.Event // Return a deletion event
}

// ObjectStoreConfig provides the necessary methods to store a particular object
// type inside MemoryStore.
type ObjectStoreConfig struct {
	Name             string
	Table            *memdb.TableSchema
	Save             func(ReadTx, *api.StoreSnapshot) error
	Restore          func(Tx, *api.StoreSnapshot) error
	ApplyStoreAction func(Tx, *api.StoreAction) error
	NewStoreAction   func(state.Event) (api.StoreAction, error)
}
