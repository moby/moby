package store

import "fmt"

// ErrMetaNotFound indicates we did not find a particular piece
// of metadata in the store
type ErrMetaNotFound struct {
	Resource string
}

func (err ErrMetaNotFound) Error() string {
	return fmt.Sprintf("%s trust data unavailable.  Has a notary repository been initialized?", err.Resource)
}
