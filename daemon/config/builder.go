package config

import "github.com/docker/docker/api/types/filters"

// BuilderGCRule represents a GC rule for buildkit cache
type BuilderGCRule struct {
	All         bool         `json:",omitempty"`
	Filter      filters.Args `json:",omitempty"`
	KeepStorage string       `json:",omitempty"`
}

// BuilderGCConfig contains GC config for a buildkit builder
type BuilderGCConfig struct {
	Enabled            bool            `json:",omitempty"`
	Policy             []BuilderGCRule `json:",omitempty"`
	DefaultKeepStorage string          `json:",omitempty"`
}

// BuilderConfig contains config for the builder
type BuilderConfig struct {
	GC BuilderGCConfig `json:",omitempty"`
}
