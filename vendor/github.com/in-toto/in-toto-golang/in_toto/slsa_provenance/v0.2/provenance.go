package v02

import (
	"time"

	"github.com/in-toto/in-toto-golang/in_toto/slsa_provenance/common"
)

const (
	// PredicateSLSAProvenance represents a build provenance for an artifact.
	PredicateSLSAProvenance = "https://slsa.dev/provenance/v0.2"
)

// These are type aliases to common to avoid backwards incompatible changes.
type (
	DigestSet          = common.DigestSet
	ProvenanceBuilder  = common.ProvenanceBuilder
	ProvenanceMaterial = common.ProvenanceMaterial
)

// ProvenancePredicate is the provenance predicate definition.
type ProvenancePredicate struct {
	// Builder identifies the entity that executed the invocation, which is trusted to have
	// correctly performed the operation and populated this provenance.
	//
	// The identity MUST reflect the trust base that consumers care about. How detailed to be is a
	// judgement call. For example, GitHub Actions supports both GitHub-hosted runners and
	// self-hosted runners. The GitHub-hosted runner might be a single identity because it’s all
	// GitHub from the consumer’s perspective. Meanwhile, each self-hosted runner might have its
	// own identity because not all runners are trusted by all consumers.
	Builder common.ProvenanceBuilder `json:"builder"`

	// BuildType is a URI indicating what type of build was performed. It determines the meaning of
	// [Invocation], [BuildConfig] and [Materials].
	BuildType string `json:"buildType"`

	// Invocation identifies the event that kicked off the build. When combined with materials,
	// this SHOULD fully describe the build, such that re-running this invocation results in
	// bit-for-bit identical output (if the build is reproducible).
	//
	// MAY be unset/null if unknown, but this is DISCOURAGED.
	Invocation ProvenanceInvocation `json:"invocation,omitempty"`

	// BuildConfig lists the steps in the build. If [ProvenanceInvocation.ConfigSource] is not
	// available, BuildConfig can be used to verify information about the build.
	//
	// This is an arbitrary JSON object with a schema defined by [BuildType].
	BuildConfig interface{} `json:"buildConfig,omitempty"`

	// Metadata contains other properties of the build.
	Metadata *ProvenanceMetadata `json:"metadata,omitempty"`

	// Materials is the collection of artifacts that influenced the build including sources,
	// dependencies, build tools, base images, and so on.
	//
	// This is considered to be incomplete unless metadata.completeness.materials is true.
	Materials []common.ProvenanceMaterial `json:"materials,omitempty"`
}

// ProvenanceInvocation identifies the event that kicked off the build.
type ProvenanceInvocation struct {
	// ConfigSource describes where the config file that kicked off the build came from. This is
	// effectively a pointer to the source where [ProvenancePredicate.BuildConfig] came from.
	ConfigSource ConfigSource `json:"configSource,omitempty"`

	// Parameters is a collection of all external inputs that influenced the build on top of
	// ConfigSource. For example, if the invocation type were “make”, then this might be the
	// flags passed to make aside from the target, which is captured in [ConfigSource.EntryPoint].
	//
	// Consumers SHOULD accept only “safe” Parameters. The simplest and safest way to
	// achieve this is to disallow any parameters altogether.
	//
	// This is an arbitrary JSON object with a schema defined by buildType.
	Parameters interface{} `json:"parameters,omitempty"`

	// Environment contains any other builder-controlled inputs necessary for correctly evaluating
	// the build. Usually only needed for reproducing the build but not evaluated as part of
	// policy.
	//
	// This SHOULD be minimized to only include things that are part of the public API, that cannot
	// be recomputed from other values in the provenance, and that actually affect the evaluation
	// of the build. For example, this might include variables that are referenced in the workflow
	// definition, but it SHOULD NOT include a dump of all environment variables or include things
	// like the hostname (assuming hostname is not part of the public API).
	Environment interface{} `json:"environment,omitempty"`
}

type ConfigSource struct {
	// URI indicating the identity of the source of the config.
	URI string `json:"uri,omitempty"`
	// Digest is a collection of cryptographic digests for the contents of the artifact specified
	// by [URI].
	Digest common.DigestSet `json:"digest,omitempty"`
	// EntryPoint identifying the entry point into the build. This is often a path to a
	// configuration file and/or a target label within that file. The syntax and meaning are
	// defined by buildType. For example, if the buildType were “make”, then this would reference
	// the directory in which to run make as well as which target to use.
	//
	// Consumers SHOULD accept only specific [ProvenanceInvocation.EntryPoint] values. For example,
	// a policy might only allow the "release" entry point but not the "debug" entry point.
	// MAY be omitted if the buildType specifies a default value.
	EntryPoint string `json:"entryPoint,omitempty"`
}

// ProvenanceMetadata contains metadata for the built artifact.
type ProvenanceMetadata struct {
	// BuildInvocationID identifies this particular build invocation, which can be useful for
	// finding associated logs or other ad-hoc analysis. The exact meaning and format is defined
	// by [common.ProvenanceBuilder.ID]; by default it is treated as opaque and case-sensitive.
	// The value SHOULD be globally unique.
	BuildInvocationID string `json:"buildInvocationID,omitempty"`

	// BuildStartedOn is the timestamp of when the build started.
	//
	// Use pointer to make sure that the abscense of a time is not
	// encoded as the Epoch time.
	BuildStartedOn *time.Time `json:"buildStartedOn,omitempty"`
	// BuildFinishedOn is the timestamp of when the build completed.
	BuildFinishedOn *time.Time `json:"buildFinishedOn,omitempty"`

	// Completeness indicates that the builder claims certain fields in this message to be
	// complete.
	Completeness ProvenanceComplete `json:"completeness"`

	// Reproducible if true, means the builder claims that running invocation on materials will
	// produce bit-for-bit identical output.
	Reproducible bool `json:"reproducible"`
}

// ProvenanceComplete indicates wheter the claims in build/recipe are complete.
// For in depth information refer to the specifictaion:
// https://github.com/in-toto/attestation/blob/v0.1.0/spec/predicates/provenance.md
type ProvenanceComplete struct {
	// Parameters if true, means the builder claims that [ProvenanceInvocation.Parameters] is
	// complete, meaning that all external inputs are properly captured in
	// ProvenanceInvocation.Parameters.
	Parameters bool `json:"parameters"`
	// Environment if true, means the builder claims that [ProvenanceInvocation.Environment] is
	// complete.
	Environment bool `json:"environment"`
	// Materials if true, means the builder claims that materials is complete, usually through some
	// controls to prevent network access. Sometimes called “hermetic”.
	Materials bool `json:"materials"`
}
