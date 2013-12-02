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
	"strconv"
	"strings"
	"syscall"
)

type DockerInitArgs struct {
	user    string
	gateway string
	workDir string
	env     []string
	args    []string
}

// Setup networking
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

// Setup working directory
func setupWorkingDirectory(args *DockerInitArgs) error {
	if args.workDir == "" {
		return nil
	}
	if err := syscall.Chdir(args.workDir); err != nil {
		return fmt.Errorf("Unable to change dir to %v: %v", args.workDir, err)
	}
	return nil
}

// Takes care of dropping privileges to the desired user
func changeUser(args *DockerInitArgs) error {
	if args.user == "" {
		return nil
	}
	userent, err := utils.UserLookup(args.user)
	if err != nil {
		return fmt.Errorf("Unable to find user %v: %v", args.user, err)
	}

	uid, err := strconv.Atoi(userent.Uid)
	if err != nil {
		return fmt.Errorf("Invalid uid: %v", userent.Uid)
	}
	gid, err := strconv.Atoi(userent.Gid)
	if err != nil {
		return fmt.Errorf("Invalid gid: %v", userent.Gid)
	}

	if err := syscall.Setgid(gid); err != nil {
		return fmt.Errorf("setgid failed: %v", err)
	}
	if err := syscall.Setuid(uid); err != nil {
		return fmt.Errorf("setuid failed: %v", err)
	}

	return nil
}

// Clear environment pollution introduced by lxc-start
func setupEnv(args *DockerInitArgs) {
	os.Clearenv()
	for _, kv := range args.env {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) == 1 {
			parts = append(parts, "")
		}
		os.Setenv(parts[0], parts[1])
	}
}

func executeProgram(args *DockerInitArgs) error {
	setupEnv(args)

	if err := setupNetworking(args); err != nil {
		return err
	}

	if err := setupWorkingDirectory(args); err != nil {
		return err
	}

	if err := changeUser(args); err != nil {
		return err
	}

	path, err := exec.LookPath(args.args[0])
	if err != nil {
		log.Printf("Unable to locate %v", args.args[0])
		os.Exit(127)
	}

	if err := syscall.Exec(path, args.args, os.Environ()); err != nil {
		panic(err)
	}

	// Will never reach here
	return nil
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
	if err := json.Unmarshal(content, &env); err != nil {
		log.Fatalf("Unable to unmarshal environment variables: %v", err)
	}

	args := &DockerInitArgs{
		user:    *user,
		gateway: *gateway,
		workDir: *workDir,
		env:     env,
		args:    flag.Args(),
	}

	if err := executeProgram(args); err != nil {
		log.Fatal(err)
	}
}
