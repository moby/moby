package docker

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"strings"
	"testing"
	"time"
)

func TestLXCConfig(t *testing.T) {
	root, err := ioutil.TempDir("", "TestLXCConfig")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)
	// Memory is allocated randomly for testing
	rand.Seed(time.Now().UTC().UnixNano())
	memMin := 33554432
	memMax := 536870912
	mem := memMin + rand.Intn(memMax-memMin)
	// CPU shares as well
	cpuMin := 100
	cpuMax := 10000
	cpu := cpuMin + rand.Intn(cpuMax-cpuMin)
	container := &Container{
		root: root,
		Config: &Config{
			Hostname:        "foobar",
			Memory:          int64(mem),
			CpuShares:       int64(cpu),
			NetworkDisabled: true,
		},
		hostConfig: &HostConfig{
			Privileged: false,
		},
	}
	if err := container.generateLXCConfig(); err != nil {
		t.Fatal(err)
	}
	grepFile(t, container.lxcConfigPath(), "lxc.utsname = foobar")
	grepFile(t, container.lxcConfigPath(),
		fmt.Sprintf("lxc.cgroup.memory.limit_in_bytes = %d", mem))
	grepFile(t, container.lxcConfigPath(),
		fmt.Sprintf("lxc.cgroup.memory.memsw.limit_in_bytes = %d", mem*2))
}

func TestCustomLxcConfig(t *testing.T) {
	root, err := ioutil.TempDir("", "TestCustomLxcConfig")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)
	container := &Container{
		root: root,
		Config: &Config{
			Hostname:        "foobar",
			NetworkDisabled: true,
		},
		hostConfig: &HostConfig{
			Privileged: false,
			LxcConf: []KeyValuePair{
				{
					Key:   "lxc.utsname",
					Value: "docker",
				},
				{
					Key:   "lxc.cgroup.cpuset.cpus",
					Value: "0,1",
				},
			},
		},
	}
	if err := container.generateLXCConfig(); err != nil {
		t.Fatal(err)
	}
	grepFile(t, container.lxcConfigPath(), "lxc.utsname = docker")
	grepFile(t, container.lxcConfigPath(), "lxc.cgroup.cpuset.cpus = 0,1")
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
