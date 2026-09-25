//go:generate go tool mobyextgen

// Package storagekvv0 defines persistent record storage for Moby extensions.
package storagekvv0

import (
	"context"

	"github.com/moby/extensions"
)

// Storage saves key/value records in named namespaces.
// Callers using the same namespace share records; names do not control access.
type Storage interface {
	// Get reads a record, or returns not-found if it is missing.
	Get(ctx context.Context, req *RecordRequest) (*GetResponse, error)

	// Create adds a record, or returns a conflict if the key exists.
	Create(ctx context.Context, req *WriteRequest) error

	// Update replaces a record, or returns not-found if it is missing.
	Update(ctx context.Context, req *WriteRequest) error

	// Delete removes a record, doing nothing if it is missing.
	Delete(ctx context.Context, req *RecordRequest) error

	// List returns a page of matching keys in sorted order.
	// Results can change between pages.
	List(ctx context.Context, req *ListRequest) (*ListResponse, error)
}

// Point selects one storage provider.
var Point = extensions.DefineSinglePoint[Storage]("org.mobyproject.extension.storage.kv.v0")

const (
	// MaxNamespaceSize is the maximum namespace length in bytes.
	// Names start with an ASCII letter or digit and allow letters, digits, . _ -.
	MaxNamespaceSize = 128
	// MaxKeySize is the maximum key or prefix length in bytes.
	// Keys are nonempty UTF-8 strings without NUL bytes, not paths.
	MaxKeySize = 1024
	// MaxValueSize is the maximum value length in bytes.
	MaxValueSize = 1 << 20
	// DefaultPageSize applies when ListRequest.Limit is zero.
	DefaultPageSize = 100
	// MaxPageSize is the maximum number of keys per page.
	MaxPageSize = 1000
)

// RecordRequest identifies a record.
type RecordRequest struct {
	Namespace string `pb:"1"`
	Key       string `pb:"2"`
}

// WriteRequest supplies a record's value, which may be empty.
type WriteRequest struct {
	Namespace string `pb:"1"`
	Key       string `pb:"2"`
	Value     []byte `pb:"3"`
}

// GetResponse contains a record's value.
type GetResponse struct {
	Value []byte `pb:"1"`
}

// ListRequest selects a page of keys.
type ListRequest struct {
	Namespace string `pb:"1"`
	// Prefix filters keys by their start; empty matches all keys.
	Prefix string `pb:"2"`
	// Limit sets the page size; zero uses DefaultPageSize.
	Limit uint32 `pb:"3"`
	// Cursor continues a listing with the same namespace and prefix.
	Cursor string `pb:"4"`
}

// ListResponse contains a page of keys.
// An empty NextCursor means there are no more results.
type ListResponse struct {
	Keys       []string `pb:"1"`
	NextCursor string   `pb:"2"`
}
