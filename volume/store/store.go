package store

import (
	"errors"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/pkg/locker"
	"github.com/docker/docker/volume"
	"github.com/docker/docker/volume/drivers"
)

var (
	// ErrVolumeInUse is a typed error returned when trying to remove a volume that is currently in use by a container
	ErrVolumeInUse = errors.New("volume is in use")
	// ErrNoSuchVolume is a typed error returned if the requested volume doesn't exist in the volume store
	ErrNoSuchVolume = errors.New("no such volume")
	// ErrInvalidName is a typed error returned when creating a volume with a name that is not valid on the platform
	ErrInvalidName = errors.New("volume name is not valid on this platform")
)

// New initializes a VolumeStore to keep
// reference counting of volumes in the system.
func New() *VolumeStore {
	return &VolumeStore{
		vols:  make(map[string]*volumeCounter),
		locks: &locker.Locker{},
	}
}

func (s *VolumeStore) get(name string) (*volumeCounter, bool) {
	s.globalLock.Lock()
	vc, exists := s.vols[name]
	s.globalLock.Unlock()
	return vc, exists
}

func (s *VolumeStore) set(name string, vc *volumeCounter) {
	s.globalLock.Lock()
	s.vols[name] = vc
	s.globalLock.Unlock()
}

func (s *VolumeStore) remove(name string) {
	s.globalLock.Lock()
	delete(s.vols, name)
	s.globalLock.Unlock()
}

// VolumeStore is a struct that stores the list of volumes available and keeps track of their usage counts
type VolumeStore struct {
	vols       map[string]*volumeCounter
	locks      *locker.Locker
	globalLock sync.Mutex
}

// volumeCounter keeps track of references to a volume
type volumeCounter struct {
	volume.Volume
	count uint
}

// AddAll adds a list of volumes to the store
func (s *VolumeStore) AddAll(vols []volume.Volume) {
	for _, v := range vols {
		s.vols[normaliseVolumeName(v.Name())] = &volumeCounter{v, 0}
	}
}

// Create tries to find an existing volume with the given name or create a new one from the passed in driver
func (s *VolumeStore) Create(name, driverName string, opts map[string]string) (volume.Volume, error) {
	name = normaliseVolumeName(name)
	s.locks.Lock(name)
	defer s.locks.Unlock(name)

	if vc, exists := s.get(name); exists {
		v := vc.Volume
		return v, nil
	}
	logrus.Debugf("Registering new volume reference: driver %s, name %s", driverName, name)

	vd, err := volumedrivers.GetDriver(driverName)
	if err != nil {
		return nil, err
	}

	// Validate the name in a platform-specific manner
	valid, err := volume.IsVolumeNameValid(name)
	if err != nil {
		return nil, err
	}
	if !valid {
		return nil, ErrInvalidName
	}

	v, err := vd.Create(name, opts)
	if err != nil {
		return nil, err
	}

	s.set(name, &volumeCounter{v, 0})
	return v, nil
}

// Get looks if a volume with the given name exists and returns it if so
func (s *VolumeStore) Get(name string) (volume.Volume, error) {
	name = normaliseVolumeName(name)
	s.locks.Lock(name)
	defer s.locks.Unlock(name)

	vc, exists := s.get(name)
	if !exists {
		return nil, ErrNoSuchVolume
	}
	return vc.Volume, nil
}

// Remove removes the requested volume. A volume is not removed if the usage count is > 0
func (s *VolumeStore) Remove(v volume.Volume) error {
	name := normaliseVolumeName(v.Name())
	s.locks.Lock(name)
	defer s.locks.Unlock(name)

	logrus.Debugf("Removing volume reference: driver %s, name %s", v.DriverName(), name)
	vc, exists := s.get(name)
	if !exists {
		return ErrNoSuchVolume
	}

	if vc.count > 0 {
		return ErrVolumeInUse
	}

	vd, err := volumedrivers.GetDriver(vc.DriverName())
	if err != nil {
		return err
	}
	if err := vd.Remove(vc.Volume); err != nil {
		return err
	}

	s.remove(name)
	return nil
}

// Increment increments the usage count of the passed in volume by 1
func (s *VolumeStore) Increment(v volume.Volume) {
	name := normaliseVolumeName(v.Name())
	s.locks.Lock(name)
	defer s.locks.Unlock(name)

	logrus.Debugf("Incrementing volume reference: driver %s, name %s", v.DriverName(), v.Name())
	vc, exists := s.get(name)
	if !exists {
		s.set(name, &volumeCounter{v, 1})
		return
	}
	vc.count++
}

// Decrement decrements the usage count of the passed in volume by 1
func (s *VolumeStore) Decrement(v volume.Volume) {
	name := normaliseVolumeName(v.Name())
	s.locks.Lock(name)
	defer s.locks.Unlock(name)
	logrus.Debugf("Decrementing volume reference: driver %s, name %s", v.DriverName(), v.Name())

	vc, exists := s.get(name)
	if !exists {
		return
	}
	if vc.count == 0 {
		return
	}
	vc.count--
}

// Count returns the usage count of the passed in volume
func (s *VolumeStore) Count(v volume.Volume) uint {
	name := normaliseVolumeName(v.Name())
	s.locks.Lock(name)
	defer s.locks.Unlock(name)

	vc, exists := s.get(name)
	if !exists {
		return 0
	}
	return vc.count
}

// List returns all the available volumes
func (s *VolumeStore) List() []volume.Volume {
	s.globalLock.Lock()
	defer s.globalLock.Unlock()
	var ls []volume.Volume
	for _, vc := range s.vols {
		ls = append(ls, vc.Volume)
	}
	return ls
}

// FilterByDriver returns the available volumes filtered by driver name
func (s *VolumeStore) FilterByDriver(name string) []volume.Volume {
	return s.filter(byDriver(name))
}

// filterFunc defines a function to allow filter volumes in the store
type filterFunc func(vol volume.Volume) bool

// byDriver generates a filterFunc to filter volumes by their driver name
func byDriver(name string) filterFunc {
	return func(vol volume.Volume) bool {
		return vol.DriverName() == name
	}
}

// filter returns the available volumes filtered by a filterFunc function
func (s *VolumeStore) filter(f filterFunc) []volume.Volume {
	s.globalLock.Lock()
	defer s.globalLock.Unlock()
	var ls []volume.Volume
	for _, vc := range s.vols {
		if f(vc.Volume) {
			ls = append(ls, vc.Volume)
		}
	}
	return ls
}
