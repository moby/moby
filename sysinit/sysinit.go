package sysinit

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"

	"github.com/docker/docker/daemon/execdriver"
	_ "github.com/docker/docker/daemon/execdriver/lxc"
	_ "github.com/docker/docker/daemon/execdriver/native"
)

func executeProgram(args *execdriver.InitArgs) error {
	dockerInitFct, err := execdriver.GetInitFunc(args.Driver)
	if err != nil {
		panic(err)
	}
	return dockerInitFct(args)
}

// Sys Init code
// This code is run INSIDE the container and is responsible for setting
// up the environment before running the actual process
func SysInit() {
	// The very first thing that we should do is lock the thread so that other
	// system level options will work and not have issues, i.e. setns
	runtime.LockOSThread()

	if len(os.Args) <= 1 {
		fmt.Println("You should not invoke dockerinit manually")
		os.Exit(1)
	}

	var (
		// Get cmdline arguments
		user       = flag.String("u", "", "username or uid")
		gateway    = flag.String("g", "", "gateway address")
		ip         = flag.String("i", "", "ip address")
		workDir    = flag.String("w", "", "workdir")
		privileged = flag.Bool("privileged", false, "privileged mode")
		mtu        = flag.Int("mtu", 1500, "interface mtu")
		driver     = flag.String("driver", "", "exec driver")
		pipe       = flag.Int("pipe", 0, "sync pipe fd")
		console    = flag.String("console", "", "console (pty slave) path")
		root       = flag.String("root", ".", "root path for configuration files")
		capAdd     = flag.String("cap-add", "", "capabilities to add")
		capDrop    = flag.String("cap-drop", "", "capabilities to drop")
	)
	flag.Parse()

	args := &execdriver.InitArgs{
		User:       *user,
		Gateway:    *gateway,
		Ip:         *ip,
		WorkDir:    *workDir,
		Privileged: *privileged,
		Args:       flag.Args(),
		Mtu:        *mtu,
		Driver:     *driver,
		Console:    *console,
		Pipe:       *pipe,
		Root:       *root,
		CapAdd:     *capAdd,
		CapDrop:    *capDrop,
	}

	if err := executeProgram(args); err != nil {
		log.Fatal(err)
	}
}
