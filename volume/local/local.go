// Package local provides the default implementation for volumes. It
// is used to mount data volume containers and directories local to
// the host server.
package local

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"sync"

	"github.com/pkg/errors"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/pkg/idtools"
	"github.com/docker/docker/pkg/mount"
	"github.com/docker/docker/utils"
	"github.com/docker/docker/volume"
)

// VolumeDataPathName is the name of the directory where the volume data is stored.
// It uses a very distinctive name to avoid collisions migrating data between
// Docker versions.
const (
	VolumeDataPathName = "_data"
	volumesPathName    = "volumes"
)

var (
	// ErrNotFound is the typed error returned when the requested volume name can't be found
	ErrNotFound = fmt.Errorf("volume not found")
	// volumeNameRegex ensures the name assigned for the volume is valid.
	// This name is used to create the bind directory, so we need to avoid characters that
	// would make the path to escape the root directory.
	volumeNameRegex = utils.RestrictedVolumeNamePattern
)

type validationError struct {
	error
}

func (validationError) IsValidationError() bool {
	return true
}

type activeMount struct {
	count   uint64
	mounted bool
}

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

	mountInfos, err := mount.GetMounts()
	if err != nil {
		logrus.Debugf("error looking up mounts for local volume cleanup: %v", err)
	}

	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}

		name := filepath.Base(d.Name())
		v := &localVolume{
			driverName: r.Name(),
			name:       name,
			path:       r.DataPath(name),
		}
		r.volumes[name] = v
		optsFilePath := filepath.Join(rootDirectory, name, "opts.json")
		if b, err := ioutil.ReadFile(optsFilePath); err == nil {
			opts := optsConfig{}
			if err := json.Unmarshal(b, &opts); err != nil {
				return nil, errors.Wrapf(err, "error while unmarshaling volume options for volume: %s", name)
			}
			// Make sure this isn't an empty optsConfig.
			// This could be empty due to buggy behavior in older versions of Docker.
			if !reflect.DeepEqual(opts, optsConfig{}) {
				v.opts = &opts
			}

			// unmount anything that may still be mounted (for example, from an unclean shutdown)
			for _, info := range mountInfos {
				if info.Mountpoint == v.path {
					mount.Unmount(v.path)
					break
				}
			}
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
func (r *Root) List() ([]volume.Volume, error) {
	var ls []volume.Volume
	r.m.Lock()
	for _, v := range r.volumes {
		ls = append(ls, v)
	}
	r.m.Unlock()
	return ls, nil
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
func (r *Root) Create(name string, opts map[string]string) (volume.Volume, error) {
	if err := r.validateName(name); err != nil {
		return nil, err
	}

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
		return nil, errors.Wrapf(err, "error while creating volume path '%s'", path)
	}

	var err error
	defer func() {
		if err != nil {
			os.RemoveAll(filepath.Dir(path))
		}
	}()

	v = &localVolume{
		driverName: r.Name(),
		name:       name,
		path:       path,
	}

	if len(opts) != 0 {
		if err = setOpts(v, opts); err != nil {
			return nil, err
		}
		var b []byte
		b, err = json.Marshal(v.opts)
		if err != nil {
			return nil, err
		}
		if err = ioutil.WriteFile(filepath.Join(filepath.Dir(path), "opts.json"), b, 600); err != nil {
			return nil, errors.Wrap(err, "error while persisting volume options")
		}
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
		return fmt.Errorf("unknown volume type %T", v)
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
		return errors.Wrapf(err, "error removing volume path '%s'", path)
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

// Scope returns the local volume scope
func (r *Root) Scope() string {
	return volume.LocalScope
}

func (r *Root) validateName(name string) error {
	if len(name) == 1 {
		return validationError{fmt.Errorf("volume name is too short, names should be at least two alphanumeric characters")}
	}
	if !volumeNameRegex.MatchString(name) {
		return validationError{fmt.Errorf("%q includes invalid characters for a local volume name, only %q are allowed", name, utils.RestrictedNameChars)}
	}
	return nil
}

// localVolume implements the Volume interface from the volume package and
// represents the volumes created by Root.
type localVolume struct {
	m sync.Mutex
	// unique name of the volume
	name string
	// path is the path on the host where the data lives
	path string
	// driverName is the name of the driver that created the volume.
	driverName string
	// opts is the parsed list of options used to create the volume
	opts *optsConfig
	// active refcounts the active mounts
	active activeMount
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
func (v *localVolume) Mount(id string) (string, error) {
	v.m.Lock()
	defer v.m.Unlock()
	if v.opts != nil {
		if !v.active.mounted {
			if err := v.mount(); err != nil {
				return "", err
			}
			v.active.mounted = true
		}
		v.active.count++
	}
	return v.path, nil
}

// Umount is for satisfying the localVolume interface and does not do anything in this driver.
func (v *localVolume) Unmount(id string) error {
	v.m.Lock()
	defer v.m.Unlock()
	if v.opts != nil {
		v.active.count--
		if v.active.count == 0 {
			if err := mount.Unmount(v.path); err != nil {
				v.active.count++
				return errors.Wrapf(err, "error while unmounting volume path '%s'", v.path)
			}
			v.active.mounted = false
		}
	}
	return nil
}

func validateOpts(opts map[string]string) error {
	for opt := range opts {
		if !validOpts[opt] {
			return validationError{fmt.Errorf("invalid option key: %q", opt)}
		}
	}
	return nil
}

func (v *localVolume) Status() map[string]interface{} {
	return nil
}
