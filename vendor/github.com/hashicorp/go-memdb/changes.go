package memdb

// Changes describes a set of mutations to memDB tables performed during a
// transaction.
type Changes []Change

// Change describes a mutation to an object in a table.
type Change struct {
	Table  string
	Before interface{}
	After  interface{}

	// primaryKey stores the raw key value from the primary index so that we can
	// de-duplicate multiple updates of the same object in the same transaction
	// but we don't expose this implementation detail to the consumer.
	primaryKey []byte
}

// Created returns true if the mutation describes a new object being inserted.
func (m *Change) Created() bool {
	return m.Before == nil && m.After != nil
}

// Updated returns true if the mutation describes an existing object being
// updated.
func (m *Change) Updated() bool {
	return m.Before != nil && m.After != nil
}

// Deleted returns true if the mutation describes an existing object being
// deleted.
func (m *Change) Deleted() bool {
	return m.Before != nil && m.After == nil
}
