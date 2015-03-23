package bridge

import (
	"fmt"
	"net"
	"strconv"
	"testing"

	"github.com/docker/docker/daemon/networkdriver/portmapper"
	"github.com/docker/docker/engine"
	"github.com/docker/docker/pkg/iptables"
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

func newInterfaceAllocation(t *testing.T, input engine.Env) (output engine.Env) {
	eng := engine.New()
	eng.Logging = false

	done := make(chan bool)

	// set IPv6 global if given
	if input.Exists("globalIPv6Network") {
		_, globalIPv6Network, _ = net.ParseCIDR(input.Get("globalIPv6Network"))
	}

	job := eng.Job("allocate_interface", "container_id")
	job.Env().Init(&input)
	reader, _ := job.Stdout.AddPipe()
	go func() {
		output.Decode(reader)
		done <- true
	}()

	res := Allocate(job)
	job.Stdout.Close()
	<-done

	if input.Exists("expectFail") && input.GetBool("expectFail") {
		if res == engine.StatusOK {
			t.Fatal("Doesn't fail to allocate network interface")
		}
	} else {
		if res != engine.StatusOK {
			t.Fatal("Failed to allocate network interface")
		}
	}

	if input.Exists("globalIPv6Network") {
		// check for bug #11427
		_, subnet, _ := net.ParseCIDR(input.Get("globalIPv6Network"))
		if globalIPv6Network.IP.String() != subnet.IP.String() {
			t.Fatal("globalIPv6Network was modified during allocation")
		}
		// clean up IPv6 global
		globalIPv6Network = nil
	}

	return
}

func TestIPv6InterfaceAllocationAutoNetmaskGt80(t *testing.T) {

	input := engine.Env{}

	_, subnet, _ := net.ParseCIDR("2001:db8:1234:1234:1234::/81")

	// set global ipv6
	input.Set("globalIPv6Network", subnet.String())

	output := newInterfaceAllocation(t, input)

	// ensure low manually assigend global ip
	ip := net.ParseIP(output.Get("GlobalIPv6"))
	_, subnet, _ = net.ParseCIDR(fmt.Sprintf("%s/%d", subnet.IP.String(), 120))
	if !subnet.Contains(ip) {
		t.Fatalf("Error ip %s not in subnet %s", ip.String(), subnet.String())
	}
}

func TestIPv6InterfaceAllocationAutoNetmaskLe80(t *testing.T) {

	input := engine.Env{}

	_, subnet, _ := net.ParseCIDR("2001:db8:1234:1234:1234::/80")

	// set global ipv6
	input.Set("globalIPv6Network", subnet.String())
	input.Set("RequestedMac", "ab:cd:ab:cd:ab:cd")

	output := newInterfaceAllocation(t, input)

	// ensure global ip with mac
	ip := net.ParseIP(output.Get("GlobalIPv6"))
	expected_ip := net.ParseIP("2001:db8:1234:1234:1234:abcd:abcd:abcd")
	if ip.String() != expected_ip.String() {
		t.Fatalf("Error ip %s should be %s", ip.String(), expected_ip.String())
	}

	// ensure link local format
	ip = net.ParseIP(output.Get("LinkLocalIPv6"))
	expected_ip = net.ParseIP("fe80::a9cd:abff:fecd:abcd")
	if ip.String() != expected_ip.String() {
		t.Fatalf("Error ip %s should be %s", ip.String(), expected_ip.String())
	}

}

func TestIPv6InterfaceAllocationRequest(t *testing.T) {

	input := engine.Env{}

	_, subnet, _ := net.ParseCIDR("2001:db8:1234:1234:1234::/80")
	expected_ip := net.ParseIP("2001:db8:1234:1234:1234::1328")

	// set global ipv6
	input.Set("globalIPv6Network", subnet.String())
	input.Set("RequestedIPv6", expected_ip.String())

	output := newInterfaceAllocation(t, input)

	// ensure global ip with mac
	ip := net.ParseIP(output.Get("GlobalIPv6"))
	if ip.String() != expected_ip.String() {
		t.Fatalf("Error ip %s should be %s", ip.String(), expected_ip.String())
	}

	// retry -> fails for duplicated address
	input.SetBool("expectFail", true)
	output = newInterfaceAllocation(t, input)
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

func TestLinkContainers(t *testing.T) {
	eng := engine.New()
	eng.Logging = false

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

	job.Args[0] = "-I"

	job.Setenv("ChildIP", "172.17.0.2")
	job.Setenv("ParentIP", "172.17.0.1")
	job.SetenvBool("IgnoreErrors", false)
	job.SetenvList("Ports", []string{"1234"})

	bridgeIface = "lo"
	_, err := iptables.NewChain("DOCKER", bridgeIface, iptables.Filter)
	if err != nil {
		t.Fatal(err)
	}

	if res := LinkContainers(job); res != engine.StatusOK {
		t.Fatalf("LinkContainers failed")
	}

	// flush rules
	if _, err = iptables.Raw([]string{"-F", "DOCKER"}...); err != nil {
		t.Fatal(err)
	}

}
