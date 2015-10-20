package store

import "errors"

var (
	// errVolumeInUse is a typed error returned when trying to remove a volume that is currently in use by a container
	errVolumeInUse = errors.New("volume is in use")
	// errNoSuchVolume is a typed error returned if the requested volume doesn't exist in the volume store
	errNoSuchVolume = errors.New("no such volume")
	// errInvalidName is a typed error returned when creating a volume with a name that is not valid on the platform
	errInvalidName = errors.New("volume name is not valid on this platform")
)

// OpErr is the error type returned by functions in the store package. It describes
// the operation, volume name, and error.
type OpErr struct {
	// Err is the error that occurred during the operation.
	Err error
	// Op is the operation which caused the error, such as "create", or "list".
	Op string
	// Name is the name of the resource being requested for this op, typically the volume name or the driver name.
	Name string
}

// Error satifies the built-in error interface type.
func (e *OpErr) Error() string {
	if e == nil {
		return "<nil>"
	}
	s := e.Op
	if e.Name != "" {
		s = s + " " + e.Name
	}

	s = s + ": " + e.Err.Error()
	return s
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

func isErr(err error, expected error) bool {
	switch pe := err.(type) {
	case nil:
		return false
	case *OpErr:
		err = pe.Err
	}
	return err == expected
}
