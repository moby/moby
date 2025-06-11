package types

import (
	"slices"

	slsa "github.com/in-toto/in-toto-golang/in_toto/slsa_provenance/common"
	slsa02 "github.com/in-toto/in-toto-golang/in_toto/slsa_provenance/v0.2"
	slsa1 "github.com/in-toto/in-toto-golang/in_toto/slsa_provenance/v1"
	resourcestypes "github.com/moby/buildkit/executor/resources/types"
	"github.com/moby/buildkit/solver/pb"
	digest "github.com/opencontainers/go-digest"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

const (
	BuildKitBuildType = "https://mobyproject.org/buildkit@v1"
)

type BuildConfig struct {
	Definition    []BuildStep              `json:"llbDefinition,omitempty"`
	DigestMapping map[digest.Digest]string `json:"digestMapping,omitempty"`
}

type BuildStep struct {
	ID            string                  `json:"id,omitempty"`
	Op            *pb.Op                  `json:"op,omitempty"`
	Inputs        []string                `json:"inputs,omitempty"`
	ResourceUsage *resourcestypes.Samples `json:"resourceUsage,omitempty"`
}

type Source struct {
	Locations map[string]*pb.Locations `json:"locations,omitempty"`
	Infos     []SourceInfo             `json:"infos,omitempty"`
}

type SourceInfo struct {
	Filename      string                   `json:"filename,omitempty"`
	Language      string                   `json:"language,omitempty"`
	Data          []byte                   `json:"data,omitempty"`
	Definition    []BuildStep              `json:"llbDefinition,omitempty"`
	DigestMapping map[digest.Digest]string `json:"digestMapping,omitempty"`
}

type ImageSource struct {
	Ref      string
	Platform *ocispecs.Platform
	Digest   digest.Digest
	Local    bool
}

type GitSource struct {
	URL    string
	Commit string
}

type HTTPSource struct {
	URL    string
	Digest digest.Digest
}

type LocalSource struct {
	Name string `json:"name"`
}

type Secret struct {
	ID       string `json:"id"`
	Optional bool   `json:"optional,omitempty"`
}

type SSH struct {
	ID       string `json:"id"`
	Optional bool   `json:"optional,omitempty"`
}

type Sources struct {
	Images []ImageSource
	Git    []GitSource
	HTTP   []HTTPSource
	Local  []LocalSource
}

const (
	ProvenanceSLSA1  = ProvenanceSLSA("v1")
	ProvenanceSLSA02 = ProvenanceSLSA("v0.2")
)

type ProvenanceSLSA string

var provenanceSLSAs = []ProvenanceSLSA{
	ProvenanceSLSA1,
	ProvenanceSLSA02,
}

func (ps *ProvenanceSLSA) Validate() error {
	if *ps == "" {
		return errors.New("provenance SLSA version cannot be empty")
	}
	if slices.Contains(provenanceSLSAs, *ps) {
		return nil
	}
	return errors.New("invalid provenance SLSA version: " + string(*ps))
}

type ProvenancePredicateSLSA02 struct {
	slsa02.ProvenancePredicate
	Invocation  ProvenanceInvocationSLSA02 `json:"invocation,omitempty"`
	BuildConfig *BuildConfig               `json:"buildConfig,omitempty"`
	Metadata    *ProvenanceMetadataSLSA02  `json:"metadata,omitempty"`
}

type ProvenanceInvocationSLSA02 struct {
	ConfigSource slsa02.ConfigSource `json:"configSource,omitempty"`
	Parameters   Parameters          `json:"parameters,omitempty"`
	Environment  Environment         `json:"environment,omitempty"`
}

type ProvenanceMetadataSLSA02 struct {
	slsa02.ProvenanceMetadata
	BuildKitMetadata BuildKitMetadata `json:"https://mobyproject.org/buildkit@v1#metadata,omitempty"`
	Hermetic         bool             `json:"https://mobyproject.org/buildkit@v1#hermetic,omitempty"`
}

type ProvenancePredicateSLSA1 struct {
	slsa1.ProvenancePredicate
	BuildDefinition ProvenanceBuildDefinitionSLSA1 `json:"buildDefinition,omitempty"`
	RunDetails      ProvenanceRunDetailsSLSA1      `json:"runDetails,omitempty"`
}

type ProvenanceBuildDefinitionSLSA1 struct {
	slsa1.ProvenanceBuildDefinition
	ExternalParameters ProvenanceExternalParametersSLSA1 `json:"externalParameters,omitempty"`
	InternalParameters ProvenanceInternalParametersSLSA1 `json:"internalParameters,omitempty"`
}

type ProvenanceRunDetailsSLSA1 struct {
	slsa1.ProvenanceRunDetails
	Metadata *ProvenanceMetadataSLSA1 `json:"metadata,omitempty"`
}

type ProvenanceExternalParametersSLSA1 struct {
	ConfigSource ProvenanceConfigSourceSLSA1 `json:"configSource,omitempty"`
	Request      Parameters                  `json:"request,omitempty"`
}

type ProvenanceConfigSourceSLSA1 struct {
	URI    string         `json:"uri,omitempty"`
	Digest slsa.DigestSet `json:"digest,omitempty"`
	Path   string         `json:"path,omitempty"`
}

type ProvenanceInternalParametersSLSA1 struct {
	BuildConfig     *BuildConfig `json:"buildConfig,omitempty"`
	BuilderPlatform string       `json:"builderPlatform"`
}

type ProvenanceMetadataSLSA1 struct {
	slsa1.BuildMetadata
	BuildKitMetadata BuildKitMetadata `json:"buildkit_metadata,omitempty"`
	Hermetic         bool             `json:"buildkit_hermetic,omitempty"`
	Completeness     BuildKitComplete `json:"buildkit_completeness,omitempty"`
	Reproducible     bool             `json:"buildkit_reproducible,omitempty"`
}

type Parameters struct {
	Frontend string            `json:"frontend,omitempty"`
	Args     map[string]string `json:"args,omitempty"`
	Secrets  []*Secret         `json:"secrets,omitempty"`
	SSH      []*SSH            `json:"ssh,omitempty"`
	Locals   []*LocalSource    `json:"locals,omitempty"`
	// TODO: select export attributes
	// TODO: frontend inputs
}

type Environment struct {
	Platform string `json:"platform"`
}

type BuildKitMetadata struct {
	VCS      map[string]string                  `json:"vcs,omitempty"`
	Source   *Source                            `json:"source,omitempty"`
	Layers   map[string][][]ocispecs.Descriptor `json:"layers,omitempty"`
	SysUsage []*resourcestypes.SysSample        `json:"sysUsage,omitempty"`
}

type BuildKitComplete struct {
	Request              bool `json:"request"`
	ResolvedDependencies bool `json:"resolvedDependencies"`
}

// ConvertSLSA02ToSLSA1 converts a SLSA 0.2 provenance predicate to a SLSA 1.0
// provenance predicate.
// FIXME: It should be the other way around when v1 is the default.
func ConvertSLSA02ToSLSA1(p02 *ProvenancePredicateSLSA02) *ProvenancePredicateSLSA1 {
	if p02 == nil {
		return nil
	}

	var resolvedDeps []slsa1.ResourceDescriptor
	for _, m := range p02.Materials {
		resolvedDeps = append(resolvedDeps, slsa1.ResourceDescriptor{
			URI:    m.URI,
			Digest: m.Digest,
		})
	}

	buildDef := ProvenanceBuildDefinitionSLSA1{
		ProvenanceBuildDefinition: slsa1.ProvenanceBuildDefinition{
			BuildType:            "https://github.com/moby/buildkit/blob/master/docs/attestations/slsa-definitions.md",
			ResolvedDependencies: resolvedDeps,
		},
		ExternalParameters: ProvenanceExternalParametersSLSA1{
			ConfigSource: ProvenanceConfigSourceSLSA1{
				URI:    p02.Invocation.ConfigSource.URI,
				Digest: p02.Invocation.ConfigSource.Digest,
				Path:   p02.Invocation.ConfigSource.EntryPoint,
			},
			Request: p02.Invocation.Parameters,
		},
		InternalParameters: ProvenanceInternalParametersSLSA1{
			BuildConfig:     p02.BuildConfig,
			BuilderPlatform: p02.Invocation.Environment.Platform,
		},
	}

	var meta *ProvenanceMetadataSLSA1
	if p02.Metadata != nil {
		meta = &ProvenanceMetadataSLSA1{
			BuildMetadata: slsa1.BuildMetadata{
				InvocationID: p02.Metadata.BuildInvocationID,
				StartedOn:    p02.Metadata.BuildStartedOn,
				FinishedOn:   p02.Metadata.BuildFinishedOn,
			},
			BuildKitMetadata: p02.Metadata.BuildKitMetadata,
			Hermetic:         p02.Metadata.Hermetic,
			Completeness: BuildKitComplete{
				Request:              p02.Metadata.Completeness.Parameters,
				ResolvedDependencies: p02.Metadata.Completeness.Materials,
			},
			Reproducible: p02.Metadata.Reproducible,
		}
	}

	runDetails := ProvenanceRunDetailsSLSA1{
		ProvenanceRunDetails: slsa1.ProvenanceRunDetails{
			Builder: slsa1.Builder{
				ID: p02.Builder.ID,
				// TODO: handle builder components versions
				// Version: map[string]string{
				// 	"buildkit": version.Version,
				// },
			},
		},
		Metadata: meta,
	}

	return &ProvenancePredicateSLSA1{
		BuildDefinition: buildDef,
		RunDetails:      runDetails,
	}
}
