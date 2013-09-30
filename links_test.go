package docker

import (
	"strings"
	"testing"
)

func newMockLinkContainer(id string, ip string) *Container {
	return &Container{
		Config: &Config{},
		ID:     id,
		NetworkSettings: &NetworkSettings{
			IPAddress: ip,
		},
	}
}

func TestLinkNew(t *testing.T) {
	toID := GenerateID()
	fromID := GenerateID()

	from := newMockLinkContainer(fromID, "172.0.17.2")
	from.Config.Env = []string{}
	from.State = State{Running: true}
	ports := make(map[Port]struct{})

	ports[Port("6379/tcp")] = struct{}{}

	from.Config.ExposedPorts = ports

	to := newMockLinkContainer(toID, "172.0.17.3")

	link, err := NewLink(to, from, "/db/docker", "172.0.17.1")
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
	if link.BridgeInterface != "172.0.17.1" {
		t.Fail()
	}
	for _, p := range link.Ports {
		if p != Port("6379/tcp") {
			t.Fail()
		}
	}
}

func TestLinkEnv(t *testing.T) {
	toID := GenerateID()
	fromID := GenerateID()

	from := newMockLinkContainer(fromID, "172.0.17.2")
	from.Config.Env = []string{"PASSWORD=gordon"}
	from.State = State{Running: true}
	ports := make(map[Port]struct{})

	ports[Port("6379/tcp")] = struct{}{}

	from.Config.ExposedPorts = ports

	to := newMockLinkContainer(toID, "172.0.17.3")

	link, err := NewLink(to, from, "/db/docker", "172.0.17.1")
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
	if env["docker_PORT"] != "tcp://172.0.17.2:6379" {
		t.Fatalf("Expected tcp://172.0.17.2:6379, got %s", env["docker_PORT"])
	}
	if env["docker_PORT_6379_tcp"] != "tcp://172.0.17.2:6379" {
		t.Fatalf("Expected tcp://172.0.17.2:6379, got %s", env["docker_PORT_6379_tcp"])
	}
	if env["docker_NAME"] != "/db/docker" {
		t.Fatalf("Expected /db/docker, got %s", env["docker_NAME"])
	}
	if env["docker_ENV_PASSWORD"] != "gordon" {
		t.Fatalf("Expected gordon, got %s", env["docker_ENV_PASSWORD"])
	}
}
