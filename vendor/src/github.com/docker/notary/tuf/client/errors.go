package client

import (
	"fmt"
)

// ErrCorruptedCache - local data is incorrect
type ErrCorruptedCache struct {
	file string
}

func (e ErrCorruptedCache) Error() string {
	return fmt.Sprintf("cache is corrupted: %s", e.file)
}
