package service // import "github.com/docker/docker/volume/service"

import (
	"fmt"
	"strings"
)

const (
	// errVolumeInUse is a typed error returned when trying to remove a volume that is currently in use by a container
	errVolumeInUse conflictError = "volume is in use"
	// errNoSuchVolume is a typed error returned if the requested volume doesn't exist in the volume store
	errNoSuchVolume notFoundError = "no such volume"
	// errNameConflict is a typed error returned on create when a volume exists with the given name, but for a different driver
	errNameConflict conflictError = "volume name must be unique"
)

type conflictError string

func (e conflictError) Error() string {
	return string(e)
}
func (conflictError) Conflict() {}

type notFoundError string

func (e notFoundError) Error() string {
	return string(e)
}

func (notFoundError) NotFound() {}

// OpErr is the error type returned by functions in the store package. It describes
// the operation, volume name, and error.
type OpErr struct {
	// Err is the error that occurred during the operation.
	Err error
	// Op is the operation which caused the error, such as "create", or "list".
	Op string
	// Name is the name of the resource being requested for this op, typically the volume name or the driver name.
	Name string
	// Refs is the list of references associated with the resource.
	Refs []string
}

// Error satisfies the built-in error interface type.
func (e *OpErr) Error() string {
	if e == nil {
		return "<nil>"
	}
	s := e.Op
	if e.Name != "" {
		s = s + " " + e.Name
	}

	s = s + ": " + e.Err.Error()
	if len(e.Refs) > 0 {
		s = s + " - " + "[" + strings.Join(e.Refs, ", ") + "]"
	}
	return s
}

// Cause returns the error the caused this error
func (e *OpErr) Cause() error {
	return e.Err
}

// IsInUse returns a boolean indicating whether the error indicates that a
// volume is in use
func IsInUse(err error) bool {
	return isErr(err, errVolumeInUse)
}

// IsNotExist returns a boolean indicating whether the error indicates that the volume does not exist
func IsNotExist(err error) bool {
	return isErr(err, errNoSuchVolume)
}

// IsNameConflict returns a boolean indicating whether the error indicates that a
// volume name is already taken
func IsNameConflict(err error) bool {
	return isErr(err, errNameConflict)
}

type causal interface {
	Cause() error
}

func isErr(err error, expected error) bool {
	switch pe := err.(type) {
	case nil:
		return false
	case causal:
		return isErr(pe.Cause(), expected)
	}
	return err == expected
}

type invalidFilter struct {
	filter string
	value  interface{}
}

func (e invalidFilter) Error() string {
	msg := "Invalid filter '" + e.filter
	if e.value != nil {
		msg += fmt.Sprintf("=%s", e.value)
	}
	return msg + "'"
}

func (e invalidFilter) InvalidParameter() {}
