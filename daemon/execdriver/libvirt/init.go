// +build linux

package libvirt

import (
	"flag"
	"fmt"
	"github.com/docker/docker/daemon/execdriver"
	"github.com/docker/docker/pkg/rpcfd"
	"github.com/docker/docker/pkg/signal"
	"github.com/docker/docker/reexec"
	"github.com/docker/libcontainer/namespaces"
	"github.com/kr/pty"
	"log"
	"net"
	"net/rpc"
	"os"
	"os/exec"
	"path"
	"sync"
	"syscall"
	"time"
)

const DriverName = "libvirt"
const SocketPath = "/.dockersocket"
const RpcSocketName = "rpc.sock"
const DockerInitName = ".dockerinit-libvirt"

type State int32

const (
	Initial State = iota
	ConsoleReady
	RunReady
	Running
	Exited
	FailedToStart
	Dead
)

// Args provided to the init function for a driver
type InitArgs struct {
	*execdriver.InitArgs
	Tty       bool
	OpenStdin bool
}

type StateInfo struct {
	State    State
	Error    string
	ExitCode int
}

type DockerInit struct {
	StateInfo
	sync.Mutex

	resume      chan int
	cancel      chan int
	process     *os.Process
	processLock chan struct{}

	stdin     *os.File
	stdout    *os.File
	stderr    *os.File
	ptyMaster *os.File
}

func init() {
	reexec.Register("/"+DockerInitName, initializer)
}

func rpcSocketPath() string {
	return path.Join(SocketPath, RpcSocketName)
}

// RPC: ABI version
func (init *DockerInit) GetVersion(_ int, version *int) error {
	*version = 1
	return nil
}

// RPC: Get current State
func (init *DockerInit) GetState(_ int, stateInfo *StateInfo) error {
	init.Lock()
	*stateInfo = init.StateInfo
	init.Unlock()
	return nil
}

// RPC: Acknowledge the current state and allow dockerinit to start
// transitioning to the next state
func (init *DockerInit) Resume(_ int, _ *int) error {
	select {
	case init.resume <- 1:
	case <-init.cancel:
		// Docker daemon died, return to nowhere
		return fmt.Errorf("foo")
	}
	return nil
}

// RPC: Pass pty master FD
func (init *DockerInit) GetPtyMaster(_ int, rpcFd *rpcfd.RpcFd) error {
	if init.ptyMaster == nil {
		return fmt.Errorf("ptyMaster is nil")
	}
	rpcFd.Fd = init.ptyMaster.Fd()
	return nil
}

// RPC: Pass stdin FD
func (init *DockerInit) GetStdin(_ int, rpcFd *rpcfd.RpcFd) error {
	if init.stdin == nil {
		return fmt.Errorf("stdin is nil")
	}
	rpcFd.Fd = init.stdin.Fd()
	return nil
}

// RPC: Pass stdout FD
func (init *DockerInit) GetStdout(_ int, rpcFd *rpcfd.RpcFd) error {
	if init.stdout == nil {
		return fmt.Errorf("stdout is nil")
	}
	rpcFd.Fd = init.stdout.Fd()
	return nil
}

// RPC: Pass stderr FD
func (init *DockerInit) GetStderr(_ int, rpcFd *rpcfd.RpcFd) error {
	if init.stderr == nil {
		return fmt.Errorf("stderr is nil")
	}
	rpcFd.Fd = init.stderr.Fd()
	return nil
}

// RPC: Get pid
func (init *DockerInit) GetPid(_ int, pid *rpcfd.RpcPid) error {
	if init.process == nil {
		return fmt.Errorf("process doesn't exist")
	}
	// RpcPid converts pid 1 to host namespace pid
	pid.Pid = uintptr(1)
	return nil
}

// RPC: Send a signal to the container process
func (init *DockerInit) Signal(signal syscall.Signal, _ *int) error {
	<-init.processLock // Wait until we have a process
	return init.process.Signal(signal)
}

// Serve RPC commands over a UNIX socket
func rpcServer(init *DockerInit) {

	if err := rpc.Register(init); err != nil {
		log.Fatal(err)
	}

	os.Remove(rpcSocketPath())
	addr := &net.UnixAddr{Net: "unix", Name: rpcSocketPath()}
	listener, err := net.ListenUnix("unix", addr)
	if err != nil {
		log.Fatal(err)
	}

	for {
		conn, err := listener.AcceptUnix()
		if err != nil {
			log.Printf("rpc socket accept error: %s", err)
			continue
		}

		rpcfd.ServeConn(conn)

		conn.Close()

		// The RPC connection has closed, which means the docker daemon
		// exited.  Cancel the WaitForStateChange() call.
		init.cancel <- 1
	}
}

// Wait for docker to call Resume to acknowledge the previous state.
// Then transition to the new state.
func (init *DockerInit) syncNewState(state State) error {
	select {
	case <-init.resume:
	case <-time.After(10 * time.Second):
		return fmt.Errorf("timeout waiting for docker Resume(): state=%v", init.State)
	}

	init.Lock()
	init.State = state
	init.Unlock()

	return nil
}

func dockerInitNew() *DockerInit {
	init := &DockerInit{
		resume:      make(chan int),
		cancel:      make(chan int),
		processLock: make(chan struct{}),
	}
	init.ExitCode = -1
	init.Error = ""
	return init
}

func start(args *InitArgs, cmd *exec.Cmd) error {

	// Process runs in its own session
	cmd.SysProcAttr.Setsid = true

	cmd.Dir = args.WorkDir

	if err := setupHostname(args); err != nil {
		return err
	}

	if err := execdriver.SetupNetworking(args.InitArgs); err != nil {
		return err
	}

	if err := setupCgroups(args); err != nil {
		return err
	}

	if err := setupCapabilities(args); err != nil {
		return err
	}

	// Update uid/gid credentials if needed
	if err := namespaces.SetupUser(args.User); err != nil {
		return fmt.Errorf("setup user %s", err)
	}

	// FIXME: Workaround for libvirt "/.oldroot" directory leak
	// https://bugzilla.redhat.com/show_bug.cgi?id=1026814
	os.Remove("/.oldroot")

	// Start the process
	if err := cmd.Start(); err != nil {
		return err
	}

	return nil
}

// Wait for the process to exit.
// We also forward all signals to the process.
// Also, as pid 1 it's our job to reap all orphaned zombies.
func wait(process *os.Process, sigchan chan os.Signal) int {
	var wstatus syscall.WaitStatus
	var rusage syscall.Rusage

	for sig := range sigchan {
		if sig == syscall.SIGCHLD {
			for {
				pid, err := syscall.Wait4(-1, &wstatus, syscall.WNOHANG, &rusage)
				if err == nil && pid == process.Pid {
					return wstatus.ExitStatus()
				}
				if err != nil && err != syscall.EINTR {
					break
				}
			}
		} else {
			process.Signal(sig)
		}
	}

	panic("unreachable")
}

func getArgs() *InitArgs {
	var (
		// Get cmdline arguments
		user       = flag.String("u", "", "username or uid")
		gateway    = flag.String("g", "", "gateway address")
		ip         = flag.String("i", "", "ip address")
		workDir    = flag.String("w", "", "workdir")
		privileged = flag.Bool("privileged", false, "privileged mode")
		mtu        = flag.Int("mtu", 1500, "interface mtu")
		tty        = flag.Bool("tty", false, "tty")
		openstdin  = flag.Bool("openstdin", false, "open input stream")
	)

	flag.Parse()

	return &InitArgs{
		InitArgs: &execdriver.InitArgs{
			User:       *user,
			Gateway:    *gateway,
			Ip:         *ip,
			WorkDir:    *workDir,
			Privileged: *privileged,
			Args:       flag.Args(),
			Mtu:        *mtu,
		},
		Tty:       *tty,
		OpenStdin: *openstdin,
	}
}

func initializer() {

	var cmd *exec.Cmd
	args := getArgs()
	if err := execdriver.SetupEnv(args.InitArgs); err != nil {
		log.Printf("Can't read .dockerenv: %s", err)
		os.Exit(127)
	}

	init := dockerInitNew()

	// Start the server in Initial state
	go rpcServer(init)

	// Console setup.  Hook up the container process's stdin/stdout/stderr
	// to either a pty or pipes.  The FDs for the controlling side of the
	// pty/pipes will be passed to docker later via rpc.
	earlyErr := func() error {

		// Prepare the cmd based on the given args
		cmdPath, err := exec.LookPath(args.Args[0])
		if err != nil {
			log.Printf("Unable to locate %v", args.Args[0])
			os.Exit(127)
		}
		cmd = exec.Command(cmdPath, args.Args[1:]...)
		cmd.SysProcAttr = &syscall.SysProcAttr{}

		if args.Tty {
			ptyMaster, ptySlave, err := pty.Open()
			if err != nil {
				return err
			}
			init.ptyMaster = ptyMaster
			cmd.Stdout = ptySlave
			cmd.Stderr = ptySlave
			if args.OpenStdin {
				cmd.Stdin = ptySlave
				cmd.SysProcAttr.Setctty = true
			}
		} else {
			stdout, err := cmd.StdoutPipe()
			if err != nil {
				return err
			}
			init.stdout = stdout.(*os.File)

			stderr, err := cmd.StderrPipe()
			if err != nil {
				return err
			}
			init.stderr = stderr.(*os.File)
			if args.OpenStdin {
				// Can't use cmd.StdinPipe() here, since in Go 1.2 it
				// returns an io.WriteCloser with the underlying object
				// being an *exec.closeOnce, neither of which provides
				// a way to convert to an FD.
				pipeRead, pipeWrite, err := os.Pipe()
				if err != nil {
					return err
				}
				cmd.Stdin = pipeRead
				init.stdin = pipeWrite
			}
		}
		return nil
	}()

	// Report any early errors
	if earlyErr != nil {
		init.Error = earlyErr.Error()

		if err := init.syncNewState(FailedToStart); err != nil {
			log.Fatal(err)
			return
		}

		if err := init.syncNewState(Dead); err != nil {
			log.Fatal(err)
			return
		}
	}

	// Tell docker the console FDs are ready for retrieval
	if err := init.syncNewState(ConsoleReady); err != nil {
		log.Fatal(err)
		return
	}

	// Wait for docker to retrieve console FDs and resume
	if err := init.syncNewState(RunReady); err != nil {
		log.Fatal(err)
		return
	}

	// For StdinOnce mode, allow docker to close dockerinit's reference to
	// stdin so that docker can close it later
	//
	// FIXME: is StdinOnce mode obsolete now that dockerinit can keep stdin
	// open?
	if init.stdin != nil {
		init.stdin.Close()
		init.stdin = nil
	}

	// Unmount the socket directory to prevent the container process from
	// trying to impersonate dockerinit.
	syscall.Unmount(SocketPath, syscall.MNT_DETACH)

	// Register a signal handler for forwarding signals to the process and
	// for monitoring children.  Do this before starting the process and
	// setting run state so we don't miss anything.
	sigchan := make(chan os.Signal, 1)
	signal.CatchAll(sigchan)

	// Start the process
	if err := start(args, cmd); err != nil {
		init.Error = err.Error()
		if err := init.syncNewState(FailedToStart); err != nil {
			log.Fatal(err)
			return
		}

		if err := init.syncNewState(Dead); err != nil {
			log.Fatal(err)
			return
		}
	}

	init.process = cmd.Process
	close(init.processLock)

	// Tell docker the process is running
	if err := init.syncNewState(Running); err != nil {
		log.Fatal(err)
		return
	}

	// Wait for it to exit
	init.ExitCode = wait(init.process, sigchan)

	// Tell docker the process has exited
	if err := init.syncNewState(Exited); err != nil {
		log.Fatal(err)
		return
	}

	// Wait for docker to call Resume() one last time.  This gives docker a
	// chance to get the exit code from the RPC interface before we die.
	if err := init.syncNewState(Dead); err != nil {
		log.Fatal(err)
		return
	}

	os.Exit(init.ExitCode)

	panic("unreachable")
}
