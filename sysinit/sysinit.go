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
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"syscall"
)

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

	os.Exit(wstatus.ExitStatus())
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
