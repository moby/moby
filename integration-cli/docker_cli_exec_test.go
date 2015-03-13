// +build !test_no_exec

package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestExec(t *testing.T) {
	defer deleteAllContainers()

	runCmd := exec.Command(dockerBinary, "run", "-d", "--name", "testing", "busybox", "sh", "-c", "echo test > /tmp/file && sleep 100")
	if out, _, _, err := runCommandWithStdoutStderr(runCmd); err != nil {
		t.Fatal(out, err)
	}

	execCmd := exec.Command(dockerBinary, "exec", "testing", "cat", "/tmp/file")
	out, _, err := runCommandWithOutput(execCmd)
	if err != nil {
		t.Fatal(out, err)
	}

	out = strings.Trim(out, "\r\n")

	if expected := "test"; out != expected {
		t.Errorf("container exec should've printed %q but printed %q", expected, out)
	}

	logDone("exec - basic test")
}

func TestExecInteractiveStdinClose(t *testing.T) {
	defer deleteAllContainers()
	out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "run", "-itd", "busybox", "/bin/cat"))
	if err != nil {
		t.Fatal(err)
	}

	contId := strings.TrimSpace(out)

	returnchan := make(chan struct{})

	go func() {
		var err error
		cmd := exec.Command(dockerBinary, "exec", "-i", contId, "/bin/ls", "/")
		cmd.Stdin = os.Stdin
		if err != nil {
			t.Fatal(err)
		}

		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatal(err, string(out))
		}

		if string(out) == "" {
			t.Fatalf("Output was empty, likely blocked by standard input")
		}

		returnchan <- struct{}{}
	}()

	select {
	case <-returnchan:
	case <-time.After(10 * time.Second):
		t.Fatal("timed out running docker exec")
	}

	logDone("exec - interactive mode closes stdin after execution")
}

func TestExecInteractive(t *testing.T) {
	defer deleteAllContainers()

	runCmd := exec.Command(dockerBinary, "run", "-d", "--name", "testing", "busybox", "sh", "-c", "echo test > /tmp/file && sleep 100")
	if out, _, _, err := runCommandWithStdoutStderr(runCmd); err != nil {
		t.Fatal(out, err)
	}

	execCmd := exec.Command(dockerBinary, "exec", "-i", "testing", "sh")
	stdin, err := execCmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := execCmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}

	if err := execCmd.Start(); err != nil {
		t.Fatal(err)
	}
	if _, err := stdin.Write([]byte("cat /tmp/file\n")); err != nil {
		t.Fatal(err)
	}

	r := bufio.NewReader(stdout)
	line, err := r.ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	line = strings.TrimSpace(line)
	if line != "test" {
		t.Fatalf("Output should be 'test', got '%q'", line)
	}
	if err := stdin.Close(); err != nil {
		t.Fatal(err)
	}
	finish := make(chan struct{})
	go func() {
		if err := execCmd.Wait(); err != nil {
			t.Fatal(err)
		}
		close(finish)
	}()
	select {
	case <-finish:
	case <-time.After(1 * time.Second):
		t.Fatal("docker exec failed to exit on stdin close")
	}

	logDone("exec - Interactive test")
}

func TestExecAfterContainerRestart(t *testing.T) {
	defer deleteAllContainers()

	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "top")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}

	cleanedContainerID := stripTrailingCharacters(out)

	runCmd = exec.Command(dockerBinary, "restart", cleanedContainerID)
	if out, _, err = runCommandWithOutput(runCmd); err != nil {
		t.Fatal(out, err)
	}

	runCmd = exec.Command(dockerBinary, "exec", cleanedContainerID, "echo", "hello")
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}

	outStr := strings.TrimSpace(out)
	if outStr != "hello" {
		t.Errorf("container should've printed hello, instead printed %q", outStr)
	}

	logDone("exec - exec running container after container restart")
}

func TestExecAfterDaemonRestart(t *testing.T) {
	testRequires(t, SameHostDaemon)
	defer deleteAllContainers()

	d := NewDaemon(t)
	if err := d.StartWithBusybox(); err != nil {
		t.Fatalf("Could not start daemon with busybox: %v", err)
	}
	defer d.Stop()

	if out, err := d.Cmd("run", "-d", "--name", "top", "-p", "80", "busybox:latest", "top"); err != nil {
		t.Fatalf("Could not run top: err=%v\n%s", err, out)
	}

	if err := d.Restart(); err != nil {
		t.Fatalf("Could not restart daemon: %v", err)
	}

	if out, err := d.Cmd("start", "top"); err != nil {
		t.Fatalf("Could not start top after daemon restart: err=%v\n%s", err, out)
	}

	out, err := d.Cmd("exec", "top", "echo", "hello")
	if err != nil {
		t.Fatalf("Could not exec on container top: err=%v\n%s", err, out)
	}

	outStr := strings.TrimSpace(string(out))
	if outStr != "hello" {
		t.Errorf("container should've printed hello, instead printed %q", outStr)
	}

	logDone("exec - exec running container after daemon restart")
}

// Regression test for #9155, #9044
func TestExecEnv(t *testing.T) {
	defer deleteAllContainers()

	runCmd := exec.Command(dockerBinary, "run",
		"-e", "LALA=value1",
		"-e", "LALA=value2",
		"-d", "--name", "testing", "busybox", "top")
	if out, _, _, err := runCommandWithStdoutStderr(runCmd); err != nil {
		t.Fatal(out, err)
	}

	execCmd := exec.Command(dockerBinary, "exec", "testing", "env")
	out, _, err := runCommandWithOutput(execCmd)
	if err != nil {
		t.Fatal(out, err)
	}

	if strings.Contains(out, "LALA=value1") ||
		!strings.Contains(out, "LALA=value2") ||
		!strings.Contains(out, "HOME=/root") {
		t.Errorf("exec env(%q), expect %q, %q", out, "LALA=value2", "HOME=/root")
	}

	logDone("exec - exec inherits correct env")
}

func TestExecExitStatus(t *testing.T) {
	defer deleteAllContainers()

	runCmd := exec.Command(dockerBinary, "run", "-d", "--name", "top", "busybox", "top")
	if out, _, _, err := runCommandWithStdoutStderr(runCmd); err != nil {
		t.Fatal(out, err)
	}

	// Test normal (non-detached) case first
	cmd := exec.Command(dockerBinary, "exec", "top", "sh", "-c", "exit 23")
	ec, _ := runCommand(cmd)

	if ec != 23 {
		t.Fatalf("Should have had an ExitCode of 23, not: %d", ec)
	}

	logDone("exec - exec non-zero ExitStatus")
}

func TestExecPausedContainer(t *testing.T) {
	defer deleteAllContainers()
	defer unpauseAllContainers()

	runCmd := exec.Command(dockerBinary, "run", "-d", "--name", "testing", "busybox", "top")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}

	ContainerID := stripTrailingCharacters(out)

	pausedCmd := exec.Command(dockerBinary, "pause", "testing")
	out, _, _, err = runCommandWithStdoutStderr(pausedCmd)
	if err != nil {
		t.Fatal(out, err)
	}

	execCmd := exec.Command(dockerBinary, "exec", "-i", "-t", ContainerID, "echo", "hello")
	out, _, err = runCommandWithOutput(execCmd)
	if err == nil {
		t.Fatal("container should fail to exec new command if it is paused")
	}

	expected := ContainerID + " is paused, unpause the container before exec"
	if !strings.Contains(out, expected) {
		t.Fatal("container should not exec new command if it is paused")
	}

	logDone("exec - exec should not exec a pause container")
}

// regression test for #9476
func TestExecTtyCloseStdin(t *testing.T) {
	defer deleteAllContainers()

	cmd := exec.Command(dockerBinary, "run", "-d", "-it", "--name", "exec_tty_stdin", "busybox")
	if out, _, err := runCommandWithOutput(cmd); err != nil {
		t.Fatal(out, err)
	}

	cmd = exec.Command(dockerBinary, "exec", "-i", "exec_tty_stdin", "cat")
	stdinRw, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}

	stdinRw.Write([]byte("test"))
	stdinRw.Close()

	if out, _, err := runCommandWithOutput(cmd); err != nil {
		t.Fatal(out, err)
	}

	cmd = exec.Command(dockerBinary, "top", "exec_tty_stdin")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(out, err)
	}

	outArr := strings.Split(out, "\n")
	if len(outArr) > 3 || strings.Contains(out, "nsenter-exec") {
		// This is the really bad part
		if out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "rm", "-f", "exec_tty_stdin")); err != nil {
			t.Fatal(out, err)
		}

		t.Fatalf("exec process left running\n\t %s", out)
	}

	logDone("exec - stdin is closed properly with tty enabled")
}

func TestExecTtyWithoutStdin(t *testing.T) {
	defer deleteAllContainers()

	cmd := exec.Command(dockerBinary, "run", "-d", "-ti", "busybox")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatalf("failed to start container: %v (%v)", out, err)
	}

	id := strings.TrimSpace(out)
	if err := waitRun(id); err != nil {
		t.Fatal(err)
	}

	defer func() {
		cmd := exec.Command(dockerBinary, "kill", id)
		if out, _, err := runCommandWithOutput(cmd); err != nil {
			t.Fatalf("failed to kill container: %v (%v)", out, err)
		}
	}()

	done := make(chan struct{})
	go func() {
		defer close(done)

		cmd := exec.Command(dockerBinary, "exec", "-ti", id, "true")
		if _, err := cmd.StdinPipe(); err != nil {
			t.Fatal(err)
		}

		expected := "cannot enable tty mode"
		if out, _, err := runCommandWithOutput(cmd); err == nil {
			t.Fatal("exec should have failed")
		} else if !strings.Contains(out, expected) {
			t.Fatalf("exec failed with error %q: expected %q", out, expected)
		}
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("exec is running but should have failed")
	}

	logDone("exec - forbid piped stdin to tty enabled container")
}

func TestExecParseError(t *testing.T) {
	defer deleteAllContainers()

	runCmd := exec.Command(dockerBinary, "run", "-d", "--name", "top", "busybox", "top")
	if out, _, err := runCommandWithOutput(runCmd); err != nil {
		t.Fatal(out, err)
	}

	// Test normal (non-detached) case first
	cmd := exec.Command(dockerBinary, "exec", "top")
	if _, stderr, code, err := runCommandWithStdoutStderr(cmd); err == nil || !strings.Contains(stderr, "See '"+dockerBinary+" exec --help'") || code == 0 {
		t.Fatalf("Should have thrown error & point to help: %s", stderr)
	}
	logDone("exec - error on parseExec should point to help")
}

func TestExecStopNotHanging(t *testing.T) {
	defer deleteAllContainers()
	if out, err := exec.Command(dockerBinary, "run", "-d", "--name", "testing", "busybox", "top").CombinedOutput(); err != nil {
		t.Fatal(out, err)
	}

	if err := exec.Command(dockerBinary, "exec", "testing", "top").Start(); err != nil {
		t.Fatal(err)
	}

	wait := make(chan struct{})
	go func() {
		if out, err := exec.Command(dockerBinary, "stop", "testing").CombinedOutput(); err != nil {
			t.Fatal(out, err)
		}
		close(wait)
	}()
	select {
	case <-time.After(3 * time.Second):
		t.Fatal("Container stop timed out")
	case <-wait:
	}
	logDone("exec - container with exec not hanging on stop")
}

func TestExecCgroup(t *testing.T) {
	defer deleteAllContainers()
	var cmd *exec.Cmd

	cmd = exec.Command(dockerBinary, "run", "-d", "--name", "testing", "busybox", "top")
	_, err := runCommand(cmd)
	if err != nil {
		t.Fatal(err)
	}

	cmd = exec.Command(dockerBinary, "exec", "testing", "cat", "/proc/1/cgroup")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(out, err)
	}
	containerCgroups := sort.StringSlice(strings.Split(string(out), "\n"))

	var wg sync.WaitGroup
	var s sync.Mutex
	execCgroups := []sort.StringSlice{}
	// exec a few times concurrently to get consistent failure
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			cmd := exec.Command(dockerBinary, "exec", "testing", "cat", "/proc/self/cgroup")
			out, _, err := runCommandWithOutput(cmd)
			if err != nil {
				t.Fatal(out, err)
			}
			cg := sort.StringSlice(strings.Split(string(out), "\n"))

			s.Lock()
			execCgroups = append(execCgroups, cg)
			s.Unlock()
			wg.Done()
		}()
	}
	wg.Wait()

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
			t.Fatal("cgroups mismatched")
		}
	}

	logDone("exec - exec has the container cgroups")
}

func TestInspectExecID(t *testing.T) {
	defer deleteAllContainers()

	out, exitCode, err := runCommandWithOutput(exec.Command(dockerBinary, "run", "-d", "busybox", "top"))
	if exitCode != 0 || err != nil {
		t.Fatalf("failed to run container: %s, %v", out, err)
	}
	id := strings.TrimSuffix(out, "\n")

	out, err = inspectField(id, "ExecIDs")
	if err != nil {
		t.Fatalf("failed to inspect container: %s, %v", out, err)
	}
	if out != "<no value>" {
		t.Fatalf("ExecIDs should be empty, got: %s", out)
	}

	exitCode, err = runCommand(exec.Command(dockerBinary, "exec", "-d", id, "ls", "/"))
	if exitCode != 0 || err != nil {
		t.Fatalf("failed to exec in container: %s, %v", out, err)
	}

	out, err = inspectField(id, "ExecIDs")
	if err != nil {
		t.Fatalf("failed to inspect container: %s, %v", out, err)
	}

	out = strings.TrimSuffix(out, "\n")
	if out == "[]" || out == "<no value>" {
		t.Fatalf("ExecIDs should not be empty, got: %s", out)
	}

	logDone("inspect - inspect a container with ExecIDs")
}

func TestLinksPingLinkedContainersOnRename(t *testing.T) {
	defer deleteAllContainers()

	var out string
	out, _, _ = dockerCmd(t, "run", "-d", "--name", "container1", "busybox", "sleep", "10")
	idA := stripTrailingCharacters(out)
	if idA == "" {
		t.Fatal(out, "id should not be nil")
	}
	out, _, _ = dockerCmd(t, "run", "-d", "--link", "container1:alias1", "--name", "container2", "busybox", "sleep", "10")
	idB := stripTrailingCharacters(out)
	if idB == "" {
		t.Fatal(out, "id should not be nil")
	}

	execCmd := exec.Command(dockerBinary, "exec", "container2", "ping", "-c", "1", "alias1", "-W", "1")
	out, _, err := runCommandWithOutput(execCmd)
	if err != nil {
		t.Fatal(out, err)
	}

	dockerCmd(t, "rename", "container1", "container_new")

	execCmd = exec.Command(dockerBinary, "exec", "container2", "ping", "-c", "1", "alias1", "-W", "1")
	out, _, err = runCommandWithOutput(execCmd)
	if err != nil {
		t.Fatal(out, err)
	}

	logDone("links - ping linked container upon rename")
}

func TestRunExecDir(t *testing.T) {
	testRequires(t, SameHostDaemon)
	cmd := exec.Command(dockerBinary, "run", "-d", "busybox", "top")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(err, out)
	}
	id := strings.TrimSpace(out)
	execDir := filepath.Join(execDriverPath, id)
	stateFile := filepath.Join(execDir, "state.json")

	{
		fi, err := os.Stat(execDir)
		if err != nil {
			t.Fatal(err)
		}
		if !fi.IsDir() {
			t.Fatalf("%q must be a directory", execDir)
		}
		fi, err = os.Stat(stateFile)
		if err != nil {
			t.Fatal(err)
		}
	}

	stopCmd := exec.Command(dockerBinary, "stop", id)
	out, _, err = runCommandWithOutput(stopCmd)
	if err != nil {
		t.Fatal(err, out)
	}
	{
		_, err := os.Stat(execDir)
		if err == nil {
			t.Fatal(err)
		}
		if err == nil {
			t.Fatalf("Exec directory %q exists for removed container!", execDir)
		}
		if !os.IsNotExist(err) {
			t.Fatalf("Error should be about non-existing, got %s", err)
		}
	}
	startCmd := exec.Command(dockerBinary, "start", id)
	out, _, err = runCommandWithOutput(startCmd)
	if err != nil {
		t.Fatal(err, out)
	}
	{
		fi, err := os.Stat(execDir)
		if err != nil {
			t.Fatal(err)
		}
		if !fi.IsDir() {
			t.Fatalf("%q must be a directory", execDir)
		}
		fi, err = os.Stat(stateFile)
		if err != nil {
			t.Fatal(err)
		}
	}
	rmCmd := exec.Command(dockerBinary, "rm", "-f", id)
	out, _, err = runCommandWithOutput(rmCmd)
	if err != nil {
		t.Fatal(err, out)
	}
	{
		_, err := os.Stat(execDir)
		if err == nil {
			t.Fatal(err)
		}
		if err == nil {
			t.Fatalf("Exec directory %q is exists for removed container!", execDir)
		}
		if !os.IsNotExist(err) {
			t.Fatalf("Error should be about non-existing, got %s", err)
		}
	}

	logDone("run - check execdriver dir behavior")
}

func TestRunMutableNetworkFiles(t *testing.T) {
	testRequires(t, SameHostDaemon)
	defer deleteAllContainers()

	for _, fn := range []string{"resolv.conf", "hosts"} {
		deleteAllContainers()

		content, err := runCommandAndReadContainerFile(fn, exec.Command(dockerBinary, "run", "-d", "--name", "c1", "busybox", "sh", "-c", fmt.Sprintf("echo success >/etc/%s && top", fn)))
		if err != nil {
			t.Fatal(err)
		}

		if strings.TrimSpace(string(content)) != "success" {
			t.Fatal("Content was not what was modified in the container", string(content))
		}

		out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "run", "-d", "--name", "c2", "busybox", "top"))
		if err != nil {
			t.Fatal(err)
		}

		contID := strings.TrimSpace(out)

		netFilePath := containerStorageFile(contID, fn)

		f, err := os.OpenFile(netFilePath, os.O_WRONLY|os.O_SYNC|os.O_APPEND, 0644)
		if err != nil {
			t.Fatal(err)
		}

		if _, err := f.Seek(0, 0); err != nil {
			f.Close()
			t.Fatal(err)
		}

		if err := f.Truncate(0); err != nil {
			f.Close()
			t.Fatal(err)
		}

		if _, err := f.Write([]byte("success2\n")); err != nil {
			f.Close()
			t.Fatal(err)
		}
		f.Close()

		res, err := exec.Command(dockerBinary, "exec", contID, "cat", "/etc/"+fn).CombinedOutput()
		if err != nil {
			t.Fatalf("Output: %s, error: %s", res, err)
		}
		if string(res) != "success2\n" {
			t.Fatalf("Expected content of %s: %q, got: %q", fn, "success2\n", res)
		}
	}
	logDone("run - mutable network files")
}
