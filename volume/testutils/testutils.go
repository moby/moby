package volumetestutils

import (
	"fmt"

	"github.com/docker/docker/volume"
)

// NoopVolume is a volume that doesn't perform any operation
type NoopVolume struct{}

// Name is the name of the volume
func (NoopVolume) Name() string { return "noop" }

// DriverName is the name of the driver
func (NoopVolume) DriverName() string { return "noop" }

// Path is the filesystem path to the volume
func (NoopVolume) Path() string { return "noop" }

// Mount mounts the volume in the container
func (NoopVolume) Mount() (string, error) { return "noop", nil }

// Unmount unmounts the volume from the container
func (NoopVolume) Unmount() error { return nil }

// FakeVolume is a fake volume with a random name
type FakeVolume struct {
	name string
}

// NewFakeVolume creates a new fake volume for testing
func NewFakeVolume(name string) volume.Volume {
	return FakeVolume{name: name}
}

// Name is the name of the volume
func (f FakeVolume) Name() string { return f.name }

// DriverName is the name of the driver
func (FakeVolume) DriverName() string { return "fake" }

// Path is the filesystem path to the volume
func (FakeVolume) Path() string { return "fake" }

// Mount mounts the volume in the container
func (FakeVolume) Mount() (string, error) { return "fake", nil }

// Unmount unmounts the volume from the container
func (FakeVolume) Unmount() error { return nil }

// FakeDriver is a driver that generates fake volumes
type FakeDriver struct{}

// Name is the name of the driver
func (FakeDriver) Name() string { return "fake" }

// Create initializes a fake volume.
// It returns an error if the options include an "error" key with a message
func (FakeDriver) Create(name string, opts map[string]string) (volume.Volume, error) {
	if opts != nil && opts["error"] != "" {
		return nil, fmt.Errorf(opts["error"])
	}
	return NewFakeVolume(name), nil
}

// Remove deletes a volume.
func (FakeDriver) Remove(v volume.Volume) error { return nil }
