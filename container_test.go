package docker

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestCommitRun(t *testing.T) {
	docker, err := newTestDocker()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(docker)
	container1, err := docker.Create(
		"/bin/sh",
		[]string{"-c", "echo hello > /world"},
		GetTestImage(docker).Id,
		&Config{
			Memory: 33554432,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer docker.Destroy(container1)

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
	img, err := docker.graph.Create(rwTar, container1.Image, "unit test commited image")
	if err != nil {
		t.Error(err)
	}

	// FIXME: Make a TestCommit that stops here and check docker.root/layers/img.id/world

	container2, err := docker.Create(
		"cat",
		[]string{"/world"},
		img.Id,
		&Config{
			Memory: 33554432,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer docker.Destroy(container2)

	stdout, err := container2.StdoutPipe()
	stderr, err := container2.StderrPipe()
	if err := container2.Start(); err != nil {
		t.Fatal(err)
	}
	container2.Wait()
	output, err := ioutil.ReadAll(stdout)
	output2, err := ioutil.ReadAll(stderr)
	stdout.Close()
	stderr.Close()
	if string(output) != "hello\n" {
		t.Fatalf("\nout: %s\nerr: %s\n", string(output), string(output2))
	}
}

func TestRun(t *testing.T) {
	docker, err := newTestDocker()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(docker)
	container, err := docker.Create(
		"ls",
		[]string{"-al"},
		GetTestImage(docker).Id,
		&Config{
			Memory: 33554432,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer docker.Destroy(container)

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
	docker, err := newTestDocker()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(docker)
	container, err := docker.Create(
		"echo",
		[]string{"-n", "foobar"},
		GetTestImage(docker).Id,
		&Config{},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer docker.Destroy(container)
	output, err := container.Output()
	if err != nil {
		t.Fatal(err)
	}
	if string(output) != "foobar" {
		t.Error(string(output))
	}
}

func TestKill(t *testing.T) {
	docker, err := newTestDocker()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(docker)
	container, err := docker.Create(
		"cat",
		[]string{"/dev/zero"},
		GetTestImage(docker).Id,
		&Config{},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer docker.Destroy(container)

	if container.State.Running {
		t.Errorf("Container shouldn't be running")
	}
	if err := container.Start(); err != nil {
		t.Fatal(err)
	}
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
	docker, err := newTestDocker()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(docker)

	trueContainer, err := docker.Create(
		"/bin/true",
		[]string{""},
		GetTestImage(docker).Id,
		&Config{},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer docker.Destroy(trueContainer)
	if err := trueContainer.Run(); err != nil {
		t.Fatal(err)
	}

	falseContainer, err := docker.Create(
		"/bin/false",
		[]string{""},
		GetTestImage(docker).Id,
		&Config{},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer docker.Destroy(falseContainer)
	if err := falseContainer.Run(); err != nil {
		t.Fatal(err)
	}

	if trueContainer.State.ExitCode != 0 {
		t.Errorf("Unexpected exit code %v", trueContainer.State.ExitCode)
	}

	if falseContainer.State.ExitCode != 1 {
		t.Errorf("Unexpected exit code %v", falseContainer.State.ExitCode)
	}
}

func TestRestart(t *testing.T) {
	docker, err := newTestDocker()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(docker)
	container, err := docker.Create(
		"echo",
		[]string{"-n", "foobar"},
		GetTestImage(docker).Id,
		&Config{},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer docker.Destroy(container)
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
	docker, err := newTestDocker()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(docker)
	container, err := docker.Create(
		"cat",
		[]string{},
		GetTestImage(docker).Id,
		&Config{
			OpenStdin: true,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer docker.Destroy(container)

	stdin, err := container.StdinPipe()
	stdout, err := container.StdoutPipe()
	if err := container.Start(); err != nil {
		t.Fatal(err)
	}
	io.WriteString(stdin, "hello world")
	stdin.Close()
	container.Wait()
	output, err := ioutil.ReadAll(stdout)
	stdout.Close()
	if string(output) != "hello world" {
		t.Fatal(string(output))
	}

	// Restart and try again
	stdin, err = container.StdinPipe()
	stdout, err = container.StdoutPipe()
	if err := container.Start(); err != nil {
		t.Fatal(err)
	}
	io.WriteString(stdin, "hello world #2")
	stdin.Close()
	container.Wait()
	output, err = ioutil.ReadAll(stdout)
	stdout.Close()
	if string(output) != "hello world #2" {
		t.Fatal(string(output))
	}
}

func TestUser(t *testing.T) {
	docker, err := newTestDocker()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(docker)

	// Default user must be root
	container, err := docker.Create(
		"id",
		[]string{},
		GetTestImage(docker).Id,
		&Config{},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer docker.Destroy(container)
	output, err := container.Output()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(output), "uid=0(root) gid=0(root)") {
		t.Error(string(output))
	}

	// Set a username
	container, err = docker.Create(
		"id",
		[]string{},
		GetTestImage(docker).Id,
		&Config{
			User: "root",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer docker.Destroy(container)
	output, err = container.Output()
	if err != nil || container.State.ExitCode != 0 {
		t.Fatal(err)
	}
	if !strings.Contains(string(output), "uid=0(root) gid=0(root)") {
		t.Error(string(output))
	}

	// Set a UID
	container, err = docker.Create(
		"id",
		[]string{},
		GetTestImage(docker).Id,
		&Config{
			User: "0",
		},
	)
	if err != nil || container.State.ExitCode != 0 {
		t.Fatal(err)
	}
	defer docker.Destroy(container)
	output, err = container.Output()
	if err != nil || container.State.ExitCode != 0 {
		t.Fatal(err)
	}
	if !strings.Contains(string(output), "uid=0(root) gid=0(root)") {
		t.Error(string(output))
	}

	// Set a different user by uid
	container, err = docker.Create(
		"id",
		[]string{},
		GetTestImage(docker).Id,
		&Config{
			User: "1",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer docker.Destroy(container)
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
	container, err = docker.Create(
		"id",
		[]string{},
		GetTestImage(docker).Id,
		&Config{
			User: "daemon",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer docker.Destroy(container)
	output, err = container.Output()
	if err != nil || container.State.ExitCode != 0 {
		t.Fatal(err)
	}
	if !strings.Contains(string(output), "uid=1(daemon) gid=1(daemon)") {
		t.Error(string(output))
	}
}

func TestMultipleContainers(t *testing.T) {
	docker, err := newTestDocker()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(docker)

	container1, err := docker.Create(
		"cat",
		[]string{"/dev/zero"},
		GetTestImage(docker).Id,
		&Config{},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer docker.Destroy(container1)

	container2, err := docker.Create(
		"cat",
		[]string{"/dev/zero"},
		GetTestImage(docker).Id,
		&Config{},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer docker.Destroy(container2)

	// Start both containers
	if err := container1.Start(); err != nil {
		t.Fatal(err)
	}
	if err := container2.Start(); err != nil {
		t.Fatal(err)
	}

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
	docker, err := newTestDocker()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(docker)
	container, err := docker.Create(
		"cat",
		[]string{},
		GetTestImage(docker).Id,
		&Config{
			OpenStdin: true,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer docker.Destroy(container)

	stdin, err := container.StdinPipe()
	stdout, err := container.StdoutPipe()
	defer stdin.Close()
	defer stdout.Close()
	if err := container.Start(); err != nil {
		t.Fatal(err)
	}
	io.WriteString(stdin, "hello world")
	stdin.Close()
	container.Wait()
	output, err := ioutil.ReadAll(stdout)
	if string(output) != "hello world" {
		t.Fatal(string(output))
	}
}

func TestTty(t *testing.T) {
	docker, err := newTestDocker()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(docker)
	container, err := docker.Create(
		"cat",
		[]string{},
		GetTestImage(docker).Id,
		&Config{
			OpenStdin: true,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer docker.Destroy(container)

	stdin, err := container.StdinPipe()
	stdout, err := container.StdoutPipe()
	defer stdin.Close()
	defer stdout.Close()
	if err := container.Start(); err != nil {
		t.Fatal(err)
	}
	io.WriteString(stdin, "hello world")
	stdin.Close()
	container.Wait()
	output, err := ioutil.ReadAll(stdout)
	if string(output) != "hello world" {
		t.Fatal(string(output))
	}
}

func TestEnv(t *testing.T) {
	docker, err := newTestDocker()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(docker)
	container, err := docker.Create(
		"/usr/bin/env",
		[]string{},
		GetTestImage(docker).Id,
		&Config{},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer docker.Destroy(container)
	stdout, err := container.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer stdout.Close()
	if err := container.Start(); err != nil {
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
	docker, err := newTestDocker()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(docker)
	// Memory is allocated randomly for testing
	rand.Seed(time.Now().UTC().UnixNano())
	memMin := 33554432
	memMax := 536870912
	mem := memMin + rand.Intn(memMax-memMin)
	container, err := docker.Create(
		"/bin/true",
		[]string{},
		GetTestImage(docker).Id,
		&Config{
			Hostname: "foobar",
			Memory:   int64(mem),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer docker.Destroy(container)
	container.generateLXCConfig()
	grepFile(t, container.lxcConfigPath(), "lxc.utsname = foobar")
	grepFile(t, container.lxcConfigPath(),
		fmt.Sprintf("lxc.cgroup.memory.limit_in_bytes = %d", mem))
	grepFile(t, container.lxcConfigPath(),
		fmt.Sprintf("lxc.cgroup.memory.memsw.limit_in_bytes = %d", mem*2))
}

func BenchmarkRunSequencial(b *testing.B) {
	docker, err := newTestDocker()
	if err != nil {
		b.Fatal(err)
	}
	defer nuke(docker)
	for i := 0; i < b.N; i++ {
		container, err := docker.Create(
			"echo",
			[]string{"-n", "foo"},
			GetTestImage(docker).Id,
			&Config{},
		)
		if err != nil {
			b.Fatal(err)
		}
		defer docker.Destroy(container)
		output, err := container.Output()
		if err != nil {
			b.Fatal(err)
		}
		if string(output) != "foo" {
			b.Fatalf("Unexecpted output: %v", string(output))
		}
		if err := docker.Destroy(container); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRunParallel(b *testing.B) {
	docker, err := newTestDocker()
	if err != nil {
		b.Fatal(err)
	}
	defer nuke(docker)

	var tasks []chan error

	for i := 0; i < b.N; i++ {
		complete := make(chan error)
		tasks = append(tasks, complete)
		go func(i int, complete chan error) {
			container, err := docker.Create(
				"echo",
				[]string{"-n", "foo"},
				GetTestImage(docker).Id,
				&Config{},
			)
			if err != nil {
				complete <- err
				return
			}
			defer docker.Destroy(container)
			if err := container.Start(); err != nil {
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
			if err := docker.Destroy(container); err != nil {
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
