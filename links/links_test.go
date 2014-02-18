package links

import (
	"github.com/dotcloud/docker/nat"
	"strings"
	"testing"
)

func TestLinkNew(t *testing.T) {
	ports := make(nat.PortSet)
	ports[nat.Port("6379/tcp")] = struct{}{}

	link, err := NewLink("172.0.17.3", "172.0.17.2", "2001:db8::3", "2001:db8::2", "/db/docker", nil, ports, nil)
	if err != nil {
		t.Fatal(err)
	}

	if link == nil {
		t.FailNow()
	}
	if link.Name != "/db/docker" {
		t.Fail()
	}
	if link.Alias() != "docker" {
		t.Fail()
	}
	if link.ParentIP != "172.0.17.3" {
		t.Fail()
	}
	if link.ChildIP != "172.0.17.2" {
		t.Fail()
	}
	if link.ParentIP6 != "2001:db8::3" {
		t.Fail()
	}
	if link.ChildIP6 != "2001:db8::2" {
		t.Fail()
	}
	for _, p := range link.Ports {
		if p != nat.Port("6379/tcp") {
			t.Fail()
		}
	}
}

func TestLinkEnv(t *testing.T) {
	ports := make(nat.PortSet)
	ports[nat.Port("6379/tcp")] = struct{}{}

	link, err := NewLink("172.0.17.3", "172.0.17.2", "2001:db8::3", "2001:db8::2", "/db/docker", []string{"PASSWORD=gordon"}, ports, nil)
	if err != nil {
		t.Fatal(err)
	}

	rawEnv := link.ToEnv()
	env := make(map[string]string, len(rawEnv))
	for _, e := range rawEnv {
		parts := strings.Split(e, "=")
		if len(parts) != 2 {
			t.FailNow()
		}
		env[parts[0]] = parts[1]
	}
	if env["DOCKER_PORT"] != "tcp://172.0.17.2:6379" {
		t.Fatalf("Expected 172.0.17.2:6379, got %s", env["DOCKER_PORT"])
	}
	if env["DOCKER_PORT6"] != "tcp://[2001:db8::2]:6379" {
		t.Fatalf("Expected [2001:db8::2]:6379, got %s", env["DOCKER_PORT6"])
	}
	if env["DOCKER_PORT_6379_TCP"] != "tcp://172.0.17.2:6379" {
		t.Fatalf("Expected tcp://172.0.17.2:6379, got %s", env["DOCKER_PORT_6379_TCP"])
	}
	if env["DOCKER_PORT6_6379_TCP"] != "tcp://[2001:db8::2]:6379" {
		t.Fatalf("Expected tcp://[2001:db8::2]:6379, got %s", env["DOCKER_PORT6_6379_TCP"])
	}
	if env["DOCKER_PORT_6379_TCP_PROTO"] != "tcp" {
		t.Fatalf("Expected tcp, got %s", env["DOCKER_PORT_6379_TCP_PROTO"])
	}
	if env["DOCKER_PORT_6379_TCP_ADDR"] != "172.0.17.2" {
		t.Fatalf("Expected 172.0.17.2, got %s", env["DOCKER_PORT_6379_TCP_ADDR"])
	}
	if env["DOCKER_PORT_6379_TCP_PORT"] != "6379" {
		t.Fatalf("Expected 6379, got %s", env["DOCKER_PORT_6379_TCP_PORT"])
	}
	if env["DOCKER_NAME"] != "/db/docker" {
		t.Fatalf("Expected /db/docker, got %s", env["DOCKER_NAME"])
	}
	if env["DOCKER_ENV_PASSWORD"] != "gordon" {
		t.Fatalf("Expected gordon, got %s", env["DOCKER_ENV_PASSWORD"])
	}
}

func TestLinkEnvNoIPv6(t *testing.T) {
	ports := make(nat.PortSet)
	ports[nat.Port("6379/tcp")] = struct{}{}

	link, err := NewLink("172.0.17.3", "172.0.17.2", "", "", "/db/docker", []string{"PASSWORD=gordon"}, ports, nil)
	if err != nil {
		t.Fatal(err)
	}

	rawEnv := link.ToEnv()
	env := make(map[string]string, len(rawEnv))
	for _, e := range rawEnv {
		parts := strings.Split(e, "=")
		if len(parts) != 2 {
			t.FailNow()
		}
		env[parts[0]] = parts[1]
	}
	if _,ok := env["DOCKER_PORT"]; !ok {
		t.Fatal("Expected to find DOCKER_PORT in environment")
	}
	if val,ok := env["DOCKER_PORT6"]; ok {
		t.Fatalf("Expected to not find DOCKER_PORT6 in environment. Value: %s", val)
	}
	if _,ok := env["DOCKER_PORT_6379_TCP"]; !ok {
		t.Fatal("Expected to find DOCKER_PORT_6379_TCP in environment")
	}
	if val,ok := env["DOCKER_PORT6_6379_TCP"]; ok {
		t.Fatalf("Expected to not find DOCKER_PORT6_6379_TCP in environment. Value: %s", val)
	}
}
