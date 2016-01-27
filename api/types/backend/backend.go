// Package backend includes types to send information to server backends.
// TODO(calavera): This package is pending of extraction to engine-api
// when the server package is clean of daemon dependencies.
package backend

import (
	"io"
	"net/http"

	"github.com/docker/engine-api/types"
)

// ContainerAttachWithLogsConfig holds the streams to use when connecting to a container to view logs.
type ContainerAttachWithLogsConfig struct {
	Hijacker   http.Hijacker
	Upgrade    bool
	UseStdin   bool
	UseStdout  bool
	UseStderr  bool
	Logs       bool
	Stream     bool
	DetachKeys []byte
}

// ContainerWsAttachWithLogsConfig attach with websockets, since all
// stream data is delegated to the websocket to handle there.
type ContainerWsAttachWithLogsConfig struct {
	InStream   io.ReadCloser // Reader to attach to stdin of container
	OutStream  io.Writer     // Writer to attach to stdout of container
	ErrStream  io.Writer     // Writer to attach to stderr of container
	Logs       bool          // If true return log output
	Stream     bool          // If true return stream output
	DetachKeys []byte
}

// ContainerLogsConfig holds configs for logging operations. Exists
// for users of the backend to to pass it a logging configuration.
type ContainerLogsConfig struct {
	types.ContainerLogsOptions
	OutStream io.Writer
	Stop      <-chan bool
}

// ContainerStatsConfig holds information for configuring the runtime
// behavior of a backend.ContainerStats() call.
type ContainerStatsConfig struct {
	Stream    bool
	OutStream io.Writer
	Stop      <-chan bool
	Version   string
}
