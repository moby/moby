package store

import (
	"strings"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/moby/swarmkit/v2/api"
)

const tableNetwork = "network"

func init() {
	register(ObjectStoreConfig{
		Table: &memdb.TableSchema{
			Name: tableNetwork,
			Indexes: map[string]*memdb.IndexSchema{
				indexID: {
					Name:    indexID,
					Unique:  true,
					Indexer: api.NetworkIndexerByID{},
				},
				indexName: {
					Name:    indexName,
					Unique:  true,
					Indexer: api.NetworkIndexerByName{},
				},
				indexCustom: {
					Name:         indexCustom,
					Indexer:      api.NetworkCustomIndexer{},
					AllowMissing: true,
				},
			},
		},
		Save: func(tx ReadTx, snapshot *api.StoreSnapshot) error {
			var err error
			snapshot.Networks, err = FindNetworks(tx, All)
			return err
		},
		Restore: func(tx Tx, snapshot *api.StoreSnapshot) error {
			toStoreObj := make([]api.StoreObject, len(snapshot.Networks))
			for i, x := range snapshot.Networks {
				toStoreObj[i] = x
			}
			return RestoreTable(tx, tableNetwork, toStoreObj)
		},
		ApplyStoreAction: func(tx Tx, sa *api.StoreAction) error {
			switch v := sa.Target.(type) {
			case *api.StoreAction_Network:
				obj := v.Network
				switch sa.Action {
				case api.StoreActionKind_STORE_ACTION_CREATE:
					return CreateNetwork(tx, obj)
				case api.StoreActionKind_STORE_ACTION_UPDATE:
					return UpdateNetwork(tx, obj)
				case api.StoreActionKind_STORE_ACTION_REMOVE:
					return DeleteNetwork(tx, obj.Id)
				}
			}
			return errUnknownStoreAction
		},
	})
}

// CreateNetwork adds a new network to the store.
// Returns ErrExist if the ID is already taken.
func CreateNetwork(tx Tx, n *api.Network) error {
	// Ensure the name is not already in use.
	if tx.lookup(tableNetwork, indexName, strings.ToLower(n.GetSpec().GetAnnotations().GetName())) != nil {
		return ErrNameConflict
	}

	return tx.create(tableNetwork, n)
}

// UpdateNetwork updates an existing network in the store.
// Returns ErrNotExist if the network doesn't exist.
func UpdateNetwork(tx Tx, n *api.Network) error {
	// Ensure the name is either not in use or already used by this same Network.
	if existing := tx.lookup(tableNetwork, indexName, strings.ToLower(n.GetSpec().GetAnnotations().GetName())); existing != nil {
		if existing.GetId() != n.Id {
			return ErrNameConflict
		}
	}

	return tx.update(tableNetwork, n)
}

// DeleteNetwork removes a network from the store.
// Returns ErrNotExist if the network doesn't exist.
func DeleteNetwork(tx Tx, id string) error {
	return tx.delete(tableNetwork, id)
}

// GetNetwork looks up a network by ID.
// Returns nil if the network doesn't exist.
func GetNetwork(tx ReadTx, id string) *api.Network {
	n := tx.get(tableNetwork, id)
	if n == nil {
		return nil
	}
	return n.(*api.Network)
}

// FindNetworks selects a set of networks and returns them.
func FindNetworks(tx ReadTx, by By) ([]*api.Network, error) {
	checkType := func(by By) error {
		switch by.(type) {
		case byName, byNamePrefix, byIDPrefix, byCustom, byCustomPrefix, byAll:
			return nil
		default:
			return ErrInvalidFindBy
		}
	}

	networkList := []*api.Network{}
	appendResult := func(o api.StoreObject) {
		networkList = append(networkList, o.(*api.Network))
	}

	err := tx.find(tableNetwork, by, checkType, appendResult)
	return networkList, err
}
