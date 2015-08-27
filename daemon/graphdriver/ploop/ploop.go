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
	"github.com/kolyshkin/goploop"
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

type Driver struct {
	home    string
	master  string
	size    uint64
	mode    ploop.ImageMode
	clog    uint
	mountsM sync.RWMutex
	mounts  map[string]*mount
}

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

	// Create a new master image, this also checks all the prereqs
	file := path.Join(d.master, imagePrefix)
	cp := ploop.CreateParam{Size: d.size, Mode: d.mode, File: file, CLog: d.clog, Flags: ploop.NoLazy}

	if err := ploop.Create(&cp); err != nil {
		logrus.Debugf("[ploop] Create(): %s", err)
		logrus.Debugf("Can't create ploop image! Maybe some prerequisites are not met?")
		logrus.Debugf("Make sure you have ext4 filesystem in %s.", home)
		logrus.Debugf("Check that e2fsprogs and parted are installed.")
		return nil, graphdriver.ErrPrerequisites
	}

	return graphdriver.NaiveDiffDriver(d), nil
}

func (d *Driver) String() string {
	return "ploop"
}

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

// GetMetadata returns image/container metadata related to graph driver
func (d *Driver) GetMetadata(id string) (map[string]string, error) {
	logrus.Debugf("[ploop] GetMetadata(id=%s)", id)
	var device string
	var count int32

	d.mountsM.Lock()
	m, ok := d.mounts[id]
	if ok {
		device = m.device
		count = m.count
	}
	d.mountsM.Unlock()
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
	data["Mounted at"] = d.mnt(id)
	data["Mount count"] = strconv.Itoa(int(count))

	f, err := ploop.FSInfo(dd)
	if err != nil {
		logrus.Warnf("[ploop] GetMetadata(): FSInfo(): %s", err)
		// ignore the error, return what we got so far
		return data, nil
	}
	bTotal := f.Blocks * f.BlockSize
	bFree := f.BlocksFree * f.BlockSize
	bUsed := bTotal - bFree

	iTotal := f.Inodes
	iFree := f.InodesFree
	iUsed := iTotal - iFree

	data["Disk space total"] = units.BytesSize(float64(bTotal))
	data["Disk space used"] = units.BytesSize(float64(bUsed))
	data["Disk space free"] = units.BytesSize(float64(bFree))

	data["Inodes total"] = units.HumanSize(float64(iTotal))
	data["Inodes used"] = units.HumanSize(float64(iUsed))
	data["Inodes free"] = units.HumanSize(float64(iFree))

	p, err := ploop.Open(dd)
	if err != nil {
		logrus.Warnf("[ploop] GetMetadata(): Open(): %s", err)
		// ignore the error, return what we got so far
		return data, nil
	}
	defer p.Close()

	i, err := p.ImageInfo()
	if err != nil {
		logrus.Warnf("[ploop] GetMetadata(): ImageInfo(): %s", err)
		return data, nil
	}
	data["Image size"] = units.BytesSize(float64(i.Blocks << 9))

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

func (d *Driver) Cleanup() error {
	logrus.Debugf("[ploop] Cleanup()")

	d.removeMaster(false)

	d.mountsM.Lock()
	for id, m := range d.mounts {
		logrus.Warnf("[ploop] Cleanup: unexpected ploop device %s, unmounting", m.device)
		if err := ploop.UmountByDevice(m.device); err != nil {
			logrus.Warnf("[ploop] Cleanup: %s", err)
		}
		delete(d.mounts, id)
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
		m = &mount{0, ""}
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

func (d *Driver) Put(id string) error {
	d.mountsM.Lock()
	m, ok := d.mounts[id]
	if ok {
		if m.count > 1 {
			atomic.AddInt32(&m.count, -1)
			logrus.Debugf("[ploop] skip Put(id=%s), dev=%s, count=%d", id, m.device, m.count)
			d.mountsM.Unlock()
			return nil
		} else if m.count < 1 {
			logrus.Warnf("[ploop] Put(id=%s): unexpected mount count %d", m.count)
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
	if ploop.IsNotMounted(err) {
		err = nil
	}

	d.mountsM.Lock()
	delete(d.mounts, id)
	d.mountsM.Unlock()

	return err
}

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
