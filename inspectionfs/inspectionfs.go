package inspectionfs

import (
	"path/filepath"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
	"github.com/hanwen/go-fuse/fuse/pathfs"
)

type DaemonConnector interface {
	ReadJSON(path string) ([]byte, error)
}

type entry struct {
	attr *fuse.Attr
	file func(*FS, string) (nodefs.File, fuse.Status)
}

var hier = map[string]entry{
	"":                {&fuse.Attr{Mode: fuse.S_IFDIR | 0500}, nil},
	"container":       {&fuse.Attr{Mode: fuse.S_IFDIR | 0500}, nil},
	"container/json":  {&fuse.Attr{Mode: fuse.S_IFREG | 0500}, openJSON},
	"swarm":           {&fuse.Attr{Mode: fuse.S_IFDIR | 0500}, nil},
	"swarm/task":      {&fuse.Attr{Mode: fuse.S_IFDIR | 0500}, nil},
	"swarm/task/json": {&fuse.Attr{Mode: fuse.S_IFREG | 0500}, openJSON},
}

func init() {
	now := time.Now()
	sec, nsec := uint64(now.Unix()), uint32(now.Nanosecond())
	for _, entry := range hier {
		attr := entry.attr
		attr.Ctime, attr.Ctimensec = sec, nsec
		attr.Mtime, attr.Mtimensec = sec, nsec
	}
}

// FS is a FUSE filesystem object
type FS struct {
	pathfs.FileSystem
	connector DaemonConnector
}

func (fs *FS) GetAttr(name string, context *fuse.Context) (*fuse.Attr, fuse.Status) {
	for f, entry := range hier {
		if f == name {
			if entry.file != nil {
				file, status := entry.file(fs, name)
				if status != fuse.OK {
					return nil, status
				}
				var attr fuse.Attr
				status = file.GetAttr(&attr)
				return &attr, status
			}
			return entry.attr, fuse.OK
		}
	}
	return nil, fuse.ENOENT
}

func (fs *FS) OpenDir(name string, context *fuse.Context) ([]fuse.DirEntry, fuse.Status) {
	var dentries []fuse.DirEntry
	for f, entry := range hier {
		dir, base := filepath.Dir(f), filepath.Base(f)
		if dir == name || (dir == "." && name == "") {
			dentry := fuse.DirEntry{
				Name: base,
				Mode: entry.attr.Mode,
			}
			dentries = append(dentries, dentry)
		}
	}
	return dentries, fuse.OK
}

func (fs *FS) Open(name string, flags uint32, context *fuse.Context) (nodefs.File, fuse.Status) {
	for f, entry := range hier {
		if f == name && entry.file != nil {
			return entry.file(fs, name)
		}
	}
	return nil, fuse.ENOENT
}

func openJSON(fs *FS, path string) (nodefs.File, fuse.Status) {
	b, err := fs.connector.ReadJSON(path)
	if err != nil {
		logrus.Warnf("error while opening %s: %v", path, err)
		return nil, fuse.EIO
	}
	return nodefs.NewDataFile(b), fuse.OK
}

func NewServer(mountpoint string, connector DaemonConnector) (*fuse.Server, error) {
	fs := pathfs.NewPathNodeFs(&FS{FileSystem: pathfs.NewDefaultFileSystem(), connector: connector}, nil)
	server, _, err := nodefs.MountRoot(mountpoint, fs.Root(), nil)
	return server, err
}
