package reference

import (
	"github.com/docker/distribution/digest"
	distreference "github.com/docker/distribution/reference"
)

// Reference is an opaque object reference identifier that may include
// modifiers such as a hostname, name, tag, and digest.
type Reference interface {
	// String returns the full reference
	String() string
}

// Named is an object with a full name
type Named interface {
	Reference
	Name() string
}

// NamedTagged is an object including a name and tag.
type NamedTagged interface {
	Named
	Tag() string
}

// Canonical reference is an object with a fully unique
// name including a name with hostname and digest
type Canonical interface {
	Named
	Digest() digest.Digest
}

// ParseNamed parses s and returns a syntactically valid reference implementing
// the Named interface. The reference must have a name, otherwise an error is
// returned.
// If an error was encountered it is returned, along with a nil Reference.
// NOTE: ParseNamed will not handle short digests.
func ParseNamed(s string) (Named, error) {
	// todo: docker specific validation
	return distreference.ParseNamed(s)
}

// WithName returns a named object representing the given string. If the input
// is invalid ErrReferenceInvalidFormat will be returned.
func WithName(name string) (Named, error) {
	return distreference.WithName(name)
}

// WithTag combines the name from "name" and the tag from "tag" to form a
// reference incorporating both the name and the tag.
func WithTag(name Named, tag string) (NamedTagged, error) {
	return distreference.WithTag(name, tag)
}

// WithDigest combines the name from "name" and the digest from "digest" to form
// a reference incorporating both the name and the digest.
func WithDigest(name Named, digest digest.Digest) (Canonical, error) {
	return distreference.WithDigest(name, digest)
}
