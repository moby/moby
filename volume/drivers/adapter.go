package volumedrivers

import "github.com/docker/docker/volume"

type volumeDriverAdapter struct {
	name  string
	proxy *volumeDriverProxy
}

func (a *volumeDriverAdapter) Name() string {
	return a.name
}

func (a *volumeDriverAdapter) Create(name string) (volume.Volume, error) {
	err := a.proxy.Create(name)
	if err != nil {
		return nil, err
	}
	return &volumeAdapter{a.proxy, name, a.name}, nil
}

func (a *volumeDriverAdapter) Remove(v volume.Volume) error {
	return a.proxy.Remove(v.Name())
}

type volumeAdapter struct {
	proxy      *volumeDriverProxy
	name       string
	driverName string
}

func (a *volumeAdapter) Name() string {
	return a.name
}

func (a *volumeAdapter) DriverName() string {
	return a.driverName
}

func (a *volumeAdapter) Path() string {
	m, _ := a.proxy.Path(a.name)
	return m
}

func (a *volumeAdapter) Mount() (string, error) {
	return a.proxy.Mount(a.name)
}

func (a *volumeAdapter) Unmount() error {
	return a.proxy.Unmount(a.name)
}
