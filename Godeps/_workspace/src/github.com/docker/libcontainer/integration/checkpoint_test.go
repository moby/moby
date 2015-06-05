package integration

import (
	"bytes"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/docker/libcontainer"
)

func TestCheckpoint(t *testing.T) {
	if testing.Short() {
		return
	}
	root, err := newTestRoot()
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)

	rootfs, err := newRootfs()
	if err != nil {
		t.Fatal(err)
	}
	defer remove(rootfs)

	config := newTemplateConfig(rootfs)

	factory, err := libcontainer.New(root, libcontainer.Cgroupfs)

	if err != nil {
		t.Fatal(err)
	}

	container, err := factory.Create("test", config)
	if err != nil {
		t.Fatal(err)
	}
	defer container.Destroy()

	stdinR, stdinW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer

	pconfig := libcontainer.Process{
		Args:  []string{"cat"},
		Env:   standardEnvironment,
		Stdin: stdinR,
		Stdout: &stdout,
	}

	err = container.Start(&pconfig)
	stdinR.Close()
	defer stdinW.Close()
	if err != nil {
		t.Fatal(err)
	}

	pid, err := pconfig.Pid()
	if err != nil {
		t.Fatal(err)
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		t.Fatal(err)
	}

	imagesDir, err := ioutil.TempDir("", "criu")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(imagesDir)

	checkpointOpts := &libcontainer.CriuOpts{
		ImagesDirectory: imagesDir,
		WorkDirectory: imagesDir,
	}

	if err := container.Checkpoint(checkpointOpts); err != nil {
		t.Fatal(err)
	}

	state, err := container.Status()
	if err != nil {
		t.Fatal(err)
	}

	if state != libcontainer.Checkpointed {
		t.Fatal("Unexpected state: ", state)
	}

	stdinW.Close()
	_, err = process.Wait()
	if err != nil {
		t.Fatal(err)
	}

	// reload the container
	container, err = factory.Load("test")
	if err != nil {
		t.Fatal(err)
	}

	restoreStdinR, restoreStdinW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}

	restoreProcessConfig := &libcontainer.Process{
		Stdin:  restoreStdinR,
		Stdout: &stdout,
	}

	err = container.Restore(restoreProcessConfig, &libcontainer.CriuOpts{
		ImagesDirectory: imagesDir,
	})
	restoreStdinR.Close()
	defer restoreStdinW.Close()

	state, err = container.Status()
	if err != nil {
		t.Fatal(err)
	}
	if state != libcontainer.Running {
		t.Fatal("Unexpected state: ", state)
	}

	pid, err = restoreProcessConfig.Pid()
	if err != nil {
		t.Fatal(err)
	}

	process, err = os.FindProcess(pid)
	if err != nil {
		t.Fatal(err)
	}

	_, err = restoreStdinW.WriteString("Hello!")
	if err != nil {
		t.Fatal(err)
	}

	restoreStdinW.Close()
	s, err := process.Wait()
	if err != nil {
		t.Fatal(err)
	}

	if !s.Success() {
		t.Fatal(s.String(), pid)
	}

	output := string(stdout.Bytes())
	if !strings.Contains(output, "Hello!") {
		t.Fatal("Did not restore the pipe correctly:", output)
	}
}
