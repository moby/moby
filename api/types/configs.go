package types

import (
	"io"
	"net/http"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/version"
)

// configs holds structs used for internal communication between the
// frontend (such as an http server) and the backend (such as the
// docker daemon).

// ContainersConfig is the filtering specified by the user to iterate over containers.
type ContainersConfig struct {
	All     bool   // If true show all containers, otherwise only running containers.
	Since   string // Show all containers created after this container id
	Before  string // Show all containers created before this container id
	Limit   int    // Number of containers to return at most
	Size    bool   // If true include the sizes of the containers in the response
	Filters string // Return only containers that match the filters
}

// ContainerCreateConfig is the parameter set to ContainerCreate()
type ContainerCreateConfig struct {
	Name            string
	Config          *container.Config
	HostConfig      *container.HostConfig
	AdjustCPUShares bool
}

// ContainerRmConfig holds arguments for the container remove
// operation. This struct is used to tell the backend what operations
// to perform.
type ContainerRmConfig struct {
	ForceRemove, RemoveVolume, RemoveLink bool
}

// ContainerCommitConfig contains build configs for commit operation,
// and is used when making a commit with the current state of the container.
type ContainerCommitConfig struct {
	Pause   bool
	Repo    string
	Tag     string
	Author  string
	Comment string
	// merge container config into commit config before commit
	MergeConfigs bool
	Config       *container.Config
}

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
// for users of the daemon to to pass it a logging configuration.
type ContainerLogsConfig struct {
	Follow     bool      // If true stream log output
	Timestamps bool      // If true include timestamps for each line of log output
	Tail       string    // Return that many lines of log output from the end
	Since      time.Time // Filter logs by returning only entries after this time
	UseStdout  bool      // Whether or not to show stdout output in addition to log entries
	UseStderr  bool      // Whether or not to show stderr output in addition to log entries
	OutStream  io.Writer
	Stop       <-chan bool
}

// ContainerStatsConfig holds information for configuring the runtime
// behavior of a daemon.ContainerStats() call.
type ContainerStatsConfig struct {
	Stream    bool
	OutStream io.Writer
	Stop      <-chan bool
	Version   version.Version
}

// ExecConfig is a small subset of the Config struct that hold the configuration
// for the exec feature of docker.
type ExecConfig struct {
	User         string   // User that will run the command
	Privileged   bool     // Is the container in privileged mode
	Tty          bool     // Attach standard streams to a tty.
	Container    string   // Name of the container (to execute in)
	AttachStdin  bool     // Attach the standard input, makes possible user interaction
	AttachStderr bool     // Attach the standard output
	AttachStdout bool     // Attach the standard error
	Detach       bool     // Execute in detach mode
	DetachKeys   string   // Escape keys for detach
	Cmd          []string // Execution commands and args
}
