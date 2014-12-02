package runconfig

import (
	"fmt"

	"github.com/docker/docker/engine"
	flag "github.com/docker/docker/pkg/mflag"
)

type ExecConfig struct {
	User         string
	Privileged   bool
	Tty          bool
	Container    string
	AttachStdin  bool
	AttachStderr bool
	AttachStdout bool
	Detach       bool
	Cmd          []string
}

func ExecConfigFromJob(job *engine.Job) (*ExecConfig, error) {
	execConfig := &ExecConfig{
		// TODO(vishh): Expose 'User' once it is supported.
		//User:         job.Getenv("User"),
		// TODO(vishh): Expose 'Privileged' once it is supported.
		//Privileged:   job.GetenvBool("Privileged"),
		Tty:          job.GetenvBool("Tty"),
		AttachStdin:  job.GetenvBool("AttachStdin"),
		AttachStderr: job.GetenvBool("AttachStderr"),
		AttachStdout: job.GetenvBool("AttachStdout"),
	}
	cmd := job.GetenvList("Cmd")
	if len(cmd) == 0 {
		return nil, fmt.Errorf("No exec command specified")
	}

	execConfig.Cmd = cmd

	return execConfig, nil
}

func ParseExec(cmd *flag.FlagSet, args []string) (*ExecConfig, error) {
	var (
		flStdin   = cmd.Bool([]string{"i", "-interactive"}, false, "Keep STDIN open even if not attached")
		flTty     = cmd.Bool([]string{"t", "-tty"}, false, "Allocate a pseudo-TTY")
		flDetach  = cmd.Bool([]string{"d", "-detach"}, false, "Detached mode: run command in the background")
		execCmd   []string
		container string
	)
	if err := cmd.Parse(args); err != nil {
		return nil, err
	}
	parsedArgs := cmd.Args()
	if len(parsedArgs) < 2 {
		return nil, fmt.Errorf("not enough arguments to create exec command")
	}
	container = cmd.Arg(0)
	execCmd = parsedArgs[1:]

	execConfig := &ExecConfig{
		// TODO(vishh): Expose '-u' flag once it is supported.
		User: "",
		// TODO(vishh): Expose '-p' flag once it is supported.
		Privileged: false,
		Tty:        *flTty,
		Cmd:        execCmd,
		Container:  container,
		Detach:     *flDetach,
	}

	// If -d is not set, attach to everything by default
	if !*flDetach {
		execConfig.AttachStdout = true
		execConfig.AttachStderr = true
		if *flStdin {
			execConfig.AttachStdin = true
		}
	}

	return execConfig, nil
}
