package devmapper

import (
	"github.com/dotcloud/docker/archive"
)

type Image interface {
	ID() string
	Parent() (Image, error)
	Path() string
}

type DMBackend struct {
	*DeviceSet
}

func (b *DMBackend) Init(home string) error {
	b.DeviceSet = NewDeviceSet(home)
	if err := b.DeviceSet.ensureInit(); err != nil {
		return err
	}
	return nil
}

func (b *DMBackend) Shutdown() error {
	return b.DeviceSet.Shutdown()
}

func (b *DMBackend) Create(img image, layer archive.Archive) error {
	// Determine the source of the snapshot (parent id or init device)
	var parentID string
	if parent, err := img.Parent(); err != nil {
		return err
	} else if parent != nil {
		parentID = parent.ID()
	}
	// Create the device for this image by snapshotting source
	if err := b.DeviceSet.AddDevice(img.ID(), parentID); err != nil {
		return err
	}
	// Mount the device in rootfs
	mp := b.mountpoint(id)
	if err := os.MkdirAll(mp); err != nil {
		return err
	}
	if err := b.DeviceSet.MountDevice(id, mp, false); err != nil {
		return err
	}
	// Apply the layer as a diff 
	if layer != nil {
		if err := archive.ApplyLayer(mp, layer); err != nil {
			return err
		}
	}
	return nil
}


func (b *DMBackend) mountpoint(id string) string {
	if b.home == "" {
		return ""
	}
	return path.Join(b.home, "mnt", id)
}

func (b *DMBackend) Changes(img *Image, dest string) ([]Change, error) {
	return nil, fmt.Errorf("Not implemented")
}

func (b *DMBackend) Layer(img *Image, dest string) (Archive, error) {
	return nil, fmt.Errorf("Not implemented")
}
