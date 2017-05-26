package backend

import (
	"io"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/streamformatter"
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
	ForcePull  bool
	AuthConfig map[string]types.AuthConfig
	Output     io.Writer
}
