package store

import (
	"strings"

	"github.com/docker/swarmkit/api"
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
		Table: &memdb.TableSchema{
			Name: tableCluster,
			Indexes: map[string]*memdb.IndexSchema{
				indexID: {
					Name:    indexID,
					Unique:  true,
					Indexer: api.ClusterIndexerByID{},
				},
				indexName: {
					Name:    indexName,
					Unique:  true,
					Indexer: api.ClusterIndexerByName{},
				},
				indexCustom: {
					Name:         indexCustom,
					Indexer:      api.ClusterCustomIndexer{},
					AllowMissing: true,
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
		ApplyStoreAction: func(tx Tx, sa api.StoreAction) error {
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
	})
}

// CreateCluster adds a new cluster to the store.
// Returns ErrExist if the ID is already taken.
func CreateCluster(tx Tx, c *api.Cluster) error {
	// Ensure the name is not already in use.
	if tx.lookup(tableCluster, indexName, strings.ToLower(c.Spec.Annotations.Name)) != nil {
		return ErrNameConflict
	}

	return tx.create(tableCluster, c)
}

// UpdateCluster updates an existing cluster in the store.
// Returns ErrNotExist if the cluster doesn't exist.
func UpdateCluster(tx Tx, c *api.Cluster) error {
	// Ensure the name is either not in use or already used by this same Cluster.
	if existing := tx.lookup(tableCluster, indexName, strings.ToLower(c.Spec.Annotations.Name)); existing != nil {
		if existing.GetID() != c.ID {
			return ErrNameConflict
		}
	}

	return tx.update(tableCluster, c)
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
	return n.(*api.Cluster)
}

// FindClusters selects a set of clusters and returns them.
func FindClusters(tx ReadTx, by By) ([]*api.Cluster, error) {
	checkType := func(by By) error {
		switch by.(type) {
		case byName, byNamePrefix, byIDPrefix, byCustom, byCustomPrefix:
			return nil
		default:
			return ErrInvalidFindBy
		}
	}

	clusterList := []*api.Cluster{}
	appendResult := func(o api.StoreObject) {
		clusterList = append(clusterList, o.(*api.Cluster))
	}

	err := tx.find(tableCluster, by, checkType, appendResult)
	return clusterList, err
}
