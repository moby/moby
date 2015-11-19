package distribution

import (
	"github.com/docker/distribution/context"
	"github.com/docker/distribution/digest"
	"github.com/docker/distribution/manifest/schema1"
)

// Scope defines the set of items that match a namespace.
type Scope interface {
	// Contains returns true if the name belongs to the namespace.
	Contains(name string) bool
}

type fullScope struct{}

func (f fullScope) Contains(string) bool {
	return true
}

// GlobalScope represents the full namespace scope which contains
// all other scopes.
var GlobalScope = Scope(fullScope{})

// Namespace represents a collection of repositories, addressable by name.
// Generally, a namespace is backed by a set of one or more services,
// providing facilities such as registry access, trust, and indexing.
type Namespace interface {
	// Scope describes the names that can be used with this Namespace. The
	// global namespace will have a scope that matches all names. The scope
	// effectively provides an identity for the namespace.
	Scope() Scope

	// Repository should return a reference to the named repository. The
	// registry may or may not have the repository but should always return a
	// reference.
	Repository(ctx context.Context, name string) (Repository, error)

	// Repositories fills 'repos' with a lexigraphically sorted catalog of repositories
	// up to the size of 'repos' and returns the value 'n' for the number of entries
	// which were filled.  'last' contains an offset in the catalog, and 'err' will be
	// set to io.EOF if there are no more entries to obtain.
	Repositories(ctx context.Context, repos []string, last string) (n int, err error)
}

// ManifestServiceOption is a function argument for Manifest Service methods
type ManifestServiceOption func(ManifestService) error

// Repository is a named collection of manifests and layers.
type Repository interface {
	// Name returns the name of the repository.
	Name() string

	// Manifests returns a reference to this repository's manifest service.
	// with the supplied options applied.
	Manifests(ctx context.Context, options ...ManifestServiceOption) (ManifestService, error)

	// Blobs returns a reference to this repository's blob service.
	Blobs(ctx context.Context) BlobStore

	// TODO(stevvooe): The above BlobStore return can probably be relaxed to
	// be a BlobService for use with clients. This will allow such
	// implementations to avoid implementing ServeBlob.

	// Signatures returns a reference to this repository's signatures service.
	Signatures() SignatureService
}

// TODO(stevvooe): Must add close methods to all these. May want to change the
// way instances are created to better reflect internal dependency
// relationships.

// ManifestService provides operations on image manifests.
type ManifestService interface {
	// Exists returns true if the manifest exists.
	Exists(dgst digest.Digest) (bool, error)

	// Get retrieves the identified by the digest, if it exists.
	Get(dgst digest.Digest) (*schema1.SignedManifest, error)

	// Delete removes the manifest, if it exists.
	Delete(dgst digest.Digest) error

	// Put creates or updates the manifest.
	Put(manifest *schema1.SignedManifest) error

	// TODO(stevvooe): The methods after this message should be moved to a
	// discrete TagService, per active proposals.

	// Tags lists the tags under the named repository.
	Tags() ([]string, error)

	// ExistsByTag returns true if the manifest exists.
	ExistsByTag(tag string) (bool, error)

	// GetByTag retrieves the named manifest, if it exists.
	GetByTag(tag string, options ...ManifestServiceOption) (*schema1.SignedManifest, error)

	// TODO(stevvooe): There are several changes that need to be done to this
	// interface:
	//
	//	1. Allow explicit tagging with Tag(digest digest.Digest, tag string)
	//	2. Support reading tags with a re-entrant reader to avoid large
	//       allocations in the registry.
	//	3. Long-term: Provide All() method that lets one scroll through all of
	//       the manifest entries.
	//	4. Long-term: break out concept of signing from manifests. This is
	//       really a part of the distribution sprint.
	//	5. Long-term: Manifest should be an interface. This code shouldn't
	//       really be concerned with the storage format.
}

// SignatureService provides operations on signatures.
type SignatureService interface {
	// Get retrieves all of the signature blobs for the specified digest.
	Get(dgst digest.Digest) ([][]byte, error)

	// Put stores the signature for the provided digest.
	Put(dgst digest.Digest, signatures ...[]byte) error
}
