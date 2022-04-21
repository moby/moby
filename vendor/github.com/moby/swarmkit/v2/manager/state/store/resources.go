package store

import (
	"strings"

	"github.com/moby/swarmkit/v2/api"
	memdb "github.com/hashicorp/go-memdb"
	"github.com/pkg/errors"
)

const tableResource = "resource"

var (
	// ErrNoKind is returned by resource create operations if the provided Kind
	// of the resource does not exist
	ErrNoKind = errors.New("object kind is unregistered")
)

func init() {
	register(ObjectStoreConfig{
		Table: &memdb.TableSchema{
			Name: tableResource,
			Indexes: map[string]*memdb.IndexSchema{
				indexID: {
					Name:    indexID,
					Unique:  true,
					Indexer: resourceIndexerByID{},
				},
				indexName: {
					Name:    indexName,
					Unique:  true,
					Indexer: resourceIndexerByName{},
				},
				indexKind: {
					Name:    indexKind,
					Indexer: resourceIndexerByKind{},
				},
				indexCustom: {
					Name:         indexCustom,
					Indexer:      resourceCustomIndexer{},
					AllowMissing: true,
				},
			},
		},
		Save: func(tx ReadTx, snapshot *api.StoreSnapshot) error {
			var err error
			snapshot.Resources, err = FindResources(tx, All)
			return err
		},
		Restore: func(tx Tx, snapshot *api.StoreSnapshot) error {
			toStoreObj := make([]api.StoreObject, len(snapshot.Resources))
			for i, x := range snapshot.Resources {
				toStoreObj[i] = resourceEntry{x}
			}
			return RestoreTable(tx, tableResource, toStoreObj)
		},
		ApplyStoreAction: func(tx Tx, sa api.StoreAction) error {
			switch v := sa.Target.(type) {
			case *api.StoreAction_Resource:
				obj := v.Resource
				switch sa.Action {
				case api.StoreActionKindCreate:
					return CreateResource(tx, obj)
				case api.StoreActionKindUpdate:
					return UpdateResource(tx, obj)
				case api.StoreActionKindRemove:
					return DeleteResource(tx, obj.ID)
				}
			}
			return errUnknownStoreAction
		},
	})
}

type resourceEntry struct {
	*api.Resource
}

func (r resourceEntry) CopyStoreObject() api.StoreObject {
	return resourceEntry{Resource: r.Resource.Copy()}
}

// ensure that when update events are emitted, we unwrap resourceEntry
func (r resourceEntry) EventUpdate(oldObject api.StoreObject) api.Event {
	if oldObject != nil {
		return api.EventUpdateResource{Resource: r.Resource, OldResource: oldObject.(resourceEntry).Resource}
	}
	return api.EventUpdateResource{Resource: r.Resource}
}

func confirmExtension(tx Tx, r *api.Resource) error {
	// There must be an extension corresponding to the Kind field.
	extensions, err := FindExtensions(tx, ByName(r.Kind))
	if err != nil {
		return errors.Wrap(err, "failed to query extensions")
	}
	if len(extensions) == 0 {
		return ErrNoKind
	}
	return nil
}

// CreateResource adds a new resource object to the store.
// Returns ErrExist if the ID is already taken.
// Returns ErrNameConflict if a Resource with this Name already exists
// Returns ErrNoKind if the specified Kind does not exist
func CreateResource(tx Tx, r *api.Resource) error {
	if err := confirmExtension(tx, r); err != nil {
		return err
	}
	// TODO(dperny): currently the "name" index is unique, which means only one
	// Resource of _any_ Kind can exist with that name. This isn't a problem
	// right now, but the ideal case would be for names to be namespaced to the
	// kind.
	if tx.lookup(tableResource, indexName, strings.ToLower(r.Annotations.Name)) != nil {
		return ErrNameConflict
	}
	return tx.create(tableResource, resourceEntry{r})
}

// UpdateResource updates an existing resource object in the store.
// Returns ErrNotExist if the object doesn't exist.
func UpdateResource(tx Tx, r *api.Resource) error {
	if err := confirmExtension(tx, r); err != nil {
		return err
	}
	return tx.update(tableResource, resourceEntry{r})
}

// DeleteResource removes a resource object from the store.
// Returns ErrNotExist if the object doesn't exist.
func DeleteResource(tx Tx, id string) error {
	return tx.delete(tableResource, id)
}

// GetResource looks up a resource object by ID.
// Returns nil if the object doesn't exist.
func GetResource(tx ReadTx, id string) *api.Resource {
	r := tx.get(tableResource, id)
	if r == nil {
		return nil
	}
	return r.(resourceEntry).Resource
}

// FindResources selects a set of resource objects and returns them.
func FindResources(tx ReadTx, by By) ([]*api.Resource, error) {
	checkType := func(by By) error {
		switch by.(type) {
		case byIDPrefix, byName, byNamePrefix, byKind, byCustom, byCustomPrefix:
			return nil
		default:
			return ErrInvalidFindBy
		}
	}

	resourceList := []*api.Resource{}
	appendResult := func(o api.StoreObject) {
		resourceList = append(resourceList, o.(resourceEntry).Resource)
	}

	err := tx.find(tableResource, by, checkType, appendResult)
	return resourceList, err
}

type resourceIndexerByKind struct{}

func (ri resourceIndexerByKind) FromArgs(args ...interface{}) ([]byte, error) {
	return fromArgs(args...)
}

func (ri resourceIndexerByKind) FromObject(obj interface{}) (bool, []byte, error) {
	r := obj.(resourceEntry)

	// Add the null character as a terminator
	val := r.Resource.Kind + "\x00"
	return true, []byte(val), nil
}

type resourceIndexerByID struct{}

func (indexer resourceIndexerByID) FromArgs(args ...interface{}) ([]byte, error) {
	return api.ResourceIndexerByID{}.FromArgs(args...)
}
func (indexer resourceIndexerByID) PrefixFromArgs(args ...interface{}) ([]byte, error) {
	return api.ResourceIndexerByID{}.PrefixFromArgs(args...)
}
func (indexer resourceIndexerByID) FromObject(obj interface{}) (bool, []byte, error) {
	return api.ResourceIndexerByID{}.FromObject(obj.(resourceEntry).Resource)
}

type resourceIndexerByName struct{}

func (indexer resourceIndexerByName) FromArgs(args ...interface{}) ([]byte, error) {
	return api.ResourceIndexerByName{}.FromArgs(args...)
}
func (indexer resourceIndexerByName) PrefixFromArgs(args ...interface{}) ([]byte, error) {
	return api.ResourceIndexerByName{}.PrefixFromArgs(args...)
}
func (indexer resourceIndexerByName) FromObject(obj interface{}) (bool, []byte, error) {
	return api.ResourceIndexerByName{}.FromObject(obj.(resourceEntry).Resource)
}

type resourceCustomIndexer struct{}

func (indexer resourceCustomIndexer) FromArgs(args ...interface{}) ([]byte, error) {
	return api.ResourceCustomIndexer{}.FromArgs(args...)
}
func (indexer resourceCustomIndexer) PrefixFromArgs(args ...interface{}) ([]byte, error) {
	return api.ResourceCustomIndexer{}.PrefixFromArgs(args...)
}
func (indexer resourceCustomIndexer) FromObject(obj interface{}) (bool, [][]byte, error) {
	return api.ResourceCustomIndexer{}.FromObject(obj.(resourceEntry).Resource)
}
