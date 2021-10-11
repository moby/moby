//go:build linux
// +build linux

package devmapper // import "github.com/docker/docker/daemon/graphdriver/devmapper"

import (
	"fmt"
	"os"
	"path"
	"strconv"

	"github.com/docker/docker/daemon/graphdriver"
	"github.com/docker/docker/pkg/containerfs"
	"github.com/docker/docker/pkg/devicemapper"
	"github.com/docker/docker/pkg/idtools"
	units "github.com/docker/go-units"
	"github.com/moby/locker"
	"github.com/moby/sys/mount"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

func init() {
	graphdriver.Register("devicemapper", Init)
}

// Driver contains the device set mounted and the home directory
type Driver struct {
	*DeviceSet
	home    string
	uidMaps []idtools.IDMap
	gidMaps []idtools.IDMap
	ctr     *graphdriver.RefCounter
	locker  *locker.Locker
}

// Init creates a driver with the given home and the set of options.
func Init(home string, options []string, uidMaps, gidMaps []idtools.IDMap) (graphdriver.Driver, error) {
	deviceSet, err := NewDeviceSet(home, true, options, uidMaps, gidMaps)
	if err != nil {
		return nil, err
	}

	d := &Driver{
		DeviceSet: deviceSet,
		home:      home,
		uidMaps:   uidMaps,
		gidMaps:   gidMaps,
		ctr:       graphdriver.NewRefCounter(graphdriver.NewDefaultChecker()),
		locker:    locker.New(),
	}

	return graphdriver.NewNaiveDiffDriver(d, uidMaps, gidMaps), nil
}

func (d *Driver) String() string {
	return "devicemapper"
}

// Status returns the status about the driver in a printable format.
// Information returned contains Pool Name, Data File, Metadata file, disk usage by
// the data and metadata, etc.
func (d *Driver) Status() [][2]string {
	s := d.DeviceSet.Status()

	status := [][2]string{
		{"Pool Name", s.PoolName},
		{"Pool Blocksize", units.HumanSize(float64(s.SectorSize))},
		{"Base Device Size", units.HumanSize(float64(s.BaseDeviceSize))},
		{"Backing Filesystem", s.BaseDeviceFS},
		{"Udev Sync Supported", fmt.Sprintf("%v", s.UdevSyncSupported)},
	}

	if len(s.DataFile) > 0 {
		status = append(status, [2]string{"Data file", s.DataFile})
	}
	if len(s.MetadataFile) > 0 {
		status = append(status, [2]string{"Metadata file", s.MetadataFile})
	}
	if len(s.DataLoopback) > 0 {
		status = append(status, [2]string{"Data loop file", s.DataLoopback})
	}
	if len(s.MetadataLoopback) > 0 {
		status = append(status, [2]string{"Metadata loop file", s.MetadataLoopback})
	}

	status = append(status, [][2]string{
		{"Data Space Used", units.HumanSize(float64(s.Data.Used))},
		{"Data Space Total", units.HumanSize(float64(s.Data.Total))},
		{"Data Space Available", units.HumanSize(float64(s.Data.Available))},
		{"Metadata Space Used", units.HumanSize(float64(s.Metadata.Used))},
		{"Metadata Space Total", units.HumanSize(float64(s.Metadata.Total))},
		{"Metadata Space Available", units.HumanSize(float64(s.Metadata.Available))},
		{"Thin Pool Minimum Free Space", units.HumanSize(float64(s.MinFreeSpace))},
		{"Deferred Removal Enabled", fmt.Sprintf("%v", s.DeferredRemoveEnabled)},
		{"Deferred Deletion Enabled", fmt.Sprintf("%v", s.DeferredDeleteEnabled)},
		{"Deferred Deleted Device Count", fmt.Sprintf("%v", s.DeferredDeletedDeviceCount)},
	}...)

	if vStr, err := devicemapper.GetLibraryVersion(); err == nil {
		status = append(status, [2]string{"Library Version", vStr})
	}
	return status
}

// GetMetadata returns a map of information about the device.
func (d *Driver) GetMetadata(id string) (map[string]string, error) {
	m, err := d.DeviceSet.exportDeviceMetadata(id)

	if err != nil {
		return nil, err
	}

	metadata := make(map[string]string)
	metadata["DeviceId"] = strconv.Itoa(m.deviceID)
	metadata["DeviceSize"] = strconv.FormatUint(m.deviceSize, 10)
	metadata["DeviceName"] = m.deviceName
	return metadata, nil
}

// Cleanup unmounts a device.
func (d *Driver) Cleanup() error {
	err := d.DeviceSet.Shutdown(d.home)
	umountErr := mount.RecursiveUnmount(d.home)

	// in case we have two errors, prefer the one from Shutdown()
	if err != nil {
		return err
	}

	return umountErr
}

// CreateReadWrite creates a layer that is writable for use as a container
// file system.
func (d *Driver) CreateReadWrite(id, parent string, opts *graphdriver.CreateOpts) error {
	return d.Create(id, parent, opts)
}

// Create adds a device with a given id and the parent.
func (d *Driver) Create(id, parent string, opts *graphdriver.CreateOpts) error {
	var storageOpt map[string]string
	if opts != nil {
		storageOpt = opts.StorageOpt
	}
	return d.DeviceSet.AddDevice(id, parent, storageOpt)
}

// Remove removes a device with a given id, unmounts the filesystem, and removes the mount point.
func (d *Driver) Remove(id string) error {
	d.locker.Lock(id)
	defer d.locker.Unlock(id)
	if !d.DeviceSet.HasDevice(id) {
		// Consider removing a non-existing device a no-op
		// This is useful to be able to progress on container removal
		// if the underlying device has gone away due to earlier errors
		return nil
	}

	// This assumes the device has been properly Get/Put:ed and thus is unmounted
	if err := d.DeviceSet.DeleteDevice(id, false); err != nil {
		return fmt.Errorf("failed to remove device %s: %v", id, err)
	}

	// Most probably the mount point is already removed on Put()
	// (see DeviceSet.UnmountDevice()), but just in case it was not
	// let's try to remove it here as well, ignoring errors as
	// an older kernel can return EBUSY if e.g. the mount was leaked
	// to other mount namespaces. A failure to remove the container's
	// mount point is not important and should not be treated
	// as a failure to remove the container.
	mp := path.Join(d.home, "mnt", id)
	err := unix.Rmdir(mp)
	if err != nil && !os.IsNotExist(err) {
		logrus.WithField("storage-driver", "devicemapper").Warnf("unable to remove mount point %q: %s", mp, err)
	}

	return nil
}

// Get mounts a device with given id into the root filesystem
func (d *Driver) Get(id, mountLabel string) (containerfs.ContainerFS, error) {
	d.locker.Lock(id)
	defer d.locker.Unlock(id)
	mp := path.Join(d.home, "mnt", id)
	rootFs := path.Join(mp, "rootfs")
	if count := d.ctr.Increment(mp); count > 1 {
		return containerfs.NewLocalContainerFS(rootFs), nil
	}

	uid, gid, err := idtools.GetRootUIDGID(d.uidMaps, d.gidMaps)
	if err != nil {
		d.ctr.Decrement(mp)
		return nil, err
	}

	// Create the target directories if they don't exist
	if err := idtools.MkdirAllAndChown(path.Join(d.home, "mnt"), 0755, idtools.Identity{UID: uid, GID: gid}); err != nil {
		d.ctr.Decrement(mp)
		return nil, err
	}
	if err := idtools.MkdirAndChown(mp, 0755, idtools.Identity{UID: uid, GID: gid}); err != nil && !os.IsExist(err) {
		d.ctr.Decrement(mp)
		return nil, err
	}

	// Mount the device
	if err := d.DeviceSet.MountDevice(id, mp, mountLabel); err != nil {
		d.ctr.Decrement(mp)
		return nil, err
	}

	if err := idtools.MkdirAllAndChown(rootFs, 0755, idtools.Identity{UID: uid, GID: gid}); err != nil {
		d.ctr.Decrement(mp)
		d.DeviceSet.UnmountDevice(id, mp)
		return nil, err
	}

	idFile := path.Join(mp, "id")
	if _, err := os.Stat(idFile); err != nil && os.IsNotExist(err) {
		// Create an "id" file with the container/image id in it to help reconstruct this in case
		// of later problems
		if err := os.WriteFile(idFile, []byte(id), 0600); err != nil {
			d.ctr.Decrement(mp)
			d.DeviceSet.UnmountDevice(id, mp)
			return nil, err
		}
	}

	return containerfs.NewLocalContainerFS(rootFs), nil
}

// Put unmounts a device and removes it.
func (d *Driver) Put(id string) error {
	d.locker.Lock(id)
	defer d.locker.Unlock(id)
	mp := path.Join(d.home, "mnt", id)
	if count := d.ctr.Decrement(mp); count > 0 {
		return nil
	}

	err := d.DeviceSet.UnmountDevice(id, mp)
	if err != nil {
		logrus.WithField("storage-driver", "devicemapper").Errorf("Error unmounting device %s: %v", id, err)
	}

	return err
}

// Exists checks to see if the device exists.
func (d *Driver) Exists(id string) bool {
	return d.DeviceSet.HasDevice(id)
}
