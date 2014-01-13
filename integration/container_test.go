package docker

import (
	"io"
	"io/ioutil"
	"os"
	"path"
	"reflect"
	"regexp"
	"strings"
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
				&runconfig.HostConfig{},
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
			if _, err := container.WaitStop(15 * time.Second); err != nil {
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

func TestGetCgroupSubsysem(t *testing.T) {
	eng := NewTestEngine(t)
	runtime := mkRuntimeFromEngine(eng, t)
	defer nuke(runtime)
	config, hc, _, err := docker.ParseRun([]string{"-i", "-lxc-conf", "lxc.cgroup.cpuset.cpus=0,1", unitTestImageID, "/bin/cat"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	jobCreate := eng.Job("create")
	if err := jobCreate.ImportEnv(config); err != nil {
		t.Fatal(err)
	}
	var id string
	jobCreate.Stdout.AddString(&id)
	if err := jobCreate.Run(); err != nil {
		t.Fatal(err)
	}
	jobStart := eng.Job("start", id)
	if err := jobStart.ImportEnv(hc); err != nil {
		t.Fatal(err)
	}
	if err := jobStart.Run(); err != nil {
		t.Fatal(err)
	}
	// FIXME: this hack can be removed once Wait is a job
	container := runtime.Get(id)
	if container == nil {
		t.Fatalf("Couldn't retrieve container %s from runtime", id)
	}

	defer runtime.Destroy(container)

	cStdin, err := container.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}

	// Give some time to the process to start
	container.WaitTimeout(500 * time.Millisecond)

	if !container.State.IsRunning() {
		t.Errorf("Container should be running")
	}

	out, err := container.GetCgroupSubsysem("cpuset.cpus")
	if err != nil {
		t.Fatal(err)
	}

	if out != "0-1" {
		t.Fatalf("Except output is 0-1, but actual is %s", out)
	}

	// Try to avoid the timeout in destroy. Best effort, don't check error
	cStdin.Close()
	container.WaitTimeout(2 * time.Second)
}

func TestSetCgroupSubsysem(t *testing.T) {
	eng := NewTestEngine(t)
	runtime := mkRuntimeFromEngine(eng, t)
	defer nuke(runtime)
	config, hc, _, err := docker.ParseRun([]string{"-i", "-lxc-conf", "lxc.cgroup.cpuset.cpus=0", unitTestImageID, "/bin/cat"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	jobCreate := eng.Job("create")
	if err := jobCreate.ImportEnv(config); err != nil {
		t.Fatal(err)
	}
	var id string
	jobCreate.Stdout.AddString(&id)
	if err := jobCreate.Run(); err != nil {
		t.Fatal(err)
	}
	jobStart := eng.Job("start", id)
	if err := jobStart.ImportEnv(hc); err != nil {
		t.Fatal(err)
	}
	if err := jobStart.Run(); err != nil {
		t.Fatal(err)
	}
	// FIXME: this hack can be removed once Wait is a job
	container := runtime.Get(id)
	if container == nil {
		t.Fatalf("Couldn't retrieve container %s from runtime", id)
	}

	defer runtime.Destroy(container)

	cStdin, err := container.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}

	// Give some time to the process to start
	container.WaitTimeout(500 * time.Millisecond)

	if !container.State.IsRunning() {
		t.Errorf("Container should be running")
	}

	if out, err := container.SetCgroupSubsysem("cpuset.cpus", "1,2"); err != nil {
		t.Fatal(err)
		if out != "" {
			t.Fatalf("Except output is empty, but actual is %s", out)
		}
	}

	if out, err := container.GetCgroupSubsysem("cpuset.cpus"); err != nil {
		t.Fatal(err)
		if out != "1-2" {
			t.Fatalf("Except output is 1-2, but actual is %s", out)
		}
	}

	// Try to avoid the timeout in destroy. Best effort, don't check error
	cStdin.Close()
	container.WaitTimeout(2 * time.Second)
}

func TestAddLXCConfig(t *testing.T) {
	eng := NewTestEngine(t)
	runtime := mkRuntimeFromEngine(eng, t)
	defer nuke(runtime)
	config, hc, _, err := runconfig.Parse([]string{"-i", "-lxc-conf", "lxc.cgroup.cpuset.cpus=0", unitTestImageID, "/bin/cat"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	jobCreate := eng.Job("create")
	if err := jobCreate.ImportEnv(config); err != nil {
		t.Fatal(err)
	}
	var id string
	jobCreate.Stdout.AddString(&id)
	if err := jobCreate.Run(); err != nil {
		t.Fatal(err)
	}
	jobStart := eng.Job("start", id)
	if err := jobStart.ImportEnv(hc); err != nil {
		t.Fatal(err)
	}
	if err := jobStart.Run(); err != nil {
		t.Fatal(err)
	}
	// FIXME: this hack can be removed once Wait is a job
	container := runtime.Get(id)
	if container == nil {
		t.Fatalf("Couldn't retrieve container %s from runtime", id)
	}

	defer runtime.Destroy(container)

	cStdin, err := container.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}

	// Give some time to the process to start
	container.WaitTimeout(500 * time.Millisecond)

	if !container.State.IsRunning() {
		t.Errorf("Container should be running")
	}

	// Can not access unexport field hostConfig, So use reflect to test result.
	r := reflect.ValueOf(container)
	f := reflect.Indirect(r).FieldByName("hostConfig").Elem().FieldByName("LxcConf")

	if f.Len() != 1 {
		t.Fatalf("Except length is 1, but actual is %d", f.Len())
	}

	if err := container.AddLXCConfig("cpuset.cpus", "1,2"); err != nil {
		t.Fatal(err)
	}

	if err := container.AddLXCConfig("blkio.weight", "500"); err != nil {
		t.Fatal(err)
	}

	if f.Len() != 2 {
		t.Fatalf("Except length is 2, but actual is %d", f.Len())
	}

	for i := 0; i < f.Len(); i++ {
		pair := f.Index(i)
		key := pair.FieldByName("Key").String()
		value := pair.FieldByName("Value").String()
		if key == "cpuset.cpus" && value != "1,2" {
			t.Fatalf("Unmatched pair")
		}
		if key == "blkio.weight" && value != "500" {
			t.Fatalf("Unmatched pair")
		}
	}

	// Try to avoid the timeout in destroy. Best effort, don't check error
	cStdin.Close()
	container.WaitTimeout(2 * time.Second)
}
