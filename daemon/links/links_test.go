package links // import "github.com/docker/docker/daemon/links"

import (
	"sort"
	"testing"

	"github.com/docker/go-connections/nat"
	"gotest.tools/v3/assert"
)

func TestLinkNaming(t *testing.T) {
	link := NewLink("172.0.17.3", "172.0.17.2", "/db/docker-1", nil, nat.PortSet{
		"6379/tcp": struct{}{},
	})

	expectedEnv := []string{
		"DOCKER_1_NAME=/db/docker-1",
		"DOCKER_1_PORT=tcp://172.0.17.2:6379",
		"DOCKER_1_PORT_6379_TCP=tcp://172.0.17.2:6379",
		"DOCKER_1_PORT_6379_TCP_ADDR=172.0.17.2",
		"DOCKER_1_PORT_6379_TCP_PORT=6379",
		"DOCKER_1_PORT_6379_TCP_PROTO=tcp",
	}

	actual := link.ToEnv()
	sort.Strings(actual) // order of env-vars is not relevant
	assert.DeepEqual(t, expectedEnv, actual)
}

func TestLinkNew(t *testing.T) {
	link := NewLink("172.0.17.3", "172.0.17.2", "/db/docker", nil, nat.PortSet{
		"6379/tcp": struct{}{},
	})

	expected := &Link{
		Name:     "/db/docker",
		ParentIP: "172.0.17.3",
		ChildIP:  "172.0.17.2",
		Ports:    []nat.Port{"6379/tcp"},
	}

	assert.DeepEqual(t, expected, link)
}

func TestLinkEnv(t *testing.T) {
	link := NewLink("172.0.17.3", "172.0.17.2", "/db/docker", []string{"PASSWORD=gordon"}, nat.PortSet{
		"6379/tcp": struct{}{},
	})

	expectedEnv := []string{
		"DOCKER_ENV_PASSWORD=gordon",
		"DOCKER_NAME=/db/docker",
		"DOCKER_PORT=tcp://172.0.17.2:6379",
		"DOCKER_PORT_6379_TCP=tcp://172.0.17.2:6379",
		"DOCKER_PORT_6379_TCP_ADDR=172.0.17.2",
		"DOCKER_PORT_6379_TCP_PORT=6379",
		"DOCKER_PORT_6379_TCP_PROTO=tcp",
	}

	actual := link.ToEnv()
	sort.Strings(actual) // order of env-vars is not relevant
	assert.DeepEqual(t, expectedEnv, actual)
}

func TestLinkMultipleEnv(t *testing.T) {
	link := NewLink("172.0.17.3", "172.0.17.2", "/db/docker", []string{"PASSWORD=gordon"}, nat.PortSet{
		"6379/tcp": struct{}{},
		"6380/tcp": struct{}{},
		"6381/tcp": struct{}{},
	})

	expectedEnv := []string{
		"DOCKER_ENV_PASSWORD=gordon",
		"DOCKER_NAME=/db/docker",
		"DOCKER_PORT=tcp://172.0.17.2:6379",
		"DOCKER_PORT_6379_TCP=tcp://172.0.17.2:6379",
		"DOCKER_PORT_6379_TCP_ADDR=172.0.17.2",
		"DOCKER_PORT_6379_TCP_ADDR=172.0.17.2", // FIXME(thaJeztah): duplicate?
		"DOCKER_PORT_6379_TCP_END=tcp://172.0.17.2:6381",
		"DOCKER_PORT_6379_TCP_PORT=6379",
		"DOCKER_PORT_6379_TCP_PORT_END=6381",
		"DOCKER_PORT_6379_TCP_PORT_START=6379",
		"DOCKER_PORT_6379_TCP_PROTO=tcp",
		"DOCKER_PORT_6379_TCP_PROTO=tcp", // FIXME(thaJeztah): duplicate?
		"DOCKER_PORT_6379_TCP_START=tcp://172.0.17.2:6379",

		"DOCKER_PORT_6380_TCP=tcp://172.0.17.2:6380",
		"DOCKER_PORT_6380_TCP_ADDR=172.0.17.2",
		"DOCKER_PORT_6380_TCP_PORT=6380",
		"DOCKER_PORT_6380_TCP_PROTO=tcp",

		"DOCKER_PORT_6381_TCP=tcp://172.0.17.2:6381",
		"DOCKER_PORT_6381_TCP_ADDR=172.0.17.2",
		"DOCKER_PORT_6381_TCP_PORT=6381",
		"DOCKER_PORT_6381_TCP_PROTO=tcp",
	}

	actual := link.ToEnv()
	sort.Strings(actual) // order of env-vars is not relevant
	assert.DeepEqual(t, expectedEnv, actual)
}
