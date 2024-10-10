package types

import (
	slsa02 "github.com/in-toto/in-toto-golang/in_toto/slsa_provenance/v0.2"
	resourcestypes "github.com/moby/buildkit/executor/resources/types"
	"github.com/moby/buildkit/solver/pb"
	digest "github.com/opencontainers/go-digest"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
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

type ProvenancePredicate struct {
	slsa02.ProvenancePredicate
	Invocation  ProvenanceInvocation `json:"invocation,omitempty"`
	BuildConfig *BuildConfig         `json:"buildConfig,omitempty"`
	Metadata    *ProvenanceMetadata  `json:"metadata,omitempty"`
}

type ProvenanceInvocation struct {
	ConfigSource slsa02.ConfigSource `json:"configSource,omitempty"`
	Parameters   Parameters          `json:"parameters,omitempty"`
	Environment  Environment         `json:"environment,omitempty"`
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

type ProvenanceMetadata struct {
	slsa02.ProvenanceMetadata
	BuildKitMetadata BuildKitMetadata `json:"https://mobyproject.org/buildkit@v1#metadata,omitempty"`
	Hermetic         bool             `json:"https://mobyproject.org/buildkit@v1#hermetic,omitempty"`
}

type BuildKitMetadata struct {
	VCS      map[string]string                  `json:"vcs,omitempty"`
	Source   *Source                            `json:"source,omitempty"`
	Layers   map[string][][]ocispecs.Descriptor `json:"layers,omitempty"`
	SysUsage []*resourcestypes.SysSample        `json:"sysUsage,omitempty"`
}
