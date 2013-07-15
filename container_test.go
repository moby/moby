package docker

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestIDFormat(t *testing.T) {
	runtime := mkRuntime(t)
	defer nuke(runtime)
	container1, err := NewBuilder(runtime).Create(
		&Config{
			Image: GetTestImage(runtime).ID,
			Cmd:   []string{"/bin/sh", "-c", "echo hello world"},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	match, err := regexp.Match("^[0-9a-f]{64}$", []byte(container1.ID))
	if err != nil {
		t.Fatal(err)
	}
	if !match {
		t.Fatalf("Invalid container ID: %s", container1.ID)
	}
}

func TestMultipleAttachRestart(t *testing.T) {
	runtime := mkRuntime(t)
	defer nuke(runtime)
	container, err := NewBuilder(runtime).Create(
		&Config{
			Image: GetTestImage(runtime).ID,
			Cmd: []string{"/bin/sh", "-c",
				"i=1; while [ $i -le 5 ]; do i=`expr $i + 1`;  echo hello; done"},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Destroy(container)

	// Simulate 3 client attaching to the container and stop/restart

	stdout1, err := container.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout2, err := container.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout3, err := container.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	hostConfig := &HostConfig{}
	if err := container.Start(hostConfig); err != nil {
		t.Fatal(err)
	}
	l1, err := bufio.NewReader(stdout1).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if strings.Trim(l1, " \r\n") != "hello" {
		t.Fatalf("Unexpected output. Expected [%s], received [%s]", "hello", l1)
	}
	l2, err := bufio.NewReader(stdout2).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if strings.Trim(l2, " \r\n") != "hello" {
		t.Fatalf("Unexpected output. Expected [%s], received [%s]", "hello", l2)
	}
	l3, err := bufio.NewReader(stdout3).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if strings.Trim(l3, " \r\n") != "hello" {
		t.Fatalf("Unexpected output. Expected [%s], received [%s]", "hello", l3)
	}

	if err := container.Stop(10); err != nil {
		t.Fatal(err)
	}

	stdout1, err = container.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout2, err = container.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout3, err = container.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := container.Start(hostConfig); err != nil {
		t.Fatal(err)
	}

	setTimeout(t, "Timeout reading from the process", 3*time.Second, func() {
		l1, err = bufio.NewReader(stdout1).ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		if strings.Trim(l1, " \r\n") != "hello" {
			t.Fatalf("Unexpected output. Expected [%s], received [%s]", "hello", l1)
		}
		l2, err = bufio.NewReader(stdout2).ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		if strings.Trim(l2, " \r\n") != "hello" {
			t.Fatalf("Unexpected output. Expected [%s], received [%s]", "hello", l2)
		}
		l3, err = bufio.NewReader(stdout3).ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		if strings.Trim(l3, " \r\n") != "hello" {
			t.Fatalf("Unexpected output. Expected [%s], received [%s]", "hello", l3)
		}
	})
	container.Wait()
}

func TestDiff(t *testing.T) {
	runtime := mkRuntime(t)
	defer nuke(runtime)

	builder := NewBuilder(runtime)

	// Create a container and remove a file
	container1, err := builder.Create(
		&Config{
			Image: GetTestImage(runtime).ID,
			Cmd:   []string{"/bin/rm", "/etc/passwd"},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Destroy(container1)

	if err := container1.Run(); err != nil {
		t.Fatal(err)
	}

	// Check the changelog
	c, err := container1.Changes()
	if err != nil {
		t.Fatal(err)
	}
	success := false
	for _, elem := range c {
		if elem.Path == "/etc/passwd" && elem.Kind == 2 {
			success = true
		}
	}
	if !success {
		t.Fatalf("/etc/passwd as been removed but is not present in the diff")
	}

	// Commit the container
	rwTar, err := container1.ExportRw()
	if err != nil {
		t.Error(err)
	}
	img, err := runtime.graph.Create(rwTar, container1, "unit test commited image - diff", "", nil)
	if err != nil {
		t.Error(err)
	}

	// Create a new container from the commited image
	container2, err := builder.Create(
		&Config{
			Image: img.ID,
			Cmd:   []string{"cat", "/etc/passwd"},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Destroy(container2)

	if err := container2.Run(); err != nil {
		t.Fatal(err)
	}

	// Check the changelog
	c, err = container2.Changes()
	if err != nil {
		t.Fatal(err)
	}
	for _, elem := range c {
		if elem.Path == "/etc/passwd" {
			t.Fatalf("/etc/passwd should not be present in the diff after commit.")
		}
	}

	// Create a new containere
	container3, err := builder.Create(
		&Config{
			Image: GetTestImage(runtime).ID,
			Cmd:   []string{"rm", "/bin/httpd"},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Destroy(container3)

	if err := container3.Run(); err != nil {
		t.Fatal(err)
	}

	// Check the changelog
	c, err = container3.Changes()
	if err != nil {
		t.Fatal(err)
	}
	success = false
	for _, elem := range c {
		if elem.Path == "/bin/httpd" && elem.Kind == 2 {
			success = true
		}
	}
	if !success {
		t.Fatalf("/bin/httpd should be present in the diff after commit.")
	}
}

func TestCommitAutoRun(t *testing.T) {
	runtime := mkRuntime(t)
	defer nuke(runtime)

	builder := NewBuilder(runtime)
	container1, err := builder.Create(
		&Config{
			Image: GetTestImage(runtime).ID,
			Cmd:   []string{"/bin/sh", "-c", "echo hello > /world"},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Destroy(container1)

	if container1.State.Running {
		t.Errorf("Container shouldn't be running")
	}
	if err := container1.Run(); err != nil {
		t.Fatal(err)
	}
	if container1.State.Running {
		t.Errorf("Container shouldn't be running")
	}

	rwTar, err := container1.ExportRw()
	if err != nil {
		t.Error(err)
	}
	img, err := runtime.graph.Create(rwTar, container1, "unit test commited image", "", &Config{Cmd: []string{"cat", "/world"}})
	if err != nil {
		t.Error(err)
	}

	// FIXME: Make a TestCommit that stops here and check docker.root/layers/img.id/world
	container2, err := builder.Create(
		&Config{
			Image: img.ID,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Destroy(container2)
	stdout, err := container2.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stderr, err := container2.StderrPipe()
	if err != nil {
		t.Fatal(err)
	}
	hostConfig := &HostConfig{}
	if err := container2.Start(hostConfig); err != nil {
		t.Fatal(err)
	}
	container2.Wait()
	output, err := ioutil.ReadAll(stdout)
	if err != nil {
		t.Fatal(err)
	}
	output2, err := ioutil.ReadAll(stderr)
	if err != nil {
		t.Fatal(err)
	}
	if err := stdout.Close(); err != nil {
		t.Fatal(err)
	}
	if err := stderr.Close(); err != nil {
		t.Fatal(err)
	}
	if string(output) != "hello\n" {
		t.Fatalf("Unexpected output. Expected %s, received: %s (err: %s)", "hello\n", output, output2)
	}
}

func TestCommitRun(t *testing.T) {
	runtime := mkRuntime(t)
	defer nuke(runtime)

	builder := NewBuilder(runtime)

	container1, err := builder.Create(
		&Config{
			Image: GetTestImage(runtime).ID,
			Cmd:   []string{"/bin/sh", "-c", "echo hello > /world"},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Destroy(container1)

	if container1.State.Running {
		t.Errorf("Container shouldn't be running")
	}
	if err := container1.Run(); err != nil {
		t.Fatal(err)
	}
	if container1.State.Running {
		t.Errorf("Container shouldn't be running")
	}

	rwTar, err := container1.ExportRw()
	if err != nil {
		t.Error(err)
	}
	img, err := runtime.graph.Create(rwTar, container1, "unit test commited image", "", nil)
	if err != nil {
		t.Error(err)
	}

	// FIXME: Make a TestCommit that stops here and check docker.root/layers/img.id/world

	container2, err := builder.Create(
		&Config{
			Image: img.ID,
			Cmd:   []string{"cat", "/world"},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Destroy(container2)
	stdout, err := container2.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stderr, err := container2.StderrPipe()
	if err != nil {
		t.Fatal(err)
	}
	hostConfig := &HostConfig{}
	if err := container2.Start(hostConfig); err != nil {
		t.Fatal(err)
	}
	container2.Wait()
	output, err := ioutil.ReadAll(stdout)
	if err != nil {
		t.Fatal(err)
	}
	output2, err := ioutil.ReadAll(stderr)
	if err != nil {
		t.Fatal(err)
	}
	if err := stdout.Close(); err != nil {
		t.Fatal(err)
	}
	if err := stderr.Close(); err != nil {
		t.Fatal(err)
	}
	if string(output) != "hello\n" {
		t.Fatalf("Unexpected output. Expected %s, received: %s (err: %s)", "hello\n", output, output2)
	}
}

func TestStart(t *testing.T) {
	runtime := mkRuntime(t)
	defer nuke(runtime)
	container, err := NewBuilder(runtime).Create(
		&Config{
			Image:     GetTestImage(runtime).ID,
			Memory:    33554432,
			CpuShares: 1000,
			Cmd:       []string{"/bin/cat"},
			OpenStdin: true,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Destroy(container)

	cStdin, err := container.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}

	hostConfig := &HostConfig{}
	if err := container.Start(hostConfig); err != nil {
		t.Fatal(err)
	}

	// Give some time to the process to start
	container.WaitTimeout(500 * time.Millisecond)

	if !container.State.Running {
		t.Errorf("Container should be running")
	}
	if err := container.Start(hostConfig); err == nil {
		t.Fatalf("A running containter should be able to be started")
	}

	// Try to avoid the timeoout in destroy. Best effort, don't check error
	cStdin.Close()
	container.WaitTimeout(2 * time.Second)
}

func TestRun(t *testing.T) {
	runtime := mkRuntime(t)
	defer nuke(runtime)
	container, err := NewBuilder(runtime).Create(
		&Config{
			Image: GetTestImage(runtime).ID,
			Cmd:   []string{"ls", "-al"},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Destroy(container)

	if container.State.Running {
		t.Errorf("Container shouldn't be running")
	}
	if err := container.Run(); err != nil {
		t.Fatal(err)
	}
	if container.State.Running {
		t.Errorf("Container shouldn't be running")
	}
}

func TestOutput(t *testing.T) {
	runtime := mkRuntime(t)
	defer nuke(runtime)
	container, err := NewBuilder(runtime).Create(
		&Config{
			Image: GetTestImage(runtime).ID,
			Cmd:   []string{"echo", "-n", "foobar"},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Destroy(container)
	output, err := container.Output()
	if err != nil {
		t.Fatal(err)
	}
	if string(output) != "foobar" {
		t.Error(string(output))
	}
}

func TestKillDifferentUser(t *testing.T) {
	runtime := mkRuntime(t)
	defer nuke(runtime)
	container, err := NewBuilder(runtime).Create(&Config{
		Image: GetTestImage(runtime).ID,
		Cmd:   []string{"tail", "-f", "/etc/resolv.conf"},
		User:  "daemon",
	},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Destroy(container)

	if container.State.Running {
		t.Errorf("Container shouldn't be running")
	}
	hostConfig := &HostConfig{}
	if err := container.Start(hostConfig); err != nil {
		t.Fatal(err)
	}

	setTimeout(t, "Waiting for the container to be started timed out", 2*time.Second, func() {
		for !container.State.Running {
			time.Sleep(10 * time.Millisecond)
		}
	})

	// Even if the state is running, lets give some time to lxc to spawn the process
	container.WaitTimeout(500 * time.Millisecond)

	if err := container.Kill(); err != nil {
		t.Fatal(err)
	}

	if container.State.Running {
		t.Errorf("Container shouldn't be running")
	}
	container.Wait()
	if container.State.Running {
		t.Errorf("Container shouldn't be running")
	}
	// Try stopping twice
	if err := container.Kill(); err != nil {
		t.Fatal(err)
	}
}

// Test that creating a container with a volume doesn't crash. Regression test for #995.
func TestCreateVolume(t *testing.T) {
	runtime := mkRuntime(t)
	defer nuke(runtime)

	config, hc, _, err := ParseRun([]string{"-v", "/var/lib/data", GetTestImage(runtime).ID, "echo", "hello", "world"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	c, err := NewBuilder(runtime).Create(config)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Destroy(c)
	if err := c.Start(hc); err != nil {
		t.Fatal(err)
	}
	c.WaitTimeout(500 * time.Millisecond)
	c.Wait()
}

func TestKill(t *testing.T) {
	runtime := mkRuntime(t)
	defer nuke(runtime)
	container, err := NewBuilder(runtime).Create(&Config{
		Image: GetTestImage(runtime).ID,
		Cmd:   []string{"sleep", "2"},
	},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Destroy(container)

	if container.State.Running {
		t.Errorf("Container shouldn't be running")
	}
	hostConfig := &HostConfig{}
	if err := container.Start(hostConfig); err != nil {
		t.Fatal(err)
	}

	// Give some time to lxc to spawn the process
	container.WaitTimeout(500 * time.Millisecond)

	if !container.State.Running {
		t.Errorf("Container should be running")
	}
	if err := container.Kill(); err != nil {
		t.Fatal(err)
	}
	if container.State.Running {
		t.Errorf("Container shouldn't be running")
	}
	container.Wait()
	if container.State.Running {
		t.Errorf("Container shouldn't be running")
	}
	// Try stopping twice
	if err := container.Kill(); err != nil {
		t.Fatal(err)
	}
}

func TestExitCode(t *testing.T) {
	runtime := mkRuntime(t)
	defer nuke(runtime)

	builder := NewBuilder(runtime)

	trueContainer, err := builder.Create(&Config{
		Image: GetTestImage(runtime).ID,
		Cmd:   []string{"/bin/true", ""},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Destroy(trueContainer)
	if err := trueContainer.Run(); err != nil {
		t.Fatal(err)
	}
	if trueContainer.State.ExitCode != 0 {
		t.Errorf("Unexpected exit code %d (expected 0)", trueContainer.State.ExitCode)
	}

	falseContainer, err := builder.Create(&Config{
		Image: GetTestImage(runtime).ID,
		Cmd:   []string{"/bin/false", ""},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Destroy(falseContainer)
	if err := falseContainer.Run(); err != nil {
		t.Fatal(err)
	}
	if falseContainer.State.ExitCode != 1 {
		t.Errorf("Unexpected exit code %d (expected 1)", falseContainer.State.ExitCode)
	}
}

func TestRestart(t *testing.T) {
	runtime := mkRuntime(t)
	defer nuke(runtime)
	container, err := NewBuilder(runtime).Create(&Config{
		Image: GetTestImage(runtime).ID,
		Cmd:   []string{"echo", "-n", "foobar"},
	},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Destroy(container)
	output, err := container.Output()
	if err != nil {
		t.Fatal(err)
	}
	if string(output) != "foobar" {
		t.Error(string(output))
	}

	// Run the container again and check the output
	output, err = container.Output()
	if err != nil {
		t.Fatal(err)
	}
	if string(output) != "foobar" {
		t.Error(string(output))
	}
}

func TestRestartStdin(t *testing.T) {
	runtime := mkRuntime(t)
	defer nuke(runtime)
	container, err := NewBuilder(runtime).Create(&Config{
		Image: GetTestImage(runtime).ID,
		Cmd:   []string{"cat"},

		OpenStdin: true,
	},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Destroy(container)

	stdin, err := container.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := container.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	hostConfig := &HostConfig{}
	if err := container.Start(hostConfig); err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(stdin, "hello world"); err != nil {
		t.Fatal(err)
	}
	if err := stdin.Close(); err != nil {
		t.Fatal(err)
	}
	container.Wait()
	output, err := ioutil.ReadAll(stdout)
	if err != nil {
		t.Fatal(err)
	}
	if err := stdout.Close(); err != nil {
		t.Fatal(err)
	}
	if string(output) != "hello world" {
		t.Fatalf("Unexpected output. Expected %s, received: %s", "hello world", string(output))
	}

	// Restart and try again
	stdin, err = container.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err = container.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := container.Start(hostConfig); err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(stdin, "hello world #2"); err != nil {
		t.Fatal(err)
	}
	if err := stdin.Close(); err != nil {
		t.Fatal(err)
	}
	container.Wait()
	output, err = ioutil.ReadAll(stdout)
	if err != nil {
		t.Fatal(err)
	}
	if err := stdout.Close(); err != nil {
		t.Fatal(err)
	}
	if string(output) != "hello world #2" {
		t.Fatalf("Unexpected output. Expected %s, received: %s", "hello world #2", string(output))
	}
}

func TestUser(t *testing.T) {
	runtime := mkRuntime(t)
	defer nuke(runtime)

	builder := NewBuilder(runtime)

	// Default user must be root
	container, err := builder.Create(&Config{
		Image: GetTestImage(runtime).ID,
		Cmd:   []string{"id"},
	},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Destroy(container)
	output, err := container.Output()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(output), "uid=0(root) gid=0(root)") {
		t.Error(string(output))
	}

	// Set a username
	container, err = builder.Create(&Config{
		Image: GetTestImage(runtime).ID,
		Cmd:   []string{"id"},

		User: "root",
	},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Destroy(container)
	output, err = container.Output()
	if err != nil || container.State.ExitCode != 0 {
		t.Fatal(err)
	}
	if !strings.Contains(string(output), "uid=0(root) gid=0(root)") {
		t.Error(string(output))
	}

	// Set a UID
	container, err = builder.Create(&Config{
		Image: GetTestImage(runtime).ID,
		Cmd:   []string{"id"},

		User: "0",
	},
	)
	if err != nil || container.State.ExitCode != 0 {
		t.Fatal(err)
	}
	defer runtime.Destroy(container)
	output, err = container.Output()
	if err != nil || container.State.ExitCode != 0 {
		t.Fatal(err)
	}
	if !strings.Contains(string(output), "uid=0(root) gid=0(root)") {
		t.Error(string(output))
	}

	// Set a different user by uid
	container, err = builder.Create(&Config{
		Image: GetTestImage(runtime).ID,
		Cmd:   []string{"id"},

		User: "1",
	},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Destroy(container)
	output, err = container.Output()
	if err != nil {
		t.Fatal(err)
	} else if container.State.ExitCode != 0 {
		t.Fatalf("Container exit code is invalid: %d\nOutput:\n%s\n", container.State.ExitCode, output)
	}
	if !strings.Contains(string(output), "uid=1(daemon) gid=1(daemon)") {
		t.Error(string(output))
	}

	// Set a different user by username
	container, err = builder.Create(&Config{
		Image: GetTestImage(runtime).ID,
		Cmd:   []string{"id"},

		User: "daemon",
	},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Destroy(container)
	output, err = container.Output()
	if err != nil || container.State.ExitCode != 0 {
		t.Fatal(err)
	}
	if !strings.Contains(string(output), "uid=1(daemon) gid=1(daemon)") {
		t.Error(string(output))
	}
}

func TestMultipleContainers(t *testing.T) {
	runtime := mkRuntime(t)
	defer nuke(runtime)

	builder := NewBuilder(runtime)

	container1, err := builder.Create(&Config{
		Image: GetTestImage(runtime).ID,
		Cmd:   []string{"sleep", "2"},
	},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Destroy(container1)

	container2, err := builder.Create(&Config{
		Image: GetTestImage(runtime).ID,
		Cmd:   []string{"sleep", "2"},
	},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Destroy(container2)

	// Start both containers
	hostConfig := &HostConfig{}
	if err := container1.Start(hostConfig); err != nil {
		t.Fatal(err)
	}
	if err := container2.Start(hostConfig); err != nil {
		t.Fatal(err)
	}

	// Make sure they are running before trying to kill them
	container1.WaitTimeout(250 * time.Millisecond)
	container2.WaitTimeout(250 * time.Millisecond)

	// If we are here, both containers should be running
	if !container1.State.Running {
		t.Fatal("Container not running")
	}
	if !container2.State.Running {
		t.Fatal("Container not running")
	}

	// Kill them
	if err := container1.Kill(); err != nil {
		t.Fatal(err)
	}

	if err := container2.Kill(); err != nil {
		t.Fatal(err)
	}
}

func TestStdin(t *testing.T) {
	runtime := mkRuntime(t)
	defer nuke(runtime)
	container, err := NewBuilder(runtime).Create(&Config{
		Image: GetTestImage(runtime).ID,
		Cmd:   []string{"cat"},

		OpenStdin: true,
	},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Destroy(container)

	stdin, err := container.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := container.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	hostConfig := &HostConfig{}
	if err := container.Start(hostConfig); err != nil {
		t.Fatal(err)
	}
	defer stdin.Close()
	defer stdout.Close()
	if _, err := io.WriteString(stdin, "hello world"); err != nil {
		t.Fatal(err)
	}
	if err := stdin.Close(); err != nil {
		t.Fatal(err)
	}
	container.Wait()
	output, err := ioutil.ReadAll(stdout)
	if err != nil {
		t.Fatal(err)
	}
	if string(output) != "hello world" {
		t.Fatalf("Unexpected output. Expected %s, received: %s", "hello world", string(output))
	}
}

func TestTty(t *testing.T) {
	runtime := mkRuntime(t)
	defer nuke(runtime)
	container, err := NewBuilder(runtime).Create(&Config{
		Image: GetTestImage(runtime).ID,
		Cmd:   []string{"cat"},

		OpenStdin: true,
	},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Destroy(container)

	stdin, err := container.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := container.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	hostConfig := &HostConfig{}
	if err := container.Start(hostConfig); err != nil {
		t.Fatal(err)
	}
	defer stdin.Close()
	defer stdout.Close()
	if _, err := io.WriteString(stdin, "hello world"); err != nil {
		t.Fatal(err)
	}
	if err := stdin.Close(); err != nil {
		t.Fatal(err)
	}
	container.Wait()
	output, err := ioutil.ReadAll(stdout)
	if err != nil {
		t.Fatal(err)
	}
	if string(output) != "hello world" {
		t.Fatalf("Unexpected output. Expected %s, received: %s", "hello world", string(output))
	}
}

func TestEnv(t *testing.T) {
	runtime := mkRuntime(t)
	defer nuke(runtime)
	container, err := NewBuilder(runtime).Create(&Config{
		Image: GetTestImage(runtime).ID,
		Cmd:   []string{"env"},
	},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Destroy(container)

	stdout, err := container.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer stdout.Close()
	hostConfig := &HostConfig{}
	if err := container.Start(hostConfig); err != nil {
		t.Fatal(err)
	}
	container.Wait()
	output, err := ioutil.ReadAll(stdout)
	if err != nil {
		t.Fatal(err)
	}
	actualEnv := strings.Split(string(output), "\n")
	if actualEnv[len(actualEnv)-1] == "" {
		actualEnv = actualEnv[:len(actualEnv)-1]
	}
	sort.Strings(actualEnv)
	goodEnv := []string{
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"HOME=/",
	}
	sort.Strings(goodEnv)
	if len(goodEnv) != len(actualEnv) {
		t.Fatalf("Wrong environment: should be %d variables, not: '%s'\n", len(goodEnv), strings.Join(actualEnv, ", "))
	}
	for i := range goodEnv {
		if actualEnv[i] != goodEnv[i] {
			t.Fatalf("Wrong environment variable: should be %s, not %s", goodEnv[i], actualEnv[i])
		}
	}
}

func TestEntrypoint(t *testing.T) {
	runtime := mkRuntime(t)
	defer nuke(runtime)
	container, err := NewBuilder(runtime).Create(
		&Config{
			Image:      GetTestImage(runtime).ID,
			Entrypoint: []string{"/bin/echo"},
			Cmd:        []string{"-n", "foobar"},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Destroy(container)
	output, err := container.Output()
	if err != nil {
		t.Fatal(err)
	}
	if string(output) != "foobar" {
		t.Error(string(output))
	}
}

func grepFile(t *testing.T, path string, pattern string) {
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := bufio.NewReader(f)
	var (
		line string
	)
	err = nil
	for err == nil {
		line, err = r.ReadString('\n')
		if strings.Contains(line, pattern) == true {
			return
		}
	}
	t.Fatalf("grepFile: pattern \"%s\" not found in \"%s\"", pattern, path)
}

func TestLXCConfig(t *testing.T) {
	runtime := mkRuntime(t)
	defer nuke(runtime)
	// Memory is allocated randomly for testing
	rand.Seed(time.Now().UTC().UnixNano())
	memMin := 33554432
	memMax := 536870912
	mem := memMin + rand.Intn(memMax-memMin)
	// CPU shares as well
	cpuMin := 100
	cpuMax := 10000
	cpu := cpuMin + rand.Intn(cpuMax-cpuMin)
	container, err := NewBuilder(runtime).Create(&Config{
		Image: GetTestImage(runtime).ID,
		Cmd:   []string{"/bin/true"},

		Hostname:  "foobar",
		Memory:    int64(mem),
		CpuShares: int64(cpu),
	},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Destroy(container)
	container.generateLXCConfig()
	grepFile(t, container.lxcConfigPath(), "lxc.utsname = foobar")
	grepFile(t, container.lxcConfigPath(),
		fmt.Sprintf("lxc.cgroup.memory.limit_in_bytes = %d", mem))
	grepFile(t, container.lxcConfigPath(),
		fmt.Sprintf("lxc.cgroup.memory.memsw.limit_in_bytes = %d", mem*2))
}

func BenchmarkRunSequencial(b *testing.B) {
	runtime := mkRuntime(b)
	defer nuke(runtime)
	for i := 0; i < b.N; i++ {
		container, err := NewBuilder(runtime).Create(&Config{
			Image: GetTestImage(runtime).ID,
			Cmd:   []string{"echo", "-n", "foo"},
		},
		)
		if err != nil {
			b.Fatal(err)
		}
		defer runtime.Destroy(container)
		output, err := container.Output()
		if err != nil {
			b.Fatal(err)
		}
		if string(output) != "foo" {
			b.Fatalf("Unexpected output: %s", output)
		}
		if err := runtime.Destroy(container); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRunParallel(b *testing.B) {
	runtime := mkRuntime(b)
	defer nuke(runtime)

	var tasks []chan error

	for i := 0; i < b.N; i++ {
		complete := make(chan error)
		tasks = append(tasks, complete)
		go func(i int, complete chan error) {
			container, err := NewBuilder(runtime).Create(&Config{
				Image: GetTestImage(runtime).ID,
				Cmd:   []string{"echo", "-n", "foo"},
			},
			)
			if err != nil {
				complete <- err
				return
			}
			defer runtime.Destroy(container)
			hostConfig := &HostConfig{}
			if err := container.Start(hostConfig); err != nil {
				complete <- err
				return
			}
			if err := container.WaitTimeout(15 * time.Second); err != nil {
				complete <- err
				return
			}
			// if string(output) != "foo" {
			// 	complete <- fmt.Errorf("Unexecpted output: %v", string(output))
			// }
			if err := runtime.Destroy(container); err != nil {
				complete <- err
				return
			}
			complete <- nil
		}(i, complete)
	}
	var errors []error
	for _, task := range tasks {
		err := <-task
		if err != nil {
			errors = append(errors, err)
		}
	}
	if len(errors) > 0 {
		b.Fatal(errors)
	}
}

func tempDir(t *testing.T) string {
	tmpDir, err := ioutil.TempDir("", "docker-test")
	if err != nil {
		t.Fatal(err)
	}
	return tmpDir
}

func TestBindMounts(t *testing.T) {
	r := mkRuntime(t)
	defer nuke(r)
	tmpDir := tempDir(t)
	defer os.RemoveAll(tmpDir)
	writeFile(path.Join(tmpDir, "touch-me"), "", t)

	// Test reading from a read-only bind mount
	stdout, _ := runContainer(r, []string{"-b", fmt.Sprintf("%s:/tmp:ro", tmpDir), "_", "ls", "/tmp"}, t)
	if !strings.Contains(stdout, "touch-me") {
		t.Fatal("Container failed to read from bind mount")
	}

	// test writing to bind mount
	runContainer(r, []string{"-b", fmt.Sprintf("%s:/tmp:rw", tmpDir), "_", "touch", "/tmp/holla"}, t)
	readFile(path.Join(tmpDir, "holla"), t) // Will fail if the file doesn't exist

	// test mounting to an illegal destination directory
	if _, err := runContainer(r, []string{"-b", fmt.Sprintf("%s:.", tmpDir), "ls", "."}, nil); err == nil {
		t.Fatal("Container bind mounted illegal directory")

	}
}

// Test that VolumesRW values are copied to the new container.  Regression test for #1201
func TestVolumesFromReadonlyMount(t *testing.T) {
	runtime := mkRuntime(t)
	defer nuke(runtime)
	container, err := NewBuilder(runtime).Create(
		&Config{
			Image:   GetTestImage(runtime).ID,
			Cmd:     []string{"/bin/echo", "-n", "foobar"},
			Volumes: map[string]struct{}{"/test": {}},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Destroy(container)
	_, err = container.Output()
	if err != nil {
		t.Fatal(err)
	}
	if !container.VolumesRW["/test"] {
		t.Fail()
	}

	container2, err := NewBuilder(runtime).Create(
		&Config{
			Image:       GetTestImage(runtime).ID,
			Cmd:         []string{"/bin/echo", "-n", "foobar"},
			VolumesFrom: container.ID,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Destroy(container2)

	_, err = container2.Output()
	if err != nil {
		t.Fatal(err)
	}

	if container.Volumes["/test"] != container2.Volumes["/test"] {
		t.Fail()
	}

	actual, exists := container2.VolumesRW["/test"]
	if !exists {
		t.Fail()
	}

	if container.VolumesRW["/test"] != actual {
		t.Fail()
	}
}
