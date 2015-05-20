// +build experimental

package daemon

import (
	"testing"

	"github.com/docker/docker/volume"
	"github.com/docker/docker/volume/drivers"
)

type fakeDriver struct{}

func (fakeDriver) Name() string                              { return "fake" }
func (fakeDriver) Create(name string) (volume.Volume, error) { return nil, nil }
func (fakeDriver) Remove(v volume.Volume) error              { return nil }

func TestGetVolumeDriver(t *testing.T) {
	_, err := getVolumeDriver("missing")
	if err == nil {
		t.Fatal("Expected error, was nil")
	}

	volumedrivers.Register(fakeDriver{}, "fake")
	d, err := getVolumeDriver("fake")
	if err != nil {
		t.Fatal(err)
	}
	if d.Name() != "fake" {
		t.Fatalf("Expected fake driver, got %s\n", d.Name())
	}
}
