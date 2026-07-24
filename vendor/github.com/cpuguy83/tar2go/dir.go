package tar2go

import (
	"archive/tar"
	"errors"
	"io"
	"io/fs"
	"path"
	"sort"
	"strings"
)

var (
	errNotDir = errors.New("not a directory")
	errIsDir  = errors.New("is a directory")
)

// treeNode is a node in the synthesized directory tree. Directories have a
// non-nil children map (even when empty); files have a nil children map and a
// non-nil idx. hdr is nil for directories that only exist implicitly in the
// archive.
type treeNode struct {
	name     string
	hdr      *tar.Header
	idx      *indexReader
	children map[string]*treeNode
	// entries is the memoized, sorted directory listing. It is built once and
	// only ever written while holding the Index lock.
	entries []fs.DirEntry
}

func (n *treeNode) isDir() bool {
	return n.children != nil
}

func (n *treeNode) info() *fileinfo {
	return &fileinfo{name: n.name, h: n.hdr, dir: n.isDir()}
}

func (n *treeNode) buildDirEntries() []fs.DirEntry {
	names := make([]string, 0, len(n.children))
	for name := range n.children {
		names = append(names, name)
	}
	sort.Strings(names)

	entries := make([]fs.DirEntry, 0, len(names))
	for _, name := range names {
		entries = append(entries, fs.FileInfoToDirEntry(n.children[name].info()))
	}
	return entries
}

// indexAll scans the remaining tar entries to EOF, caching each one. It is safe
// to call repeatedly; the scan is performed at most once.
//
// This function must be called with the lock held.
func (i *Index) indexAll() error {
	if i.complete {
		return nil
	}
	if i.tar == nil {
		i.tar = tar.NewReader(i.rdr)
	}

	for {
		hdr, err := i.tar.Next()
		if err != nil {
			if err == io.EOF {
				i.complete = true
				return nil
			}
			return err
		}

		pos, err := i.rdr.Seek(0, io.SeekCurrent)
		if err != nil {
			return err
		}

		hdrName := filterFSPrefix(hdr.Name)
		if _, ok := i.idx[hdrName]; !ok {
			i.idx[hdrName] = &indexReader{rdr: i.rdr, offset: pos, size: hdr.Size, hdr: hdr}
		}
	}
}

// buildTree constructs (and caches) the directory tree, synthesizing any
// intermediate directories that only exist implicitly in the archive.
//
// This function must be called with the lock held.
func (i *Index) buildTree() error {
	if i.tree != nil {
		return nil
	}
	if err := i.indexAll(); err != nil {
		return err
	}

	root := &treeNode{name: ".", children: map[string]*treeNode{}}
	for key, rdr := range i.idx {
		clean := path.Clean(filterFSPrefix(key))
		if clean == "." || clean == "/" {
			if rdr.hdr != nil {
				root.hdr = rdr.hdr
			}
			continue
		}

		parts := strings.Split(clean, "/")
		cur := root
		for j, part := range parts {
			child, ok := cur.children[part]
			if !ok {
				child = &treeNode{name: part}
				cur.children[part] = child
			}

			if j < len(parts)-1 {
				// Intermediate component: must be a directory.
				if child.children == nil {
					child.children = map[string]*treeNode{}
				}
				cur = child
				continue
			}

			// Leaf component.
			if rdr.hdr != nil && rdr.hdr.Typeflag == tar.TypeDir {
				if child.children == nil {
					child.children = map[string]*treeNode{}
				}
				child.hdr = rdr.hdr
			} else {
				child.hdr = rdr.hdr
				child.idx = rdr
			}
		}
	}

	i.tree = root
	return nil
}

// find resolves a cleaned fs path against the tree. name must already be
// filtered/cleaned by the caller.
func (n *treeNode) find(name string) (*treeNode, error) {
	if name == "." {
		return n, nil
	}

	cur := n
	for _, part := range strings.Split(name, "/") {
		if cur.children == nil {
			return nil, fs.ErrNotExist
		}
		child, ok := cur.children[part]
		if !ok {
			return nil, fs.ErrNotExist
		}
		cur = child
	}
	return cur, nil
}

func (i *Index) lookupWithLock(name string) (*treeNode, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	if err := i.buildTree(); err != nil {
		return nil, err
	}
	return i.tree.find(path.Clean(filterFSPrefix(name)))
}

// dirEntriesLocked returns the memoized directory listing for n, building it on
// first use. The tree is immutable once built, so the listing is stable. Must
// be called with the lock held.
func (i *Index) dirEntriesLocked(n *treeNode) []fs.DirEntry {
	if n.entries == nil {
		n.entries = n.buildDirEntries()
	}
	return n.entries
}

// snapshotDirEntries returns a copy of a directory's memoized listing. Taking
// the lock here lets an open dirFile read entries without holding it.
func (i *Index) snapshotDirEntries(n *treeNode) []fs.DirEntry {
	i.mu.Lock()
	defer i.mu.Unlock()
	return copyDirEntries(i.dirEntriesLocked(n))
}

func copyDirEntries(src []fs.DirEntry) []fs.DirEntry {
	out := make([]fs.DirEntry, len(src))
	copy(out, src)
	return out
}

func (i *Index) readDir(name string) ([]fs.DirEntry, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	if err := i.buildTree(); err != nil {
		return nil, err
	}

	node, err := i.tree.find(path.Clean(filterFSPrefix(name)))
	if err != nil {
		return nil, err
	}
	if !node.isDir() {
		return nil, errNotDir
	}
	return copyDirEntries(i.dirEntriesLocked(node)), nil
}

// dirFile is the fs.File returned when opening a directory. It also implements
// fs.ReadDirFile so the Open-then-ReadDir path works.
type dirFile struct {
	idx     *Index
	node    *treeNode
	name    string
	entries []fs.DirEntry
	offset  int
}

func newDirFile(idx *Index, node *treeNode, name string) *dirFile {
	return &dirFile{idx: idx, node: node, name: name}
}

func (d *dirFile) Stat() (fs.FileInfo, error) {
	return d.node.info(), nil
}

func (d *dirFile) Read([]byte) (int, error) {
	return 0, &fs.PathError{Op: "read", Path: d.name, Err: errIsDir}
}

func (d *dirFile) Close() error {
	return nil
}

func (d *dirFile) ReadDir(n int) ([]fs.DirEntry, error) {
	if d.entries == nil {
		d.entries = d.idx.snapshotDirEntries(d.node)
	}

	if n <= 0 {
		rest := d.entries[d.offset:]
		d.offset = len(d.entries)
		return rest, nil
	}

	if d.offset >= len(d.entries) {
		return nil, io.EOF
	}

	end := d.offset + n
	if end > len(d.entries) {
		end = len(d.entries)
	}
	out := d.entries[d.offset:end]
	d.offset = end
	return out, nil
}
