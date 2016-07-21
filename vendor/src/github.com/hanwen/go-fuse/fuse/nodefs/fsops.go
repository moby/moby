package nodefs

// This file contains FileSystemConnector's implementation of
// RawFileSystem

import (
	"bytes"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/hanwen/go-fuse/fuse"
)

// Returns the RawFileSystem so it can be mounted.
func (c *FileSystemConnector) RawFS() fuse.RawFileSystem {
	return (*rawBridge)(c)
}

type rawBridge FileSystemConnector

func (c *rawBridge) Fsync(input *fuse.FsyncIn) fuse.Status {
	node := c.toInode(input.NodeId)
	opened := node.mount.getOpenedFile(input.Fh)

	if opened != nil {
		return opened.WithFlags.File.Fsync(int(input.FsyncFlags))
	}

	return fuse.ENOSYS
}

func (c *rawBridge) SetDebug(debug bool) {
	c.fsConn().SetDebug(debug)
}

func (c *rawBridge) FsyncDir(input *fuse.FsyncIn) fuse.Status {
	return fuse.ENOSYS
}

func (c *rawBridge) fsConn() *FileSystemConnector {
	return (*FileSystemConnector)(c)
}

func (c *rawBridge) String() string {
	if c.rootNode == nil || c.rootNode.mount == nil {
		return "go-fuse:unmounted"
	}

	name := fmt.Sprintf("%T", c.rootNode.Node())
	name = strings.TrimLeft(name, "*")
	return name
}

func (c *rawBridge) Init(s *fuse.Server) {
	c.server = s
	c.rootNode.Node().OnMount((*FileSystemConnector)(c))
}

func (c *FileSystemConnector) lookupMountUpdate(out *fuse.Attr, mount *fileSystemMount) (node *Inode, code fuse.Status) {
	code = mount.mountInode.Node().GetAttr(out, nil, nil)
	if !code.Ok() {
		log.Println("Root getattr should not return error", code)
		out.Mode = fuse.S_IFDIR | 0755
		return mount.mountInode, fuse.OK
	}

	return mount.mountInode, fuse.OK
}

// internalLookup executes a lookup without affecting NodeId reference counts.
func (c *FileSystemConnector) internalLookup(out *fuse.Attr, parent *Inode, name string, header *fuse.InHeader) (node *Inode, code fuse.Status) {

	// We may already know the child because it was created using Create or Mkdir,
	// from an earlier lookup, or because the nodes were created in advance
	// (in-memory filesystems).
	child := parent.GetChild(name)

	if child != nil && child.mountPoint != nil {
		return c.lookupMountUpdate(out, child.mountPoint)
	}

	if child != nil {
		parent = nil
	}
	if child != nil {
		code = child.fsInode.GetAttr(out, nil, &header.Context)
	} else {
		child, code = parent.fsInode.Lookup(out, name, &header.Context)
	}

	return child, code
}

func (c *rawBridge) Lookup(header *fuse.InHeader, name string, out *fuse.EntryOut) (code fuse.Status) {
	parent := c.toInode(header.NodeId)
	if !parent.IsDir() {
		log.Printf("Lookup %q called on non-Directory node %d", name, header.NodeId)
		return fuse.ENOTDIR
	}
	outAttr := (*fuse.Attr)(&out.Attr)
	child, code := c.fsConn().internalLookup(outAttr, parent, name, header)
	if code == fuse.ENOENT && parent.mount.negativeEntry(out) {
		return fuse.OK
	}
	if !code.Ok() {
		return code
	}
	if child == nil {
		log.Println("Lookup returned fuse.OK with nil child", name)
	}

	child.mount.fillEntry(out)
	out.NodeId, out.Generation = c.fsConn().lookupUpdate(child)
	if out.Ino == 0 {
		out.Ino = out.NodeId
	}

	return fuse.OK
}

func (c *rawBridge) Forget(nodeID, nlookup uint64) {
	c.fsConn().forgetUpdate(nodeID, int(nlookup))
}

func (c *rawBridge) GetAttr(input *fuse.GetAttrIn, out *fuse.AttrOut) (code fuse.Status) {
	node := c.toInode(input.NodeId)

	var f File
	if input.Flags()&fuse.FUSE_GETATTR_FH != 0 {
		if opened := node.mount.getOpenedFile(input.Fh()); opened != nil {
			f = opened.WithFlags.File
		}
	}

	dest := (*fuse.Attr)(&out.Attr)
	code = node.fsInode.GetAttr(dest, f, &input.Context)
	if !code.Ok() {
		return code
	}

	if out.Nlink == 0 {
		// With Nlink == 0, newer kernels will refuse link
		// operations.
		out.Nlink = 1
	}

	node.mount.fillAttr(out, input.NodeId)
	return fuse.OK
}

func (c *rawBridge) OpenDir(input *fuse.OpenIn, out *fuse.OpenOut) (code fuse.Status) {
	node := c.toInode(input.NodeId)
	stream, err := node.fsInode.OpenDir(&input.Context)
	if err != fuse.OK {
		return err
	}
	stream = append(stream, node.getMountDirEntries()...)
	de := &connectorDir{
		node: node.Node(),
		stream: append(stream,
			fuse.DirEntry{Mode: fuse.S_IFDIR, Name: "."},
			fuse.DirEntry{Mode: fuse.S_IFDIR, Name: ".."}),
		rawFS: c,
	}
	h, opened := node.mount.registerFileHandle(node, de, nil, input.Flags)
	out.OpenFlags = opened.FuseFlags
	out.Fh = h
	return fuse.OK
}

func (c *rawBridge) ReadDir(input *fuse.ReadIn, out *fuse.DirEntryList) fuse.Status {
	node := c.toInode(input.NodeId)
	opened := node.mount.getOpenedFile(input.Fh)
	return opened.dir.ReadDir(input, out)
}

func (c *rawBridge) ReadDirPlus(input *fuse.ReadIn, out *fuse.DirEntryList) fuse.Status {
	node := c.toInode(input.NodeId)
	opened := node.mount.getOpenedFile(input.Fh)
	return opened.dir.ReadDirPlus(input, out)
}

func (c *rawBridge) Open(input *fuse.OpenIn, out *fuse.OpenOut) (status fuse.Status) {
	node := c.toInode(input.NodeId)
	f, code := node.fsInode.Open(input.Flags, &input.Context)
	if !code.Ok() || f == nil {
		return code
	}
	h, opened := node.mount.registerFileHandle(node, nil, f, input.Flags)
	out.OpenFlags = opened.FuseFlags
	out.Fh = h
	return fuse.OK
}

func (c *rawBridge) SetAttr(input *fuse.SetAttrIn, out *fuse.AttrOut) (code fuse.Status) {
	node := c.toInode(input.NodeId)

	var f File
	if input.Valid&fuse.FATTR_FH != 0 {
		opened := node.mount.getOpenedFile(input.Fh)
		f = opened.WithFlags.File
	}

	if code.Ok() && input.Valid&fuse.FATTR_MODE != 0 {
		permissions := uint32(07777) & input.Mode
		code = node.fsInode.Chmod(f, permissions, &input.Context)
	}
	if code.Ok() && (input.Valid&(fuse.FATTR_UID|fuse.FATTR_GID) != 0) {
		var uid uint32 = ^uint32(0) // means "do not change" in chown(2)
		var gid uint32 = ^uint32(0)
		if input.Valid&fuse.FATTR_UID != 0 {
			uid = input.Uid
		}
		if input.Valid&fuse.FATTR_GID != 0 {
			gid = input.Gid
		}
		code = node.fsInode.Chown(f, uid, gid, &input.Context)
	}
	if code.Ok() && input.Valid&fuse.FATTR_SIZE != 0 {
		code = node.fsInode.Truncate(f, input.Size, &input.Context)
	}
	if code.Ok() && (input.Valid&(fuse.FATTR_ATIME|fuse.FATTR_MTIME|fuse.FATTR_ATIME_NOW|fuse.FATTR_MTIME_NOW) != 0) {
		now := time.Now()
		var atime *time.Time
		var mtime *time.Time

		if input.Valid&fuse.FATTR_ATIME != 0 {
			if input.Valid&fuse.FATTR_ATIME_NOW != 0 {
				atime = &now
			} else {
				t := time.Unix(int64(input.Atime), int64(input.Atimensec))
				atime = &t
			}
		}

		if input.Valid&fuse.FATTR_MTIME != 0 {
			if input.Valid&fuse.FATTR_MTIME_NOW != 0 {
				mtime = &now
			} else {
				t := time.Unix(int64(input.Mtime), int64(input.Mtimensec))
				mtime = &t
			}
		}

		code = node.fsInode.Utimens(f, atime, mtime, &input.Context)
	}

	if !code.Ok() {
		return code
	}

	// Must call GetAttr(); the filesystem may override some of
	// the changes we effect here.
	attr := (*fuse.Attr)(&out.Attr)
	code = node.fsInode.GetAttr(attr, nil, &input.Context)
	if code.Ok() {
		node.mount.fillAttr(out, input.NodeId)
	}
	return code
}

func (c *rawBridge) Fallocate(input *fuse.FallocateIn) (code fuse.Status) {
	n := c.toInode(input.NodeId)
	opened := n.mount.getOpenedFile(input.Fh)

	return n.fsInode.Fallocate(opened, input.Offset, input.Length, input.Mode, &input.Context)
}

func (c *rawBridge) Readlink(header *fuse.InHeader) (out []byte, code fuse.Status) {
	n := c.toInode(header.NodeId)
	return n.fsInode.Readlink(&header.Context)
}

func (c *rawBridge) Mknod(input *fuse.MknodIn, name string, out *fuse.EntryOut) (code fuse.Status) {
	parent := c.toInode(input.NodeId)

	child, code := parent.fsInode.Mknod(name, input.Mode, uint32(input.Rdev), &input.Context)
	if code.Ok() {
		c.childLookup(out, child, &input.Context)
		code = child.fsInode.GetAttr((*fuse.Attr)(&out.Attr), nil, &input.Context)
	}
	return code
}

func (c *rawBridge) Mkdir(input *fuse.MkdirIn, name string, out *fuse.EntryOut) (code fuse.Status) {
	parent := c.toInode(input.NodeId)

	child, code := parent.fsInode.Mkdir(name, input.Mode, &input.Context)
	if code.Ok() {
		c.childLookup(out, child, &input.Context)
		code = child.fsInode.GetAttr((*fuse.Attr)(&out.Attr), nil, &input.Context)
	}
	return code
}

func (c *rawBridge) Unlink(header *fuse.InHeader, name string) (code fuse.Status) {
	parent := c.toInode(header.NodeId)
	return parent.fsInode.Unlink(name, &header.Context)
}

func (c *rawBridge) Rmdir(header *fuse.InHeader, name string) (code fuse.Status) {
	parent := c.toInode(header.NodeId)
	return parent.fsInode.Rmdir(name, &header.Context)
}

func (c *rawBridge) Symlink(header *fuse.InHeader, pointedTo string, linkName string, out *fuse.EntryOut) (code fuse.Status) {
	parent := c.toInode(header.NodeId)

	child, code := parent.fsInode.Symlink(linkName, pointedTo, &header.Context)
	if code.Ok() {
		c.childLookup(out, child, &header.Context)
		code = child.fsInode.GetAttr((*fuse.Attr)(&out.Attr), nil, &header.Context)
	}
	return code
}

func (c *rawBridge) Rename(input *fuse.RenameIn, oldName string, newName string) (code fuse.Status) {
	oldParent := c.toInode(input.NodeId)

	child := oldParent.GetChild(oldName)
	if child == nil {
		return fuse.ENOENT
	}
	if child.mountPoint != nil {
		return fuse.EBUSY
	}

	newParent := c.toInode(input.Newdir)
	if oldParent.mount != newParent.mount {
		return fuse.EXDEV
	}

	return oldParent.fsInode.Rename(oldName, newParent.fsInode, newName, &input.Context)
}

func (c *rawBridge) Link(input *fuse.LinkIn, name string, out *fuse.EntryOut) (code fuse.Status) {
	existing := c.toInode(input.Oldnodeid)
	parent := c.toInode(input.NodeId)

	if existing.mount != parent.mount {
		return fuse.EXDEV
	}

	child, code := parent.fsInode.Link(name, existing.fsInode, &input.Context)
	if code.Ok() {
		c.childLookup(out, child, &input.Context)
		code = child.fsInode.GetAttr((*fuse.Attr)(&out.Attr), nil, &input.Context)
	}

	return code
}

func (c *rawBridge) Access(input *fuse.AccessIn) (code fuse.Status) {
	n := c.toInode(input.NodeId)
	return n.fsInode.Access(input.Mask, &input.Context)
}

func (c *rawBridge) Create(input *fuse.CreateIn, name string, out *fuse.CreateOut) (code fuse.Status) {
	parent := c.toInode(input.NodeId)
	f, child, code := parent.fsInode.Create(name, uint32(input.Flags), input.Mode, &input.Context)
	if !code.Ok() {
		return code
	}

	c.childLookup(&out.EntryOut, child, &input.Context)
	handle, opened := parent.mount.registerFileHandle(child, nil, f, input.Flags)

	out.OpenOut.OpenFlags = opened.FuseFlags
	out.OpenOut.Fh = handle
	return code
}

func (c *rawBridge) Release(input *fuse.ReleaseIn) {
	if input.Fh != 0 {
		node := c.toInode(input.NodeId)
		opened := node.mount.unregisterFileHandle(input.Fh, node)
		opened.WithFlags.File.Release()
	}
}

func (c *rawBridge) ReleaseDir(input *fuse.ReleaseIn) {
	if input.Fh != 0 {
		node := c.toInode(input.NodeId)
		node.mount.unregisterFileHandle(input.Fh, node)
	}
}

func (c *rawBridge) GetXAttrSize(header *fuse.InHeader, attribute string) (sz int, code fuse.Status) {
	node := c.toInode(header.NodeId)
	data, errno := node.fsInode.GetXAttr(attribute, &header.Context)
	return len(data), errno
}

func (c *rawBridge) GetXAttrData(header *fuse.InHeader, attribute string) (data []byte, code fuse.Status) {
	node := c.toInode(header.NodeId)
	return node.fsInode.GetXAttr(attribute, &header.Context)
}

func (c *rawBridge) RemoveXAttr(header *fuse.InHeader, attr string) fuse.Status {
	node := c.toInode(header.NodeId)
	return node.fsInode.RemoveXAttr(attr, &header.Context)
}

func (c *rawBridge) SetXAttr(input *fuse.SetXAttrIn, attr string, data []byte) fuse.Status {
	node := c.toInode(input.NodeId)
	return node.fsInode.SetXAttr(attr, data, int(input.Flags), &input.Context)
}

func (c *rawBridge) ListXAttr(header *fuse.InHeader) (data []byte, code fuse.Status) {
	node := c.toInode(header.NodeId)
	attrs, code := node.fsInode.ListXAttr(&header.Context)
	if code != fuse.OK {
		return nil, code
	}

	b := bytes.NewBuffer([]byte{})
	for _, v := range attrs {
		b.Write([]byte(v))
		b.WriteByte(0)
	}

	return b.Bytes(), code
}

////////////////
// files.

func (c *rawBridge) Write(input *fuse.WriteIn, data []byte) (written uint32, code fuse.Status) {
	node := c.toInode(input.NodeId)
	opened := node.mount.getOpenedFile(input.Fh)

	var f File
	if opened != nil {
		f = opened.WithFlags.File
	}

	return node.Node().Write(f, data, int64(input.Offset), &input.Context)
}

func (c *rawBridge) Read(input *fuse.ReadIn, buf []byte) (fuse.ReadResult, fuse.Status) {
	node := c.toInode(input.NodeId)
	opened := node.mount.getOpenedFile(input.Fh)

	var f File
	if opened != nil {
		f = opened.WithFlags.File
	}

	return node.Node().Read(f, buf, int64(input.Offset), &input.Context)
}

func (c *rawBridge) StatFs(header *fuse.InHeader, out *fuse.StatfsOut) fuse.Status {
	node := c.toInode(header.NodeId)
	s := node.Node().StatFs()
	if s == nil {
		return fuse.ENOSYS
	}
	*out = *s
	return fuse.OK
}

func (c *rawBridge) Flush(input *fuse.FlushIn) fuse.Status {
	node := c.toInode(input.NodeId)
	opened := node.mount.getOpenedFile(input.Fh)

	if opened != nil {
		return opened.WithFlags.File.Flush()
	}
	return fuse.OK
}
