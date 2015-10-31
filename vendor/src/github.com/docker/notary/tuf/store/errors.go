package store

// ErrMetaNotFound indicates we did not find a particular piece
// of metadata in the store
type ErrMetaNotFound struct{}

func (err ErrMetaNotFound) Error() string {
	return "no trust data available"
}
