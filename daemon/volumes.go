package daemon

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/chrootarchive"
	"github.com/docker/docker/pkg/system"
	"github.com/docker/docker/volume"
	"github.com/docker/docker/volume/drivers"
)

var (
	// ErrVolumeReadonly is used to signal an error when trying to copy data into
	// a volume mount that is not writable.
	ErrVolumeReadonly = errors.New("mounted volume is marked read-only")
	// ErrVolumeInUse is a typed error returned when trying to remove a volume that is currently in use by a container
	ErrVolumeInUse = errors.New("volume is in use")
	// ErrNoSuchVolume is a typed error returned if the requested volume doesn't exist in the volume store
	ErrNoSuchVolume = errors.New("no such volume")
)

// mountPoint is the intersection point between a volume and a container. It
// specifies which volume is to be used and where inside a container it should
// be mounted.
type mountPoint struct {
	Name        string
	Destination string
	Driver      string
	RW          bool
	Volume      volume.Volume `json:"-"`
	Source      string
	Mode        string `json:"Relabel"` // Originally field was `Relabel`"
}

// Setup sets up a mount point by either mounting the volume if it is
// configured, or creating the source directory if supplied.
func (m *mountPoint) Setup() (string, error) {
	if m.Volume != nil {
		return m.Volume.Mount()
	}

	if len(m.Source) > 0 {
		if _, err := os.Stat(m.Source); err != nil {
			if !os.IsNotExist(err) {
				return "", err
			}
			if err := system.MkdirAll(m.Source, 0755); err != nil {
				return "", err
			}
		}
		return m.Source, nil
	}

	return "", fmt.Errorf("Unable to setup mount point, neither source nor volume defined")
}

// hasResource checks whether the given absolute path for a container is in
// this mount point. If the relative path starts with `../` then the resource
// is outside of this mount point, but we can't simply check for this prefix
// because it misses `..` which is also outside of the mount, so check both.
func (m *mountPoint) hasResource(absolutePath string) bool {
	relPath, err := filepath.Rel(m.Destination, absolutePath)

	return err == nil && relPath != ".." && !strings.HasPrefix(relPath, fmt.Sprintf("..%c", filepath.Separator))
}

// Path returns the path of a volume in a mount point.
func (m *mountPoint) Path() string {
	if m.Volume != nil {
		return m.Volume.Path()
	}

	return m.Source
}

// copyExistingContents copies from the source to the destination and
// ensures the ownership is appropriately set.
func copyExistingContents(source, destination string) error {
	volList, err := ioutil.ReadDir(source)
	if err != nil {
		return err
	}
	if len(volList) > 0 {
		srcList, err := ioutil.ReadDir(destination)
		if err != nil {
			return err
		}
		if len(srcList) == 0 {
			// If the source volume is empty copy files from the root into the volume
			if err := chrootarchive.CopyWithTar(source, destination); err != nil {
				return err
			}
		}
	}
	return copyOwnership(source, destination)
}

func newVolumeStore(vols []volume.Volume) *volumeStore {
	store := &volumeStore{
		vols: make(map[string]*volumeCounter),
	}
	for _, v := range vols {
		store.vols[v.Name()] = &volumeCounter{v, 0}
	}
	return store
}

// volumeStore is a struct that stores the list of volumes available and keeps track of their usage counts
type volumeStore struct {
	vols map[string]*volumeCounter
	mu   sync.Mutex
}

type volumeCounter struct {
	volume.Volume
	count int
}

func getVolumeDriver(name string) (volume.Driver, error) {
	if name == "" {
		name = volume.DefaultDriverName
	}
	return volumedrivers.Lookup(name)
}

// Create tries to find an existing volume with the given name or create a new one from the passed in driver
func (s *volumeStore) Create(name, driverName string, opts map[string]string) (volume.Volume, error) {
	s.mu.Lock()
	if vc, exists := s.vols[name]; exists {
		v := vc.Volume
		s.mu.Unlock()
		return v, nil
	}
	s.mu.Unlock()

	vd, err := getVolumeDriver(driverName)
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
func (s *volumeStore) Get(name string) (volume.Volume, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	vc, exists := s.vols[name]
	if !exists {
		return nil, ErrNoSuchVolume
	}
	return vc.Volume, nil
}

// Remove removes the requested volume. A volume is not removed if the usage count is > 0
func (s *volumeStore) Remove(v volume.Volume) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	name := v.Name()
	vc, exists := s.vols[name]
	if !exists {
		return ErrNoSuchVolume
	}

	if vc.count != 0 {
		return ErrVolumeInUse
	}

	vd, err := getVolumeDriver(vc.DriverName())
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
func (s *volumeStore) Increment(v volume.Volume) {
	s.mu.Lock()
	defer s.mu.Unlock()

	vc, exists := s.vols[v.Name()]
	if !exists {
		s.vols[v.Name()] = &volumeCounter{v, 1}
		return
	}
	vc.count++
	return
}

// Decrement decrements the usage count of the passed in volume by 1
func (s *volumeStore) Decrement(v volume.Volume) {
	s.mu.Lock()
	defer s.mu.Unlock()

	vc, exists := s.vols[v.Name()]
	if !exists {
		return
	}
	vc.count--
	return
}

// Count returns the usage count of the passed in volume
func (s *volumeStore) Count(v volume.Volume) int {
	vc, exists := s.vols[v.Name()]
	if !exists {
		return 0
	}
	return vc.count
}

// List returns all the available volumes
func (s *volumeStore) List() []volume.Volume {
	var ls []volume.Volume
	for _, vc := range s.vols {
		ls = append(ls, vc.Volume)
	}
	return ls
}

// volumeToAPIType converts a volume.Volume to the type used by the remote API
func volumeToAPIType(v volume.Volume) *types.Volume {
	return &types.Volume{
		Name:       v.Name(),
		Driver:     v.DriverName(),
		Mountpoint: v.Path(),
	}
}
