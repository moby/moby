//+build windows

package lcow

// TODO @jhowardmsft. This is a placeholder driver with no implementation to
// support Linux Containers on Windows.

import (
	"io"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/daemon/graphdriver"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/idtools"
)

// filterDriver is an HCSShim driver type for the Windows Filter driver.
const filterDriver = 1

// init registers the LCOW driver to the register.
func init() {
	graphdriver.Register("lcow", InitLCOW)
}

// Driver represents a windows graph driver.
type Driver struct {
}

// InitLCOW returns a new Windows storage filter driver.
func InitLCOW(home string, options []string, uidMaps, gidMaps []idtools.IDMap) (graphdriver.Driver, error) {
	logrus.Debugf("lcow InitLCOW at %s", home)
	return &Driver{}, nil
}

// String returns the string representation of a driver. This should match
// the name the graph driver has been registered with.
func (d *Driver) String() string {
	return "lcow"
}

// Status returns the status of the driver.
func (d *Driver) Status() [][2]string {
	return [][2]string{
		{"LCOW", ""},
	}
}

// Exists returns true if the given id is registered with this driver.
func (d *Driver) Exists(id string) bool {
	logrus.Debugf("LCOWDriver Exists() id %s", id)
	return false
}

// CreateReadWrite creates a layer that is writable for use as a container
// file system.
func (d *Driver) CreateReadWrite(id, parent string, opts *graphdriver.CreateOpts) error {
	logrus.Debugf("LCOWDriver CreateReadWrite() id %s", id)
	return nil
}

// Create creates a new read-only layer with the given id.
func (d *Driver) Create(id, parent string, opts *graphdriver.CreateOpts) error {
	logrus.Debugf("LCOWDriver Create() id %s", id)
	return nil
}

// Remove unmounts and removes the dir information.
func (d *Driver) Remove(id string) error {
	logrus.Debugf("LCOWDriver Remove() id %s", id)
	return nil
}

// Get returns the rootfs path for the id. This will mount the dir at its given path.
func (d *Driver) Get(id, mountLabel string) (string, error) {
	logrus.Debugf("LCOWDriver Get() id %s", id)
	return "", nil
}

// Put adds a new layer to the driver.
func (d *Driver) Put(id string) error {
	logrus.Debugf("LCOWDriver Put() id %s", id)
	return nil
}

// Cleanup ensures the information the driver stores is properly removed.
// We use this opportunity to cleanup any -removing folders which may be
// still left if the daemon was killed while it was removing a layer.
func (d *Driver) Cleanup() error {
	logrus.Debugf("LCOWDriver Cleanup()")
	return nil
}

// Diff produces an archive of the changes between the specified
// layer and its parent layer which may be "".
// The layer should be mounted when calling this function
func (d *Driver) Diff(id, parent string) (_ io.ReadCloser, err error) {
	logrus.Debugf("LCOWDriver Diff() id %s parent %s", id, parent)
	return nil, nil
}

// Changes produces a list of changes between the specified layer
// and its parent layer. If parent is "", then all changes will be ADD changes.
// The layer should not be mounted when calling this function.
func (d *Driver) Changes(id, parent string) ([]archive.Change, error) {
	logrus.Debugf("LCOWDriver Changes() id %s parent %s", id, parent)
	return nil, nil
}

// ApplyDiff extracts the changeset from the given diff into the
// layer with the specified id and parent, returning the size of the
// new layer in bytes.
// The layer should not be mounted when calling this function
func (d *Driver) ApplyDiff(id, parent string, diff io.Reader) (int64, error) {
	logrus.Debugf("LCOWDriver ApplyDiff() id %s parent %s", id, parent)
	return 0, nil
}

// DiffSize calculates the changes between the specified layer
// and its parent and returns the size in bytes of the changes
// relative to its base filesystem directory.
func (d *Driver) DiffSize(id, parent string) (size int64, err error) {
	logrus.Debugf("LCOWDriver DiffSize() id %s parent %s", id, parent)
	return 0, nil
}

// DiffGetter returns a FileGetCloser that can read files from the directory that
// contains files for the layer differences. Used for direct access for tar-split.
func (d *Driver) DiffGetter(id string) (graphdriver.FileGetCloser, error) {
	return nil, nil
}

// GetMetadata returns custom driver information.
func (d *Driver) GetMetadata(id string) (map[string]string, error) {
	logrus.Debugf("LCOWDriver GetMetadata() id %s", id)
	return nil, nil
}
