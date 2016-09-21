package storage

import (
	"errors"
	"fmt"
)

var (
	// ErrPathOutsideStore indicates that the returned path would be
	// outside the store
	ErrPathOutsideStore = errors.New("path outside file store")
)

// ErrMetaNotFound indicates we did not find a particular piece
// of metadata in the store
type ErrMetaNotFound struct {
	Resource string
}

func (err ErrMetaNotFound) Error() string {
	return fmt.Sprintf("%s trust data unavailable.  Has a notary repository been initialized?", err.Resource)
}
