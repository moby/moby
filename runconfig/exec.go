package runconfig

import (
	flag "github.com/docker/docker/pkg/mflag"
)

// ExecConfig is a small subset of the Config struct that hold the configuration
// for the exec feature of docker.
type ExecConfig struct {
	User         string   // User that will run the command
	Privileged   bool     // Is the container in privileged mode
	Tty          bool     // Attach standard streams to a tty.
	Container    string   // Name of the container (to execute in)
	AttachStdin  bool     // Attach the standard input, makes possible user interaction
	AttachStderr bool     // Attach the standard output
	AttachStdout bool     // Attach the standard error
	Detach       bool     // Execute in detach mode
	Cmd          []string // Execution commands and args
}

// ParseExec parses the specified args for the specified command and generates
// an ExecConfig from it.
// If the minimal number of specified args is not right or if specified args are
// not valid, it will return an error.
func ParseExec(cmd *flag.FlagSet, args []string) (*ExecConfig, error) {
	var (
		flStdin      = cmd.Bool([]string{"i", "-interactive"}, false, "Keep STDIN open even if not attached")
		flTty        = cmd.Bool([]string{"t", "-tty"}, false, "Allocate a pseudo-TTY")
		flDetach     = cmd.Bool([]string{"d", "-detach"}, false, "Detached mode: run command in the background")
		flUser       = cmd.String([]string{"u", "-user"}, "", "Username or UID (format: <name|uid>[:<group|gid>])")
		flPrivileged = cmd.Bool([]string{"-privileged"}, false, "Give extended privileges to the command")
		execCmd      []string
		container    string
	)
	cmd.Require(flag.Min, 2)
	if err := cmd.ParseFlags(args, true); err != nil {
		return nil, err
	}
	container = cmd.Arg(0)
	parsedArgs := cmd.Args()
	execCmd = parsedArgs[1:]

	execConfig := &ExecConfig{
		User:       *flUser,
		Privileged: *flPrivileged,
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
