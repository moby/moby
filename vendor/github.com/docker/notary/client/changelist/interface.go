package changelist

// Changelist is the interface for all TUF change lists
type Changelist interface {
	// List returns the ordered list of changes
	// currently stored
	List() []Change

	// Add change appends the provided change to
	// the list of changes
	Add(Change) error

	// Clear empties the current change list.
	// Archive may be provided as a directory path
	// to save a copy of the changelist in that location
	Clear(archive string) error

	// Remove deletes the changes corresponding with the indices given
	Remove(idxs []int) error

	// Close syncronizes any pending writes to the underlying
	// storage and closes the file/connection
	Close() error

	// NewIterator returns an iterator for walking through the list
	// of changes currently stored
	NewIterator() (ChangeIterator, error)
}

const (
	// ActionCreate represents a Create action
	ActionCreate = "create"
	// ActionUpdate represents an Update action
	ActionUpdate = "update"
	// ActionDelete represents a Delete action
	ActionDelete = "delete"
)

// Change is the interface for a TUF Change
type Change interface {
	// "create","update", or "delete"
	Action() string

	// Where the change should be made.
	// For TUF this will be the role
	Scope() string

	// The content type being affected.
	// For TUF this will be "target", or "delegation".
	// If the type is "delegation", the Scope will be
	// used to determine if a root role is being updated
	// or a target delegation.
	Type() string

	// Path indicates the entry within a role to be affected by the
	// change. For targets, this is simply the target's path,
	// for delegations it's the delegated role name.
	Path() string

	// Serialized content that the interpreter of a changelist
	// can use to apply the change.
	// For TUF this will be the serialized JSON that needs
	// to be inserted or merged. In the case of a "delete"
	// action, it will be nil.
	Content() []byte
}

// ChangeIterator is the interface for iterating across collections of
// TUF Change items
type ChangeIterator interface {
	Next() (Change, error)
	HasNext() bool
}
