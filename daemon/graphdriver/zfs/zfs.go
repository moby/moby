// Package zfs implements a graphdriver for ZFS-on-Linux.
package zfs

import (
	"fmt"
	"os"
	"path"
	"strings"
	"sync"
	"syscall"

	"github.com/docker/docker/daemon/graphdriver"
	"github.com/docker/docker/pkg/mount"
)

func init() {
	_ = graphdriver.Register("zfs", Init)
}

// Driver represents an initialized ZFS graphdriver.
type Driver struct {
	home          string
	poolName      string
	rootDataset   string
	openDatasetsM sync.Mutex
	openDatasets  map[string]struct{}
}

func (d *Driver) checkout(id string) {
	d.openDatasetsM.Lock()
	defer d.openDatasetsM.Unlock()
	d.openDatasets[id] = struct{}{}
}

func (d *Driver) checkin(id string) {
	d.openDatasetsM.Lock()
	defer d.openDatasetsM.Unlock()
	delete(d.openDatasets, id)
}

// Init initializes the graphdriver on a given mountpoint. It performs few
// sanity checks to make sure the specified path is actually a ZFS pool before
// initializing and returning the data structure.
func Init(home string, options []string) (graphdriver.Driver, error) {
	rootdir := path.Dir(home)

	var buf syscall.Statfs_t
	if err := syscall.Statfs(rootdir, &buf); err != nil {
		return nil, err
	}

	if graphdriver.FsMagic(buf.Type) != graphdriver.FsMagicZfs {
		return nil, graphdriver.ErrPrerequisites
	}

	if err := os.MkdirAll(home, 0700); err != nil {
		return nil, err
	}

	poolName, mountPoint, err := zfsGetPool(home)
	if err != nil {
		return nil, err
	}

	rootDataset := poolName + strings.TrimPrefix(home, mountPoint)
	if err := createParentDatasets(rootDataset); err != nil {
		return nil, err
	}

	if err := graphdriver.MakePrivate(home); err != nil {
		return nil, err
	}

	driver := &Driver{
		home:         home,
		poolName:     poolName,
		rootDataset:  rootDataset,
		openDatasets: make(map[string]struct{}),
	}

	// cd into the mountpoint. As long as the daemon doesn't cd anywhere
	// else later, this prevents the pool from being unmounted.
	if err := os.Chdir(home); err != nil {
		return nil, fmt.Errorf("zfs-Init: Could not change to the mount point '%s'", home)
	}

	return graphdriver.NaiveDiffDriver(driver), nil
}

// If we're given, e.g. --storage-path=/zpool/docker, then the home is
// /zpool/docker/zfs. If the ZFS mountpoint is actually /zpool, then we have to
// ensure that /zpool/docker/zfs and /zpool/docker are both mounted ZFS
// datasets, else deeper-nested dataset creation will fail.
func createParentDatasets(target string) error {
	parts := strings.Split(target, "/")
	for {
		parts = parts[0 : len(parts)-1]
		if len(parts) == 1 {
			// We've reached the pool, which certainly exists.
			return nil
		}
		ds := strings.Join(parts, "/")
		if zfsDatasetExists(ds) {
			continue
		}
		if err := zfsMountDataset(ds, ""); err != nil {
			if err := zfsCreateAutomountingDataset(ds); err != nil {
				return err
			}
		}
	}
}

// String returns a string representation of this driver.
func (d *Driver) String() string {
	return "zfs"
}

// Status returns a set of key-value pairs which give low level diagnostic
// status about this driver. Root Dir and Pool Name are the path and name of the
// pool, respectively, and "Open Datasets" is the number of open references per
// dataset tracked.
func (d *Driver) Status() [][2]string {
	d.openDatasetsM.Lock()
	defer d.openDatasetsM.Unlock()
	return [][2]string{
		{"Root Dir", d.home},
		{"Pool Name", d.poolName},
		{"Open Datasets", fmt.Sprintf("%v", d.openDatasets)},
	}
}

// Cleanup performs necessary tasks to release resources held by the driver:
// In this case, that means unmounting all mounted datasets.
func (d *Driver) Cleanup() error {
	d.openDatasetsM.Lock()
	defer d.openDatasetsM.Unlock()
	for id := range d.openDatasets {
		// we have to ignore this elsewhere, so we might as well here.
		_ = zfsUnmountDataset(d.getDataset(id))
	}
	return mount.Unmount(d.home)
}

func (d *Driver) getDataset(id string) string {
	return path.Join(d.rootDataset, id)
}

func (d *Driver) getPath(id string) string {
	return path.Join(d.home, id)
}

// Create creates a new, empty, filesystem layer with the specified id and
// parent. Parent may be "".
func (d *Driver) Create(id string, parent string) error {
	if parent == "" {
		return zfsCreateDataset(d.getDataset(id))
	}
	parentDir, err := d.Get(parent, "")
	if err != nil {
		return err
	}
	_ = parentDir
	return zfsCloneDataset(d.getDataset(parent), d.getDataset(id))
}

// Remove attempts to remove the dataset with this ID.
func (d *Driver) Remove(id string) error {
	return zfsDestroyDataset(d.getDataset(id))
}

// Get returns the mountpoint for the layered filesystem referred to by this
// id, optionally with the given SELinux label. If the given dataset is already
// mounted with a different label, the label will not be changed.
func (d *Driver) Get(id, mountLabel string) (string, error) {
	err := zfsMountDataset(d.getDataset(id), mountLabel)
	if err != nil {
		return "", err
	}
	return d.getPath(id), nil
}

// Put unmounts the underlying dataset when it has been called as many times as
// Get (i.e.: reference counting is used).
func (d *Driver) Put(id string) {
	d.checkin(id)
	_ = zfsUnmountDataset(d.getDataset(id))
}

// Exists returns whether a dataset with this ID exists.
func (d *Driver) Exists(id string) bool {
	return zfsDatasetExists(id)
}
