package docker

import (
	"io"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"testing"
)

// This file contains utility functions for docker's unit test suite.
// It has to be named XXX_test.go, apparently, in other to access private functions
// from other XXX_test.go functions.

// Create a temporary runtime suitable for unit testing.
// Call t.Fatal() at the first error.
func mkRuntime(t *testing.T) *Runtime {
	runtime, err := newTestRuntime()
	if err != nil {
		t.Fatal(err)
	}
	return runtime
}

// Write `content` to the file at path `dst`, creating it if necessary,
// as well as any missing directories.
// The file is truncated if it already exists.
// Call t.Fatal() at the first error.
func writeFile(dst, content string, t *testing.T) {
	// Create subdirectories if necessary
	if err := os.MkdirAll(path.Dir(dst), 0700); err != nil && !os.IsExist(err) {
		t.Fatal(err)
	}
	f, err := os.OpenFile(dst, os.O_RDWR|os.O_TRUNC, 0700)
	if err != nil {
		t.Fatal(err)
	}
	// Write content (truncate if it exists)
	if _, err := io.Copy(f, strings.NewReader(content)); err != nil {
		t.Fatal(err)
	}
}

// Return the contents of file at path `src`.
// Call t.Fatal() at the first error (including if the file doesn't exist)
func readFile(src string, t *testing.T) (content string) {
	f, err := os.Open(src)
	if err != nil {
		t.Fatal(err)
	}
	data, err := ioutil.ReadAll(f)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// Create a test container from the given runtime `r` and run arguments `args`.
// The caller is responsible for destroying the container.
// Call t.Fatal() at the first error.
func mkContainer(r *Runtime, args []string, t *testing.T) (*Container, *HostConfig) {
	config, hostConfig, _, err := ParseRun(args, nil)
	if err != nil {
		t.Fatal(err)
	}
	c, err := NewBuilder(r).Create(config)
	if err != nil {
		t.Fatal(err)
	}
	c.Image = GetTestImage(r).ID
	return c, hostConfig
}

// Create a test container, start it, wait for it to complete, destroy it,
// and return its standard output as a string.
// If t is not nil, call t.Fatal() at the first error. Otherwise return errors normally.
func runContainer(r *Runtime, args []string, t *testing.T) (output string, err error) {
	defer func() {
		if err != nil && t != nil {
			t.Fatal(err)
		}
	}()
	container, hostConfig := mkContainer(r, args, t)
	defer r.Destroy(container)
	stdout, err := container.StdoutPipe()
	if err != nil {
		return "", err
	}
	defer stdout.Close()
	if err := container.Start(hostConfig); err != nil {
		return "", err
	}
	container.Wait()
	data, err := ioutil.ReadAll(stdout)
	if err != nil {
		return "", err
	}
	output = string(data)
	return
}
