package container

import "github.com/hashicorp/go-memdb"

const (
	memdbTable   = "containers"
	memdbIDField = "ID"
	memdbIDIndex = "id"
)

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

// MemDB provides an in-memory transactional (ACID) container Store
type MemDB struct {
	store *memdb.MemDB
}

// NewMemDB provides the default implementation, with the default schema
func NewMemDB() (*MemDB, error) {
	store, err := memdb.NewMemDB(schema)
	if err != nil {
		return nil, err
	}
	return &MemDB{store: store}, nil
}

// Snapshot provides a consistent read-only View of the database
func (db *MemDB) Snapshot() *View {
	return &View{db.store.Txn(false)}
}

// Save atomically updates the in-memory store
func (db *MemDB) Save(snapshot *Snapshot) error {
	txn := db.store.Txn(true)
	defer txn.Commit()
	return txn.Insert(memdbTable, snapshot)
}

// Delete removes an item by ID
func (db *MemDB) Delete(id string) error {
	txn := db.store.Txn(true)
	defer txn.Commit()
	return txn.Delete(memdbTable, &Snapshot{ID: id})
}

// View can be used by readers to avoid locking
type View struct {
	txn *memdb.Txn
}

// All returns a all items in this snapshot
func (v *View) All() ([]Snapshot, error) {
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
func (v *View) Get(id string) (*Snapshot, error) {
	s, err := v.txn.First(memdbTable, memdbIDIndex, id)
	if err != nil {
		return nil, err
	}
	snapshot := *(s.(*Snapshot)) // force a copy
	return &snapshot, nil
}
