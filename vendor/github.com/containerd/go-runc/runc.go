package runc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	specs "github.com/opencontainers/runtime-spec/specs-go"
)

// Format is the type of log formatting options avaliable
type Format string

const (
	none Format = ""
	JSON Format = "json"
	Text Format = "text"
	// DefaultCommand is the default command for Runc
	DefaultCommand = "runc"
)

// Runc is the client to the runc cli
type Runc struct {
	//If command is empty, DefaultCommand is used
	Command       string
	Root          string
	Debug         bool
	Log           string
	LogFormat     Format
	PdeathSignal  syscall.Signal
	Setpgid       bool
	Criu          string
	SystemdCgroup bool
}

// List returns all containers created inside the provided runc root directory
func (r *Runc) List(context context.Context) ([]*Container, error) {
	data, err := cmdOutput(r.command(context, "list", "--format=json"), false)
	if err != nil {
		return nil, err
	}
	var out []*Container
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// State returns the state for the container provided by id
func (r *Runc) State(context context.Context, id string) (*Container, error) {
	data, err := cmdOutput(r.command(context, "state", id), true)
	if err != nil {
		return nil, fmt.Errorf("%s: %s", err, data)
	}
	var c Container
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

type ConsoleSocket interface {
	Path() string
}

type CreateOpts struct {
	IO
	// PidFile is a path to where a pid file should be created
	PidFile       string
	ConsoleSocket ConsoleSocket
	Detach        bool
	NoPivot       bool
	NoNewKeyring  bool
	ExtraFiles    []*os.File
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
	return out, nil
}

// Create creates a new container and returns its pid if it was created successfully
func (r *Runc) Create(context context.Context, id, bundle string, opts *CreateOpts) error {
	args := []string{"create", "--bundle", bundle}
	if opts != nil {
		oargs, err := opts.args()
		if err != nil {
			return err
		}
		args = append(args, oargs...)
	}
	cmd := r.command(context, append(args, id)...)
	if opts != nil && opts.IO != nil {
		opts.Set(cmd)
	}
	cmd.ExtraFiles = opts.ExtraFiles

	if cmd.Stdout == nil && cmd.Stderr == nil {
		data, err := cmdOutput(cmd, true)
		if err != nil {
			return fmt.Errorf("%s: %s", err, data)
		}
		return nil
	}
	ec, err := Monitor.Start(cmd)
	if err != nil {
		return err
	}
	if opts != nil && opts.IO != nil {
		if c, ok := opts.IO.(StartCloser); ok {
			if err := c.CloseAfterStart(); err != nil {
				return err
			}
		}
	}
	status, err := Monitor.Wait(cmd, ec)
	if err == nil && status != 0 {
		err = fmt.Errorf("%s did not terminate sucessfully", cmd.Args[0])
	}
	return err
}

// Start will start an already created container
func (r *Runc) Start(context context.Context, id string) error {
	return r.runOrError(r.command(context, "start", id))
}

type ExecOpts struct {
	IO
	PidFile       string
	ConsoleSocket ConsoleSocket
	Detach        bool
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
	return out, nil
}

// Exec executres and additional process inside the container based on a full
// OCI Process specification
func (r *Runc) Exec(context context.Context, id string, spec specs.Process, opts *ExecOpts) error {
	f, err := ioutil.TempFile("", "runc-process")
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
	if opts != nil {
		oargs, err := opts.args()
		if err != nil {
			return err
		}
		args = append(args, oargs...)
	}
	cmd := r.command(context, append(args, id)...)
	if opts != nil && opts.IO != nil {
		opts.Set(cmd)
	}
	if cmd.Stdout == nil && cmd.Stderr == nil {
		data, err := cmdOutput(cmd, true)
		if err != nil {
			return fmt.Errorf("%s: %s", err, data)
		}
		return nil
	}
	ec, err := Monitor.Start(cmd)
	if err != nil {
		return err
	}
	if opts != nil && opts.IO != nil {
		if c, ok := opts.IO.(StartCloser); ok {
			if err := c.CloseAfterStart(); err != nil {
				return err
			}
		}
	}
	status, err := Monitor.Wait(cmd, ec)
	if err == nil && status != 0 {
		err = fmt.Errorf("%s did not terminate sucessfully", cmd.Args[0])
	}
	return err
}

// Run runs the create, start, delete lifecycle of the container
// and returns its exit status after it has exited
func (r *Runc) Run(context context.Context, id, bundle string, opts *CreateOpts) (int, error) {
	args := []string{"run", "--bundle", bundle}
	if opts != nil {
		oargs, err := opts.args()
		if err != nil {
			return -1, err
		}
		args = append(args, oargs...)
	}
	cmd := r.command(context, append(args, id)...)
	if opts != nil && opts.IO != nil {
		opts.Set(cmd)
	}
	ec, err := Monitor.Start(cmd)
	if err != nil {
		return -1, err
	}
	return Monitor.Wait(cmd, ec)
}

type DeleteOpts struct {
	Force bool
}

func (o *DeleteOpts) args() (out []string) {
	if o.Force {
		out = append(out, "--force")
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
	All bool
}

func (o *KillOpts) args() (out []string) {
	if o.All {
		out = append(out, "--all")
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
	ec, err := Monitor.Start(cmd)
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
	ec, err := Monitor.Start(cmd)
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
	data, err := cmdOutput(r.command(context, "ps", "--format", "json", id), true)
	if err != nil {
		return nil, fmt.Errorf("%s: %s", err, data)
	}
	var pids []int
	if err := json.Unmarshal(data, &pids); err != nil {
		return nil, err
	}
	return pids, nil
}

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
}

type CgroupMode string

const (
	Soft   CgroupMode = "soft"
	Full   CgroupMode = "full"
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
	return out
}

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
	if opts != nil {
		args = append(args, opts.args()...)
	}
	for _, a := range actions {
		args = a(args)
	}
	return r.runOrError(r.command(context, append(args, id)...))
}

type RestoreOpts struct {
	CheckpointOpts
	IO

	Detach      bool
	PidFile     string
	NoSubreaper bool
	NoPivot     bool
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
	if o.NoPivot {
		out = append(out, "--no-pivot")
	}
	if o.NoSubreaper {
		out = append(out, "-no-subreaper")
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
	ec, err := Monitor.Start(cmd)
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
	return Monitor.Wait(cmd, ec)
}

// Update updates the current container with the provided resource spec
func (r *Runc) Update(context context.Context, id string, resources *specs.LinuxResources) error {
	buf := getBuf()
	defer putBuf(buf)

	if err := json.NewEncoder(buf).Encode(resources); err != nil {
		return err
	}
	args := []string{"update", "--resources", "-", id}
	cmd := r.command(context, args...)
	cmd.Stdin = buf
	return r.runOrError(cmd)
}

var ErrParseRuncVersion = errors.New("unable to parse runc version")

type Version struct {
	Runc   string
	Commit string
	Spec   string
}

// Version returns the runc and runtime-spec versions
func (r *Runc) Version(context context.Context) (Version, error) {
	data, err := cmdOutput(r.command(context, "--version"), false)
	if err != nil {
		return Version{}, err
	}
	return parseVersion(data)
}

func parseVersion(data []byte) (Version, error) {
	var v Version
	parts := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(parts) != 3 {
		return v, ErrParseRuncVersion
	}

	for i, p := range []struct {
		dest  *string
		split string
	}{
		{
			dest:  &v.Runc,
			split: "version ",
		},
		{
			dest:  &v.Commit,
			split: ": ",
		},
		{
			dest:  &v.Spec,
			split: ": ",
		},
	} {
		p2 := strings.Split(parts[i], p.split)
		if len(p2) != 2 {
			return v, fmt.Errorf("unable to parse version line %q", parts[i])
		}
		*p.dest = p2[1]
	}
	return v, nil
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
	if r.Criu != "" {
		out = append(out, "--criu", r.Criu)
	}
	if r.SystemdCgroup {
		out = append(out, "--systemd-cgroup")
	}
	return out
}

// runOrError will run the provided command.  If an error is
// encountered and neither Stdout or Stderr was set the error and the
// stderr of the command will be returned in the format of <error>:
// <stderr>
func (r *Runc) runOrError(cmd *exec.Cmd) error {
	if cmd.Stdout != nil || cmd.Stderr != nil {
		ec, err := Monitor.Start(cmd)
		if err != nil {
			return err
		}
		status, err := Monitor.Wait(cmd, ec)
		if err == nil && status != 0 {
			err = fmt.Errorf("%s did not terminate sucessfully", cmd.Args[0])
		}
		return err
	}
	data, err := cmdOutput(cmd, true)
	if err != nil {
		return fmt.Errorf("%s: %s", err, data)
	}
	return nil
}

func cmdOutput(cmd *exec.Cmd, combined bool) ([]byte, error) {
	b := getBuf()
	defer putBuf(b)

	cmd.Stdout = b
	if combined {
		cmd.Stderr = b
	}
	ec, err := Monitor.Start(cmd)
	if err != nil {
		return nil, err
	}

	status, err := Monitor.Wait(cmd, ec)
	if err == nil && status != 0 {
		err = fmt.Errorf("%s did not terminate sucessfully", cmd.Args[0])
	}

	return b.Bytes(), err
}
