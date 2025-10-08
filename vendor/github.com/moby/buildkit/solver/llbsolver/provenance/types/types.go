package types

import (
	"encoding/json"
	"maps"
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
	BuildKitBuildType1  = "https://github.com/moby/buildkit/blob/master/docs/attestations/slsa-definitions.md"
	BuildKitBuildType02 = "https://mobyproject.org/buildkit@v1"

	ProvenanceSLSA1  = ProvenanceSLSA("v1")
	ProvenanceSLSA02 = ProvenanceSLSA("v0.2")
)

type ProvenanceSLSA string

var provenanceSLSAs = []ProvenanceSLSA{
	ProvenanceSLSA1,
	ProvenanceSLSA02,
}

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
	Invocation  ProvenanceInvocationSLSA02 `json:"invocation"`
	BuildConfig *BuildConfig               `json:"buildConfig,omitempty"`
	Metadata    *ProvenanceMetadataSLSA02  `json:"metadata,omitempty"`
}

type ProvenanceInvocationSLSA02 struct {
	ConfigSource slsa02.ConfigSource `json:"configSource"`
	Parameters   Parameters          `json:"parameters"`
	Environment  Environment         `json:"environment"`
}

type ProvenanceMetadataSLSA02 struct {
	slsa02.ProvenanceMetadata
	BuildKitMetadata BuildKitMetadata `json:"https://mobyproject.org/buildkit@v1#metadata"`
	Hermetic         bool             `json:"https://mobyproject.org/buildkit@v1#hermetic,omitempty"`
}

type ProvenancePredicateSLSA1 struct {
	slsa1.ProvenancePredicate
	BuildDefinition ProvenanceBuildDefinitionSLSA1 `json:"buildDefinition"`
	RunDetails      ProvenanceRunDetailsSLSA1      `json:"runDetails"`
}

type ProvenanceBuildDefinitionSLSA1 struct {
	slsa1.ProvenanceBuildDefinition
	ExternalParameters ProvenanceExternalParametersSLSA1 `json:"externalParameters"`
	InternalParameters ProvenanceInternalParametersSLSA1 `json:"internalParameters"`
}

type ProvenanceRunDetailsSLSA1 struct {
	slsa1.ProvenanceRunDetails
	Metadata *ProvenanceMetadataSLSA1 `json:"metadata,omitempty"`
}

type ProvenanceExternalParametersSLSA1 struct {
	ConfigSource ProvenanceConfigSourceSLSA1 `json:"configSource"`
	Request      Parameters                  `json:"request"`
}

type ProvenanceConfigSourceSLSA1 struct {
	URI    string         `json:"uri,omitempty"`
	Digest slsa.DigestSet `json:"digest,omitempty"`
	Path   string         `json:"path,omitempty"`
}

type ProvenanceInternalParametersSLSA1 struct {
	BuildConfig     *BuildConfig `json:"buildConfig,omitempty"`
	BuilderPlatform string       `json:"builderPlatform"`
	ProvenanceCustomEnv
}

type ProvenanceMetadataSLSA1 struct {
	slsa1.BuildMetadata
	BuildKitMetadata BuildKitMetadata `json:"buildkit_metadata"`
	Hermetic         bool             `json:"buildkit_hermetic,omitempty"`
	Completeness     BuildKitComplete `json:"buildkit_completeness"`
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
	ProvenanceCustomEnv
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

type ProvenanceCustomEnv map[string]any

// ConvertToSLSA02 converts to a SLSA v0.2 provenance predicate.
func (p *ProvenancePredicateSLSA1) ConvertToSLSA02() *ProvenancePredicateSLSA02 {
	var materials []slsa02.ProvenanceMaterial
	for _, m := range p.BuildDefinition.ResolvedDependencies {
		materials = append(materials, slsa02.ProvenanceMaterial{
			URI:    m.URI,
			Digest: m.Digest,
		})
	}

	var meta *ProvenanceMetadataSLSA02
	if p.RunDetails.Metadata != nil {
		meta = &ProvenanceMetadataSLSA02{
			ProvenanceMetadata: slsa02.ProvenanceMetadata{
				BuildInvocationID: p.RunDetails.Metadata.InvocationID,
				BuildStartedOn:    p.RunDetails.Metadata.StartedOn,
				BuildFinishedOn:   p.RunDetails.Metadata.FinishedOn,
				Completeness: slsa02.ProvenanceComplete{
					Parameters:  p.RunDetails.Metadata.Completeness.Request,
					Environment: true,
					Materials:   p.RunDetails.Metadata.Completeness.ResolvedDependencies,
				},
				Reproducible: p.RunDetails.Metadata.Reproducible,
			},
			BuildKitMetadata: p.RunDetails.Metadata.BuildKitMetadata,
			Hermetic:         p.RunDetails.Metadata.Hermetic,
		}
	}

	return &ProvenancePredicateSLSA02{
		ProvenancePredicate: slsa02.ProvenancePredicate{
			Builder: slsa02.ProvenanceBuilder{
				ID: p.RunDetails.Builder.ID,
			},
			BuildType: BuildKitBuildType02,
			Materials: materials,
		},
		Invocation: ProvenanceInvocationSLSA02{
			ConfigSource: slsa02.ConfigSource{
				URI:        p.BuildDefinition.ExternalParameters.ConfigSource.URI,
				Digest:     p.BuildDefinition.ExternalParameters.ConfigSource.Digest,
				EntryPoint: p.BuildDefinition.ExternalParameters.ConfigSource.Path,
			},
			Parameters: p.BuildDefinition.ExternalParameters.Request,
			Environment: Environment{
				Platform:            p.BuildDefinition.InternalParameters.BuilderPlatform,
				ProvenanceCustomEnv: p.BuildDefinition.InternalParameters.ProvenanceCustomEnv,
			},
		},
		BuildConfig: p.BuildDefinition.InternalParameters.BuildConfig,
		Metadata:    meta,
	}
}

// ConvertToSLSA1 converts to a SLSA v1 provenance predicate.
func (p *ProvenancePredicateSLSA02) ConvertToSLSA1() *ProvenancePredicateSLSA1 {
	var resolvedDeps []slsa1.ResourceDescriptor
	for _, m := range p.Materials {
		resolvedDeps = append(resolvedDeps, slsa1.ResourceDescriptor{
			URI:    m.URI,
			Digest: m.Digest,
		})
	}

	buildDef := ProvenanceBuildDefinitionSLSA1{
		ProvenanceBuildDefinition: slsa1.ProvenanceBuildDefinition{
			BuildType:            BuildKitBuildType1,
			ResolvedDependencies: resolvedDeps,
		},
		ExternalParameters: ProvenanceExternalParametersSLSA1{
			ConfigSource: ProvenanceConfigSourceSLSA1{
				URI:    p.Invocation.ConfigSource.URI,
				Digest: p.Invocation.ConfigSource.Digest,
				Path:   p.Invocation.ConfigSource.EntryPoint,
			},
			Request: p.Invocation.Parameters,
		},
		InternalParameters: ProvenanceInternalParametersSLSA1{
			BuildConfig:         p.BuildConfig,
			BuilderPlatform:     p.Invocation.Environment.Platform,
			ProvenanceCustomEnv: p.Invocation.Environment.ProvenanceCustomEnv,
		},
	}

	var meta *ProvenanceMetadataSLSA1
	if p.Metadata != nil {
		meta = &ProvenanceMetadataSLSA1{
			BuildMetadata: slsa1.BuildMetadata{
				InvocationID: p.Metadata.BuildInvocationID,
				StartedOn:    p.Metadata.BuildStartedOn,
				FinishedOn:   p.Metadata.BuildFinishedOn,
			},
			BuildKitMetadata: p.Metadata.BuildKitMetadata,
			Hermetic:         p.Metadata.Hermetic,
			Completeness: BuildKitComplete{
				Request:              p.Metadata.Completeness.Parameters,
				ResolvedDependencies: p.Metadata.Completeness.Materials,
			},
			Reproducible: p.Metadata.Reproducible,
		}
	}

	runDetails := ProvenanceRunDetailsSLSA1{
		ProvenanceRunDetails: slsa1.ProvenanceRunDetails{
			Builder: slsa1.Builder{
				ID: p.Builder.ID,
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

// MarshalJSON flattens ProvenanceCustomEnv into top level.
func (p ProvenanceInternalParametersSLSA1) MarshalJSON() ([]byte, error) {
	type Alias ProvenanceInternalParametersSLSA1
	base, err := json.Marshal(Alias(p))
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(base, &m); err != nil {
		return nil, err
	}
	maps.Copy(m, p.ProvenanceCustomEnv)
	delete(m, "ProvenanceCustomEnv")
	return json.Marshal(m)
}

// UnmarshalJSON fills both struct fields and flattened custom env.
func (p *ProvenanceInternalParametersSLSA1) UnmarshalJSON(data []byte) error {
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}

	type Alias ProvenanceInternalParametersSLSA1
	var a Alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}

	// Unmarshal known struct again to identify its keys
	structBytes, err := json.Marshal(a)
	if err != nil {
		return err
	}
	var known map[string]any
	if err := json.Unmarshal(structBytes, &known); err != nil {
		return err
	}

	for k := range known {
		delete(m, k)
	}

	*p = ProvenanceInternalParametersSLSA1(a)
	p.ProvenanceCustomEnv = m
	return nil
}

func (p Environment) MarshalJSON() ([]byte, error) {
	type Alias Environment
	base, err := json.Marshal(Alias(p))
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(base, &m); err != nil {
		return nil, err
	}
	maps.Copy(m, p.ProvenanceCustomEnv)
	delete(m, "ProvenanceCustomEnv")
	return json.Marshal(m)
}

func (p *Environment) UnmarshalJSON(data []byte) error {
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}

	type Alias Environment
	var a Alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	// Unmarshal known struct again to identify its keys
	structBytes, err := json.Marshal(a)
	if err != nil {
		return err
	}
	var known map[string]any
	if err := json.Unmarshal(structBytes, &known); err != nil {
		return err
	}

	for k := range known {
		delete(m, k)
	}
	*p = Environment(a)
	p.ProvenanceCustomEnv = m
	return nil
}
