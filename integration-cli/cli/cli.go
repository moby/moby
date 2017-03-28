package cli

import (
	"fmt"
	"sync"
	"time"

	"github.com/docker/docker/integration-cli/daemon"
	"github.com/docker/docker/integration-cli/environment"
	icmd "github.com/docker/docker/pkg/testutil/cmd"
)

var (
	testEnv  *environment.Execution
	onlyOnce sync.Once
)

// EnsureTestEnvIsLoaded make sure the test environment is loaded for this package
func EnsureTestEnvIsLoaded(t testingT) {
	var doIt bool
	var err error
	onlyOnce.Do(func() {
		doIt = true
	})

	if !doIt {
		return
	}
	testEnv, err = environment.New()
	if err != nil {
		t.Fatalf("error loading testenv : %v", err)
	}
}

// CmdOperator defines functions that can modify a command
type CmdOperator func(*icmd.Cmd) func()

type testingT interface {
	Fatalf(string, ...interface{})
}

// DockerCmd executes the specified docker command and expect a success
func DockerCmd(t testingT, command string, args ...string) *icmd.Result {
	return Docker(Cmd(command, args...)).Assert(t, icmd.Success)
}

// BuildCmd executes the specified docker build command and expect a success
func BuildCmd(t testingT, name string, cmdOperators ...CmdOperator) *icmd.Result {
	return Docker(Build(name), cmdOperators...).Assert(t, icmd.Success)
}

// InspectCmd executes the specified docker inspect command and expect a success
func InspectCmd(t testingT, name string, cmdOperators ...CmdOperator) *icmd.Result {
	return Docker(Inspect(name), cmdOperators...).Assert(t, icmd.Success)
}

// Docker executes the specified docker command
func Docker(cmd icmd.Cmd, cmdOperators ...CmdOperator) *icmd.Result {
	for _, op := range cmdOperators {
		deferFn := op(&cmd)
		if deferFn != nil {
			defer deferFn()
		}
	}
	appendDocker(&cmd)
	return icmd.RunCmd(cmd)
}

// Build executes the specified docker build command
func Build(name string) icmd.Cmd {
	return icmd.Command("build", "-t", name)
}

// Inspect executes the specified docker inspect command
func Inspect(name string) icmd.Cmd {
	return icmd.Command("inspect", name)
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

func appendDocker(cmd *icmd.Cmd) {
	cmd.Command = append([]string{testEnv.DockerBinary()}, cmd.Command...)
}

// Cmd build an icmd.Cmd struct from the specified command and arguments
func Cmd(command string, args ...string) icmd.Cmd {
	return icmd.Command(command, args...)
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

// WithConfigFile sets the location of the client config file
func WithConfigFile(dir string) func(*icmd.Cmd) func() {
	return func(cmd *icmd.Cmd) func() {
		cmd.Command = append(
			[]string{"--config", dir},
			cmd.Command...,
		)
		return nil
	}
}
