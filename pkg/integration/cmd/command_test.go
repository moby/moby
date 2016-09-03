package cmd

import (
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/pkg/testutil/assert"
	"github.com/go-check/check"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DockerSuite struct{}

var _ = check.Suite(&DockerSuite{})

func (s *DockerSuite) TestRunCommand(c *check.C) {
	// TODO Windows: Port this test
	if runtime.GOOS == "windows" {
		c.Skip("Needs porting to Windows")
	}

	result := RunCommand("ls")
	result.Assert(c, Expected{})

	result = RunCommand("doesnotexists")
	expectedError := `exec: "doesnotexists": executable file not found`
	result.Assert(c, Expected{ExitCode: 127, Error: expectedError})

	result = RunCommand("ls", "-z")
	result.Assert(c, Expected{
		ExitCode: 2,
		Error:    "exit status 2",
		Err:      "invalid option",
	})
	assert.Contains(c, result.Combined(), "invalid option")
}

func (s *DockerSuite) TestRunCommandWithCombined(c *check.C) {
	// TODO Windows: Port this test
	if runtime.GOOS == "windows" {
		c.Skip("Needs porting to Windows")
	}

	result := RunCommand("ls", "-a")
	result.Assert(c, Expected{})

	assert.Contains(c, result.Combined(), "..")
	assert.Contains(c, result.Stdout(), "..")
}

func (s *DockerSuite) TestRunCommandWithTimeoutFinished(c *check.C) {
	// TODO Windows: Port this test
	if runtime.GOOS == "windows" {
		c.Skip("Needs porting to Windows")
	}

	result := RunCmd(Cmd{
		Command: []string{"ls", "-a"},
		Timeout: 50 * time.Millisecond,
	})
	result.Assert(c, Expected{Out: ".."})
}

func (s *DockerSuite) TestRunCommandWithTimeoutKilled(c *check.C) {
	// TODO Windows: Port this test
	if runtime.GOOS == "windows" {
		c.Skip("Needs porting to Windows")
	}

	command := []string{"sh", "-c", "while true ; do echo 1 ; sleep .1 ; done"}
	result := RunCmd(Cmd{Command: command, Timeout: 500 * time.Millisecond})
	result.Assert(c, Expected{Timeout: true})

	ones := strings.Split(result.Stdout(), "\n")
	assert.Equal(c, len(ones), 6)
}

func (s *DockerSuite) TestRunCommandWithErrors(c *check.C) {
	result := RunCommand("/foobar")
	result.Assert(c, Expected{Error: "foobar", ExitCode: 127})
}

func (s *DockerSuite) TestRunCommandWithStdoutStderr(c *check.C) {
	result := RunCommand("echo", "hello", "world")
	result.Assert(c, Expected{Out: "hello world\n", Err: None})
}

func (s *DockerSuite) TestRunCommandWithStdoutStderrError(c *check.C) {
	result := RunCommand("doesnotexists")

	expected := `exec: "doesnotexists": executable file not found`
	result.Assert(c, Expected{Out: None, Err: None, ExitCode: 127, Error: expected})

	switch runtime.GOOS {
	case "windows":
		expected = "ls: unknown option"
	default:
		expected = "ls: invalid option"
	}

	result = RunCommand("ls", "-z")
	result.Assert(c, Expected{
		Out:      None,
		Err:      expected,
		ExitCode: 2,
		Error:    "exit status 2",
	})
}
