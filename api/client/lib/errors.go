package lib

import (
	"errors"
	"fmt"
)

var ErrConnectionFailed = errors.New("Cannot connect to the Docker daemon. Is the docker daemon running on this host?")

// imageNotFoundError implements an error returned when an image is not in the docker host.
type imageNotFoundError struct {
	imageID string
}

// Error returns a string representation of an imageNotFoundError
func (i imageNotFoundError) Error() string {
	return fmt.Sprintf("Image not found: %s", i.imageID)
}

// IsImageNotFound returns true if the error is caused
// when an image is not found in the docker host.
func IsErrImageNotFound(err error) bool {
	_, ok := err.(imageNotFoundError)
	return ok
}

// unauthorizedError represents an authorization error in a remote registry.
type unauthorizedError struct {
	cause error
}

// Error returns a string representation of an unauthorizedError
func (u unauthorizedError) Error() string {
	return u.cause.Error()
}

// IsUnauthorized returns true if the error is caused
// when an the remote registry authentication fails
func IsErrUnauthorized(err error) bool {
	_, ok := err.(unauthorizedError)
	return ok
}
