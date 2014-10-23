package daemon

import (
	"github.com/docker/docker/nat"
	"testing"
)

func TestParseNetworkOptsPrivateOnly(t *testing.T) {
	ports, bindings, err := nat.ParsePortSpecs([]string{"192.168.1.100::80"})
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
	ports, bindings, err := nat.ParsePortSpecs([]string{"192.168.1.100:8080:80"})
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

func TestParseNetworkOptsPublicNoPort(t *testing.T) {
	ports, bindings, err := nat.ParsePortSpecs([]string{"192.168.1.100"})

	if err == nil {
		t.Logf("Expected error Invalid containerPort")
		t.Fail()
	}
	if ports != nil {
		t.Logf("Expected nil got %s", ports)
		t.Fail()
	}
	if bindings != nil {
		t.Logf("Expected nil got %s", bindings)
		t.Fail()
	}
}

func TestParseNetworkOptsNegativePorts(t *testing.T) {
	ports, bindings, err := nat.ParsePortSpecs([]string{"192.168.1.100:-1:-1"})

	if err == nil {
		t.Fail()
	}
	t.Logf("%v", len(ports))
	t.Logf("%v", bindings)
	if len(ports) != 0 {
		t.Logf("Expected nil got %s", len(ports))
		t.Fail()
	}
	if len(bindings) != 0 {
		t.Logf("Expected 0 got %s", len(bindings))
		t.Fail()
	}
}

func TestParseNetworkOptsUdp(t *testing.T) {
	ports, bindings, err := nat.ParsePortSpecs([]string{"192.168.1.100::6000/udp"})
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
	name, err := GetFullContainerName("testing")
	if err != nil {
		t.Fatal(err)
	}
	if name != "/testing" {
		t.Fatalf("Expected /testing got %s", name)
	}
	if _, err := GetFullContainerName(""); err == nil {
		t.Fatal("Error should not be nil")
	}
}

func TestValidContainerNames(t *testing.T) {
	invalidNames := []string{"-rm", "&sdfsfd", "safd%sd"}
	validNames := []string{"word-word", "word_word", "1weoid"}

	for _, name := range invalidNames {
		if validContainerNamePattern.MatchString(name) {
			t.Fatalf("%q is not a valid container name and was returned as valid.", name)
		}
	}

	for _, name := range validNames {
		if !validContainerNamePattern.MatchString(name) {
			t.Fatalf("%q is a valid container name and was returned as invalid.", name)
		}
	}
}
