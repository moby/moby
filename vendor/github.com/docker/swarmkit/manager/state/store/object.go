package store

import (
	"github.com/docker/swarmkit/api"
	memdb "github.com/hashicorp/go-memdb"
)

// ObjectStoreConfig provides the necessary methods to store a particular object
// type inside MemoryStore.
type ObjectStoreConfig struct {
	Table            *memdb.TableSchema
	Save             func(ReadTx, *api.StoreSnapshot) error
	Restore          func(Tx, *api.StoreSnapshot) error
	ApplyStoreAction func(Tx, api.StoreAction) error
}
