package lmctfy

import (
	"encoding/json"
	"fmt"
	"github.com/dotcloud/docker/pkg/system"
	"github.com/dotcloud/docker/pkg/user"
	"github.com/dotcloud/docker/runtime/execdriver"
	"github.com/syndtr/gocapability/capability"
	"io/ioutil"
	"os"
	"strings"
	"syscall"
)

func setupEnv(args *execdriver.InitArgs) error {
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

func setupHostname(args *execdriver.InitArgs) error {
	hostname := getEnv(args, "HOSTNAME")
	if hostname == "" {
		return nil
	}
	return syscall.Sethostname([]byte(hostname))
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

// Takes care of dropping privileges to the desired user
func changeUser(args *execdriver.InitArgs) error {
	uid, gid, suppGids, err := user.GetUserGroupSupplementary(
		args.User,
		syscall.Getuid(), syscall.Getgid(),
	)
	if err != nil {
		return err
	}

	if err := syscall.Setgroups(suppGids); err != nil {
		return fmt.Errorf("Setgroups failed: %v", err)
	}
	if err := syscall.Setgid(gid); err != nil {
		return fmt.Errorf("Setgid failed: %v", err)
	}
	if err := syscall.Setuid(uid); err != nil {
		return fmt.Errorf("Setuid failed: %v", err)
	}

	return nil
}

func setupCapabilities(args *execdriver.InitArgs) error {
	if args.Privileged {
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
		capability.CAP_NET_ADMIN,
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

func getEnv(args *execdriver.InitArgs, key string) string {
	for _, kv := range args.Env {
		parts := strings.SplitN(kv, "=", 2)
		if parts[0] == key && len(parts) == 2 {
			return parts[1]
		}
	}
	return ""
}

// dupSlave dup2 the pty slave's fd into stdout and stdin and ensures that
// the slave's fd is 0, or stdin
func dupSlave(slave *os.File) error {
	if err := system.Dup2(slave.Fd(), 0); err != nil {
		return err
	}
	if err := system.Dup2(slave.Fd(), 1); err != nil {
		return err
	}
	if err := system.Dup2(slave.Fd(), 2); err != nil {
		return err
	}
	return nil
}
