package memdb

import "fmt"

// DBSchema is the schema to use for the full database with a MemDB instance.
//
// MemDB will require a valid schema. Schema validation can be tested using
// the Validate function. Calling this function is recommended in unit tests.
type DBSchema struct {
	// Tables is the set of tables within this database. The key is the
	// table name and must match the Name in TableSchema.
	Tables map[string]*TableSchema
}

// Validate validates the schema.
func (s *DBSchema) Validate() error {
	if s == nil {
		return fmt.Errorf("schema is nil")
	}

	if len(s.Tables) == 0 {
		return fmt.Errorf("schema has no tables defined")
	}

	for name, table := range s.Tables {
		if name != table.Name {
			return fmt.Errorf("table name mis-match for '%s'", name)
		}

		if err := table.Validate(); err != nil {
			return fmt.Errorf("table %q: %s", name, err)
		}
	}

	return nil
}

// TableSchema is the schema for a single table.
type TableSchema struct {
	// Name of the table. This must match the key in the Tables map in DBSchema.
	Name string

	// Indexes is the set of indexes for querying this table. The key
	// is a unique name for the index and must match the Name in the
	// IndexSchema.
	Indexes map[string]*IndexSchema
}

// Validate is used to validate the table schema
func (s *TableSchema) Validate() error {
	if s.Name == "" {
		return fmt.Errorf("missing table name")
	}

	if len(s.Indexes) == 0 {
		return fmt.Errorf("missing table indexes for '%s'", s.Name)
	}

	if _, ok := s.Indexes["id"]; !ok {
		return fmt.Errorf("must have id index")
	}

	if !s.Indexes["id"].Unique {
		return fmt.Errorf("id index must be unique")
	}

	if _, ok := s.Indexes["id"].Indexer.(SingleIndexer); !ok {
		return fmt.Errorf("id index must be a SingleIndexer")
	}

	for name, index := range s.Indexes {
		if name != index.Name {
			return fmt.Errorf("index name mis-match for '%s'", name)
		}

		if err := index.Validate(); err != nil {
			return fmt.Errorf("index %q: %s", name, err)
		}
	}

	return nil
}

// IndexSchema is the schema for an index. An index defines how a table is
// queried.
type IndexSchema struct {
	// Name of the index. This must be unique among a tables set of indexes.
	// This must match the key in the map of Indexes for a TableSchema.
	Name string

	// AllowMissing if true ignores this index if it doesn't produce a
	// value. For example, an index that extracts a field that doesn't
	// exist from a structure.
	AllowMissing bool

	Unique  bool
	Indexer Indexer
}

func (s *IndexSchema) Validate() error {
	if s.Name == "" {
		return fmt.Errorf("missing index name")
	}
	if s.Indexer == nil {
		return fmt.Errorf("missing index function for '%s'", s.Name)
	}
	switch s.Indexer.(type) {
	case SingleIndexer:
	case MultiIndexer:
	default:
		return fmt.Errorf("indexer for '%s' must be a SingleIndexer or MultiIndexer", s.Name)
	}
	return nil
}
