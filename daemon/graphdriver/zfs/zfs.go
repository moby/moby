package zfs

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"syscall"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/daemon/graphdriver"
	"github.com/docker/docker/pkg/mount"
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
	if err != nil {
		return nil, err
	}
	options.mountPath = base

	rootdir := path.Dir(base)

	if options.fsName == "" {
		err = checkRootdirFs(rootdir)
		if err != nil {
			return nil, err
		}
	}

	if _, err := exec.LookPath("zfs"); err != nil {
		return nil, fmt.Errorf("zfs command is not available: %v", err)
	}

	file, err := os.OpenFile("/dev/zfs", os.O_RDWR, 600)
	if err != nil {
		return nil, fmt.Errorf("cannot open /dev/zfs: %v", err)
	}
	defer file.Close()

	if options.fsName == "" {
		options.fsName, err = lookupZfsDataset(rootdir)
		if err != nil {
			return nil, err
		}
	}

	zfs.SetLogger(new(Logger))

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
			return options, fmt.Errorf("Unknown option %s", key)
		}
	}
	return options, nil
}

func checkRootdirFs(rootdir string) error {
	var buf syscall.Statfs_t
	if err := syscall.Statfs(rootdir, &buf); err != nil {
		return fmt.Errorf("Failed to access '%s': %s", rootdir, err)
	}

	if graphdriver.FsMagic(buf.Type) != graphdriver.FsMagicZfs {
		log.Debugf("[zfs] no zfs dataset found for rootdir '%s'", rootdir)
		return graphdriver.ErrPrerequisites
	}
	return nil
}

func lookupZfsDataset(rootdir string) (string, error) {
	var stat syscall.Stat_t
	if err := syscall.Stat(rootdir, &stat); err != nil {
		return "", fmt.Errorf("Failed to access '%s': %s", rootdir, err)
	}
	wantedDev := stat.Dev

	mounts, err := mount.GetMounts()
	if err != nil {
		return "", err
	}
	for _, m := range mounts {
		if err := syscall.Stat(m.Mountpoint, &stat); err != nil {
			log.Debugf("[zfs] failed to stat '%s' while scanning for zfs mount: %v", m.Mountpoint, err)
			continue // may fail on fuse file systems
		}

		if stat.Dev == wantedDev && m.Fstype == "zfs" {
			return m.Source, nil
		}
	}

	return "", fmt.Errorf("Failed to find zfs dataset mounted on '%s' in /proc/mounts", rootdir)
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

	var poolName, poolHealth string
	if err == nil {
		poolName = pool.Name
		poolHealth = pool.Health
	} else {
		poolName = fmt.Sprintf("error while getting pool information %v", err)
		poolHealth = "not available"
	}

	quota := "no"
	if d.dataset.Quota != 0 {
		quota = strconv.FormatUint(d.dataset.Quota, 10)
	}

	return [][2]string{
		{"Zpool", poolName},
		{"Zpool Health", poolHealth},
		{"Parent Dataset", d.dataset.Name},
		{"Space Used By Parent", strconv.FormatUint(d.dataset.Used, 10)},
		{"Space Available", strconv.FormatUint(d.dataset.Avail, 10)},
		{"Parent Quota", quota},
		{"Compression", d.dataset.Compression},
	}
}

func cloneFilesystem(id, parent, mountpoint string) error {
	parentDataset, err := zfs.GetDataset(parent)
	if err != nil {
		return err
	}
	snapshotName := fmt.Sprintf("%d", time.Now().Nanosecond())
	snapshot, err := parentDataset.Snapshot(snapshotName /*recursive */, false)
	if err != nil {
		return err
	}

	_, err = snapshot.Clone(id, map[string]string{
		"mountpoint": mountpoint,
	})
	if err != nil {
		snapshot.Destroy(zfs.DestroyDeferDeletion)
		return err
	}
	return snapshot.Destroy(zfs.DestroyDeferDeletion)
}

func (d *Driver) ZfsPath(id string) string {
	return d.options.fsName + "/" + id
}

func (d *Driver) Create(id string, parent string) error {
	datasetName := d.ZfsPath(id)
	dataset, err := zfs.GetDataset(datasetName)
	if err == nil {
		// cleanup existing dataset from an aborted build
		err := dataset.Destroy(zfs.DestroyRecursiveClones)
		if err != nil {
			log.Warnf("[zfs] failed to destroy dataset '%s': %v", dataset.Name, err)
		}
	} else if zfsError, ok := err.(*zfs.Error); ok {
		if !strings.HasSuffix(zfsError.Stderr, "dataset does not exist\n") {
			return err
		}
	} else {
		return err
	}

	mountPoint := path.Join(d.options.mountPath, "graph", id)
	if parent == "" {
		_, err := zfs.CreateFilesystem(datasetName, map[string]string{
			"mountpoint": mountPoint,
		})
		return err
	}
	return cloneFilesystem(datasetName, d.ZfsPath(parent), mountPoint)
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
	if err != nil {
		return "", err
	}
	return dataset.Mountpoint, nil
}

func (d *Driver) Put(id string) error {
	// FS is already mounted
	return nil
}

func (d *Driver) Exists(id string) bool {
	_, err := zfs.GetDataset(d.ZfsPath(id))
	return err == nil
}
