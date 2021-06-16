//go:generate pluginrpc-gen -i $GOFILE -o proxy.go -type volumeDriver -name VolumeDriver

package drivers // import "github.com/docker/docker/volume/drivers"

import (
	"fmt"
	"sort"
	"sync"

	"github.com/docker/docker/errdefs"
	getter "github.com/docker/docker/pkg/plugingetter"
	"github.com/docker/docker/pkg/plugins"
	"github.com/docker/docker/volume"
	"github.com/moby/locker"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const extName = "VolumeDriver"

// volumeDriver defines the available functions that volume plugins must implement.
// This interface is only defined to generate the proxy objects.
// It's not intended to be public or reused.
//nolint: deadcode
type volumeDriver interface {
	// Create a volume with the given name
	Create(name string, opts map[string]string) (err error)
	// Remove the volume with the given name
	Remove(name string) (err error)
	// Get the mountpoint of the given volume
	Path(name string) (mountpoint string, err error)
	// Mount the given volume and return the mountpoint
	Mount(name, id string) (mountpoint string, err error)
	// Unmount the given volume
	Unmount(name, id string) (err error)
	// List lists all the volumes known to the driver
	List() (volumes []*proxyVolume, err error)
	// Get retrieves the volume with the requested name
	Get(name string) (volume *proxyVolume, err error)
	// Capabilities gets the list of capabilities of the driver
	Capabilities() (capabilities volume.Capability, err error)
}

// Store is an in-memory store for volume drivers
type Store struct {
	extensions   map[string]volume.Driver
	mu           sync.Mutex
	driverLock   *locker.Locker
	pluginGetter getter.PluginGetter
}

// NewStore creates a new volume driver store
func NewStore(pg getter.PluginGetter) *Store {
	return &Store{
		extensions:   make(map[string]volume.Driver),
		driverLock:   locker.New(),
		pluginGetter: pg,
	}
}

type driverNotFoundError string

func (e driverNotFoundError) Error() string {
	return "volume driver not found: " + string(e)
}

func (driverNotFoundError) NotFound() {}

// lookup returns the driver associated with the given name. If a
// driver with the given name has not been registered it checks if
// there is a VolumeDriver plugin available with the given name.
func (s *Store) lookup(name string, mode int) (volume.Driver, error) {
	if name == "" {
		return nil, errdefs.InvalidParameter(errors.New("driver name cannot be empty"))
	}
	s.driverLock.Lock(name)
	defer s.driverLock.Unlock(name)

	s.mu.Lock()
	ext, ok := s.extensions[name]
	s.mu.Unlock()
	if ok {
		return ext, nil
	}
	if s.pluginGetter != nil {
		p, err := s.pluginGetter.Get(name, extName, mode)
		if err != nil {
			return nil, errors.Wrap(err, "error looking up volume plugin "+name)
		}

		d, err := makePluginAdapter(p)
		if err != nil {
			return nil, errors.Wrap(err, "error making plugin client")
		}
		if err := validateDriver(d); err != nil {
			if mode > 0 {
				// Undo any reference count changes from the initial `Get`
				if _, err := s.pluginGetter.Get(name, extName, mode*-1); err != nil {
					logrus.WithError(err).WithField("action", "validate-driver").WithField("plugin", name).Error("error releasing reference to plugin")
				}
			}
			return nil, err
		}

		if p.IsV1() {
			s.mu.Lock()
			s.extensions[name] = d
			s.mu.Unlock()
		}
		return d, nil
	}
	return nil, driverNotFoundError(name)
}

func validateDriver(vd volume.Driver) error {
	scope := vd.Scope()
	if scope != volume.LocalScope && scope != volume.GlobalScope {
		return fmt.Errorf("Driver %q provided an invalid capability scope: %s", vd.Name(), scope)
	}
	return nil
}

// Register associates the given driver to the given name, checking if
// the name is already associated
func (s *Store) Register(d volume.Driver, name string) bool {
	if name == "" {
		return false
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.extensions[name]; exists {
		return false
	}

	if err := validateDriver(d); err != nil {
		return false
	}

	s.extensions[name] = d
	return true
}

// GetDriver returns a volume driver by its name.
// If the driver is empty, it looks for the local driver.
func (s *Store) GetDriver(name string) (volume.Driver, error) {
	return s.lookup(name, getter.Lookup)
}

// CreateDriver returns a volume driver by its name and increments RefCount.
// If the driver is empty, it looks for the local driver.
func (s *Store) CreateDriver(name string) (volume.Driver, error) {
	return s.lookup(name, getter.Acquire)
}

// ReleaseDriver returns a volume driver by its name and decrements RefCount..
// If the driver is empty, it looks for the local driver.
func (s *Store) ReleaseDriver(name string) (volume.Driver, error) {
	return s.lookup(name, getter.Release)
}

// GetDriverList returns list of volume drivers registered.
// If no driver is registered, empty string list will be returned.
func (s *Store) GetDriverList() []string {
	var driverList []string
	s.mu.Lock()
	defer s.mu.Unlock()
	for driverName := range s.extensions {
		driverList = append(driverList, driverName)
	}
	sort.Strings(driverList)
	return driverList
}

// GetAllDrivers lists all the registered drivers
func (s *Store) GetAllDrivers() ([]volume.Driver, error) {
	var plugins []getter.CompatPlugin
	if s.pluginGetter != nil {
		var err error
		plugins, err = s.pluginGetter.GetAllByCap(extName)
		if err != nil {
			return nil, fmt.Errorf("error listing plugins: %v", err)
		}
	}
	var ds []volume.Driver

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, d := range s.extensions {
		ds = append(ds, d)
	}

	for _, p := range plugins {
		name := p.Name()

		if _, ok := s.extensions[name]; ok {
			continue
		}

		ext, err := makePluginAdapter(p)
		if err != nil {
			return nil, errors.Wrap(err, "error making plugin client")
		}
		if p.IsV1() {
			s.extensions[name] = ext
		}
		ds = append(ds, ext)
	}
	return ds, nil
}

func makePluginAdapter(p getter.CompatPlugin) (*volumeDriverAdapter, error) {
	if pc, ok := p.(getter.PluginWithV1Client); ok {
		return &volumeDriverAdapter{name: p.Name(), scopePath: p.ScopedPath, proxy: &volumeDriverProxy{pc.Client()}}, nil
	}

	pa, ok := p.(getter.PluginAddr)
	if !ok {
		return nil, errdefs.System(errors.Errorf("got unknown plugin instance %T", p))
	}

	if pa.Protocol() != plugins.ProtocolSchemeHTTPV1 {
		return nil, errors.Errorf("plugin protocol not supported: %s", p)
	}

	addr := pa.Addr()
	client, err := plugins.NewClientWithTimeout(addr.Network()+"://"+addr.String(), nil, pa.Timeout())
	if err != nil {
		return nil, errors.Wrap(err, "error creating plugin client")
	}

	return &volumeDriverAdapter{name: p.Name(), scopePath: p.ScopedPath, proxy: &volumeDriverProxy{client}}, nil
}
