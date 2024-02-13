package cli // import "github.com/docker/docker/integration-cli/cli"

import (
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/integration-cli/daemon"
	"github.com/docker/docker/integration-cli/environment"
	"github.com/pkg/errors"
	"gotest.tools/v3/icmd"
)

var testEnv *environment.Execution

// SetTestEnvironment sets a static test environment
// TODO: decouple this package from environment
func SetTestEnvironment(env *environment.Execution) {
	testEnv = env
}

// CmdOperator defines functions that can modify a command
type CmdOperator func(*icmd.Cmd) func()

// DockerCmd executes the specified docker command and expect a success
func DockerCmd(t testing.TB, args ...string) *icmd.Result {
	t.Helper()
	return Docker(Args(args...)).Assert(t, icmd.Success)
}

// BuildCmd executes the specified docker build command and expect a success
func BuildCmd(t testing.TB, name string, cmdOperators ...CmdOperator) *icmd.Result {
	t.Helper()
	return Docker(Args("build", "-t", name), cmdOperators...).Assert(t, icmd.Success)
}

// InspectCmd executes the specified docker inspect command and expect a success
func InspectCmd(t testing.TB, name string, cmdOperators ...CmdOperator) *icmd.Result {
	t.Helper()
	return Docker(Args("inspect", name), cmdOperators...).Assert(t, icmd.Success)
}

// WaitRun will wait for the specified container to be running, maximum 5 seconds.
func WaitRun(t testing.TB, name string, cmdOperators ...CmdOperator) {
	t.Helper()
	waitForInspectResult(t, name, "{{.State.Running}}", "true", 5*time.Second, cmdOperators...)
}

// WaitExited will wait for the specified container to state exit, subject
// to a maximum time limit in seconds supplied by the caller
func WaitExited(t testing.TB, name string, timeout time.Duration, cmdOperators ...CmdOperator) {
	t.Helper()
	waitForInspectResult(t, name, "{{.State.Status}}", "exited", timeout, cmdOperators...)
}

// waitForInspectResult waits for the specified expression to be equals to the specified expected string in the given time.
func waitForInspectResult(t testing.TB, name, expr, expected string, timeout time.Duration, cmdOperators ...CmdOperator) {
	after := time.After(timeout)

	args := []string{"inspect", "-f", expr, name}
	for {
		result := Docker(Args(args...), cmdOperators...)
		if result.Error != nil {
			if !strings.Contains(strings.ToLower(result.Stderr()), "no such") {
				t.Fatalf("error executing docker inspect: %v\n%s",
					result.Stderr(), result.Stdout())
			}
			select {
			case <-after:
				t.Fatal(result.Error)
			default:
				time.Sleep(10 * time.Millisecond)
				continue
			}
		}

		out := strings.TrimSpace(result.Stdout())
		if out == expected {
			break
		}

		select {
		case <-after:
			t.Fatalf("condition \"%q == %q\" not true in time (%v)", out, expected, timeout)
		default:
		}

		time.Sleep(100 * time.Millisecond)
	}
}

// Docker executes the specified docker command
func Docker(cmd icmd.Cmd, cmdOperators ...CmdOperator) *icmd.Result {
	for _, op := range cmdOperators {
		deferFn := op(&cmd)
		if deferFn != nil {
			defer deferFn()
		}
	}
	cmd.Command = append([]string{testEnv.DockerBinary()}, cmd.Command...)
	if err := validateArgs(cmd.Command...); err != nil {
		return &icmd.Result{
			Error: err,
		}
	}
	return icmd.RunCmd(cmd)
}

// validateArgs is a checker to ensure tests are not running commands which are
// not supported on platforms. Specifically on Windows this is 'busybox top'.
func validateArgs(args ...string) error {
	if testEnv.DaemonInfo.OSType != "windows" {
		return nil
	}
	foundBusybox := -1
	for key, value := range args {
		if strings.ToLower(value) == "busybox" {
			foundBusybox = key
		}
		if (foundBusybox != -1) && (key == foundBusybox+1) && (strings.ToLower(value) == "top") {
			return errors.New("cannot use 'busybox top' in tests on Windows. Use runSleepingContainer()")
		}
	}
	return nil
}

// Format sets the specified format with --format flag
func Format(format string) func(*icmd.Cmd) func() {
	return func(cmd *icmd.Cmd) func() {
		cmd.Command = append(
			[]string{cmd.Command[0]},
			append([]string{"--format", fmt.Sprintf("{{%s}}", format)}, cmd.Command[1:]...)...,
		)
		return nil
	}
}

// Args build an icmd.Cmd struct from the specified (command and) arguments.
func Args(commandAndArgs ...string) icmd.Cmd {
	return icmd.Cmd{Command: commandAndArgs}
}

// Daemon points to the specified daemon
func Daemon(d *daemon.Daemon) func(*icmd.Cmd) func() {
	return func(cmd *icmd.Cmd) func() {
		cmd.Command = append([]string{"--host", d.Sock()}, cmd.Command...)
		return nil
	}
}

// WithTimeout sets the timeout for the command to run
func WithTimeout(timeout time.Duration) func(cmd *icmd.Cmd) func() {
	return func(cmd *icmd.Cmd) func() {
		cmd.Timeout = timeout
		return nil
	}
}

// WithEnvironmentVariables sets the specified environment variables for the command to run
func WithEnvironmentVariables(envs ...string) func(cmd *icmd.Cmd) func() {
	return func(cmd *icmd.Cmd) func() {
		cmd.Env = envs
		return nil
	}
}

// WithFlags sets the specified flags for the command to run
func WithFlags(flags ...string) func(*icmd.Cmd) func() {
	return func(cmd *icmd.Cmd) func() {
		cmd.Command = append(cmd.Command, flags...)
		return nil
	}
}

// InDir sets the folder in which the command should be executed
func InDir(path string) func(*icmd.Cmd) func() {
	return func(cmd *icmd.Cmd) func() {
		cmd.Dir = path
		return nil
	}
}

// WithStdout sets the standard output writer of the command
func WithStdout(writer io.Writer) func(*icmd.Cmd) func() {
	return func(cmd *icmd.Cmd) func() {
		cmd.Stdout = writer
		return nil
	}
}

// WithStdin sets the standard input reader for the command
func WithStdin(stdin io.Reader) func(*icmd.Cmd) func() {
	return func(cmd *icmd.Cmd) func() {
		cmd.Stdin = stdin
		return nil
	}
}
