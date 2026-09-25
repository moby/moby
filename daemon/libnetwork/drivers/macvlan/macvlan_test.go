//go:build linux

package macvlan

import (
	"testing"

	"github.com/moby/moby/v2/daemon/libnetwork/driverapi"
	"github.com/moby/moby/v2/internal/testutil/storeutils"
)

const testNetworkType = "macvlan"

type driverTester struct {
	t *testing.T
	d *driver
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

func TestMacvlanRegister(t *testing.T) {
	if err := Register(&driverTester{t: t}, storeutils.NewTempStore(t)); err != nil {
		t.Fatal(err)
	}
}

func TestMacvlanNilConfig(t *testing.T) {
	dt := &driverTester{t: t}
	if err := Register(dt, storeutils.NewTempStore(t)); err != nil {
		t.Fatal(err)
	}

	if err := dt.d.initStore(); err != nil {
		t.Fatal(err)
	}
}

func TestMacvlanType(t *testing.T) {
	dt := &driverTester{t: t}
	if err := Register(dt, storeutils.NewTempStore(t)); err != nil {
		t.Fatal(err)
	}

	if dt.d.Type() != testNetworkType {
		t.Fatalf("Expected Type() to return %q. Instead got %q", testNetworkType,
			dt.d.Type())
	}
}
