// +build linux

package namespaces

import (
	"encoding/json"
	"github.com/docker/libcontainer"
	"github.com/docker/libcontainer/label"
	"github.com/docker/libcontainer/system"
	"io"
	"os"
	"os/exec"
	"strconv"
	"syscall"
)

// Runs the command under 'args' inside an existing container referred to by 'container'.
// Returns the exitcode of the command upon success and appropriate error on failure.
func RunIn(container *libcontainer.Config, state *libcontainer.State, args []string, nsinitPath string, stdin io.Reader, stdout, stderr io.Writer, console string, startCallback func(*exec.Cmd)) (int, error) {
	initArgs, err := getNsEnterCommand(strconv.Itoa(state.InitPid), container, console, args)
	if err != nil {
		return -1, err
	}

	cmd := exec.Command(nsinitPath, initArgs...)
	// Note: these are only used in non-tty mode
	// if there is a tty for the container it will be opened within the namespace and the
	// fds will be duped to stdin, stdiout, and stderr
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Start(); err != nil {
		return -1, err
	}
	if startCallback != nil {
		startCallback(cmd)
	}

	if err := cmd.Wait(); err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			return -1, err
		}
	}

	return cmd.ProcessState.Sys().(syscall.WaitStatus).ExitStatus(), nil
}

// ExecIn uses an existing pid and joins the pid's namespaces with the new command.
func ExecIn(container *libcontainer.Config, state *libcontainer.State, args []string) error {
	// Enter the namespace and then finish setup
	args, err := getNsEnterCommand(strconv.Itoa(state.InitPid), container, "", args)
	if err != nil {
		return err
	}

	finalArgs := append([]string{os.Args[0]}, args...)

	if err := system.Execv(finalArgs[0], finalArgs[0:], os.Environ()); err != nil {
		return err
	}

	panic("unreachable")
}

func getContainerJson(container *libcontainer.Config) (string, error) {
	// TODO(vmarmol): If this gets too long, send it over a pipe to the child.
	// Marshall the container into JSON since it won't be available in the namespace.
	containerJson, err := json.Marshal(container)
	if err != nil {
		return "", err
	}
	return string(containerJson), nil
}

func getNsEnterCommand(initPid string, container *libcontainer.Config, console string, args []string) ([]string, error) {
	containerJson, err := getContainerJson(container)
	if err != nil {
		return nil, err
	}

	out := []string{
		"nsenter",
		"--nspid", initPid,
		"--containerjson", containerJson,
	}

	if console != "" {
		out = append(out, "--console", console)
	}
	out = append(out, "--")
	out = append(out, args...)

	return out, nil
}

// Run a command in a container after entering the namespace.
func NsEnter(container *libcontainer.Config, args []string) error {
	// clear the current processes env and replace it with the environment
	// defined on the container
	if err := LoadContainerEnvironment(container); err != nil {
		return err
	}
	if err := FinalizeNamespace(container); err != nil {
		return err
	}

	if container.ProcessLabel != "" {
		if err := label.SetProcessLabel(container.ProcessLabel); err != nil {
			return err
		}
	}

	if err := system.Execv(args[0], args[0:], container.Env); err != nil {
		return err
	}
	panic("unreachable")
}
