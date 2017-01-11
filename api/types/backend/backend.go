// Package backend includes types to send information to server backends.
package backend

import (
	"io"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/streamformatter"
)

// ContainerAttachConfig holds the streams to use when connecting to a container to view logs.
type ContainerAttachConfig struct {
	GetStreams func() (io.ReadCloser, io.Writer, io.Writer, error)
	UseStdin   bool
	UseStdout  bool
	UseStderr  bool
	Logs       bool
	Stream     bool
	DetachKeys string

	// Used to signify that streams are multiplexed and therefore need a StdWriter to encode stdout/sderr messages accordingly.
	// TODO @cpuguy83: This shouldn't be needed. It was only added so that http and websocket endpoints can use the same function, and the websocket function was not using a stdwriter prior to this change...
	// HOWEVER, the websocket endpoint is using a single stream and SHOULD be encoded with stdout/stderr as is done for HTTP since it is still just a single stream.
	// Since such a change is an API change unrelated to the current changeset we'll keep it as is here and change separately.
	MuxStreams bool
}

// ContainerLogsConfig holds configs for logging operations. Exists
// for users of the backend to to pass it a logging configuration.
type ContainerLogsConfig struct {
	types.ContainerLogsOptions
	OutStream io.Writer
}

// ContainerStatsConfig holds information for configuring the runtime
// behavior of a backend.ContainerStats() call.
type ContainerStatsConfig struct {
	Stream    bool
	OutStream io.Writer
	Version   string
}

// ExecInspect holds information about a running process started
// with docker exec.
type ExecInspect struct {
	ID            string
	Running       bool
	ExitCode      *int
	ProcessConfig *ExecProcessConfig
	OpenStdin     bool
	OpenStderr    bool
	OpenStdout    bool
	CanRemove     bool
	ContainerID   string
	DetachKeys    []byte
	Pid           int
}

// ExecProcessConfig holds information about the exec process
// running on the host.
type ExecProcessConfig struct {
	Tty        bool     `json:"tty"`
	Entrypoint string   `json:"entrypoint"`
	Arguments  []string `json:"arguments"`
	Privileged *bool    `json:"privileged,omitempty"`
	User       string   `json:"user,omitempty"`
}

// ContainerCommitConfig is a wrapper around
// types.ContainerCommitConfig that also
// transports configuration changes for a container.
type ContainerCommitConfig struct {
	types.ContainerCommitConfig
	Changes []string
}

// ProgressWriter is an interface
// to transport progress streams.
type ProgressWriter struct {
	Output             io.Writer
	StdoutFormatter    *streamformatter.StdoutFormatter
	StderrFormatter    *streamformatter.StderrFormatter
	ProgressReaderFunc func(io.ReadCloser) io.ReadCloser
}
