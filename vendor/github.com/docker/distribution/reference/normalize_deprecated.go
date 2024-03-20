package reference

import (
	"regexp"

	"github.com/distribution/reference"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/go-digest/digestset"
)

// ParseNormalizedNamed parses a string into a named reference
// transforming a familiar name from Docker UI to a fully
// qualified reference. If the value may be an identifier
// use ParseAnyReference.
//
// Deprecated: use [reference.ParseNormalizedNamed].
func ParseNormalizedNamed(s string) (reference.Named, error) {
	return reference.ParseNormalizedNamed(s)
}

// ParseDockerRef normalizes the image reference following the docker convention,
// which allows for references to contain both a tag and a digest.
//
// Deprecated: use [reference.ParseDockerRef].
func ParseDockerRef(ref string) (reference.Named, error) {
	return reference.ParseDockerRef(ref)
}

// TagNameOnly adds the default tag "latest" to a reference if it only has
// a repo name.
//
// Deprecated: use [reference.TagNameOnly].
func TagNameOnly(ref reference.Named) reference.Named {
	return reference.TagNameOnly(ref)
}

// ParseAnyReference parses a reference string as a possible identifier,
// full digest, or familiar name.
//
// Deprecated: use [reference.ParseAnyReference].
func ParseAnyReference(ref string) (reference.Reference, error) {
	return reference.ParseAnyReference(ref)
}

// Functions and types below have been removed in distribution v3 and
// have not been ported to github.com/distribution/reference. See
// https://github.com/distribution/distribution/pull/3774

var (
	// ShortIdentifierRegexp is the format used to represent a prefix
	// of an identifier. A prefix may be used to match a sha256 identifier
	// within a list of trusted identifiers.
	//
	// Deprecated: support for short-identifiers is deprecated, and will be removed in v3.
	ShortIdentifierRegexp = regexp.MustCompile(shortIdentifier)

	shortIdentifier = `([a-f0-9]{6,64})`

	// anchoredShortIdentifierRegexp is used to check if a value
	// is a possible identifier prefix, anchored at start and end
	// of string.
	anchoredShortIdentifierRegexp = regexp.MustCompile(`^` + shortIdentifier + `$`)
)

type digestReference digest.Digest

func (d digestReference) String() string {
	return digest.Digest(d).String()
}

func (d digestReference) Digest() digest.Digest {
	return digest.Digest(d)
}

// ParseAnyReferenceWithSet parses a reference string as a possible short
// identifier to be matched in a digest set, a full digest, or familiar name.
//
// Deprecated: support for short-identifiers is deprecated, and will be removed in v3.
func ParseAnyReferenceWithSet(ref string, ds *digestset.Set) (Reference, error) {
	if ok := anchoredShortIdentifierRegexp.MatchString(ref); ok {
		dgst, err := ds.Lookup(ref)
		if err == nil {
			return digestReference(dgst), nil
		}
	} else {
		if dgst, err := digest.Parse(ref); err == nil {
			return digestReference(dgst), nil
		}
	}

	return reference.ParseNormalizedNamed(ref)
}
