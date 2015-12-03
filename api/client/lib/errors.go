package lib

import "fmt"

// imageNotFoundError implements an error returned when an image is not in the docker host.
type imageNotFoundError struct {
	imageID string
}

// Error returns a string representation of an imageNotFoundError
func (i imageNotFoundError) Error() string {
	return fmt.Sprintf("Image not found: %s", i.imageID)
}

// ImageNotFound returns the ID of the image not found on the docker host.
func (i imageNotFoundError) ImageIDNotFound() string {
	return i.imageID
}

// ImageNotFound is an interface that describes errors caused
// when an image is not found in the docker host.
type ImageNotFound interface {
	ImageIDNotFound() string
}

// IsImageNotFound returns true when the error is caused
// when an image is not found in the docker host.
func IsErrImageNotFound(err error) bool {
	_, ok := err.(ImageNotFound)
	return ok
}
