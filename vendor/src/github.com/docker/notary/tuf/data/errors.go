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

// ErrMissingMeta - couldn't find the FileMeta object for the given Role, or
// the FileMeta object contained no supported checksums
type ErrMissingMeta struct {
	Role string
}

func (e ErrMissingMeta) Error() string {
	return fmt.Sprintf("no checksums for supported algorithms were provided for %s", e.Role)
}

// ErrInvalidChecksum is the error to be returned when checksum is invalid
type ErrInvalidChecksum struct {
	alg string
}

func (e ErrInvalidChecksum) Error() string {
	return fmt.Sprintf("%s checksum invalid", e.alg)
}

// ErrMismatchedChecksum is the error to be returned when checksum is mismatched
type ErrMismatchedChecksum struct {
	alg      string
	name     string
	expected string
}

func (e ErrMismatchedChecksum) Error() string {
	return fmt.Sprintf("%s checksum for %s did not match: expected %s", e.alg, e.name,
		e.expected)
}
