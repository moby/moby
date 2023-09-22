package provenance

import (
	resourcestypes "github.com/moby/buildkit/executor/resources/types"
	"github.com/moby/buildkit/solver/pb"
	digest "github.com/opencontainers/go-digest"
)

type BuildConfig struct {
	Definition    []BuildStep              `json:"llbDefinition,omitempty"`
	DigestMapping map[digest.Digest]string `json:"digestMapping,omitempty"`
}

type BuildStep struct {
	ID            string                  `json:"id,omitempty"`
	Op            interface{}             `json:"op,omitempty"`
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
