// +build linux freebsd

package zfs

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/daemon/graphdriver"
	"github.com/docker/docker/pkg/idtools"
	"github.com/docker/docker/pkg/mount"
	"github.com/docker/docker/pkg/parsers"
	"github.com/docker/go-units"
	zfs "github.com/mistifyio/go-zfs"
	"github.com/opencontainers/runc/libcontainer/label"
)

const (
	// valid cli options
	zfsNameOption             = "zfs.fsname"
	zfsLoopbackGenerateOption = "zfs.loopback.generate"
	zfsLoopbackNameOption     = "zfs.loopback.name"
	zfsLoopbackPathOption     = "zfs.loopback.path"
	zfsLoopbackSizeOption     = "zfs.loopback.size"

	// default values
	defaultLoopbackName = "zroot-docker-lo"
	defaultLoopbackPath = "/var/lib/docker.img"
	defaultLoopbackSize = "10G"
)

type activeMount struct {
	count   int
	path    string
	mounted bool
}

type zfsOptions struct {
	fsName           string
	mountPath        string
	loopbackPath     string
	loopbackSize     string
	loopbackName     string
	loopbackGenerate bool
}

func init() {
	graphdriver.Register("zfs", bootstrapZFS{})
}

// Logger returns a zfs logger implementation.
type Logger struct{}

// Log wraps log message from ZFS driver with a prefix '[zfs]'.
func (*Logger) Log(cmd []string) {
	logrus.Debugf("[zfs] %s", strings.Join(cmd, " "))
}

type bootstrapZFS struct{}

// ValidateSupport validates that DevMapper can be used in this host.
// It returns an error if the kernel doesn't support ZFS
// or the command line tools are not installed in the host.
func (bootstrapZFS) ValidateSupport(_ string) error {
	if _, err := exec.LookPath("zfs"); err != nil {
		logrus.Debugf("[zfs] zfs command is not available: %v", err)
		return graphdriver.ErrPrerequisites
	}

	file, err := os.OpenFile("/dev/zfs", os.O_RDWR, 600)
	if err != nil {
		logrus.Debugf("[zfs] cannot open /dev/zfs: %v", err)
		return graphdriver.ErrPrerequisites
	}
	file.Close()
	return nil
}

// Init initializes a new ZFS driver.
// It bootstraps the Zpool and Dataset if the option is set too.
func (bootstrapZFS) Init(base string, opt []string, uidMaps, gidMaps []idtools.IDMap) (d graphdriver.Driver, graphErr error) {
	options, err := parseOptions(opt)
	if err != nil {
		return nil, err
	}
	options.mountPath = base
	rootDir := path.Dir(base)

	if options.loopbackGenerate {
		fsName, err := generateZFSLoopback(rootDir, options)
		if err != nil {
			return nil, err
		}
		options.fsName = fsName

		defer func() {
			if graphErr != nil {
				destroyLoopbackZpool(options.loopbackName)
			}
		}()
	}

	if err := graphdriver.InitRootFilesystem(rootDir, opt, uidMaps, gidMaps); err != nil {
		return nil, err
	}

	return initDriver(rootDir, options, uidMaps, gidMaps)
}

// initDriver returns a new ZFS driver.
// It takes base mount path and a array of options which are represented as key value pairs.
// Each option is in the for key=value. 'zfs.fsname' is expected to be a valid key in the options.
func initDriver(rootDir string, options zfsOptions, uidMaps, gidMaps []idtools.IDMap) (graphdriver.Driver, error) {
	if options.fsName == "" {
		if err := checkRootdirFs(rootDir); err != nil {
			return nil, err
		}
		fsName, err := lookupZfsDataset(rootDir)
		if err != nil {
			return nil, err
		}
		options.fsName = fsName
	}

	zfs.SetLogger(new(Logger))

	filesystems, err := zfs.Filesystems(options.fsName)
	if err != nil {
		return nil, fmt.Errorf("Cannot find root filesystem %s: %v", options.fsName, err)
	}

	filesystemsCache := make(map[string]bool, len(filesystems))
	var rootDataset *zfs.Dataset
	for _, fs := range filesystems {
		if fs.Name == options.fsName {
			rootDataset = fs
		}
		filesystemsCache[fs.Name] = true
	}

	if rootDataset == nil {
		return nil, fmt.Errorf("BUG: zfs get all -t filesystem -rHp '%s' should contain '%s'", options.fsName, options.fsName)
	}

	d := &Driver{
		dataset:          rootDataset,
		options:          options,
		filesystemsCache: filesystemsCache,
		uidMaps:          uidMaps,
		gidMaps:          gidMaps,
	}
	return graphdriver.NewNaiveDiffDriver(d, uidMaps, gidMaps), nil
}

func parseOptions(opt []string) (zfsOptions, error) {
	var options zfsOptions
	options.fsName = ""
	for _, option := range opt {
		key, val, err := parsers.ParseKeyValueOpt(option)
		if err != nil {
			return options, err
		}
		key = strings.ToLower(key)
		switch key {
		case zfsNameOption:
			options.fsName = val
		case zfsLoopbackGenerateOption:
			b, err := strconv.ParseBool(val)
			if err != nil {
				return options, err
			}
			options.loopbackGenerate = b
		case zfsLoopbackNameOption:
			options.loopbackName = val
		case zfsLoopbackPathOption:
			options.loopbackPath = val
		case zfsLoopbackSizeOption:
			if _, err := units.FromHumanSize(val); err != nil {
				return options, err
			}
			options.loopbackSize = val
		default:
			return options, fmt.Errorf("Unknown option %s", key)
		}
	}
	return options, nil
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
			logrus.Debugf("[zfs] failed to stat '%s' while scanning for zfs mount: %v", m.Mountpoint, err)
			continue // may fail on fuse file systems
		}

		if stat.Dev == wantedDev && m.Fstype == "zfs" {
			return m.Source, nil
		}
	}

	return "", fmt.Errorf("Failed to find zfs dataset mounted on '%s' in /proc/mounts", rootdir)
}

// Driver holds information about the driver, such as zfs dataset, options and cache.
type Driver struct {
	dataset          *zfs.Dataset
	options          zfsOptions
	sync.Mutex       // protects filesystem cache against concurrent access
	filesystemsCache map[string]bool
	uidMaps          []idtools.IDMap
	gidMaps          []idtools.IDMap
}

func (d *Driver) String() string {
	return "zfs"
}

// Cleanup is used to implement graphdriver.ProtoDriver. There is no cleanup required for this driver.
func (d *Driver) Cleanup() error {
	return nil
}

// Status returns information about the ZFS filesystem. It returns a two dimensional array of information
// such as pool name, dataset name, disk usage, parent quota and compression used.
// Currently it return 'Zpool', 'Zpool Health', 'Parent Dataset', 'Space Used By Parent',
// 'Space Available', 'Parent Quota' and 'Compression'.
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

	status := [][2]string{
		{"Zpool", poolName},
		{"Zpool Health", poolHealth},
		{"Parent Dataset", d.dataset.Name},
		{"Space Used By Parent", strconv.FormatUint(d.dataset.Used, 10)},
		{"Space Available", strconv.FormatUint(d.dataset.Avail, 10)},
		{"Parent Quota", quota},
		{"Compression", d.dataset.Compression},
	}

	if d.options.loopbackGenerate {
		status = append(status, [2]string{"Data Loop File", d.options.loopbackPath})
	}

	return status
}

// GetMetadata returns image/container metadata related to graph driver
func (d *Driver) GetMetadata(id string) (map[string]string, error) {
	return nil, nil
}

func (d *Driver) cloneFilesystem(name, parentName string) error {
	snapshotName := fmt.Sprintf("%d", time.Now().Nanosecond())
	parentDataset := zfs.Dataset{Name: parentName}
	snapshot, err := parentDataset.Snapshot(snapshotName /*recursive */, false)
	if err != nil {
		return err
	}

	_, err = snapshot.Clone(name, map[string]string{"mountpoint": "legacy"})
	if err == nil {
		d.Lock()
		d.filesystemsCache[name] = true
		d.Unlock()
	}

	if err != nil {
		snapshot.Destroy(zfs.DestroyDeferDeletion)
		return err
	}
	return snapshot.Destroy(zfs.DestroyDeferDeletion)
}

func (d *Driver) zfsPath(id string) string {
	return d.options.fsName + "/" + id
}

func (d *Driver) mountPath(id string) string {
	return path.Join(d.options.mountPath, "graph", getMountpoint(id))
}

// Create prepares the dataset and filesystem for the ZFS driver for the given id under the parent.
func (d *Driver) Create(id string, parent string, mountLabel string, storageOpt map[string]string) error {
	if len(storageOpt) != 0 {
		return fmt.Errorf("--storage-opt is not supported for zfs")
	}

	err := d.create(id, parent)
	if err == nil {
		return nil
	}
	if zfsError, ok := err.(*zfs.Error); ok {
		if !strings.HasSuffix(zfsError.Stderr, "dataset already exists\n") {
			return err
		}
		// aborted build -> cleanup
	} else {
		return err
	}

	dataset := zfs.Dataset{Name: d.zfsPath(id)}
	if err := dataset.Destroy(zfs.DestroyRecursiveClones); err != nil {
		return err
	}

	// retry
	return d.create(id, parent)
}

func (d *Driver) create(id, parent string) error {
	name := d.zfsPath(id)
	if parent == "" {
		mountoptions := map[string]string{"mountpoint": "legacy"}
		fs, err := zfs.CreateFilesystem(name, mountoptions)
		if err == nil {
			d.Lock()
			d.filesystemsCache[fs.Name] = true
			d.Unlock()
		}
		return err
	}
	return d.cloneFilesystem(name, d.zfsPath(parent))
}

// Remove deletes the dataset, filesystem and the cache for the given id.
func (d *Driver) Remove(id string) error {
	name := d.zfsPath(id)
	dataset := zfs.Dataset{Name: name}
	err := dataset.Destroy(zfs.DestroyRecursive)
	if err == nil {
		d.Lock()
		delete(d.filesystemsCache, name)
		d.Unlock()
	}
	return err
}

// Get returns the mountpoint for the given id after creating the target directories if necessary.
func (d *Driver) Get(id, mountLabel string) (string, error) {
	mountpoint := d.mountPath(id)
	filesystem := d.zfsPath(id)
	options := label.FormatMountLabel("", mountLabel)
	logrus.Debugf(`[zfs] mount("%s", "%s", "%s")`, filesystem, mountpoint, options)

	rootUID, rootGID, err := idtools.GetRootUIDGID(d.uidMaps, d.gidMaps)
	if err != nil {
		return "", err
	}
	// Create the target directories if they don't exist
	if err := idtools.MkdirAllAs(mountpoint, 0755, rootUID, rootGID); err != nil {
		return "", err
	}

	if err := mount.Mount(filesystem, mountpoint, "zfs", options); err != nil {
		return "", fmt.Errorf("error creating zfs mount of %s to %s: %v", filesystem, mountpoint, err)
	}
	// this could be our first mount after creation of the filesystem, and the root dir may still have root
	// permissions instead of the remapped root uid:gid (if user namespaces are enabled):
	if err := os.Chown(mountpoint, rootUID, rootGID); err != nil {
		return "", fmt.Errorf("error modifying zfs mountpoint (%s) directory ownership: %v", mountpoint, err)
	}

	return mountpoint, nil
}

// Put removes the existing mountpoint for the given id if it exists.
func (d *Driver) Put(id string) error {
	mountpoint := d.mountPath(id)
	mounted, err := graphdriver.Mounted(graphdriver.FsMagicZfs, mountpoint)
	if err != nil || !mounted {
		return err
	}

	logrus.Debugf(`[zfs] unmount("%s")`, mountpoint)

	if err := mount.Unmount(mountpoint); err != nil {
		return fmt.Errorf("error unmounting to %s: %v", mountpoint, err)
	}
	return nil
}

// Exists checks to see if the cache entry exists for the given id.
func (d *Driver) Exists(id string) bool {
	d.Lock()
	defer d.Unlock()
	return d.filesystemsCache[d.zfsPath(id)] == true
}

func generateZFSLoopback(rootDir string, options zfsOptions) (string, error) {
	size := defaultLoopbackSize
	if options.loopbackSize != "" {
		size = options.loopbackSize
	}
	loPath := defaultLoopbackPath
	if options.loopbackPath != "" {
		loPath = options.loopbackPath
	}
	loName := defaultLoopbackName
	if options.loopbackName != "" {
		loName = options.loopbackName
	}

	// create the Zpool and the loopback image only when it doesn't
	// exist already.
	if _, err := zfs.GetZpool(loName); err != nil {
		if err := generateLoopbackImage(loPath, size); err != nil {
			return "", err
		}

		if _, err := zfs.CreateZpool(loName, nil, loPath, "-m", "none"); err != nil {
			return "", err
		}
	}

	fsName := loName + "/dataset"
	// create the Dataset only when it doesn't exist already.
	if _, err := zfs.GetDataset(fsName); err != nil {
		logrus.Debugf("[zfs] creating new dataset %s, mounted in %s", fsName, rootDir)
		props := map[string]string{
			"compression": "lz4", // best compression
			"mountpoint":  rootDir,
		}
		fi, err := os.Stat(rootDir)
		if err != nil {
			if !os.IsNotExist(err) {
				return "", err
			}
			logrus.Debugf("[zfs] creating mount point in %s", rootDir)
			if err := os.MkdirAll(rootDir, 0700); err != nil {
				return "", err
			}
		} else if !fi.IsDir() {
			return "", fmt.Errorf("%s is not a directory", fi.Name())
		}

		if _, err := zfs.CreateFilesystem(fsName, props); err != nil {
			return "", err
		}
	}
	return fsName, nil
}

func generateLoopbackImage(loPath, size string) error {
	fi, err := os.Stat(loPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err == nil {
		if fi.IsDir() {
			return fmt.Errorf("%s is a directory", fi.Name())
		}
		// we don't want to truncate the image
		// if it already exists.
		return nil
	}

	p, err := exec.LookPath("truncate")
	if err != nil {
		return err
	}

	return exec.Command(p, "-s", size, loPath).Run()
}

func destroyLoopbackZpool(loName string) {
	fsName := loName + "/dataset"
	ds, err := zfs.GetDataset(fsName)
	if err != nil {
		return
	}
	ds.Destroy(zfs.DestroyRecursive | zfs.DestroyForceUmount)

	zp, err := zfs.GetZpool(loName)
	if err != nil {
		return
	}
	zp.Destroy()
}
