package sandbox

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/docker/libnetwork/netutils"
	"github.com/vishvananda/netns"
)

func newKey(t *testing.T) (string, error) {
	name, err := netutils.GenerateRandomName("netns", 12)
	if err != nil {
		return "", err
	}

	name = filepath.Join("/tmp", name)
	if _, err := os.Create(name); err != nil {
		return "", err
	}

	return name, nil
}

func verifySandbox(t *testing.T, s Sandbox) {
	_, ok := s.(*networkNamespace)
	if !ok {
		t.Fatalf("The sandox interface returned is not of type networkNamespace")
	}

	origns, err := netns.Get()
	if err != nil {
		t.Fatalf("Could not get the current netns: %v", err)
	}
	defer origns.Close()

	f, err := os.OpenFile(s.Key(), os.O_RDONLY, 0)
	if err != nil {
		t.Fatalf("Failed top open network namespace path %q: %v", s.Key(), err)
	}
	defer f.Close()

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	nsFD := f.Fd()
	if err = netns.Set(netns.NsHandle(nsFD)); err != nil {
		t.Fatalf("Setting to the namespace pointed to by the sandbox %s failed: %v", s.Key(), err)
	}

	netns.Set(origns)
}
