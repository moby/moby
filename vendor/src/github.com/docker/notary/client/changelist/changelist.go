package changelist

// memChangeList implements a simple in memory change list.
type memChangelist struct {
	changes []Change
}

// NewMemChangelist instantiates a new in-memory changelist
func NewMemChangelist() Changelist {
	return &memChangelist{}
}

// List returns a list of Changes
func (cl memChangelist) List() []Change {
	return cl.changes
}

// Add adds a change to the in-memory change list
func (cl *memChangelist) Add(c Change) error {
	cl.changes = append(cl.changes, c)
	return nil
}

// Clear empties the changelist file.
func (cl *memChangelist) Clear(archive string) error {
	// appending to a nil list initializes it.
	cl.changes = nil
	return nil
}

// Close is a no-op in this in-memory change-list
func (cl *memChangelist) Close() error {
	return nil
}

func (cl *memChangelist) NewIterator() (ChangeIterator, error) {
	return &MemChangeListIterator{index: 0, collection: cl.changes}, nil
}

// MemChangeListIterator is a concrete instance of ChangeIterator
type MemChangeListIterator struct {
	index      int
	collection []Change // Same type as memChangeList.changes
}

// Next returns the next Change
func (m *MemChangeListIterator) Next() (item Change, err error) {
	if m.index >= len(m.collection) {
		return nil, IteratorBoundsError(m.index)
	}
	item = m.collection[m.index]
	m.index++
	return item, err
}

// HasNext indicates whether the iterator is exhausted
func (m *MemChangeListIterator) HasNext() bool {
	return m.index < len(m.collection)
}
