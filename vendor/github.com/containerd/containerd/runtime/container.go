package runtime

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/containerd/containerd/specs"
	ocs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"golang.org/x/sys/unix"
)

// Container defines the operations allowed on a container
type Container interface {
	// ID returns the container ID
	ID() string
	// Path returns the path to the bundle
	Path() string
	// Start starts the init process of the container
	Start(ctx context.Context, checkpointPath string, s Stdio) (Process, error)
	// Exec starts another process in an existing container
	Exec(context.Context, string, specs.ProcessSpec, Stdio) (Process, error)
	// Delete removes the container's state and any resources
	Delete() error
	// Processes returns all the containers processes that have been added
	Processes() ([]Process, error)
	// State returns the containers runtime state
	State() State
	// Resume resumes a paused container
	Resume() error
	// Pause pauses a running container
	Pause() error
	// RemoveProcess removes the specified process from the container
	RemoveProcess(string) error
	// Checkpoints returns all the checkpoints for a container
	Checkpoints(checkpointDir string) ([]Checkpoint, error)
	// Checkpoint creates a new checkpoint
	Checkpoint(checkpoint Checkpoint, checkpointDir string) error
	// DeleteCheckpoint deletes the checkpoint for the provided name
	DeleteCheckpoint(name string, checkpointDir string) error
	// Labels are user provided labels for the container
	Labels() []string
	// Pids returns all pids inside the container
	Pids() ([]int, error)
	// Stats returns realtime container stats and resource information
	Stats() (*Stat, error)
	// Name or path of the OCI compliant runtime used to execute the container
	Runtime() string
	// OOM signals the channel if the container received an OOM notification
	OOM() (OOM, error)
	// UpdateResource updates the containers resources to new values
	UpdateResources(*Resource) error

	// Status return the current status of the container.
	Status() (State, error)
}

// OOM wraps a container OOM.
type OOM interface {
	io.Closer
	FD() int
	ContainerID() string
	Flush()
	Removed() bool
}

// Stdio holds the path to the 3 pipes used for the standard ios.
type Stdio struct {
	Stdin  string
	Stdout string
	Stderr string
}

// NewStdio wraps the given standard io path into an Stdio struct.
// If a given parameter is the empty string, it is replaced by "/dev/null"
func NewStdio(stdin, stdout, stderr string) Stdio {
	for _, s := range []*string{
		&stdin, &stdout, &stderr,
	} {
		if *s == "" {
			*s = "/dev/null"
		}
	}
	return Stdio{
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
	}
}

// ContainerOpts keeps the options passed at container creation
type ContainerOpts struct {
	Root        string
	ID          string
	Bundle      string
	Runtime     string
	RuntimeArgs []string
	Shim        string
	Labels      []string
	NoPivotRoot bool
	Timeout     time.Duration
}

// New returns a new container
func New(opts ContainerOpts) (Container, error) {
	c := &container{
		root:        opts.Root,
		id:          opts.ID,
		bundle:      opts.Bundle,
		labels:      opts.Labels,
		processes:   make(map[string]*process),
		runtime:     opts.Runtime,
		runtimeArgs: opts.RuntimeArgs,
		shim:        opts.Shim,
		noPivotRoot: opts.NoPivotRoot,
		timeout:     opts.Timeout,
	}
	if err := os.Mkdir(filepath.Join(c.root, c.id), 0755); err != nil {
		return nil, err
	}
	f, err := os.Create(filepath.Join(c.root, c.id, StateFile))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if err := json.NewEncoder(f).Encode(state{
		Bundle:      c.bundle,
		Labels:      c.labels,
		Runtime:     c.runtime,
		RuntimeArgs: c.runtimeArgs,
		Shim:        c.shim,
		NoPivotRoot: opts.NoPivotRoot,
	}); err != nil {
		return nil, err
	}
	return c, nil
}

// Load return a new container from the matchin state file on disk.
func Load(root, id, shimName string, timeout time.Duration) (Container, error) {
	var s state
	f, err := os.Open(filepath.Join(root, id, StateFile))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if err := json.NewDecoder(f).Decode(&s); err != nil {
		return nil, err
	}
	c := &container{
		root:        root,
		id:          id,
		bundle:      s.Bundle,
		labels:      s.Labels,
		runtime:     s.Runtime,
		runtimeArgs: s.RuntimeArgs,
		shim:        s.Shim,
		noPivotRoot: s.NoPivotRoot,
		processes:   make(map[string]*process),
		timeout:     timeout,
	}

	if c.shim == "" {
		c.shim = shimName
	}

	dirs, err := ioutil.ReadDir(filepath.Join(root, id))
	if err != nil {
		return nil, err
	}
	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}
		pid := d.Name()
		s, err := readProcessState(filepath.Join(root, id, pid))
		if err != nil {
			return nil, err
		}
		p, err := loadProcess(filepath.Join(root, id, pid), pid, c, s)
		if err != nil {
			logrus.WithField("id", id).WithField("pid", pid).Debugf("containerd: error loading process %s", err)
			continue
		}
		c.processes[pid] = p
	}

	_, err = os.Stat(c.bundle)
	if err != nil && !os.IsExist(err) {
		for key, p := range c.processes {
			if key == InitProcessID {
				p.Delete()
				break
			}
		}
		return nil, fmt.Errorf("bundle dir %s don't exist", c.bundle)
	}
	return c, nil
}

func readProcessState(dir string) (*ProcessState, error) {
	f, err := os.Open(filepath.Join(dir, "process.json"))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var s ProcessState
	if err := json.NewDecoder(f).Decode(&s); err != nil {
		return nil, err
	}
	return &s, nil
}

type container struct {
	// path to store runtime state information
	root        string
	id          string
	bundle      string
	runtime     string
	runtimeArgs []string
	shim        string
	processes   map[string]*process
	labels      []string
	oomFds      []int
	noPivotRoot bool
	timeout     time.Duration
}

func (c *container) ID() string {
	return c.id
}

func (c *container) Path() string {
	return c.bundle
}

func (c *container) Labels() []string {
	return c.labels
}

func (c *container) readSpec() (*specs.Spec, error) {
	var spec specs.Spec
	f, err := os.Open(filepath.Join(c.bundle, "config.json"))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if err := json.NewDecoder(f).Decode(&spec); err != nil {
		return nil, err
	}
	return &spec, nil
}

func (c *container) Delete() error {
	var err error
	args := append(c.runtimeArgs, "delete", c.id)
	if b, derr := exec.Command(c.runtime, args...).CombinedOutput(); derr != nil && !strings.Contains(string(b), "does not exist") {
		err = fmt.Errorf("%s: %q", derr, string(b))
	}
	if rerr := os.RemoveAll(filepath.Join(c.root, c.id)); rerr != nil {
		if err != nil {
			err = fmt.Errorf("%s; failed to remove %s: %s", err, filepath.Join(c.root, c.id), rerr)
		} else {
			err = rerr
		}
	}
	return err
}

func (c *container) Processes() ([]Process, error) {
	out := []Process{}
	for _, p := range c.processes {
		out = append(out, p)
	}
	return out, nil
}

func (c *container) RemoveProcess(pid string) error {
	delete(c.processes, pid)
	return os.RemoveAll(filepath.Join(c.root, c.id, pid))
}

func (c *container) State() State {
	proc := c.processes[InitProcessID]
	if proc == nil {
		return Stopped
	}
	return proc.State()
}

func (c *container) Runtime() string {
	return c.runtime
}

func (c *container) Pause() error {
	args := c.runtimeArgs
	args = append(args, "pause", c.id)
	b, err := exec.Command(c.runtime, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %q", err.Error(), string(b))
	}
	return nil
}

func (c *container) Resume() error {
	args := c.runtimeArgs
	args = append(args, "resume", c.id)
	b, err := exec.Command(c.runtime, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %q", err.Error(), string(b))
	}
	return nil
}

func (c *container) Checkpoints(checkpointDir string) ([]Checkpoint, error) {
	if checkpointDir == "" {
		checkpointDir = filepath.Join(c.bundle, "checkpoints")
	}

	dirs, err := ioutil.ReadDir(checkpointDir)
	if err != nil {
		return nil, err
	}
	var out []Checkpoint
	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}
		path := filepath.Join(checkpointDir, d.Name(), "config.json")
		data, err := ioutil.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var cpt Checkpoint
		if err := json.Unmarshal(data, &cpt); err != nil {
			return nil, err
		}
		out = append(out, cpt)
	}
	return out, nil
}

func (c *container) Checkpoint(cpt Checkpoint, checkpointDir string) error {
	if checkpointDir == "" {
		checkpointDir = filepath.Join(c.bundle, "checkpoints")
	}

	if err := os.MkdirAll(checkpointDir, 0755); err != nil {
		return err
	}

	path := filepath.Join(checkpointDir, cpt.Name)
	if err := os.Mkdir(path, 0755); err != nil {
		return err
	}
	f, err := os.Create(filepath.Join(path, "config.json"))
	if err != nil {
		return err
	}
	cpt.Created = time.Now()
	err = json.NewEncoder(f).Encode(cpt)
	f.Close()
	if err != nil {
		return err
	}
	args := []string{
		"checkpoint",
		"--image-path", path,
		"--work-path", filepath.Join(path, "criu.work"),
	}
	add := func(flags ...string) {
		args = append(args, flags...)
	}
	add(c.runtimeArgs...)
	if !cpt.Exit {
		add("--leave-running")
	}
	if cpt.Shell {
		add("--shell-job")
	}
	if cpt.TCP {
		add("--tcp-established")
	}
	if cpt.UnixSockets {
		add("--ext-unix-sk")
	}
	for _, ns := range cpt.EmptyNS {
		add("--empty-ns", ns)
	}
	add(c.id)
	out, err := exec.Command(c.runtime, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %q", err.Error(), string(out))
	}
	return err
}

func (c *container) DeleteCheckpoint(name string, checkpointDir string) error {
	if checkpointDir == "" {
		checkpointDir = filepath.Join(c.bundle, "checkpoints")
	}
	return os.RemoveAll(filepath.Join(checkpointDir, name))
}

func (c *container) Start(ctx context.Context, checkpointPath string, s Stdio) (Process, error) {
	processRoot := filepath.Join(c.root, c.id, InitProcessID)
	if err := os.Mkdir(processRoot, 0755); err != nil {
		return nil, err
	}
	cmd := exec.Command(c.shim,
		c.id, c.bundle, c.runtime,
	)
	cmd.Dir = processRoot
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
	spec, err := c.readSpec()
	if err != nil {
		return nil, err
	}
	config := &processConfig{
		checkpoint:  checkpointPath,
		root:        processRoot,
		id:          InitProcessID,
		c:           c,
		stdio:       s,
		spec:        spec,
		processSpec: specs.ProcessSpec(spec.Process),
	}
	p, err := newProcess(config)
	if err != nil {
		return nil, err
	}
	if err := c.createCmd(ctx, InitProcessID, cmd, p); err != nil {
		return nil, err
	}
	return p, nil
}

func (c *container) Exec(ctx context.Context, pid string, pspec specs.ProcessSpec, s Stdio) (pp Process, err error) {
	processRoot := filepath.Join(c.root, c.id, pid)
	if err := os.Mkdir(processRoot, 0755); err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			c.RemoveProcess(pid)
		}
	}()
	cmd := exec.Command(c.shim,
		c.id, c.bundle, c.runtime,
	)
	cmd.Dir = processRoot
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
	spec, err := c.readSpec()
	if err != nil {
		return nil, err
	}
	config := &processConfig{
		exec:        true,
		id:          pid,
		root:        processRoot,
		c:           c,
		processSpec: pspec,
		spec:        spec,
		stdio:       s,
	}
	p, err := newProcess(config)
	if err != nil {
		return nil, err
	}
	if err := c.createCmd(ctx, pid, cmd, p); err != nil {
		return nil, err
	}
	return p, nil
}

func (c *container) createCmd(ctx context.Context, pid string, cmd *exec.Cmd, p *process) error {
	p.cmd = cmd
	if err := cmd.Start(); err != nil {
		close(p.cmdDoneCh)
		if exErr, ok := err.(*exec.Error); ok {
			if exErr.Err == exec.ErrNotFound || exErr.Err == os.ErrNotExist {
				return fmt.Errorf("%s not installed on system", c.shim)
			}
		}
		return err
	}
	// We need the pid file to have been written to run
	defer func() {
		go func() {
			err := p.cmd.Wait()
			if err == nil {
				p.cmdSuccess = true
			}

			if same, err := p.isSameProcess(); same && p.pid > 0 {
				// The process changed its PR_SET_PDEATHSIG, so force
				// kill it
				logrus.Infof("containerd: %s:%s (pid %v) has become an orphan, killing it", p.container.id, p.id, p.pid)
				err = unix.Kill(p.pid, syscall.SIGKILL)
				if err != nil && err != syscall.ESRCH {
					logrus.Errorf("containerd: unable to SIGKILL %s:%s (pid %v): %v", p.container.id, p.id, p.pid, err)
				} else {
					for {
						err = unix.Kill(p.pid, 0)
						if err != nil {
							break
						}
						time.Sleep(5 * time.Millisecond)
					}
				}
			}
			close(p.cmdDoneCh)
		}()
	}()

	ch := make(chan error)
	go func() {
		if err := c.waitForCreate(p, cmd); err != nil {
			ch <- err
			return
		}
		c.processes[pid] = p
		ch <- nil
	}()
	select {
	case <-ctx.Done():
		cmd.Process.Kill()
		cmd.Wait()
		<-ch
		return ctx.Err()
	case err := <-ch:
		return err
	}
	return nil
}

func hostIDFromMap(id uint32, mp []ocs.LinuxIDMapping) int {
	for _, m := range mp {
		if (id >= m.ContainerID) && (id <= (m.ContainerID + m.Size - 1)) {
			return int(m.HostID + (id - m.ContainerID))
		}
	}
	return 0
}

func (c *container) Stats() (*Stat, error) {
	now := time.Now()
	args := c.runtimeArgs
	args = append(args, "events", "--stats", c.id)
	out, err := exec.Command(c.runtime, args...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%s: %q", err.Error(), out)
	}
	s := struct {
		Data *Stat `json:"data"`
	}{}
	if err := json.Unmarshal(out, &s); err != nil {
		return nil, err
	}
	s.Data.Timestamp = now
	return s.Data, nil
}

// Status implements the runtime Container interface.
func (c *container) Status() (State, error) {
	args := c.runtimeArgs
	args = append(args, "state", c.id)

	out, err := exec.Command(c.runtime, args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s: %q", err.Error(), out)
	}

	// We only require the runtime json output to have a top level Status field.
	var s struct {
		Status State `json:"status"`
	}
	if err := json.Unmarshal(out, &s); err != nil {
		return "", err
	}
	return s.Status, nil
}

func (c *container) writeEventFD(root string, cfd, efd int) error {
	f, err := os.OpenFile(filepath.Join(root, "cgroup.event_control"), os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(fmt.Sprintf("%d %d", efd, cfd))
	return err
}

type waitArgs struct {
	pid int
	err error
}

func (c *container) waitForCreate(p *process, cmd *exec.Cmd) error {
	wc := make(chan error, 1)
	go func() {
		for {
			if _, err := p.getPidFromFile(); err != nil {
				if os.IsNotExist(err) || err == errInvalidPidInt || err == errContainerNotFound {
					alive, err := isAlive(cmd)
					if err != nil {
						wc <- err
						return
					}
					if !alive {
						// runc could have failed to run the container so lets get the error
						// out of the logs or the shim could have encountered an error
						messages, err := readLogMessages(filepath.Join(p.root, "shim-log.json"))
						if err != nil {
							wc <- err
							return
						}
						for _, m := range messages {
							if m.Level == "error" {
								wc <- fmt.Errorf("shim error: %v", m.Msg)
								return
							}
						}
						// no errors reported back from shim, check for runc/runtime errors
						messages, err = readLogMessages(filepath.Join(p.root, "log.json"))
						if err != nil {
							if os.IsNotExist(err) {
								err = ErrContainerNotStarted
							}
							wc <- err
							return
						}
						for _, m := range messages {
							if m.Level == "error" {
								wc <- fmt.Errorf("oci runtime error: %v", m.Msg)
								return
							}
						}
						wc <- ErrContainerNotStarted
						return
					}
					time.Sleep(15 * time.Millisecond)
					continue
				}
				wc <- err
				return
			}
			// the pid file was read successfully
			wc <- nil
			return
		}
	}()
	select {
	case err := <-wc:
		if err != nil {
			return err
		}
		err = p.saveStartTime()
		if err != nil && !os.IsNotExist(err) {
			logrus.Warnf("containerd: unable to save %s:%s starttime: %v", p.container.id, p.id, err)
		}
		return nil
	case <-time.After(c.timeout):
		cmd.Process.Kill()
		cmd.Wait()
		return ErrContainerStartTimeout
	}
}

// isAlive checks if the shim that launched the container is still alive
func isAlive(cmd *exec.Cmd) (bool, error) {
	if _, err := syscall.Wait4(cmd.Process.Pid, nil, syscall.WNOHANG, nil); err == nil {
		return true, nil
	}
	if err := syscall.Kill(cmd.Process.Pid, 0); err != nil {
		if err == syscall.ESRCH {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

type oom struct {
	id      string
	root    string
	eventfd int
}

func (o *oom) ContainerID() string {
	return o.id
}

func (o *oom) FD() int {
	return o.eventfd
}

func (o *oom) Flush() {
	buf := make([]byte, 8)
	syscall.Read(o.eventfd, buf)
}

func (o *oom) Removed() bool {
	_, err := os.Lstat(filepath.Join(o.root, "cgroup.event_control"))
	return os.IsNotExist(err)
}

func (o *oom) Close() error {
	return syscall.Close(o.eventfd)
}

type message struct {
	Level string `json:"level"`
	Msg   string `json:"msg"`
}

func readLogMessages(path string) ([]message, error) {
	var out []message
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	for {
		var m message
		if err := dec.Decode(&m); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		out = append(out, m)
	}
	return out, nil
}
