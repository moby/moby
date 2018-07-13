/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package content

import (
	"context"
	"io"
	"time"

	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// ReaderAt extends the standard io.ReaderAt interface with reporting of Size and io.Closer
type ReaderAt interface {
	io.ReaderAt
	io.Closer
	Size() int64
}

// Provider provides a reader interface for specific content
type Provider interface {
	// ReaderAt only requires desc.Digest to be set.
	// Other fields in the descriptor may be used internally for resolving
	// the location of the actual data.
	ReaderAt(ctx context.Context, dec ocispec.Descriptor) (ReaderAt, error)
}

// Ingester writes content
type Ingester interface {
	// Some implementations require WithRef to be included in opts.
	Writer(ctx context.Context, opts ...WriterOpt) (Writer, error)
}

// Info holds content specific information
//
// TODO(stevvooe): Consider a very different name for this struct. Info is way
// to general. It also reads very weird in certain context, like pluralization.
type Info struct {
	Digest    digest.Digest
	Size      int64
	CreatedAt time.Time
	UpdatedAt time.Time
	Labels    map[string]string
}

// Status of a content operation
type Status struct {
	Ref       string
	Offset    int64
	Total     int64
	Expected  digest.Digest
	StartedAt time.Time
	UpdatedAt time.Time
}

// WalkFunc defines the callback for a blob walk.
type WalkFunc func(Info) error

// Manager provides methods for inspecting, listing and removing content.
type Manager interface {
	// Info will return metadata about content available in the content store.
	//
	// If the content is not present, ErrNotFound will be returned.
	Info(ctx context.Context, dgst digest.Digest) (Info, error)

	// Update updates mutable information related to content.
	// If one or more fieldpaths are provided, only those
	// fields will be updated.
	// Mutable fields:
	//  labels.*
	Update(ctx context.Context, info Info, fieldpaths ...string) (Info, error)

	// Walk will call fn for each item in the content store which
	// match the provided filters. If no filters are given all
	// items will be walked.
	Walk(ctx context.Context, fn WalkFunc, filters ...string) error

	// Delete removes the content from the store.
	Delete(ctx context.Context, dgst digest.Digest) error
}

// IngestManager provides methods for managing ingests.
type IngestManager interface {
	// Status returns the status of the provided ref.
	Status(ctx context.Context, ref string) (Status, error)

	// ListStatuses returns the status of any active ingestions whose ref match the
	// provided regular expression. If empty, all active ingestions will be
	// returned.
	ListStatuses(ctx context.Context, filters ...string) ([]Status, error)

	// Abort completely cancels the ingest operation targeted by ref.
	Abort(ctx context.Context, ref string) error
}

// Writer handles the write of content into a content store
type Writer interface {
	// Close is expected to be called after Commit() when commission is needed.
	// Closing a writer without commit allows resuming or aborting.
	io.WriteCloser

	// Digest may return empty digest or panics until committed.
	Digest() digest.Digest

	// Commit commits the blob (but no roll-back is guaranteed on an error).
	// size and expected can be zero-value when unknown.
	Commit(ctx context.Context, size int64, expected digest.Digest, opts ...Opt) error

	// Status returns the current state of write
	Status() (Status, error)

	// Truncate updates the size of the target blob
	Truncate(size int64) error
}

// Store combines the methods of content-oriented interfaces into a set that
// are commonly provided by complete implementations.
type Store interface {
	Manager
	Provider
	IngestManager
	Ingester
}

// Opt is used to alter the mutable properties of content
type Opt func(*Info) error

// WithLabels allows labels to be set on content
func WithLabels(labels map[string]string) Opt {
	return func(info *Info) error {
		info.Labels = labels
		return nil
	}
}

// WriterOpts is internally used by WriterOpt.
type WriterOpts struct {
	Ref  string
	Desc ocispec.Descriptor
}

// WriterOpt is used for passing options to Ingester.Writer.
type WriterOpt func(*WriterOpts) error

// WithDescriptor specifies an OCI descriptor.
// Writer may optionally use the descriptor internally for resolving
// the location of the actual data.
// Write does not require any field of desc to be set.
// If the data size is unknown, desc.Size should be set to 0.
// Some implementations may also accept negative values as "unknown".
func WithDescriptor(desc ocispec.Descriptor) WriterOpt {
	return func(opts *WriterOpts) error {
		opts.Desc = desc
		return nil
	}
}

// WithRef specifies a ref string.
func WithRef(ref string) WriterOpt {
	return func(opts *WriterOpts) error {
		opts.Ref = ref
		return nil
	}
}
