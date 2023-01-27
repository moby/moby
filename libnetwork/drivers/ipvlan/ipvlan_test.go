//go:build linux
// +build linux

package ipvlan

import (
	"testing"

	"github.com/docker/docker/libnetwork/driverapi"
)

const testNetworkType = "ipvlan"

type driverTester struct {
	t *testing.T
	d *driver
}

func (dt *driverTester) RegisterDriver(name string, drv driverapi.Driver,
	cap driverapi.Capability) error {
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

func TestIpvlanRegister(t *testing.T) {
	if err := Register(&driverTester{t: t}, nil); err != nil {
		t.Fatal(err)
	}
}

func TestIpvlanNilConfig(t *testing.T) {
	dt := &driverTester{t: t}
	if err := Register(dt, nil); err != nil {
		t.Fatal(err)
	}

	if err := dt.d.initStore(nil); err != nil {
		t.Fatal(err)
	}
}

func TestIpvlanType(t *testing.T) {
	dt := &driverTester{t: t}
	if err := Register(dt, nil); err != nil {
		t.Fatal(err)
	}

	if dt.d.Type() != testNetworkType {
		t.Fatalf("Expected Type() to return %q. Instead got %q", testNetworkType,
			dt.d.Type())
	}
}
