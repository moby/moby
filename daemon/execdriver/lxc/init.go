package lxc

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"syscall"

	"github.com/docker/docker/daemon/execdriver"
	"github.com/docker/docker/reexec"
)

func init() {
	// like always lxc requires a hack to get this to work
	reexec.Register("/.dockerinit", dockerInititalizer)
}

func dockerInititalizer() {
	initializer()
}

// initializer is the lxc driver's init function that is run inside the namespace to setup
// additional configurations
func initializer() {
	runtime.LockOSThread()

	args := getArgs()

	if err := setupNamespace(args); err != nil {
		log.Fatal(err)
	}
}

func setupNamespace(args *execdriver.InitArgs) error {
	if err := execdriver.SetupEnv(args); err != nil {
		return err
	}
	if err := setupHostname(args); err != nil {
		return err
	}
	if err := execdriver.SetupNetworking(args); err != nil {
		return err
	}
	if err := finalizeNamespace(args); err != nil {
		return err
	}

	path, err := exec.LookPath(args.Args[0])
	if err != nil {
		log.Printf("Unable to locate %v", args.Args[0])
		os.Exit(127)
	}

	if err := syscall.Exec(path, args.Args, os.Environ()); err != nil {
		return fmt.Errorf("dockerinit unable to execute %s - %s", path, err)
	}

	return nil
}

func getArgs() *execdriver.InitArgs {
	var (
		// Get cmdline arguments
		user       = flag.String("u", "", "username or uid")
		gateway    = flag.String("g", "", "gateway address")
		ip         = flag.String("i", "", "ip address")
		workDir    = flag.String("w", "", "workdir")
		privileged = flag.Bool("privileged", false, "privileged mode")
		mtu        = flag.Int("mtu", 1500, "interface mtu")
		capAdd     = flag.String("cap-add", "", "capabilities to add")
		capDrop    = flag.String("cap-drop", "", "capabilities to drop")
	)

	flag.Parse()

	return &execdriver.InitArgs{
		User:       *user,
		Gateway:    *gateway,
		Ip:         *ip,
		WorkDir:    *workDir,
		Privileged: *privileged,
		Args:       flag.Args(),
		Mtu:        *mtu,
		CapAdd:     *capAdd,
		CapDrop:    *capDrop,
	}
}

func setupHostname(args *execdriver.InitArgs) error {
	hostname := args.GetEnv("HOSTNAME")
	if hostname == "" {
		return nil
	}
	return setHostname(hostname)
}

// Setup working directory
func setupWorkingDirectory(args *execdriver.InitArgs) error {
	if args.WorkDir == "" {
		return nil
	}
	if err := syscall.Chdir(args.WorkDir); err != nil {
		return fmt.Errorf("Unable to change dir to %v: %v", args.WorkDir, err)
	}
	return nil
}
