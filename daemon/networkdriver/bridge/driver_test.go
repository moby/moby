package bridge

import (
	"net"
	"strconv"
	"testing"

	"github.com/docker/docker/daemon/networkdriver/portmapper"
	"github.com/docker/docker/engine"
)

func init() {
	// reset the new proxy command for mocking out the userland proxy in tests
	portmapper.NewProxy = portmapper.NewMockProxyCommand
}

func findFreePort(t *testing.T) int {
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal("Failed to find a free port")
	}
	defer l.Close()

	result, err := net.ResolveTCPAddr("tcp", l.Addr().String())
	if err != nil {
		t.Fatal("Failed to resolve address to identify free port")
	}
	return result.Port
}

func newPortAllocationJob(eng *engine.Engine, port int) (job *engine.Job) {
	strPort := strconv.Itoa(port)

	job = eng.Job("allocate_port", "container_id")
	job.Setenv("HostIP", "127.0.0.1")
	job.Setenv("HostPort", strPort)
	job.Setenv("Proto", "tcp")
	job.Setenv("ContainerPort", strPort)
	return
}

func newPortAllocationJobWithInvalidHostIP(eng *engine.Engine, port int) (job *engine.Job) {
	strPort := strconv.Itoa(port)

	job = eng.Job("allocate_port", "container_id")
	job.Setenv("HostIP", "localhost")
	job.Setenv("HostPort", strPort)
	job.Setenv("Proto", "tcp")
	job.Setenv("ContainerPort", strPort)
	return
}

func TestAllocatePortDetection(t *testing.T) {
	eng := engine.New()
	eng.Logging = false

	freePort := findFreePort(t)

	// Init driver
	job := eng.Job("initdriver")
	if res := InitDriver(job); res != engine.StatusOK {
		t.Fatal("Failed to initialize network driver")
	}

	// Allocate interface
	job = eng.Job("allocate_interface", "container_id")
	if res := Allocate(job); res != engine.StatusOK {
		t.Fatal("Failed to allocate network interface")
	}

	// Allocate same port twice, expect failure on second call
	job = newPortAllocationJob(eng, freePort)
	if res := AllocatePort(job); res != engine.StatusOK {
		t.Fatal("Failed to find a free port to allocate")
	}
	if res := AllocatePort(job); res == engine.StatusOK {
		t.Fatal("Duplicate port allocation granted by AllocatePort")
	}
}

func TestHostnameFormatChecking(t *testing.T) {
	eng := engine.New()
	eng.Logging = false

	freePort := findFreePort(t)

	// Init driver
	job := eng.Job("initdriver")
	if res := InitDriver(job); res != engine.StatusOK {
		t.Fatal("Failed to initialize network driver")
	}

	// Allocate interface
	job = eng.Job("allocate_interface", "container_id")
	if res := Allocate(job); res != engine.StatusOK {
		t.Fatal("Failed to allocate network interface")
	}

	// Allocate port with invalid HostIP, expect failure with Bad Request http status
	job = newPortAllocationJobWithInvalidHostIP(eng, freePort)
	if res := AllocatePort(job); res == engine.StatusOK {
		t.Fatal("Failed to check invalid HostIP")
	}
}

func TestMacAddrGeneration(t *testing.T) {
	ip := net.ParseIP("192.168.0.1")
	mac := generateMacAddr(ip).String()

	// Should be consistent.
	if generateMacAddr(ip).String() != mac {
		t.Fatal("Inconsistent MAC address")
	}

	// Should be unique.
	ip2 := net.ParseIP("192.168.0.2")
	if generateMacAddr(ip2).String() == mac {
		t.Fatal("Non-unique MAC address")
	}
}
