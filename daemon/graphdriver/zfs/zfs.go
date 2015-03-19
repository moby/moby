package zfs

/*
#include <locale.h>
#include <stdlib.h>
#include <dirent.h>
#include <mntent.h>
*/
import "C"

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/daemon/graphdriver"
	"github.com/docker/docker/pkg/parsers"
	zfs "github.com/mistifyio/go-zfs"
)

type ZfsOptions struct {
	fsName    string
	mountPath string
}

func init() {
	graphdriver.Register("zfs", Init)
}

type Logger struct{}

func (*Logger) Log(cmd []string) {
	log.Debugf("[zfs] %s", strings.Join(cmd, " "))
}

func Init(base string, opt []string) (graphdriver.Driver, error) {
	var err error
	options, err := parseOptions(opt)
	options.mountPath = base
	if err != nil {
		return nil, err
	}

	rootdir := path.Dir(base)

	if options.fsName == "" {
		err = checkRootdirFs(rootdir)
		if err != nil {
			return nil, err
		}
	}

	if _, err := exec.LookPath("zfs"); err != nil {
		return nil, fmt.Errorf("zfs command is not available")
	}

	file, err := os.OpenFile("/dev/zfs", os.O_RDWR, 600)
	defer file.Close()
	if err != nil {
		return nil, fmt.Errorf("Failed to initialize: %v", err)
	}

	if options.fsName == "" {
		options.fsName, err = lookupZfsDataset(rootdir)
		if err != nil {
			return nil, err
		}
	}

	logger := Logger{}
	zfs.SetLogger(&logger)

	dataset, err := zfs.GetDataset(options.fsName)
	if err != nil {
		return nil, fmt.Errorf("Cannot open %s", options.fsName)
	}

	d := &Driver{
		dataset: dataset,
		options: options,
	}
	return graphdriver.NaiveDiffDriver(d), nil
}

func parseOptions(opt []string) (ZfsOptions, error) {
	var options ZfsOptions
	options.fsName = ""
	for _, option := range opt {
		key, val, err := parsers.ParseKeyValueOpt(option)
		if err != nil {
			return options, err
		}
		key = strings.ToLower(key)
		switch key {
		case "zfs.fsname":
			options.fsName = val
		default:
			return options, fmt.Errorf("Unknown option %s\n", key)
		}
	}
	return options, nil
}

func checkRootdirFs(rootdir string) error {
	var buf syscall.Statfs_t
	if err := syscall.Statfs(rootdir, &buf); err != nil {
		fmt.Errorf("Failed to access '%s': %s", rootdir, err)
	}

	if graphdriver.FsMagic(buf.Type) != graphdriver.FsMagicZfs {
		log.Debugf("[zfs] no zfs dataset found for rootdir '%s'", rootdir)
		return graphdriver.ErrPrerequisites
	}
	return nil
}

var CprocMounts = C.CString("/proc/mounts")
var CopenMod = C.CString("r")

func lookupZfsDataset(rootdir string) (string, error) {
	var stat syscall.Stat_t
	var Cmnt C.struct_mntent
	var Cfp *C.FILE
	buf := string(make([]byte, 256, 256))
	Cbuf := C.CString(buf)
	defer free(Cbuf)

	if err := syscall.Stat(rootdir, &stat); err != nil {
		return "", fmt.Errorf("Failed to access '%s': %s", rootdir, err)
	}
	wantedDev := stat.Dev

	if Cfp = C.setmntent(CprocMounts, CopenMod); Cfp == nil {
		return "", fmt.Errorf("Failed to open /proc/mounts")
	}
	defer C.endmntent(Cfp)

	for C.getmntent_r(Cfp, &Cmnt, Cbuf, 256) != nil {
		dir := C.GoString(Cmnt.mnt_dir)
		if err := syscall.Stat(dir, &stat); err != nil {
			log.Debugf("[zfs] failed to stat '%s' while scanning for zfs mount: %v", dir, err)
			continue // may fail on fuse file systems
		}

		fs := C.GoString(Cmnt.mnt_type)
		if stat.Dev == wantedDev && fs == "zfs" {
			return C.GoString(Cmnt.mnt_fsname), nil
		}
	}
	// should never happen
	return "", fmt.Errorf("Failed to find zfs pool in /proc/mounts")
}

func free(p *C.char) {
	C.free(unsafe.Pointer(p))
}

type Driver struct {
	dataset *zfs.Dataset
	options ZfsOptions
}

func (d *Driver) String() string {
	return "zfs"
}

func (d *Driver) Cleanup() error {
	return nil
}

func (d *Driver) Status() [][2]string {
	parts := strings.Split(d.dataset.Name, "/")
	pool, err := zfs.GetZpool(parts[0])

	if err != nil {
		return [][2]string{
			{"error while getting pool", fmt.Sprintf("%v", err)},
		}
	}
	var quota string
	if d.dataset.Quota == 0 {
		quota = strconv.FormatUint(d.dataset.Quota, 10)
	} else {
		quota = "no"
	}

	return [][2]string{
		{"Zpool", pool.Name},
		{"Zpool Health", pool.Health},
		{"Parent Dataset", d.dataset.Name},
		{"Space Used By Parent", strconv.FormatUint(d.dataset.Used, 10)},
		{"Space Available", strconv.FormatUint(d.dataset.Avail, 10)},
		{"Parent Quota", quota},
		{"Compression", d.dataset.Compression},
	}
}

func cloneFilesystem(id, parent, mountpoint string) error {
	parentDataset, err := zfs.GetDataset(parent)
	if parentDataset == nil {
		return err
	}
	snapshotName := fmt.Sprintf("%d", time.Now().Nanosecond())
	snapshot, err := parentDataset.Snapshot(snapshotName /*recursive */, false)
	if snapshot == nil {
		return err
	}

	_, err = snapshot.Clone(id, map[string]string{
		"mountpoint": mountpoint,
	})
	if err != nil {
		snapshot.Destroy(zfs.DestroyDeferDeletion)
		return err
	}
	err = snapshot.Destroy(zfs.DestroyDeferDeletion)
	return err
}

func (d *Driver) ZfsPath(id string) string {
	return d.options.fsName + "/" + id
}

func (d *Driver) Create(id string, parent string) error {
	mountPoint := path.Join(d.options.mountPath, "graph", id)
	datasetName := d.ZfsPath(id)
	dataset, err := zfs.GetDataset(datasetName)
	if err == nil {
		// cleanup existing dataset from an aborted build
		dataset.Destroy(zfs.DestroyRecursiveClones)
	}

	if parent == "" {
		_, err := zfs.CreateFilesystem(datasetName, map[string]string{
			"mountpoint": mountPoint,
		})
		return err
	} else {
		return cloneFilesystem(datasetName, d.ZfsPath(parent), mountPoint)
	}
	return nil
}

func (d *Driver) Remove(id string) error {
	dataset, err := zfs.GetDataset(d.ZfsPath(id))
	if dataset == nil {
		return err
	}

	return dataset.Destroy(zfs.DestroyRecursive)
}

func (d *Driver) Get(id, mountLabel string) (string, error) {
	dataset, err := zfs.GetDataset(d.ZfsPath(id))
	if dataset == nil {
		return "", err
	} else {
		return dataset.Mountpoint, nil
	}
}

func (d *Driver) Put(id string) error {
	// FS is already mounted
	return nil
}

func (d *Driver) Exists(id string) bool {
	_, err := zfs.GetDataset(d.ZfsPath(id))
	return err == nil
}
