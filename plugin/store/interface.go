package store

import "github.com/docker/docker/pkg/plugins"

// CompatPlugin is a abstraction to handle both new and legacy (v1) plugins.
type CompatPlugin interface {
	Client() *plugins.Client
	Name() string
	IsLegacy() bool
}
