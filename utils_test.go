package docker

import (
	"fmt"
	"github.com/dotcloud/docker/utils"
	"io"
	"io/ioutil"
	"os"
	"path"
	"runtime"
	"strings"
	"testing"
)

// This file contains utility functions for docker's unit test suite.
// It has to be named XXX_test.go, apparently, in other to access private functions
// from other XXX_test.go functions.

var globalTestID string

// Create a temporary runtime suitable for unit testing.
// Call t.Fatal() at the first error.
func mkRuntime(f Fataler) *Runtime {
	// Use the caller function name as a prefix.
	// This helps trace temp directories back to their test.
	pc, _, _, _ := runtime.Caller(1)
	callerLongName := runtime.FuncForPC(pc).Name()
	parts := strings.Split(callerLongName, ".")
	callerShortName := parts[len(parts)-1]
	if globalTestID == "" {
		globalTestID = GenerateID()[:4]
	}
	prefix := fmt.Sprintf("docker-test%s-%s-", globalTestID, callerShortName)
	utils.Debugf("prefix = '%s'", prefix)

	runtime, err := newTestRuntime(prefix)
	if err != nil {
		f.Fatal(err)
	}
	return runtime
}

// A common interface to access the Fatal method of
// both testing.B and testing.T.
type Fataler interface {
	Fatal(args ...interface{})
}

func newTestRuntime(prefix string) (runtime *Runtime, err error) {
	if prefix == "" {
		prefix = "docker-test-"
	}
	utils.Debugf("prefix = %s", prefix)
	utils.Debugf("newTestRuntime start")
	root, err := ioutil.TempDir("", prefix)
	defer func() {
		utils.Debugf("newTestRuntime: %s", root)
	}()
	if err != nil {
		return nil, err
	}
	if err := os.Remove(root); err != nil {
		return nil, err
	}
	if err := utils.CopyDirectory(unitTestStoreBase, root); err != nil {
		return nil, err
	}

	config := &DaemonConfig{
		Root:   root,
		AutoRestart: false,
	}
	runtime, err = NewRuntimeFromDirectory(config)
	if err != nil {
		return nil, err
	}
	runtime.UpdateCapabilities(true)
	return runtime, nil
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
	f, err := os.OpenFile(dst, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0700)
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
// If the image name is "_", (eg. []string{"-i", "-t", "_", "bash"}, it is
// dynamically replaced by the current test image.
// The caller is responsible for destroying the container.
// Call t.Fatal() at the first error.
func mkContainer(r *Runtime, args []string, t *testing.T) (*Container, error) {
	config, hostConfig, _, err := ParseRun(args, nil)
	defer func() {
		if err != nil && t != nil {
			t.Fatal(err)
		}
	}()
	if err != nil {
		return nil, err
	}
	if config.Image == "_" {
		config.Image = GetTestImage(r).ID
	}
	c, _, err := r.Create(config, "")
	if err != nil {
		return nil, err
	}
	c.hostConfig = hostConfig
	return c, nil
}

// Create a test container, start it, wait for it to complete, destroy it,
// and return its standard output as a string.
// The image name (eg. the XXX in []string{"-i", "-t", "XXX", "bash"}, is dynamically replaced by the current test image.
// If t is not nil, call t.Fatal() at the first error. Otherwise return errors normally.
func runContainer(r *Runtime, args []string, t *testing.T) (output string, err error) {
	defer func() {
		if err != nil && t != nil {
			t.Fatal(err)
		}
	}()
	container, err := mkContainer(r, args, t)
	if err != nil {
		return "", err
	}
	defer r.Destroy(container)
	stdout, err := container.StdoutPipe()
	if err != nil {
		return "", err
	}
	defer stdout.Close()
	if err := container.Start(); err != nil {
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

func TestCompareConfig(t *testing.T) {
	volumes1 := make(map[string]struct{})
	volumes1["/test1"] = struct{}{}
	config1 := Config{
		Dns:         []string{"1.1.1.1", "2.2.2.2"},
		PortSpecs:   []string{"1111:1111", "2222:2222"},
		Env:         []string{"VAR1=1", "VAR2=2"},
		VolumesFrom: "11111111",
		Volumes:     volumes1,
	}
	config2 := Config{
		Dns:         []string{"0.0.0.0", "2.2.2.2"},
		PortSpecs:   []string{"1111:1111", "2222:2222"},
		Env:         []string{"VAR1=1", "VAR2=2"},
		VolumesFrom: "11111111",
		Volumes:     volumes1,
	}
	config3 := Config{
		Dns:         []string{"1.1.1.1", "2.2.2.2"},
		PortSpecs:   []string{"0000:0000", "2222:2222"},
		Env:         []string{"VAR1=1", "VAR2=2"},
		VolumesFrom: "11111111",
		Volumes:     volumes1,
	}
	config4 := Config{
		Dns:         []string{"1.1.1.1", "2.2.2.2"},
		PortSpecs:   []string{"0000:0000", "2222:2222"},
		Env:         []string{"VAR1=1", "VAR2=2"},
		VolumesFrom: "22222222",
		Volumes:     volumes1,
	}
	volumes2 := make(map[string]struct{})
	volumes2["/test2"] = struct{}{}
	config5 := Config{
		Dns:         []string{"1.1.1.1", "2.2.2.2"},
		PortSpecs:   []string{"0000:0000", "2222:2222"},
		Env:         []string{"VAR1=1", "VAR2=2"},
		VolumesFrom: "11111111",
		Volumes:     volumes2,
	}
	if CompareConfig(&config1, &config2) {
		t.Fatalf("CompareConfig should return false, Dns are different")
	}
	if CompareConfig(&config1, &config3) {
		t.Fatalf("CompareConfig should return false, PortSpecs are different")
	}
	if CompareConfig(&config1, &config4) {
		t.Fatalf("CompareConfig should return false, VolumesFrom are different")
	}
	if CompareConfig(&config1, &config5) {
		t.Fatalf("CompareConfig should return false, Volumes are different")
	}
	if !CompareConfig(&config1, &config1) {
		t.Fatalf("CompareConfig should return true")
	}
}

func TestMergeConfig(t *testing.T) {
	volumesImage := make(map[string]struct{})
	volumesImage["/test1"] = struct{}{}
	volumesImage["/test2"] = struct{}{}
	configImage := &Config{
		Dns:         []string{"1.1.1.1", "2.2.2.2"},
		PortSpecs:   []string{"1111:1111", "2222:2222"},
		Env:         []string{"VAR1=1", "VAR2=2"},
		VolumesFrom: "1111",
		Volumes:     volumesImage,
	}

	volumesUser := make(map[string]struct{})
	volumesUser["/test3"] = struct{}{}
	configUser := &Config{
		Dns:       []string{"3.3.3.3"},
		PortSpecs: []string{"3333:2222", "3333:3333"},
		Env:       []string{"VAR2=3", "VAR3=3"},
		Volumes:   volumesUser,
	}

	MergeConfig(configUser, configImage)

	if len(configUser.Dns) != 3 {
		t.Fatalf("Expected 3 dns, 1.1.1.1, 2.2.2.2 and 3.3.3.3, found %d", len(configUser.Dns))
	}
	for _, dns := range configUser.Dns {
		if dns != "1.1.1.1" && dns != "2.2.2.2" && dns != "3.3.3.3" {
			t.Fatalf("Expected 1.1.1.1 or 2.2.2.2 or 3.3.3.3, found %s", dns)
		}
	}

	if len(configUser.ExposedPorts) != 3 {
		t.Fatalf("Expected 3 portSpecs, 1111, 2222 and 3333, found %d", len(configUser.PortSpecs))
	}
	for portSpecs := range configUser.ExposedPorts {
		if portSpecs.Port() != "1111" && portSpecs.Port() != "2222" && portSpecs.Port() != "3333" {
			t.Fatalf("Expected 1111 or 2222 or 3333, found %s", portSpecs)
		}
	}
	if len(configUser.Env) != 3 {
		t.Fatalf("Expected 3 env var, VAR1=1, VAR2=3 and VAR3=3, found %d", len(configUser.Env))
	}
	for _, env := range configUser.Env {
		if env != "VAR1=1" && env != "VAR2=3" && env != "VAR3=3" {
			t.Fatalf("Expected VAR1=1 or VAR2=3 or VAR3=3, found %s", env)
		}
	}

	if len(configUser.Volumes) != 3 {
		t.Fatalf("Expected 3 volumes, /test1, /test2 and /test3, found %d", len(configUser.Volumes))
	}
	for v := range configUser.Volumes {
		if v != "/test1" && v != "/test2" && v != "/test3" {
			t.Fatalf("Expected /test1 or /test2 or /test3, found %s", v)
		}
	}

	if configUser.VolumesFrom != "1111" {
		t.Fatalf("Expected VolumesFrom to be 1111, found %s", configUser.VolumesFrom)
	}
}

func TestParseLxcConfOpt(t *testing.T) {
	opts := []string{"lxc.utsname=docker", "lxc.utsname = docker "}

	for _, o := range opts {
		k, v, err := parseLxcOpt(o)
		if err != nil {
			t.FailNow()
		}
		if k != "lxc.utsname" {
			t.Fail()
		}
		if v != "docker" {
			t.Fail()
		}
	}
}

func TestParseNetworkOptsPrivateOnly(t *testing.T) {
	ports, bindings, err := parsePortSpecs([]string{"192.168.1.100::80"})
	if err != nil {
		t.Fatal(err)
	}
	if len(ports) != 1 {
		t.Logf("Expected 1 got %d", len(ports))
		t.FailNow()
	}
	if len(bindings) != 1 {
		t.Logf("Expected 1 got %d", len(bindings))
		t.FailNow()
	}
	for k := range ports {
		if k.Proto() != "tcp" {
			t.Logf("Expected tcp got %s", k.Proto())
			t.Fail()
		}
		if k.Port() != "80" {
			t.Logf("Expected 80 got %s", k.Port())
			t.Fail()
		}
		b, exists := bindings[k]
		if !exists {
			t.Log("Binding does not exist")
			t.FailNow()
		}
		if len(b) != 1 {
			t.Logf("Expected 1 got %d", len(b))
			t.FailNow()
		}
		s := b[0]
		if s.HostPort != "" {
			t.Logf("Expected \"\" got %s", s.HostPort)
			t.Fail()
		}
		if s.HostIp != "192.168.1.100" {
			t.Fail()
		}
	}
}

func TestParseNetworkOptsPublic(t *testing.T) {
	ports, bindings, err := parsePortSpecs([]string{"192.168.1.100:8080:80"})
	if err != nil {
		t.Fatal(err)
	}
	if len(ports) != 1 {
		t.Logf("Expected 1 got %d", len(ports))
		t.FailNow()
	}
	if len(bindings) != 1 {
		t.Logf("Expected 1 got %d", len(bindings))
		t.FailNow()
	}
	for k := range ports {
		if k.Proto() != "tcp" {
			t.Logf("Expected tcp got %s", k.Proto())
			t.Fail()
		}
		if k.Port() != "80" {
			t.Logf("Expected 80 got %s", k.Port())
			t.Fail()
		}
		b, exists := bindings[k]
		if !exists {
			t.Log("Binding does not exist")
			t.FailNow()
		}
		if len(b) != 1 {
			t.Logf("Expected 1 got %d", len(b))
			t.FailNow()
		}
		s := b[0]
		if s.HostPort != "8080" {
			t.Logf("Expected 8080 got %s", s.HostPort)
			t.Fail()
		}
		if s.HostIp != "192.168.1.100" {
			t.Fail()
		}
	}
}

func TestParseNetworkOptsUdp(t *testing.T) {
	ports, bindings, err := parsePortSpecs([]string{"192.168.1.100::6000/udp"})
	if err != nil {
		t.Fatal(err)
	}
	if len(ports) != 1 {
		t.Logf("Expected 1 got %d", len(ports))
		t.FailNow()
	}
	if len(bindings) != 1 {
		t.Logf("Expected 1 got %d", len(bindings))
		t.FailNow()
	}
	for k := range ports {
		if k.Proto() != "udp" {
			t.Logf("Expected udp got %s", k.Proto())
			t.Fail()
		}
		if k.Port() != "6000" {
			t.Logf("Expected 6000 got %s", k.Port())
			t.Fail()
		}
		b, exists := bindings[k]
		if !exists {
			t.Log("Binding does not exist")
			t.FailNow()
		}
		if len(b) != 1 {
			t.Logf("Expected 1 got %d", len(b))
			t.FailNow()
		}
		s := b[0]
		if s.HostPort != "" {
			t.Logf("Expected \"\" got %s", s.HostPort)
			t.Fail()
		}
		if s.HostIp != "192.168.1.100" {
			t.Fail()
		}
	}
}
