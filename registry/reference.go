package registry

import (
	"strings"

	"github.com/docker/distribution/digest"
)

// Reference represents a tag or digest within a repository
type Reference interface {
	// HasDigest returns whether the reference has a verifiable
	// content addressable reference which may be considered secure.
	HasDigest() bool

	// ImageName returns an image name for the given repository
	ImageName(string) string

	// Returns a string representation of the reference
	String() string
}

type tagReference struct {
	tag string
}

func (tr tagReference) HasDigest() bool {
	return false
}

func (tr tagReference) ImageName(repo string) string {
	return repo + ":" + tr.tag
}

func (tr tagReference) String() string {
	return tr.tag
}

type digestReference struct {
	digest digest.Digest
}

func (dr digestReference) HasDigest() bool {
	return true
}

func (dr digestReference) ImageName(repo string) string {
	return repo + "@" + dr.String()
}

func (dr digestReference) String() string {
	return dr.digest.String()
}

// ParseReference parses a reference into either a digest or tag reference
func ParseReference(ref string) Reference {
	if strings.Contains(ref, ":") {
		dgst, err := digest.ParseDigest(ref)
		if err == nil {
			return digestReference{digest: dgst}
		}
	}
	return tagReference{tag: ref}
}

// DigestReference creates a digest reference using a digest
func DigestReference(dgst digest.Digest) Reference {
	return digestReference{digest: dgst}
}
