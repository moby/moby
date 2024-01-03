/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package runc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-spec/specs-go/features"
)

// Format is the type of log formatting options available
type Format string

// TopResults represents the structured data of the full ps output
type TopResults struct {
	// Processes running in the container, where each is process is an array of values corresponding to the headers
	Processes [][]string `json:"Processes"`

	// Headers are the names of the columns
	Headers []string `json:"Headers"`
}

const (
	none Format = ""
	// JSON represents the JSON format
	JSON Format = "json"
	// Text represents plain text format
	Text Format = "text"
)

// DefaultCommand is the default command for Runc
var DefaultCommand = "runc"

// Runc is the client to the runc cli
type Runc struct {
	// Command overrides the name of the runc binary. If empty, DefaultCommand
	// is used.
	Command   string
	Root      string
	Debug     bool
	Log       string
	LogFormat Format
	// PdeathSignal sets a signal the child process will receive when the
	// parent dies.
	//
	// When Pdeathsig is set, command invocations will call runtime.LockOSThread
	// to prevent OS thread termination from spuriously triggering the
	// signal. See https://github.com/golang/go/issues/27505 and
	// https://github.com/golang/go/blob/126c22a09824a7b52c019ed9a1d198b4e7781676/src/syscall/exec_linux.go#L48-L51
	//
	// A program with GOMAXPROCS=1 might hang because of the use of
	// runtime.LockOSThread. Callers should ensure they retain at least one
	// unlocked thread.
	PdeathSignal syscall.Signal // using syscall.Signal to allow compilation on non-unix (unix.Syscall is an alias for syscall.Signal)
	Setpgid      bool

	// Criu sets the path to the criu binary used for checkpoint and restore.
	//
	// Deprecated: runc option --criu is now ignored (with a warning), and the
	// option will be removed entirely in a future release. Users who need a non-
	// standard criu binary should rely on the standard way of looking up binaries
	// in $PATH.
	Criu          string
	SystemdCgroup bool
	Rootless      *bool // nil stands for "auto"
	ExtraArgs     []string
}

// List returns all containers created inside the provided runc root directory
func (r *Runc) List(context context.Context) ([]*Container, error) {
	data, err := r.cmdOutput(r.command(context, "list", "--format=json"), false, nil)
	defer putBuf(data)
	if err != nil {
		return nil, err
	}
	var out []*Container
	if err := json.Unmarshal(data.Bytes(), &out); err != nil {
		return nil, err
	}
	return out, nil
}

// State returns the state for the container provided by id
func (r *Runc) State(context context.Context, id string) (*Container, error) {
	data, err := r.cmdOutput(r.command(context, "state", id), true, nil)
	defer putBuf(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %s", err, data.String())
	}
	var c Container
	if err := json.Unmarshal(data.Bytes(), &c); err != nil {
		return nil, err
	}
	return &c, nil
}

// ConsoleSocket handles the path of the socket for console access
type ConsoleSocket interface {
	Path() string
}

// CreateOpts holds all the options information for calling runc with supported options
type CreateOpts struct {
	IO
	// PidFile is a path to where a pid file should be created
	PidFile       string
	ConsoleSocket ConsoleSocket
	Detach        bool
	NoPivot       bool
	NoNewKeyring  bool
	ExtraFiles    []*os.File
	Started       chan<- int
	ExtraArgs     []string
}

func (o *CreateOpts) args() (out []string, err error) {
	if o.PidFile != "" {
		abs, err := filepath.Abs(o.PidFile)
		if err != nil {
			return nil, err
		}
		out = append(out, "--pid-file", abs)
	}
	if o.ConsoleSocket != nil {
		out = append(out, "--console-socket", o.ConsoleSocket.Path())
	}
	if o.NoPivot {
		out = append(out, "--no-pivot")
	}
	if o.NoNewKeyring {
		out = append(out, "--no-new-keyring")
	}
	if o.Detach {
		out = append(out, "--detach")
	}
	if o.ExtraFiles != nil {
		out = append(out, "--preserve-fds", strconv.Itoa(len(o.ExtraFiles)))
	}
	if len(o.ExtraArgs) > 0 {
		out = append(out, o.ExtraArgs...)
	}
	return out, nil
}

func (r *Runc) startCommand(cmd *exec.Cmd) (chan Exit, error) {
	if r.PdeathSignal != 0 {
		return Monitor.StartLocked(cmd)
	}
	return Monitor.Start(cmd)
}

// Create creates a new container and returns its pid if it was created successfully
func (r *Runc) Create(context context.Context, id, bundle string, opts *CreateOpts) error {
	args := []string{"create", "--bundle", bundle}
	if opts == nil {
		opts = &CreateOpts{}
	}

	oargs, err := opts.args()
	if err != nil {
		return err
	}
	args = append(args, oargs...)
	cmd := r.command(context, append(args, id)...)
	if opts.IO != nil {
		opts.Set(cmd)
	}
	cmd.ExtraFiles = opts.ExtraFiles

	if cmd.Stdout == nil && cmd.Stderr == nil {
		data, err := r.cmdOutput(cmd, true, nil)
		defer putBuf(data)
		if err != nil {
			return fmt.Errorf("%s: %s", err, data.String())
		}
		return nil
	}
	ec, err := r.startCommand(cmd)
	if err != nil {
		return err
	}
	if opts.IO != nil {
		if c, ok := opts.IO.(StartCloser); ok {
			if err := c.CloseAfterStart(); err != nil {
				return err
			}
		}
	}
	status, err := Monitor.Wait(cmd, ec)
	if err == nil && status != 0 {
		err = fmt.Errorf("%s did not terminate successfully: %w", cmd.Args[0], &ExitError{status})
	}
	return err
}

// Start will start an already created container
func (r *Runc) Start(context context.Context, id string) error {
	return r.runOrError(r.command(context, "start", id))
}

// ExecOpts holds optional settings when starting an exec process with runc
type ExecOpts struct {
	IO
	PidFile       string
	ConsoleSocket ConsoleSocket
	Detach        bool
	Started       chan<- int
	ExtraArgs     []string
}

func (o *ExecOpts) args() (out []string, err error) {
	if o.ConsoleSocket != nil {
		out = append(out, "--console-socket", o.ConsoleSocket.Path())
	}
	if o.Detach {
		out = append(out, "--detach")
	}
	if o.PidFile != "" {
		abs, err := filepath.Abs(o.PidFile)
		if err != nil {
			return nil, err
		}
		out = append(out, "--pid-file", abs)
	}
	if len(o.ExtraArgs) > 0 {
		out = append(out, o.ExtraArgs...)
	}
	return out, nil
}

// Exec executes an additional process inside the container based on a full
// OCI Process specification
func (r *Runc) Exec(context context.Context, id string, spec specs.Process, opts *ExecOpts) error {
	if opts == nil {
		opts = &ExecOpts{}
	}
	if opts.Started != nil {
		defer close(opts.Started)
	}
	f, err := os.CreateTemp(os.Getenv("XDG_RUNTIME_DIR"), "runc-process")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	err = json.NewEncoder(f).Encode(spec)
	f.Close()
	if err != nil {
		return err
	}
	args := []string{"exec", "--process", f.Name()}
	oargs, err := opts.args()
	if err != nil {
		return err
	}
	args = append(args, oargs...)
	cmd := r.command(context, append(args, id)...)
	if opts.IO != nil {
		opts.Set(cmd)
	}
	if cmd.Stdout == nil && cmd.Stderr == nil {
		data, err := r.cmdOutput(cmd, true, opts.Started)
		defer putBuf(data)
		if err != nil {
			return fmt.Errorf("%w: %s", err, data.String())
		}
		return nil
	}
	ec, err := r.startCommand(cmd)
	if err != nil {
		return err
	}
	if opts.Started != nil {
		opts.Started <- cmd.Process.Pid
	}
	if opts.IO != nil {
		if c, ok := opts.IO.(StartCloser); ok {
			if err := c.CloseAfterStart(); err != nil {
				return err
			}
		}
	}
	status, err := Monitor.Wait(cmd, ec)
	if err == nil && status != 0 {
		err = fmt.Errorf("%s did not terminate successfully: %w", cmd.Args[0], &ExitError{status})
	}
	return err
}

// Run runs the create, start, delete lifecycle of the container
// and returns its exit status after it has exited
func (r *Runc) Run(context context.Context, id, bundle string, opts *CreateOpts) (int, error) {
	if opts == nil {
		opts = &CreateOpts{}
	}
	if opts.Started != nil {
		defer close(opts.Started)
	}
	args := []string{"run", "--bundle", bundle}
	oargs, err := opts.args()
	if err != nil {
		return -1, err
	}
	args = append(args, oargs...)
	cmd := r.command(context, append(args, id)...)
	if opts.IO != nil {
		opts.Set(cmd)
	}
	cmd.ExtraFiles = opts.ExtraFiles
	ec, err := r.startCommand(cmd)
	if err != nil {
		return -1, err
	}
	if opts.Started != nil {
		opts.Started <- cmd.Process.Pid
	}
	status, err := Monitor.Wait(cmd, ec)
	if err == nil && status != 0 {
		err = fmt.Errorf("%s did not terminate successfully: %w", cmd.Args[0], &ExitError{status})
	}
	return status, err
}

// DeleteOpts holds the deletion options for calling `runc delete`
type DeleteOpts struct {
	Force     bool
	ExtraArgs []string
}

func (o *DeleteOpts) args() (out []string) {
	if o.Force {
		out = append(out, "--force")
	}
	if len(o.ExtraArgs) > 0 {
		out = append(out, o.ExtraArgs...)
	}
	return out
}

// Delete deletes the container
func (r *Runc) Delete(context context.Context, id string, opts *DeleteOpts) error {
	args := []string{"delete"}
	if opts != nil {
		args = append(args, opts.args()...)
	}
	return r.runOrError(r.command(context, append(args, id)...))
}

// KillOpts specifies options for killing a container and its processes
type KillOpts struct {
	All       bool
	ExtraArgs []string
}

func (o *KillOpts) args() (out []string) {
	if o.All {
		out = append(out, "--all")
	}
	if len(o.ExtraArgs) > 0 {
		out = append(out, o.ExtraArgs...)
	}
	return out
}

// Kill sends the specified signal to the container
func (r *Runc) Kill(context context.Context, id string, sig int, opts *KillOpts) error {
	args := []string{
		"kill",
	}
	if opts != nil {
		args = append(args, opts.args()...)
	}
	return r.runOrError(r.command(context, append(args, id, strconv.Itoa(sig))...))
}

// Stats return the stats for a container like cpu, memory, and io
func (r *Runc) Stats(context context.Context, id string) (*Stats, error) {
	cmd := r.command(context, "events", "--stats", id)
	rd, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	ec, err := r.startCommand(cmd)
	if err != nil {
		return nil, err
	}
	defer func() {
		rd.Close()
		Monitor.Wait(cmd, ec)
	}()
	var e Event
	if err := json.NewDecoder(rd).Decode(&e); err != nil {
		return nil, err
	}
	return e.Stats, nil
}

// Events returns an event stream from runc for a container with stats and OOM notifications
func (r *Runc) Events(context context.Context, id string, interval time.Duration) (chan *Event, error) {
	cmd := r.command(context, "events", fmt.Sprintf("--interval=%ds", int(interval.Seconds())), id)
	rd, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	ec, err := r.startCommand(cmd)
	if err != nil {
		rd.Close()
		return nil, err
	}
	var (
		dec = json.NewDecoder(rd)
		c   = make(chan *Event, 128)
	)
	go func() {
		defer func() {
			close(c)
			rd.Close()
			Monitor.Wait(cmd, ec)
		}()
		for {
			var e Event
			if err := dec.Decode(&e); err != nil {
				if err == io.EOF {
					return
				}
				e = Event{
					Type: "error",
					Err:  err,
				}
			}
			c <- &e
		}
	}()
	return c, nil
}

// Pause the container with the provided id
func (r *Runc) Pause(context context.Context, id string) error {
	return r.runOrError(r.command(context, "pause", id))
}

// Resume the container with the provided id
func (r *Runc) Resume(context context.Context, id string) error {
	return r.runOrError(r.command(context, "resume", id))
}

// Ps lists all the processes inside the container returning their pids
func (r *Runc) Ps(context context.Context, id string) ([]int, error) {
	data, err := r.cmdOutput(r.command(context, "ps", "--format", "json", id), true, nil)
	defer putBuf(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %s", err, data.String())
	}
	var pids []int
	if err := json.Unmarshal(data.Bytes(), &pids); err != nil {
		return nil, err
	}
	return pids, nil
}

// Top lists all the processes inside the container returning the full ps data
func (r *Runc) Top(context context.Context, id string, psOptions string) (*TopResults, error) {
	data, err := r.cmdOutput(r.command(context, "ps", "--format", "table", id, psOptions), true, nil)
	defer putBuf(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %s", err, data.String())
	}

	topResults, err := ParsePSOutput(data.Bytes())
	if err != nil {
		return nil, fmt.Errorf("%s: ", err)
	}
	return topResults, nil
}

// CheckpointOpts holds the options for performing a criu checkpoint using runc
type CheckpointOpts struct {
	// ImagePath is the path for saving the criu image file
	ImagePath string
	// WorkDir is the working directory for criu
	WorkDir string
	// ParentPath is the path for previous image files from a pre-dump
	ParentPath string
	// AllowOpenTCP allows open tcp connections to be checkpointed
	AllowOpenTCP bool
	// AllowExternalUnixSockets allows external unix sockets to be checkpointed
	AllowExternalUnixSockets bool
	// AllowTerminal allows the terminal(pty) to be checkpointed with a container
	AllowTerminal bool
	// CriuPageServer is the address:port for the criu page server
	CriuPageServer string
	// FileLocks handle file locks held by the container
	FileLocks bool
	// Cgroups is the cgroup mode for how to handle the checkpoint of a container's cgroups
	Cgroups CgroupMode
	// EmptyNamespaces creates a namespace for the container but does not save its properties
	// Provide the namespaces you wish to be checkpointed without their settings on restore
	EmptyNamespaces []string
	// LazyPages uses userfaultfd to lazily restore memory pages
	LazyPages bool
	// StatusFile is the file criu writes \0 to once lazy-pages is ready
	StatusFile *os.File
	ExtraArgs  []string
}

// CgroupMode defines the cgroup mode used for checkpointing
type CgroupMode string

const (
	// Soft is the "soft" cgroup mode
	Soft CgroupMode = "soft"
	// Full is the "full" cgroup mode
	Full CgroupMode = "full"
	// Strict is the "strict" cgroup mode
	Strict CgroupMode = "strict"
)

func (o *CheckpointOpts) args() (out []string) {
	if o.ImagePath != "" {
		out = append(out, "--image-path", o.ImagePath)
	}
	if o.WorkDir != "" {
		out = append(out, "--work-path", o.WorkDir)
	}
	if o.ParentPath != "" {
		out = append(out, "--parent-path", o.ParentPath)
	}
	if o.AllowOpenTCP {
		out = append(out, "--tcp-established")
	}
	if o.AllowExternalUnixSockets {
		out = append(out, "--ext-unix-sk")
	}
	if o.AllowTerminal {
		out = append(out, "--shell-job")
	}
	if o.CriuPageServer != "" {
		out = append(out, "--page-server", o.CriuPageServer)
	}
	if o.FileLocks {
		out = append(out, "--file-locks")
	}
	if string(o.Cgroups) != "" {
		out = append(out, "--manage-cgroups-mode", string(o.Cgroups))
	}
	for _, ns := range o.EmptyNamespaces {
		out = append(out, "--empty-ns", ns)
	}
	if o.LazyPages {
		out = append(out, "--lazy-pages")
	}
	if len(o.ExtraArgs) > 0 {
		out = append(out, o.ExtraArgs...)
	}
	return out
}

// CheckpointAction represents specific actions executed during checkpoint/restore
type CheckpointAction func([]string) []string

// LeaveRunning keeps the container running after the checkpoint has been completed
func LeaveRunning(args []string) []string {
	return append(args, "--leave-running")
}

// PreDump allows a pre-dump of the checkpoint to be made and completed later
func PreDump(args []string) []string {
	return append(args, "--pre-dump")
}

// Checkpoint allows you to checkpoint a container using criu
func (r *Runc) Checkpoint(context context.Context, id string, opts *CheckpointOpts, actions ...CheckpointAction) error {
	args := []string{"checkpoint"}
	extraFiles := []*os.File{}
	if opts != nil {
		args = append(args, opts.args()...)
		if opts.StatusFile != nil {
			// pass the status file to the child process
			extraFiles = []*os.File{opts.StatusFile}
			// set status-fd to 3 as this will be the file descriptor
			// of the first file passed with cmd.ExtraFiles
			args = append(args, "--status-fd", "3")
		}
	}
	for _, a := range actions {
		args = a(args)
	}
	cmd := r.command(context, append(args, id)...)
	cmd.ExtraFiles = extraFiles
	return r.runOrError(cmd)
}

// RestoreOpts holds the options for performing a criu restore using runc
type RestoreOpts struct {
	CheckpointOpts
	IO

	Detach        bool
	PidFile       string
	NoSubreaper   bool
	NoPivot       bool
	ConsoleSocket ConsoleSocket
	ExtraArgs     []string
}

func (o *RestoreOpts) args() ([]string, error) {
	out := o.CheckpointOpts.args()
	if o.Detach {
		out = append(out, "--detach")
	}
	if o.PidFile != "" {
		abs, err := filepath.Abs(o.PidFile)
		if err != nil {
			return nil, err
		}
		out = append(out, "--pid-file", abs)
	}
	if o.ConsoleSocket != nil {
		out = append(out, "--console-socket", o.ConsoleSocket.Path())
	}
	if o.NoPivot {
		out = append(out, "--no-pivot")
	}
	if o.NoSubreaper {
		out = append(out, "-no-subreaper")
	}
	if len(o.ExtraArgs) > 0 {
		out = append(out, o.ExtraArgs...)
	}
	return out, nil
}

// Restore restores a container with the provide id from an existing checkpoint
func (r *Runc) Restore(context context.Context, id, bundle string, opts *RestoreOpts) (int, error) {
	args := []string{"restore"}
	if opts != nil {
		oargs, err := opts.args()
		if err != nil {
			return -1, err
		}
		args = append(args, oargs...)
	}
	args = append(args, "--bundle", bundle)
	cmd := r.command(context, append(args, id)...)
	if opts != nil && opts.IO != nil {
		opts.Set(cmd)
	}
	ec, err := r.startCommand(cmd)
	if err != nil {
		return -1, err
	}
	if opts != nil && opts.IO != nil {
		if c, ok := opts.IO.(StartCloser); ok {
			if err := c.CloseAfterStart(); err != nil {
				return -1, err
			}
		}
	}
	status, err := Monitor.Wait(cmd, ec)
	if err == nil && status != 0 {
		err = fmt.Errorf("%s did not terminate successfully: %w", cmd.Args[0], &ExitError{status})
	}
	return status, err
}

// Update updates the current container with the provided resource spec
func (r *Runc) Update(context context.Context, id string, resources *specs.LinuxResources) error {
	buf := getBuf()
	defer putBuf(buf)

	if err := json.NewEncoder(buf).Encode(resources); err != nil {
		return err
	}
	args := []string{"update", "--resources=-", id}
	cmd := r.command(context, args...)
	cmd.Stdin = buf
	return r.runOrError(cmd)
}

// ErrParseRuncVersion is used when the runc version can't be parsed
var ErrParseRuncVersion = errors.New("unable to parse runc version")

// Version represents the runc version information
type Version struct {
	Runc   string
	Commit string
	Spec   string
}

// Version returns the runc and runtime-spec versions
func (r *Runc) Version(context context.Context) (Version, error) {
	data, err := r.cmdOutput(r.command(context, "--version"), false, nil)
	defer putBuf(data)
	if err != nil {
		return Version{}, err
	}
	return parseVersion(data.Bytes())
}

func parseVersion(data []byte) (Version, error) {
	var v Version
	parts := strings.Split(strings.TrimSpace(string(data)), "\n")

	if len(parts) > 0 {
		if !strings.HasPrefix(parts[0], "runc version ") {
			return v, nil
		}
		v.Runc = parts[0][13:]

		for _, part := range parts[1:] {
			if strings.HasPrefix(part, "commit: ") {
				v.Commit = part[8:]
			} else if strings.HasPrefix(part, "spec: ") {
				v.Spec = part[6:]
			}
		}
	}

	return v, nil
}

// Features shows the features implemented by the runtime.
//
// Availability:
//
//   - runc:  supported since runc v1.1.0
//   - crun:  https://github.com/containers/crun/issues/1177
//   - youki: https://github.com/containers/youki/issues/815
func (r *Runc) Features(context context.Context) (*features.Features, error) {
	data, err := r.cmdOutput(r.command(context, "features"), false, nil)
	defer putBuf(data)
	if err != nil {
		return nil, err
	}
	var feat features.Features
	if err := json.Unmarshal(data.Bytes(), &feat); err != nil {
		return nil, err
	}
	return &feat, nil
}

func (r *Runc) args() (out []string) {
	if r.Root != "" {
		out = append(out, "--root", r.Root)
	}
	if r.Debug {
		out = append(out, "--debug")
	}
	if r.Log != "" {
		out = append(out, "--log", r.Log)
	}
	if r.LogFormat != none {
		out = append(out, "--log-format", string(r.LogFormat))
	}
	if r.SystemdCgroup {
		out = append(out, "--systemd-cgroup")
	}
	if r.Rootless != nil {
		// nil stands for "auto" (differs from explicit "false")
		out = append(out, "--rootless="+strconv.FormatBool(*r.Rootless))
	}
	if len(r.ExtraArgs) > 0 {
		out = append(out, r.ExtraArgs...)
	}
	return out
}

// runOrError will run the provided command.  If an error is
// encountered and neither Stdout or Stderr was set the error and the
// stderr of the command will be returned in the format of <error>:
// <stderr>
func (r *Runc) runOrError(cmd *exec.Cmd) error {
	if cmd.Stdout != nil || cmd.Stderr != nil {
		ec, err := r.startCommand(cmd)
		if err != nil {
			return err
		}
		status, err := Monitor.Wait(cmd, ec)
		if err == nil && status != 0 {
			err = fmt.Errorf("%s did not terminate successfully: %w", cmd.Args[0], &ExitError{status})
		}
		return err
	}
	data, err := r.cmdOutput(cmd, true, nil)
	defer putBuf(data)
	if err != nil {
		return fmt.Errorf("%s: %s", err, data.String())
	}
	return nil
}

// callers of cmdOutput are expected to call putBuf on the returned Buffer
// to ensure it is released back to the shared pool after use.
func (r *Runc) cmdOutput(cmd *exec.Cmd, combined bool, started chan<- int) (*bytes.Buffer, error) {
	b := getBuf()

	cmd.Stdout = b
	if combined {
		cmd.Stderr = b
	}
	ec, err := r.startCommand(cmd)
	if err != nil {
		return nil, err
	}
	if started != nil {
		started <- cmd.Process.Pid
	}

	status, err := Monitor.Wait(cmd, ec)
	if err == nil && status != 0 {
		err = fmt.Errorf("%s did not terminate successfully: %w", cmd.Args[0], &ExitError{status})
	}

	return b, err
}

// ExitError holds the status return code when a process exits with an error code
type ExitError struct {
	Status int
}

func (e *ExitError) Error() string {
	return fmt.Sprintf("exit status %d", e.Status)
}
