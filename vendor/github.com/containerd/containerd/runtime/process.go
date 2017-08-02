package runtime

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/containerd/containerd/osutils"
	"github.com/containerd/containerd/specs"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

// Process holds the operation allowed on a container's process
type Process interface {
	io.Closer

	// ID of the process.
	// This is either "init" when it is the container's init process or
	// it is a user provided id for the process similar to the container id
	ID() string
	// Start unblocks the associated container init process.
	// This should only be called on the process with ID "init"
	Start() error
	CloseStdin() error
	Resize(int, int) error
	// ExitFD returns the fd the provides an event when the process exits
	ExitFD() int
	// ExitStatus returns the exit status of the process or an error if it
	// has not exited
	ExitStatus() (uint32, error)
	// Spec returns the process spec that created the process
	Spec() specs.ProcessSpec
	// Signal sends the provided signal to the process
	Signal(os.Signal) error
	// Container returns the container that the process belongs to
	Container() Container
	// Stdio of the container
	Stdio() Stdio
	// SystemPid is the pid on the system
	SystemPid() int
	// State returns if the process is running or not
	State() State
	// Wait reaps the shim process if avaliable
	Wait()
}

type processConfig struct {
	id          string
	root        string
	processSpec specs.ProcessSpec
	spec        *specs.Spec
	c           *container
	stdio       Stdio
	exec        bool
	checkpoint  string
}

func newProcess(config *processConfig) (*process, error) {
	p := &process{
		root:      config.root,
		id:        config.id,
		container: config.c,
		spec:      config.processSpec,
		stdio:     config.stdio,
		cmdDoneCh: make(chan struct{}),
		state:     Running,
	}
	uid, gid, err := getRootIDs(config.spec)
	if err != nil {
		return nil, err
	}
	f, err := os.Create(filepath.Join(config.root, "process.json"))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	ps := ProcessState{
		ProcessSpec: config.processSpec,
		Exec:        config.exec,
		PlatformProcessState: PlatformProcessState{
			Checkpoint: config.checkpoint,
			RootUID:    uid,
			RootGID:    gid,
		},
		Stdin:       config.stdio.Stdin,
		Stdout:      config.stdio.Stdout,
		Stderr:      config.stdio.Stderr,
		RuntimeArgs: config.c.runtimeArgs,
		NoPivotRoot: config.c.noPivotRoot,
	}

	if err := json.NewEncoder(f).Encode(ps); err != nil {
		return nil, err
	}
	exit, err := getExitPipe(filepath.Join(config.root, ExitFile))
	if err != nil {
		return nil, err
	}
	control, err := getControlPipe(filepath.Join(config.root, ControlFile))
	if err != nil {
		return nil, err
	}
	p.exitPipe = exit
	p.controlPipe = control
	return p, nil
}

func loadProcess(root, id string, c *container, s *ProcessState) (*process, error) {
	p := &process{
		root:      root,
		id:        id,
		container: c,
		spec:      s.ProcessSpec,
		stdio: Stdio{
			Stdin:  s.Stdin,
			Stdout: s.Stdout,
			Stderr: s.Stderr,
		},
		state: Stopped,
	}

	startTime, err := ioutil.ReadFile(filepath.Join(p.root, StartTimeFile))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	p.startTime = string(startTime)

	if _, err := p.getPidFromFile(); err != nil {
		return nil, err
	}
	if _, err := p.ExitStatus(); err != nil {
		if err == ErrProcessNotExited {
			exit, err := getExitPipe(filepath.Join(root, ExitFile))
			if err != nil {
				return nil, err
			}
			p.exitPipe = exit

			control, err := getControlPipe(filepath.Join(root, ControlFile))
			if err != nil {
				return nil, err
			}
			p.controlPipe = control

			p.state = Running
			return p, nil
		}
		return nil, err
	}
	return p, nil
}

func readProcStatField(pid int, field int) (string, error) {
	data, err := ioutil.ReadFile(filepath.Join(string(filepath.Separator), "proc", strconv.Itoa(pid), "stat"))
	if err != nil {
		return "", err
	}

	if field > 2 {
		// First, split out the name since he could contains spaces.
		parts := strings.Split(string(data), ") ")
		// Now split out the rest, we end up with 2 fields less
		parts = strings.Split(parts[1], " ")
		return parts[field-2-1], nil // field count start at 1 in manual
	}

	parts := strings.Split(string(data), " (")

	if field == 1 {
		return parts[0], nil
	}

	parts = strings.Split(parts[1], ") ")
	return parts[0], nil
}

type process struct {
	root        string
	id          string
	pid         int
	exitPipe    *os.File
	controlPipe *os.File
	container   *container
	spec        specs.ProcessSpec
	stdio       Stdio
	cmd         *exec.Cmd
	cmdSuccess  bool
	cmdDoneCh   chan struct{}
	state       State
	stateLock   sync.Mutex
	startTime   string
}

func (p *process) ID() string {
	return p.id
}

func (p *process) Container() Container {
	return p.container
}

func (p *process) SystemPid() int {
	return p.pid
}

// ExitFD returns the fd of the exit pipe
func (p *process) ExitFD() int {
	return int(p.exitPipe.Fd())
}

func (p *process) CloseStdin() error {
	_, err := fmt.Fprintf(p.controlPipe, "%d %d %d\n", 0, 0, 0)
	return err
}

func (p *process) Resize(w, h int) error {
	_, err := fmt.Fprintf(p.controlPipe, "%d %d %d\n", 1, w, h)
	return err
}

func (p *process) updateExitStatusFile(status uint32) (uint32, error) {
	p.stateLock.Lock()
	p.state = Stopped
	p.stateLock.Unlock()
	err := ioutil.WriteFile(filepath.Join(p.root, ExitStatusFile), []byte(fmt.Sprintf("%u", status)), 0644)
	return status, err
}

func (p *process) handleSigkilledShim(rst uint32, rerr error) (uint32, error) {
	if p.cmd == nil || p.cmd.Process == nil {
		e := unix.Kill(p.pid, 0)
		if e == syscall.ESRCH {
			logrus.Warnf("containerd: %s:%s (pid %d) does not exist", p.container.id, p.id, p.pid)
			// The process died while containerd was down (probably of
			// SIGKILL, but no way to be sure)
			return p.updateExitStatusFile(UnknownStatus)
		}

		// If it's not the same process, just mark it stopped and set
		// the status to the UnknownStatus value (i.e. 255)
		if same, err := p.isSameProcess(); !same {
			logrus.Warnf("containerd: %s:%s (pid %d) is not the same process anymore (%v)", p.container.id, p.id, p.pid, err)
			// Create the file so we get the exit event generated once monitor kicks in
			// without having to go through all this process again
			return p.updateExitStatusFile(UnknownStatus)
		}

		ppid, err := readProcStatField(p.pid, 4)
		if err != nil {
			return rst, fmt.Errorf("could not check process ppid: %v (%v)", err, rerr)
		}
		if ppid == "1" {
			logrus.Warnf("containerd: %s:%s shim died, killing associated process", p.container.id, p.id)
			unix.Kill(p.pid, syscall.SIGKILL)
			if err != nil && err != syscall.ESRCH {
				return UnknownStatus, fmt.Errorf("containerd: unable to SIGKILL %s:%s (pid %v): %v", p.container.id, p.id, p.pid, err)
			}

			// wait for the process to die
			for {
				e := unix.Kill(p.pid, 0)
				if e == syscall.ESRCH {
					break
				}
				time.Sleep(5 * time.Millisecond)
			}
			// Create the file so we get the exit event generated once monitor kicks in
			// without having to go through all this process again
			return p.updateExitStatusFile(128 + uint32(syscall.SIGKILL))
		}

		return rst, rerr
	}

	// Possible that the shim was SIGKILLED
	e := unix.Kill(p.cmd.Process.Pid, 0)
	if e != syscall.ESRCH {
		return rst, rerr
	}

	// Ensure we got the shim ProcessState
	<-p.cmdDoneCh

	shimStatus := p.cmd.ProcessState.Sys().(syscall.WaitStatus)
	if shimStatus.Signaled() && shimStatus.Signal() == syscall.SIGKILL {
		logrus.Debugf("containerd: ExitStatus(container: %s, process: %s): shim was SIGKILL'ed reaping its child with pid %d", p.container.id, p.id, p.pid)

		rerr = nil
		rst = 128 + uint32(shimStatus.Signal())

		p.stateLock.Lock()
		p.state = Stopped
		p.stateLock.Unlock()
	}

	return rst, rerr
}

func (p *process) ExitStatus() (rst uint32, rerr error) {
	data, err := ioutil.ReadFile(filepath.Join(p.root, ExitStatusFile))
	defer func() {
		if rerr != nil {
			rst, rerr = p.handleSigkilledShim(rst, rerr)
		}
	}()
	if err != nil {
		if os.IsNotExist(err) {
			return UnknownStatus, ErrProcessNotExited
		}
		return UnknownStatus, err
	}
	if len(data) == 0 {
		return UnknownStatus, ErrProcessNotExited
	}
	p.stateLock.Lock()
	p.state = Stopped
	p.stateLock.Unlock()

	i, err := strconv.ParseUint(string(data), 10, 32)
	return uint32(i), err
}

func (p *process) Spec() specs.ProcessSpec {
	return p.spec
}

func (p *process) Stdio() Stdio {
	return p.stdio
}

// Close closes any open files and/or resouces on the process
func (p *process) Close() error {
	err := p.exitPipe.Close()
	if cerr := p.controlPipe.Close(); err == nil {
		err = cerr
	}
	return err
}

func (p *process) State() State {
	p.stateLock.Lock()
	defer p.stateLock.Unlock()
	return p.state
}

func (p *process) readStartTime() (string, error) {
	return readProcStatField(p.pid, 22)
}

func (p *process) saveStartTime() error {
	startTime, err := p.readStartTime()
	if err != nil {
		return err
	}

	p.startTime = startTime
	return ioutil.WriteFile(filepath.Join(p.root, StartTimeFile), []byte(startTime), 0644)
}

func (p *process) isSameProcess() (bool, error) {
	if p.pid == 0 {
		_, err := p.getPidFromFile()
		if err != nil {
			return false, err
		}
	}

	// for backward compat assume it's the same if startTime wasn't set
	if p.startTime == "" {
		// Sometimes the process dies before we can get the starttime,
		// check that the process actually exists
		if err := unix.Kill(p.pid, 0); err != syscall.ESRCH {
			return true, nil
		}
		return false, nil
	}

	startTime, err := p.readStartTime()
	if err != nil {
		return false, err
	}

	return startTime == p.startTime, nil
}

// Wait will reap the shim process
func (p *process) Wait() {
	if p.cmdDoneCh != nil {
		<-p.cmdDoneCh
	}
}

func getExitPipe(path string) (*os.File, error) {
	if err := unix.Mkfifo(path, 0755); err != nil && !os.IsExist(err) {
		return nil, err
	}
	// add NONBLOCK in case the other side has already closed or else
	// this function would never return
	return os.OpenFile(path, syscall.O_RDONLY|syscall.O_NONBLOCK, 0)
}

func getControlPipe(path string) (*os.File, error) {
	if err := unix.Mkfifo(path, 0755); err != nil && !os.IsExist(err) {
		return nil, err
	}
	return os.OpenFile(path, syscall.O_RDWR|syscall.O_NONBLOCK, 0)
}

// Signal sends the provided signal to the process
func (p *process) Signal(s os.Signal) error {
	return syscall.Kill(p.pid, s.(syscall.Signal))
}

// Start unblocks the associated container init process.
// This should only be called on the process with ID "init"
func (p *process) Start() error {
	if p.ID() == InitProcessID {
		var (
			errC = make(chan error, 1)
			args = append(p.container.runtimeArgs, "start", p.container.id)
			cmd  = exec.Command(p.container.runtime, args...)
		)
		go func() {
			out, err := cmd.CombinedOutput()
			if err != nil {
				errC <- fmt.Errorf("%s: %q", err.Error(), out)
			}
			errC <- nil
		}()
		select {
		case err := <-errC:
			if err != nil {
				return err
			}
		case <-p.cmdDoneCh:
			if !p.cmdSuccess {
				if cmd.Process != nil {
					cmd.Process.Kill()
				}
				cmd.Wait()
				return ErrShimExited
			}
			err := <-errC
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// Delete delete any resources held by the container
func (p *process) Delete() error {
	var (
		args = append(p.container.runtimeArgs, "delete", "-f", p.container.id)
		cmd  = exec.Command(p.container.runtime, args...)
	)

	cmd.SysProcAttr = osutils.SetPDeathSig()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %v", out, err)
	}
	return nil
}
