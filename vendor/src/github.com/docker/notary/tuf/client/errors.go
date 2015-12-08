package client

import (
	"fmt"
)

// ErrChecksumMismatch - a checksum failed verification
type ErrChecksumMismatch struct {
	role string
}

func (e ErrChecksumMismatch) Error() string {
	return fmt.Sprintf("tuf: checksum for %s did not match", e.role)
}

// ErrMissingMeta - couldn't find the FileMeta object for a role or target
type ErrMissingMeta struct {
	role string
}

func (e ErrMissingMeta) Error() string {
	return fmt.Sprintf("tuf: sha256 checksum required for %s", e.role)
}

// ErrCorruptedCache - local data is incorrect
type ErrCorruptedCache struct {
	file string
}

func (e ErrCorruptedCache) Error() string {
	return fmt.Sprintf("cache is corrupted: %s", e.file)
}
