package store

import (
	"errors"
	"strings"

	"github.com/docker/swarmkit/api"
	memdb "github.com/hashicorp/go-memdb"
)

const tableExtension = "extension"

func init() {
	register(ObjectStoreConfig{
		Table: &memdb.TableSchema{
			Name: tableExtension,
			Indexes: map[string]*memdb.IndexSchema{
				indexID: {
					Name:    indexID,
					Unique:  true,
					Indexer: api.ExtensionIndexerByID{},
				},
				indexName: {
					Name:    indexName,
					Unique:  true,
					Indexer: api.ExtensionIndexerByName{},
				},
				indexCustom: {
					Name:         indexCustom,
					Indexer:      api.ExtensionCustomIndexer{},
					AllowMissing: true,
				},
			},
		},
		Save: func(tx ReadTx, snapshot *api.StoreSnapshot) error {
			var err error
			snapshot.Extensions, err = FindExtensions(tx, All)
			return err
		},
		Restore: func(tx Tx, snapshot *api.StoreSnapshot) error {
			extensions, err := FindExtensions(tx, All)
			if err != nil {
				return err
			}
			for _, e := range extensions {
				if err := DeleteExtension(tx, e.ID); err != nil {
					return err
				}
			}
			for _, e := range snapshot.Extensions {
				if err := CreateExtension(tx, e); err != nil {
					return err
				}
			}
			return nil
		},
		ApplyStoreAction: func(tx Tx, sa api.StoreAction) error {
			switch v := sa.Target.(type) {
			case *api.StoreAction_Extension:
				obj := v.Extension
				switch sa.Action {
				case api.StoreActionKindCreate:
					return CreateExtension(tx, obj)
				case api.StoreActionKindUpdate:
					return UpdateExtension(tx, obj)
				case api.StoreActionKindRemove:
					return DeleteExtension(tx, obj.ID)
				}
			}
			return errUnknownStoreAction
		},
	})
}

type extensionEntry struct {
	*api.Extension
}

// CreateExtension adds a new extension to the store.
// Returns ErrExist if the ID is already taken.
func CreateExtension(tx Tx, e *api.Extension) error {
	// Ensure the name is not already in use.
	if tx.lookup(tableExtension, indexName, strings.ToLower(e.Annotations.Name)) != nil {
		return ErrNameConflict
	}

	// It can't conflict with built-in kinds either.
	if _, ok := schema.Tables[e.Annotations.Name]; ok {
		return ErrNameConflict
	}

	return tx.create(tableExtension, extensionEntry{e})
}

// UpdateExtension updates an existing extension in the store.
// Returns ErrNotExist if the object doesn't exist.
func UpdateExtension(tx Tx, e *api.Extension) error {
	// TODO(aaronl): For the moment, extensions are immutable
	return errors.New("extensions are immutable")
}

// DeleteExtension removes an extension from the store.
// Returns ErrNotExist if the object doesn't exist.
func DeleteExtension(tx Tx, id string) error {
	e := tx.get(tableExtension, id)
	if e == nil {
		return ErrNotExist
	}

	resources, err := FindResources(tx, ByKind(e.(extensionEntry).Annotations.Name))
	if err != nil {
		return err
	}

	if len(resources) != 0 {
		return errors.New("cannot delete extension because objects of this type exist in the data store")
	}

	return tx.delete(tableExtension, id)
}

// GetExtension looks up an extension by ID.
// Returns nil if the object doesn't exist.
func GetExtension(tx ReadTx, id string) *api.Extension {
	e := tx.get(tableExtension, id)
	if e == nil {
		return nil
	}
	return e.(extensionEntry).Extension
}

// FindExtensions selects a set of extensions and returns them.
func FindExtensions(tx ReadTx, by By) ([]*api.Extension, error) {
	checkType := func(by By) error {
		switch by.(type) {
		case byIDPrefix, byName, byCustom, byCustomPrefix:
			return nil
		default:
			return ErrInvalidFindBy
		}
	}

	extensionList := []*api.Extension{}
	appendResult := func(o api.StoreObject) {
		extensionList = append(extensionList, o.(extensionEntry).Extension)
	}

	err := tx.find(tableExtension, by, checkType, appendResult)
	return extensionList, err
}
