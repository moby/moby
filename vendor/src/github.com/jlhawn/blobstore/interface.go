package blobstore

import (
	"crypto"
	"io"
)

// Store is the interface for managing image manifest and rootfs diff blobs.
// Any errors returned by this interface are of type Error.
type Store interface {
	// Get the blob with the given digest from this store.
	Get(digest string) (Blob, error)
	// List returns a slice of digest strings for each blob in this store.
	List() ([]string, error)
	// NewWriter begins the process of writing a new blob using the given hash
	// to compute the digest.
	NewWriter(crypto.Hash) (BlobWriter, error)
	// Ref associates a reference ID with the blob with the given digest.
	Ref(digest, refID string) (Descriptor, error)
	// Deref dissociates a reference ID with the blob with the given digest.
	// If no references to the blob remain, the blob will be removed from the
	// store.
	Deref(digest, refID string) error
}

// BlobWriter provides a handle for writing a new blob to the blob store.
type BlobWriter interface {
	io.Writer
	// Digest returns the digest of the data which has been written so far.
	Digest() string
	// Commit completes the blob writing process. The new blob is stored with
	// the given mediaType and starting RefID. If another blob already exists
	// with the same computed digest then the reference is added to that blob.
	// If there is an error, it is of type Error.
	Commit(mediaType, refID string) (Descriptor, error)
	// Cancel ends the writing process, cleaning up any temporary resources. If
	// there is an error, it is of type *Error.
	Cancel() error
}

// Descriptor describes a blob.
type Descriptor interface {
	Digest() string
	MediaType() string
	Size() uint64
	References() []string
}

// Blob is the interface for accessing a blob.
type Blob interface {
	Descriptor
	// Open should open the underlying blob for reading. It is the
	// responsibility of the caller to close the returned io.ReadCloser.
	// Returns a nil error on success.
	Open() (io.ReadCloser, error)
}

// Error is the error type returned by the Store interface.
type Error interface {
	error
	IsBlobNotExists() bool
}
