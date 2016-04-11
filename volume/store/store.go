package store

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/boltdb/bolt"
	"github.com/docker/docker/pkg/locker"
	"github.com/docker/docker/volume"
	"github.com/docker/docker/volume/drivers"
)

const (
	volumeDataDir    = "volumes"
	volumeBucketName = "volumes"
)

type volumeMetadata struct {
	Name   string
	Labels map[string]string
}

type volumeWrapper struct {
	volume.Volume
	labels map[string]string
	scope  string
}

func (v volumeWrapper) Labels() map[string]string {
	return v.labels
}

func (v volumeWrapper) Scope() string {
	return v.scope
}

func (v volumeWrapper) CachedPath() string {
	if vv, ok := v.Volume.(interface {
		CachedPath() string
	}); ok {
		return vv.CachedPath()
	}
	return v.Volume.Path()
}

// New initializes a VolumeStore to keep
// reference counting of volumes in the system.
func New(rootPath string) (*VolumeStore, error) {
	vs := &VolumeStore{
		locks:  &locker.Locker{},
		names:  make(map[string]volume.Volume),
		refs:   make(map[string][]string),
		labels: make(map[string]map[string]string),
	}

	if rootPath != "" {
		// initialize metadata store
		volPath := filepath.Join(rootPath, volumeDataDir)
		if err := os.MkdirAll(volPath, 750); err != nil {
			return nil, err
		}

		dbPath := filepath.Join(volPath, "metadata.db")

		var err error
		vs.db, err = bolt.Open(dbPath, 0600, &bolt.Options{Timeout: 1 * time.Second})
		if err != nil {
			return nil, err
		}

		// initialize volumes bucket
		if err := vs.db.Update(func(tx *bolt.Tx) error {
			if _, err := tx.CreateBucketIfNotExists([]byte(volumeBucketName)); err != nil {
				return err
			}

			return nil
		}); err != nil {
			return nil, err
		}
	}

	return vs, nil
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
	delete(s.labels, name)
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
	// labels stores volume labels for each volume
	labels map[string]map[string]string
	db     *bolt.DB
}

// List proxies to all registered volume drivers to get the full list of volumes
// If a driver returns a volume that has name which conflicts with another volume from a different driver,
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
		// Note: it's not safe to populate the cache here because the volume may have been
		// deleted before we acquire a lock on its name
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
			for i, v := range vs {
				vs[i] = volumeWrapper{v, s.labels[v.Name()], d.Scope()}
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
func (s *VolumeStore) CreateWithRef(name, driverName, ref string, opts, labels map[string]string) (volume.Volume, error) {
	name = normaliseVolumeName(name)
	s.locks.Lock(name)
	defer s.locks.Unlock(name)

	v, err := s.create(name, driverName, opts, labels)
	if err != nil {
		return nil, &OpErr{Err: err, Name: name, Op: "create"}
	}

	s.setNamed(v, ref)
	return v, nil
}

// Create creates a volume with the given name and driver.
func (s *VolumeStore) Create(name, driverName string, opts, labels map[string]string) (volume.Volume, error) {
	name = normaliseVolumeName(name)
	s.locks.Lock(name)
	defer s.locks.Unlock(name)

	v, err := s.create(name, driverName, opts, labels)
	if err != nil {
		return nil, &OpErr{Err: err, Name: name, Op: "create"}
	}
	s.setNamed(v, "")
	return v, nil
}

// create asks the given driver to create a volume with the name/opts.
// If a volume with the name is already known, it will ask the stored driver for the volume.
// If the passed in driver name does not match the driver name which is stored for the given volume name, an error is returned.
// It is expected that callers of this function hold any necessary locks.
func (s *VolumeStore) create(name, driverName string, opts, labels map[string]string) (volume.Volume, error) {
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

	vd, err := volumedrivers.GetDriver(driverName)

	if err != nil {
		return nil, &OpErr{Op: "create", Name: name, Err: err}
	}

	logrus.Debugf("Registering new volume reference: driver %q, name %q", vd.Name(), name)

	if v, _ := vd.Get(name); v != nil {
		return v, nil
	}
	v, err := vd.Create(name, opts)
	if err != nil {
		return nil, err
	}
	s.globalLock.Lock()
	s.labels[name] = labels
	s.globalLock.Unlock()

	if s.db != nil {
		metadata := &volumeMetadata{
			Name:   name,
			Labels: labels,
		}

		volData, err := json.Marshal(metadata)
		if err != nil {
			return nil, err
		}

		if err := s.db.Update(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte(volumeBucketName))
			err := b.Put([]byte(name), volData)
			return err
		}); err != nil {
			return nil, err
		}
	}

	return volumeWrapper{v, labels, vd.Scope()}, nil
}

// GetWithRef gets a volume with the given name from the passed in driver and stores the ref
// This is just like Get(), but we store the reference while holding the lock.
// This makes sure there are no races between checking for the existence of a volume and adding a reference for it
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

	return volumeWrapper{v, s.labels[name], vd.Scope()}, nil
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

// getVolume requests the volume, if the driver info is stored it just accesses that driver,
// if the driver is unknown it probes all drivers until it finds the first volume with that name.
// it is expected that callers of this function hold any necessary locks
func (s *VolumeStore) getVolume(name string) (volume.Volume, error) {
	labels := map[string]string{}

	if s.db != nil {
		// get meta
		if err := s.db.Update(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte(volumeBucketName))
			data := b.Get([]byte(name))

			if string(data) == "" {
				return nil
			}

			var meta volumeMetadata
			buf := bytes.NewBuffer(data)

			if err := json.NewDecoder(buf).Decode(&meta); err != nil {
				return err
			}
			labels = meta.Labels

			return nil
		}); err != nil {
			return nil, err
		}
	}

	logrus.Debugf("Getting volume reference for name: %s", name)
	s.globalLock.Lock()
	v, exists := s.names[name]
	s.globalLock.Unlock()
	if exists {
		vd, err := volumedrivers.GetDriver(v.DriverName())
		if err != nil {
			return nil, err
		}
		vol, err := vd.Get(name)
		if err != nil {
			return nil, err
		}
		return volumeWrapper{vol, labels, vd.Scope()}, nil
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

		return volumeWrapper{v, labels, d.Scope()}, nil
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
	vol := unwrapVolume(v)
	if err := vd.Remove(vol); err != nil {
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
	var refs []string

	for _, r := range s.refs[v.Name()] {
		if r != ref {
			refs = append(refs, r)
		}
	}
	s.refs[v.Name()] = refs
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
	for i, v := range ls {
		ls[i] = volumeWrapper{v, s.labels[v.Name()], vd.Scope()}
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

func unwrapVolume(v volume.Volume) volume.Volume {
	if vol, ok := v.(volumeWrapper); ok {
		return vol.Volume
	}

	return v
}
