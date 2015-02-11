package client

import (
	"fmt"

	"github.com/docker/distribution/digest"
)

// RepositoryNotFoundError is returned when making an operation against a
// repository that does not exist in the registry.
type RepositoryNotFoundError struct {
	Name string
}

func (e *RepositoryNotFoundError) Error() string {
	return fmt.Sprintf("No repository found with Name: %s", e.Name)
}

// ImageManifestNotFoundError is returned when making an operation against a
// given image manifest that does not exist in the registry.
type ImageManifestNotFoundError struct {
	Name string
	Tag  string
}

func (e *ImageManifestNotFoundError) Error() string {
	return fmt.Sprintf("No manifest found with Name: %s, Tag: %s",
		e.Name, e.Tag)
}

// BlobNotFoundError is returned when making an operation against a given image
// layer that does not exist in the registry.
type BlobNotFoundError struct {
	Name   string
	Digest digest.Digest
}

func (e *BlobNotFoundError) Error() string {
	return fmt.Sprintf("No blob found with Name: %s, Digest: %s",
		e.Name, e.Digest)
}

// BlobUploadNotFoundError is returned when making a blob upload operation against an
// invalid blob upload location url.
// This may be the result of using a cancelled, completed, or stale upload
// location.
type BlobUploadNotFoundError struct {
	Location string
}

func (e *BlobUploadNotFoundError) Error() string {
	return fmt.Sprintf("No blob upload found at Location: %s", e.Location)
}

// BlobUploadInvalidRangeError is returned when attempting to upload an image
// blob chunk that is out of order.
// This provides the known BlobSize and LastValidRange which can be used to
// resume the upload.
type BlobUploadInvalidRangeError struct {
	Location       string
	LastValidRange int
	BlobSize       int
}

func (e *BlobUploadInvalidRangeError) Error() string {
	return fmt.Sprintf(
		"Invalid range provided for upload at Location: %s. Last Valid Range: %d, Blob Size: %d",
		e.Location, e.LastValidRange, e.BlobSize)
}

// UnexpectedHTTPStatusError is returned when an unexpected HTTP status is
// returned when making a registry api call.
type UnexpectedHTTPStatusError struct {
	Status string
}

func (e *UnexpectedHTTPStatusError) Error() string {
	return fmt.Sprintf("Received unexpected HTTP status: %s", e.Status)
}
