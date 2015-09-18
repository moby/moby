package store

import (
	"errors"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/volume"
	"github.com/docker/docker/volume/drivers"
)

var (
	// ErrVolumeInUse is a typed error returned when trying to remove a volume that is currently in use by a container
	ErrVolumeInUse = errors.New("volume is in use")
	// ErrNoSuchVolume is a typed error returned if the requested volume doesn't exist in the volume store
	ErrNoSuchVolume = errors.New("no such volume")
)

// New initializes a VolumeStore to keep
// reference counting of volumes in the system.
func New() *VolumeStore {
	return &VolumeStore{
		vols: make(map[string]*volumeCounter),
	}
}

// VolumeStore is a struct that stores the list of volumes available and keeps track of their usage counts
type VolumeStore struct {
	vols map[string]*volumeCounter
	mu   sync.Mutex
}

// volumeCounter keeps track of references to a volume
type volumeCounter struct {
	volume.Volume
	count uint
}

// AddAll adds a list of volumes to the store
func (s *VolumeStore) AddAll(vols []volume.Volume) {
	for _, v := range vols {
		s.vols[v.Name()] = &volumeCounter{v, 0}
	}
}

// Create tries to find an existing volume with the given name or create a new one from the passed in driver
func (s *VolumeStore) Create(name, driverName string, opts map[string]string) (volume.Volume, error) {
	s.mu.Lock()
	if vc, exists := s.vols[name]; exists {
		v := vc.Volume
		s.mu.Unlock()
		return v, nil
	}
	s.mu.Unlock()
	logrus.Debugf("Registering new volume reference: driver %s, name %s", driverName, name)

	vd, err := volumedrivers.GetDriver(driverName)
	if err != nil {
		return nil, err
	}

	v, err := vd.Create(name, opts)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	s.vols[v.Name()] = &volumeCounter{v, 0}
	s.mu.Unlock()

	return v, nil
}

// Get looks if a volume with the given name exists and returns it if so
func (s *VolumeStore) Get(name string) (volume.Volume, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	vc, exists := s.vols[name]
	if !exists {
		return nil, ErrNoSuchVolume
	}
	return vc.Volume, nil
}

// Remove removes the requested volume. A volume is not removed if the usage count is > 0
func (s *VolumeStore) Remove(v volume.Volume) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	name := v.Name()
	logrus.Debugf("Removing volume reference: driver %s, name %s", v.DriverName(), name)
	vc, exists := s.vols[name]
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
	delete(s.vols, name)
	return nil
}

// Increment increments the usage count of the passed in volume by 1
func (s *VolumeStore) Increment(v volume.Volume) {
	s.mu.Lock()
	defer s.mu.Unlock()
	logrus.Debugf("Incrementing volume reference: driver %s, name %s", v.DriverName(), v.Name())

	vc, exists := s.vols[v.Name()]
	if !exists {
		s.vols[v.Name()] = &volumeCounter{v, 1}
		return
	}
	vc.count++
}

// Decrement decrements the usage count of the passed in volume by 1
func (s *VolumeStore) Decrement(v volume.Volume) {
	s.mu.Lock()
	defer s.mu.Unlock()
	logrus.Debugf("Decrementing volume reference: driver %s, name %s", v.DriverName(), v.Name())

	vc, exists := s.vols[v.Name()]
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
	s.mu.Lock()
	defer s.mu.Unlock()
	vc, exists := s.vols[v.Name()]
	if !exists {
		return 0
	}
	return vc.count
}

// List returns all the available volumes
func (s *VolumeStore) List() []volume.Volume {
	s.mu.Lock()
	defer s.mu.Unlock()
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
	s.mu.Lock()
	defer s.mu.Unlock()
	var ls []volume.Volume
	for _, vc := range s.vols {
		if f(vc.Volume) {
			ls = append(ls, vc.Volume)
		}
	}
	return ls
}
