package graphdriver

import (
	"errors"
	"fmt"
	"os"
	"path"

	"github.com/docker/docker/archive"
	"github.com/docker/docker/pkg/mount"
)

type FsMagic uint64

const (
	FsMagicBtrfs = FsMagic(0x9123683E)
	FsMagicAufs  = FsMagic(0x61756673)
)

type InitFunc func(root string, options []string) (Driver, error)

type Driver interface {
	String() string

	Create(id, parent string) error
	Remove(id string) error

	Get(id, mountLabel string) (dir string, err error)
	Put(id string)
	Exists(id string) bool

	Status() [][2]string

	Cleanup() error
}

type Differ interface {
	Diff(id string) (archive.Archive, error)
	Changes(id string) ([]archive.Change, error)
	ApplyDiff(id string, diff archive.ArchiveReader) error
	DiffSize(id string) (bytes int64, err error)
}

var (
	DefaultDriver string
	// All registred drivers
	drivers map[string]InitFunc
	// Slice of drivers that should be used in an order
	priority = []string{
		"aufs",
		"btrfs",
		"devicemapper",
		"vfs",
	}

	ErrNotSupported   = errors.New("driver not supported")
	ErrPrerequisites  = errors.New("prerequisites for driver not satisfied (wrong filesystem?)")
	ErrIncompatibleFS = fmt.Errorf("backing file system is unsupported for this graph driver")
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

func GetDriver(name, home string, options []string) (Driver, error) {
	if initFunc, exists := drivers[name]; exists {
		return initFunc(path.Join(home, name), options)
	}
	return nil, ErrNotSupported
}

func New(root string, options []string) (driver Driver, err error) {
	for _, name := range []string{os.Getenv("DOCKER_DRIVER"), DefaultDriver} {
		if name != "" {
			return GetDriver(name, root, options)
		}
	}

	// Check for priority drivers first
	for _, name := range priority {
		driver, err = GetDriver(name, root, options)
		if err != nil {
			if err == ErrNotSupported || err == ErrPrerequisites || err == ErrIncompatibleFS {
				continue
			}
			return nil, err
		}
		return driver, nil
	}

	// Check all registered drivers if no priority driver is found
	for _, initFunc := range drivers {
		if driver, err = initFunc(root, options); err != nil {
			if err == ErrNotSupported || err == ErrPrerequisites || err == ErrIncompatibleFS {
				continue
			}
			return nil, err
		}
		return driver, nil
	}
	return nil, fmt.Errorf("No supported storage backend found")
}

func MakePrivate(mountPoint string) error {
	mounted, err := mount.Mounted(mountPoint)
	if err != nil {
		return err
	}

	if !mounted {
		if err := mount.Mount(mountPoint, mountPoint, "none", "bind,rw"); err != nil {
			return err
		}
	}

	return mount.ForceMount("", mountPoint, "none", "private")
}
