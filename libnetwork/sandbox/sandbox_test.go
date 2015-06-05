package sandbox

import (
	"os"
	"runtime"
	"testing"

	"github.com/docker/docker/pkg/reexec"
	"github.com/docker/libnetwork/netutils"
)

func TestMain(m *testing.M) {
	if reexec.Init() {
		return
	}
	os.Exit(m.Run())
}

func TestSandboxCreate(t *testing.T) {
	defer netutils.SetupTestNetNS(t)()

	key, err := newKey(t)
	if err != nil {
		t.Fatalf("Failed to obtain a key: %v", err)
	}

	s, err := NewSandbox(key, true)
	if err != nil {
		t.Fatalf("Failed to create a new sandbox: %v", err)
	}
	runtime.LockOSThread()

	if s.Key() != key {
		t.Fatalf("s.Key() returned %s. Expected %s", s.Key(), key)
	}

	tbox, err := newInfo(t)
	if err != nil {
		t.Fatalf("Failed to generate new sandbox info: %v", err)
	}

	for _, i := range tbox.Info().Interfaces() {
		err = s.AddInterface(i.SrcName(), i.DstName(),
			tbox.InterfaceOptions().Address(i.Address()),
			tbox.InterfaceOptions().AddressIPv6(i.AddressIPv6()))
		if err != nil {
			t.Fatalf("Failed to add interfaces to sandbox: %v", err)
		}
		runtime.LockOSThread()
	}

	err = s.SetGateway(tbox.Info().Gateway())
	if err != nil {
		t.Fatalf("Failed to set gateway to sandbox: %v", err)
	}
	runtime.LockOSThread()

	err = s.SetGatewayIPv6(tbox.Info().GatewayIPv6())
	if err != nil {
		t.Fatalf("Failed to set ipv6 gateway to sandbox: %v", err)
	}
	runtime.LockOSThread()

	verifySandbox(t, s, []string{"0", "1"})
	runtime.LockOSThread()

	s.Destroy()
	verifyCleanup(t, s, true)
}

func TestSandboxCreateTwice(t *testing.T) {
	defer netutils.SetupTestNetNS(t)()

	key, err := newKey(t)
	if err != nil {
		t.Fatalf("Failed to obtain a key: %v", err)
	}

	_, err = NewSandbox(key, true)
	if err != nil {
		t.Fatalf("Failed to create a new sandbox: %v", err)
	}
	runtime.LockOSThread()

	// Create another sandbox with the same key to see if we handle it
	// gracefully.
	s, err := NewSandbox(key, true)
	if err != nil {
		t.Fatalf("Failed to create a new sandbox: %v", err)
	}
	runtime.LockOSThread()
	s.Destroy()
}

func TestSandboxGC(t *testing.T) {
	key, err := newKey(t)
	if err != nil {
		t.Fatalf("Failed to obtain a key: %v", err)
	}

	s, err := NewSandbox(key, true)
	if err != nil {
		t.Fatalf("Failed to create a new sandbox: %v", err)
	}

	s.Destroy()

	GC()
	verifyCleanup(t, s, false)
}

func TestAddRemoveInterface(t *testing.T) {
	defer netutils.SetupTestNetNS(t)()

	key, err := newKey(t)
	if err != nil {
		t.Fatalf("Failed to obtain a key: %v", err)
	}

	s, err := NewSandbox(key, true)
	if err != nil {
		t.Fatalf("Failed to create a new sandbox: %v", err)
	}
	runtime.LockOSThread()

	if s.Key() != key {
		t.Fatalf("s.Key() returned %s. Expected %s", s.Key(), key)
	}

	tbox, err := newInfo(t)
	if err != nil {
		t.Fatalf("Failed to generate new sandbox info: %v", err)
	}

	for _, i := range tbox.Info().Interfaces() {
		err = s.AddInterface(i.SrcName(), i.DstName(),
			tbox.InterfaceOptions().Address(i.Address()),
			tbox.InterfaceOptions().AddressIPv6(i.AddressIPv6()))
		if err != nil {
			t.Fatalf("Failed to add interfaces to sandbox: %v", err)
		}
		runtime.LockOSThread()
	}

	verifySandbox(t, s, []string{"0", "1"})
	runtime.LockOSThread()

	interfaces := s.Info().Interfaces()
	if err := interfaces[0].Remove(); err != nil {
		t.Fatalf("Failed to remove interfaces from sandbox: %v", err)
	}
	runtime.LockOSThread()

	verifySandbox(t, s, []string{"1"})
	runtime.LockOSThread()

	i := tbox.Info().Interfaces()[0]
	if err := s.AddInterface(i.SrcName(), i.DstName(),
		tbox.InterfaceOptions().Address(i.Address()),
		tbox.InterfaceOptions().AddressIPv6(i.AddressIPv6())); err != nil {
		t.Fatalf("Failed to add interfaces to sandbox: %v", err)
	}
	runtime.LockOSThread()

	verifySandbox(t, s, []string{"1", "2"})
	runtime.LockOSThread()

	s.Destroy()
}
