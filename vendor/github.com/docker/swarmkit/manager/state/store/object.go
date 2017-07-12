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

// RestoreTable takes a list of new objects of a particular type (e.g. clusters,
// nodes, etc., which conform to the StoreObject interface) and replaces the
// existing objects in the store of that type with the new objects.
func RestoreTable(tx Tx, table string, newObjects []api.StoreObject) error {
	checkType := func(by By) error {
		return nil
	}
	var oldObjects []api.StoreObject
	appendResult := func(o api.StoreObject) {
		oldObjects = append(oldObjects, o)
	}

	err := tx.find(table, All, checkType, appendResult)
	if err != nil {
		return nil
	}

	updated := make(map[string]struct{})

	for _, o := range newObjects {
		objectID := o.GetID()
		if existing := tx.lookup(table, indexID, objectID); existing != nil {
			if err := tx.update(table, o); err != nil {
				return err
			}
			updated[objectID] = struct{}{}
		} else {
			if err := tx.create(table, o); err != nil {
				return err
			}
		}
	}
	for _, o := range oldObjects {
		objectID := o.GetID()
		if _, ok := updated[objectID]; !ok {
			if err := tx.delete(table, objectID); err != nil {
				return err
			}
		}
	}
	return nil
}
