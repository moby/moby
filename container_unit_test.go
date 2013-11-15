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
