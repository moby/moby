package store

import (
	"strings"

	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/manager/state"
	memdb "github.com/hashicorp/go-memdb"
)

const tableService = "service"

func init() {
	register(ObjectStoreConfig{
		Name: tableService,
		Table: &memdb.TableSchema{
			Name: tableService,
			Indexes: map[string]*memdb.IndexSchema{
				indexID: {
					Name:    indexID,
					Unique:  true,
					Indexer: serviceIndexerByID{},
				},
				indexName: {
					Name:    indexName,
					Unique:  true,
					Indexer: serviceIndexerByName{},
				},
				indexNetwork: {
					Name:         indexNetwork,
					AllowMissing: true,
					Indexer:      serviceIndexerByNetwork{},
				},
				indexSecret: {
					Name:         indexSecret,
					AllowMissing: true,
					Indexer:      serviceIndexerBySecret{},
				},
			},
		},
		Save: func(tx ReadTx, snapshot *api.StoreSnapshot) error {
			var err error
			snapshot.Services, err = FindServices(tx, All)
			return err
		},
		Restore: func(tx Tx, snapshot *api.StoreSnapshot) error {
			services, err := FindServices(tx, All)
			if err != nil {
				return err
			}
			for _, s := range services {
				if err := DeleteService(tx, s.ID); err != nil {
					return err
				}
			}
			for _, s := range snapshot.Services {
				if err := CreateService(tx, s); err != nil {
					return err
				}
			}
			return nil
		},
		ApplyStoreAction: func(tx Tx, sa *api.StoreAction) error {
			switch v := sa.Target.(type) {
			case *api.StoreAction_Service:
				obj := v.Service
				switch sa.Action {
				case api.StoreActionKindCreate:
					return CreateService(tx, obj)
				case api.StoreActionKindUpdate:
					return UpdateService(tx, obj)
				case api.StoreActionKindRemove:
					return DeleteService(tx, obj.ID)
				}
			}
			return errUnknownStoreAction
		},
		NewStoreAction: func(c state.Event) (api.StoreAction, error) {
			var sa api.StoreAction
			switch v := c.(type) {
			case state.EventCreateService:
				sa.Action = api.StoreActionKindCreate
				sa.Target = &api.StoreAction_Service{
					Service: v.Service,
				}
			case state.EventUpdateService:
				sa.Action = api.StoreActionKindUpdate
				sa.Target = &api.StoreAction_Service{
					Service: v.Service,
				}
			case state.EventDeleteService:
				sa.Action = api.StoreActionKindRemove
				sa.Target = &api.StoreAction_Service{
					Service: v.Service,
				}
			default:
				return api.StoreAction{}, errUnknownStoreAction
			}
			return sa, nil
		},
	})
}

type serviceEntry struct {
	*api.Service
}

func (s serviceEntry) ID() string {
	return s.Service.ID
}

func (s serviceEntry) Meta() api.Meta {
	return s.Service.Meta
}

func (s serviceEntry) SetMeta(meta api.Meta) {
	s.Service.Meta = meta
}

func (s serviceEntry) Copy() Object {
	return serviceEntry{s.Service.Copy()}
}

func (s serviceEntry) EventCreate() state.Event {
	return state.EventCreateService{Service: s.Service}
}

func (s serviceEntry) EventUpdate() state.Event {
	return state.EventUpdateService{Service: s.Service}
}

func (s serviceEntry) EventDelete() state.Event {
	return state.EventDeleteService{Service: s.Service}
}

// CreateService adds a new service to the store.
// Returns ErrExist if the ID is already taken.
func CreateService(tx Tx, s *api.Service) error {
	// Ensure the name is not already in use.
	if tx.lookup(tableService, indexName, strings.ToLower(s.Spec.Annotations.Name)) != nil {
		return ErrNameConflict
	}

	return tx.create(tableService, serviceEntry{s})
}

// UpdateService updates an existing service in the store.
// Returns ErrNotExist if the service doesn't exist.
func UpdateService(tx Tx, s *api.Service) error {
	// Ensure the name is either not in use or already used by this same Service.
	if existing := tx.lookup(tableService, indexName, strings.ToLower(s.Spec.Annotations.Name)); existing != nil {
		if existing.ID() != s.ID {
			return ErrNameConflict
		}
	}

	return tx.update(tableService, serviceEntry{s})
}

// DeleteService removes a service from the store.
// Returns ErrNotExist if the service doesn't exist.
func DeleteService(tx Tx, id string) error {
	return tx.delete(tableService, id)
}

// GetService looks up a service by ID.
// Returns nil if the service doesn't exist.
func GetService(tx ReadTx, id string) *api.Service {
	s := tx.get(tableService, id)
	if s == nil {
		return nil
	}
	return s.(serviceEntry).Service
}

// FindServices selects a set of services and returns them.
func FindServices(tx ReadTx, by By) ([]*api.Service, error) {
	checkType := func(by By) error {
		switch by.(type) {
		case byName, byNamePrefix, byIDPrefix, byReferencedNetworkID, byReferencedSecretID:
			return nil
		default:
			return ErrInvalidFindBy
		}
	}

	serviceList := []*api.Service{}
	appendResult := func(o Object) {
		serviceList = append(serviceList, o.(serviceEntry).Service)
	}

	err := tx.find(tableService, by, checkType, appendResult)
	return serviceList, err
}

type serviceIndexerByID struct{}

func (si serviceIndexerByID) FromArgs(args ...interface{}) ([]byte, error) {
	return fromArgs(args...)
}

func (si serviceIndexerByID) FromObject(obj interface{}) (bool, []byte, error) {
	s, ok := obj.(serviceEntry)
	if !ok {
		panic("unexpected type passed to FromObject")
	}

	// Add the null character as a terminator
	val := s.Service.ID + "\x00"
	return true, []byte(val), nil
}

func (si serviceIndexerByID) PrefixFromArgs(args ...interface{}) ([]byte, error) {
	return prefixFromArgs(args...)
}

type serviceIndexerByName struct{}

func (si serviceIndexerByName) FromArgs(args ...interface{}) ([]byte, error) {
	return fromArgs(args...)
}

func (si serviceIndexerByName) FromObject(obj interface{}) (bool, []byte, error) {
	s, ok := obj.(serviceEntry)
	if !ok {
		panic("unexpected type passed to FromObject")
	}

	// Add the null character as a terminator
	return true, []byte(strings.ToLower(s.Spec.Annotations.Name) + "\x00"), nil
}

func (si serviceIndexerByName) PrefixFromArgs(args ...interface{}) ([]byte, error) {
	return prefixFromArgs(args...)
}

type serviceIndexerByNetwork struct{}

func (si serviceIndexerByNetwork) FromArgs(args ...interface{}) ([]byte, error) {
	return fromArgs(args...)
}

func (si serviceIndexerByNetwork) FromObject(obj interface{}) (bool, [][]byte, error) {
	s, ok := obj.(serviceEntry)
	if !ok {
		panic("unexpected type passed to FromObject")
	}

	var networkIDs [][]byte

	specNetworks := s.Spec.Task.Networks

	if len(specNetworks) == 0 {
		specNetworks = s.Spec.Networks
	}

	for _, na := range specNetworks {
		// Add the null character as a terminator
		networkIDs = append(networkIDs, []byte(na.Target+"\x00"))
	}

	return len(networkIDs) != 0, networkIDs, nil
}

type serviceIndexerBySecret struct{}

func (si serviceIndexerBySecret) FromArgs(args ...interface{}) ([]byte, error) {
	return fromArgs(args...)
}

func (si serviceIndexerBySecret) FromObject(obj interface{}) (bool, [][]byte, error) {
	s, ok := obj.(serviceEntry)
	if !ok {
		panic("unexpected type passed to FromObject")
	}

	container := s.Spec.Task.GetContainer()
	if container == nil {
		return false, nil, nil
	}

	var secretIDs [][]byte

	for _, secretRef := range container.Secrets {
		// Add the null character as a terminator
		secretIDs = append(secretIDs, []byte(secretRef.SecretID+"\x00"))
	}

	return len(secretIDs) != 0, secretIDs, nil
}
