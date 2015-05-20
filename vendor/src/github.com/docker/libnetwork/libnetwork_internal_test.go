package libnetwork

import (
	"testing"

	"github.com/docker/libnetwork/driverapi"
)

func TestDriverRegistration(t *testing.T) {
	bridgeNetType := "bridge"
	c, err := New()
	if err != nil {
		t.Fatal(err)
	}
	err = c.(*controller).RegisterDriver(bridgeNetType, nil)
	if err == nil {
		t.Fatalf("Expecting the RegisterDriver to fail for %s", bridgeNetType)
	}
	if _, ok := err.(driverapi.ErrActiveRegistration); !ok {
		t.Fatalf("Failed for unexpected reason: %v", err)
	}
	err = c.(*controller).RegisterDriver("test-dummy", nil)
	if err != nil {
		t.Fatalf("Test failed with an error %v", err)
	}
}
