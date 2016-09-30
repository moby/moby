package store

import (
	"strings"

	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/manager/state"
	memdb "github.com/hashicorp/go-memdb"
)

const (
	tableCluster = "cluster"

	// DefaultClusterName is the default name to use for the cluster
	// object.
	DefaultClusterName = "default"
)

func init() {
	register(ObjectStoreConfig{
		Name: tableCluster,
		Table: &memdb.TableSchema{
			Name: tableCluster,
			Indexes: map[string]*memdb.IndexSchema{
				indexID: {
					Name:    indexID,
					Unique:  true,
					Indexer: clusterIndexerByID{},
				},
				indexName: {
					Name:    indexName,
					Unique:  true,
					Indexer: clusterIndexerByName{},
				},
			},
		},
		Save: func(tx ReadTx, snapshot *api.StoreSnapshot) error {
			var err error
			snapshot.Clusters, err = FindClusters(tx, All)
			return err
		},
		Restore: func(tx Tx, snapshot *api.StoreSnapshot) error {
			clusters, err := FindClusters(tx, All)
			if err != nil {
				return err
			}
			for _, n := range clusters {
				if err := DeleteCluster(tx, n.ID); err != nil {
					return err
				}
			}
			for _, n := range snapshot.Clusters {
				if err := CreateCluster(tx, n); err != nil {
					return err
				}
			}
			return nil
		},
		ApplyStoreAction: func(tx Tx, sa *api.StoreAction) error {
			switch v := sa.Target.(type) {
			case *api.StoreAction_Cluster:
				obj := v.Cluster
				switch sa.Action {
				case api.StoreActionKindCreate:
					return CreateCluster(tx, obj)
				case api.StoreActionKindUpdate:
					return UpdateCluster(tx, obj)
				case api.StoreActionKindRemove:
					return DeleteCluster(tx, obj.ID)
				}
			}
			return errUnknownStoreAction
		},
		NewStoreAction: func(c state.Event) (api.StoreAction, error) {
			var sa api.StoreAction
			switch v := c.(type) {
			case state.EventCreateCluster:
				sa.Action = api.StoreActionKindCreate
				sa.Target = &api.StoreAction_Cluster{
					Cluster: v.Cluster,
				}
			case state.EventUpdateCluster:
				sa.Action = api.StoreActionKindUpdate
				sa.Target = &api.StoreAction_Cluster{
					Cluster: v.Cluster,
				}
			case state.EventDeleteCluster:
				sa.Action = api.StoreActionKindRemove
				sa.Target = &api.StoreAction_Cluster{
					Cluster: v.Cluster,
				}
			default:
				return api.StoreAction{}, errUnknownStoreAction
			}
			return sa, nil
		},
	})
}

type clusterEntry struct {
	*api.Cluster
}

func (c clusterEntry) ID() string {
	return c.Cluster.ID
}

func (c clusterEntry) Meta() api.Meta {
	return c.Cluster.Meta
}

func (c clusterEntry) SetMeta(meta api.Meta) {
	c.Cluster.Meta = meta
}

func (c clusterEntry) Copy() Object {
	return clusterEntry{c.Cluster.Copy()}
}

func (c clusterEntry) EventCreate() state.Event {
	return state.EventCreateCluster{Cluster: c.Cluster}
}

func (c clusterEntry) EventUpdate() state.Event {
	return state.EventUpdateCluster{Cluster: c.Cluster}
}

func (c clusterEntry) EventDelete() state.Event {
	return state.EventDeleteCluster{Cluster: c.Cluster}
}

// CreateCluster adds a new cluster to the store.
// Returns ErrExist if the ID is already taken.
func CreateCluster(tx Tx, c *api.Cluster) error {
	// Ensure the name is not already in use.
	if tx.lookup(tableCluster, indexName, strings.ToLower(c.Spec.Annotations.Name)) != nil {
		return ErrNameConflict
	}

	return tx.create(tableCluster, clusterEntry{c})
}

// UpdateCluster updates an existing cluster in the store.
// Returns ErrNotExist if the cluster doesn't exist.
func UpdateCluster(tx Tx, c *api.Cluster) error {
	// Ensure the name is either not in use or already used by this same Cluster.
	if existing := tx.lookup(tableCluster, indexName, strings.ToLower(c.Spec.Annotations.Name)); existing != nil {
		if existing.ID() != c.ID {
			return ErrNameConflict
		}
	}

	return tx.update(tableCluster, clusterEntry{c})
}

// DeleteCluster removes a cluster from the store.
// Returns ErrNotExist if the cluster doesn't exist.
func DeleteCluster(tx Tx, id string) error {
	return tx.delete(tableCluster, id)
}

// GetCluster looks up a cluster by ID.
// Returns nil if the cluster doesn't exist.
func GetCluster(tx ReadTx, id string) *api.Cluster {
	n := tx.get(tableCluster, id)
	if n == nil {
		return nil
	}
	return n.(clusterEntry).Cluster
}

// FindClusters selects a set of clusters and returns them.
func FindClusters(tx ReadTx, by By) ([]*api.Cluster, error) {
	checkType := func(by By) error {
		switch by.(type) {
		case byName, byNamePrefix, byIDPrefix:
			return nil
		default:
			return ErrInvalidFindBy
		}
	}

	clusterList := []*api.Cluster{}
	appendResult := func(o Object) {
		clusterList = append(clusterList, o.(clusterEntry).Cluster)
	}

	err := tx.find(tableCluster, by, checkType, appendResult)
	return clusterList, err
}

type clusterIndexerByID struct{}

func (ci clusterIndexerByID) FromArgs(args ...interface{}) ([]byte, error) {
	return fromArgs(args...)
}

func (ci clusterIndexerByID) FromObject(obj interface{}) (bool, []byte, error) {
	c, ok := obj.(clusterEntry)
	if !ok {
		panic("unexpected type passed to FromObject")
	}

	// Add the null character as a terminator
	val := c.Cluster.ID + "\x00"
	return true, []byte(val), nil
}

func (ci clusterIndexerByID) PrefixFromArgs(args ...interface{}) ([]byte, error) {
	return prefixFromArgs(args...)
}

type clusterIndexerByName struct{}

func (ci clusterIndexerByName) FromArgs(args ...interface{}) ([]byte, error) {
	return fromArgs(args...)
}

func (ci clusterIndexerByName) FromObject(obj interface{}) (bool, []byte, error) {
	c, ok := obj.(clusterEntry)
	if !ok {
		panic("unexpected type passed to FromObject")
	}

	// Add the null character as a terminator
	return true, []byte(strings.ToLower(c.Spec.Annotations.Name) + "\x00"), nil
}

func (ci clusterIndexerByName) PrefixFromArgs(args ...interface{}) ([]byte, error) {
	return prefixFromArgs(args...)
}
