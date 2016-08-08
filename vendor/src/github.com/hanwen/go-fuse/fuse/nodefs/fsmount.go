package nodefs

import (
	"log"
	"sync"
	"unsafe"

	"github.com/hanwen/go-fuse/fuse"
)

// openedFile stores either an open dir or an open file.
type openedFile struct {
	handled

	WithFlags

	dir *connectorDir
}

type fileSystemMount struct {
	// Node that we were mounted on.
	mountInode *Inode

	// Parent to the mountInode.
	parentInode *Inode

	// Options for the mount.
	options *Options

	// Protects the "children" and "parents" hashmaps of the inodes
	// within the mount.
	// treeLock should be acquired before openFilesLock.
	//
	// If multiple treeLocks must be acquired, the treeLocks
	// closer to the root must be acquired first.
	treeLock sync.RWMutex

	// Manage filehandles of open files.
	openFiles handleMap

	Debug bool

	connector *FileSystemConnector
}

// Must called with lock for parent held.
func (m *fileSystemMount) mountName() string {
	for k, v := range m.parentInode.children {
		if m.mountInode == v {
			return k
		}
	}
	panic("not found")
}

func (m *fileSystemMount) setOwner(attr *fuse.Attr) {
	if m.options.Owner != nil {
		attr.Owner = *(*fuse.Owner)(m.options.Owner)
	}
}

func (m *fileSystemMount) fillEntry(out *fuse.EntryOut) {
	splitDuration(m.options.EntryTimeout, &out.EntryValid, &out.EntryValidNsec)
	splitDuration(m.options.AttrTimeout, &out.AttrValid, &out.AttrValidNsec)
	m.setOwner(&out.Attr)
	if out.Mode&fuse.S_IFDIR == 0 && out.Nlink == 0 {
		out.Nlink = 1
	}
}

func (m *fileSystemMount) fillAttr(out *fuse.AttrOut, nodeId uint64) {
	splitDuration(m.options.AttrTimeout, &out.AttrValid, &out.AttrValidNsec)
	m.setOwner(&out.Attr)
	if out.Ino == 0 {
		out.Ino = nodeId
	}
}

func (m *fileSystemMount) getOpenedFile(h uint64) *openedFile {
	var b *openedFile
	if h != 0 {
		b = (*openedFile)(unsafe.Pointer(m.openFiles.Decode(h)))
	}

	if b != nil && m.connector.debug && b.WithFlags.Description != "" {
		log.Printf("File %d = %q", h, b.WithFlags.Description)
	}
	return b
}

func (m *fileSystemMount) unregisterFileHandle(handle uint64, node *Inode) *openedFile {
	_, obj := m.openFiles.Forget(handle, 1)
	opened := (*openedFile)(unsafe.Pointer(obj))
	node.openFilesMutex.Lock()
	idx := -1
	for i, v := range node.openFiles {
		if v == opened {
			idx = i
			break
		}
	}

	l := len(node.openFiles)
	if idx == l-1 {
		node.openFiles[idx] = nil
	} else {
		node.openFiles[idx] = node.openFiles[l-1]
	}
	node.openFiles = node.openFiles[:l-1]
	node.openFilesMutex.Unlock()

	return opened
}

func (m *fileSystemMount) registerFileHandle(node *Inode, dir *connectorDir, f File, flags uint32) (uint64, *openedFile) {
	node.openFilesMutex.Lock()
	b := &openedFile{
		dir: dir,
		WithFlags: WithFlags{
			File:      f,
			OpenFlags: flags,
		},
	}

	for {
		withFlags, ok := f.(*WithFlags)
		if !ok {
			break
		}

		b.WithFlags.File = withFlags.File
		b.WithFlags.FuseFlags |= withFlags.FuseFlags
		b.WithFlags.Description += withFlags.Description
		f = withFlags.File
	}

	if b.WithFlags.File != nil {
		b.WithFlags.File.SetInode(node)
	}
	node.openFiles = append(node.openFiles, b)
	handle, _ := m.openFiles.Register(&b.handled)
	node.openFilesMutex.Unlock()
	return handle, b
}

// Creates a return entry for a non-existent path.
func (m *fileSystemMount) negativeEntry(out *fuse.EntryOut) bool {
	if m.options.NegativeTimeout > 0.0 {
		out.NodeId = 0
		splitDuration(m.options.NegativeTimeout, &out.EntryValid, &out.EntryValidNsec)
		return true
	}
	return false
}
