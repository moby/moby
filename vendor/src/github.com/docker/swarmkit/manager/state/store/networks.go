package store

import (
	"strings"

	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/manager/state"
	memdb "github.com/hashicorp/go-memdb"
)

const tableNetwork = "network"

func init() {
	register(ObjectStoreConfig{
		Name: tableNetwork,
		Table: &memdb.TableSchema{
			Name: tableNetwork,
			Indexes: map[string]*memdb.IndexSchema{
				indexID: {
					Name:    indexID,
					Unique:  true,
					Indexer: networkIndexerByID{},
				},
				indexName: {
					Name:    indexName,
					Unique:  true,
					Indexer: networkIndexerByName{},
				},
			},
		},
		Save: func(tx ReadTx, snapshot *api.StoreSnapshot) error {
			var err error
			snapshot.Networks, err = FindNetworks(tx, All)
			return err
		},
		Restore: func(tx Tx, snapshot *api.StoreSnapshot) error {
			networks, err := FindNetworks(tx, All)
			if err != nil {
				return err
			}
			for _, n := range networks {
				if err := DeleteNetwork(tx, n.ID); err != nil {
					return err
				}
			}
			for _, n := range snapshot.Networks {
				if err := CreateNetwork(tx, n); err != nil {
					return err
				}
			}
			return nil
		},
		ApplyStoreAction: func(tx Tx, sa *api.StoreAction) error {
			switch v := sa.Target.(type) {
			case *api.StoreAction_Network:
				obj := v.Network
				switch sa.Action {
				case api.StoreActionKindCreate:
					return CreateNetwork(tx, obj)
				case api.StoreActionKindUpdate:
					return UpdateNetwork(tx, obj)
				case api.StoreActionKindRemove:
					return DeleteNetwork(tx, obj.ID)
				}
			}
			return errUnknownStoreAction
		},
		NewStoreAction: func(c state.Event) (api.StoreAction, error) {
			var sa api.StoreAction
			switch v := c.(type) {
			case state.EventCreateNetwork:
				sa.Action = api.StoreActionKindCreate
				sa.Target = &api.StoreAction_Network{
					Network: v.Network,
				}
			case state.EventUpdateNetwork:
				sa.Action = api.StoreActionKindUpdate
				sa.Target = &api.StoreAction_Network{
					Network: v.Network,
				}
			case state.EventDeleteNetwork:
				sa.Action = api.StoreActionKindRemove
				sa.Target = &api.StoreAction_Network{
					Network: v.Network,
				}
			default:
				return api.StoreAction{}, errUnknownStoreAction
			}
			return sa, nil
		},
	})
}

type networkEntry struct {
	*api.Network
}

func (n networkEntry) ID() string {
	return n.Network.ID
}

func (n networkEntry) Meta() api.Meta {
	return n.Network.Meta
}

func (n networkEntry) SetMeta(meta api.Meta) {
	n.Network.Meta = meta
}

func (n networkEntry) Copy() Object {
	return networkEntry{n.Network.Copy()}
}

func (n networkEntry) EventCreate() state.Event {
	return state.EventCreateNetwork{Network: n.Network}
}

func (n networkEntry) EventUpdate() state.Event {
	return state.EventUpdateNetwork{Network: n.Network}
}

func (n networkEntry) EventDelete() state.Event {
	return state.EventDeleteNetwork{Network: n.Network}
}

// CreateNetwork adds a new network to the store.
// Returns ErrExist if the ID is already taken.
func CreateNetwork(tx Tx, n *api.Network) error {
	// Ensure the name is not already in use.
	if tx.lookup(tableNetwork, indexName, strings.ToLower(n.Spec.Annotations.Name)) != nil {
		return ErrNameConflict
	}

	return tx.create(tableNetwork, networkEntry{n})
}

// UpdateNetwork updates an existing network in the store.
// Returns ErrNotExist if the network doesn't exist.
func UpdateNetwork(tx Tx, n *api.Network) error {
	// Ensure the name is either not in use or already used by this same Network.
	if existing := tx.lookup(tableNetwork, indexName, strings.ToLower(n.Spec.Annotations.Name)); existing != nil {
		if existing.ID() != n.ID {
			return ErrNameConflict
		}
	}

	return tx.update(tableNetwork, networkEntry{n})
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
	return n.(networkEntry).Network
}

// FindNetworks selects a set of networks and returns them.
func FindNetworks(tx ReadTx, by By) ([]*api.Network, error) {
	checkType := func(by By) error {
		switch by.(type) {
		case byName, byNamePrefix, byIDPrefix:
			return nil
		default:
			return ErrInvalidFindBy
		}
	}

	networkList := []*api.Network{}
	appendResult := func(o Object) {
		networkList = append(networkList, o.(networkEntry).Network)
	}

	err := tx.find(tableNetwork, by, checkType, appendResult)
	return networkList, err
}

type networkIndexerByID struct{}

func (ni networkIndexerByID) FromArgs(args ...interface{}) ([]byte, error) {
	return fromArgs(args...)
}

func (ni networkIndexerByID) FromObject(obj interface{}) (bool, []byte, error) {
	n, ok := obj.(networkEntry)
	if !ok {
		panic("unexpected type passed to FromObject")
	}

	// Add the null character as a terminator
	val := n.Network.ID + "\x00"
	return true, []byte(val), nil
}

func (ni networkIndexerByID) PrefixFromArgs(args ...interface{}) ([]byte, error) {
	return prefixFromArgs(args...)
}

type networkIndexerByName struct{}

func (ni networkIndexerByName) FromArgs(args ...interface{}) ([]byte, error) {
	return fromArgs(args...)
}

func (ni networkIndexerByName) FromObject(obj interface{}) (bool, []byte, error) {
	n, ok := obj.(networkEntry)
	if !ok {
		panic("unexpected type passed to FromObject")
	}

	// Add the null character as a terminator
	return true, []byte(strings.ToLower(n.Spec.Annotations.Name) + "\x00"), nil
}

func (ni networkIndexerByName) PrefixFromArgs(args ...interface{}) ([]byte, error) {
	return prefixFromArgs(args...)
}
