package docker

import (
	"testing"
)



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




// FIXME: test that destroying a container actually removes its root directory


/*
func TestLXCConfig(t *testing.T) {
	// Memory is allocated randomly for testing
	rand.Seed(time.Now().UTC().UnixNano())
	memMin := 33554432
	memMax := 536870912
	mem := memMin + rand.Intn(memMax-memMin)
	// CPU shares as well
	cpuMin := 100
	cpuMax := 10000
	cpu := cpuMin + rand.Intn(cpuMax-cpuMin)
	container, _, err := runtime.Create(&Config{
		Image: GetTestImage(runtime).ID,
		Cmd:   []string{"/bin/true"},

		Hostname:  "foobar",
		Memory:    int64(mem),
		CpuShares: int64(cpu),
	},
		"",
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Destroy(container)
	container.generateLXCConfig()
	grepFile(t, container.lxcConfigPath(), "lxc.utsname = foobar")
	grepFile(t, container.lxcConfigPath(),
		fmt.Sprintf("lxc.cgroup.memory.limit_in_bytes = %d", mem))
	grepFile(t, container.lxcConfigPath(),
		fmt.Sprintf("lxc.cgroup.memory.memsw.limit_in_bytes = %d", mem*2))
}


func TestCustomLxcConfig(t *testing.T) {
	runtime := mkRuntime(t)
	defer nuke(runtime)
	container, _, err := runtime.Create(&Config{
		Image: GetTestImage(runtime).ID,
		Cmd:   []string{"/bin/true"},

		Hostname: "foobar",
	},
		"",
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Destroy(container)
	container.hostConfig = &HostConfig{LxcConf: []KeyValuePair{
		{
			Key:   "lxc.utsname",
			Value: "docker",
		},
		{
			Key:   "lxc.cgroup.cpuset.cpus",
			Value: "0,1",
		},
	}}

	container.generateLXCConfig()
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
*/


func TestGetFullName(t *testing.T) {
	name, err := getFullName("testing")
	if err != nil {
		t.Fatal(err)
	}
	if name != "/testing" {
		t.Fatalf("Expected /testing got %s", name)
	}
	if _, err := getFullName(""); err == nil {
		t.Fatal("Error should not be nil")
	}
}
