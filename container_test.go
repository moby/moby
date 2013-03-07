package docker

import (
	"fmt"
	"io"
	"io/ioutil"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestStart(t *testing.T) {
	docker, err := newTestDocker()
	if err != nil {
		t.Fatal(err)
	}
	container, err := docker.Create(
		"start_test",
		"ls",
		[]string{"-al"},
		[]string{testLayerPath},
		&Config{
			Ram: 33554432,
		},
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
	container.Wait()
	if container.State.Running {
		t.Errorf("Container shouldn't be running")
	}
	// We should be able to call Wait again
	container.Wait()
	if container.State.Running {
		t.Errorf("Container shouldn't be running")
	}
}

func TestRun(t *testing.T) {
	docker, err := newTestDocker()
	if err != nil {
		t.Fatal(err)
	}
	container, err := docker.Create(
		"run_test",
		"ls",
		[]string{"-al"},
		[]string{testLayerPath},
		&Config{
			Ram: 33554432,
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
	container, err := docker.Create(
		"output_test",
		"echo",
		[]string{"-n", "foobar"},
		[]string{testLayerPath},
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
	container, err := docker.Create(
		"stop_test",
		"cat",
		[]string{"/dev/zero"},
		[]string{testLayerPath},
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

	trueContainer, err := docker.Create(
		"exit_test_1",
		"/bin/true",
		[]string{""},
		[]string{testLayerPath},
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
		"exit_test_2",
		"/bin/false",
		[]string{""},
		[]string{testLayerPath},
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
	container, err := docker.Create(
		"restart_test",
		"echo",
		[]string{"-n", "foobar"},
		[]string{testLayerPath},
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
	container, err := docker.Create(
		"restart_stdin_test",
		"cat",
		[]string{},
		[]string{testLayerPath},
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

	// Default user must be root
	container, err := docker.Create(
		"user_default",
		"id",
		[]string{},
		[]string{testLayerPath},
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
		"user_root",
		"id",
		[]string{},
		[]string{testLayerPath},
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
		"user_uid0",
		"id",
		[]string{},
		[]string{testLayerPath},
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
		"user_uid1",
		"id",
		[]string{},
		[]string{testLayerPath},
		&Config{
			User: "1",
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

	// Set a different user by username
	container, err = docker.Create(
		"user_daemon",
		"id",
		[]string{},
		[]string{testLayerPath},
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

	container1, err := docker.Create(
		"container1",
		"cat",
		[]string{"/dev/zero"},
		[]string{testLayerPath},
		&Config{},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer docker.Destroy(container1)

	container2, err := docker.Create(
		"container2",
		"cat",
		[]string{"/dev/zero"},
		[]string{testLayerPath},
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
	container, err := docker.Create(
		"stdin_test",
		"cat",
		[]string{},
		[]string{testLayerPath},
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
	container, err := docker.Create(
		"tty_test",
		"cat",
		[]string{},
		[]string{testLayerPath},
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
	container, err := docker.Create(
		"env_test",
		"/usr/bin/env",
		[]string{},
		[]string{testLayerPath},
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

func BenchmarkRunSequencial(b *testing.B) {
	docker, err := newTestDocker()
	if err != nil {
		b.Fatal(err)
	}
	for i := 0; i < b.N; i++ {
		container, err := docker.Create(
			fmt.Sprintf("bench_%v", i),
			"echo",
			[]string{"-n", "foo"},
			[]string{testLayerPath},
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

	var tasks []chan error

	for i := 0; i < b.N; i++ {
		complete := make(chan error)
		tasks = append(tasks, complete)
		go func(i int, complete chan error) {
			container, err := docker.Create(
				fmt.Sprintf("bench_%v", i),
				"echo",
				[]string{"-n", "foo"},
				[]string{testLayerPath},
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
