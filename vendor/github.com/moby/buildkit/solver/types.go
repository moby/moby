package solver

import (
	"context"
	"time"

	"github.com/containerd/containerd/content"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/solver/pb"
	digest "github.com/opencontainers/go-digest"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
)

// Vertex is a node in a build graph. It defines an interface for a
// content-addressable operation and its inputs.
type Vertex interface {
	// Digest returns a checksum of the definition up to the vertex including
	// all of its inputs.
	Digest() digest.Digest

	// Sys returns an object used to resolve the executor for this vertex.
	// In LLB solver, this value would be of type `llb.Op`.
	Sys() interface{}

	// Options return metadata associated with the vertex that doesn't change the
	// definition or equality check of it.
	Options() VertexOptions

	// Inputs returns an array of edges the vertex depends on. An input edge is
	// a vertex and an index from the returned array of results from an executor
	// returned by Sys(). A vertex may have zero inputs.
	Inputs() []Edge

	Name() string
}

// Index is an index value for the return array of an operation. Index starts
// counting from zero.
type Index int

// Edge is a connection point between vertexes. An edge references a specific
// output of a vertex's operation. Edges are used as inputs to other vertexes.
type Edge struct {
	Index  Index
	Vertex Vertex
}

// VertexOptions define optional metadata for a vertex that doesn't change the
// definition or equality check of it. These options are not contained in the
// vertex digest.
type VertexOptions struct {
	IgnoreCache  bool
	CacheSources []CacheManager
	Description  map[string]string // text values with no special meaning for solver
	ExportCache  *bool
	// WorkerConstraint
}

// Result is an abstract return value for a solve
type Result interface {
	ID() string
	Release(context.Context) error
	Sys() interface{}
	Clone() Result
}

// CachedResult is a result connected with its cache key
type CachedResult interface {
	Result
	CacheKeys() []ExportableCacheKey
}

type ResultProxy interface {
	Result(context.Context) (CachedResult, error)
	Release(context.Context) error
	Definition() *pb.Definition
}

// CacheExportMode is the type for setting cache exporting modes
type CacheExportMode int

const (
	// CacheExportModeMin exports a topmost allowed vertex and its dependencies
	// that already have transferable layers
	CacheExportModeMin CacheExportMode = iota
	// CacheExportModeMax exports all possible non-root vertexes
	CacheExportModeMax
	// CacheExportModeRemoteOnly only exports vertexes that already have
	// transferable layers
	CacheExportModeRemoteOnly
)

// CacheExportOpt defines options for exporting build cache
type CacheExportOpt struct {
	// Convert can convert a build result to transferable object
	Convert func(context.Context, Result) (*Remote, error)
	// Mode defines a cache export algorithm
	Mode CacheExportMode
	// Session is the session group to client (for auth credentials etc)
	Session session.Group
}

// CacheExporter can export the artifacts of the build chain
type CacheExporter interface {
	ExportTo(ctx context.Context, t CacheExporterTarget, opt CacheExportOpt) ([]CacheExporterRecord, error)
}

// CacheExporterTarget defines object capable of receiving exports
type CacheExporterTarget interface {
	Add(dgst digest.Digest) CacheExporterRecord
	Visit(interface{})
	Visited(interface{}) bool
}

// CacheExporterRecord is a single object being exported
type CacheExporterRecord interface {
	AddResult(createdAt time.Time, result *Remote)
	LinkFrom(src CacheExporterRecord, index int, selector string)
}

// Remote is a descriptor or a list of stacked descriptors that can be pulled
// from a content provider
// TODO: add closer to keep referenced data from getting deleted
type Remote struct {
	Descriptors []ocispecs.Descriptor
	Provider    content.Provider
}

// CacheLink is a link between two cache records
type CacheLink struct {
	Source   digest.Digest `json:",omitempty"`
	Input    Index         `json:",omitempty"`
	Output   Index         `json:",omitempty"`
	Base     digest.Digest `json:",omitempty"`
	Selector digest.Digest `json:",omitempty"`
}

type ReleaseFunc func()

// Op defines how the solver can evaluate the properties of a vertex operation.
// An op is executed in the worker, and is retrieved from the vertex by the
// value of `vertex.Sys()`. The solver is configured with a resolve function to
// convert a `vertex.Sys()` into an `Op`.
type Op interface {
	// CacheMap returns structure describing how the operation is cached.
	// Currently only roots are allowed to return multiple cache maps per op.
	CacheMap(context.Context, session.Group, int) (*CacheMap, bool, error)

	// Exec runs an operation given results from previous operations.
	Exec(ctx context.Context, g session.Group, inputs []Result) (outputs []Result, err error)

	// Acquire acquires the necessary resources to execute the `Op`.
	Acquire(ctx context.Context) (release ReleaseFunc, err error)
}

type ResultBasedCacheFunc func(context.Context, Result, session.Group) (digest.Digest, error)
type PreprocessFunc func(context.Context, Result, session.Group) error

// CacheMap is a description for calculating the cache key of an operation.
type CacheMap struct {
	// Digest returns a checksum for the operation. The operation result can be
	// cached by a checksum that combines this digest and the cache keys of the
	// operation's inputs.
	//
	// For example, in LLB this digest is a manifest digest for OCI images, or
	// commit SHA for git sources.
	Digest digest.Digest

	// Deps contain optional selectors or content-based cache functions for its
	// inputs.
	Deps []struct {
		// Selector is a digest that is merged with the cache key of the input.
		// Selectors are not merged with the result of the `ComputeDigestFunc` for
		// this input.
		Selector digest.Digest

		// ComputeDigestFunc should return a digest for the input based on its return
		// value.
		//
		// For example, in LLB this is invoked to calculate the cache key based on
		// the checksum of file contents from input snapshots.
		ComputeDigestFunc ResultBasedCacheFunc

		// PreprocessFunc is a function that runs on an input before it is passed to op
		PreprocessFunc PreprocessFunc
	}

	// Opts specifies generic options that will be passed to cache load calls if/when
	// the key associated with this CacheMap is used to load a ref. It allows options
	// such as oci descriptor content providers and progress writers to be passed to
	// the cache. Opts should not have any impact on the computed cache key.
	Opts CacheOpts
}

// ExportableCacheKey is a cache key connected with an exporter that can export
// a chain of cacherecords pointing to that key
type ExportableCacheKey struct {
	*CacheKey
	Exporter CacheExporter
}

// CacheRecord is an identifier for loading in cache
type CacheRecord struct {
	ID        string
	Size      int
	CreatedAt time.Time
	Priority  int

	cacheManager *cacheManager
	key          *CacheKey
}

// CacheManager determines if there is a result that matches the cache keys
// generated during the build that could be reused instead of fully
// reevaluating the vertex and its inputs. There can be multiple cache
// managers, and specific managers can be defined per vertex using
// `VertexOptions`.
type CacheManager interface {
	// ID is used to identify cache providers that are backed by same source
	// to avoid duplicate calls to the same provider.
	ID() string

	// Query searches for cache paths from one cache key to the output of a
	// possible match.
	Query(inp []CacheKeyWithSelector, inputIndex Index, dgst digest.Digest, outputIndex Index) ([]*CacheKey, error)
	Records(ck *CacheKey) ([]*CacheRecord, error)

	// Load loads a cache record into a result reference.
	Load(ctx context.Context, rec *CacheRecord) (Result, error)

	// Save saves a result based on a cache key
	Save(key *CacheKey, s Result, createdAt time.Time) (*ExportableCacheKey, error)
}
