package container

import "github.com/moby/moby/api/types/common"

// ExecCreateResponse is the response for a successful exec-create request.
// It holds the ID of the exec that was created.
//
// TODO(thaJeztah): make this a distinct type.
type ExecCreateResponse = common.IDResponse

// ExecOptions is a small subset of the Config struct that holds the configuration
// for the exec feature of docker.
type ExecOptions struct {
	User         string   // User that will run the command
	Privileged   bool     // Is the container in privileged mode
	Tty          bool     // Attach standard streams to a tty.
	ConsoleSize  *[2]uint `json:",omitempty"` // Initial console size [height, width]
	AttachStdin  bool     // Attach the standard input, makes possible user interaction
	AttachStderr bool     // Attach the standard error
	AttachStdout bool     // Attach the standard output
	DetachKeys   string   // Escape keys for detach
	Env          []string // Environment variables
	WorkingDir   string   // Working directory
	Cmd          []string // Execution commands and args

	// Deprecated: the Detach field is not used, and will be removed in a future release.
	Detach bool
}

// ExecStartOptions is a temp struct used by execStart
// Config fields is part of ExecConfig in runconfig package
type ExecStartOptions struct {
	// ExecStart will first check if it's detached
	Detach bool
	// Check if there's a tty
	Tty bool
	// Terminal size [height, width], unused if Tty == false
	ConsoleSize *[2]uint `json:",omitempty"`
}

// ExecAttachOptions is a temp struct used by execAttach.
//
// TODO(thaJeztah): make this a separate type; ContainerExecAttach does not use the Detach option, and cannot run detached.
type ExecAttachOptions = ExecStartOptions

// ExecInspect holds information returned by exec inspect.
//
// It is used by the client to unmarshal a [ExecInspectResponse],
// but currently only provides a subset of the information included
// in that type.
//
// TODO(thaJeztah): merge [ExecInspect] and [ExecInspectResponse],
type ExecInspect struct {
	ExecID      string `json:"ID"`
	ContainerID string `json:"ContainerID"`
	Running     bool   `json:"Running"`
	ExitCode    int    `json:"ExitCode"`
	Pid         int    `json:"Pid"`
}

// ExecInspectResponse is the API response for the "GET /exec/{id}/json"
// endpoint and holds information about and exec.
type ExecInspectResponse struct {
	ID            string `json:"ID"`
	Running       bool   `json:"Running"`
	ExitCode      *int   `json:"ExitCode"`
	ProcessConfig *ExecProcessConfig
	OpenStdin     bool   `json:"OpenStdin"`
	OpenStderr    bool   `json:"OpenStderr"`
	OpenStdout    bool   `json:"OpenStdout"`
	CanRemove     bool   `json:"CanRemove"`
	ContainerID   string `json:"ContainerID"`
	DetachKeys    []byte `json:"DetachKeys"`
	Pid           int    `json:"Pid"`
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
