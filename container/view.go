package container

import "github.com/hashicorp/go-memdb"

const (
	memdbTable   = "containers"
	memdbIDField = "ID"
	memdbIDIndex = "id"
)

// ViewDB provides an in-memory transactional (ACID) container Store
type ViewDB interface {
	Snapshot() View
	Save(snapshot *Snapshot) error
	Delete(id string) error
}

// View can be used by readers to avoid locking
type View interface {
	All() ([]Snapshot, error)
	Get(id string) (*Snapshot, error)
}

var schema = &memdb.DBSchema{
	Tables: map[string]*memdb.TableSchema{
		memdbTable: {
			Name: memdbTable,
			Indexes: map[string]*memdb.IndexSchema{
				memdbIDIndex: {
					Name:    memdbIDIndex,
					Unique:  true,
					Indexer: &memdb.StringFieldIndex{Field: memdbIDField},
				},
			},
		},
	},
}

type memDB struct {
	store *memdb.MemDB
}

// NewViewDB provides the default implementation, with the default schema
func NewViewDB() (ViewDB, error) {
	store, err := memdb.NewMemDB(schema)
	if err != nil {
		return nil, err
	}
	return &memDB{store: store}, nil
}

// Snapshot provides a consistent read-only View of the database
func (db *memDB) Snapshot() View {
	return &memdbView{db.store.Txn(false)}
}

// Save atomically updates the in-memory store
func (db *memDB) Save(snapshot *Snapshot) error {
	txn := db.store.Txn(true)
	defer txn.Commit()
	return txn.Insert(memdbTable, snapshot)
}

// Delete removes an item by ID
func (db *memDB) Delete(id string) error {
	txn := db.store.Txn(true)
	defer txn.Commit()
	return txn.Delete(memdbTable, &Snapshot{ID: id})
}

type memdbView struct {
	txn *memdb.Txn
}

// All returns a all items in this snapshot
func (v *memdbView) All() ([]Snapshot, error) {
	var all []Snapshot
	iter, err := v.txn.Get(memdbTable, memdbIDIndex)
	if err != nil {
		return nil, err
	}
	for {
		item := iter.Next()
		if item == nil {
			break
		}
		snapshot := *(item.(*Snapshot)) // force a copy
		all = append(all, snapshot)
	}
	return all, nil
}

//Get returns an item by id
func (v *memdbView) Get(id string) (*Snapshot, error) {
	s, err := v.txn.First(memdbTable, memdbIDIndex, id)
	if err != nil {
		return nil, err
	}
	snapshot := *(s.(*Snapshot)) // force a copy
	return &snapshot, nil
}
