package daemon // import "github.com/docker/docker/integration-cli/daemon"

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/docker/docker/integration-cli/checker"
	"github.com/docker/docker/internal/test/daemon"
	"github.com/go-check/check"
	"github.com/gotestyourself/gotestyourself/assert"
	"github.com/gotestyourself/gotestyourself/icmd"
	"github.com/pkg/errors"
)

type testingT interface {
	assert.TestingT
	logT
	Fatalf(string, ...interface{})
}

type logT interface {
	Logf(string, ...interface{})
}

// Daemon represents a Docker daemon for the testing framework.
type Daemon struct {
	*daemon.Daemon
	dockerBinary string
}

// New returns a Daemon instance to be used for testing.
// This will create a directory such as d123456789 in the folder specified by $DOCKER_INTEGRATION_DAEMON_DEST or $DEST.
// The daemon will not automatically start.
func New(t testingT, dockerBinary string, dockerdBinary string, ops ...func(*daemon.Daemon)) *Daemon {
	ops = append(ops, daemon.WithDockerdBinary(dockerdBinary))
	d := daemon.New(t, ops...)
	return &Daemon{
		Daemon:       d,
		dockerBinary: dockerBinary,
	}
}

// Cmd executes a docker CLI command against this daemon.
// Example: d.Cmd("version") will run docker -H unix://path/to/unix.sock version
func (d *Daemon) Cmd(args ...string) (string, error) {
	result := icmd.RunCmd(d.Command(args...))
	return result.Combined(), result.Error
}

// Command creates a docker CLI command against this daemon, to be executed later.
// Example: d.Command("version") creates a command to run "docker -H unix://path/to/unix.sock version"
func (d *Daemon) Command(args ...string) icmd.Cmd {
	return icmd.Command(d.dockerBinary, d.PrependHostArg(args)...)
}

// PrependHostArg prepend the specified arguments by the daemon host flags
func (d *Daemon) PrependHostArg(args []string) []string {
	for _, arg := range args {
		if arg == "--host" || arg == "-H" {
			return args
		}
	}
	return append([]string{"--host", d.Sock()}, args...)
}

// GetIDByName returns the ID of an object (container, volume, …) given its name
func (d *Daemon) GetIDByName(name string) (string, error) {
	return d.inspectFieldWithError(name, "Id")
}

// ActiveContainers returns the list of ids of the currently running containers
func (d *Daemon) ActiveContainers() (ids []string) {
	// FIXME(vdemeester) shouldn't ignore the error
	out, _ := d.Cmd("ps", "-q")
	for _, id := range strings.Split(out, "\n") {
		if id = strings.TrimSpace(id); id != "" {
			ids = append(ids, id)
		}
	}
	return
}

// InspectField returns the field filter by 'filter'
func (d *Daemon) InspectField(name, filter string) (string, error) {
	return d.inspectFilter(name, filter)
}

func (d *Daemon) inspectFilter(name, filter string) (string, error) {
	format := fmt.Sprintf("{{%s}}", filter)
	out, err := d.Cmd("inspect", "-f", format, name)
	if err != nil {
		return "", errors.Errorf("failed to inspect %s: %s", name, out)
	}
	return strings.TrimSpace(out), nil
}

func (d *Daemon) inspectFieldWithError(name, field string) (string, error) {
	return d.inspectFilter(name, fmt.Sprintf(".%s", field))
}

// FindContainerIP returns the ip of the specified container
func (d *Daemon) FindContainerIP(id string) (string, error) {
	out, err := d.Cmd("inspect", "--format='{{ .NetworkSettings.Networks.bridge.IPAddress }}'", id)
	if err != nil {
		return "", err
	}
	return strings.Trim(out, " \r\n'"), nil
}

// BuildImageWithOut builds an image with the specified dockerfile and options and returns the output
func (d *Daemon) BuildImageWithOut(name, dockerfile string, useCache bool, buildFlags ...string) (string, int, error) {
	buildCmd := BuildImageCmdWithHost(d.dockerBinary, name, dockerfile, d.Sock(), useCache, buildFlags...)
	result := icmd.RunCmd(icmd.Cmd{
		Command: buildCmd.Args,
		Env:     buildCmd.Env,
		Dir:     buildCmd.Dir,
		Stdin:   buildCmd.Stdin,
		Stdout:  buildCmd.Stdout,
	})
	return result.Combined(), result.ExitCode, result.Error
}

// CheckActiveContainerCount returns the number of active containers
// FIXME(vdemeester) should re-use ActivateContainers in some way
func (d *Daemon) CheckActiveContainerCount(c *check.C) (interface{}, check.CommentInterface) {
	out, err := d.Cmd("ps", "-q")
	c.Assert(err, checker.IsNil)
	if len(strings.TrimSpace(out)) == 0 {
		return 0, nil
	}
	return len(strings.Split(strings.TrimSpace(out), "\n")), check.Commentf("output: %q", string(out))
}

// WaitRun waits for a container to be running for 10s
func (d *Daemon) WaitRun(contID string) error {
	args := []string{"--host", d.Sock()}
	return WaitInspectWithArgs(d.dockerBinary, contID, "{{.State.Running}}", "true", 10*time.Second, args...)
}

// CmdRetryOutOfSequence tries the specified command against the current daemon for 10 times
func (d *Daemon) CmdRetryOutOfSequence(args ...string) (string, error) {
	for i := 0; ; i++ {
		out, err := d.Cmd(args...)
		if err != nil {
			if strings.Contains(out, "update out of sequence") {
				if i < 10 {
					continue
				}
			}
		}
		return out, err
	}
}

// WaitInspectWithArgs waits for the specified expression to be equals to the specified expected string in the given time.
// Deprecated: use cli.WaitCmd instead
func WaitInspectWithArgs(dockerBinary, name, expr, expected string, timeout time.Duration, arg ...string) error {
	after := time.After(timeout)

	args := append(arg, "inspect", "-f", expr, name)
	for {
		result := icmd.RunCommand(dockerBinary, args...)
		if result.Error != nil {
			if !strings.Contains(strings.ToLower(result.Stderr()), "no such") {
				return errors.Errorf("error executing docker inspect: %v\n%s",
					result.Stderr(), result.Stdout())
			}
			select {
			case <-after:
				return result.Error
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
			return errors.Errorf("condition \"%q == %q\" not true in time (%v)", out, expected, timeout)
		default:
		}

		time.Sleep(100 * time.Millisecond)
	}
	return nil
}

// BuildImageCmdWithHost create a build command with the specified arguments.
// Deprecated
// FIXME(vdemeester) move this away
func BuildImageCmdWithHost(dockerBinary, name, dockerfile, host string, useCache bool, buildFlags ...string) *exec.Cmd {
	args := []string{}
	if host != "" {
		args = append(args, "--host", host)
	}
	args = append(args, "build", "-t", name)
	if !useCache {
		args = append(args, "--no-cache")
	}
	args = append(args, buildFlags...)
	args = append(args, "-")
	buildCmd := exec.Command(dockerBinary, args...)
	buildCmd.Stdin = strings.NewReader(dockerfile)
	return buildCmd
}
