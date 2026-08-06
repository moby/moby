package store

import (
	"strings"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/moby/swarmkit/v2/api"
	"github.com/moby/swarmkit/v2/api/naming"
)

const tableService = "service"

func init() {
	register(ObjectStoreConfig{
		Table: &memdb.TableSchema{
			Name: tableService,
			Indexes: map[string]*memdb.IndexSchema{
				indexID: {
					Name:    indexID,
					Unique:  true,
					Indexer: api.ServiceIndexerByID{},
				},
				indexName: {
					Name:    indexName,
					Unique:  true,
					Indexer: api.ServiceIndexerByName{},
				},
				indexRuntime: {
					Name:         indexRuntime,
					AllowMissing: true,
					Indexer:      serviceIndexerByRuntime{},
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
				indexConfig: {
					Name:         indexConfig,
					AllowMissing: true,
					Indexer:      serviceIndexerByConfig{},
				},
				indexCustom: {
					Name:         indexCustom,
					Indexer:      api.ServiceCustomIndexer{},
					AllowMissing: true,
				},
			},
		},
		Save: func(tx ReadTx, snapshot *api.StoreSnapshot) error {
			var err error
			snapshot.Services, err = FindServices(tx, All)
			return err
		},
		Restore: func(tx Tx, snapshot *api.StoreSnapshot) error {
			toStoreObj := make([]api.StoreObject, len(snapshot.Services))
			for i, x := range snapshot.Services {
				toStoreObj[i] = x
			}
			return RestoreTable(tx, tableService, toStoreObj)
		},
		ApplyStoreAction: func(tx Tx, sa *api.StoreAction) error {
			switch v := sa.Target.(type) {
			case *api.StoreAction_Service:
				obj := v.Service
				switch sa.Action {
				case api.StoreActionKind_STORE_ACTION_CREATE:
					return CreateService(tx, obj)
				case api.StoreActionKind_STORE_ACTION_UPDATE:
					return UpdateService(tx, obj)
				case api.StoreActionKind_STORE_ACTION_REMOVE:
					return DeleteService(tx, obj.Id)
				}
			}
			return errUnknownStoreAction
		},
	})
}

// CreateService adds a new service to the store.
// Returns ErrExist if the ID is already taken.
func CreateService(tx Tx, s *api.Service) error {
	// Ensure the name is not already in use.
	if tx.lookup(tableService, indexName, strings.ToLower(s.GetSpec().GetAnnotations().GetName())) != nil {
		return ErrNameConflict
	}

	return tx.create(tableService, s)
}

// UpdateService updates an existing service in the store.
// Returns ErrNotExist if the service doesn't exist.
func UpdateService(tx Tx, s *api.Service) error {
	// Ensure the name is either not in use or already used by this same Service.
	if existing := tx.lookup(tableService, indexName, strings.ToLower(s.GetSpec().GetAnnotations().GetName())); existing != nil {
		if existing.GetId() != s.Id {
			return ErrNameConflict
		}
	}

	return tx.update(tableService, s)
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
	return s.(*api.Service)
}

// FindServices selects a set of services and returns them.
func FindServices(tx ReadTx, by By) ([]*api.Service, error) {
	checkType := func(by By) error {
		switch by.(type) {
		case byName, byNamePrefix, byIDPrefix, byRuntime, byReferencedNetworkID, byReferencedSecretID, byReferencedConfigID, byCustom, byCustomPrefix, byAll:
			return nil
		default:
			return ErrInvalidFindBy
		}
	}

	serviceList := []*api.Service{}
	appendResult := func(o api.StoreObject) {
		serviceList = append(serviceList, o.(*api.Service))
	}

	err := tx.find(tableService, by, checkType, appendResult)
	return serviceList, err
}

type serviceIndexerByRuntime struct{}

func (si serviceIndexerByRuntime) FromArgs(args ...any) ([]byte, error) {
	return fromArgs(args...)
}

func (si serviceIndexerByRuntime) FromObject(obj any) (bool, []byte, error) {
	s := obj.(*api.Service)
	r, err := naming.Runtime(s.GetSpec().GetTask())
	if err != nil {
		return false, nil, nil
	}
	return true, []byte(r + "\x00"), nil
}

func (si serviceIndexerByRuntime) PrefixFromArgs(args ...any) ([]byte, error) {
	return prefixFromArgs(args...)
}

type serviceIndexerByNetwork struct{}

func (si serviceIndexerByNetwork) FromArgs(args ...any) ([]byte, error) {
	return fromArgs(args...)
}

func (si serviceIndexerByNetwork) FromObject(obj any) (bool, [][]byte, error) {
	s := obj.(*api.Service)

	var networkIDs [][]byte

	specNetworks := s.GetSpec().GetTask().GetNetworks()

	if len(specNetworks) == 0 {
		specNetworks = s.GetSpec().GetNetworks()
	}

	for _, na := range specNetworks {
		// Add the null character as a terminator
		networkIDs = append(networkIDs, []byte(na.Target+"\x00"))
	}

	return len(networkIDs) != 0, networkIDs, nil
}

type serviceIndexerBySecret struct{}

func (si serviceIndexerBySecret) FromArgs(args ...any) ([]byte, error) {
	return fromArgs(args...)
}

func (si serviceIndexerBySecret) FromObject(obj any) (bool, [][]byte, error) {
	s := obj.(*api.Service)

	container := s.GetSpec().GetTask().GetContainer()
	if container == nil {
		return false, nil, nil
	}

	var secretIDs [][]byte

	for _, secretRef := range container.Secrets {
		// Add the null character as a terminator
		secretIDs = append(secretIDs, []byte(secretRef.SecretId+"\x00"))
	}

	return len(secretIDs) != 0, secretIDs, nil
}

type serviceIndexerByConfig struct{}

func (si serviceIndexerByConfig) FromArgs(args ...any) ([]byte, error) {
	return fromArgs(args...)
}

func (si serviceIndexerByConfig) FromObject(obj any) (bool, [][]byte, error) {
	s, ok := obj.(*api.Service)
	if !ok {
		panic("unexpected type passed to FromObject")
	}

	container := s.GetSpec().GetTask().GetContainer()
	if container == nil {
		return false, nil, nil
	}

	var configIDs [][]byte

	for _, configRef := range container.Configs {
		// Add the null character as a terminator
		configIDs = append(configIDs, []byte(configRef.ConfigId+"\x00"))
	}

	return len(configIDs) != 0, configIDs, nil
}
