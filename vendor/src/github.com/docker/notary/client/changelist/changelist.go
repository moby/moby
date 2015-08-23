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
