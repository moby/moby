// Package local provides the default implementation for volumes. It
// is used to mount data volume containers and directories local to
// the host server.
package local

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"

	"github.com/docker/docker/pkg/idtools"
	"github.com/docker/docker/volume"
)

// VolumeDataPathName is the name of the directory where the volume data is stored.
// It uses a very distintive name to avoid collisions migrating data between
// Docker versions.
const (
	VolumeDataPathName = "_data"
	volumesPathName    = "volumes"
)

// ErrNotFound is the typed error returned when the requested volume name can't be found
var ErrNotFound = errors.New("volume not found")

// New instantiates a new Root instance with the provided scope. Scope
// is the base path that the Root instance uses to store its
// volumes. The base path is created here if it does not exist.
func New(scope string, rootUID, rootGID int) (*Root, error) {
	rootDirectory := filepath.Join(scope, volumesPathName)

	if err := idtools.MkdirAllAs(rootDirectory, 0700, rootUID, rootGID); err != nil {
		return nil, err
	}

	r := &Root{
		scope:   scope,
		path:    rootDirectory,
		volumes: make(map[string]*localVolume),
		rootUID: rootUID,
		rootGID: rootGID,
	}

	dirs, err := ioutil.ReadDir(rootDirectory)
	if err != nil {
		return nil, err
	}

	for _, d := range dirs {
		name := filepath.Base(d.Name())
		r.volumes[name] = &localVolume{
			driverName: r.Name(),
			name:       name,
			path:       r.DataPath(name),
		}
	}

	return r, nil
}

// Root implements the Driver interface for the volume package and
// manages the creation/removal of volumes. It uses only standard vfs
// commands to create/remove dirs within its provided scope.
type Root struct {
	m       sync.Mutex
	scope   string
	path    string
	volumes map[string]*localVolume
	rootUID int
	rootGID int
}

// List lists all the volumes
func (r *Root) List() []volume.Volume {
	var ls []volume.Volume
	for _, v := range r.volumes {
		ls = append(ls, v)
	}
	return ls
}

// DataPath returns the constructed path of this volume.
func (r *Root) DataPath(volumeName string) string {
	return filepath.Join(r.path, volumeName, VolumeDataPathName)
}

// Name returns the name of Root, defined in the volume package in the DefaultDriverName constant.
func (r *Root) Name() string {
	return volume.DefaultDriverName
}

// Create creates a new volume.Volume with the provided name, creating
// the underlying directory tree required for this volume in the
// process.
func (r *Root) Create(name string, _ map[string]string) (volume.Volume, error) {
	r.m.Lock()
	defer r.m.Unlock()

	v, exists := r.volumes[name]
	if exists {
		return v, nil
	}

	path := r.DataPath(name)
	if err := idtools.MkdirAllAs(path, 0755, r.rootUID, r.rootGID); err != nil {
		if os.IsExist(err) {
			return nil, fmt.Errorf("volume already exists under %s", filepath.Dir(path))
		}
		return nil, err
	}
	v = &localVolume{
		driverName: r.Name(),
		name:       name,
		path:       path,
	}
	r.volumes[name] = v
	return v, nil
}

// Remove removes the specified volume and all underlying data. If the
// given volume does not belong to this driver and an error is
// returned. The volume is reference counted, if all references are
// not released then the volume is not removed.
func (r *Root) Remove(v volume.Volume) error {
	r.m.Lock()
	defer r.m.Unlock()

	lv, ok := v.(*localVolume)
	if !ok {
		return errors.New("unknown volume type")
	}

	realPath, err := filepath.EvalSymlinks(lv.path)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		realPath = filepath.Dir(lv.path)
	}

	if !r.scopedPath(realPath) {
		return fmt.Errorf("Unable to remove a directory of out the Docker root %s: %s", r.scope, realPath)
	}

	if err := removePath(realPath); err != nil {
		return err
	}

	delete(r.volumes, lv.name)
	return removePath(filepath.Dir(lv.path))
}

func removePath(path string) error {
	if err := os.RemoveAll(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return nil
}

// Get looks up the volume for the given name and returns it if found
func (r *Root) Get(name string) (volume.Volume, error) {
	r.m.Lock()
	v, exists := r.volumes[name]
	r.m.Unlock()
	if !exists {
		return nil, ErrNotFound
	}
	return v, nil
}

// localVolume implements the Volume interface from the volume package and
// represents the volumes created by Root.
type localVolume struct {
	m         sync.Mutex
	usedCount int
	// unique name of the volume
	name string
	// path is the path on the host where the data lives
	path string
	// driverName is the name of the driver that created the volume.
	driverName string
}

// Name returns the name of the given Volume.
func (v *localVolume) Name() string {
	return v.name
}

// DriverName returns the driver that created the given Volume.
func (v *localVolume) DriverName() string {
	return v.driverName
}

// Path returns the data location.
func (v *localVolume) Path() string {
	return v.path
}

// Mount implements the localVolume interface, returning the data location.
func (v *localVolume) Mount() (string, error) {
	return v.path, nil
}

// Umount is for satisfying the localVolume interface and does not do anything in this driver.
func (v *localVolume) Unmount() error {
	return nil
}
