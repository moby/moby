package runconfig

import (
	"github.com/docker/docker/api/types"
	flag "github.com/docker/docker/pkg/mflag"
)

// ParseExec parses the specified args for the specified command and generates
// an ExecConfig from it.
// If the minimal number of specified args is not right or if specified args are
// not valid, it will return an error.
func ParseExec(cmd *flag.FlagSet, args []string) (*types.ExecConfig, error) {
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

	execConfig := &types.ExecConfig{
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
