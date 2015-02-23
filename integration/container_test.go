package docker

import (
	"io"
	"io/ioutil"
	"testing"
	"time"

	"github.com/docker/docker/runconfig"
)

func TestRestartStdin(t *testing.T) {
	daemon := mkDaemon(t)
	defer nuke(daemon)
	container, _, err := daemon.Create(&runconfig.Config{
		Image: GetTestImage(daemon).ID,
		Cmd:   []string{"cat"},

		OpenStdin: true,
	},
		&runconfig.HostConfig{},
		"",
	)
	if err != nil {
		t.Fatal(err)
	}
	defer daemon.Rm(container)

	stdin := container.StdinPipe()
	stdout := container.StdoutPipe()
	if err := container.Start(); err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(stdin, "hello world"); err != nil {
		t.Fatal(err)
	}
	if err := stdin.Close(); err != nil {
		t.Fatal(err)
	}
	container.WaitStop(-1 * time.Second)
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
	stdin = container.StdinPipe()
	stdout = container.StdoutPipe()
	if err := container.Start(); err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(stdin, "hello world #2"); err != nil {
		t.Fatal(err)
	}
	if err := stdin.Close(); err != nil {
		t.Fatal(err)
	}
	container.WaitStop(-1 * time.Second)
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
		&runconfig.HostConfig{},
		"",
	)
	if err != nil {
		t.Fatal(err)
	}
	defer daemon.Rm(container)

	stdin := container.StdinPipe()
	stdout := container.StdoutPipe()
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
	container.WaitStop(-1 * time.Second)
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
		&runconfig.HostConfig{},
		"",
	)
	if err != nil {
		t.Fatal(err)
	}
	defer daemon.Rm(container)

	stdin := container.StdinPipe()
	stdout := container.StdoutPipe()
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
	container.WaitStop(-1 * time.Second)
	output, err := ioutil.ReadAll(stdout)
	if err != nil {
		t.Fatal(err)
	}
	if string(output) != "hello world" {
		t.Fatalf("Unexpected output. Expected %s, received: %s", "hello world", string(output))
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
			&runconfig.HostConfig{},
			"",
		)
		if err != nil {
			b.Fatal(err)
		}
		defer daemon.Rm(container)
		output, err := container.Output()
		if err != nil {
			b.Fatal(err)
		}
		if string(output) != "foo" {
			b.Fatalf("Unexpected output: %s", output)
		}
		if err := daemon.Rm(container); err != nil {
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
				&runconfig.HostConfig{},
				"",
			)
			if err != nil {
				complete <- err
				return
			}
			defer daemon.Rm(container)
			if err := container.Start(); err != nil {
				complete <- err
				return
			}
			if _, err := container.WaitStop(15 * time.Second); err != nil {
				complete <- err
				return
			}
			// if string(output) != "foo" {
			// 	complete <- fmt.Errorf("Unexecpted output: %v", string(output))
			// }
			if err := daemon.Rm(container); err != nil {
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
