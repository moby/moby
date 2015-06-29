package runconfig

import (
	"github.com/docker/docker/opts"
	flag "github.com/docker/docker/pkg/mflag"
)

type ExecConfig struct {
	User         string
	Env          []string
	Privileged   bool
	Tty          bool
	Container    string
	AttachStdin  bool
	AttachStderr bool
	AttachStdout bool
	Detach       bool
	Cmd          []string
}

func ParseExec(cmd *flag.FlagSet, args []string) (*ExecConfig, error) {
	var (
		flStdin   = cmd.Bool([]string{"i", "-interactive"}, false, "Keep STDIN open even if not attached")
		flTty     = cmd.Bool([]string{"t", "-tty"}, false, "Allocate a pseudo-TTY")
		flDetach  = cmd.Bool([]string{"d", "-detach"}, false, "Detached mode: run command in the background")
		flUser    = cmd.String([]string{"u", "-user"}, "", "Username or UID (format: <name|uid>[:<group|gid>])")
		flEnv     = opts.NewListOpts(opts.ValidateEnv)
		flEnvFile = opts.NewListOpts(nil)
		execCmd   []string
		container string
	)
	cmd.Var(&flEnv, []string{"e", "-env"}, "Set environment variables")
	cmd.Var(&flEnvFile, []string{"-env-file"}, "Read in a file of environment variables")

	cmd.Require(flag.Min, 2)
	if err := cmd.ParseFlags(args, true); err != nil {
		return nil, err
	}
	container = cmd.Arg(0)
	parsedArgs := cmd.Args()
	execCmd = parsedArgs[1:]

	// collect all the environment variables for the container
	envVariables, err := readKVStrings(flEnvFile.GetAll(), flEnv.GetAll())
	if err != nil {
		return nil, err
	}

	execConfig := &ExecConfig{
		User: *flUser,
		// TODO(vishh): Expose 'Privileged' once it is supported.
		// +		//Privileged:   job.GetenvBool("Privileged"),
		Tty:       *flTty,
		Cmd:       execCmd,
		Container: container,
		Detach:    *flDetach,
		Env:       envVariables,
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
