package volumedrivers

import (
	"testing"

	"github.com/docker/docker/volume/testutils"
)

func TestGetDriver(t *testing.T) {
	_, err := GetDriver("missing")
	if err == nil {
		t.Fatal("Expected error, was nil")
	}

	Register(volumetestutils.FakeDriver{}, "fake")
	d, err := GetDriver("fake")
	if err != nil {
		t.Fatal(err)
	}
	if d.Name() != "fake" {
		t.Fatalf("Expected fake driver, got %s\n", d.Name())
	}
}
