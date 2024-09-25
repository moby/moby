package networking

import (
	"bytes"
	"net/netip"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"testing"

	"github.com/vishvananda/netns"
)

// CurrentNetns can be passed to L3Segment.AddHost to indicate that the
// host lives in the current network namespace (eg. where dockerd runs).
const CurrentNetns = ""

func runCommand(t *testing.T, cmd string, args ...string) {
	t.Log(strings.Join(append([]string{cmd}, args...), " "))

	var b bytes.Buffer
	c := exec.Command(cmd, args...)
	c.Stdout = &b
	c.Stderr = &b
	err := c.Run()
	if err != nil {
		t.Log(b.String())
		t.Fatalf("Error: %v", err)
	}
}

// L3Segment simulates a switched, dual-stack capable network that
// interconnects multiple hosts running in their own network namespace.
type L3Segment struct {
	Hosts  map[string]Host
	bridge Host
}

// NewL3Segment creates a new L3Segment. The bridge interface interconnecting
// all the hosts is created in a new network namespace named nsName and it's
// assigned one or more IP addresses. Those need to be unmasked netip.Prefix.
func NewL3Segment(t *testing.T, nsName string, addrs ...netip.Prefix) *L3Segment {
	t.Helper()

	l3 := &L3Segment{
		Hosts: map[string]Host{},
	}

	l3.bridge = newHost(t, nsName, "br0")
	defer func() {
		if t.Failed() {
			l3.Destroy(t)
		}
	}()

	l3.bridge.Run(t, "ip", "link", "add", l3.bridge.Iface, "type", "bridge")
	for _, addr := range addrs {
		l3.bridge.Run(t, "ip", "addr", "add", addr.String(), "dev", l3.bridge.Iface, "nodad")
		l3.bridge.Run(t, "ip", "link", "set", l3.bridge.Iface, "up")
	}

	return l3
}

func (l3 *L3Segment) AddHost(t *testing.T, hostname, nsName, ifname string, addrs ...netip.Prefix) {
	t.Helper()

	if len(hostname) >= syscall.IFNAMSIZ {
		// hostname is reused as the name for the veth interface added to the
		// bridge. Hence, it needs to be shorter than ifnamsiz.
		t.Fatalf("hostname too long")
	}

	host := newHost(t, nsName, ifname)
	l3.Hosts[hostname] = host

	host.Run(t, "ip", "link", "add", hostname, "netns", l3.bridge.ns, "type", "veth", "peer", "name", host.Iface)
	l3.bridge.Run(t, "ip", "link", "set", hostname, "up", "master", l3.bridge.Iface)
	host.Run(t, "ip", "link", "set", host.Iface, "up")

	for _, addr := range addrs {
		host.Run(t, "ip", "addr", "add", addr.String(), "dev", host.Iface, "nodad")
	}
}

func (l3 *L3Segment) Destroy(t *testing.T) {
	for _, host := range l3.Hosts {
		host.Destroy(t)
	}
	l3.bridge.Destroy(t)
}

type Host struct {
	Iface string // Iface is the interface name in the host network namespace.
	ns    string // ns is the network namespace name.
}

func newHost(t *testing.T, nsName, ifname string) Host {
	t.Helper()

	if len(ifname) >= syscall.IFNAMSIZ {
		t.Fatalf("ifname too long")
	}

	if nsName != CurrentNetns {
		runCommand(t, "ip", "netns", "add", nsName)
	}

	return Host{
		Iface: ifname,
		ns:    nsName,
	}
}

// Run executes the provided command in the host's network namespace.
func (h Host) Run(t *testing.T, cmd string, args ...string) {
	t.Helper()

	if h.ns != CurrentNetns {
		args = append([]string{"netns", "exec", h.ns, cmd}, args...)
		cmd = "ip"
	}
	runCommand(t, cmd, args...)
}

// Do run the provided function in the host's network namespace.
func (h Host) Do(t *testing.T, fn func()) {
	t.Helper()

	targetNs, err := netns.GetFromName(h.ns)
	if err != nil {
		t.Fatalf("failed to get netns handle: %v", err)
	}
	defer targetNs.Close()

	origNs, err := netns.Get()
	if err != nil {
		t.Fatalf("failed to get current netns: %v", err)
	}
	defer origNs.Close()

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := netns.Set(targetNs); err != nil {
		t.Fatalf("failed to enter netns: %v", err)
	}
	defer netns.Set(origNs)

	fn()
}

func (h Host) Destroy(t *testing.T) {
	t.Helper()

	if h.ns != CurrentNetns {
		runCommand(t, "ip", "netns", "delete", h.ns)
	}
}
