package store

import "fmt"

// ErrMetaNotFound indicates we did not find a particular piece
// of metadata in the store
type ErrMetaNotFound struct {
	Role string
}

func (err ErrMetaNotFound) Error() string {
	return fmt.Sprintf("%s trust data unavailable", err.Role)
}
