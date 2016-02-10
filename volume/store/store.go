package store

import (
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/pkg/locker"
	"github.com/docker/docker/volume"
	"github.com/docker/docker/volume/drivers"
)

// New initializes a VolumeStore to keep
// reference counting of volumes in the system.
func New() *VolumeStore {
	return &VolumeStore{
		locks: &locker.Locker{},
		names: make(map[string]volume.Volume),
		refs:  make(map[string][]string),
	}
}

func (s *VolumeStore) getNamed(name string) (volume.Volume, bool) {
	s.globalLock.Lock()
	v, exists := s.names[name]
	s.globalLock.Unlock()
	return v, exists
}

func (s *VolumeStore) setNamed(v volume.Volume, ref string) {
	s.globalLock.Lock()
	s.names[v.Name()] = v
	if len(ref) > 0 {
		s.refs[v.Name()] = append(s.refs[v.Name()], ref)
	}
	s.globalLock.Unlock()
}

func (s *VolumeStore) purge(name string) {
	s.globalLock.Lock()
	delete(s.names, name)
	delete(s.refs, name)
	s.globalLock.Unlock()
}

// VolumeStore is a struct that stores the list of volumes available and keeps track of their usage counts
type VolumeStore struct {
	locks      *locker.Locker
	globalLock sync.Mutex
	// names stores the volume name -> driver name relationship.
	// This is used for making lookups faster so we don't have to probe all drivers
	names map[string]volume.Volume
	// refs stores the volume name and the list of things referencing it
	refs map[string][]string
}

// List proxies to all registered volume drivers to get the full list of volumes
// If a driver returns a volume that has name which conflicts with a another volume from a different driver,
// the first volume is chosen and the conflicting volume is dropped.
func (s *VolumeStore) List() ([]volume.Volume, []string, error) {
	vols, warnings, err := s.list()
	if err != nil {
		return nil, nil, &OpErr{Err: err, Op: "list"}
	}
	var out []volume.Volume

	for _, v := range vols {
		name := normaliseVolumeName(v.Name())

		s.locks.Lock(name)
		storedV, exists := s.getNamed(name)
		if !exists {
			s.setNamed(v, "")
		}
		if exists && storedV.DriverName() != v.DriverName() {
			logrus.Warnf("Volume name %s already exists for driver %s, not including volume returned by %s", v.Name(), storedV.DriverName(), v.DriverName())
			s.locks.Unlock(v.Name())
			continue
		}

		out = append(out, v)
		s.locks.Unlock(v.Name())
	}
	return out, warnings, nil
}

// list goes through each volume driver and asks for its list of volumes.
func (s *VolumeStore) list() ([]volume.Volume, []string, error) {
	drivers, err := volumedrivers.GetAllDrivers()
	if err != nil {
		return nil, nil, err
	}
	var (
		ls       []volume.Volume
		warnings []string
	)

	type vols struct {
		vols       []volume.Volume
		err        error
		driverName string
	}
	chVols := make(chan vols, len(drivers))

	for _, vd := range drivers {
		go func(d volume.Driver) {
			vs, err := d.List()
			if err != nil {
				chVols <- vols{driverName: d.Name(), err: &OpErr{Err: err, Name: d.Name(), Op: "list"}}
				return
			}
			chVols <- vols{vols: vs}
		}(vd)
	}

	badDrivers := make(map[string]struct{})
	for i := 0; i < len(drivers); i++ {
		vs := <-chVols

		if vs.err != nil {
			warnings = append(warnings, vs.err.Error())
			badDrivers[vs.driverName] = struct{}{}
			logrus.Warn(vs.err)
		}
		ls = append(ls, vs.vols...)
	}

	if len(badDrivers) > 0 {
		for _, v := range s.names {
			if _, exists := badDrivers[v.DriverName()]; exists {
				ls = append(ls, v)
			}
		}
	}
	return ls, warnings, nil
}

// CreateWithRef creates a volume with the given name and driver and stores the ref
// This is just like Create() except we store the reference while holding the lock.
// This ensures there's no race between creating a volume and then storing a reference.
func (s *VolumeStore) CreateWithRef(name, driverName, ref string, opts map[string]string) (volume.Volume, error) {
	name = normaliseVolumeName(name)
	s.locks.Lock(name)
	defer s.locks.Unlock(name)

	v, err := s.create(name, driverName, opts)
	if err != nil {
		return nil, &OpErr{Err: err, Name: name, Op: "create"}
	}

	s.setNamed(v, ref)
	return v, nil
}

// Create creates a volume with the given name and driver.
func (s *VolumeStore) Create(name, driverName string, opts map[string]string) (volume.Volume, error) {
	name = normaliseVolumeName(name)
	s.locks.Lock(name)
	defer s.locks.Unlock(name)

	v, err := s.create(name, driverName, opts)
	if err != nil {
		return nil, &OpErr{Err: err, Name: name, Op: "create"}
	}
	s.setNamed(v, "")
	return v, nil
}

// create asks the given driver to create a volume with the name/opts.
// If a volume with the name is already known, it will ask the stored driver for the volume.
// If the passed in driver name does not match the driver name which is stored for the given volume name, an error is returned.
// It is expected that callers of this function hold any neccessary locks.
func (s *VolumeStore) create(name, driverName string, opts map[string]string) (volume.Volume, error) {
	// Validate the name in a platform-specific manner
	valid, err := volume.IsVolumeNameValid(name)
	if err != nil {
		return nil, err
	}
	if !valid {
		return nil, &OpErr{Err: errInvalidName, Name: name, Op: "create"}
	}

	if v, exists := s.getNamed(name); exists {
		if v.DriverName() != driverName && driverName != "" && driverName != volume.DefaultDriverName {
			return nil, errNameConflict
		}
		return v, nil
	}

	// Since there isn't a specified driver name, let's see if any of the existing drivers have this volume name
	if driverName == "" {
		v, _ := s.getVolume(name)
		if v != nil {
			return v, nil
		}
	}

	logrus.Debugf("Registering new volume reference: driver %q, name %q", driverName, name)
	vd, err := volumedrivers.GetDriver(driverName)
	if err != nil {
		return nil, &OpErr{Op: "create", Name: name, Err: err}
	}

	if v, _ := vd.Get(name); v != nil {
		return v, nil
	}
	return vd.Create(name, opts)
}

// GetWithRef gets a volume with the given name from the passed in driver and stores the ref
// This is just like Get(), but we store the reference while holding the lock.
// This makes sure there are no races between checking for the existance of a volume and adding a reference for it
func (s *VolumeStore) GetWithRef(name, driverName, ref string) (volume.Volume, error) {
	name = normaliseVolumeName(name)
	s.locks.Lock(name)
	defer s.locks.Unlock(name)

	vd, err := volumedrivers.GetDriver(driverName)
	if err != nil {
		return nil, &OpErr{Err: err, Name: name, Op: "get"}
	}

	v, err := vd.Get(name)
	if err != nil {
		return nil, &OpErr{Err: err, Name: name, Op: "get"}
	}

	s.setNamed(v, ref)
	return v, nil
}

// Get looks if a volume with the given name exists and returns it if so
func (s *VolumeStore) Get(name string) (volume.Volume, error) {
	name = normaliseVolumeName(name)
	s.locks.Lock(name)
	defer s.locks.Unlock(name)

	v, err := s.getVolume(name)
	if err != nil {
		return nil, &OpErr{Err: err, Name: name, Op: "get"}
	}
	s.setNamed(v, "")
	return v, nil
}

// get requests the volume, if the driver info is stored it just access that driver,
// if the driver is unknown it probes all drivers until it finds the first volume with that name.
// it is expected that callers of this function hold any neccessary locks
func (s *VolumeStore) getVolume(name string) (volume.Volume, error) {
	logrus.Debugf("Getting volume reference for name: %s", name)
	if v, exists := s.names[name]; exists {
		vd, err := volumedrivers.GetDriver(v.DriverName())
		if err != nil {
			return nil, err
		}
		return vd.Get(name)
	}

	logrus.Debugf("Probing all drivers for volume with name: %s", name)
	drivers, err := volumedrivers.GetAllDrivers()
	if err != nil {
		return nil, err
	}

	for _, d := range drivers {
		v, err := d.Get(name)
		if err != nil {
			continue
		}
		return v, nil
	}
	return nil, errNoSuchVolume
}

// Remove removes the requested volume. A volume is not removed if it has any refs
func (s *VolumeStore) Remove(v volume.Volume) error {
	name := normaliseVolumeName(v.Name())
	s.locks.Lock(name)
	defer s.locks.Unlock(name)

	if refs, exists := s.refs[name]; exists && len(refs) > 0 {
		return &OpErr{Err: errVolumeInUse, Name: v.Name(), Op: "remove", Refs: refs}
	}

	vd, err := volumedrivers.GetDriver(v.DriverName())
	if err != nil {
		return &OpErr{Err: err, Name: vd.Name(), Op: "remove"}
	}

	logrus.Debugf("Removing volume reference: driver %s, name %s", v.DriverName(), name)
	if err := vd.Remove(v); err != nil {
		return &OpErr{Err: err, Name: name, Op: "remove"}
	}

	s.purge(name)
	return nil
}

// Dereference removes the specified reference to the volume
func (s *VolumeStore) Dereference(v volume.Volume, ref string) {
	s.locks.Lock(v.Name())
	defer s.locks.Unlock(v.Name())

	s.globalLock.Lock()
	defer s.globalLock.Unlock()
	refs, exists := s.refs[v.Name()]
	if !exists {
		return
	}

	for i, r := range refs {
		if r == ref {
			s.refs[v.Name()] = append(s.refs[v.Name()][:i], s.refs[v.Name()][i+1:]...)
		}
	}
}

// Refs gets the current list of refs for the given volume
func (s *VolumeStore) Refs(v volume.Volume) []string {
	s.locks.Lock(v.Name())
	defer s.locks.Unlock(v.Name())

	s.globalLock.Lock()
	defer s.globalLock.Unlock()
	refs, exists := s.refs[v.Name()]
	if !exists {
		return nil
	}

	refsOut := make([]string, len(refs))
	copy(refsOut, refs)
	return refsOut
}

// FilterByDriver returns the available volumes filtered by driver name
func (s *VolumeStore) FilterByDriver(name string) ([]volume.Volume, error) {
	vd, err := volumedrivers.GetDriver(name)
	if err != nil {
		return nil, &OpErr{Err: err, Name: name, Op: "list"}
	}
	ls, err := vd.List()
	if err != nil {
		return nil, &OpErr{Err: err, Name: name, Op: "list"}
	}
	return ls, nil
}

// FilterByUsed returns the available volumes filtered by if they are in use or not.
// `used=true` returns only volumes that are being used, while `used=false` returns
// only volumes that are not being used.
func (s *VolumeStore) FilterByUsed(vols []volume.Volume, used bool) []volume.Volume {
	return s.filter(vols, func(v volume.Volume) bool {
		s.locks.Lock(v.Name())
		l := len(s.refs[v.Name()])
		s.locks.Unlock(v.Name())
		if (used && l > 0) || (!used && l == 0) {
			return true
		}
		return false
	})
}

// filterFunc defines a function to allow filter volumes in the store
type filterFunc func(vol volume.Volume) bool

// filter returns the available volumes filtered by a filterFunc function
func (s *VolumeStore) filter(vols []volume.Volume, f filterFunc) []volume.Volume {
	var ls []volume.Volume
	for _, v := range vols {
		if f(v) {
			ls = append(ls, v)
		}
	}
	return ls
}
