package store

import (
	"strconv"
	"strings"

	"github.com/docker/swarmkit/api"
	memdb "github.com/hashicorp/go-memdb"
)

const tableNode = "node"

func init() {
	register(ObjectStoreConfig{
		Table: &memdb.TableSchema{
			Name: tableNode,
			Indexes: map[string]*memdb.IndexSchema{
				indexID: {
					Name:    indexID,
					Unique:  true,
					Indexer: api.NodeIndexerByID{},
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
				indexCustom: {
					Name:         indexCustom,
					Indexer:      api.NodeCustomIndexer{},
					AllowMissing: true,
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
		ApplyStoreAction: func(tx Tx, sa api.StoreAction) error {
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
	})
}

// CreateNode adds a new node to the store.
// Returns ErrExist if the ID is already taken.
func CreateNode(tx Tx, n *api.Node) error {
	return tx.create(tableNode, n)
}

// UpdateNode updates an existing node in the store.
// Returns ErrNotExist if the node doesn't exist.
func UpdateNode(tx Tx, n *api.Node) error {
	return tx.update(tableNode, n)
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
	return n.(*api.Node)
}

// FindNodes selects a set of nodes and returns them.
func FindNodes(tx ReadTx, by By) ([]*api.Node, error) {
	checkType := func(by By) error {
		switch by.(type) {
		case byName, byNamePrefix, byIDPrefix, byRole, byMembership, byCustom, byCustomPrefix:
			return nil
		default:
			return ErrInvalidFindBy
		}
	}

	nodeList := []*api.Node{}
	appendResult := func(o api.StoreObject) {
		nodeList = append(nodeList, o.(*api.Node))
	}

	err := tx.find(tableNode, by, checkType, appendResult)
	return nodeList, err
}

type nodeIndexerByHostname struct{}

func (ni nodeIndexerByHostname) FromArgs(args ...interface{}) ([]byte, error) {
	return fromArgs(args...)
}

func (ni nodeIndexerByHostname) FromObject(obj interface{}) (bool, []byte, error) {
	n := obj.(*api.Node)

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
	n := obj.(*api.Node)

	// Add the null character as a terminator
	return true, []byte(strconv.FormatInt(int64(n.Role), 10) + "\x00"), nil
}

type nodeIndexerByMembership struct{}

func (ni nodeIndexerByMembership) FromArgs(args ...interface{}) ([]byte, error) {
	return fromArgs(args...)
}

func (ni nodeIndexerByMembership) FromObject(obj interface{}) (bool, []byte, error) {
	n := obj.(*api.Node)

	// Add the null character as a terminator
	return true, []byte(strconv.FormatInt(int64(n.Spec.Membership), 10) + "\x00"), nil
}
