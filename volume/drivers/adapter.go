package volumedrivers

import (
	"sync"

	"github.com/docker/docker/volume"
)

type volumeDriverAdapter struct {
	name     string
	proxy    *volumeDriverProxy
	adapters map[string]*volumeAdapter
}

func (a *volumeDriverAdapter) Name() string {
	return a.name
}

func (a *volumeDriverAdapter) Create(name string) (volume.Volume, error) {
	err := a.proxy.Create(name)
	if err != nil {
		return nil, err
	}

	if _, ok := a.adapters[name]; !ok {
		a.adapters[name] = &volumeAdapter{
			proxy:         a.proxy,
			name:          name,
			driverName:    a.name,
			driverAdapter: a}
	}

	return a.adapters[name], nil
}

func (a *volumeDriverAdapter) Remove(v volume.Volume) error {
	if v.UsedCount() == 0 {
		return a.proxy.Remove(v.Name())
	}

	return nil
}

type volumeAdapter struct {
	proxy         *volumeDriverProxy
	name          string
	driverName    string
	eMount        string // ephemeral host volume path
	usedCount     int
	m             sync.Mutex
	driverAdapter *volumeDriverAdapter
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
	defer a.use()
	return a.proxy.Mount(a.name)
}

func (a *volumeAdapter) Unmount() error {
	defer a.release()

	if a.usedCount == 1 {
		return a.proxy.Unmount(a.name)
	}

	return nil
}

func (a *volumeAdapter) UsedCount() int {
	return a.usedCount
}

func (a *volumeAdapter) use() {
	a.m.Lock()
	a.usedCount++
	a.m.Unlock()
}

func (a *volumeAdapter) release() {
	a.m.Lock()
	a.usedCount--
	a.m.Unlock()
}
