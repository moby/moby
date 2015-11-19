package integration

import (
	"fmt"
	"os"
	"os/exec"
	"testing"

	"io/ioutil"
	"strings"
	"time"

	"github.com/go-check/check"
)

const dockerBinary = "docker"

// Setup go-check for this test
func Test(t *testing.T) {
	check.TestingT(t)
}

func init() {
	check.Suite(&DockerCmdSuite{})
}

type DockerCmdSuite struct{}

// Fake the exec.Command to use our mock.
func (s *DockerCmdSuite) SetUpTest(c *check.C) {
	execCommand = fakeExecCommand
}

// And bring it back to normal after the test.
func (s *DockerCmdSuite) TearDownTest(c *check.C) {
	execCommand = exec.Command
}

// DockerCmdWithError tests

func (s *DockerCmdSuite) TestDockerCmdWithError(c *check.C) {
	cmds := []struct {
		binary           string
		args             []string
		expectedOut      string
		expectedExitCode int
		expectedError    error
	}{
		{
			"doesnotexists",
			[]string{},
			"Command doesnotexists not found.",
			1,
			fmt.Errorf("exit status 1"),
		},
		{
			dockerBinary,
			[]string{"an", "error"},
			"an error has occurred",
			1,
			fmt.Errorf("exit status 1"),
		},
		{
			dockerBinary,
			[]string{"an", "exitCode", "127"},
			"an error has occurred with exitCode 127",
			127,
			fmt.Errorf("exit status 127"),
		},
		{
			dockerBinary,
			[]string{"run", "-ti", "ubuntu", "echo", "hello"},
			"hello",
			0,
			nil,
		},
	}
	for _, cmd := range cmds {
		out, exitCode, error := DockerCmdWithError(cmd.binary, cmd.args...)
		c.Assert(out, check.Equals, cmd.expectedOut, check.Commentf("Expected output %q for arguments %v, got %q", cmd.expectedOut, cmd.args, out))
		c.Assert(exitCode, check.Equals, cmd.expectedExitCode, check.Commentf("Expected exitCode %q for arguments %v, got %q", cmd.expectedExitCode, cmd.args, exitCode))
		if cmd.expectedError != nil {
			c.Assert(error, check.NotNil, check.Commentf("Expected an error %q, got nothing", cmd.expectedError))
			c.Assert(error.Error(), check.Equals, cmd.expectedError.Error(), check.Commentf("Expected error %q for arguments %v, got %q", cmd.expectedError.Error(), cmd.args, error.Error()))
		} else {
			c.Assert(error, check.IsNil, check.Commentf("Expected no error, got %v", error))
		}
	}
}

// DockerCmdWithStdoutStderr tests

type dockerCmdWithStdoutStderrErrorSuite struct{}

func (s *dockerCmdWithStdoutStderrErrorSuite) Test(c *check.C) {
	// Should fail, the test too
	DockerCmdWithStdoutStderr(dockerBinary, c, "an", "error")
}

type dockerCmdWithStdoutStderrSuccessSuite struct{}

func (s *dockerCmdWithStdoutStderrSuccessSuite) Test(c *check.C) {
	stdout, stderr, exitCode := DockerCmdWithStdoutStderr(dockerBinary, c, "run", "-ti", "ubuntu", "echo", "hello")
	c.Assert(stdout, check.Equals, "hello")
	c.Assert(stderr, check.Equals, "")
	c.Assert(exitCode, check.Equals, 0)

}

func (s *DockerCmdSuite) TestDockerCmdWithStdoutStderrError(c *check.C) {
	// Run error suite, should fail.
	output := String{}
	result := check.Run(&dockerCmdWithStdoutStderrErrorSuite{}, &check.RunConf{Output: &output})
	c.Check(result.Succeeded, check.Equals, 0)
	c.Check(result.Failed, check.Equals, 1)
}

func (s *DockerCmdSuite) TestDockerCmdWithStdoutStderrSuccess(c *check.C) {
	// Run error suite, should fail.
	output := String{}
	result := check.Run(&dockerCmdWithStdoutStderrSuccessSuite{}, &check.RunConf{Output: &output})
	c.Check(result.Succeeded, check.Equals, 1)
	c.Check(result.Failed, check.Equals, 0)
}

// DockerCmd tests

type dockerCmdErrorSuite struct{}

func (s *dockerCmdErrorSuite) Test(c *check.C) {
	// Should fail, the test too
	DockerCmd(dockerBinary, c, "an", "error")
}

type dockerCmdSuccessSuite struct{}

func (s *dockerCmdSuccessSuite) Test(c *check.C) {
	stdout, exitCode := DockerCmd(dockerBinary, c, "run", "-ti", "ubuntu", "echo", "hello")
	c.Assert(stdout, check.Equals, "hello")
	c.Assert(exitCode, check.Equals, 0)

}

func (s *DockerCmdSuite) TestDockerCmdError(c *check.C) {
	// Run error suite, should fail.
	output := String{}
	result := check.Run(&dockerCmdErrorSuite{}, &check.RunConf{Output: &output})
	c.Check(result.Succeeded, check.Equals, 0)
	c.Check(result.Failed, check.Equals, 1)
}

func (s *DockerCmdSuite) TestDockerCmdSuccess(c *check.C) {
	// Run error suite, should fail.
	output := String{}
	result := check.Run(&dockerCmdSuccessSuite{}, &check.RunConf{Output: &output})
	c.Check(result.Succeeded, check.Equals, 1)
	c.Check(result.Failed, check.Equals, 0)
}

// DockerCmdWithTimeout tests

func (s *DockerCmdSuite) TestDockerCmdWithTimeout(c *check.C) {
	cmds := []struct {
		binary           string
		args             []string
		timeout          time.Duration
		expectedOut      string
		expectedExitCode int
		expectedError    error
	}{
		{
			"doesnotexists",
			[]string{},
			200 * time.Millisecond,
			`Command doesnotexists not found.`,
			1,
			fmt.Errorf(`"" failed with errors: exit status 1 : "Command doesnotexists not found."`),
		},
		{
			dockerBinary,
			[]string{"an", "error"},
			200 * time.Millisecond,
			`an error has occurred`,
			1,
			fmt.Errorf(`"an error" failed with errors: exit status 1 : "an error has occurred"`),
		},
		{
			dockerBinary,
			[]string{"a", "command", "that", "times", "out"},
			5 * time.Millisecond,
			"",
			0,
			fmt.Errorf(`"a command that times out" failed with errors: command timed out : ""`),
		},
		{
			dockerBinary,
			[]string{"run", "-ti", "ubuntu", "echo", "hello"},
			200 * time.Millisecond,
			"hello",
			0,
			nil,
		},
	}
	for _, cmd := range cmds {
		out, exitCode, error := DockerCmdWithTimeout(cmd.binary, cmd.timeout, cmd.args...)
		c.Assert(out, check.Equals, cmd.expectedOut, check.Commentf("Expected output %q for arguments %v, got %q", cmd.expectedOut, cmd.args, out))
		c.Assert(exitCode, check.Equals, cmd.expectedExitCode, check.Commentf("Expected exitCode %q for arguments %v, got %q", cmd.expectedExitCode, cmd.args, exitCode))
		if cmd.expectedError != nil {
			c.Assert(error, check.NotNil, check.Commentf("Expected an error %q, got nothing", cmd.expectedError))
			c.Assert(error.Error(), check.Equals, cmd.expectedError.Error(), check.Commentf("Expected error %q for arguments %v, got %q", cmd.expectedError.Error(), cmd.args, error.Error()))
		} else {
			c.Assert(error, check.IsNil, check.Commentf("Expected no error, got %v", error))
		}
	}
}

// DockerCmdInDir tests

func (s *DockerCmdSuite) TestDockerCmdInDir(c *check.C) {
	tempFolder, err := ioutil.TempDir("", "test-docker-cmd-in-dir")
	c.Assert(err, check.IsNil)

	cmds := []struct {
		binary           string
		args             []string
		expectedOut      string
		expectedExitCode int
		expectedError    error
	}{
		{
			"doesnotexists",
			[]string{},
			`Command doesnotexists not found.`,
			1,
			fmt.Errorf(`"dir:%s" failed with errors: exit status 1 : "Command doesnotexists not found."`, tempFolder),
		},
		{
			dockerBinary,
			[]string{"an", "error"},
			`an error has occurred`,
			1,
			fmt.Errorf(`"dir:%s an error" failed with errors: exit status 1 : "an error has occurred"`, tempFolder),
		},
		{
			dockerBinary,
			[]string{"run", "-ti", "ubuntu", "echo", "hello"},
			"hello",
			0,
			nil,
		},
	}
	for _, cmd := range cmds {
		// We prepend the arguments with dir:thefolder.. the fake command will check
		// that the current workdir is the same as the one we are passing.
		args := append([]string{"dir:" + tempFolder}, cmd.args...)
		out, exitCode, error := DockerCmdInDir(cmd.binary, tempFolder, args...)
		c.Assert(out, check.Equals, cmd.expectedOut, check.Commentf("Expected output %q for arguments %v, got %q", cmd.expectedOut, cmd.args, out))
		c.Assert(exitCode, check.Equals, cmd.expectedExitCode, check.Commentf("Expected exitCode %q for arguments %v, got %q", cmd.expectedExitCode, cmd.args, exitCode))
		if cmd.expectedError != nil {
			c.Assert(error, check.NotNil, check.Commentf("Expected an error %q, got nothing", cmd.expectedError))
			c.Assert(error.Error(), check.Equals, cmd.expectedError.Error(), check.Commentf("Expected error %q for arguments %v, got %q", cmd.expectedError.Error(), cmd.args, error.Error()))
		} else {
			c.Assert(error, check.IsNil, check.Commentf("Expected no error, got %v", error))
		}
	}
}

// DockerCmdInDirWithTimeout tests

func (s *DockerCmdSuite) TestDockerCmdInDirWithTimeout(c *check.C) {
	tempFolder, err := ioutil.TempDir("", "test-docker-cmd-in-dir")
	c.Assert(err, check.IsNil)

	cmds := []struct {
		binary           string
		args             []string
		timeout          time.Duration
		expectedOut      string
		expectedExitCode int
		expectedError    error
	}{
		{
			"doesnotexists",
			[]string{},
			200 * time.Millisecond,
			`Command doesnotexists not found.`,
			1,
			fmt.Errorf(`"dir:%s" failed with errors: exit status 1 : "Command doesnotexists not found."`, tempFolder),
		},
		{
			dockerBinary,
			[]string{"an", "error"},
			200 * time.Millisecond,
			`an error has occurred`,
			1,
			fmt.Errorf(`"dir:%s an error" failed with errors: exit status 1 : "an error has occurred"`, tempFolder),
		},
		{
			dockerBinary,
			[]string{"a", "command", "that", "times", "out"},
			5 * time.Millisecond,
			"",
			0,
			fmt.Errorf(`"dir:%s a command that times out" failed with errors: command timed out : ""`, tempFolder),
		},
		{
			dockerBinary,
			[]string{"run", "-ti", "ubuntu", "echo", "hello"},
			200 * time.Millisecond,
			"hello",
			0,
			nil,
		},
	}
	for _, cmd := range cmds {
		// We prepend the arguments with dir:thefolder.. the fake command will check
		// that the current workdir is the same as the one we are passing.
		args := append([]string{"dir:" + tempFolder}, cmd.args...)
		out, exitCode, error := DockerCmdInDirWithTimeout(cmd.binary, cmd.timeout, tempFolder, args...)
		c.Assert(out, check.Equals, cmd.expectedOut, check.Commentf("Expected output %q for arguments %v, got %q", cmd.expectedOut, cmd.args, out))
		c.Assert(exitCode, check.Equals, cmd.expectedExitCode, check.Commentf("Expected exitCode %q for arguments %v, got %q", cmd.expectedExitCode, cmd.args, exitCode))
		if cmd.expectedError != nil {
			c.Assert(error, check.NotNil, check.Commentf("Expected an error %q, got nothing", cmd.expectedError))
			c.Assert(error.Error(), check.Equals, cmd.expectedError.Error(), check.Commentf("Expected error %q for arguments %v, got %q", cmd.expectedError.Error(), cmd.args, error.Error()))
		} else {
			c.Assert(error, check.IsNil, check.Commentf("Expected no error, got %v", error))
		}
	}
}

// Helpers :)

// Type implementing the io.Writer interface for analyzing output.
type String struct {
	value string
}

// The only function required by the io.Writer interface.  Will append
// written data to the String.value string.
func (s *String) Write(p []byte) (n int, err error) {
	s.value += string(p)
	return len(p), nil
}

// Helper function that mock the exec.Command call (and call the test binary)
func fakeExecCommand(command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestHelperProcess", "--", command}
	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
	return cmd
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	args := os.Args

	// Previous arguments are tests stuff, that looks like :
	// /tmp/go-build970079519/…/_test/integration.test -test.run=TestHelperProcess --
	cmd, args := args[3], args[4:]
	// Handle the case where args[0] is dir:...
	if len(args) > 0 && strings.HasPrefix(args[0], "dir:") {
		expectedCwd := args[0][4:]
		if len(args) > 1 {
			args = args[1:]
		}
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to get workingdir: %v", err)
			os.Exit(1)
		}
		// This checks that the given path is the same as the currend working dire
		if expectedCwd != cwd {
			fmt.Fprintf(os.Stderr, "Current workdir should be %q, but is %q", expectedCwd, cwd)
		}
	}
	switch cmd {
	case dockerBinary:
		argsStr := strings.Join(args, " ")
		switch argsStr {
		case "an exitCode 127":
			fmt.Fprintf(os.Stderr, "an error has occurred with exitCode 127")
			os.Exit(127)
		case "an error":
			fmt.Fprintf(os.Stderr, "an error has occurred")
			os.Exit(1)
		case "a command that times out":
			time.Sleep(10 * time.Millisecond)
			fmt.Fprintf(os.Stdout, "too long, should be killed")
			// A random exit code (that should never happened in tests)
			os.Exit(7)
		case "run -ti ubuntu echo hello":
			fmt.Fprintf(os.Stdout, "hello")
		default:
			fmt.Fprintf(os.Stdout, "no arguments")
		}
	default:
		fmt.Fprintf(os.Stderr, "Command %s not found.", cmd)
		os.Exit(1)
	}
	// some code here to check arguments perhaps?
	os.Exit(0)
}
