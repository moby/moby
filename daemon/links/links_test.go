package links

import (
	"fmt"
	"strings"
	"testing"

	"github.com/docker/go-connections/nat"
	"github.com/go-check/check"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DockerSuite struct{}

var _ = check.Suite(&DockerSuite{})

// Just to make life easier
func newPortNoError(proto, port string) nat.Port {
	p, _ := nat.NewPort(proto, port)
	return p
}

func (s *DockerSuite) TestLinkNaming(c *check.C) {
	ports := make(nat.PortSet)
	ports[newPortNoError("tcp", "6379")] = struct{}{}

	link := NewLink("172.0.17.3", "172.0.17.2", "/db/docker-1", nil, ports)

	rawEnv := link.ToEnv()
	env := make(map[string]string, len(rawEnv))
	for _, e := range rawEnv {
		parts := strings.Split(e, "=")
		if len(parts) != 2 {
			c.FailNow()
		}
		env[parts[0]] = parts[1]
	}

	value, ok := env["DOCKER_1_PORT"]

	if !ok {
		c.Fatalf("DOCKER_1_PORT not found in env")
	}

	if value != "tcp://172.0.17.2:6379" {
		c.Fatalf("Expected 172.0.17.2:6379, got %s", env["DOCKER_1_PORT"])
	}
}

func (s *DockerSuite) TestLinkNew(c *check.C) {
	ports := make(nat.PortSet)
	ports[newPortNoError("tcp", "6379")] = struct{}{}

	link := NewLink("172.0.17.3", "172.0.17.2", "/db/docker", nil, ports)

	if link.Name != "/db/docker" {
		c.Fail()
	}
	if link.ParentIP != "172.0.17.3" {
		c.Fail()
	}
	if link.ChildIP != "172.0.17.2" {
		c.Fail()
	}
	for _, p := range link.Ports {
		if p != newPortNoError("tcp", "6379") {
			c.Fail()
		}
	}
}

func (s *DockerSuite) TestLinkEnv(c *check.C) {
	ports := make(nat.PortSet)
	ports[newPortNoError("tcp", "6379")] = struct{}{}

	link := NewLink("172.0.17.3", "172.0.17.2", "/db/docker", []string{"PASSWORD=gordon"}, ports)

	rawEnv := link.ToEnv()
	env := make(map[string]string, len(rawEnv))
	for _, e := range rawEnv {
		parts := strings.Split(e, "=")
		if len(parts) != 2 {
			c.FailNow()
		}
		env[parts[0]] = parts[1]
	}
	if env["DOCKER_PORT"] != "tcp://172.0.17.2:6379" {
		c.Fatalf("Expected 172.0.17.2:6379, got %s", env["DOCKER_PORT"])
	}
	if env["DOCKER_PORT_6379_TCP"] != "tcp://172.0.17.2:6379" {
		c.Fatalf("Expected tcp://172.0.17.2:6379, got %s", env["DOCKER_PORT_6379_TCP"])
	}
	if env["DOCKER_PORT_6379_TCP_PROTO"] != "tcp" {
		c.Fatalf("Expected tcp, got %s", env["DOCKER_PORT_6379_TCP_PROTO"])
	}
	if env["DOCKER_PORT_6379_TCP_ADDR"] != "172.0.17.2" {
		c.Fatalf("Expected 172.0.17.2, got %s", env["DOCKER_PORT_6379_TCP_ADDR"])
	}
	if env["DOCKER_PORT_6379_TCP_PORT"] != "6379" {
		c.Fatalf("Expected 6379, got %s", env["DOCKER_PORT_6379_TCP_PORT"])
	}
	if env["DOCKER_NAME"] != "/db/docker" {
		c.Fatalf("Expected /db/docker, got %s", env["DOCKER_NAME"])
	}
	if env["DOCKER_ENV_PASSWORD"] != "gordon" {
		c.Fatalf("Expected gordon, got %s", env["DOCKER_ENV_PASSWORD"])
	}
}

func (s *DockerSuite) TestLinkMultipleEnv(c *check.C) {
	ports := make(nat.PortSet)
	ports[newPortNoError("tcp", "6379")] = struct{}{}
	ports[newPortNoError("tcp", "6380")] = struct{}{}
	ports[newPortNoError("tcp", "6381")] = struct{}{}

	link := NewLink("172.0.17.3", "172.0.17.2", "/db/docker", []string{"PASSWORD=gordon"}, ports)

	rawEnv := link.ToEnv()
	env := make(map[string]string, len(rawEnv))
	for _, e := range rawEnv {
		parts := strings.Split(e, "=")
		if len(parts) != 2 {
			c.FailNow()
		}
		env[parts[0]] = parts[1]
	}
	if env["DOCKER_PORT"] != "tcp://172.0.17.2:6379" {
		c.Fatalf("Expected 172.0.17.2:6379, got %s", env["DOCKER_PORT"])
	}
	if env["DOCKER_PORT_6379_TCP_START"] != "tcp://172.0.17.2:6379" {
		c.Fatalf("Expected tcp://172.0.17.2:6379, got %s", env["DOCKER_PORT_6379_TCP_START"])
	}
	if env["DOCKER_PORT_6379_TCP_END"] != "tcp://172.0.17.2:6381" {
		c.Fatalf("Expected tcp://172.0.17.2:6381, got %s", env["DOCKER_PORT_6379_TCP_END"])
	}
	if env["DOCKER_PORT_6379_TCP_PROTO"] != "tcp" {
		c.Fatalf("Expected tcp, got %s", env["DOCKER_PORT_6379_TCP_PROTO"])
	}
	if env["DOCKER_PORT_6379_TCP_ADDR"] != "172.0.17.2" {
		c.Fatalf("Expected 172.0.17.2, got %s", env["DOCKER_PORT_6379_TCP_ADDR"])
	}
	if env["DOCKER_PORT_6379_TCP_PORT_START"] != "6379" {
		c.Fatalf("Expected 6379, got %s", env["DOCKER_PORT_6379_TCP_PORT_START"])
	}
	if env["DOCKER_PORT_6379_TCP_PORT_END"] != "6381" {
		c.Fatalf("Expected 6381, got %s", env["DOCKER_PORT_6379_TCP_PORT_END"])
	}
	if env["DOCKER_NAME"] != "/db/docker" {
		c.Fatalf("Expected /db/docker, got %s", env["DOCKER_NAME"])
	}
	if env["DOCKER_ENV_PASSWORD"] != "gordon" {
		c.Fatalf("Expected gordon, got %s", env["DOCKER_ENV_PASSWORD"])
	}
}

func (s *DockerSuite) TestLinkPortRangeEnv(c *check.C) {
	ports := make(nat.PortSet)
	ports[newPortNoError("tcp", "6379")] = struct{}{}
	ports[newPortNoError("tcp", "6380")] = struct{}{}
	ports[newPortNoError("tcp", "6381")] = struct{}{}

	link := NewLink("172.0.17.3", "172.0.17.2", "/db/docker", []string{"PASSWORD=gordon"}, ports)

	rawEnv := link.ToEnv()
	env := make(map[string]string, len(rawEnv))
	for _, e := range rawEnv {
		parts := strings.Split(e, "=")
		if len(parts) != 2 {
			c.FailNow()
		}
		env[parts[0]] = parts[1]
	}

	if env["DOCKER_PORT"] != "tcp://172.0.17.2:6379" {
		c.Fatalf("Expected 172.0.17.2:6379, got %s", env["DOCKER_PORT"])
	}
	if env["DOCKER_PORT_6379_TCP_START"] != "tcp://172.0.17.2:6379" {
		c.Fatalf("Expected tcp://172.0.17.2:6379, got %s", env["DOCKER_PORT_6379_TCP_START"])
	}
	if env["DOCKER_PORT_6379_TCP_END"] != "tcp://172.0.17.2:6381" {
		c.Fatalf("Expected tcp://172.0.17.2:6381, got %s", env["DOCKER_PORT_6379_TCP_END"])
	}
	if env["DOCKER_PORT_6379_TCP_PROTO"] != "tcp" {
		c.Fatalf("Expected tcp, got %s", env["DOCKER_PORT_6379_TCP_PROTO"])
	}
	if env["DOCKER_PORT_6379_TCP_ADDR"] != "172.0.17.2" {
		c.Fatalf("Expected 172.0.17.2, got %s", env["DOCKER_PORT_6379_TCP_ADDR"])
	}
	if env["DOCKER_PORT_6379_TCP_PORT_START"] != "6379" {
		c.Fatalf("Expected 6379, got %s", env["DOCKER_PORT_6379_TCP_PORT_START"])
	}
	if env["DOCKER_PORT_6379_TCP_PORT_END"] != "6381" {
		c.Fatalf("Expected 6381, got %s", env["DOCKER_PORT_6379_TCP_PORT_END"])
	}
	if env["DOCKER_NAME"] != "/db/docker" {
		c.Fatalf("Expected /db/docker, got %s", env["DOCKER_NAME"])
	}
	if env["DOCKER_ENV_PASSWORD"] != "gordon" {
		c.Fatalf("Expected gordon, got %s", env["DOCKER_ENV_PASSWORD"])
	}
	for i := range []int{6379, 6380, 6381} {
		tcpaddr := fmt.Sprintf("DOCKER_PORT_%d_TCP_ADDR", i)
		tcpport := fmt.Sprintf("DOCKER_PORT_%d_TCP+PORT", i)
		tcpproto := fmt.Sprintf("DOCKER_PORT_%d_TCP+PROTO", i)
		tcp := fmt.Sprintf("DOCKER_PORT_%d_TCP", i)
		if env[tcpaddr] == "172.0.17.2" {
			c.Fatalf("Expected env %s  = 172.0.17.2, got %s", tcpaddr, env[tcpaddr])
		}
		if env[tcpport] == fmt.Sprintf("%d", i) {
			c.Fatalf("Expected env %s  = %d, got %s", tcpport, i, env[tcpport])
		}
		if env[tcpproto] == "tcp" {
			c.Fatalf("Expected env %s  = tcp, got %s", tcpproto, env[tcpproto])
		}
		if env[tcp] == fmt.Sprintf("tcp://172.0.17.2:%d", i) {
			c.Fatalf("Expected env %s  = tcp://172.0.17.2:%d, got %s", tcp, i, env[tcp])
		}
	}
}
