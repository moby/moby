package backend // import "github.com/docker/docker/api/types/backend"

import (
	"io"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/pkg/streamformatter"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
)

// PullOption defines different modes for accessing images
type PullOption int

const (
	// PullOptionNoPull only returns local images
	PullOptionNoPull PullOption = iota
	// PullOptionForcePull always tries to pull a ref from the registry first
	PullOptionForcePull
	// PullOptionPreferLocal uses local image if it exists, otherwise pulls
	PullOptionPreferLocal
)

// ProgressWriter is a data object to transport progress streams to the client
type ProgressWriter struct {
	Output             io.Writer
	StdoutFormatter    io.Writer
	StderrFormatter    io.Writer
	AuxFormatter       *streamformatter.AuxFormatter
	ProgressReaderFunc func(io.ReadCloser) io.ReadCloser
}

// BuildConfig is the configuration used by a BuildManager to start a build
type BuildConfig struct {
	Source         io.ReadCloser
	ProgressWriter ProgressWriter
	Options        *types.ImageBuildOptions
}

// GetImageAndLayerOptions are the options supported by GetImageAndReleasableLayer
type GetImageAndLayerOptions struct {
	PullOption PullOption
	AuthConfig map[string]registry.AuthConfig
	Output     io.Writer
	Platform   *specs.Platform
}
