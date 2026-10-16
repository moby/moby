//go:build linux

package overlay

import (
	"testing"

	"github.com/moby/moby/v2/daemon/libnetwork/driverapi"
	"github.com/moby/moby/v2/pkg/plugingetter"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

type driverTester struct {
	t *testing.T
	d *driver
}

const testNetworkType = "overlay"

func (dt *driverTester) GetPluginGetter() plugingetter.PluginGetter {
	return nil
}

func (dt *driverTester) RegisterDriver(name string, drv driverapi.Driver, capability driverapi.Capability) error {
	if name != testNetworkType {
		dt.t.Fatalf("Expected driver register name to be %q. Instead got %q",
			testNetworkType, name)
	}

	if _, ok := drv.(*driver); !ok {
		dt.t.Fatalf("Expected driver type to be %T. Instead got %T",
			&driver{}, drv)
	}

	dt.d = drv.(*driver)
	return nil
}

func (dt *driverTester) RegisterNetworkAllocator(name string, _ driverapi.NetworkAllocator) error {
	dt.t.Fatalf("Unexpected call to RegisterNetworkAllocator for %q", name)
	return nil
}

func TestOverlayInit(t *testing.T) {
	if err := Register(&driverTester{t: t}, t.TempDir()); err != nil {
		t.Fatal(err)
	}
}

// populateVNITbl recognizes an overlay sandbox by the "-" in its name, and
// cleanupStaleSandboxes recognizes its network by the part of the name after
// the "-", a prefix of the network's ID.
func TestSandboxKey(t *testing.T) {
	const nid = "0123456789abcdef0123456789abcdef"
	assert.Check(t, is.Equal(sandboxKey("/run/docker/netns", 1, nid), "/run/docker/netns/1-0123456789"))
	assert.Check(t, is.Equal(sandboxKey("/run/docker/netns", 10, nid), "/run/docker/netns/10-012345678"))
}
