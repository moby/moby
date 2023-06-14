// Package local provides the default implementation for volumes. It
// is used to mount data volume containers and directories local to
// the host server.
package local // import "github.com/docker/docker/volume/local"

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"

	"github.com/containerd/containerd/log"
	"github.com/docker/docker/daemon/names"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/pkg/idtools"
	"github.com/docker/docker/quota"
	"github.com/docker/docker/volume"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	// volumeDataPathName is the name of the directory where the volume data is stored.
	// It uses a very distinctive name to avoid collisions migrating data between
	// Docker versions.
	volumeDataPathName = "_data"
	volumesPathName    = "volumes"
)

var (
	// ErrNotFound is the typed error returned when the requested volume name can't be found
	ErrNotFound = errors.New("volume not found")
	// volumeNameRegex ensures the name assigned for the volume is valid.
	// This name is used to create the bind directory, so we need to avoid characters that
	// would make the path to escape the root directory.
	volumeNameRegex = names.RestrictedNamePattern

	_ volume.LiveRestorer = (*localVolume)(nil)
)

type activeMount struct {
	count   uint64
	mounted bool
}

// New instantiates a new Root instance with the provided scope. Scope
// is the base path that the Root instance uses to store its
// volumes. The base path is created here if it does not exist.
func New(scope string, rootIdentity idtools.Identity) (*Root, error) {
	r := &Root{
		path:         filepath.Join(scope, volumesPathName),
		volumes:      make(map[string]*localVolume),
		rootIdentity: rootIdentity,
	}

	if err := idtools.MkdirAllAndChown(r.path, 0701, idtools.CurrentIdentity()); err != nil {
		return nil, err
	}

	dirs, err := os.ReadDir(r.path)
	if err != nil {
		return nil, err
	}

	if r.quotaCtl, err = quota.NewControl(r.path); err != nil {
		logrus.Debugf("No quota support for local volumes in %s: %v", r.path, err)
	}

	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}

		name := d.Name()
		v := &localVolume{
			driverName: r.Name(),
			name:       name,
			rootPath:   filepath.Join(r.path, name),
			path:       filepath.Join(r.path, name, volumeDataPathName),
			quotaCtl:   r.quotaCtl,
		}

		// unmount anything that may still be mounted (for example, from an
		// unclean shutdown). This is a no-op on windows
		unmount(v.path)

		if err := v.loadOpts(); err != nil {
			return nil, err
		}
		r.volumes[name] = v
	}

	return r, nil
}

// Root implements the Driver interface for the volume package and
// manages the creation/removal of volumes. It uses only standard vfs
// commands to create/remove dirs within its provided scope.
type Root struct {
	m            sync.Mutex
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
	if err := r.validateOpts(opts); err != nil {
		return nil, err
	}

	r.m.Lock()
	defer r.m.Unlock()

	v, exists := r.volumes[name]
	if exists {
		return v, nil
	}

	v = &localVolume{
		driverName: r.Name(),
		name:       name,
		rootPath:   filepath.Join(r.path, name),
		path:       filepath.Join(r.path, name, volumeDataPathName),
		quotaCtl:   r.quotaCtl,
	}

	// Root dir does not need to be accessed by the remapped root
	if err := idtools.MkdirAllAndChown(v.rootPath, 0701, idtools.CurrentIdentity()); err != nil {
		return nil, errors.Wrapf(errdefs.System(err), "error while creating volume root path '%s'", v.rootPath)
	}

	// Remapped root does need access to the data path
	if err := idtools.MkdirAllAndChown(v.path, 0755, r.rootIdentity); err != nil {
		return nil, errors.Wrapf(errdefs.System(err), "error while creating volume data path '%s'", v.path)
	}

	var err error
	defer func() {
		if err != nil {
			os.RemoveAll(v.rootPath)
		}
	}()

	if err = v.setOpts(opts); err != nil {
		return nil, err
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
		return errdefs.System(errors.New("volume has active mounts"))
	}

	if err := lv.unmount(); err != nil {
		return err
	}

	// TODO(thaJeztah) is there a reason we're evaluating the data-path here, and not the volume's rootPath?
	realPath, err := filepath.EvalSymlinks(lv.path)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		// if the volume's data-directory wasn't found, fall back to using the
		// volume's root path (see 8d27417bfeff316346d00c07a456b0e1b056e788)
		realPath = lv.rootPath
	}

	if realPath == r.path || !strings.HasPrefix(realPath, r.path) {
		return errdefs.System(errors.Errorf("unable to remove a directory outside of the local volume root %s: %s", r.path, realPath))
	}

	if err := removePath(realPath); err != nil {
		return err
	}

	delete(r.volumes, lv.name)
	return removePath(lv.rootPath)
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
	// rootPath is the volume's root path, where the volume's metadata is stored.
	rootPath string
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
	logger := log.G(context.TODO()).WithField("volume", v.name)
	if v.needsMount() {
		if !v.active.mounted {
			logger.Debug("Mounting volume")
			if err := v.mount(); err != nil {
				return "", errdefs.System(err)
			}
			v.active.mounted = true
		}
		v.active.count++
		logger.WithField("active mounts", v.active).Debug("Decremented active mount count")
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
	logger := log.G(context.TODO()).WithField("volume", v.name)

	// Always decrement the count, even if the unmount fails
	// Essentially docker doesn't care if this fails, it will send an error, but
	// ultimately there's nothing that can be done. If we don't decrement the count
	// this volume can never be removed until a daemon restart occurs.
	if v.needsMount() {
		v.active.count--
		logger.WithField("active mounts", v.active).Debug("Decremented active mount count")
	}

	if v.active.count > 0 {
		return nil
	}

	logger.Debug("Unmounting volume")
	return v.unmount()
}

func (v *localVolume) Status() map[string]interface{} {
	return nil
}

func (v *localVolume) loadOpts() error {
	b, err := os.ReadFile(filepath.Join(v.rootPath, "opts.json"))
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			logrus.WithError(err).Warnf("error while loading volume options for volume: %s", v.name)
		}
		return nil
	}
	opts := optsConfig{}
	if err := json.Unmarshal(b, &opts); err != nil {
		return errors.Wrapf(err, "error while unmarshaling volume options for volume: %s", v.name)
	}
	// Make sure this isn't an empty optsConfig.
	// This could be empty due to buggy behavior in older versions of Docker.
	if !reflect.DeepEqual(opts, optsConfig{}) {
		v.opts = &opts
	}
	return nil
}

func (v *localVolume) saveOpts() error {
	var b []byte
	b, err := json.Marshal(v.opts)
	if err != nil {
		return err
	}
	err = os.WriteFile(filepath.Join(v.rootPath, "opts.json"), b, 0600)
	if err != nil {
		return errdefs.System(errors.Wrap(err, "error while persisting volume options"))
	}
	return nil
}

// LiveRestoreVolume restores reference counts for mounts
// It is assumed that the volume is already mounted since this is only called for active, live-restored containers.
func (v *localVolume) LiveRestoreVolume(ctx context.Context, _ string) error {
	v.m.Lock()
	defer v.m.Unlock()

	if !v.needsMount() {
		return nil
	}
	v.active.count++
	v.active.mounted = true
	log.G(ctx).WithFields(logrus.Fields{
		"volume":        v.name,
		"active mounts": v.active,
	}).Debugf("Live restored volume")
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

// getPassword finds out a password from options
func getPassword(opts string) string {
	optsList := strings.Split(opts, ",")
	for i := 0; i < len(optsList); i++ {
		if strings.HasPrefix(optsList[i], "password=") {
			passwd := strings.SplitN(optsList[i], "=", 2)[1]
			return passwd
		}
	}
	return ""
}
