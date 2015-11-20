// +build linux

package clr

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"syscall"
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
	dockerenv, err := os.Open(".dockerenv")
	if err != nil {
		return fmt.Errorf("Unable to load environment variables: %v", err)
	}
	defer dockerenv.Close()
	if err := json.NewDecoder(dockerenv).Decode(&env); err != nil {
		return fmt.Errorf("Unable to decode environment variables: %v", err)
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
