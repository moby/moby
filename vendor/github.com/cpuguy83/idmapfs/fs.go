package idmapfs

import (
	"fmt"
	"io"
	"io/ioutil"
	"runtime"
	"time"

	"github.com/cpuguy83/idmapfs/idtools"
	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
	"github.com/hanwen/go-fuse/fuse/pathfs"
)

func New(fs pathfs.FileSystem, m *idtools.IdentityMapping, fsName string, logger io.Writer) pathfs.FileSystem {
	if logger == nil {
		logger = ioutil.Discard
	}
	return &mapFS{
		base:        fs,
		m:           m,
		name:        fsName,
		debugWriter: logger,
	}
}

type mapFS struct {
	base        pathfs.FileSystem
	m           *idtools.IdentityMapping
	name        string
	debug       bool
	debugWriter io.Writer
}

func (m *mapFS) String() string {
	if m.name == "" {
		return m.base.String()
	}
	return m.name
}

func (m *mapFS) SetDebug(debug bool) {
	m.debug = debug
	m.base.SetDebug(debug)
}

func (m *mapFS) GetAttr(name string, c *fuse.Context) (attr *fuse.Attr, status fuse.Status) {
	m.unmapContext(c)

	if m.debug {
		defer func() {
			var da fuse.Attr
			if attr != nil {
				da = *attr
			}
			fmt.Fprintf(m.debugWriter, "GetAttr name: %s, context: %+v, attr: %+v, status: %s\n", name, *c, da, status)
		}()
	}

	a, status := m.base.GetAttr(name, c)
	if a != nil {
		m.mapAttr(a)
	}
	return a, status
}

func (m *mapFS) Chmod(name string, mode uint32, c *fuse.Context) fuse.Status {
	m.unmapContext(c)
	return m.base.Chmod(name, mode, c)
}

func (m *mapFS) Chown(name string, uid uint32, gid uint32, c *fuse.Context) fuse.Status {
	id, err := m.m.ToHost(idtools.Identity{UID: int(uid), GID: int(gid)})
	if err != nil {
		uid = uint32(id.UID)
		gid = uint32(id.GID)
	}
	m.unmapContext(c)
	return m.base.Chown(name, uid, gid, c)
}

func (m *mapFS) Utimens(name string, aTime *time.Time, mTime *time.Time, c *fuse.Context) fuse.Status {
	m.unmapContext(c)
	return m.base.Utimens(name, aTime, mTime, c)
}

func (m *mapFS) Truncate(name string, size uint64, c *fuse.Context) fuse.Status {
	m.unmapContext(c)
	return m.base.Truncate(name, size, c)
}

func (m *mapFS) Access(name string, mode uint32, c *fuse.Context) (s fuse.Status) {
	if m.debug {
		defer func() {
			fmt.Fprintf(m.debugWriter, "Access name: %s, mode: %d, context: %+v, status: %s\n", name, mode, *c, s)
		}()
	}
	m.unmapContext(c)
	return m.base.Access(name, mode, c)
}

func (m *mapFS) Link(src string, dst string, c *fuse.Context) fuse.Status {
	m.unmapContext(c)
	return m.base.Link(src, dst, c)
}

func (m *mapFS) Mkdir(name string, mode uint32, c *fuse.Context) fuse.Status {
	m.unmapContext(c)
	return m.base.Mkdir(name, mode, c)
}

func (m *mapFS) Mknod(name string, mode uint32, dev uint32, c *fuse.Context) fuse.Status {
	m.unmapContext(c)
	return m.base.Mknod(name, mode, dev, c)
}

func (m *mapFS) Rename(oldName string, newName string, c *fuse.Context) fuse.Status {
	m.unmapContext(c)
	return m.base.Rename(oldName, newName, c)
}

func (m *mapFS) Rmdir(name string, c *fuse.Context) fuse.Status {
	m.unmapContext(c)
	return m.base.Rmdir(name, c)
}

func (m *mapFS) Unlink(name string, c *fuse.Context) fuse.Status {
	m.unmapContext(c)
	return m.base.Unlink(name, c)
}

func (m *mapFS) GetXAttr(name string, attr string, c *fuse.Context) (data []byte, status fuse.Status) {
	m.unmapContext(c)
	return m.base.GetXAttr(name, attr, c)
}

func (m *mapFS) ListXAttr(name string, c *fuse.Context) ([]string, fuse.Status) {
	m.unmapContext(c)
	return m.base.ListXAttr(name, c)
}

func (m *mapFS) RemoveXAttr(name string, attr string, c *fuse.Context) fuse.Status {
	m.unmapContext(c)
	return m.base.RemoveXAttr(name, attr, c)
}

func (m *mapFS) SetXAttr(name string, attr string, data []byte, flags int, c *fuse.Context) fuse.Status {
	m.unmapContext(c)
	return m.base.SetXAttr(name, attr, data, flags, c)
}

func (m *mapFS) OnMount(nodeFs *pathfs.PathNodeFs) {
	m.base.OnMount(nodeFs)
}

func (m *mapFS) Create(name string, flags uint32, mode uint32, c *fuse.Context) (nodefs.File, fuse.Status) {
	m.unmapContext(c)
	f, status := m.base.Create(name, flags, mode, c)
	return &mappedFile{fs: m, File: f}, status
}

func (m *mapFS) OnUnmount() {
	m.base.OnUnmount()
}

func (m *mapFS) Open(name string, flags uint32, c *fuse.Context) (nodefs.File, fuse.Status) {
	m.unmapContext(c)
	f, status := m.base.Open(name, flags, c)
	return f, status
}

func (m *mapFS) OpenDir(name string, c *fuse.Context) (entries []fuse.DirEntry, status fuse.Status) {
	if m.debug {
		defer func() {
			fmt.Fprintf(m.debugWriter, "OpenDir for %d: %+v\n", c.Owner, entries)
		}()
	}
	m.unmapContext(c)
	return m.base.OpenDir(name, c)
}

func (m *mapFS) Symlink(value string, linkName string, c *fuse.Context) fuse.Status {
	m.unmapContext(c)
	return m.base.Symlink(value, linkName, c)
}

func (m *mapFS) Readlink(name string, c *fuse.Context) (string, fuse.Status) {
	m.unmapContext(c)
	return m.base.Readlink(name, c)
}

func (m *mapFS) StatFs(name string) *fuse.StatfsOut {
	return m.base.StatFs(name)
}

func (m *mapFS) mapAttr(a *fuse.Attr) {
	uid, gid, err := m.m.ToContainer(idFromOwner(&a.Owner))
	if err != nil {
		if m.debug {
			fmt.Fprintf(m.debugWriter, "no mapping for host attr owner %d:%d\n", a.Owner.Uid, a.Owner.Gid)
		}
		return
	}
	if m.debug {
		fmt.Fprintf(m.debugWriter, "mapping host attr owner %d:%d to container %d:%d\n", a.Owner.Uid, a.Owner.Gid, uid, gid)
	}
	a.Owner.Uid = uint32(uid)
	a.Owner.Gid = uint32(gid)
}

func (m *mapFS) unmapContext(c *fuse.Context) {
	var caller string
	if m.debug {
		_, file, line, _ := runtime.Caller(1)
		caller = fmt.Sprintf("%s#%d", file, line)
	}

	id, err := m.m.ToHost(idFromOwner(&c.Owner))
	if err != nil {
		if m.debug {
			fmt.Fprintf(m.debugWriter, "no mapping for user context %d:%d, caller: %s\n", c.Owner.Uid, c.Owner.Gid, caller)
		}
		return
	}
	if m.debug {
		fmt.Fprintf(m.debugWriter, "mapping user context %d:%d to container context %d:%d, caller: %s\n", c.Owner.Uid, c.Owner.Gid, id.UID, id.GID, caller)
	}
	c.Owner.Uid = uint32(id.UID)
	c.Owner.Gid = uint32(id.GID)
}

func idFromOwner(o *fuse.Owner) idtools.Identity {
	return idtools.Identity{UID: int(o.Uid), GID: int(o.Gid)}
}
