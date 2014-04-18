package docker

import (
	"bufio"
	"fmt"
	"github.com/dotcloud/docker/runconfig"
	"github.com/dotcloud/docker/utils"
	"io"
	"io/ioutil"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestCpuShares(t *testing.T) {
	_, err1 := os.Stat("/sys/fs/cgroup/cpuacct,cpu")
	_, err2 := os.Stat("/sys/fs/cgroup/cpu,cpuacct")
	if err1 == nil || err2 == nil {
		t.Skip("Fixme. Setting cpu cgroup shares doesn't work in dind on a Fedora host.  The lxc utils are confused by the cpu,cpuacct mount.")
	}
	daemon := mkDaemon(t)
	defer nuke(daemon)
	container, _, _ := mkContainer(daemon, []string{"-m", "33554432", "-c", "1000", "-i", "_", "/bin/cat"}, t)
	defer daemon.Destroy(container)

	cStdin, err := container.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}

	if err := container.Start(); err != nil {
		t.Fatal(err)
	}

	// Give some time to the process to start
	container.WaitTimeout(500 * time.Millisecond)

	if !container.State.IsRunning() {
		t.Errorf("Container should be running")
	}
	if err := container.Start(); err != nil {
		t.Fatalf("A running container should be able to be started")
	}

	// Try to avoid the timeout in destroy. Best effort, don't check error
	cStdin.Close()
	container.WaitTimeout(2 * time.Second)
}

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
	container.Wait()
	if container.State.IsRunning() {
		t.Errorf("Container shouldn't be running")
	}
	// Try stopping twice
	if err := container.Kill(); err != nil {
		t.Fatal(err)
	}
}

func TestRestart(t *testing.T) {
	daemon := mkDaemon(t)
	defer nuke(daemon)
	container, _, err := daemon.Create(&runconfig.Config{
		Image: GetTestImage(daemon).ID,
		Cmd:   []string{"echo", "-n", "foobar"},
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
	if err := container.Start(); err != nil {
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
	os.Setenv("TRUE", "false")
	os.Setenv("TRICKY", "tri\ncky\n")
	daemon := mkDaemon(t)
	defer nuke(daemon)
	config, _, _, err := runconfig.Parse([]string{"-e=FALSE=true", "-e=TRUE", "-e=TRICKY", GetTestImage(daemon).ID, "env"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	container, _, err := daemon.Create(config, "")
	if err != nil {
		t.Fatal(err)
	}
	defer daemon.Destroy(container)

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
		"HOSTNAME=" + utils.TruncateID(container.ID),
		"FALSE=true",
		"TRUE=false",
		"TRICKY=tri",
		"cky",
		"",
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
			if err := container.WaitTimeout(15 * time.Second); err != nil {
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

// Test for #1737
func TestCopyVolumeUidGid(t *testing.T) {
	eng := NewTestEngine(t)
	r := mkDaemonFromEngine(eng, t)
	defer r.Nuke()

	// Add directory not owned by root
	container1, _, _ := mkContainer(r, []string{"_", "/bin/sh", "-c", "mkdir -p /hello && touch /hello/test.txt && chown daemon.daemon /hello"}, t)
	defer r.Destroy(container1)

	if container1.State.IsRunning() {
		t.Errorf("Container shouldn't be running")
	}
	if err := container1.Run(); err != nil {
		t.Fatal(err)
	}
	if container1.State.IsRunning() {
		t.Errorf("Container shouldn't be running")
	}

	img, err := r.Commit(container1, "", "", "unit test commited image", "", nil)
	if err != nil {
		t.Error(err)
	}

	// Test that the uid and gid is copied from the image to the volume
	tmpDir1 := tempDir(t)
	defer os.RemoveAll(tmpDir1)
	stdout1, _ := runContainer(eng, r, []string{"-v", "/hello", img.ID, "stat", "-c", "%U %G", "/hello"}, t)
	if !strings.Contains(stdout1, "daemon daemon") {
		t.Fatal("Container failed to transfer uid and gid to volume")
	}
}

// Test for #1582
func TestCopyVolumeContent(t *testing.T) {
	eng := NewTestEngine(t)
	r := mkDaemonFromEngine(eng, t)
	defer r.Nuke()

	// Put some content in a directory of a container and commit it
	container1, _, _ := mkContainer(r, []string{"_", "/bin/sh", "-c", "mkdir -p /hello/local && echo hello > /hello/local/world"}, t)
	defer r.Destroy(container1)

	if container1.State.IsRunning() {
		t.Errorf("Container shouldn't be running")
	}
	if err := container1.Run(); err != nil {
		t.Fatal(err)
	}
	if container1.State.IsRunning() {
		t.Errorf("Container shouldn't be running")
	}

	img, err := r.Commit(container1, "", "", "unit test commited image", "", nil)
	if err != nil {
		t.Error(err)
	}

	// Test that the content is copied from the image to the volume
	tmpDir1 := tempDir(t)
	defer os.RemoveAll(tmpDir1)
	stdout1, _ := runContainer(eng, r, []string{"-v", "/hello", img.ID, "find", "/hello"}, t)
	if !(strings.Contains(stdout1, "/hello/local/world") && strings.Contains(stdout1, "/hello/local")) {
		t.Fatal("Container failed to transfer content to volume")
	}
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

// Test that restarting a container with a volume does not create a new volume on restart. Regression test for #819.
func TestRestartWithVolumes(t *testing.T) {
	daemon := mkDaemon(t)
	defer nuke(daemon)

	container, _, err := daemon.Create(&runconfig.Config{
		Image:   GetTestImage(daemon).ID,
		Cmd:     []string{"echo", "-n", "foobar"},
		Volumes: map[string]struct{}{"/test": {}},
	},
		"",
	)
	if err != nil {
		t.Fatal(err)
	}
	defer daemon.Destroy(container)

	for key := range container.Config.Volumes {
		if key != "/test" {
			t.Fail()
		}
	}

	_, err = container.Output()
	if err != nil {
		t.Fatal(err)
	}

	expected := container.Volumes["/test"]
	if expected == "" {
		t.Fail()
	}
	// Run the container again to verify the volume path persists
	_, err = container.Output()
	if err != nil {
		t.Fatal(err)
	}

	actual := container.Volumes["/test"]
	if expected != actual {
		t.Fatalf("Expected volume path: %s Actual path: %s", expected, actual)
	}
}

func TestContainerNetwork(t *testing.T) {
	daemon := mkDaemon(t)
	defer nuke(daemon)
	container, _, err := daemon.Create(
		&runconfig.Config{
			Image: GetTestImage(daemon).ID,
			// If I change this to ping 8.8.8.8 it fails.  Any idea why? - timthelion
			Cmd: []string{"ping", "-c", "1", "127.0.0.1"},
		},
		"",
	)
	if err != nil {
		t.Fatal(err)
	}
	defer daemon.Destroy(container)
	if err := container.Run(); err != nil {
		t.Fatal(err)
	}
	if code := container.State.GetExitCode(); code != 0 {
		t.Fatalf("Unexpected ping 127.0.0.1 exit code %d (expected 0)", code)
	}
}

// Issue #4681
func TestLoopbackFunctionsWhenNetworkingIsDissabled(t *testing.T) {
	daemon := mkDaemon(t)
	defer nuke(daemon)
	container, _, err := daemon.Create(
		&runconfig.Config{
			Image:           GetTestImage(daemon).ID,
			Cmd:             []string{"ping", "-c", "1", "127.0.0.1"},
			NetworkDisabled: true,
		},
		"",
	)
	if err != nil {
		t.Fatal(err)
	}
	defer daemon.Destroy(container)
	if err := container.Run(); err != nil {
		t.Fatal(err)
	}
	if code := container.State.GetExitCode(); code != 0 {
		t.Fatalf("Unexpected ping 127.0.0.1 exit code %d (expected 0)", code)
	}
}

func TestOnlyLoopbackExistsWhenUsingDisableNetworkOption(t *testing.T) {
	eng := NewTestEngine(t)
	daemon := mkDaemonFromEngine(eng, t)
	defer nuke(daemon)

	config, hc, _, err := runconfig.Parse([]string{"-n=false", GetTestImage(daemon).ID, "ip", "addr", "show", "up"}, nil)
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
	// FIXME: this hack can be removed once Wait is a job
	c := daemon.Get(id)
	if c == nil {
		t.Fatalf("Couldn't retrieve container %s from daemon", id)
	}
	stdout, err := c.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}

	jobStart := eng.Job("start", id)
	if err := jobStart.ImportEnv(hc); err != nil {
		t.Fatal(err)
	}
	if err := jobStart.Run(); err != nil {
		t.Fatal(err)
	}

	c.WaitTimeout(500 * time.Millisecond)
	c.Wait()
	output, err := ioutil.ReadAll(stdout)
	if err != nil {
		t.Fatal(err)
	}

	interfaces := regexp.MustCompile(`(?m)^[0-9]+: [a-zA-Z0-9]+`).FindAllString(string(output), -1)
	if len(interfaces) != 1 {
		t.Fatalf("Wrong interface count in test container: expected [*: lo], got %s", interfaces)
	}
	if !strings.HasSuffix(interfaces[0], ": lo") {
		t.Fatalf("Wrong interface in test container: expected [*: lo], got %s", interfaces)
	}
}

func TestPrivilegedCanMknod(t *testing.T) {
	eng := NewTestEngine(t)
	daemon := mkDaemonFromEngine(eng, t)
	defer daemon.Nuke()
	if output, err := runContainer(eng, daemon, []string{"--privileged", "_", "sh", "-c", "mknod /tmp/sda b 8 0 && echo ok"}, t); output != "ok\n" {
		t.Fatalf("Could not mknod into privileged container %s %v", output, err)
	}
}

func TestPrivilegedCanMount(t *testing.T) {
	eng := NewTestEngine(t)
	daemon := mkDaemonFromEngine(eng, t)
	defer daemon.Nuke()
	if output, _ := runContainer(eng, daemon, []string{"--privileged", "_", "sh", "-c", "mount -t tmpfs none /tmp && echo ok"}, t); output != "ok\n" {
		t.Fatal("Could not mount into privileged container")
	}
}

func TestUnprivilegedCanMknod(t *testing.T) {
	eng := NewTestEngine(t)
	daemon := mkDaemonFromEngine(eng, t)
	defer daemon.Nuke()
	if output, _ := runContainer(eng, daemon, []string{"_", "sh", "-c", "mknod /tmp/sda b 8 0 && echo ok"}, t); output != "ok\n" {
		t.Fatal("Couldn't mknod into secure container")
	}
}

func TestUnprivilegedCannotMount(t *testing.T) {
	eng := NewTestEngine(t)
	daemon := mkDaemonFromEngine(eng, t)
	defer daemon.Nuke()
	if output, _ := runContainer(eng, daemon, []string{"_", "sh", "-c", "mount -t tmpfs none /tmp || echo ok"}, t); output != "ok\n" {
		t.Fatal("Could mount into secure container")
	}
}
