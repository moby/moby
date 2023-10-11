package source

import (
	"github.com/moby/buildkit/solver/llbsolver/provenance"
	"github.com/pkg/errors"
)

var (
	errInvalid  = errors.New("invalid")
	errNotFound = errors.New("not found")
)

type Identifier interface {
	// Scheme returns the scheme of the identifier so that it can be routed back
	// to an appropriate Source.
	Scheme() string

	// Capture records the provenance of the identifier.
	Capture(dest *provenance.Capture, pin string) error
}
