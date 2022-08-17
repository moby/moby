package store

import (
	"strings"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/moby/swarmkit/v2/api"
)

const tableSecret = "secret"

func init() {
	register(ObjectStoreConfig{
		Table: &memdb.TableSchema{
			Name: tableSecret,
			Indexes: map[string]*memdb.IndexSchema{
				indexID: {
					Name:    indexID,
					Unique:  true,
					Indexer: api.SecretIndexerByID{},
				},
				indexName: {
					Name:    indexName,
					Unique:  true,
					Indexer: api.SecretIndexerByName{},
				},
				indexCustom: {
					Name:         indexCustom,
					Indexer:      api.SecretCustomIndexer{},
					AllowMissing: true,
				},
			},
		},
		Save: func(tx ReadTx, snapshot *api.StoreSnapshot) error {
			var err error
			snapshot.Secrets, err = FindSecrets(tx, All)
			return err
		},
		Restore: func(tx Tx, snapshot *api.StoreSnapshot) error {
			toStoreObj := make([]api.StoreObject, len(snapshot.Secrets))
			for i, x := range snapshot.Secrets {
				toStoreObj[i] = x
			}
			return RestoreTable(tx, tableSecret, toStoreObj)
		},
		ApplyStoreAction: func(tx Tx, sa api.StoreAction) error {
			switch v := sa.Target.(type) {
			case *api.StoreAction_Secret:
				obj := v.Secret
				switch sa.Action {
				case api.StoreActionKindCreate:
					return CreateSecret(tx, obj)
				case api.StoreActionKindUpdate:
					return UpdateSecret(tx, obj)
				case api.StoreActionKindRemove:
					return DeleteSecret(tx, obj.ID)
				}
			}
			return errUnknownStoreAction
		},
	})
}

// CreateSecret adds a new secret to the store.
// Returns ErrExist if the ID is already taken.
func CreateSecret(tx Tx, s *api.Secret) error {
	// Ensure the name is not already in use.
	if tx.lookup(tableSecret, indexName, strings.ToLower(s.Spec.Annotations.Name)) != nil {
		return ErrNameConflict
	}

	return tx.create(tableSecret, s)
}

// UpdateSecret updates an existing secret in the store.
// Returns ErrNotExist if the secret doesn't exist.
func UpdateSecret(tx Tx, s *api.Secret) error {
	// Ensure the name is either not in use or already used by this same Secret.
	if existing := tx.lookup(tableSecret, indexName, strings.ToLower(s.Spec.Annotations.Name)); existing != nil {
		if existing.GetID() != s.ID {
			return ErrNameConflict
		}
	}

	return tx.update(tableSecret, s)
}

// DeleteSecret removes a secret from the store.
// Returns ErrNotExist if the secret doesn't exist.
func DeleteSecret(tx Tx, id string) error {
	return tx.delete(tableSecret, id)
}

// GetSecret looks up a secret by ID.
// Returns nil if the secret doesn't exist.
func GetSecret(tx ReadTx, id string) *api.Secret {
	n := tx.get(tableSecret, id)
	if n == nil {
		return nil
	}
	return n.(*api.Secret)
}

// FindSecrets selects a set of secrets and returns them.
func FindSecrets(tx ReadTx, by By) ([]*api.Secret, error) {
	checkType := func(by By) error {
		switch by.(type) {
		case byName, byNamePrefix, byIDPrefix, byCustom, byCustomPrefix:
			return nil
		default:
			return ErrInvalidFindBy
		}
	}

	secretList := []*api.Secret{}
	appendResult := func(o api.StoreObject) {
		secretList = append(secretList, o.(*api.Secret))
	}

	err := tx.find(tableSecret, by, checkType, appendResult)
	return secretList, err
}
