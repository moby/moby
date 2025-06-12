package v1

import (
	"time"

	"github.com/in-toto/in-toto-golang/in_toto/slsa_provenance/common"
)

const (
	// PredicateSLSAProvenance represents a build provenance for an artifact.
	PredicateSLSAProvenance = "https://slsa.dev/provenance/v1"
)

// ProvenancePredicate is the provenance predicate definition.
type ProvenancePredicate struct {
	// The BuildDefinition describes all of the inputs to the build. The
	// accuracy and completeness are implied by runDetails.builder.id.
	//
	// It SHOULD contain all the information necessary and sufficient to
	// initialize the build and begin execution.
	BuildDefinition ProvenanceBuildDefinition `json:"buildDefinition"`

	// Details specific to this particular execution of the build.
	RunDetails ProvenanceRunDetails `json:"runDetails"`
}

// ProvenanceBuildDefinition describes the inputs to the build.
type ProvenanceBuildDefinition struct {
	// Identifies the template for how to perform the build and interpret the
	// parameters and dependencies.

	// The URI SHOULD resolve to a human-readable specification that includes:
	// overall description of the build type; schema for externalParameters and
	// systemParameters; unambiguous instructions for how to initiate the build
	// given this BuildDefinition, and a complete example.
	BuildType string `json:"buildType"`

	// The parameters that are under external control, such as those set by a
	// user or tenant of the build system. They MUST be complete at SLSA Build
	// L3, meaning that that there is no additional mechanism for an external
	// party to influence the build. (At lower SLSA Build levels, the
	// completeness MAY be best effort.)

	// The build system SHOULD be designed to minimize the size and complexity
	// of externalParameters, in order to reduce fragility and ease
	// verification. Consumers SHOULD have an expectation of what “good” looks
	// like; the more information that they need to check, the harder that task
	// becomes.
	ExternalParameters interface{} `json:"externalParameters"`

	// The parameters that are under the control of the entity represented by
	// builder.id. The primary intention of this field is for debugging,
	// incident response, and vulnerability management. The values here MAY be
	// necessary for reproducing the build. There is no need to verify these
	// parameters because the build system is already trusted, and in many cases
	// it is not practical to do so.
	InternalParameters interface{} `json:"internalParameters,omitempty"`

	// Unordered collection of artifacts needed at build time. Completeness is
	// best effort, at least through SLSA Build L3. For example, if the build
	// script fetches and executes “example.com/foo.sh”, which in turn fetches
	// “example.com/bar.tar.gz”, then both “foo.sh” and “bar.tar.gz” SHOULD be
	// listed here.
	ResolvedDependencies []ResourceDescriptor `json:"resolvedDependencies,omitempty"`
}

// ProvenanceRunDetails includes details specific to a particular execution of a
// build.
type ProvenanceRunDetails struct {
	// Identifies the entity that executed the invocation, which is trusted to
	// have correctly performed the operation and populated this provenance.
	//
	// This field is REQUIRED for SLSA Build 1 unless id is implicit from the
	// attestation envelope.
	Builder Builder `json:"builder"`

	// Metadata about this particular execution of the build.
	BuildMetadata BuildMetadata `json:"metadata,omitempty"`

	// Additional artifacts generated during the build that are not considered
	// the “output” of the build but that might be needed during debugging or
	// incident response. For example, this might reference logs generated
	// during the build and/or a digest of the fully evaluated build
	// configuration.
	//
	// In most cases, this SHOULD NOT contain all intermediate files generated
	// during the build. Instead, this SHOULD only contain files that are
	// likely to be useful later and that cannot be easily reproduced.
	Byproducts []ResourceDescriptor `json:"byproducts,omitempty"`
}

// ResourceDescriptor describes a particular software artifact or resource
// (mutable or immutable).
// See https://github.com/in-toto/attestation/blob/main/spec/v1.0/resource_descriptor.md
type ResourceDescriptor struct {
	// A URI used to identify the resource or artifact globally. This field is
	// REQUIRED unless either digest or content is set.
	URI string `json:"uri,omitempty"`

	// A set of cryptographic digests of the contents of the resource or
	// artifact. This field is REQUIRED unless either uri or content is set.
	Digest common.DigestSet `json:"digest,omitempty"`

	// TMachine-readable identifier for distinguishing between descriptors.
	Name string `json:"name,omitempty"`

	// The location of the described resource or artifact, if different from the
	// uri.
	DownloadLocation string `json:"downloadLocation,omitempty"`

	// The MIME Type (i.e., media type) of the described resource or artifact.
	MediaType string `json:"mediaType,omitempty"`

	// The contents of the resource or artifact. This field is REQUIRED unless
	// either uri or digest is set.
	Content []byte `json:"content,omitempty"`

	// This field MAY be used to provide additional information or metadata
	// about the resource or artifact that may be useful to the consumer when
	// evaluating the attestation against a policy.
	Annotations map[string]interface{} `json:"annotations,omitempty"`
}

// Builder represents the transitive closure of all the entities that are, by
// necessity, trusted to faithfully run the build and record the provenance.
type Builder struct {
	// URI indicating the transitive closure of the trusted builder.
	ID string `json:"id"`

	// Version numbers of components of the builder.
	Version map[string]string `json:"version,omitempty"`

	// Dependencies used by the orchestrator that are not run within the
	// workload and that do not affect the build, but might affect the
	// provenance generation or security guarantees.
	BuilderDependencies []ResourceDescriptor `json:"builderDependencies,omitempty"`
}

type BuildMetadata struct {
	// Identifies this particular build invocation, which can be useful for
	// finding associated logs or other ad-hoc analysis. The exact meaning and
	// format is defined by builder.id; by default it is treated as opaque and
	// case-sensitive. The value SHOULD be globally unique.
	InvocationID string `json:"invocationID,omitempty"`

	// The timestamp of when the build started.
	StartedOn *time.Time `json:"startedOn,omitempty"`

	// The timestamp of when the build completed.
	FinishedOn *time.Time `json:"finishedOn,omitempty"`
}
