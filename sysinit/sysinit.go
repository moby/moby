package sysinit

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/dotcloud/docker/netlink"
	"github.com/dotcloud/docker/utils"
	"io/ioutil"
	"log"
	"net"
	"net/rpc"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const SharedPath = "/.docker-shared"
const RpcSocketName = "rpc.sock"

func rpcSocketPath() string {
	return path.Join(SharedPath, RpcSocketName)
}

type DockerInitRpc struct {
	resume      chan int
	cancel      chan int
	exitCode    chan int
	process     *os.Process
	processLock chan struct{}
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

func setupNetworking(args *DockerInitArgs) error {
	if args.gateway == "" {
		return nil
	}

	ip := net.ParseIP(args.gateway)
	if ip == nil {
		return fmt.Errorf("Unable to set up networking, %s is not a valid IP", args.gateway)
	}

	if err := netlink.AddDefaultGw(ip); err != nil {
		return fmt.Errorf("Unable to set up networking: %v", err)
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

// Start the RPC server and wait for docker to tell us to
// resume starting the container.
func startServerAndWait(dockerInitRpc *DockerInitRpc) error {

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
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Update uid/gid credentials if needed
	credential, err := getCredential(args)
	if err != nil {
		return err
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Credential: credential}

	dockerInitRpc := dockerInitRpcNew()

	// Start the RPC and server and wait for the resume call from docker
	err = startServerAndWait(dockerInitRpc)
	if err != nil {
		return err
	}

	// Network setup
	err = setupNetworking(args)
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

type DockerInitArgs struct {
	user    string
	gateway string
	workDir string
	env     []string
	args    []string
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

	args := &DockerInitArgs{
		user:    *user,
		gateway: *gateway,
		workDir: *workDir,
		env:     env,
		args:    flag.Args(),
	}

	err = dockerInitApp(args)
	if err != nil {
		log.Fatal(err)
	}
}
