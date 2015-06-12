package localvolumedriver

import (
	volumedrivers "github.com/docker/docker/volume/drivers"
)

func init() {
	volumedrivers.RegisterDriver("VolumeDriver", Init)
}

func Init(name string) (volumedrivers.Driver, error) {
	return &volumeDriverAdapter{}, nil
}

type volumeDriverAdapter struct {
	name    string
	proxy   *volumeDriverProxy
	volumes map[string]*volumeAdapter
}

func (a *volumeDriverAdapter) Name() string {
	return a.name
}

func (a *volumeDriverAdapter) Create(name string, opts volumedrivers.Opts) (volumedrivers.Volume, error) {
	err := a.proxy.Create(name, opts)
	if err != nil {
		return nil, err
	}

	a.volumes[name] = &volumeAdapter{
		name:       name,
		driverName: a.name,
		proxy:      a.proxy,
	}

	return a.volumes[name], nil
}

func (a *volumeDriverAdapter) Remove(v volumedrivers.Volume) error {
	delete(a.volumes, v.Name())
	return a.proxy.Remove(v.Name())
}

func (a *volumeDriverAdapter) GetVolume(name string) volumedrivers.Volume {
	if volume, ok := a.volumes[name]; ok {
		return volume
	}
	return nil
}

func (a *volumeDriverAdapter) New(name string, c interface{}) volumedrivers.Driver {
	proxy := NewProxy(c.(client))
	return &volumeDriverAdapter{name, proxy, make(map[string]*volumeAdapter)}
}

type volumeAdapter struct {
	proxy      *volumeDriverProxy
	name       string
	driverName string
	eMount     string // ephemeral host volume path
}

func (a *volumeAdapter) Name() string {
	return a.name
}

func (a *volumeAdapter) DriverName() string {
	return a.driverName
}

func (a *volumeAdapter) Path() string {
	if len(a.eMount) > 0 {
		return a.eMount
	}
	m, _ := a.proxy.Path(a.name)
	return m
}

func (a *volumeAdapter) Mount() (string, error) {
	var err error
	a.eMount, err = a.proxy.Mount(a.name)
	return a.eMount, err
}

func (a *volumeAdapter) Unmount() error {
	return a.proxy.Unmount(a.name)
}
