package v02

import "time"

const (
	// PredicateSLSAProvenance represents a build provenance for an artifact.
	PredicateSLSAProvenance = "https://slsa.dev/provenance/v0.2"
)

// ProvenancePredicate is the provenance predicate definition.
type ProvenancePredicate struct {
	Builder     ProvenanceBuilder    `json:"builder"`
	BuildType   string               `json:"buildType"`
	Invocation  ProvenanceInvocation `json:"invocation,omitempty"`
	BuildConfig interface{}          `json:"buildConfig,omitempty"`
	Metadata    *ProvenanceMetadata  `json:"metadata,omitempty"`
	Materials   []ProvenanceMaterial `json:"materials,omitempty"`
}

// ProvenanceBuilder idenfifies the entity that executed the build steps.
type ProvenanceBuilder struct {
	ID string `json:"id"`
}

// ProvenanceInvocation identifies the event that kicked off the build.
type ProvenanceInvocation struct {
	ConfigSource ConfigSource `json:"configSource,omitempty"`
	Parameters   interface{}  `json:"parameters,omitempty"`
	Environment  interface{}  `json:"environment,omitempty"`
}

type ConfigSource struct {
	URI        string    `json:"uri,omitempty"`
	Digest     DigestSet `json:"digest,omitempty"`
	EntryPoint string    `json:"entryPoint,omitempty"`
}

// ProvenanceMetadata contains metadata for the built artifact.
type ProvenanceMetadata struct {
	BuildInvocationID string `json:"buildInvocationID,omitempty"`
	// Use pointer to make sure that the abscense of a time is not
	// encoded as the Epoch time.
	BuildStartedOn  *time.Time         `json:"buildStartedOn,omitempty"`
	BuildFinishedOn *time.Time         `json:"buildFinishedOn,omitempty"`
	Completeness    ProvenanceComplete `json:"completeness"`
	Reproducible    bool               `json:"reproducible"`
}

// ProvenanceMaterial defines the materials used to build an artifact.
type ProvenanceMaterial struct {
	URI    string    `json:"uri,omitempty"`
	Digest DigestSet `json:"digest,omitempty"`
}

// ProvenanceComplete indicates wheter the claims in build/recipe are complete.
// For in depth information refer to the specifictaion:
// https://github.com/in-toto/attestation/blob/v0.1.0/spec/predicates/provenance.md
type ProvenanceComplete struct {
	Parameters  bool `json:"parameters"`
	Environment bool `json:"environment"`
	Materials   bool `json:"materials"`
}

// DigestSet contains a set of digests. It is represented as a map from
// algorithm name to lowercase hex-encoded value.
type DigestSet map[string]string
