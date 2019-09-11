package idmapfs

import (
	"github.com/cpuguy83/idmapfs/idtools"
	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
)

type mappedFile struct {
	fs *mapFS
	nodefs.File
}

func (f *mappedFile) Chown(uid uint32, gid uint32) fuse.Status {
	id, err := f.fs.m.ToHost(idtools.Identity{UID: int(uid), GID: int(gid)})
	if err != nil {
		return f.File.Chown(uid, gid)
	}
	return f.File.Chown(uint32(id.UID), uint32(id.GID))
}
