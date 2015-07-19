package volumedrivers

import "github.com/docker/docker/volume"

type volumeDriverAdapter struct {
	name  string
	proxy *volumeDriverProxy
}

func (a *volumeDriverAdapter) Name() string {
	return a.name
}

func (a *volumeDriverAdapter) Create(name, id string) (volume.Volume, error) {
	err := a.proxy.Create(name, id)
	if err != nil {
		return nil, err
	}
	return &volumeAdapter{
		proxy:      a.proxy,
		name:       name,
		id:         id,
		driverName: a.name}, nil
}

func (a *volumeDriverAdapter) Remove(v volume.Volume) error {
	return a.proxy.Remove(v.Name(), v.Id())
}

type volumeAdapter struct {
	proxy      *volumeDriverProxy
	name       string
	id         string
	driverName string
	eMount     string // ephemeral host volume path
}

func (a *volumeAdapter) Name() string {
	return a.name
}

func (a *volumeAdapter) Id() string {
	return a.id
}

func (a *volumeAdapter) DriverName() string {
	return a.driverName
}

func (a *volumeAdapter) Path() string {
	if len(a.eMount) > 0 {
		return a.eMount
	}
	m, _ := a.proxy.Path(a.name, a.id)
	return m
}

func (a *volumeAdapter) Mount() (string, error) {
	var err error
	a.eMount, err = a.proxy.Mount(a.name, a.id)
	return a.eMount, err
}

func (a *volumeAdapter) Unmount() error {
	return a.proxy.Unmount(a.name, a.id)
}
