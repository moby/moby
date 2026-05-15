package image

import (
	"encoding/json"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// AttestationStatement is a single in-toto statement attached to an image.
type AttestationStatement struct {
	// Descriptor is the OCI descriptor of the statement blob (media type,
	// digest, size, annotations).
	Descriptor ocispec.Descriptor `json:"Descriptor"`
	// PredicateType is the in-toto predicate type URI of this statement.
	PredicateType string `json:"PredicateType"`
	// Statement is the verbatim in-toto statement JSON. Omitted unless the
	// caller opts in via the statement=true query parameter.
	Statement *json.RawMessage `json:"Statement,omitempty"`
}
