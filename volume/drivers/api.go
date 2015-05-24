package volumedrivers

import "github.com/docker/docker/volume"

type client interface {
	Call(string, interface{}, interface{}) error
}

func NewVolumeDriver(name string, c client) volume.Driver {
	proxy := &volumeDriverProxy{c}
	return &volumeDriverAdapter{name, proxy}
}

type VolumeDriver interface {
	Create(name string) (err error)
	Remove(name string) (err error)
	Path(name string) (mountpoint string, err error)
	Mount(name string) (mountpoint string, err error)
	Unmount(name string) (err error)
}
