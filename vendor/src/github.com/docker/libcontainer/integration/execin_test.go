package integration

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/docker/libcontainer"
	"github.com/docker/libcontainer/cgroups"
	"github.com/docker/libcontainer/cgroups/fs"
	"github.com/docker/libcontainer/cgroups/systemd"
	"github.com/docker/libcontainer/namespaces"
)

func TestExecIn(t *testing.T) {
	if testing.Short() {
		return
	}

	rootfs, err := newRootFs()
	if err != nil {
		t.Fatal(err)
	}
	defer remove(rootfs)

	config := newTemplateConfig(rootfs)
	if err := writeConfig(config); err != nil {
		t.Fatalf("failed to write config %s", err)
	}

	containerCmd, statePath, containerErr := startLongRunningContainer(config)
	defer func() {
		// kill the container
		if containerCmd.Process != nil {
			containerCmd.Process.Kill()
		}
		if err := <-containerErr; err != nil {
			t.Fatal(err)
		}
	}()

	// start the exec process
	state, err := libcontainer.GetState(statePath)
	if err != nil {
		t.Fatalf("failed to get state %s", err)
	}
	buffers := newStdBuffers()
	execErr := make(chan error, 1)
	execConfig := &libcontainer.ExecConfig{
		Container: config,
		State:     state,
	}
	var execWait sync.WaitGroup
	execWait.Add(1)
	go func() {
		_, err := namespaces.ExecIn(execConfig, []string{"ps"},
			os.Args[0], "exec", buffers.Stdin, buffers.Stdout, buffers.Stderr,
			"", func(cmd *exec.Cmd) {
				pid := cmd.Process.Pid
				assertCgroups(t, state.CgroupPaths, pid)
				execWait.Done()
			})
		execWait.Wait()
		execErr <- err
	}()
	if err := <-execErr; err != nil {
		t.Fatalf("exec finished with error %s", err)
	}

	out := buffers.Stdout.String()
	if !strings.Contains(out, "sleep 10") || !strings.Contains(out, "ps") {
		t.Fatalf("unexpected running process, output %q", out)
	}
}

func TestExecInCgroup(t *testing.T) {
	if testing.Short() {
		return
	}

	rootfs, err := newRootFs()
	if err != nil {
		t.Fatal(err)
	}
	defer remove(rootfs)
	// write our test script for spawning background process
	err = ioutil.WriteFile(filepath.Join(rootfs, "test-orphan"), []byte(`
	exec 0>&-
	exec 1>&-
	exec 2>&-
	(sleep 5 &)
	`), 755)
	if err != nil {
		t.Fatalf("failed to write test script %s", err)
	}

	config := newTemplateConfig(rootfs)
	if err := writeConfig(config); err != nil {
		t.Fatalf("failed to write config %s", err)
	}

	containerCmd, statePath, containerErr := startLongRunningContainer(config)
	defer func() {
		// kill the container
		if containerCmd.Process != nil {
			containerCmd.Process.Kill()
		}
		if err := <-containerErr; err != nil {
			t.Fatal(err)
		}
	}()

	// start the exec process
	state, err := libcontainer.GetState(statePath)
	if err != nil {
		t.Fatalf("failed to get state %s", err)
	}
	buffers := newStdBuffers()
	execErr := make(chan error, 1)
	execConfig := &libcontainer.ExecConfig{
		Container: config,
		State:     state,
		Cgroups:   &cgroups.Cgroup{Name: "exec-123456"},
	}
	var execWait sync.WaitGroup
	execWait.Add(1)
	go func() {
		_, err := namespaces.ExecIn(execConfig, []string{"sh", "-c", "/test-orphan"},
			os.Args[0], "exec", buffers.Stdin, buffers.Stdout, buffers.Stderr,
			"", func(cmd *exec.Cmd) {
				defer execWait.Done()
				time.Sleep(100 * time.Millisecond) // wait for the background task to spawn
				pids, err := getPids(execConfig.Cgroups)
				// sleep should be contained here
				if err != nil || len(pids) == 0 {
					t.Errorf("failed to setup cgroup for the process")
				}
			})
		execWait.Wait()
		execErr <- err
	}()
	if err := <-execErr; err != nil {
		t.Fatalf("exec finished with error %s", err)
	}

	// exec's cgroups should be cleaned up when finished
	for _, v := range state.CgroupPaths {
		p := filepath.Join(v, "exec-123456")
		if _, err := os.Stat(p); err == nil {
			t.Errorf("failed to removed cgroups %q", p)
		}
	}
}

func TestExecInRlimit(t *testing.T) {
	if testing.Short() {
		return
	}

	rootfs, err := newRootFs()
	if err != nil {
		t.Fatal(err)
	}
	defer remove(rootfs)

	config := newTemplateConfig(rootfs)
	if err := writeConfig(config); err != nil {
		t.Fatalf("failed to write config %s", err)
	}

	containerCmd, statePath, containerErr := startLongRunningContainer(config)
	defer func() {
		// kill the container
		if containerCmd.Process != nil {
			containerCmd.Process.Kill()
		}
		if err := <-containerErr; err != nil {
			t.Fatal(err)
		}
	}()

	// start the exec process
	state, err := libcontainer.GetState(statePath)
	if err != nil {
		t.Fatalf("failed to get state %s", err)
	}
	buffers := newStdBuffers()
	execErr := make(chan error, 1)
	execConfig := &libcontainer.ExecConfig{
		Container: config,
		State:     state,
	}
	go func() {
		_, err := namespaces.ExecIn(execConfig, []string{"/bin/sh", "-c", "ulimit -n"},
			os.Args[0], "exec", buffers.Stdin, buffers.Stdout, buffers.Stderr,
			"", nil)
		execErr <- err
	}()
	if err := <-execErr; err != nil {
		t.Fatalf("exec finished with error %s", err)
	}

	out := buffers.Stdout.String()
	if limit := strings.TrimSpace(out); limit != "1024" {
		t.Fatalf("expected rlimit to be 1024, got %s", limit)
	}
}

// start a long-running container so we have time to inspect execin processes
func startLongRunningContainer(config *libcontainer.Config) (*exec.Cmd, string, chan error) {
	containerErr := make(chan error, 1)
	containerCmd := &exec.Cmd{}
	var statePath string

	createCmd := func(container *libcontainer.Config, console, dataPath, init string,
		pipe *os.File, args []string) *exec.Cmd {
		containerCmd = namespaces.DefaultCreateCommand(container, console, dataPath, init, pipe, args)
		statePath = dataPath
		return containerCmd
	}

	var containerStart sync.WaitGroup
	containerStart.Add(1)
	go func() {
		buffers := newStdBuffers()
		_, err := namespaces.Exec(config,
			buffers.Stdin, buffers.Stdout, buffers.Stderr,
			"", config.RootFs, []string{"sleep", "10"},
			createCmd, containerStart.Done)
		containerErr <- err
	}()
	containerStart.Wait()

	return containerCmd, statePath, containerErr
}

// asserts that process pid joined the cgroups paths. Non-existing cgroup paths
// are ignored
func assertCgroups(t *testing.T, paths map[string]string, pid int) {
	for _, p := range paths {
		if _, err := os.Stat(p); err != nil {
			continue
		}
		pids, err := cgroups.ReadProcsFile(p)
		if err != nil {
			t.Errorf("failed to read procs in %q", p)
			continue
		}
		var found bool
		for _, procPID := range pids {
			if procPID == pid {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("cgroups %q does not contain exec pid %d", p, pid)
		}
	}
}

// getPids return the current pids, sorted in the cgroup
func getPids(c *cgroups.Cgroup) (pids []int, err error) {
	if systemd.UseSystemd() {
		pids, err = systemd.GetPids(c)
	} else {
		pids, err = fs.GetPids(c)
	}
	sort.Ints(pids)
	return pids, err
}
