package integration

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/docker/libcontainer"
)

func TestExecIn(t *testing.T) {
	if testing.Short() {
		return
	}
	rootfs, err := newRootfs()
	if err != nil {
		t.Fatal(err)
	}
	defer remove(rootfs)
	config := newTemplateConfig(rootfs)
	container, err := newContainer(config)
	if err != nil {
		t.Fatal(err)
	}
	defer container.Destroy()

	// Execute a first process in the container
	stdinR, stdinW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	process := &libcontainer.Process{
		Args:  []string{"cat"},
		Env:   standardEnvironment,
		Stdin: stdinR,
	}
	err = container.Start(process)
	stdinR.Close()
	defer stdinW.Close()
	if err != nil {
		t.Fatal(err)
	}

	buffers := newStdBuffers()
	ps := &libcontainer.Process{
		Args:   []string{"ps"},
		Env:    standardEnvironment,
		Stdin:  buffers.Stdin,
		Stdout: buffers.Stdout,
		Stderr: buffers.Stderr,
	}
	err = container.Start(ps)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ps.Wait(); err != nil {
		t.Fatal(err)
	}
	stdinW.Close()
	if _, err := process.Wait(); err != nil {
		t.Log(err)
	}
	out := buffers.Stdout.String()
	if !strings.Contains(out, "cat") || !strings.Contains(out, "ps") {
		t.Fatalf("unexpected running process, output %q", out)
	}
}

func TestExecInRlimit(t *testing.T) {
	if testing.Short() {
		return
	}
	rootfs, err := newRootfs()
	if err != nil {
		t.Fatal(err)
	}
	defer remove(rootfs)
	config := newTemplateConfig(rootfs)
	container, err := newContainer(config)
	if err != nil {
		t.Fatal(err)
	}
	defer container.Destroy()

	stdinR, stdinW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	process := &libcontainer.Process{
		Args:  []string{"cat"},
		Env:   standardEnvironment,
		Stdin: stdinR,
	}
	err = container.Start(process)
	stdinR.Close()
	defer stdinW.Close()
	if err != nil {
		t.Fatal(err)
	}

	buffers := newStdBuffers()
	ps := &libcontainer.Process{
		Args:   []string{"/bin/sh", "-c", "ulimit -n"},
		Env:    standardEnvironment,
		Stdin:  buffers.Stdin,
		Stdout: buffers.Stdout,
		Stderr: buffers.Stderr,
	}
	err = container.Start(ps)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ps.Wait(); err != nil {
		t.Fatal(err)
	}
	stdinW.Close()
	if _, err := process.Wait(); err != nil {
		t.Log(err)
	}
	out := buffers.Stdout.String()
	if limit := strings.TrimSpace(out); limit != "1025" {
		t.Fatalf("expected rlimit to be 1025, got %s", limit)
	}
}

func TestExecInError(t *testing.T) {
	if testing.Short() {
		return
	}
	rootfs, err := newRootfs()
	if err != nil {
		t.Fatal(err)
	}
	defer remove(rootfs)
	config := newTemplateConfig(rootfs)
	container, err := newContainer(config)
	if err != nil {
		t.Fatal(err)
	}
	defer container.Destroy()

	// Execute a first process in the container
	stdinR, stdinW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	process := &libcontainer.Process{
		Args:  []string{"cat"},
		Env:   standardEnvironment,
		Stdin: stdinR,
	}
	err = container.Start(process)
	stdinR.Close()
	defer func() {
		stdinW.Close()
		if _, err := process.Wait(); err != nil {
			t.Log(err)
		}
	}()
	if err != nil {
		t.Fatal(err)
	}

	unexistent := &libcontainer.Process{
		Args: []string{"unexistent"},
		Env:  standardEnvironment,
	}
	err = container.Start(unexistent)
	if err == nil {
		t.Fatal("Should be an error")
	}
	if !strings.Contains(err.Error(), "executable file not found") {
		t.Fatalf("Should be error about not found executable, got %s", err)
	}
}

func TestExecInTTY(t *testing.T) {
	if testing.Short() {
		return
	}
	rootfs, err := newRootfs()
	if err != nil {
		t.Fatal(err)
	}
	defer remove(rootfs)
	config := newTemplateConfig(rootfs)
	container, err := newContainer(config)
	if err != nil {
		t.Fatal(err)
	}
	defer container.Destroy()

	// Execute a first process in the container
	stdinR, stdinW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	process := &libcontainer.Process{
		Args:  []string{"cat"},
		Env:   standardEnvironment,
		Stdin: stdinR,
	}
	err = container.Start(process)
	stdinR.Close()
	defer stdinW.Close()
	if err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	ps := &libcontainer.Process{
		Args: []string{"ps"},
		Env:  standardEnvironment,
	}
	console, err := ps.NewConsole(0)
	copy := make(chan struct{})
	go func() {
		io.Copy(&stdout, console)
		close(copy)
	}()
	if err != nil {
		t.Fatal(err)
	}
	err = container.Start(ps)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-time.After(5 * time.Second):
		t.Fatal("Waiting for copy timed out")
	case <-copy:
	}
	if _, err := ps.Wait(); err != nil {
		t.Fatal(err)
	}
	stdinW.Close()
	if _, err := process.Wait(); err != nil {
		t.Log(err)
	}
	out := stdout.String()
	if !strings.Contains(out, "cat") || !strings.Contains(string(out), "ps") {
		t.Fatalf("unexpected running process, output %q", out)
	}
}

func TestExecInEnvironment(t *testing.T) {
	if testing.Short() {
		return
	}
	rootfs, err := newRootfs()
	if err != nil {
		t.Fatal(err)
	}
	defer remove(rootfs)
	config := newTemplateConfig(rootfs)
	container, err := newContainer(config)
	if err != nil {
		t.Fatal(err)
	}
	defer container.Destroy()

	// Execute a first process in the container
	stdinR, stdinW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	process := &libcontainer.Process{
		Args:  []string{"cat"},
		Env:   standardEnvironment,
		Stdin: stdinR,
	}
	err = container.Start(process)
	stdinR.Close()
	defer stdinW.Close()
	if err != nil {
		t.Fatal(err)
	}

	buffers := newStdBuffers()
	process2 := &libcontainer.Process{
		Args: []string{"env"},
		Env: []string{
			"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
			"DEBUG=true",
			"DEBUG=false",
			"ENV=test",
		},
		Stdin:  buffers.Stdin,
		Stdout: buffers.Stdout,
		Stderr: buffers.Stderr,
	}
	err = container.Start(process2)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := process2.Wait(); err != nil {
		out := buffers.Stdout.String()
		t.Fatal(err, out)
	}
	stdinW.Close()
	if _, err := process.Wait(); err != nil {
		t.Log(err)
	}
	out := buffers.Stdout.String()
	// check execin's process environment
	if !strings.Contains(out, "DEBUG=false") ||
		!strings.Contains(out, "ENV=test") ||
		!strings.Contains(out, "HOME=/root") ||
		!strings.Contains(out, "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin") ||
		strings.Contains(out, "DEBUG=true") {
		t.Fatalf("unexpected running process, output %q", out)
	}
}
