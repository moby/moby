package store

import (
	"strings"

	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/manager/state"
	memdb "github.com/hashicorp/go-memdb"
)

const tableSecret = "secret"

func init() {
	register(ObjectStoreConfig{
		Name: tableSecret,
		Table: &memdb.TableSchema{
			Name: tableSecret,
			Indexes: map[string]*memdb.IndexSchema{
				indexID: {
					Name:    indexID,
					Unique:  true,
					Indexer: secretIndexerByID{},
				},
				indexName: {
					Name:    indexName,
					Unique:  true,
					Indexer: secretIndexerByName{},
				},
			},
		},
		Save: func(tx ReadTx, snapshot *api.StoreSnapshot) error {
			var err error
			snapshot.Secrets, err = FindSecrets(tx, All)
			return err
		},
		Restore: func(tx Tx, snapshot *api.StoreSnapshot) error {
			secrets, err := FindSecrets(tx, All)
			if err != nil {
				return err
			}
			for _, s := range secrets {
				if err := DeleteSecret(tx, s.ID); err != nil {
					return err
				}
			}
			for _, s := range snapshot.Secrets {
				if err := CreateSecret(tx, s); err != nil {
					return err
				}
			}
			return nil
		},
		ApplyStoreAction: func(tx Tx, sa *api.StoreAction) error {
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
		NewStoreAction: func(c state.Event) (api.StoreAction, error) {
			var sa api.StoreAction
			switch v := c.(type) {
			case state.EventCreateSecret:
				sa.Action = api.StoreActionKindCreate
				sa.Target = &api.StoreAction_Secret{
					Secret: v.Secret,
				}
			case state.EventUpdateSecret:
				sa.Action = api.StoreActionKindUpdate
				sa.Target = &api.StoreAction_Secret{
					Secret: v.Secret,
				}
			case state.EventDeleteSecret:
				sa.Action = api.StoreActionKindRemove
				sa.Target = &api.StoreAction_Secret{
					Secret: v.Secret,
				}
			default:
				return api.StoreAction{}, errUnknownStoreAction
			}
			return sa, nil
		},
	})
}

type secretEntry struct {
	*api.Secret
}

func (s secretEntry) ID() string {
	return s.Secret.ID
}

func (s secretEntry) Meta() api.Meta {
	return s.Secret.Meta
}

func (s secretEntry) SetMeta(meta api.Meta) {
	s.Secret.Meta = meta
}

func (s secretEntry) Copy() Object {
	return secretEntry{s.Secret.Copy()}
}

func (s secretEntry) EventCreate() state.Event {
	return state.EventCreateSecret{Secret: s.Secret}
}

func (s secretEntry) EventUpdate() state.Event {
	return state.EventUpdateSecret{Secret: s.Secret}
}

func (s secretEntry) EventDelete() state.Event {
	return state.EventDeleteSecret{Secret: s.Secret}
}

// CreateSecret adds a new secret to the store.
// Returns ErrExist if the ID is already taken.
func CreateSecret(tx Tx, s *api.Secret) error {
	// Ensure the name is not already in use.
	if tx.lookup(tableSecret, indexName, strings.ToLower(s.Spec.Annotations.Name)) != nil {
		return ErrNameConflict
	}

	return tx.create(tableSecret, secretEntry{s})
}

// UpdateSecret updates an existing secret in the store.
// Returns ErrNotExist if the secret doesn't exist.
func UpdateSecret(tx Tx, s *api.Secret) error {
	// Ensure the name is either not in use or already used by this same Secret.
	if existing := tx.lookup(tableSecret, indexName, strings.ToLower(s.Spec.Annotations.Name)); existing != nil {
		if existing.ID() != s.ID {
			return ErrNameConflict
		}
	}

	return tx.update(tableSecret, secretEntry{s})
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
	return n.(secretEntry).Secret
}

// FindSecrets selects a set of secrets and returns them.
func FindSecrets(tx ReadTx, by By) ([]*api.Secret, error) {
	checkType := func(by By) error {
		switch by.(type) {
		case byName, byNamePrefix, byIDPrefix:
			return nil
		default:
			return ErrInvalidFindBy
		}
	}

	secretList := []*api.Secret{}
	appendResult := func(o Object) {
		secretList = append(secretList, o.(secretEntry).Secret)
	}

	err := tx.find(tableSecret, by, checkType, appendResult)
	return secretList, err
}

type secretIndexerByID struct{}

func (ci secretIndexerByID) FromArgs(args ...interface{}) ([]byte, error) {
	return fromArgs(args...)
}

func (ci secretIndexerByID) FromObject(obj interface{}) (bool, []byte, error) {
	s, ok := obj.(secretEntry)
	if !ok {
		panic("unexpected type passed to FromObject")
	}

	// Add the null character as a terminator
	val := s.Secret.ID + "\x00"
	return true, []byte(val), nil
}

func (ci secretIndexerByID) PrefixFromArgs(args ...interface{}) ([]byte, error) {
	return prefixFromArgs(args...)
}

type secretIndexerByName struct{}

func (ci secretIndexerByName) FromArgs(args ...interface{}) ([]byte, error) {
	return fromArgs(args...)
}

func (ci secretIndexerByName) FromObject(obj interface{}) (bool, []byte, error) {
	s, ok := obj.(secretEntry)
	if !ok {
		panic("unexpected type passed to FromObject")
	}

	// Add the null character as a terminator
	return true, []byte(strings.ToLower(s.Spec.Annotations.Name) + "\x00"), nil
}

func (ci secretIndexerByName) PrefixFromArgs(args ...interface{}) ([]byte, error) {
	return prefixFromArgs(args...)
}
