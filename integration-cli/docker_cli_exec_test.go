package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/docker/docker/client"
	"github.com/docker/docker/integration-cli/cli"
	"github.com/docker/docker/integration-cli/cli/build"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/icmd"
)

type DockerCLIExecSuite struct {
	ds *DockerSuite
}

func (s *DockerCLIExecSuite) TearDownTest(c *testing.T) {
	s.ds.TearDownTest(c)
}

func (s *DockerCLIExecSuite) OnTimeout(c *testing.T) {
	s.ds.OnTimeout(c)
}

func (s *DockerCLIExecSuite) TestExec(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "-d", "--name", "testing", "busybox", "sh", "-c", "echo test > /tmp/file && top")
	assert.NilError(c, waitRun(strings.TrimSpace(out)))

	out, _ = dockerCmd(c, "exec", "testing", "cat", "/tmp/file")
	assert.Equal(c, strings.Trim(out, "\r\n"), "test")
}

func (s *DockerCLIExecSuite) TestExecInteractive(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	dockerCmd(c, "run", "-d", "--name", "testing", "busybox", "sh", "-c", "echo test > /tmp/file && top")

	execCmd := exec.Command(dockerBinary, "exec", "-i", "testing", "sh")
	stdin, err := execCmd.StdinPipe()
	assert.NilError(c, err)
	stdout, err := execCmd.StdoutPipe()
	assert.NilError(c, err)

	err = execCmd.Start()
	assert.NilError(c, err)
	_, err = stdin.Write([]byte("cat /tmp/file\n"))
	assert.NilError(c, err)

	r := bufio.NewReader(stdout)
	line, err := r.ReadString('\n')
	assert.NilError(c, err)
	line = strings.TrimSpace(line)
	assert.Equal(c, line, "test")
	err = stdin.Close()
	assert.NilError(c, err)
	errChan := make(chan error, 1)
	go func() {
		errChan <- execCmd.Wait()
		close(errChan)
	}()
	select {
	case err := <-errChan:
		assert.NilError(c, err)
	case <-time.After(1 * time.Second):
		c.Fatal("docker exec failed to exit on stdin close")
	}
}

func (s *DockerCLIExecSuite) TestExecAfterContainerRestart(c *testing.T) {
	out := runSleepingContainer(c)
	cleanedContainerID := strings.TrimSpace(out)
	assert.NilError(c, waitRun(cleanedContainerID))
	dockerCmd(c, "restart", cleanedContainerID)
	assert.NilError(c, waitRun(cleanedContainerID))

	out, _ = dockerCmd(c, "exec", cleanedContainerID, "echo", "hello")
	assert.Equal(c, strings.TrimSpace(out), "hello")
}

func (s *DockerDaemonSuite) TestExecAfterDaemonRestart(c *testing.T) {
	// TODO Windows CI: DockerDaemonSuite doesn't run on Windows, and requires a little work to get this ported.
	s.d.StartWithBusybox(c)

	out, err := s.d.Cmd("run", "-d", "--name", "top", "-p", "80", "busybox:latest", "top")
	assert.NilError(c, err, "Could not run top: %s", out)

	s.d.Restart(c)

	out, err = s.d.Cmd("start", "top")
	assert.NilError(c, err, "Could not start top after daemon restart: %s", out)

	out, err = s.d.Cmd("exec", "top", "echo", "hello")
	assert.NilError(c, err, "Could not exec on container top: %s", out)
	assert.Equal(c, strings.TrimSpace(out), "hello")
}

// Regression test for #9155, #9044
func (s *DockerCLIExecSuite) TestExecEnv(c *testing.T) {
	// TODO Windows CI: This one is interesting and may just end up being a feature
	// difference between Windows and Linux. On Windows, the environment is passed
	// into the process that is launched, not into the machine environment. Hence
	// a subsequent exec will not have LALA set/
	testRequires(c, DaemonIsLinux)
	runSleepingContainer(c, "-e", "LALA=value1", "-e", "LALA=value2", "-d", "--name", "testing")
	assert.NilError(c, waitRun("testing"))

	out, _ := dockerCmd(c, "exec", "testing", "env")
	assert.Check(c, !strings.Contains(out, "LALA=value1"))
	assert.Check(c, strings.Contains(out, "LALA=value2"))
	assert.Check(c, strings.Contains(out, "HOME=/root"))
}

func (s *DockerCLIExecSuite) TestExecSetEnv(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	runSleepingContainer(c, "-e", "HOME=/root", "-d", "--name", "testing")
	assert.NilError(c, waitRun("testing"))

	out, _ := dockerCmd(c, "exec", "-e", "HOME=/another", "-e", "ABC=xyz", "testing", "env")
	assert.Check(c, !strings.Contains(out, "HOME=/root"))
	assert.Check(c, strings.Contains(out, "HOME=/another"))
	assert.Check(c, strings.Contains(out, "ABC=xyz"))
}

func (s *DockerCLIExecSuite) TestExecExitStatus(c *testing.T) {
	runSleepingContainer(c, "-d", "--name", "top")

	result := icmd.RunCommand(dockerBinary, "exec", "top", "sh", "-c", "exit 23")
	result.Assert(c, icmd.Expected{ExitCode: 23, Error: "exit status 23"})
}

func (s *DockerCLIExecSuite) TestExecPausedContainer(c *testing.T) {
	testRequires(c, IsPausable)

	out := runSleepingContainer(c, "-d", "--name", "testing")
	ContainerID := strings.TrimSpace(out)

	dockerCmd(c, "pause", "testing")
	out, _, err := dockerCmdWithError("exec", ContainerID, "echo", "hello")
	assert.ErrorContains(c, err, "", "container should fail to exec new command if it is paused")

	expected := ContainerID + " is paused, unpause the container before exec"
	assert.Assert(c, is.Contains(out, expected), "container should not exec new command if it is paused")
}

// regression test for #9476
func (s *DockerCLIExecSuite) TestExecTTYCloseStdin(c *testing.T) {
	// TODO Windows CI: This requires some work to port to Windows.
	testRequires(c, DaemonIsLinux)
	dockerCmd(c, "run", "-d", "-it", "--name", "exec_tty_stdin", "busybox")

	cmd := exec.Command(dockerBinary, "exec", "-i", "exec_tty_stdin", "cat")
	stdinRw, err := cmd.StdinPipe()
	assert.NilError(c, err)

	stdinRw.Write([]byte("test"))
	stdinRw.Close()

	out, _, err := runCommandWithOutput(cmd)
	assert.NilError(c, err, out)

	out, _ = dockerCmd(c, "top", "exec_tty_stdin")
	outArr := strings.Split(out, "\n")
	assert.Assert(c, len(outArr) <= 3, "exec process left running")
	assert.Assert(c, !strings.Contains(out, "nsenter-exec"))
}

func (s *DockerCLIExecSuite) TestExecTTYWithoutStdin(c *testing.T) {
	out, _ := dockerCmd(c, "run", "-d", "-ti", "busybox")
	id := strings.TrimSpace(out)
	assert.NilError(c, waitRun(id))

	errChan := make(chan error, 1)
	go func() {
		defer close(errChan)

		cmd := exec.Command(dockerBinary, "exec", "-ti", id, "true")
		if _, err := cmd.StdinPipe(); err != nil {
			errChan <- err
			return
		}

		expected := "the input device is not a TTY"
		if runtime.GOOS == "windows" {
			expected += ".  If you are using mintty, try prefixing the command with 'winpty'"
		}
		if out, _, err := runCommandWithOutput(cmd); err == nil {
			errChan <- fmt.Errorf("exec should have failed")
			return
		} else if !strings.Contains(out, expected) {
			errChan <- fmt.Errorf("exec failed with error %q: expected %q", out, expected)
			return
		}
	}()

	select {
	case err := <-errChan:
		assert.NilError(c, err)
	case <-time.After(3 * time.Second):
		c.Fatal("exec is running but should have failed")
	}
}

// FIXME(vdemeester) this should be a unit tests on cli/command/container package
func (s *DockerCLIExecSuite) TestExecParseError(c *testing.T) {
	// TODO Windows CI: Requires some extra work. Consider copying the
	// runSleepingContainer helper to have an exec version.
	testRequires(c, DaemonIsLinux)
	dockerCmd(c, "run", "-d", "--name", "top", "busybox", "top")

	// Test normal (non-detached) case first
	icmd.RunCommand(dockerBinary, "exec", "top").Assert(c, icmd.Expected{
		ExitCode: 1,
		Error:    "exit status 1",
		Err:      "See 'docker exec --help'",
	})
}

func (s *DockerCLIExecSuite) TestExecStopNotHanging(c *testing.T) {
	// TODO Windows CI: Requires some extra work. Consider copying the
	// runSleepingContainer helper to have an exec version.
	testRequires(c, DaemonIsLinux)
	dockerCmd(c, "run", "-d", "--name", "testing", "busybox", "top")

	result := icmd.StartCmd(icmd.Command(dockerBinary, "exec", "testing", "top"))
	result.Assert(c, icmd.Success)
	go icmd.WaitOnCmd(0, result)

	type dstop struct {
		out string
		err error
	}
	ch := make(chan dstop, 1)
	go func() {
		result := icmd.RunCommand(dockerBinary, "stop", "testing")
		ch <- dstop{result.Combined(), result.Error}
		close(ch)
	}()
	select {
	case <-time.After(3 * time.Second):
		c.Fatal("Container stop timed out")
	case s := <-ch:
		assert.NilError(c, s.err)
	}
}

func (s *DockerCLIExecSuite) TestExecCgroup(c *testing.T) {
	// Not applicable on Windows - using Linux specific functionality
	testRequires(c, NotUserNamespace)
	testRequires(c, DaemonIsLinux)
	dockerCmd(c, "run", "-d", "--name", "testing", "busybox", "top")

	out, _ := dockerCmd(c, "exec", "testing", "cat", "/proc/1/cgroup")
	containerCgroups := sort.StringSlice(strings.Split(out, "\n"))

	var wg sync.WaitGroup
	var mu sync.Mutex
	var execCgroups []sort.StringSlice
	errChan := make(chan error, 5)
	// exec a few times concurrently to get consistent failure
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			out, _, err := dockerCmdWithError("exec", "testing", "cat", "/proc/self/cgroup")
			if err != nil {
				errChan <- err
				return
			}
			cg := sort.StringSlice(strings.Split(out, "\n"))

			mu.Lock()
			execCgroups = append(execCgroups, cg)
			mu.Unlock()
		}()
	}
	wg.Wait()
	close(errChan)

	for err := range errChan {
		assert.NilError(c, err)
	}

	for _, cg := range execCgroups {
		if !reflect.DeepEqual(cg, containerCgroups) {
			fmt.Println("exec cgroups:")
			for _, name := range cg {
				fmt.Printf(" %s\n", name)
			}

			fmt.Println("container cgroups:")
			for _, name := range containerCgroups {
				fmt.Printf(" %s\n", name)
			}
			c.Fatal("cgroups mismatched")
		}
	}
}

func (s *DockerCLIExecSuite) TestExecInspectID(c *testing.T) {
	out := runSleepingContainer(c, "-d")
	id := strings.TrimSuffix(out, "\n")

	out = inspectField(c, id, "ExecIDs")
	assert.Equal(c, out, "[]", "ExecIDs should be empty, got: %s", out)

	// Start an exec, have it block waiting so we can do some checking
	cmd := exec.Command(dockerBinary, "exec", id, "sh", "-c",
		"while ! test -e /execid1; do sleep 1; done")

	err := cmd.Start()
	assert.NilError(c, err, "failed to start the exec cmd")

	// Give the exec 10 chances/seconds to start then give up and stop the test
	tries := 10
	for i := 0; i < tries; i++ {
		// Since its still running we should see exec as part of the container
		out = strings.TrimSpace(inspectField(c, id, "ExecIDs"))

		if out != "[]" && out != "<no value>" {
			break
		}
		assert.Check(c, i+1 != tries, "ExecIDs still empty after 10 second")
		time.Sleep(1 * time.Second)
	}

	// Save execID for later
	execID, err := inspectFilter(id, "index .ExecIDs 0")
	assert.NilError(c, err, "failed to get the exec id")

	// End the exec by creating the missing file
	err = exec.Command(dockerBinary, "exec", id, "sh", "-c", "touch /execid1").Run()
	assert.NilError(c, err, "failed to run the 2nd exec cmd")

	// Wait for 1st exec to complete
	cmd.Wait()

	// Give the exec 10 chances/seconds to stop then give up and stop the test
	for i := 0; i < tries; i++ {
		// Since its still running we should see exec as part of the container
		out = strings.TrimSpace(inspectField(c, id, "ExecIDs"))

		if out == "[]" {
			break
		}
		assert.Check(c, i+1 != tries, "ExecIDs still empty after 10 second")
		time.Sleep(1 * time.Second)
	}

	// But we should still be able to query the execID
	cli, err := client.NewClientWithOpts(client.FromEnv)
	assert.NilError(c, err)
	defer cli.Close()

	_, err = cli.ContainerExecInspect(context.Background(), execID)
	assert.NilError(c, err)

	// Now delete the container and then an 'inspect' on the exec should
	// result in a 404 (not 'container not running')
	out, ec := dockerCmd(c, "rm", "-f", id)
	assert.Equal(c, ec, 0, "error removing container: %s", out)

	_, err = cli.ContainerExecInspect(context.Background(), execID)
	assert.ErrorContains(c, err, "No such exec instance")
}

func (s *DockerCLIExecSuite) TestLinksPingLinkedContainersOnRename(c *testing.T) {
	// Problematic on Windows as Windows does not support links
	testRequires(c, DaemonIsLinux)
	var out string
	out, _ = dockerCmd(c, "run", "-d", "--name", "container1", "busybox", "top")
	idA := strings.TrimSpace(out)
	assert.Assert(c, idA != "", "%s, id should not be nil", out)
	out, _ = dockerCmd(c, "run", "-d", "--link", "container1:alias1", "--name", "container2", "busybox", "top")
	idB := strings.TrimSpace(out)
	assert.Assert(c, idB != "", "%s, id should not be nil", out)

	dockerCmd(c, "exec", "container2", "ping", "-c", "1", "alias1", "-W", "1")
	dockerCmd(c, "rename", "container1", "container_new")
	dockerCmd(c, "exec", "container2", "ping", "-c", "1", "alias1", "-W", "1")
}

func (s *DockerCLIExecSuite) TestRunMutableNetworkFiles(c *testing.T) {
	// Not applicable on Windows to Windows CI.
	testRequires(c, testEnv.IsLocalDaemon, DaemonIsLinux)
	for _, fn := range []string{"resolv.conf", "hosts"} {
		containers := cli.DockerCmd(c, "ps", "-q", "-a").Combined()
		if containers != "" {
			cli.DockerCmd(c, append([]string{"rm", "-fv"}, strings.Split(strings.TrimSpace(containers), "\n")...)...)
		}

		content := runCommandAndReadContainerFile(c, fn, dockerBinary, "run", "-d", "--name", "c1", "busybox", "sh", "-c", fmt.Sprintf("echo success >/etc/%s && top", fn))

		assert.Equal(c, strings.TrimSpace(string(content)), "success", "Content was not what was modified in the container", string(content))

		out, _ := dockerCmd(c, "run", "-d", "--name", "c2", "busybox", "top")
		contID := strings.TrimSpace(out)
		netFilePath := containerStorageFile(contID, fn)

		f, err := os.OpenFile(netFilePath, os.O_WRONLY|os.O_SYNC|os.O_APPEND, 0644)
		assert.NilError(c, err)

		if _, err := f.Seek(0, 0); err != nil {
			f.Close()
			c.Fatal(err)
		}

		if err := f.Truncate(0); err != nil {
			f.Close()
			c.Fatal(err)
		}

		if _, err := f.Write([]byte("success2\n")); err != nil {
			f.Close()
			c.Fatal(err)
		}
		f.Close()

		res, _ := dockerCmd(c, "exec", contID, "cat", "/etc/"+fn)
		assert.Equal(c, res, "success2\n")
	}
}

func (s *DockerCLIExecSuite) TestExecWithUser(c *testing.T) {
	// TODO Windows CI: This may be fixable in the future once Windows
	// supports users
	testRequires(c, DaemonIsLinux)
	dockerCmd(c, "run", "-d", "--name", "parent", "busybox", "top")

	out, _ := dockerCmd(c, "exec", "-u", "1", "parent", "id")
	assert.Assert(c, strings.Contains(out, "uid=1(daemon) gid=1(daemon)"))

	out, _ = dockerCmd(c, "exec", "-u", "root", "parent", "id")
	assert.Assert(c, strings.Contains(out, "uid=0(root) gid=0(root)"), "exec with user by id expected daemon user got %s", out)
}

func (s *DockerCLIExecSuite) TestExecWithPrivileged(c *testing.T) {
	// Not applicable on Windows
	testRequires(c, DaemonIsLinux, NotUserNamespace)
	// Start main loop which attempts mknod repeatedly
	dockerCmd(c, "run", "-d", "--name", "parent", "--cap-drop=ALL", "busybox", "sh", "-c", `while (true); do if [ -e /exec_priv ]; then cat /exec_priv && mknod /tmp/sda b 8 0 && echo "Success"; else echo "Privileged exec has not run yet"; fi; usleep 10000; done`)

	// Check exec mknod doesn't work
	icmd.RunCommand(dockerBinary, "exec", "parent", "sh", "-c", "mknod /tmp/sdb b 8 16").Assert(c, icmd.Expected{
		ExitCode: 1,
		Err:      "Operation not permitted",
	})

	// Check exec mknod does work with --privileged
	result := icmd.RunCommand(dockerBinary, "exec", "--privileged", "parent", "sh", "-c", `echo "Running exec --privileged" > /exec_priv && mknod /tmp/sdb b 8 16 && usleep 50000 && echo "Finished exec --privileged" > /exec_priv && echo ok`)
	result.Assert(c, icmd.Success)

	actual := strings.TrimSpace(result.Combined())
	assert.Equal(c, actual, "ok", "exec mknod in --cap-drop=ALL container with --privileged failed, output: %q", result.Combined())

	// Check subsequent unprivileged exec cannot mknod
	icmd.RunCommand(dockerBinary, "exec", "parent", "sh", "-c", "mknod /tmp/sdc b 8 32").Assert(c, icmd.Expected{
		ExitCode: 1,
		Err:      "Operation not permitted",
	})
	// Confirm at no point was mknod allowed
	result = icmd.RunCommand(dockerBinary, "logs", "parent")
	result.Assert(c, icmd.Success)
	assert.Assert(c, !strings.Contains(result.Combined(), "Success"))
}

func (s *DockerCLIExecSuite) TestExecWithImageUser(c *testing.T) {
	// Not applicable on Windows
	testRequires(c, DaemonIsLinux)
	name := "testbuilduser"
	buildImageSuccessfully(c, name, build.WithDockerfile(`FROM busybox
		RUN echo 'dockerio:x:1001:1001::/bin:/bin/false' >> /etc/passwd
		USER dockerio`))
	dockerCmd(c, "run", "-d", "--name", "dockerioexec", name, "top")

	out, _ := dockerCmd(c, "exec", "dockerioexec", "whoami")
	assert.Assert(c, strings.Contains(out, "dockerio"), "exec with user by id expected dockerio user got %s", out)
}

func (s *DockerCLIExecSuite) TestExecOnReadonlyContainer(c *testing.T) {
	// Windows does not support read-only
	// --read-only + userns has remount issues
	testRequires(c, DaemonIsLinux, NotUserNamespace)
	dockerCmd(c, "run", "-d", "--read-only", "--name", "parent", "busybox", "top")
	dockerCmd(c, "exec", "parent", "true")
}

func (s *DockerCLIExecSuite) TestExecUlimits(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	name := "testexeculimits"
	runSleepingContainer(c, "-d", "--ulimit", "nofile=511:511", "--name", name)
	assert.NilError(c, waitRun(name))

	out, _, err := dockerCmdWithError("exec", name, "sh", "-c", "ulimit -n")
	assert.NilError(c, err)
	assert.Equal(c, strings.TrimSpace(out), "511")
}

// #15750
func (s *DockerCLIExecSuite) TestExecStartFails(c *testing.T) {
	name := "exec-15750"
	runSleepingContainer(c, "-d", "--name", name)
	assert.NilError(c, waitRun(name))

	out, _, err := dockerCmdWithError("exec", name, "no-such-cmd")
	assert.ErrorContains(c, err, "", out)

	expectedMsg := "executable file not found"
	if DaemonIsWindows() {
		expectedMsg = "The system cannot find the file specified"
	}
	assert.Assert(c, is.Contains(out, expectedMsg))
}

// Fix regression in https://github.com/docker/docker/pull/26461#issuecomment-250287297
func (s *DockerCLIExecSuite) TestExecWindowsPathNotWiped(c *testing.T) {
	testRequires(c, DaemonIsWindows)
	out, _ := dockerCmd(c, "run", "-d", "--name", "testing", minimalBaseImage(), "powershell", "start-sleep", "60")
	assert.NilError(c, waitRun(strings.TrimSpace(out)))

	out, _ = dockerCmd(c, "exec", "testing", "powershell", "write-host", "$env:PATH")
	out = strings.ToLower(strings.Trim(out, "\r\n"))
	assert.Assert(c, strings.Contains(out, `windowspowershell\v1.0`))
}

func (s *DockerCLIExecSuite) TestExecEnvLinksHost(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	runSleepingContainer(c, "-d", "--name", "foo")
	runSleepingContainer(c, "-d", "--link", "foo:db", "--hostname", "myhost", "--name", "bar")
	out, _ := dockerCmd(c, "exec", "bar", "env")
	assert.Check(c, is.Contains(out, "HOSTNAME=myhost"))
	assert.Check(c, is.Contains(out, "DB_NAME=/bar/db"))
}
