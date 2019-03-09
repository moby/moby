// Copyright 2016 the Go-FUSE Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

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
		attr.Owner = *m.options.Owner
	}
}

func (m *fileSystemMount) fillEntry(out *fuse.EntryOut) {
	out.SetEntryTimeout(m.options.EntryTimeout)
	out.SetAttrTimeout(m.options.AttrTimeout)
	m.setOwner(&out.Attr)
	if out.Mode&fuse.S_IFDIR == 0 && out.Nlink == 0 {
		out.Nlink = 1
	}
}

func (m *fileSystemMount) fillAttr(out *fuse.AttrOut, nodeId uint64) {
	out.SetTimeout(m.options.AttrTimeout)
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

// registerFileHandle registers f or dir to have a handle.
//
// The handle is then used as file-handle in communications with kernel.
//
// If dir != nil the handle is registered for OpenDir and the inner file (see
// below) must be nil. If dir = nil the handle is registered for regular open &
// friends.
//
// f can be nil, or a WithFlags that leads to File=nil. For !OpenDir, if that
// is the case, returned handle will be 0 to indicate a handleless open, and
// the filesystem operations on the opened file will be routed to be served by
// the node.
//
// other arguments:
//
//	node  - Inode for which f or dir were opened,
//	flags - file open flags, like O_RDWR.
func (m *fileSystemMount) registerFileHandle(node *Inode, dir *connectorDir, f File, flags uint32) (handle uint64, opened *openedFile) {
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

	// don't allow both dir and file
	if dir != nil && b.WithFlags.File != nil {
		panic("registerFileHandle: both dir and file are set.")
	}

	if b.WithFlags.File == nil && dir == nil {
		// it was just WithFlags{...}, but the file itself is nil
		return 0, b
	}

	if b.WithFlags.File != nil {
		b.WithFlags.File.SetInode(node)
	}

	node.openFilesMutex.Lock()
	node.openFiles = append(node.openFiles, b)
	handle, _ = m.openFiles.Register(&b.handled)
	node.openFilesMutex.Unlock()
	return handle, b
}

// Creates a return entry for a non-existent path.
func (m *fileSystemMount) negativeEntry(out *fuse.EntryOut) bool {
	if m.options.NegativeTimeout > 0.0 {
		out.NodeId = 0
		out.SetEntryTimeout(m.options.NegativeTimeout)
		return true
	}
	return false
}
