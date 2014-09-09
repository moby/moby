package runconfig

import (
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
	Hostname     string
}

func ExecConfigFromJob(job *engine.Job) *ExecConfig {
	execConfig := &ExecConfig{
		User:         job.Getenv("User"),
		Privileged:   job.GetenvBool("Privileged"),
		Tty:          job.GetenvBool("Tty"),
		Container:    job.Getenv("Container"),
		AttachStdin:  job.GetenvBool("AttachStdin"),
		AttachStderr: job.GetenvBool("AttachStderr"),
		AttachStdout: job.GetenvBool("AttachStdout"),
	}
	if Cmd := job.GetenvList("Cmd"); Cmd != nil {
		execConfig.Cmd = Cmd
	}

	return execConfig
}

func ParseExec(cmd *flag.FlagSet, args []string) (*ExecConfig, error) {
	var (
		flPrivileged = cmd.Bool([]string{"#privileged", "-privileged"}, false, "Give extended privileges to this container")
		flStdin      = cmd.Bool([]string{"i", "-interactive"}, false, "Keep STDIN open even if not attached")
		flTty        = cmd.Bool([]string{"t", "-tty"}, false, "Allocate a pseudo-TTY")
		flHostname   = cmd.String([]string{"h", "-hostname"}, "", "Container host name")
		flUser       = cmd.String([]string{"u", "-user"}, "", "Username or UID")
		flDetach     = cmd.Bool([]string{"d", "-detach"}, false, "Detached mode: run command in the background")
		execCmd      []string
		container    string
	)
	if err := cmd.Parse(args); err != nil {
		return nil, err
	}
	parsedArgs := cmd.Args()
	if len(parsedArgs) > 1 {
		container = cmd.Arg(0)
		execCmd = parsedArgs[1:]
	}

	execConfig := &ExecConfig{
		User:       *flUser,
		Privileged: *flPrivileged,
		Tty:        *flTty,
		Cmd:        execCmd,
		Container:  container,
		Hostname:   *flHostname,
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
