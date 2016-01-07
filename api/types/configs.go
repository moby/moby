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

// ContainerWsAttachWithLogsConfig attach with websockets, since all
// stream data is delegated to the websocket to handle there.
type ContainerWsAttachWithLogsConfig struct {
	InStream             io.ReadCloser
	OutStream, ErrStream io.Writer
	Logs, Stream         bool
	DetachKeys           []byte
}

// ContainerAttachWithLogsConfig holds the streams to use when
// connecting to a container to view logs.
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

// ContainerLogsConfig holds configs for logging operations. Exists
// for users of the daemon to to pass it a logging configuration.
type ContainerLogsConfig struct {
	// if true stream log output
	Follow bool
	// if true include timestamps for each line of log output
	Timestamps bool
	// return that many lines of log output from the end
	Tail string
	// filter logs by returning on those entries after this time
	Since time.Time
	// whether or not to show stdout and stderr as well as log entries.
	UseStdout, UseStderr bool
	OutStream            io.Writer
	Stop                 <-chan bool
}

// ContainerStatsConfig holds information for configuring the runtime
// behavior of a daemon.ContainerStats() call.
type ContainerStatsConfig struct {
	Stream    bool
	OutStream io.Writer
	Stop      <-chan bool
	Version   version.Version
}

// ContainersConfig is the filtering specified by the user to iterate
// over containers.
type ContainersConfig struct {
	// if true show all containers, otherwise only running containers.
	All bool
	// show all containers created after this container id
	Since string
	// show all containers created before this container id
	Before string
	// number of containers to return at most
	Limit int
	// if true include the sizes of the containers
	Size bool
	// return only containers that match filters
	Filters string
}
