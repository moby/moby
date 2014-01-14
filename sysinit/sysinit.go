package sysinit

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/dotcloud/docker/mount"
	"github.com/dotcloud/docker/pkg/netlink"
	"github.com/dotcloud/docker/utils"
	"github.com/syndtr/gocapability/capability"
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
	user       string
	gateway    string
	ip         string
	workDir    string
	privileged bool
	env        []string
	args       []string
	mtu        int
	driver     string
}

func setupHostname(args *DockerInitArgs) error {
	hostname := getEnv(args, "HOSTNAME")
	if hostname == "" {
		return nil
	}
	return setHostname(hostname)
}

// Setup networking
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
		if err := netlink.NetworkLinkAddIp(iface, ip, ipNet); err != nil {
			return fmt.Errorf("Unable to set up networking: %v", err)
		}
		if err := netlink.NetworkSetMTU(iface, args.mtu); err != nil {
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

func setupMounts(args *DockerInitArgs) error {
	return mount.ForceMount("proc", "proc", "proc", "")
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

	if err := c.Apply(capability.CAPS | capability.BOUNDS); err != nil {
		return err
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

func getEnv(args *DockerInitArgs, key string) string {
	for _, kv := range args.env {
		parts := strings.SplitN(kv, "=", 2)
		if parts[0] == key && len(parts) == 2 {
			return parts[1]
		}
	}
	return ""
}

func executeProgram(args *DockerInitArgs) error {
	setupEnv(args)

	if args.driver == "lxc" {
		if err := setupHostname(args); err != nil {
			return err
		}

		if err := setupNetworking(args); err != nil {
			return err
		}

		if err := setupCapabilities(args); err != nil {
			return err
		}
		if err := setupWorkingDirectory(args); err != nil {
			return err
		}

		if err := changeUser(args); err != nil {
			return err
		}
	} else if args.driver == "chroot" {
		if err := setupMounts(args); err != nil {
			return err
		}
		defer mount.ForceUnmount("proc")
	}

	path, err := exec.LookPath(args.args[0])
	if err != nil {
		log.Printf("Unable to locate %v", args.args[0])
		os.Exit(127)
	}

	if args.driver == "lxc" {
		if err := syscall.Exec(path, args.args, os.Environ()); err != nil {
			panic(err)
		}
		// Will never reach
	} else if args.driver == "chroot" {
		cmd := exec.Command(path, args.args[1:]...)

		cmd.Stderr = os.Stderr
		cmd.Stdout = os.Stdout
		cmd.Stdin = os.Stdin

		return cmd.Run()
	}
	panic("Should not be here")

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
	ip := flag.String("i", "", "ip address")
	workDir := flag.String("w", "", "workdir")
	privileged := flag.Bool("privileged", false, "privileged mode")
	mtu := flag.Int("mtu", 1500, "interface mtu")
	driver := flag.String("driver", "", "exec driver")
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

	// Propagate the plugin-specific container env variable
	env = append(env, "container="+os.Getenv("container"))

	args := &DockerInitArgs{
		user:       *user,
		gateway:    *gateway,
		ip:         *ip,
		workDir:    *workDir,
		privileged: *privileged,
		env:        env,
		args:       flag.Args(),
		mtu:        *mtu,
		driver:     *driver,
	}

	if err := executeProgram(args); err != nil {
		log.Fatal(err)
	}
}
