package data

import "fmt"

// ErrInvalidMetadata is the error to be returned when metadata is invalid
type ErrInvalidMetadata struct {
	role string
	msg  string
}

func (e ErrInvalidMetadata) Error() string {
	return fmt.Sprintf("%s type metadata invalid: %s", e.role, e.msg)
}

// ErrMissingMeta - couldn't find the FileMeta object for a role or target
type ErrMissingMeta struct {
	Role string
}

func (e ErrMissingMeta) Error() string {
	return fmt.Sprintf("tuf: sha256 checksum required for %s", e.Role)
}
