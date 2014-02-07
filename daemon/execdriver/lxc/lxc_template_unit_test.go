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
	p := path.Join(root, "config.lxc")
	err = execdriver.GenerateContainerConfig(LxcTemplateCompiled, command, false, p)
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

	p := path.Join(root, "config.lxc")
	err = execdriver.GenerateContainerConfig(LxcTemplateCompiled, command, false, p)
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
