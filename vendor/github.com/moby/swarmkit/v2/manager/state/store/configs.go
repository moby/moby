package store

import (
	"strings"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/moby/swarmkit/v2/api"
)

const tableConfig = "config"

func init() {
	register(ObjectStoreConfig{
		Table: &memdb.TableSchema{
			Name: tableConfig,
			Indexes: map[string]*memdb.IndexSchema{
				indexID: {
					Name:    indexID,
					Unique:  true,
					Indexer: api.ConfigIndexerByID{},
				},
				indexName: {
					Name:    indexName,
					Unique:  true,
					Indexer: api.ConfigIndexerByName{},
				},
				indexCustom: {
					Name:         indexCustom,
					Indexer:      api.ConfigCustomIndexer{},
					AllowMissing: true,
				},
			},
		},
		Save: func(tx ReadTx, snapshot *api.StoreSnapshot) error {
			var err error
			snapshot.Configs, err = FindConfigs(tx, All)
			return err
		},
		Restore: func(tx Tx, snapshot *api.StoreSnapshot) error {
			toStoreObj := make([]api.StoreObject, len(snapshot.Configs))
			for i, x := range snapshot.Configs {
				toStoreObj[i] = x
			}
			return RestoreTable(tx, tableConfig, toStoreObj)
		},
		ApplyStoreAction: func(tx Tx, sa api.StoreAction) error {
			switch v := sa.Target.(type) {
			case *api.StoreAction_Config:
				obj := v.Config
				switch sa.Action {
				case api.StoreActionKindCreate:
					return CreateConfig(tx, obj)
				case api.StoreActionKindUpdate:
					return UpdateConfig(tx, obj)
				case api.StoreActionKindRemove:
					return DeleteConfig(tx, obj.ID)
				}
			}
			return errUnknownStoreAction
		},
	})
}

// CreateConfig adds a new config to the store.
// Returns ErrExist if the ID is already taken.
func CreateConfig(tx Tx, c *api.Config) error {
	// Ensure the name is not already in use.
	if tx.lookup(tableConfig, indexName, strings.ToLower(c.Spec.Annotations.Name)) != nil {
		return ErrNameConflict
	}

	return tx.create(tableConfig, c)
}

// UpdateConfig updates an existing config in the store.
// Returns ErrNotExist if the config doesn't exist.
func UpdateConfig(tx Tx, c *api.Config) error {
	// Ensure the name is either not in use or already used by this same Config.
	if existing := tx.lookup(tableConfig, indexName, strings.ToLower(c.Spec.Annotations.Name)); existing != nil {
		if existing.GetID() != c.ID {
			return ErrNameConflict
		}
	}

	return tx.update(tableConfig, c)
}

// DeleteConfig removes a config from the store.
// Returns ErrNotExist if the config doesn't exist.
func DeleteConfig(tx Tx, id string) error {
	return tx.delete(tableConfig, id)
}

// GetConfig looks up a config by ID.
// Returns nil if the config doesn't exist.
func GetConfig(tx ReadTx, id string) *api.Config {
	c := tx.get(tableConfig, id)
	if c == nil {
		return nil
	}
	return c.(*api.Config)
}

// FindConfigs selects a set of configs and returns them.
func FindConfigs(tx ReadTx, by By) ([]*api.Config, error) {
	checkType := func(by By) error {
		switch by.(type) {
		case byName, byNamePrefix, byIDPrefix, byCustom, byCustomPrefix:
			return nil
		default:
			return ErrInvalidFindBy
		}
	}

	configList := []*api.Config{}
	appendResult := func(o api.StoreObject) {
		configList = append(configList, o.(*api.Config))
	}

	err := tx.find(tableConfig, by, checkType, appendResult)
	return configList, err
}
