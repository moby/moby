package v01

import (
	"time"

	"github.com/in-toto/in-toto-golang/in_toto/slsa_provenance/common"
)

const (
	// PredicateSLSAProvenance represents a build provenance for an artifact.
	PredicateSLSAProvenance = "https://slsa.dev/provenance/v0.1"
)

// ProvenancePredicate is the provenance predicate definition.
type ProvenancePredicate struct {
	Builder   common.ProvenanceBuilder    `json:"builder"`
	Recipe    ProvenanceRecipe            `json:"recipe"`
	Metadata  *ProvenanceMetadata         `json:"metadata,omitempty"`
	Materials []common.ProvenanceMaterial `json:"materials,omitempty"`
}

// ProvenanceRecipe describes the actions performed by the builder.
type ProvenanceRecipe struct {
	Type string `json:"type"`
	// DefinedInMaterial can be sent as the null pointer to indicate that
	// the value is not present.
	DefinedInMaterial *int        `json:"definedInMaterial,omitempty"`
	EntryPoint        string      `json:"entryPoint"`
	Arguments         interface{} `json:"arguments,omitempty"`
	Environment       interface{} `json:"environment,omitempty"`
}

// ProvenanceMetadata contains metadata for the built artifact.
type ProvenanceMetadata struct {
	// Use pointer to make sure that the abscense of a time is not
	// encoded as the Epoch time.
	BuildStartedOn  *time.Time         `json:"buildStartedOn,omitempty"`
	BuildFinishedOn *time.Time         `json:"buildFinishedOn,omitempty"`
	Completeness    ProvenanceComplete `json:"completeness"`
	Reproducible    bool               `json:"reproducible"`
}

// ProvenanceComplete indicates wheter the claims in build/recipe are complete.
// For in depth information refer to the specifictaion:
// https://github.com/in-toto/attestation/blob/v0.1.0/spec/predicates/provenance.md
type ProvenanceComplete struct {
	Arguments   bool `json:"arguments"`
	Environment bool `json:"environment"`
	Materials   bool `json:"materials"`
}
