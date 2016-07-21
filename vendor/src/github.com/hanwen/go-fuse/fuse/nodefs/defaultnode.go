package nodefs

import (
	"time"

	"github.com/hanwen/go-fuse/fuse"
)

// NewDefaultNode returns an implementation of Node that returns
// ENOSYS for all operations.
func NewDefaultNode() Node {
	return &defaultNode{}
}

type defaultNode struct {
	inode *Inode
}

func (fs *defaultNode) OnUnmount() {
}

func (fs *defaultNode) OnMount(conn *FileSystemConnector) {
}

func (n *defaultNode) StatFs() *fuse.StatfsOut {
	return nil
}

func (n *defaultNode) SetInode(node *Inode) {
	n.inode = node
}

func (n *defaultNode) Deletable() bool {
	return true
}

func (n *defaultNode) Inode() *Inode {
	return n.inode
}

func (n *defaultNode) OnForget() {
}

func (n *defaultNode) Lookup(out *fuse.Attr, name string, context *fuse.Context) (node *Inode, code fuse.Status) {
	return nil, fuse.ENOENT
}

func (n *defaultNode) Access(mode uint32, context *fuse.Context) (code fuse.Status) {
	return fuse.ENOSYS
}

func (n *defaultNode) Readlink(c *fuse.Context) ([]byte, fuse.Status) {
	return nil, fuse.ENOSYS
}

func (n *defaultNode) Mknod(name string, mode uint32, dev uint32, context *fuse.Context) (newNode *Inode, code fuse.Status) {
	return nil, fuse.ENOSYS
}
func (n *defaultNode) Mkdir(name string, mode uint32, context *fuse.Context) (newNode *Inode, code fuse.Status) {
	return nil, fuse.ENOSYS
}
func (n *defaultNode) Unlink(name string, context *fuse.Context) (code fuse.Status) {
	return fuse.ENOSYS
}
func (n *defaultNode) Rmdir(name string, context *fuse.Context) (code fuse.Status) {
	return fuse.ENOSYS
}
func (n *defaultNode) Symlink(name string, content string, context *fuse.Context) (newNode *Inode, code fuse.Status) {
	return nil, fuse.ENOSYS
}

func (n *defaultNode) Rename(oldName string, newParent Node, newName string, context *fuse.Context) (code fuse.Status) {
	return fuse.ENOSYS
}

func (n *defaultNode) Link(name string, existing Node, context *fuse.Context) (newNode *Inode, code fuse.Status) {
	return nil, fuse.ENOSYS
}

func (n *defaultNode) Create(name string, flags uint32, mode uint32, context *fuse.Context) (file File, newNode *Inode, code fuse.Status) {
	return nil, nil, fuse.ENOSYS
}

func (n *defaultNode) Open(flags uint32, context *fuse.Context) (file File, code fuse.Status) {
	return nil, fuse.ENOSYS
}

func (n *defaultNode) Flush(file File, openFlags uint32, context *fuse.Context) (code fuse.Status) {
	return fuse.ENOSYS
}

func (n *defaultNode) OpenDir(context *fuse.Context) ([]fuse.DirEntry, fuse.Status) {
	ch := n.Inode().Children()
	s := make([]fuse.DirEntry, 0, len(ch))
	for name, child := range ch {
		if child.mountPoint != nil {
			continue
		}
		var a fuse.Attr
		code := child.Node().GetAttr(&a, nil, context)
		if code.Ok() {
			s = append(s, fuse.DirEntry{Name: name, Mode: a.Mode})
		}
	}
	return s, fuse.OK
}

func (n *defaultNode) GetXAttr(attribute string, context *fuse.Context) (data []byte, code fuse.Status) {
	return nil, fuse.ENODATA
}

func (n *defaultNode) RemoveXAttr(attr string, context *fuse.Context) fuse.Status {
	return fuse.ENOSYS
}

func (n *defaultNode) SetXAttr(attr string, data []byte, flags int, context *fuse.Context) fuse.Status {
	return fuse.ENOSYS
}

func (n *defaultNode) ListXAttr(context *fuse.Context) (attrs []string, code fuse.Status) {
	return nil, fuse.ENOSYS
}

func (n *defaultNode) GetAttr(out *fuse.Attr, file File, context *fuse.Context) (code fuse.Status) {
	if n.Inode().IsDir() {
		out.Mode = fuse.S_IFDIR | 0755
	} else {
		out.Mode = fuse.S_IFREG | 0644
	}
	return fuse.OK
}

func (n *defaultNode) Chmod(file File, perms uint32, context *fuse.Context) (code fuse.Status) {
	return fuse.ENOSYS
}

func (n *defaultNode) Chown(file File, uid uint32, gid uint32, context *fuse.Context) (code fuse.Status) {
	return fuse.ENOSYS
}

func (n *defaultNode) Truncate(file File, size uint64, context *fuse.Context) (code fuse.Status) {
	return fuse.ENOSYS
}

func (n *defaultNode) Utimens(file File, atime *time.Time, mtime *time.Time, context *fuse.Context) (code fuse.Status) {
	return fuse.ENOSYS
}

func (n *defaultNode) Fallocate(file File, off uint64, size uint64, mode uint32, context *fuse.Context) (code fuse.Status) {
	return fuse.ENOSYS
}

func (n *defaultNode) Read(file File, dest []byte, off int64, context *fuse.Context) (fuse.ReadResult, fuse.Status) {
	if file != nil {
		return file.Read(dest, off)
	}
	return nil, fuse.ENOSYS
}

func (n *defaultNode) Write(file File, data []byte, off int64, context *fuse.Context) (written uint32, code fuse.Status) {
	if file != nil {
		return file.Write(data, off)
	}
	return 0, fuse.ENOSYS
}
