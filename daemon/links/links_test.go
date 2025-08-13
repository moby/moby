package links

import (
	"slices"
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/moby/moby/api/types/container"
	"gotest.tools/v3/assert"
)

func TestLinkNaming(t *testing.T) {
	actual := EnvVars("172.0.17.3", "172.0.17.2", "/db/docker-1", nil, container.PortSet{
		container.PortFrom(6379, container.TCP): struct{}{},
	})

	expectedEnv := []string{
		"DOCKER_1_NAME=/db/docker-1",
		"DOCKER_1_PORT=tcp://172.0.17.2:6379",
		"DOCKER_1_PORT_6379_TCP=tcp://172.0.17.2:6379",
		"DOCKER_1_PORT_6379_TCP_ADDR=172.0.17.2",
		"DOCKER_1_PORT_6379_TCP_PORT=6379",
		"DOCKER_1_PORT_6379_TCP_PROTO=tcp",
	}

	sort.Strings(actual) // order of env-vars is not relevant
	assert.DeepEqual(t, expectedEnv, actual)
}

func TestLinkNew(t *testing.T) {
	link := NewLink("172.0.17.3", "172.0.17.2", "/db/docker", nil, container.PortSet{
		container.PortFrom(6379, container.TCP): struct{}{},
	})

	expected := &Link{
		Name:     "/db/docker",
		ParentIP: "172.0.17.3",
		ChildIP:  "172.0.17.2",
		Ports:    []container.Port{container.PortFrom(6379, container.TCP)},
	}

	assert.DeepEqual(t, expected, link, cmpopts.EquateComparable(container.Port{}))
}

func TestLinkEnv(t *testing.T) {
	actual := EnvVars("172.0.17.3", "172.0.17.2", "/db/docker", []string{"PASSWORD=gordon"}, container.PortSet{
		container.PortFrom(6379, container.TCP): struct{}{},
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

	sort.Strings(actual) // order of env-vars is not relevant
	assert.DeepEqual(t, expectedEnv, actual)
}

// TestSortPorts verifies that ports are sorted with TCP taking priority,
// and ports with the same protocol to be sorted by port.
func TestSortPorts(t *testing.T) {
	ports := []container.Port{
		container.PortFrom(6379, container.TCP),
		container.PortFrom(6376, container.UDP),
		container.PortFrom(6380, container.TCP),
		container.PortFrom(6376, container.SCTP),
		container.PortFrom(6381, container.TCP),
		container.PortFrom(6381, container.UDP),
		container.PortFrom(6375, container.UDP),
		container.PortFrom(6375, container.SCTP),
	}

	expected := []container.Port{
		container.PortFrom(6379, container.TCP),
		container.PortFrom(6380, container.TCP),
		container.PortFrom(6381, container.TCP),
		container.PortFrom(6375, container.SCTP),
		container.PortFrom(6376, container.SCTP),
		container.PortFrom(6375, container.UDP),
		container.PortFrom(6376, container.UDP),
		container.PortFrom(6381, container.UDP),
	}

	slices.SortFunc(ports, withTCPPriority)
	assert.DeepEqual(t, expected, ports, cmpopts.EquateComparable(container.Port{}))
}

func TestLinkMultipleEnv(t *testing.T) {
	actual := EnvVars("172.0.17.3", "172.0.17.2", "/db/docker", []string{"PASSWORD=gordon"}, container.PortSet{
		container.PortFrom(6300, container.UDP): struct{}{},
		container.PortFrom(6379, container.TCP): struct{}{},
		container.PortFrom(6380, container.TCP): struct{}{},
		container.PortFrom(6381, container.TCP): struct{}{},
		container.PortFrom(6382, container.UDP): struct{}{},
	})

	expectedEnv := []string{
		"DOCKER_ENV_PASSWORD=gordon",
		"DOCKER_NAME=/db/docker",
		"DOCKER_PORT=tcp://172.0.17.2:6379",

		"DOCKER_PORT_6300_UDP=udp://172.0.17.2:6300",
		"DOCKER_PORT_6300_UDP_ADDR=172.0.17.2",
		"DOCKER_PORT_6300_UDP_PORT=6300",
		"DOCKER_PORT_6300_UDP_PROTO=udp",

		"DOCKER_PORT_6379_TCP=tcp://172.0.17.2:6379",
		"DOCKER_PORT_6379_TCP_ADDR=172.0.17.2",
		"DOCKER_PORT_6379_TCP_END=tcp://172.0.17.2:6381",
		"DOCKER_PORT_6379_TCP_PORT=6379",
		"DOCKER_PORT_6379_TCP_PORT_END=6381",
		"DOCKER_PORT_6379_TCP_PORT_START=6379",
		"DOCKER_PORT_6379_TCP_PROTO=tcp",
		"DOCKER_PORT_6379_TCP_START=tcp://172.0.17.2:6379",

		"DOCKER_PORT_6380_TCP=tcp://172.0.17.2:6380",
		"DOCKER_PORT_6380_TCP_ADDR=172.0.17.2",
		"DOCKER_PORT_6380_TCP_PORT=6380",
		"DOCKER_PORT_6380_TCP_PROTO=tcp",

		"DOCKER_PORT_6381_TCP=tcp://172.0.17.2:6381",
		"DOCKER_PORT_6381_TCP_ADDR=172.0.17.2",
		"DOCKER_PORT_6381_TCP_PORT=6381",
		"DOCKER_PORT_6381_TCP_PROTO=tcp",

		"DOCKER_PORT_6382_UDP=udp://172.0.17.2:6382",
		"DOCKER_PORT_6382_UDP_ADDR=172.0.17.2",
		"DOCKER_PORT_6382_UDP_PORT=6382",
		"DOCKER_PORT_6382_UDP_PROTO=udp",
	}

	sort.Strings(actual) // order of env-vars is not relevant
	assert.DeepEqual(t, expectedEnv, actual)
}

func BenchmarkLinkMultipleEnv(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = EnvVars("172.0.17.3", "172.0.17.2", "/db/docker", []string{"PASSWORD=gordon"}, container.PortSet{
			container.PortFrom(6300, container.UDP): struct{}{},
			container.PortFrom(6379, container.TCP): struct{}{},
			container.PortFrom(6380, container.TCP): struct{}{},
			container.PortFrom(6381, container.TCP): struct{}{},
			container.PortFrom(6382, container.UDP): struct{}{},
		})
	}
}
