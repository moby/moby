package client

import (
	"github.com/moby/moby/api/types/image"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// ImageAttestationsResult is the result of an ImageAttestations operation.
type ImageAttestationsResult struct {
	Items []image.AttestationStatement
}

// ImageAttestationsOption is a functional option for the ImageAttestations operation.
type ImageAttestationsOption interface {
	Apply(*imageAttestationsOpts) error
}

type imageAttestationsOptionFunc func(*imageAttestationsOpts) error

func (f imageAttestationsOptionFunc) Apply(o *imageAttestationsOpts) error { return f(o) }

type imageAttestationsOpts struct {
	platform         *ocispec.Platform
	predicateTypes   []string
	includeStatement bool
}

// ImageAttestationsWithPlatform filters attestations to those for the given
// platform variant. If omitted, the daemon's default platform is used.
func ImageAttestationsWithPlatform(platform ocispec.Platform) ImageAttestationsOption {
	return imageAttestationsOptionFunc(func(o *imageAttestationsOpts) error {
		o.platform = &platform
		return nil
	})
}

// ImageAttestationsWithPredicateTypes filters returned statements to those
// whose in-toto predicate type matches one of the given URIs.
// If not set, all statements are returned.
func ImageAttestationsWithPredicateTypes(types ...string) ImageAttestationsOption {
	return imageAttestationsOptionFunc(func(o *imageAttestationsOpts) error {
		o.predicateTypes = append(o.predicateTypes, types...)
		return nil
	})
}

// ImageAttestationsWithStatement asks the daemon to include the verbatim
// in-toto statement body in each returned entry. Without this option, only
// the descriptor and predicate type are returned and statement blobs are
// not read.
func ImageAttestationsWithStatement() ImageAttestationsOption {
	return imageAttestationsOptionFunc(func(o *imageAttestationsOpts) error {
		o.includeStatement = true
		return nil
	})
}
