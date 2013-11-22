package sysinit

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/dotcloud/docker/netlink"
	"github.com/dotcloud/docker/utils"
	"github.com/kr/pty"
	"github.com/syndtr/gocapability/capability"
	"io/ioutil"
	"log"
	"net"
	"net/rpc"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const SharedPath = "/.docker-shared"
const RpcSocketName = "rpc.sock"
const ConsoleSocketName = "con.sock"

func rpcSocketPath() string {
	return path.Join(SharedPath, RpcSocketName)
}

func consoleSocketPath() string {
	return path.Join(SharedPath, ConsoleSocketName)
}

type DockerInitRpc struct {
	resume      chan int
	cancel      chan int
	exitCode    chan int
	process     *os.Process
	processLock chan struct{}
}

type DockerInitConsole struct {
	stdin     *os.File
	stdout    *os.File
	stderr    *os.File
	ptyMaster *os.File
	openStdin bool
}

// RPC: Resume container start or container exit
func (dockerInitRpc *DockerInitRpc) Resume(_ int, _ *int) error {
	dockerInitRpc.resume <- 1
	return nil
}

// RPC: Wait for container app exit and return the exit code.
//
// For machine containers that have their own init, this function doesn't
// actually return, but that's ok.  The init process (pid 1) will die, which
// will automatically kill all the other container tasks, including the
// non-pid-1 dockerinit.  Docker's RPC Wait() call will detect that the socket
// closed and return an error.
func (dockerInitRpc *DockerInitRpc) Wait(_ int, exitCode *int) error {
	select {
	case *exitCode = <-dockerInitRpc.exitCode:
	case <-dockerInitRpc.cancel:
		*exitCode = -1
	}
	return nil
}

// RPC: Send a signal to the container app
func (dockerInitRpc *DockerInitRpc) Signal(signal syscall.Signal, _ *int) error {
	<-dockerInitRpc.processLock
	return dockerInitRpc.process.Signal(signal)
}

// Serve RPC commands over a UNIX socket
func rpcServer(dockerInitRpc *DockerInitRpc) {

	err := rpc.Register(dockerInitRpc)
	if err != nil {
		log.Fatal(err)
	}

	os.Remove(rpcSocketPath())
	addr := &net.UnixAddr{Net: "unix", Name: rpcSocketPath()}
	listener, err := net.ListenUnix("unix", addr)
	if err != nil {
		log.Fatal(err)
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("rpc socket accept error: %s", err)
			continue
		}

		rpc.ServeConn(conn)

		conn.Close()

		// The RPC connection has closed, which means the docker daemon
		// exited.  Cancel the Wait() call.
		dockerInitRpc.cancel <- 1
	}
}

// Send console FDs to docker over a UNIX socket
func consoleFdServer(dockerInitConsole *DockerInitConsole) {

	os.Remove(consoleSocketPath())
	addr := &net.UnixAddr{Net: "unix", Name: consoleSocketPath()}
	listener, err := net.ListenUnix("unix", addr)
	if err != nil {
		log.Fatal(err)
	}

	for {
		conn, err := listener.AcceptUnix()
		if err != nil {
			log.Printf("fd socket accept error: %s", err)
			continue
		}

		dummy := []byte("1")
		var fds []int
		if dockerInitConsole.ptyMaster != nil {
			fds = []int{int(dockerInitConsole.ptyMaster.Fd())}
		} else {
			fds = []int{
				int(dockerInitConsole.stdout.Fd()),
				int(dockerInitConsole.stderr.Fd())}

			if dockerInitConsole.stdin != nil {
				fds = append(fds, int(dockerInitConsole.stdin.Fd()))
			}
		}

		rights := syscall.UnixRights(fds...)
		_, _, err = conn.WriteMsgUnix(dummy, rights, nil)
		if err != nil {
			log.Printf("%s", err)
		}

		// Only give stdin to the first caller and then close it on our
		// side.  This gives the docker daemon the power to close the
		// app's stdin in StdinOnce mode.
		if dockerInitConsole.openStdin && dockerInitConsole.stdin != nil {
			dockerInitConsole.stdin.Close()
			dockerInitConsole.stdin = nil
		}

		conn.Close()
	}
}

func setupHostname(args *DockerInitArgs) error {
	hostname := getEnv(args, "HOSTNAME")
	if hostname == "" {
		return nil
	}
	return syscall.Sethostname([]byte(hostname))
}

func setupNetworking(args *DockerInitArgs) error {
	if args.ip != "" {
		// eth0
		iface, err := net.InterfaceByName("eth0")
		if err != nil {
			return fmt.Errorf("Unable to set up networking: %v", err)
		}
		ip, ipNet, err := net.ParseCIDR(args.ip)
		if err != nil {
			return fmt.Errorf("Unable to set up networking: %v", err)
		}
		err = netlink.NetworkLinkAddIp(iface, ip, ipNet)
		if err != nil {
			return fmt.Errorf("Unable to set up networking: %v", err)
		}
		err = netlink.NetworkLinkUp(iface)
		if err != nil {
			return fmt.Errorf("Unable to set up networking: %v", err)
		}

		// loopback
		iface, err = net.InterfaceByName("lo")
		if err != nil {
			return fmt.Errorf("Unable to set up networking: %v", err)
		}
		err = netlink.NetworkLinkUp(iface)
		if err != nil {
			return fmt.Errorf("Unable to set up networking: %v", err)
		}
	}
	if args.gateway != "" {
		gw := net.ParseIP(args.gateway)
		if gw == nil {
			return fmt.Errorf("Unable to set up networking, %s is not a valid gateway IP", args.gateway)
		}

		if err := netlink.AddDefaultGw(gw); err != nil {
			return fmt.Errorf("Unable to set up networking: %v", err)
		}
	}
	return nil
}

func getCredential(args *DockerInitArgs) (*syscall.Credential, error) {
	if args.user == "" {
		return nil, nil
	}
	userent, err := utils.UserLookup(args.user)
	if err != nil {
		return nil, fmt.Errorf("Unable to find user %v: %v", args.user, err)
	}

	uid, err := strconv.Atoi(userent.Uid)
	if err != nil {
		return nil, fmt.Errorf("Invalid uid: %v", userent.Uid)
	}
	gid, err := strconv.Atoi(userent.Gid)
	if err != nil {
		return nil, fmt.Errorf("Invalid gid: %v", userent.Gid)
	}

	return &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid)}, nil
}

func getEnv(args *DockerInitArgs, key string) string {
	for _, kv := range args.env {
		parts := strings.SplitN(kv, "=", 2)
		if parts[0] == key && len(parts) == 2 {
			return parts[1]
		}
	}
	return ""
}

func getCmdPath(args *DockerInitArgs) (string, error) {

	// Set PATH in dockerinit so we can find the cmd
	envPath := getEnv(args, "PATH")
	if envPath != "" {
		os.Setenv("PATH", envPath)
	}

	// Find the cmd
	cmdPath, err := exec.LookPath(args.args[0])
	if err != nil {
		if args.workDir == "" {
			return "", err
		}
		cmdPath, err = exec.LookPath(path.Join(args.workDir, args.args[0]))
		if err != nil {
			return "", err
		}
	}

	return cmdPath, nil
}

func setupCapabilities(args *DockerInitArgs) error {

	if args.privileged {
		return nil
	}

	drop := []capability.Cap{
		capability.CAP_SETPCAP,
		capability.CAP_SYS_MODULE,
		capability.CAP_SYS_RAWIO,
		capability.CAP_SYS_PACCT,
		capability.CAP_SYS_ADMIN,
		capability.CAP_SYS_NICE,
		capability.CAP_SYS_RESOURCE,
		capability.CAP_SYS_TIME,
		capability.CAP_SYS_TTY_CONFIG,
		capability.CAP_MKNOD,
		capability.CAP_AUDIT_WRITE,
		capability.CAP_AUDIT_CONTROL,
		capability.CAP_MAC_OVERRIDE,
		capability.CAP_MAC_ADMIN,
	}

	c, err := capability.NewPid(os.Getpid())
	if err != nil {
		return err
	}

	c.Unset(capability.CAPS|capability.BOUNDS, drop...)

	err = c.Apply(capability.CAPS | capability.BOUNDS)
	if err != nil {
		return err
	}
	return nil
}

func setupCommon(args *DockerInitArgs) error {

	err := setupHostname(args)
	if err != nil {
		return err
	}
	err = setupNetworking(args)
	if err != nil {
		return err
	}
	err = setupCapabilities(args)
	if err != nil {
		return err
	}
	return nil
}

// Start the RPC and console FD servers and wait for docker to tell us to
// resume starting the container.  This gives docker a chance to get the
// console FDs before we start so that it won't miss any console output.
func startServersAndWait(dockerInitRpc *DockerInitRpc, dockerInitConsole *DockerInitConsole) error {

	go consoleFdServer(dockerInitConsole)
	go rpcServer(dockerInitRpc)

	select {
	case <-dockerInitRpc.resume:
		break
	case <-time.After(time.Second):
		return fmt.Errorf("timeout waiting for docker Resume()")
	}

	return nil
}

func dockerInitRpcNew() *DockerInitRpc {
	return &DockerInitRpc{
		resume:      make(chan int),
		exitCode:    make(chan int),
		cancel:      make(chan int),
		processLock: make(chan struct{}),
	}
}

func dockerInitConsoleNew(args *DockerInitArgs) *DockerInitConsole {
	return &DockerInitConsole{
		openStdin: args.openStdin,
	}
}

// Run as pid 1 in the typical Docker usage: an app container that doesn't
// need its own init process.  Running as pid 1 allows us to monitor the
// container app and return its exit code.
func dockerInitApp(args *DockerInitArgs) error {

	// Prepare the cmd based on the given args
	cmdPath, err := getCmdPath(args)
	if err != nil {
		return err
	}
	cmd := exec.Command(cmdPath, args.args[1:]...)
	cmd.Dir = args.workDir
	cmd.Env = args.env

	// Update uid/gid credentials if needed
	credential, err := getCredential(args)
	if err != nil {
		return err
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Credential: credential}

	// App runs in its own session
	cmd.SysProcAttr.Setsid = true

	// Console setup.  Hook up the container app's stdin/stdout/stderr to
	// either a pty or pipes.  The FDs for the controlling side of the
	// pty/pipes will be passed to docker later via a UNIX socket.
	dockerInitConsole := dockerInitConsoleNew(args)
	if args.tty {
		ptyMaster, ptySlave, err := pty.Open()
		if err != nil {
			return err
		}
		dockerInitConsole.ptyMaster = ptyMaster
		cmd.Stdout = ptySlave
		cmd.Stderr = ptySlave
		if args.openStdin {
			cmd.Stdin = ptySlave
			cmd.SysProcAttr.Setctty = true
		}
	} else {
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return err
		}
		dockerInitConsole.stdout = stdout.(*os.File)

		stderr, err := cmd.StderrPipe()
		if err != nil {
			return err
		}
		dockerInitConsole.stderr = stderr.(*os.File)
		if args.openStdin {
			// Can't use cmd.StdinPipe() here, since in Go 1.2 it
			// returns an io.WriteCloser with the underlying object
			// being an *exec.closeOnce, neither of which provides
			// a way to convert to an FD.
			pipeRead, pipeWrite, err := os.Pipe()
			if err != nil {
				return err
			}
			cmd.Stdin = pipeRead
			dockerInitConsole.stdin = pipeWrite
		}
	}

	dockerInitRpc := dockerInitRpcNew()

	// Start the RPC and console FD servers and wait for the resume call
	// from docker
	err = startServersAndWait(dockerInitRpc, dockerInitConsole)
	if err != nil {
		return err
	}

	// Container setup
	err = setupCommon(args)
	if err != nil {
		return err
	}

	// Start the app
	err = cmd.Start()
	if err != nil {
		return err
	}

	dockerInitRpc.process = cmd.Process
	close(dockerInitRpc.processLock)

	// Forward all signals to the app
	sigchan := make(chan os.Signal, 1)
	utils.CatchAll(sigchan)
	go func() {
		for sig := range sigchan {
			if sig == syscall.SIGCHLD {
				continue
			}
			cmd.Process.Signal(sig)
		}
	}()

	// Wait for the app to exit.  Also, as pid 1 it's our job to reap all
	// orphaned zombies.
	var wstatus syscall.WaitStatus
	for {
		var rusage syscall.Rusage
		pid, err := syscall.Wait4(-1, &wstatus, 0, &rusage)
		if err == nil && pid == cmd.Process.Pid {
			break
		}
	}

	// Update the exit code for Wait() and detect timeout if Wait() hadn't
	// been called
	exitCode := wstatus.ExitStatus()
	select {
	case dockerInitRpc.exitCode <- exitCode:
	case <-time.After(time.Second):
		return fmt.Errorf("timeout waiting for docker Wait()")
	}

	// Wait for docker to call Resume() again.  This gives docker a chance
	// to get the exit code from the RPC socket call interface before we
	// die.
	select {
	case <-dockerInitRpc.resume:
	case <-time.After(time.Second):
		return fmt.Errorf("timeout waiting for docker Resume()")
	}

	os.Exit(exitCode)
	return nil
}

// Runs as pid 1 when starting a machine container that has its own init
// process.  Start the dockerinit child, do some container setup, and then exec
// the real init.
func dockerInitMachineParent(args *DockerInitArgs) error {

	// Create a pty slave to be used by the container for its console.  The
	// pty master will be owned by the dockerinit child process.
	ptyMaster, ptySlave, err := pty.Open()
	if err != nil {
		return err
	}
	defer ptyMaster.Close()
	defer ptySlave.Close()

	// Hook up /dev/console to the pty with a bind mount
	err = syscall.Mount(ptySlave.Name(), "/dev/console", "", syscall.MS_BIND, "")
	if err != nil {
		return err
	}

	// Container setup
	err = setupCommon(args)
	if err != nil {
		return err
	}

	// Hook up stdin/stdout/stderr to the pty
	fd := int(ptySlave.Fd())
	err = syscall.Dup2(fd, 0)
	if err != nil {
		return err
	}
	err = syscall.Dup2(fd, 1)
	if err != nil {
		return err
	}
	err = syscall.Dup2(fd, 2)
	if err != nil {
		return err
	}

	// Prepare to receive a signal from the child dockerinit
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, syscall.SIGUSR1)

	// Prepare to start the long-running dockerinit child via the
	// dockerinit "-child" option
	cmdArgs := append([]string{"-child"}, os.Args[1:]...)
	cmd := exec.Command(os.Args[0], cmdArgs...)

	// Pass the pty master FD to the child dockerinit so that it can access
	// the parent's console
	cmd.ExtraFiles = []*os.File{ptyMaster}

	// Put child dockerinit in its own session so that it doesn't get a
	// signal when e.g. systemd does TIOCNOTTY
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	// Set the child uid/gid credentials if needed.  Not sure if this
	// really makes sense for a machine container, but if the user asked
	// for it...
	credential, err := getCredential(args)
	if err != nil {
		return err
	}
	cmd.SysProcAttr.Credential = credential

	// Start the child
	err = cmd.Start()
	if err != nil {
		return err
	}

	// Wait for signal to continue from the child
	<-sigchan
	signal.Stop(sigchan)

	// Exec the container's real init process
	path, err := exec.LookPath(args.args[0])
	if err != nil {
		return err
	}
	err = syscall.Exec(path, args.args, args.env)
	return err
}

// Long-running non-pid-1 dockerinit for the machine container case.  Started
// by dockerInitMachineParent().
func dockerInitMachineChild(args *DockerInitArgs) error {

	// Create the RPC struct to monitor pid 1 and send signals to it
	dockerInitRpc := dockerInitRpcNew()
	var err error
	dockerInitRpc.process, err = os.FindProcess(1)
	if err != nil {
		return err
	}
	close(dockerInitRpc.processLock)

	// Create the dockerInitConsole struct and pass it the ptyMaster that
	// was sent by dockerInitMachineParent()
	dockerInitConsole := dockerInitConsoleNew(args)
	dockerInitConsole.ptyMaster = os.NewFile(3, "ptyMaster")

	// Start the RPC and console FD servers and wait for the resume call
	// from docker
	err = startServersAndWait(dockerInitRpc, dockerInitConsole)
	if err != nil {
		return err
	}

	// We're ready now.  Tell dockerInitMachineParent() to exec the real init.
	err = dockerInitRpc.process.Signal(syscall.SIGUSR1)
	if err != nil {
		return err
	}

	// Sleep forever while the servers run...
	for {
		time.Sleep(time.Hour)
	}
}

type DockerInitArgs struct {
	user       string
	gateway    string
	workDir    string
	ip         string
	tty        bool
	openStdin  bool
	privileged bool
	child      bool
	env        []string
	args       []string
}

// Sys Init code
// This code is run INSIDE the container and is responsible for setting
// up the environment before running the actual process
func SysInit() {
	if len(os.Args) <= 1 {
		fmt.Println("You should not invoke dockerinit manually")
		os.Exit(1)
	}

	// Get cmdline arguments
	user := flag.String("u", "", "username or uid")
	gateway := flag.String("g", "", "gateway address")
	workDir := flag.String("w", "", "workdir")
	ip := flag.String("i", "", "ip address")
	tty := flag.Bool("tty", false, "use pseudo-tty")
	openStdin := flag.Bool("stdin", false, "open stdin")
	privileged := flag.Bool("privileged", false, "privileged mode")
	child := flag.Bool("child", false, "is child dockerinit")
	flag.Parse()

	// Get env
	var env []string
	content, err := ioutil.ReadFile("/.dockerenv")
	if err != nil {
		log.Fatalf("Unable to load environment variables: %v", err)
	}
	err = json.Unmarshal(content, &env)
	if err != nil {
		log.Fatalf("Unable to unmarshal environment variables: %v", err)
	}

	// Propagate the plugin-specific container env variable
	env = append(env, "container="+os.Getenv("container"))

	args := &DockerInitArgs{
		user:       *user,
		gateway:    *gateway,
		workDir:    *workDir,
		ip:         *ip,
		tty:        *tty,
		openStdin:  *openStdin,
		privileged: *privileged,
		child:      *child,
		env:        env,
		args:       flag.Args(),
	}

	if args.child {
		// Machine container child
		err = dockerInitMachineChild(args)
	} else if path.Base(args.args[0]) == "systemd" || args.args[0] == "/sbin/init" {
		// Machine container parent
		err = dockerInitMachineParent(args)
	} else {
		// Typical docker usage: app container
		err = dockerInitApp(args)
	}

	if err != nil {
		log.Fatal(err)
	}
}
