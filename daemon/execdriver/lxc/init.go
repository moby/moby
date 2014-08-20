package lxc

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"

	"github.com/docker/docker/reexec"
	"github.com/docker/libcontainer/netlink"
)

// Args provided to the init function for a driver
type InitArgs struct {
	User       string
	Gateway    string
	Ip         string
	WorkDir    string
	Privileged bool
	Env        []string
	Args       []string
	Mtu        int
	Console    string
	Pipe       int
	Root       string
	CapAdd     string
	CapDrop    string
}

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

func setupNamespace(args *InitArgs) error {
	if err := setupEnv(args); err != nil {
		return err
	}
	if err := setupHostname(args); err != nil {
		return err
	}
	if err := setupNetworking(args); err != nil {
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

func getArgs() *InitArgs {
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

	return &InitArgs{
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

// Clear environment pollution introduced by lxc-start
func setupEnv(args *InitArgs) error {
	// Get env
	var env []string
	content, err := ioutil.ReadFile(".dockerenv")
	if err != nil {
		return fmt.Errorf("Unable to load environment variables: %v", err)
	}
	if err := json.Unmarshal(content, &env); err != nil {
		return fmt.Errorf("Unable to unmarshal environment variables: %v", err)
	}
	// Propagate the plugin-specific container env variable
	env = append(env, "container="+os.Getenv("container"))

	args.Env = env

	os.Clearenv()
	for _, kv := range args.Env {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) == 1 {
			parts = append(parts, "")
		}
		os.Setenv(parts[0], parts[1])
	}

	return nil
}

func setupHostname(args *InitArgs) error {
	hostname := getEnv(args, "HOSTNAME")
	if hostname == "" {
		return nil
	}
	return setHostname(hostname)
}

// Setup networking
func setupNetworking(args *InitArgs) error {
	if args.Ip != "" {
		// eth0
		iface, err := net.InterfaceByName("eth0")
		if err != nil {
			return fmt.Errorf("Unable to set up networking: %v", err)
		}
		ip, ipNet, err := net.ParseCIDR(args.Ip)
		if err != nil {
			return fmt.Errorf("Unable to set up networking: %v", err)
		}
		if err := netlink.NetworkLinkAddIp(iface, ip, ipNet); err != nil {
			return fmt.Errorf("Unable to set up networking: %v", err)
		}
		if err := netlink.NetworkSetMTU(iface, args.Mtu); err != nil {
			return fmt.Errorf("Unable to set MTU: %v", err)
		}
		if err := netlink.NetworkLinkUp(iface); err != nil {
			return fmt.Errorf("Unable to set up networking: %v", err)
		}

		// loopback
		iface, err = net.InterfaceByName("lo")
		if err != nil {
			return fmt.Errorf("Unable to set up networking: %v", err)
		}
		if err := netlink.NetworkLinkUp(iface); err != nil {
			return fmt.Errorf("Unable to set up networking: %v", err)
		}
	}
	if args.Gateway != "" {
		gw := net.ParseIP(args.Gateway)
		if gw == nil {
			return fmt.Errorf("Unable to set up networking, %s is not a valid gateway IP", args.Gateway)
		}

		if err := netlink.AddDefaultGw(gw.String(), "eth0"); err != nil {
			return fmt.Errorf("Unable to set up networking: %v", err)
		}
	}

	return nil
}

// Setup working directory
func setupWorkingDirectory(args *InitArgs) error {
	if args.WorkDir == "" {
		return nil
	}
	if err := syscall.Chdir(args.WorkDir); err != nil {
		return fmt.Errorf("Unable to change dir to %v: %v", args.WorkDir, err)
	}
	return nil
}

func getEnv(args *InitArgs, key string) string {
	for _, kv := range args.Env {
		parts := strings.SplitN(kv, "=", 2)
		if parts[0] == key && len(parts) == 2 {
			return parts[1]
		}
	}
	return ""
}
