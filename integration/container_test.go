package docker

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/runconfig"
)

func TestKillDifferentUser(t *testing.T) {
	daemon := mkDaemon(t)
	defer nuke(daemon)

	container, _, err := daemon.Create(&runconfig.Config{
		Image:     GetTestImage(daemon).ID,
		Cmd:       []string{"cat"},
		OpenStdin: true,
		User:      "daemon",
	},
		"",
	)
	if err != nil {
		t.Fatal(err)
	}
	defer daemon.Destroy(container)
	// FIXME @shykes: this seems redundant, but is very old, I'm leaving it in case
	// there is a side effect I'm not seeing.
	// defer container.stdin.Close()

	if container.State.IsRunning() {
		t.Errorf("Container shouldn't be running")
	}
	if err := container.Start(); err != nil {
		t.Fatal(err)
	}

	setTimeout(t, "Waiting for the container to be started timed out", 2*time.Second, func() {
		for !container.State.IsRunning() {
			time.Sleep(10 * time.Millisecond)
		}
	})

	setTimeout(t, "read/write assertion timed out", 2*time.Second, func() {
		out, _ := container.StdoutPipe()
		in, _ := container.StdinPipe()
		if err := assertPipe("hello\n", "hello", out, in, 150); err != nil {
			t.Fatal(err)
		}
	})

	if err := container.Kill(); err != nil {
		t.Fatal(err)
	}

	if container.State.IsRunning() {
		t.Errorf("Container shouldn't be running")
	}
	container.State.WaitStop(-1 * time.Second)
	if container.State.IsRunning() {
		t.Errorf("Container shouldn't be running")
	}
	// Try stopping twice
	if err := container.Kill(); err != nil {
		t.Fatal(err)
	}
}

func TestRestartStdin(t *testing.T) {
	daemon := mkDaemon(t)
	defer nuke(daemon)
	container, _, err := daemon.Create(&runconfig.Config{
		Image: GetTestImage(daemon).ID,
		Cmd:   []string{"cat"},

		OpenStdin: true,
	},
		"",
	)
	if err != nil {
		t.Fatal(err)
	}
	defer daemon.Destroy(container)

	stdin, err := container.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := container.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := container.Start(); err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(stdin, "hello world"); err != nil {
		t.Fatal(err)
	}
	if err := stdin.Close(); err != nil {
		t.Fatal(err)
	}
	container.State.WaitStop(-1 * time.Second)
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
	if err := container.Start(); err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(stdin, "hello world #2"); err != nil {
		t.Fatal(err)
	}
	if err := stdin.Close(); err != nil {
		t.Fatal(err)
	}
	container.State.WaitStop(-1 * time.Second)
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

func TestStdin(t *testing.T) {
	daemon := mkDaemon(t)
	defer nuke(daemon)
	container, _, err := daemon.Create(&runconfig.Config{
		Image: GetTestImage(daemon).ID,
		Cmd:   []string{"cat"},

		OpenStdin: true,
	},
		"",
	)
	if err != nil {
		t.Fatal(err)
	}
	defer daemon.Destroy(container)

	stdin, err := container.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := container.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := container.Start(); err != nil {
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
	container.State.WaitStop(-1 * time.Second)
	output, err := ioutil.ReadAll(stdout)
	if err != nil {
		t.Fatal(err)
	}
	if string(output) != "hello world" {
		t.Fatalf("Unexpected output. Expected %s, received: %s", "hello world", string(output))
	}
}

func TestTty(t *testing.T) {
	daemon := mkDaemon(t)
	defer nuke(daemon)
	container, _, err := daemon.Create(&runconfig.Config{
		Image: GetTestImage(daemon).ID,
		Cmd:   []string{"cat"},

		OpenStdin: true,
	},
		"",
	)
	if err != nil {
		t.Fatal(err)
	}
	defer daemon.Destroy(container)

	stdin, err := container.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := container.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := container.Start(); err != nil {
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
	container.State.WaitStop(-1 * time.Second)
	output, err := ioutil.ReadAll(stdout)
	if err != nil {
		t.Fatal(err)
	}
	if string(output) != "hello world" {
		t.Fatalf("Unexpected output. Expected %s, received: %s", "hello world", string(output))
	}
}

func TestEntrypoint(t *testing.T) {
	daemon := mkDaemon(t)
	defer nuke(daemon)
	container, _, err := daemon.Create(
		&runconfig.Config{
			Image:      GetTestImage(daemon).ID,
			Entrypoint: []string{"/bin/echo"},
			Cmd:        []string{"-n", "foobar"},
		},
		"",
	)
	if err != nil {
		t.Fatal(err)
	}
	defer daemon.Destroy(container)
	output, err := container.Output()
	if err != nil {
		t.Fatal(err)
	}
	if string(output) != "foobar" {
		t.Error(string(output))
	}
}

func TestEntrypointNoCmd(t *testing.T) {
	daemon := mkDaemon(t)
	defer nuke(daemon)
	container, _, err := daemon.Create(
		&runconfig.Config{
			Image:      GetTestImage(daemon).ID,
			Entrypoint: []string{"/bin/echo", "foobar"},
		},
		"",
	)
	if err != nil {
		t.Fatal(err)
	}
	defer daemon.Destroy(container)
	output, err := container.Output()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Trim(string(output), "\r\n") != "foobar" {
		t.Error(string(output))
	}
}

func BenchmarkRunSequential(b *testing.B) {
	daemon := mkDaemon(b)
	defer nuke(daemon)
	for i := 0; i < b.N; i++ {
		container, _, err := daemon.Create(&runconfig.Config{
			Image: GetTestImage(daemon).ID,
			Cmd:   []string{"echo", "-n", "foo"},
		},
			"",
		)
		if err != nil {
			b.Fatal(err)
		}
		defer daemon.Destroy(container)
		output, err := container.Output()
		if err != nil {
			b.Fatal(err)
		}
		if string(output) != "foo" {
			b.Fatalf("Unexpected output: %s", output)
		}
		if err := daemon.Destroy(container); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRunParallel(b *testing.B) {
	daemon := mkDaemon(b)
	defer nuke(daemon)

	var tasks []chan error

	for i := 0; i < b.N; i++ {
		complete := make(chan error)
		tasks = append(tasks, complete)
		go func(i int, complete chan error) {
			container, _, err := daemon.Create(&runconfig.Config{
				Image: GetTestImage(daemon).ID,
				Cmd:   []string{"echo", "-n", "foo"},
			},
				"",
			)
			if err != nil {
				complete <- err
				return
			}
			defer daemon.Destroy(container)
			if err := container.Start(); err != nil {
				complete <- err
				return
			}
			if _, err := container.State.WaitStop(15 * time.Second); err != nil {
				complete <- err
				return
			}
			// if string(output) != "foo" {
			// 	complete <- fmt.Errorf("Unexecpted output: %v", string(output))
			// }
			if err := daemon.Destroy(container); err != nil {
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
	tmpDir, err := ioutil.TempDir("", "docker-test-container")
	if err != nil {
		t.Fatal(err)
	}
	return tmpDir
}

func TestBindMounts(t *testing.T) {
	eng := NewTestEngine(t)
	r := mkDaemonFromEngine(eng, t)
	defer r.Nuke()

	tmpDir := tempDir(t)
	defer os.RemoveAll(tmpDir)
	writeFile(path.Join(tmpDir, "touch-me"), "", t)

	// Test reading from a read-only bind mount
	stdout, _ := runContainer(eng, r, []string{"-v", fmt.Sprintf("%s:/tmp:ro", tmpDir), "_", "ls", "/tmp"}, t)
	if !strings.Contains(stdout, "touch-me") {
		t.Fatal("Container failed to read from bind mount")
	}

	// test writing to bind mount
	runContainer(eng, r, []string{"-v", fmt.Sprintf("%s:/tmp:rw", tmpDir), "_", "touch", "/tmp/holla"}, t)
	readFile(path.Join(tmpDir, "holla"), t) // Will fail if the file doesn't exist

	// test mounting to an illegal destination directory
	if _, err := runContainer(eng, r, []string{"-v", fmt.Sprintf("%s:.", tmpDir), "_", "ls", "."}, nil); err == nil {
		t.Fatal("Container bind mounted illegal directory")
	}

	// test mount a file
	runContainer(eng, r, []string{"-v", fmt.Sprintf("%s/holla:/tmp/holla:rw", tmpDir), "_", "sh", "-c", "echo -n 'yotta' > /tmp/holla"}, t)
	content := readFile(path.Join(tmpDir, "holla"), t) // Will fail if the file doesn't exist
	if content != "yotta" {
		t.Fatal("Container failed to write to bind mount file")
	}
}
