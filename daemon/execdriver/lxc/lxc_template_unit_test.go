// +build linux

package lxc

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/daemon/execdriver"
	"github.com/docker/libcontainer/devices"
)

func TestLXCConfig(t *testing.T) {
	root, err := ioutil.TempDir("", "TestLXCConfig")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)

	os.MkdirAll(path.Join(root, "containers", "1"), 0777)

	// Memory is allocated randomly for testing
	rand.Seed(time.Now().UTC().UnixNano())
	var (
		memMin = 33554432
		memMax = 536870912
		mem    = memMin + rand.Intn(memMax-memMin)
		cpuMin = 100
		cpuMax = 10000
		cpu    = cpuMin + rand.Intn(cpuMax-cpuMin)
	)

	driver, err := NewDriver(root, "", false)
	if err != nil {
		t.Fatal(err)
	}
	command := &execdriver.Command{
		ID: "1",
		Resources: &execdriver.Resources{
			Memory:    int64(mem),
			CpuShares: int64(cpu),
		},
		Network: &execdriver.Network{
			Mtu:       1500,
			Interface: nil,
		},
		AllowedDevices: make([]*devices.Device, 0),
		ProcessConfig:  execdriver.ProcessConfig{},
	}
	p, err := driver.generateLXCConfig(command)
	if err != nil {
		t.Fatal(err)
	}
	grepFile(t, p,
		fmt.Sprintf("lxc.cgroup.memory.limit_in_bytes = %d", mem))

	grepFile(t, p,
		fmt.Sprintf("lxc.cgroup.memory.memsw.limit_in_bytes = %d", mem*2))
}

func TestCustomLxcConfig(t *testing.T) {
	root, err := ioutil.TempDir("", "TestCustomLxcConfig")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)

	os.MkdirAll(path.Join(root, "containers", "1"), 0777)

	driver, err := NewDriver(root, "", false)
	if err != nil {
		t.Fatal(err)
	}
	processConfig := execdriver.ProcessConfig{
		Privileged: false,
	}
	command := &execdriver.Command{
		ID: "1",
		LxcConfig: []string{
			"lxc.utsname = docker",
			"lxc.cgroup.cpuset.cpus = 0,1",
		},
		Network: &execdriver.Network{
			Mtu:       1500,
			Interface: nil,
		},
		ProcessConfig: processConfig,
	}

	p, err := driver.generateLXCConfig(command)
	if err != nil {
		t.Fatal(err)
	}

	grepFile(t, p, "lxc.utsname = docker")
	grepFile(t, p, "lxc.cgroup.cpuset.cpus = 0,1")
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

func TestEscapeFstabSpaces(t *testing.T) {
	var testInputs = map[string]string{
		" ":                      "\\040",
		"":                       "",
		"/double  space":         "/double\\040\\040space",
		"/some long test string": "/some\\040long\\040test\\040string",
		"/var/lib/docker":        "/var/lib/docker",
		" leading":               "\\040leading",
		"trailing ":              "trailing\\040",
	}
	for in, exp := range testInputs {
		if out := escapeFstabSpaces(in); exp != out {
			t.Logf("Expected %s got %s", exp, out)
			t.Fail()
		}
	}
}

func TestIsDirectory(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "TestIsDir")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	tempFile, err := ioutil.TempFile(tempDir, "TestIsDirFile")
	if err != nil {
		t.Fatal(err)
	}

	if isDirectory(tempDir) != "dir" {
		t.Logf("Could not identify %s as a directory", tempDir)
		t.Fail()
	}

	if isDirectory(tempFile.Name()) != "file" {
		t.Logf("Could not identify %s as a file", tempFile.Name())
		t.Fail()
	}
}

func TestCustomLxcConfigMounts(t *testing.T) {
	root, err := ioutil.TempDir("", "TestCustomLxcConfig")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)
	tempDir, err := ioutil.TempDir("", "TestIsDir")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	tempFile, err := ioutil.TempFile(tempDir, "TestIsDirFile")
	if err != nil {
		t.Fatal(err)
	}
	os.MkdirAll(path.Join(root, "containers", "1"), 0777)

	driver, err := NewDriver(root, "", false)
	if err != nil {
		t.Fatal(err)
	}
	processConfig := execdriver.ProcessConfig{
		Privileged: false,
	}
	mounts := []execdriver.Mount{
		{
			Source:      tempDir,
			Destination: tempDir,
			Writable:    false,
			Private:     true,
		},
		{
			Source:      tempFile.Name(),
			Destination: tempFile.Name(),
			Writable:    true,
			Private:     true,
		},
	}
	command := &execdriver.Command{
		ID: "1",
		LxcConfig: []string{
			"lxc.utsname = docker",
			"lxc.cgroup.cpuset.cpus = 0,1",
		},
		Network: &execdriver.Network{
			Mtu:       1500,
			Interface: nil,
		},
		Mounts:        mounts,
		ProcessConfig: processConfig,
	}

	p, err := driver.generateLXCConfig(command)
	if err != nil {
		t.Fatal(err)
	}

	grepFile(t, p, "lxc.utsname = docker")
	grepFile(t, p, "lxc.cgroup.cpuset.cpus = 0,1")

	grepFile(t, p, fmt.Sprintf("lxc.mount.entry = %s %s none rbind,ro,create=%s 0 0", tempDir, "/"+tempDir, "dir"))
	grepFile(t, p, fmt.Sprintf("lxc.mount.entry = %s %s none rbind,rw,create=%s 0 0", tempFile.Name(), "/"+tempFile.Name(), "file"))
}
