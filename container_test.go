package docker

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestIdFormat(t *testing.T) {
	runtime, err := newTestRuntime()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(runtime)
	container1, err := runtime.Create(
		&Config{
			Image:  GetTestImage(runtime).Id,
			Cmd:    []string{"/bin/sh", "-c", "echo hello world"},
			Memory: 33554432,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	match, err := regexp.Match("^[0-9a-f]{64}$", []byte(container1.Id))
	if err != nil {
		t.Fatal(err)
	}
	if !match {
		t.Fatalf("Invalid container ID: %s", container1.Id)
	}
}

func TestMultipleAttachRestart(t *testing.T) {
	runtime, err := newTestRuntime()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(runtime)
	container, err := runtime.Create(
		&Config{
			Image: GetTestImage(runtime).Id,
			Cmd: []string{"/bin/sh", "-c",
				"i=1; while [ $i -le 5 ]; do i=`expr $i + 1`;  echo hello; done"},
			Memory: 33554432,
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
	if err := container.Start(); err != nil {
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

	if err := container.Stop(); err != nil {
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
	if err := container.Start(); err != nil {
		t.Fatal(err)
	}
	timeout := make(chan bool)
	go func() {
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
		timeout <- false
	}()
	go func() {
		time.Sleep(3 * time.Second)
		timeout <- true
	}()
	if <-timeout {
		t.Fatalf("Timeout reading from the process")
	}
}

func TestCommitRun(t *testing.T) {
	runtime, err := newTestRuntime()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(runtime)
	container1, err := runtime.Create(
		&Config{
			Image:  GetTestImage(runtime).Id,
			Cmd:    []string{"/bin/sh", "-c", "echo hello > /world"},
			Memory: 33554432,
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
	img, err := runtime.graph.Create(rwTar, container1, "unit test commited image")
	if err != nil {
		t.Error(err)
	}

	// FIXME: Make a TestCommit that stops here and check docker.root/layers/img.id/world

	container2, err := runtime.Create(
		&Config{
			Image:  img.Id,
			Memory: 33554432,
			Cmd:    []string{"cat", "/world"},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Destroy(container2)
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
	runtime, err := newTestRuntime()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(runtime)
	container, err := runtime.Create(
		&Config{
			Image:  GetTestImage(runtime).Id,
			Memory: 33554432,
			Cmd:    []string{"ls", "-al"},
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
	runtime, err := newTestRuntime()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(runtime)
	container, err := runtime.Create(
		&Config{
			Image: GetTestImage(runtime).Id,
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

func TestKill(t *testing.T) {
	runtime, err := newTestRuntime()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(runtime)
	container, err := runtime.Create(&Config{
		Image: GetTestImage(runtime).Id,
		Cmd:   []string{"cat", "/dev/zero"},
	},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Destroy(container)

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
	runtime, err := newTestRuntime()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(runtime)

	trueContainer, err := runtime.Create(&Config{

		Image: GetTestImage(runtime).Id,
		Cmd:   []string{"/bin/true", ""},
	},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Destroy(trueContainer)
	if err := trueContainer.Run(); err != nil {
		t.Fatal(err)
	}

	falseContainer, err := runtime.Create(&Config{
		Image: GetTestImage(runtime).Id,
		Cmd:   []string{"/bin/false", ""},
	},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Destroy(falseContainer)
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
	runtime, err := newTestRuntime()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(runtime)
	container, err := runtime.Create(&Config{
		Image: GetTestImage(runtime).Id,
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
	runtime, err := newTestRuntime()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(runtime)
	container, err := runtime.Create(&Config{
		Image: GetTestImage(runtime).Id,
		Cmd:   []string{"cat"},

		OpenStdin: true,
	},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Destroy(container)

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
	runtime, err := newTestRuntime()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(runtime)

	// Default user must be root
	container, err := runtime.Create(&Config{
		Image: GetTestImage(runtime).Id,
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
	container, err = runtime.Create(&Config{
		Image: GetTestImage(runtime).Id,
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
	container, err = runtime.Create(&Config{
		Image: GetTestImage(runtime).Id,
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
	container, err = runtime.Create(&Config{
		Image: GetTestImage(runtime).Id,
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
	container, err = runtime.Create(&Config{
		Image: GetTestImage(runtime).Id,
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
	runtime, err := newTestRuntime()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(runtime)

	container1, err := runtime.Create(&Config{
		Image: GetTestImage(runtime).Id,
		Cmd:   []string{"cat", "/dev/zero"},
	},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Destroy(container1)

	container2, err := runtime.Create(&Config{
		Image: GetTestImage(runtime).Id,
		Cmd:   []string{"cat", "/dev/zero"},
	},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Destroy(container2)

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
	runtime, err := newTestRuntime()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(runtime)
	container, err := runtime.Create(&Config{
		Image: GetTestImage(runtime).Id,
		Cmd:   []string{"cat"},

		OpenStdin: true,
	},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Destroy(container)

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
	runtime, err := newTestRuntime()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(runtime)
	container, err := runtime.Create(&Config{
		Image: GetTestImage(runtime).Id,
		Cmd:   []string{"cat"},

		OpenStdin: true,
	},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Destroy(container)

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
	runtime, err := newTestRuntime()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(runtime)
	container, err := runtime.Create(&Config{
		Image: GetTestImage(runtime).Id,
		Cmd:   []string{"/usr/bin/env"},
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
	runtime, err := newTestRuntime()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(runtime)
	// Memory is allocated randomly for testing
	rand.Seed(time.Now().UTC().UnixNano())
	memMin := 33554432
	memMax := 536870912
	mem := memMin + rand.Intn(memMax-memMin)
	container, err := runtime.Create(&Config{
		Image: GetTestImage(runtime).Id,
		Cmd:   []string{"/bin/true"},

		Hostname: "foobar",
		Memory:   int64(mem),
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
	runtime, err := newTestRuntime()
	if err != nil {
		b.Fatal(err)
	}
	defer nuke(runtime)
	for i := 0; i < b.N; i++ {
		container, err := runtime.Create(&Config{
			Image: GetTestImage(runtime).Id,
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
			b.Fatalf("Unexecpted output: %v", string(output))
		}
		if err := runtime.Destroy(container); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRunParallel(b *testing.B) {
	runtime, err := newTestRuntime()
	if err != nil {
		b.Fatal(err)
	}
	defer nuke(runtime)

	var tasks []chan error

	for i := 0; i < b.N; i++ {
		complete := make(chan error)
		tasks = append(tasks, complete)
		go func(i int, complete chan error) {
			container, err := runtime.Create(&Config{
				Image: GetTestImage(runtime).Id,
				Cmd:   []string{"echo", "-n", "foo"},
			},
			)
			if err != nil {
				complete <- err
				return
			}
			defer runtime.Destroy(container)
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
