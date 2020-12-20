// Package local provides the default implementation for volumes. It
// is used to mount data volume containers and directories local to
// the host server.
package local // import "github.com/docker/docker/volume/local"

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"

	"github.com/docker/docker/daemon/names"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/pkg/idtools"
	"github.com/docker/docker/quota"
	"github.com/docker/docker/volume"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
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
	volumeNameRegex = names.RestrictedNamePattern
)

type activeMount struct {
	count   uint64
	mounted bool
}

// New instantiates a new Root instance with the provided scope. Scope
// is the base path that the Root instance uses to store its
// volumes. The base path is created here if it does not exist.
func New(scope string, rootIdentity idtools.Identity) (*Root, error) {
	rootDirectory := filepath.Join(scope, volumesPathName)

	if err := idtools.MkdirAllAndChown(rootDirectory, 0700, rootIdentity); err != nil {
		return nil, err
	}

	r := &Root{
		scope:        scope,
		path:         rootDirectory,
		volumes:      make(map[string]*localVolume),
		rootIdentity: rootIdentity,
	}

	dirs, err := ioutil.ReadDir(rootDirectory)
	if err != nil {
		return nil, err
	}

	if r.quotaCtl, err = quota.NewControl(rootDirectory); err != nil {
		logrus.Debugf("No quota support for local volumes in %s: %v", rootDirectory, err)
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
			quotaCtl:   r.quotaCtl,
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
			// unmount anything that may still be mounted (for example, from an
			// unclean shutdown). This is a no-op on windows
			unmount(v.path)
		}
	}

	return r, nil
}

// Root implements the Driver interface for the volume package and
// manages the creation/removal of volumes. It uses only standard vfs
// commands to create/remove dirs within its provided scope.
type Root struct {
	m            sync.Mutex
	scope        string
	path         string
	quotaCtl     *quota.Control
	volumes      map[string]*localVolume
	rootIdentity idtools.Identity
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
	if err := idtools.MkdirAllAndChown(path, 0755, r.rootIdentity); err != nil {
		return nil, errors.Wrapf(errdefs.System(err), "error while creating volume path '%s'", path)
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
		quotaCtl:   r.quotaCtl,
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
		if err = ioutil.WriteFile(filepath.Join(filepath.Dir(path), "opts.json"), b, 0600); err != nil {
			return nil, errdefs.System(errors.Wrap(err, "error while persisting volume options"))
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
		return errdefs.System(errors.Errorf("unknown volume type %T", v))
	}

	if lv.active.count > 0 {
		return errdefs.System(errors.Errorf("volume has active mounts"))
	}

	if err := lv.unmount(); err != nil {
		return err
	}

	realPath, err := filepath.EvalSymlinks(lv.path)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		realPath = filepath.Dir(lv.path)
	}

	if !r.scopedPath(realPath) {
		return errdefs.System(errors.Errorf("Unable to remove a directory outside of the local volume root %s: %s", r.scope, realPath))
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
		return errdefs.System(errors.Wrapf(err, "error removing volume path '%s'", path))
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
		return errdefs.InvalidParameter(errors.New("volume name is too short, names should be at least two alphanumeric characters"))
	}
	if !volumeNameRegex.MatchString(name) {
		return errdefs.InvalidParameter(errors.Errorf("%q includes invalid characters for a local volume name, only %q are allowed. If you intended to pass a host directory, use absolute path", name, names.RestrictedNameChars))
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
	// reference to Root instances quotaCtl
	quotaCtl *quota.Control
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

// CachedPath returns the data location
func (v *localVolume) CachedPath() string {
	return v.path
}

// Mount implements the localVolume interface, returning the data location.
// If there are any provided mount options, the resources will be mounted at this point
func (v *localVolume) Mount(id string) (string, error) {
	v.m.Lock()
	defer v.m.Unlock()
	if v.needsMount() {
		if !v.active.mounted {
			if err := v.mount(); err != nil {
				return "", errdefs.System(err)
			}
			v.active.mounted = true
		}
		v.active.count++
	}
	if err := v.postMount(); err != nil {
		return "", err
	}
	return v.path, nil
}

// Unmount dereferences the id, and if it is the last reference will unmount any resources
// that were previously mounted.
func (v *localVolume) Unmount(id string) error {
	v.m.Lock()
	defer v.m.Unlock()

	// Always decrement the count, even if the unmount fails
	// Essentially docker doesn't care if this fails, it will send an error, but
	// ultimately there's nothing that can be done. If we don't decrement the count
	// this volume can never be removed until a daemon restart occurs.
	if v.needsMount() {
		v.active.count--
	}

	if v.active.count > 0 {
		return nil
	}

	return v.unmount()
}

func (v *localVolume) Status() map[string]interface{} {
	return nil
}

// getAddress finds out address/hostname from options
func getAddress(opts string) string {
	optsList := strings.Split(opts, ",")
	for i := 0; i < len(optsList); i++ {
		if strings.HasPrefix(optsList[i], "addr=") {
			addr := strings.SplitN(optsList[i], "=", 2)[1]
			return addr
		}
	}
	return ""
}
