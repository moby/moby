package store

import (
	"strconv"
	"strings"

	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/manager/state"
	memdb "github.com/hashicorp/go-memdb"
)

const tableNode = "node"

func init() {
	register(ObjectStoreConfig{
		Name: tableNode,
		Table: &memdb.TableSchema{
			Name: tableNode,
			Indexes: map[string]*memdb.IndexSchema{
				indexID: {
					Name:    indexID,
					Unique:  true,
					Indexer: nodeIndexerByID{},
				},
				// TODO(aluzzardi): Use `indexHostname` instead.
				indexName: {
					Name:         indexName,
					AllowMissing: true,
					Indexer:      nodeIndexerByHostname{},
				},
				indexRole: {
					Name:    indexRole,
					Indexer: nodeIndexerByRole{},
				},
				indexMembership: {
					Name:    indexMembership,
					Indexer: nodeIndexerByMembership{},
				},
			},
		},
		Save: func(tx ReadTx, snapshot *api.StoreSnapshot) error {
			var err error
			snapshot.Nodes, err = FindNodes(tx, All)
			return err
		},
		Restore: func(tx Tx, snapshot *api.StoreSnapshot) error {
			nodes, err := FindNodes(tx, All)
			if err != nil {
				return err
			}
			for _, n := range nodes {
				if err := DeleteNode(tx, n.ID); err != nil {
					return err
				}
			}
			for _, n := range snapshot.Nodes {
				if err := CreateNode(tx, n); err != nil {
					return err
				}
			}
			return nil
		},
		ApplyStoreAction: func(tx Tx, sa *api.StoreAction) error {
			switch v := sa.Target.(type) {
			case *api.StoreAction_Node:
				obj := v.Node
				switch sa.Action {
				case api.StoreActionKindCreate:
					return CreateNode(tx, obj)
				case api.StoreActionKindUpdate:
					return UpdateNode(tx, obj)
				case api.StoreActionKindRemove:
					return DeleteNode(tx, obj.ID)
				}
			}
			return errUnknownStoreAction
		},
		NewStoreAction: func(c state.Event) (api.StoreAction, error) {
			var sa api.StoreAction
			switch v := c.(type) {
			case state.EventCreateNode:
				sa.Action = api.StoreActionKindCreate
				sa.Target = &api.StoreAction_Node{
					Node: v.Node,
				}
			case state.EventUpdateNode:
				sa.Action = api.StoreActionKindUpdate
				sa.Target = &api.StoreAction_Node{
					Node: v.Node,
				}
			case state.EventDeleteNode:
				sa.Action = api.StoreActionKindRemove
				sa.Target = &api.StoreAction_Node{
					Node: v.Node,
				}
			default:
				return api.StoreAction{}, errUnknownStoreAction
			}
			return sa, nil
		},
	})
}

type nodeEntry struct {
	*api.Node
}

func (n nodeEntry) ID() string {
	return n.Node.ID
}

func (n nodeEntry) Meta() api.Meta {
	return n.Node.Meta
}

func (n nodeEntry) SetMeta(meta api.Meta) {
	n.Node.Meta = meta
}

func (n nodeEntry) Copy() Object {
	return nodeEntry{n.Node.Copy()}
}

func (n nodeEntry) EventCreate() state.Event {
	return state.EventCreateNode{Node: n.Node}
}

func (n nodeEntry) EventUpdate() state.Event {
	return state.EventUpdateNode{Node: n.Node}
}

func (n nodeEntry) EventDelete() state.Event {
	return state.EventDeleteNode{Node: n.Node}
}

// CreateNode adds a new node to the store.
// Returns ErrExist if the ID is already taken.
func CreateNode(tx Tx, n *api.Node) error {
	return tx.create(tableNode, nodeEntry{n})
}

// UpdateNode updates an existing node in the store.
// Returns ErrNotExist if the node doesn't exist.
func UpdateNode(tx Tx, n *api.Node) error {
	return tx.update(tableNode, nodeEntry{n})
}

// DeleteNode removes a node from the store.
// Returns ErrNotExist if the node doesn't exist.
func DeleteNode(tx Tx, id string) error {
	return tx.delete(tableNode, id)
}

// GetNode looks up a node by ID.
// Returns nil if the node doesn't exist.
func GetNode(tx ReadTx, id string) *api.Node {
	n := tx.get(tableNode, id)
	if n == nil {
		return nil
	}
	return n.(nodeEntry).Node
}

// FindNodes selects a set of nodes and returns them.
func FindNodes(tx ReadTx, by By) ([]*api.Node, error) {
	checkType := func(by By) error {
		switch by.(type) {
		case byName, byNamePrefix, byIDPrefix, byRole, byMembership:
			return nil
		default:
			return ErrInvalidFindBy
		}
	}

	nodeList := []*api.Node{}
	appendResult := func(o Object) {
		nodeList = append(nodeList, o.(nodeEntry).Node)
	}

	err := tx.find(tableNode, by, checkType, appendResult)
	return nodeList, err
}

type nodeIndexerByID struct{}

func (ni nodeIndexerByID) FromArgs(args ...interface{}) ([]byte, error) {
	return fromArgs(args...)
}

func (ni nodeIndexerByID) FromObject(obj interface{}) (bool, []byte, error) {
	n, ok := obj.(nodeEntry)
	if !ok {
		panic("unexpected type passed to FromObject")
	}

	// Add the null character as a terminator
	val := n.Node.ID + "\x00"
	return true, []byte(val), nil
}

func (ni nodeIndexerByID) PrefixFromArgs(args ...interface{}) ([]byte, error) {
	return prefixFromArgs(args...)
}

type nodeIndexerByHostname struct{}

func (ni nodeIndexerByHostname) FromArgs(args ...interface{}) ([]byte, error) {
	return fromArgs(args...)
}

func (ni nodeIndexerByHostname) FromObject(obj interface{}) (bool, []byte, error) {
	n, ok := obj.(nodeEntry)
	if !ok {
		panic("unexpected type passed to FromObject")
	}

	if n.Description == nil {
		return false, nil, nil
	}
	// Add the null character as a terminator
	return true, []byte(strings.ToLower(n.Description.Hostname) + "\x00"), nil
}

func (ni nodeIndexerByHostname) PrefixFromArgs(args ...interface{}) ([]byte, error) {
	return prefixFromArgs(args...)
}

type nodeIndexerByRole struct{}

func (ni nodeIndexerByRole) FromArgs(args ...interface{}) ([]byte, error) {
	return fromArgs(args...)
}

func (ni nodeIndexerByRole) FromObject(obj interface{}) (bool, []byte, error) {
	n, ok := obj.(nodeEntry)
	if !ok {
		panic("unexpected type passed to FromObject")
	}

	// Add the null character as a terminator
	return true, []byte(strconv.FormatInt(int64(n.Spec.Role), 10) + "\x00"), nil
}

type nodeIndexerByMembership struct{}

func (ni nodeIndexerByMembership) FromArgs(args ...interface{}) ([]byte, error) {
	return fromArgs(args...)
}

func (ni nodeIndexerByMembership) FromObject(obj interface{}) (bool, []byte, error) {
	n, ok := obj.(nodeEntry)
	if !ok {
		panic("unexpected type passed to FromObject")
	}

	// Add the null character as a terminator
	return true, []byte(strconv.FormatInt(int64(n.Spec.Membership), 10) + "\x00"), nil
}
