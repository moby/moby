package types

import (
	"github.com/docker/engine-api/types/container"
	"github.com/docker/engine-api/types/network"
)

// configs holds structs used for internal communication between the
// frontend (such as an http server) and the backend (such as the
// docker daemon).

// ContainerCreateConfig is the parameter set to ContainerCreate()
type ContainerCreateConfig struct {
	Name             string                    `json:",omitempty"`
	Config           *container.Config         `json:",omitempty"`
	HostConfig       *container.HostConfig     `json:",omitempty"`
	NetworkingConfig *network.NetworkingConfig `json:",omitempty"`
	AdjustCPUShares  bool                      `json:",omitempty"`
}

// ContainerRmConfig holds arguments for the container remove
// operation. This struct is used to tell the backend what operations
// to perform.
type ContainerRmConfig struct {
	ForceRemove, RemoveVolume, RemoveLink bool `json:",omitempty"`
}

// ContainerCommitConfig contains build configs for commit operation,
// and is used when making a commit with the current state of the container.
type ContainerCommitConfig struct {
	Pause   bool   `json:",omitempty"`
	Repo    string `json:",omitempty"`
	Tag     string `json:",omitempty"`
	Author  string `json:",omitempty"`
	Comment string `json:",omitempty"`
	// merge container config into commit config before commit
	MergeConfigs bool              `json:",omitempty"`
	Config       *container.Config `json:",omitempty"`
}

// ExecConfig is a small subset of the Config struct that hold the configuration
// for the exec feature of docker.
type ExecConfig struct {
	User         string   `json:",omitempty"` // User that will run the command
	Privileged   bool     `json:",omitempty"` // Is the container in privileged mode
	Tty          bool     `json:",omitempty"` // Attach standard streams to a tty.
	Container    string   `json:",omitempty"` // Name of the container (to execute in)
	AttachStdin  bool     `json:",omitempty"` // Attach the standard input, makes possible user interaction
	AttachStderr bool     `json:",omitempty"` // Attach the standard output
	AttachStdout bool     `json:",omitempty"` // Attach the standard error
	Detach       bool     `json:",omitempty"` // Execute in detach mode
	DetachKeys   string   `json:",omitempty"` // Escape keys for detach
	Cmd          []string `json:",omitempty"` // Execution commands and args
}
