package graphdriver

import (
	"bufio"
	"fmt"
	"github.com/dotcloud/docker/archive"
	"github.com/dotcloud/docker/utils"
	"os"
	"path"
	"strings"
	"syscall"
	"time"
)

const mountinfoFormat = "%d %d %d:%d %s %s %s - %s %s %s"

type InitFunc func(root string) (Driver, error)

type Driver interface {
	String() string

	Create(id, parent string) error
	Remove(id string) error

	Get(id string) (dir string, err error)
	Exists(id string) bool

	Status() [][2]string

	Cleanup() error
}

type Differ interface {
	Diff(id string) (archive.Archive, error)
	Changes(id string) ([]archive.Change, error)
	ApplyDiff(id string, diff archive.Archive) error
	DiffSize(id string) (bytes int64, err error)
}

type Mount struct {
	Device  string
	Target  string
	Type    string
	Options string
}

var (
	DefaultDriver string
	// All registred drivers
	drivers map[string]InitFunc
	// Slice of drivers that should be used in an order
	priority = []string{
		"aufs",
		"devicemapper",
		"vfs",
	}
)

func init() {
	drivers = make(map[string]InitFunc)
}

func Register(name string, initFunc InitFunc) error {
	if _, exists := drivers[name]; exists {
		return fmt.Errorf("Name already registered %s", name)
	}
	drivers[name] = initFunc

	return nil
}

func GetDriver(name, home string) (Driver, error) {
	if initFunc, exists := drivers[name]; exists {
		return initFunc(path.Join(home, name))
	}
	return nil, fmt.Errorf("No such driver: %s", name)
}

func New(root string) (driver Driver, err error) {
	for _, name := range []string{os.Getenv("DOCKER_DRIVER"), DefaultDriver} {
		if name != "" {
			return GetDriver(name, root)
		}
	}

	// Check for priority drivers first
	for _, name := range priority {
		if driver, err = GetDriver(name, root); err != nil {
			utils.Debugf("Error loading driver %s: %s", name, err)
			continue
		}
		return driver, nil
	}

	// Check all registered drivers if no priority driver is found
	for _, initFunc := range drivers {
		if driver, err = initFunc(root); err != nil {
			continue
		}
		return driver, nil
	}
	return nil, err
}

func (m *Mount) Mount(root string) error {
	target := path.Join(root, m.Target)
	if mounted, err := Mounted(target); err != nil || mounted {
		return err
	}

	flag, data := parseOptions(m.Options)
	if err := syscall.Mount(m.Device, target, m.Type, uintptr(flag), data); err != nil {
		return err
	}
	return nil
}

func parseOptions(options string) (int, string) {
	var (
		flag int
		data []string
	)

	flags := map[string]struct {
		clear bool
		flag  int
	}{
		"defaults":      {false, 0},
		"ro":            {false, syscall.MS_RDONLY},
		"rw":            {true, syscall.MS_RDONLY},
		"suid":          {true, syscall.MS_NOSUID},
		"nosuid":        {false, syscall.MS_NOSUID},
		"dev":           {true, syscall.MS_NODEV},
		"nodev":         {false, syscall.MS_NODEV},
		"exec":          {true, syscall.MS_NOEXEC},
		"noexec":        {false, syscall.MS_NOEXEC},
		"sync":          {false, syscall.MS_SYNCHRONOUS},
		"async":         {true, syscall.MS_SYNCHRONOUS},
		"dirsync":       {false, syscall.MS_DIRSYNC},
		"remount":       {false, syscall.MS_REMOUNT},
		"mand":          {false, syscall.MS_MANDLOCK},
		"nomand":        {true, syscall.MS_MANDLOCK},
		"atime":         {true, syscall.MS_NOATIME},
		"noatime":       {false, syscall.MS_NOATIME},
		"diratime":      {true, syscall.MS_NODIRATIME},
		"nodiratime":    {false, syscall.MS_NODIRATIME},
		"bind":          {false, syscall.MS_BIND},
		"rbind":         {false, syscall.MS_BIND | syscall.MS_REC},
		"relatime":      {false, syscall.MS_RELATIME},
		"norelatime":    {true, syscall.MS_RELATIME},
		"strictatime":   {false, syscall.MS_STRICTATIME},
		"nostrictatime": {true, syscall.MS_STRICTATIME},
	}

	for _, o := range strings.Split(options, ",") {
		// If the option does not exist in the flags table then it is a
		// data value for a specific fs type
		if f, exists := flags[o]; exists {
			if f.clear {
				flag &= ^f.flag
			} else {
				flag |= f.flag
			}
		} else {
			data = append(data, o)
		}
	}
	return flag, strings.Join(data, ",")
}

func (m *Mount) Unmount(root string) (err error) {
	target := path.Join(root, m.Target)
	if mounted, err := Mounted(target); err != nil || !mounted {
		return err
	}

	// Simple retry logic for unmount
	for i := 0; i < 10; i++ {
		if err = syscall.Unmount(target, 0); err == nil {
			return nil
		}
		utils.Debugf("[Unmount] %s", err)
		time.Sleep(100 * time.Millisecond)
	}
	return
}

func Mounted(mountpoint string) (bool, error) {
	entries, err := parseMountTable()
	if err != nil {
		return false, err
	}

	// Search the table for the mountpoint
	for _, e := range entries {
		if e.mountpoint == mountpoint {
			return true, nil
		}
	}
	return false, nil
}

// Represents one line from /proc/self/mountinfo
type procEntry struct {
	id, parent, major, minor           int
	source, mountpoint, fstype, device string
	vfsopts, opts                      string
}

// Parse /proc/self/mountinfo because comparing Dev and ino does not work from bind mounts
//
// 180 20 0:2851 / /var/lib/docker/aufs/mnt/a22632d4ed3cb2438246064408f9f07734cbd331f50c81f1ca3dcbd78541ce83 rw,relatime - aufs none rw,si=e9663ac1adbdb4f8
func parseMountTable() ([]*procEntry, error) {
	f, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	out := []*procEntry{}
	for s.Scan() {
		if err := s.Err(); err != nil {
			return nil, err
		}

		p := &procEntry{}
		if _, err := fmt.Sscanf(s.Text(), mountinfoFormat,
			&p.id, &p.parent, &p.major, &p.minor,
			&p.source, &p.mountpoint, &p.vfsopts, &p.fstype,
			&p.device, &p.opts); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}
