package links

import (
	"slices"
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/moby/moby/api/types/network"
	"gotest.tools/v3/assert"
)

func TestLinkNaming(t *testing.T) {
	actual := EnvVars("172.0.17.3", "172.0.17.2", "/db/docker-1", nil, network.PortSet{
		network.MustParsePort("6379/tcp"): struct{}{},
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
	tcp6379 := network.MustParsePort("6379/tcp")
	link := NewLink("172.0.17.3", "172.0.17.2", "/db/docker", nil, network.PortSet{
		tcp6379: struct{}{},
	})

	expected := &Link{
		Name:     "/db/docker",
		ParentIP: "172.0.17.3",
		ChildIP:  "172.0.17.2",
		Ports:    []network.Port{tcp6379},
	}

	assert.DeepEqual(t, expected, link, cmpopts.EquateComparable(network.Port{}))
}

func TestLinkEnv(t *testing.T) {
	tcp6379 := network.MustParsePort("6379/tcp")
	actual := EnvVars("172.0.17.3", "172.0.17.2", "/db/docker", []string{"PASSWORD=gordon"}, network.PortSet{
		tcp6379: struct{}{},
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
	ports := []network.Port{
		network.MustParsePort("6379/tcp"),
		network.MustParsePort("6376/udp"),
		network.MustParsePort("6380/tcp"),
		network.MustParsePort("6376/sctp"),
		network.MustParsePort("6381/tcp"),
		network.MustParsePort("6381/udp"),
		network.MustParsePort("6375/udp"),
		network.MustParsePort("6375/sctp"),
	}

	expected := []network.Port{
		network.MustParsePort("6379/tcp"),
		network.MustParsePort("6380/tcp"),
		network.MustParsePort("6381/tcp"),
		network.MustParsePort("6375/sctp"),
		network.MustParsePort("6376/sctp"),
		network.MustParsePort("6375/udp"),
		network.MustParsePort("6376/udp"),
		network.MustParsePort("6381/udp"),
	}

	slices.SortFunc(ports, withTCPPriority)
	assert.DeepEqual(t, expected, ports, cmpopts.EquateComparable(network.Port{}))
}

func TestLinkMultipleEnv(t *testing.T) {
	actual := EnvVars("172.0.17.3", "172.0.17.2", "/db/docker", []string{"PASSWORD=gordon"}, network.PortSet{
		network.MustParsePort("6300/udp"): struct{}{},
		network.MustParsePort("6379/tcp"): struct{}{},
		network.MustParsePort("6380/tcp"): struct{}{},
		network.MustParsePort("6381/tcp"): struct{}{},
		network.MustParsePort("6382/udp"): struct{}{},
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

	for b.Loop() {
		_ = EnvVars("172.0.17.3", "172.0.17.2", "/db/docker", []string{"PASSWORD=gordon"}, network.PortSet{
			network.MustParsePort("6300/udp"): struct{}{},
			network.MustParsePort("6379/tcp"): struct{}{},
			network.MustParsePort("6380/tcp"): struct{}{},
			network.MustParsePort("6381/tcp"): struct{}{},
			network.MustParsePort("6382/udp"): struct{}{},
		})
	}
}
