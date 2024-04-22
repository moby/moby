package osl

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/docker/docker/internal/testutils/netnsutils"
	"github.com/docker/docker/libnetwork/ns"
	"github.com/docker/docker/libnetwork/types"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netlink/nl"
	"github.com/vishvananda/netns"
)

const (
	vethName1     = "wierdlongname1"
	vethName2     = "wierdlongname2"
	vethName3     = "wierdlongname3"
	vethName4     = "wierdlongname4"
	sboxIfaceName = "containername"
)

func generateRandomName(prefix string, size int) (string, error) {
	id := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, id); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(id)[:size], nil
}

func newKey(t *testing.T) (string, error) {
	t.Helper()
	name, err := generateRandomName("netns", 12)
	if err != nil {
		return "", err
	}

	name = filepath.Join("/tmp", name)
	f, err := os.Create(name)
	if err != nil {
		return "", err
	}
	_ = f.Close()

	// Set the rpmCleanupPeriod to be low to make the test run quicker
	gpmLock.Lock()
	gpmCleanupPeriod = 2 * time.Second
	gpmLock.Unlock()

	return name, nil
}

func newInfo(t *testing.T, hnd *netlink.Handle) (*Namespace, error) {
	t.Helper()
	err := hnd.LinkAdd(&netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{Name: vethName1, TxQLen: 0},
		PeerName:  vethName2,
	})
	if err != nil {
		return nil, err
	}

	ip4, addr, err := net.ParseCIDR("192.168.1.100/24")
	if err != nil {
		return nil, err
	}
	addr.IP = ip4

	ip6, addrv6, err := net.ParseCIDR("fdac:97b4:dbcc::2/64")
	if err != nil {
		return nil, err
	}
	addrv6.IP = ip6

	_, route, err := net.ParseCIDR("192.168.2.1/32")
	if err != nil {
		return nil, err
	}

	// Store the sandbox side pipe interface
	// This is needed for cleanup on DeleteEndpoint()
	intf1 := &Interface{
		srcName:     vethName2,
		dstName:     sboxIfaceName,
		address:     addr,
		addressIPv6: addrv6,
		routes:      []*net.IPNet{route},
	}

	intf2 := &Interface{
		srcName: "testbridge",
		dstName: sboxIfaceName,
		bridge:  true,
	}

	err = hnd.LinkAdd(&netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{Name: vethName3, TxQLen: 0},
		PeerName:  vethName4,
	})
	if err != nil {
		return nil, err
	}

	intf3 := &Interface{
		srcName: vethName4,
		dstName: sboxIfaceName,
		master:  "testbridge",
	}

	return &Namespace{
		iFaces: []*Interface{intf1, intf2, intf3},
		gw:     net.ParseIP("192.168.1.1"),
		gwv6:   net.ParseIP("fdac:97b4:dbcc::1/64"),
	}, nil
}

func verifySandbox(t *testing.T, ns *Namespace, ifaceSuffixes []string) {
	sbNs, err := netns.GetFromPath(ns.Key())
	if err != nil {
		t.Fatalf("Failed top open network namespace path %q: %v", ns.Key(), err)
	}
	defer sbNs.Close()

	nh, err := netlink.NewHandleAt(sbNs)
	if err != nil {
		t.Fatal(err)
	}
	defer nh.Close()

	for _, suffix := range ifaceSuffixes {
		_, err = nh.LinkByName(sboxIfaceName + suffix)
		if err != nil {
			t.Fatalf("Could not find the interface %s inside the sandbox: %v",
				sboxIfaceName+suffix, err)
		}
	}
}

func verifyCleanup(t *testing.T, ns *Namespace, wait bool) {
	if wait {
		time.Sleep(gpmCleanupPeriod * 2)
	}

	if _, err := os.Stat(ns.Key()); err == nil {
		if wait {
			t.Fatalf("The sandbox path %s is not getting cleaned up even after twice the cleanup period", ns.Key())
		} else {
			t.Fatalf("The sandbox path %s is not cleaned up after running gc", ns.Key())
		}
	}
}

func TestDisableIPv6DAD(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()

	key, err := newKey(t)
	if err != nil {
		t.Fatalf("Failed to obtain a key: %v", err)
	}

	n, err := NewSandbox(key, true, false)
	if err != nil {
		t.Fatalf("Failed to create a new sandbox: %v", err)
	}
	defer destroyTest(t, n)

	nlh := n.nlHandle

	ipv6, _ := types.ParseCIDR("2001:db8::44/64")
	iface := &Interface{addressIPv6: ipv6, ns: n, dstName: "sideA"}

	veth := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{Name: "sideA"},
		PeerName:  "sideB",
	}

	err = nlh.LinkAdd(veth)
	if err != nil {
		t.Fatal(err)
	}

	link, err := nlh.LinkByName("sideA")
	if err != nil {
		t.Fatal(err)
	}

	err = setInterfaceIPv6(context.Background(), nlh, link, iface)
	if err != nil {
		t.Fatal(err)
	}

	addrList, err := nlh.AddrList(link, nl.FAMILY_V6)
	if err != nil {
		t.Fatal(err)
	}

	if addrList[0].Flags&syscall.IFA_F_NODAD == 0 {
		t.Fatalf("Unexpected interface flags: 0x%x. Expected to contain 0x%x", addrList[0].Flags, syscall.IFA_F_NODAD)
	}
}

func destroyTest(t *testing.T, ns *Namespace) {
	if err := ns.Destroy(); err != nil {
		t.Log(err)
	}
}

func TestSetInterfaceIP(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()

	key, err := newKey(t)
	if err != nil {
		t.Fatalf("Failed to obtain a key: %v", err)
	}

	n, err := NewSandbox(key, true, false)
	if err != nil {
		t.Fatalf("Failed to create a new sandbox: %v", err)
	}
	defer destroyTest(t, n)

	nlh := n.nlHandle

	ipv4, _ := types.ParseCIDR("172.30.0.33/24")
	ipv6, _ := types.ParseCIDR("2001:db8::44/64")
	iface := &Interface{address: ipv4, addressIPv6: ipv6, ns: n, dstName: "sideA"}

	if err := nlh.LinkAdd(&netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{Name: "sideA"},
		PeerName:  "sideB",
	}); err != nil {
		t.Fatal(err)
	}

	linkA, err := nlh.LinkByName("sideA")
	if err != nil {
		t.Fatal(err)
	}

	linkB, err := nlh.LinkByName("sideB")
	if err != nil {
		t.Fatal(err)
	}

	if err := nlh.LinkSetUp(linkA); err != nil {
		t.Fatal(err)
	}

	if err := nlh.LinkSetUp(linkB); err != nil {
		t.Fatal(err)
	}

	if err := setInterfaceIP(context.Background(), nlh, linkA, iface); err != nil {
		t.Fatal(err)
	}

	if err := setInterfaceIPv6(context.Background(), nlh, linkA, iface); err != nil {
		t.Fatal(err)
	}

	err = setInterfaceIP(context.Background(), nlh, linkB, iface)
	if err == nil {
		t.Fatalf("Expected route conflict error, but succeeded")
	}
	if !strings.Contains(err.Error(), "conflicts with existing route") {
		t.Fatalf("Unexpected error: %v", err)
	}

	err = setInterfaceIPv6(context.Background(), nlh, linkB, iface)
	if err == nil {
		t.Fatalf("Expected route conflict error, but succeeded")
	}
	if !strings.Contains(err.Error(), "conflicts with existing route") {
		t.Fatalf("Unexpected error: %v", err)
	}
}

func TestLiveRestore(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()

	key, err := newKey(t)
	if err != nil {
		t.Fatalf("Failed to obtain a key: %v", err)
	}

	n, err := NewSandbox(key, true, false)
	if err != nil {
		t.Fatalf("Failed to create a new sandbox: %v", err)
	}
	defer destroyTest(t, n)

	nlh := n.nlHandle

	ipv4, _ := types.ParseCIDR("172.30.0.33/24")
	ipv6, _ := types.ParseCIDR("2001:db8::44/64")
	iface := &Interface{address: ipv4, addressIPv6: ipv6, ns: n, dstName: "sideA"}

	if err := nlh.LinkAdd(&netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{Name: "sideA"},
		PeerName:  "sideB",
	}); err != nil {
		t.Fatal(err)
	}

	linkA, err := nlh.LinkByName("sideA")
	if err != nil {
		t.Fatal(err)
	}

	linkB, err := nlh.LinkByName("sideB")
	if err != nil {
		t.Fatal(err)
	}

	if err := nlh.LinkSetUp(linkA); err != nil {
		t.Fatal(err)
	}

	if err := nlh.LinkSetUp(linkB); err != nil {
		t.Fatal(err)
	}

	if err := setInterfaceIP(context.Background(), nlh, linkA, iface); err != nil {
		t.Fatal(err)
	}

	if err := setInterfaceIPv6(context.Background(), nlh, linkA, iface); err != nil {
		t.Fatal(err)
	}

	err = setInterfaceIP(context.Background(), nlh, linkB, iface)
	if err == nil {
		t.Fatalf("Expected route conflict error, but succeeded")
	}
	if !strings.Contains(err.Error(), "conflicts with existing route") {
		t.Fatalf("Unexpected error: %v", err)
	}

	err = setInterfaceIPv6(context.Background(), nlh, linkB, iface)
	if err == nil {
		t.Fatalf("Expected route conflict error, but succeeded")
	}
	if !strings.Contains(err.Error(), "conflicts with existing route") {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Create newsandbox with Restore - TRUE
	n2, err := NewSandbox(key, true, true)
	if err != nil {
		t.Fatalf("Failed to create a new sandbox: %v", err)
	}
	defer destroyTest(t, n2)

	// Check if the IPV4 & IPV6 entry present
	// If present , we should get error in below call
	// It shows us , we don't delete any config in live-restore case
	if err := setInterfaceIPv6(context.Background(), nlh, linkA, iface); err == nil {
		t.Fatalf("Expected route conflict error, but succeeded for IPV6 ")
	}
	if err := setInterfaceIP(context.Background(), nlh, linkA, iface); err == nil {
		t.Fatalf("Expected route conflict error, but succeeded for IPV4 ")
	}
}

func TestSandboxCreate(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()

	key, err := newKey(t)
	if err != nil {
		t.Fatalf("Failed to obtain a key: %v", err)
	}

	s, err := NewSandbox(key, true, false)
	if err != nil {
		t.Fatalf("Failed to create a new sandbox: %v", err)
	}

	if s.Key() != key {
		t.Fatalf("s.Key() returned %s. Expected %s", s.Key(), key)
	}

	tbox, err := newInfo(t, ns.NlHandle())
	if err != nil {
		t.Fatalf("Failed to generate new sandbox info: %v", err)
	}

	for _, i := range tbox.Interfaces() {
		err = s.AddInterface(context.Background(), i.SrcName(), i.DstName(),
			WithIsBridge(i.Bridge()),
			WithIPv4Address(i.Address()),
			WithIPv6Address(i.AddressIPv6()))
		if err != nil {
			t.Fatalf("Failed to add interfaces to sandbox: %v", err)
		}
	}

	err = s.SetGateway(tbox.Gateway())
	if err != nil {
		t.Fatalf("Failed to set gateway to sandbox: %v", err)
	}

	err = s.SetGatewayIPv6(tbox.GatewayIPv6())
	if err != nil {
		t.Fatalf("Failed to set ipv6 gateway to sandbox: %v", err)
	}

	verifySandbox(t, s, []string{"0", "1", "2"})

	err = s.Destroy()
	if err != nil {
		t.Fatal(err)
	}
	verifyCleanup(t, s, true)
}

func TestSandboxCreateTwice(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()

	key, err := newKey(t)
	if err != nil {
		t.Fatalf("Failed to obtain a key: %v", err)
	}

	_, err = NewSandbox(key, true, false)
	if err != nil {
		t.Fatalf("Failed to create a new sandbox: %v", err)
	}

	// Create another sandbox with the same key to see if we handle it
	// gracefully.
	s, err := NewSandbox(key, true, false)
	if err != nil {
		t.Fatalf("Failed to create a new sandbox: %v", err)
	}

	err = s.Destroy()
	if err != nil {
		t.Fatal(err)
	}
	GC()
	verifyCleanup(t, s, false)
}

func TestSandboxGC(t *testing.T) {
	key, err := newKey(t)
	if err != nil {
		t.Fatalf("Failed to obtain a key: %v", err)
	}

	s, err := NewSandbox(key, true, false)
	if err != nil {
		t.Fatalf("Failed to create a new sandbox: %v", err)
	}

	err = s.Destroy()
	if err != nil {
		t.Fatal(err)
	}

	GC()
	verifyCleanup(t, s, false)
}

func TestAddRemoveInterface(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()

	key, err := newKey(t)
	if err != nil {
		t.Fatalf("Failed to obtain a key: %v", err)
	}

	s, err := NewSandbox(key, true, false)
	if err != nil {
		t.Fatalf("Failed to create a new sandbox: %v", err)
	}

	if s.Key() != key {
		t.Fatalf("s.Key() returned %s. Expected %s", s.Key(), key)
	}

	tbox, err := newInfo(t, ns.NlHandle())
	if err != nil {
		t.Fatalf("Failed to generate new sandbox info: %v", err)
	}

	for _, i := range tbox.Interfaces() {
		err = s.AddInterface(context.Background(), i.SrcName(), i.DstName(),
			WithIsBridge(i.Bridge()),
			WithIPv4Address(i.Address()),
			WithIPv6Address(i.AddressIPv6()),
		)
		if err != nil {
			t.Fatalf("Failed to add interfaces to sandbox: %v", err)
		}
	}

	verifySandbox(t, s, []string{"0", "1", "2"})

	interfaces := s.Interfaces()
	if err := interfaces[0].Remove(); err != nil {
		t.Fatalf("Failed to remove interfaces from sandbox: %v", err)
	}

	verifySandbox(t, s, []string{"1", "2"})

	i := tbox.Interfaces()[0]
	err = s.AddInterface(context.Background(), i.SrcName(), i.DstName(),
		WithIsBridge(i.Bridge()),
		WithIPv4Address(i.Address()),
		WithIPv6Address(i.AddressIPv6()),
	)
	if err != nil {
		t.Fatalf("Failed to add interfaces to sandbox: %v", err)
	}

	verifySandbox(t, s, []string{"1", "2", "3"})

	err = s.Destroy()
	if err != nil {
		t.Fatal(err)
	}

	GC()
	verifyCleanup(t, s, false)
}
