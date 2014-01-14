package sysinit

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/dotcloud/docker/execdriver"
	"io/ioutil"
	"log"
	"os"
	"strings"
)

// Clear environment pollution introduced by lxc-start
func setupEnv(args *execdriver.DockerInitArgs) {
	os.Clearenv()
	for _, kv := range args.Env {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) == 1 {
			parts = append(parts, "")
		}
		os.Setenv(parts[0], parts[1])
	}
}

func executeProgram(args *execdriver.DockerInitArgs) error {
	setupEnv(args)
	dockerInitFct, err := execdriver.GetDockerInitFct(args.Driver)
	if err != nil {
		panic(err)
	}
	return dockerInitFct(args)

	if args.Driver == "lxc" {
		// Will never reach
	} else if args.Driver == "chroot" {
	}

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

	args := &execdriver.DockerInitArgs{
		User:       *user,
		Gateway:    *gateway,
		Ip:         *ip,
		WorkDir:    *workDir,
		Privileged: *privileged,
		Env:        env,
		Args:       flag.Args(),
		Mtu:        *mtu,
		Driver:     *driver,
	}

	if err := executeProgram(args); err != nil {
		log.Fatal(err)
	}
}
