package sysinit

import (
	"flag"
	"github.com/dotcloud/docker/engine"
	"github.com/dotcloud/docker/runtime/execdriver"
	_ "github.com/dotcloud/docker/runtime/execdriver/lxc"
	_ "github.com/dotcloud/docker/runtime/execdriver/native"
)

// Sys Init code
// This code is run INSIDE the container and is responsible for setting
// up the environment before running the actual process
func SysInit(job *engine.Job) engine.Status {
	cli := flag.NewFlagSet(job.Args[0], flag.ExitOnError)
	var (
		// Get cmdline arguments
		user       = cli.String("u", "", "username or uid")
		gateway    = cli.String("g", "", "gateway address")
		ip         = cli.String("i", "", "ip address")
		workDir    = cli.String("w", "", "workdir")
		privileged = cli.Bool("privileged", false, "privileged mode")
		mtu        = cli.Int("mtu", 1500, "interface mtu")
		driver     = cli.String("driver", "", "exec driver")
		pipe       = cli.Int("pipe", 0, "sync pipe fd")
		console    = cli.String("console", "", "console (pty slave) path")
		root       = cli.String("root", ".", "root path for configuration files")
	)
	if err := cli.Parse(job.Args); err != nil {
		return job.Error(err)
	}

	args := &execdriver.InitArgs{
		User:       *user,
		Gateway:    *gateway,
		Ip:         *ip,
		WorkDir:    *workDir,
		Privileged: *privileged,
		Args:       cli.Args(),
		Mtu:        *mtu,
		Driver:     *driver,
		Console:    *console,
		Pipe:       *pipe,
		Root:       *root,
	}
	dockerInitFct, err := execdriver.GetInitFunc(args.Driver)
	if err != nil {
		return job.Error(err)
	}
	if err := dockerInitFct(args); err != nil {
		return job.Error(err)
	}
	return engine.StatusOK
}
