package drivers // import "github.com/docker/docker/volume/drivers"

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/containerd/log"
	"github.com/docker/docker/volume"
)

var errNoSuchVolume = errors.New("no such volume")

type volumeDriverAdapter struct {
	name         string
	scopePath    func(s string) string
	capabilities *volume.Capability
	proxy        volumeDriver
}

func (a *volumeDriverAdapter) Name() string {
	return a.name
}

func (a *volumeDriverAdapter) Create(name string, opts map[string]string) (volume.Volume, error) {
	if err := a.proxy.Create(name, opts); err != nil {
		return nil, err
	}
	return &volumeAdapter{
		proxy:      a.proxy,
		name:       name,
		driverName: a.name,
		scopePath:  a.scopePath,
	}, nil
}

func (a *volumeDriverAdapter) Remove(v volume.Volume) error {
	return a.proxy.Remove(v.Name())
}

func (a *volumeDriverAdapter) List() ([]volume.Volume, error) {
	ls, err := a.proxy.List()
	if err != nil {
		return nil, err
	}

	var out []volume.Volume
	for _, vp := range ls {
		out = append(out, &volumeAdapter{
			proxy:      a.proxy,
			name:       vp.Name,
			scopePath:  a.scopePath,
			driverName: a.name,
			eMount:     a.scopePath(vp.Mountpoint),
		})
	}
	return out, nil
}

func (a *volumeDriverAdapter) Get(name string) (volume.Volume, error) {
	v, err := a.proxy.Get(name)
	if err != nil {
		return nil, err
	}

	// plugin may have returned no volume and no error
	if v == nil {
		return nil, errNoSuchVolume
	}

	return &volumeAdapter{
		proxy:      a.proxy,
		name:       v.Name,
		driverName: a.Name(),
		eMount:     v.Mountpoint,
		createdAt:  v.CreatedAt,
		status:     v.Status,
		scopePath:  a.scopePath,
	}, nil
}

func (a *volumeDriverAdapter) Scope() string {
	cap := a.getCapabilities()
	return cap.Scope
}

func (a *volumeDriverAdapter) getCapabilities() volume.Capability {
	if a.capabilities != nil {
		return *a.capabilities
	}
	cap, err := a.proxy.Capabilities()
	if err != nil {
		// `GetCapabilities` is a not a required endpoint.
		// On error assume it's a local-only driver
		log.G(context.TODO()).WithError(err).WithField("driver", a.name).Debug("Volume driver returned an error while trying to query its capabilities, using default capabilities")
		return volume.Capability{Scope: volume.LocalScope}
	}

	// don't spam the warn log below just because the plugin didn't provide a scope
	if len(cap.Scope) == 0 {
		cap.Scope = volume.LocalScope
	}

	cap.Scope = strings.ToLower(cap.Scope)
	if cap.Scope != volume.LocalScope && cap.Scope != volume.GlobalScope {
		log.G(context.TODO()).WithField("driver", a.Name()).WithField("scope", a.Scope).Warn("Volume driver returned an invalid scope")
		cap.Scope = volume.LocalScope
	}

	a.capabilities = &cap
	return cap
}

type volumeAdapter struct {
	proxy      volumeDriver
	name       string
	scopePath  func(string) string
	driverName string
	eMount     string    // ephemeral host volume path
	createdAt  time.Time // time the directory was created
	status     map[string]interface{}
}

type proxyVolume struct {
	Name       string
	Mountpoint string
	CreatedAt  time.Time
	Status     map[string]interface{}
}

func (a *volumeAdapter) Name() string {
	return a.name
}

func (a *volumeAdapter) DriverName() string {
	return a.driverName
}

func (a *volumeAdapter) Path() string {
	if len(a.eMount) == 0 {
		mountpoint, _ := a.proxy.Path(a.name)
		a.eMount = a.scopePath(mountpoint)
	}
	return a.eMount
}

func (a *volumeAdapter) CachedPath() string {
	return a.eMount
}

func (a *volumeAdapter) Mount(id string) (string, error) {
	mountpoint, err := a.proxy.Mount(a.name, id)
	a.eMount = a.scopePath(mountpoint)
	return a.eMount, err
}

func (a *volumeAdapter) Unmount(id string) error {
	err := a.proxy.Unmount(a.name, id)
	if err == nil {
		a.eMount = ""
	}
	return err
}

func (a *volumeAdapter) CreatedAt() (time.Time, error) {
	return a.createdAt, nil
}

func (a *volumeAdapter) Status() map[string]interface{} {
	out := make(map[string]interface{}, len(a.status))
	for k, v := range a.status {
		out[k] = v
	}
	return out
}
