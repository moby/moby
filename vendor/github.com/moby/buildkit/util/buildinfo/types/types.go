package binfotypes

import (
	srctypes "github.com/moby/buildkit/source/types"
)

// ImageConfigField defines the key of build dependencies.
const ImageConfigField = "moby.buildkit.buildinfo.v1"

// ImageConfig defines the structure of build dependencies
// inside image config.
type ImageConfig struct {
	BuildInfo string `json:"moby.buildkit.buildinfo.v1,omitempty"`
}

// BuildInfo defines the main structure added to image config as
// ImageConfigField key and returned in solver ExporterResponse as
// exptypes.ExporterBuildInfo key.
type BuildInfo struct {
	// Frontend defines the frontend used to build.
	Frontend string `json:"frontend,omitempty"`
	// Attrs defines build request attributes.
	Attrs map[string]*string `json:"attrs,omitempty"`
	// Sources defines build dependencies.
	Sources []Source `json:"sources,omitempty"`
	// Deps defines context dependencies.
	Deps map[string]BuildInfo `json:"deps,omitempty"`
}

// Source defines a build dependency.
type Source struct {
	// Type defines the SourceType source type (docker-image, git, http).
	Type SourceType `json:"type,omitempty"`
	// Ref is the reference of the source.
	Ref string `json:"ref,omitempty"`
	// Alias is a special field used to match with the actual source ref
	// because frontend might have already transformed a string user typed
	// before generating LLB.
	Alias string `json:"alias,omitempty"`
	// Pin is the source digest.
	Pin string `json:"pin,omitempty"`
}

// SourceType contains source type.
type SourceType string

// List of source types.
const (
	SourceTypeDockerImage SourceType = srctypes.DockerImageScheme
	SourceTypeGit         SourceType = srctypes.GitScheme
	SourceTypeHTTP        SourceType = srctypes.HTTPScheme
)
