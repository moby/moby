// +build linux

package ploop

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/daemon/graphdriver"
	"github.com/docker/docker/pkg/parsers"
	"github.com/docker/docker/pkg/units"
	"github.com/kolyshkin/goploop-cli"
	"github.com/opencontainers/runc/libcontainer/label"
)

const (
	imagePrefix = "root.hdd"
)

func init() {
	logrus.Debugf("[ploop] init")
	graphdriver.Register("ploop", Init)
}

type mount struct {
	count  int32
	device string
}

// Driver holds some internal information about the ploop driver instance.
type Driver struct {
	home    string
	master  string
	size    uint64
	mode    ploop.ImageMode
	clog    uint
	mountsM sync.RWMutex
	mounts  map[string]*mount
}

// Init returns a new ploop graphdriver. Arguments are root directory
// to hold ploop images, and an array of options in the form key=value.
//
// Currently available options are:
//
// * ploop.size: maximum size of a filesystem inside a ploop image.
//   Value is in bytes, unless a suffix (K, M, G etc.) is given.
//   Default is 8GiB.
//
// * ploop.mode: one of "expanded", "preallocated", or "raw". D
//   Default is "expanded".
//
// * ploop.clog: cluster block size log, 6 to 15. Cluster block size is
//   512 * 2^clog bytes, for example for clog=10 it will be 512 * 2^10 = 512K.
//   Default is 9 (i.e. 256 kilobytes).
//
// * ploop.libdebug: enable ploop library debugging. A value is an integer,
//   the more the value is, the more debug is printed. Some possible values
//   are:
//     0 - only errors
//     1 - above plus warnings
//     3 - above plus debug info
//     4 - above plus timestamps for profiling
//     5 - above plus commands being executed (for goploop-cli only)
//   Default depends on the underlying ploop driver.
func Init(home string, opt []string) (graphdriver.Driver, error) {
	logrus.Debugf("[ploop] Init(home=%s)", home)

	// defaults
	m := ploop.Expanded
	var s int64 = 8589934592 // 8GiB
	var cl int64 = 9         // 9 is for 256K cluster block, 11 for 1M etc.

	for _, option := range opt {
		key, val, err := parsers.ParseKeyValueOpt(option)
		if err != nil {
			return nil, err
		}
		key = strings.ToLower(key)
		switch key {
		case "ploop.size":
			s, err = units.RAMInBytes(val)
			if err != nil {
				logrus.Errorf("[ploop] Bad value for ploop.size: %s", val)
				return nil, err
			}
		case "ploop.mode":
			m, err = ploop.ParseImageMode(val)
			if err != nil {
				logrus.Errorf("[ploop] Bad value for ploop.mode: %s", val)
				return nil, err
			}
		case "ploop.clog":
			cl, err = strconv.ParseInt(val, 10, 8)
			if err != nil || cl < 6 || cl > 16 {
				return nil, fmt.Errorf("[ploop] Bad value for ploop.clog: %s", val)
			}
		case "ploop.libdebug":
			libDebug, err := strconv.Atoi(val)
			if err != nil {
				return nil, fmt.Errorf("[ploop] Bad value for ploop.libdebug: %s", val)
			}
			ploop.SetVerboseLevel(libDebug)
		default:
			return nil, fmt.Errorf("[ploop] Unknown option %s", key)
		}
	}

	d := &Driver{
		home:   home,
		master: path.Join(home, "master"),
		mode:   m,
		size:   uint64(s >> 10), // convert to KB
		clog:   uint(cl),
		mounts: make(map[string]*mount),
	}

	// Remove old master image as image params might have changed,
	// ignoring the error if it's not there
	d.removeMaster(true)

	// create needed base dirs so we don't have to use MkdirAll() later
	dirs := []string{d.dir(""), d.mnt(""), d.master}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, err
		}
	}

	// Create a new master image
	file := path.Join(d.master, imagePrefix)
	cp := ploop.CreateParam{Size: d.size, Mode: d.mode, File: file, CLog: d.clog, Flags: ploop.NoLazy}

	if err := ploop.Create(&cp); err != nil {
		logrus.Warnf("[ploop] Create(): %s", err)
		logrus.Warnf("Can't create ploop image! Maybe some prerequisites are not met?")
		logrus.Warnf("Make sure you have ext4 filesystem in %s.", home)
		logrus.Warnf("Check that ploop, e2fsprogs and parted are installed.")
		return nil, graphdriver.ErrPrerequisites
	}

	return graphdriver.NaiveDiffDriver(d), nil
}

func (d *Driver) String() string {
	return "ploop"
}

// Status returns information about the ploop driver, shown by 'docker info'.
// Currently it returns the following information:
// * "Home directory" -- root directory for ploop images
// * "Ploop mode" -- ploop image mode (expanded, preallocated, or raw)
// * "Ploop image size" -- size of a filesystem within a ploop image
// * "Disk space used/total/available" -- same as du for ploop home dir
// * "Active device count" -- number of currently active ploop devices
// * "Active devices" -- list of currently active ploop devices
func (d *Driver) Status() [][2]string {
	var buf syscall.Statfs_t
	syscall.Statfs(d.home, &buf)
	bs := uint64(buf.Bsize)
	total := buf.Blocks * bs
	free := buf.Bfree * bs
	used := (buf.Blocks - buf.Bfree) * bs

	d.mountsM.RLock()
	devCount := len(d.mounts)
	var devices string
	for _, m := range d.mounts {
		if m.count > 0 {
			devices = devices + " " + m.device[5:]
		}
	}
	d.mountsM.RUnlock()

	status := [][2]string{
		{"Home directory", d.home},
		{"Ploop mode", d.mode.String()},
		{"Ploop image size", units.BytesSize(float64(d.size << 10))},
		{"Disk space used", units.BytesSize(float64(used))},
		{"Disk space total", units.BytesSize(float64(total))},
		{"Disk space available", units.BytesSize(float64(free))},
		{"Active device count", strconv.Itoa(devCount)},
		{"Active devices", devices},
		/*
			{"Total images", xxx},
			{"Mounted devices", xxx},
		*/
	}

	return status
}

// GetMetadata returns image/container metadata related to graph driver.
// Currently this driver returns:
//  * "Device" - ploop device name
//  * "DiskDescriptor" - path to DiskDescriptor.xml
//  * "MountPoint" - mount point
//  * "MountCount" - how many times Get() was called
func (d *Driver) GetMetadata(id string) (map[string]string, error) {
	logrus.Debugf("[ploop] GetMetadata(id=%s)", id)
	var device string
	var count int32

	d.mountsM.RLock()
	m, ok := d.mounts[id]
	if ok {
		device = m.device
		count = m.count
	}
	d.mountsM.RUnlock()
	if !ok {
		return nil, nil
	}
	if count < 1 {
		return nil, nil
	}

	data := make(map[string]string)
	dd := d.dd(id)

	data["Device"] = device
	data["DiskDescriptor"] = dd
	data["MountPoint"] = d.mnt(id)
	data["MountCount"] = strconv.Itoa(int(count))
	return data, nil
}

func (d *Driver) removeMaster(ignoreOpenError bool) {
	// Master image might be mounted
	p, err := ploop.Open(path.Join(d.master, ddxml))
	if err == nil {
		if m, _ := p.IsMounted(); m {
			p.Umount() // ignore errors
		}
		p.Close()
	} else if !ignoreOpenError {
		logrus.Warn(err)
	}
	// Remove master image
	if err := os.RemoveAll(d.master); err != nil {
		logrus.Warn(err) // might not be fatal but worth reporting
	}
}

// Cleanup unmounts all non-Put() mounts, and removes
// the master image created on Init().
func (d *Driver) Cleanup() error {
	logrus.Debugf("[ploop] Cleanup()")

	d.removeMaster(false)

	d.mountsM.Lock()
	for id, m := range d.mounts {
		logrus.Warnf("[ploop] Cleanup: unexpected ploop device %s, unmounting", m.device)
		if err := ploop.UmountByDevice(m.device); err != nil && !ploop.IsNotMounted(err) {
			logrus.Warnf("[ploop] Cleanup: %s", err)
		} else {
			delete(d.mounts, id)
		}
	}
	d.mountsM.Unlock()

	return nil
}

func (d *Driver) create(id string) error {
	return copyDir(d.master, d.dir(id))
}

// add some info to our parent
func markParent(id, parent, dir, pdir string) error {
	// 1 symlink us to parent, just for the sake of debugging
	rpdir := path.Join("..", parent)
	if err := os.Symlink(rpdir, path.Join(dir, "parent")); err != nil {
		logrus.Errorf("[ploop] markParent: %s", err)
		return err
	}

	return nil
}

// clone creates a copy of a parent ploop
func (d *Driver) clone(id, parent string) error {
	dd := d.dd(id)
	dir := d.dir(id)
	pdd := d.dd(parent)
	pdir := d.dir(parent)

	// FIXME: lock parent delta!!

	// see if we can reuse a snapshot
	snap, err := readVal(pdir, "uuid-for-children")
	if err != nil {
		logrus.Errorf("[ploop] clone(): readVal: %s", err)
		return err
	}
	if snap == "" {
		// create a snapshot
		logrus.Debugf("[ploop] clone(): creating snapshot for %s", id)
		pp, err := ploop.Open(pdd)
		if err != nil {
			return err
		}

		snap, err = pp.Snapshot()
		if err != nil {
			pp.Close()
			return err
		}

		pp.Close() // save dd.xml now!

		// save snapshot for future children
		writeVal(pdir, "uuid-for-children", snap)
	} else {
		logrus.Debugf("[ploop] clone(): reusing snapshot %s from %s", snap, id)
	}

	markParent(id, parent, dir, pdir)

	// copy dd.xml from parent dir
	if err := copyFile(pdd, dd); err != nil {
		return err
	}

	// hardlink images from parent dir
	files, err := ioutil.ReadDir(pdir)
	if err != nil {
		return err
	}
	for _, fi := range files {
		name := fi.Name()
		// TODO: maybe filter out non-files
		if !strings.HasPrefix(name, imagePrefix) {
			//			logrus.Debugf("[ploop] clone: skip %s", name)
			continue
		}
		src := path.Join(pdir, name)
		dst := path.Join(dir, name)
		//		logrus.Debugf("[ploop] clone: hardlink %s", name)
		if err = os.Link(src, dst); err != nil {
			return err
		}
	}

	// switch to snapshot, removing old top delta
	p, err := ploop.Open(dd)
	if err != nil {
		return err
	}
	defer p.Close()

	logrus.Debugf("[ploop] id=%s SwitchSnapshot(%s)", id, snap)
	if err = p.SwitchSnapshot(snap); err != nil {
		return err
	}

	return nil
}

// Create creates a new ploop image, by cloning either a parent image
// (if parent is set) or an initial (master) image.
func (d *Driver) Create(id, parent string) error {
	logrus.Debugf("[ploop] Create(id=%s, parent=%s)", id, parent)

	// Assuming Create is called for non-existing stuff only
	dir := d.dir(id)
	err := os.Mkdir(dir, 0700)
	if err != nil {
		return err
	}

	if parent == "" {
		err = d.create(id)
	} else {
		err = d.clone(id, parent)
	}

	if err != nil {
		os.RemoveAll(dir)
		return err
	}

	// Make sure the mount point exists
	mdir := d.mnt(id)
	err = os.Mkdir(mdir, 0755)
	if err != nil {
		return err
	}

	return nil
}

// Remove deletes a given ploop image.
func (d *Driver) Remove(id string) error {
	logrus.Debugf("[ploop] Remove(id=%s)", id)

	// Check if ploop was properly Get/Put:ed and is therefore unmounted
again:
	d.mountsM.Lock()
	_, ok := d.mounts[id]
	d.mountsM.Unlock()
	if ok {
		logrus.Warnf("[ploop] Remove(id=%s): unexpected on non-Put()", id)
		d.Put(id)
		goto again
	}

	dirs := []string{d.dir(id), d.mnt(id)}
	for _, d := range dirs {
		if err := os.RemoveAll(d); err != nil {
			return err
		}
	}

	return nil
}

// Get mounts a given image and returns its mountpoint.
func (d *Driver) Get(id, mountLabel string) (string, error) {
	mnt := d.mnt(id)

	d.mountsM.Lock()
	m, ok := d.mounts[id]
	if ok {
		if m.count > 0 {
			atomic.AddInt32(&m.count, 1)
			logrus.Debugf("[ploop] skip Get(id=%s), dev=%s, count=%d", id, m.device, m.count)
			d.mountsM.Unlock()
			return mnt, nil
		}
		logrus.Warnf("[ploop] Get() id=%s, dev=%s: unexpected count=%d", id, m.device, m.count)
	} else {
		m = new(mount)
		d.mounts[id] = m
	}
	d.mountsM.Unlock()

	logrus.Debugf("[ploop] Get(id=%s)", id)
	var mp ploop.MountParam

	dd := d.dd(id)
	dir := d.dir(id)
	mp.Target = mnt
	mp.Data = label.FormatMountLabel("", mountLabel)

	// Open ploop descriptor
	p, err := ploop.Open(dd)
	if err != nil {
		return "", err
	}
	defer p.Close()

	_, err = os.Stat(path.Join(dir, "uuid-for-children"))
	if err == nil {
		// This snapshot was already used to clone children from,
		// so we assume it won't be modified and mount it read-only.
		// If this assumption is not true (i.e. write access is needed)
		// we need to invalidate the snapshot by calling
		//	removeVal(dir, "uuid-for-children")
		// and then we can mount it read/write without fear.
		mp.Readonly = true
	} else if !os.IsNotExist(err) {
		logrus.Warnf("[ploop] Unexpected error: %s", err)
	}

	// Mount
	dev, err := p.Mount(&mp)
	if err != nil {
		return "", err
	}

	m.device = dev
	atomic.AddInt32(&m.count, 1)

	return mnt, nil
}

// Put unmounts a given image.
func (d *Driver) Put(id string) error {
	d.mountsM.Lock()
	m, ok := d.mounts[id]
	if ok {
		if m.count > 1 {
			atomic.AddInt32(&m.count, -1)
			logrus.Debugf("[ploop] skip Put(id=%s), dev=%s, count=%d", id, m.device, m.count)
			d.mountsM.Unlock()
			return nil
		} else if m.count == 1 {
			atomic.AddInt32(&m.count, -1)
		} else if m.count < 1 {
			logrus.Warnf("[ploop] Put(id=%s), dev=%s, unexpected count=%d", id, m.device, m.count)
		}
	}
	d.mountsM.Unlock()

	logrus.Debugf("[ploop] Put(id=%s)", id)

	dd := d.dd(id)
	p, err := ploop.Open(dd)
	if err != nil {
		return err
	}
	defer p.Close()

	err = p.Umount()
	/* Ignore "not mounted" error */
	if err != nil && ploop.IsNotMounted(err) {
		err = nil
	}

	if err != nil {
		logrus.Errorf("[ploop] Umount(%s): %s", id, err)
		return err
	}

	d.mountsM.Lock()
	delete(d.mounts, id)
	d.mountsM.Unlock()

	return nil
}

// Exists checks if a given image exists.
func (d *Driver) Exists(id string) bool {
	logrus.Debugf("[ploop] Exists(id=%s)", id)

	// Check if DiskDescriptor.xml is there
	dd := d.dd(id)
	_, err := os.Stat(dd)
	if err != nil {
		if !os.IsNotExist(err) {
			logrus.Errorf("[ploop] Unexpected error from stat(): %s", err)
		}
		return false
	}

	return true
}
