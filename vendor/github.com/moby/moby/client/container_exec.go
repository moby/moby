package client

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/moby/moby/api/types/container"
)

// ExecCreateOptions is a small subset of the Config struct that holds the configuration
// for the exec feature of docker.
type ExecCreateOptions struct {
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
}

// ContainerExecCreate creates a new exec configuration to run an exec process.
func (cli *Client) ContainerExecCreate(ctx context.Context, containerID string, options ExecCreateOptions) (container.ExecCreateResponse, error) {
	containerID, err := trimID("container", containerID)
	if err != nil {
		return container.ExecCreateResponse{}, err
	}

	req := container.ExecCreateRequest{
		User:         options.User,
		Privileged:   options.Privileged,
		Tty:          options.Tty,
		ConsoleSize:  options.ConsoleSize,
		AttachStdin:  options.AttachStdin,
		AttachStderr: options.AttachStderr,
		AttachStdout: options.AttachStdout,
		DetachKeys:   options.DetachKeys,
		Env:          options.Env,
		WorkingDir:   options.WorkingDir,
		Cmd:          options.Cmd,
	}

	resp, err := cli.post(ctx, "/containers/"+containerID+"/exec", nil, req, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return container.ExecCreateResponse{}, err
	}

	var response container.ExecCreateResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	return response, err
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

// ContainerExecStart starts an exec process already created in the docker host.
func (cli *Client) ContainerExecStart(ctx context.Context, execID string, config ExecStartOptions) error {
	req := container.ExecStartRequest{
		Detach:      config.Detach,
		Tty:         config.Tty,
		ConsoleSize: config.ConsoleSize,
	}
	resp, err := cli.post(ctx, "/exec/"+execID+"/start", nil, req, nil)
	defer ensureReaderClosed(resp)
	return err
}

// ExecAttachOptions is a temp struct used by execAttach.
//
// TODO(thaJeztah): make this a separate type; ContainerExecAttach does not use the Detach option, and cannot run detached.
type ExecAttachOptions = ExecStartOptions

// ContainerExecAttach attaches a connection to an exec process in the server.
//
// It returns a [HijackedResponse] with the hijacked connection
// and a reader to get output. It's up to the called to close
// the hijacked connection by calling [HijackedResponse.Close].
//
// The stream format on the response uses one of two formats:
//
//   - If the container is using a TTY, there is only a single stream (stdout)
//     and data is copied directly from the container output stream, no extra
//     multiplexing or headers.
//   - If the container is *not* using a TTY, streams for stdout and stderr are
//     multiplexed.
//
// You can use [stdcopy.StdCopy] to demultiplex this stream. Refer to
// [Client.ContainerAttach] for details about the multiplexed stream.
//
// [stdcopy.StdCopy]: https://pkg.go.dev/github.com/moby/moby/client/pkg/stdcopy#StdCopy
func (cli *Client) ContainerExecAttach(ctx context.Context, execID string, config ExecAttachOptions) (HijackedResponse, error) {
	req := container.ExecStartRequest{
		Detach:      config.Detach,
		Tty:         config.Tty,
		ConsoleSize: config.ConsoleSize,
	}
	return cli.postHijacked(ctx, "/exec/"+execID+"/start", nil, req, http.Header{
		"Content-Type": {"application/json"},
	})
}

// ExecInspect holds information returned by exec inspect.
//
// It provides a subset of the information included in [container.ExecInspectResponse].
//
// TODO(thaJeztah): include all fields of [container.ExecInspectResponse] ?
type ExecInspect struct {
	ExecID      string `json:"ID"`
	ContainerID string `json:"ContainerID"`
	Running     bool   `json:"Running"`
	ExitCode    int    `json:"ExitCode"`
	Pid         int    `json:"Pid"`
}

// ContainerExecInspect returns information about a specific exec process on the docker host.
func (cli *Client) ContainerExecInspect(ctx context.Context, execID string) (ExecInspect, error) {
	resp, err := cli.get(ctx, "/exec/"+execID+"/json", nil, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return ExecInspect{}, err
	}

	var response container.ExecInspectResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		return ExecInspect{}, err
	}

	var ec int
	if response.ExitCode != nil {
		ec = *response.ExitCode
	}

	return ExecInspect{
		ExecID:      response.ID,
		ContainerID: response.ContainerID,
		Running:     response.Running,
		ExitCode:    ec,
		Pid:         response.Pid,
	}, nil
}
