package fsutil

import (
	"archive/tar"
	"crypto/sha256"
	"fmt"
	"hash"
	"os"
	"path/filepath"
	"sync"

	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/symlink"
	"github.com/docker/docker/pkg/tarsum"
	iradix "github.com/hashicorp/go-immutable-radix"
	"github.com/pkg/errors"
)

type Tarsum struct {
	mu   sync.Mutex
	root string
	tree *iradix.Tree
	txn  *iradix.Txn
}

func NewTarsum(root string) *Tarsum {
	ts := &Tarsum{
		tree: iradix.New(),
		root: root,
	}
	return ts
}

type hashed interface {
	Hash() string
}

func (ts *Tarsum) HandleChange(kind ChangeKind, p string, fi os.FileInfo, err error) (retErr error) {
	ts.mu.Lock()
	if ts.txn == nil {
		ts.txn = ts.tree.Txn()
	}
	if kind == ChangeKindDelete {
		ts.txn.Delete([]byte(p))
		ts.mu.Unlock()
		return
	}

	h, ok := fi.(hashed)
	if !ok {
		ts.mu.Unlock()
		return errors.Errorf("invalid fileinfo: %p", p)
	}

	hfi := &fileInfo{
		FileInfo: fi,
		sum:      h.Hash(),
	}
	ts.txn.Insert([]byte(p), hfi)
	ts.mu.Unlock()
	return nil
}

func (ts *Tarsum) getRoot() *iradix.Node {
	ts.mu.Lock()
	if ts.txn != nil {
		ts.tree = ts.txn.Commit()
		ts.txn = nil
	}
	t := ts.tree
	ts.mu.Unlock()
	return t.Root()
}

func (ts *Tarsum) Close() error {
	return nil
}

func (ts *Tarsum) normalize(path string) (cleanpath, fullpath string, err error) {
	cleanpath = filepath.Clean(string(os.PathSeparator) + path)[1:]
	fullpath, err = symlink.FollowSymlinkInScope(filepath.Join(ts.root, path), ts.root)
	if err != nil {
		return "", "", fmt.Errorf("Forbidden path outside the context: %s (%s)", path, fullpath)
	}
	_, err = os.Lstat(fullpath)
	if err != nil {
		return "", "", convertPathError(err, path)
	}
	return
}

func (c *Tarsum) Hash(path string) (string, error) {
	n := c.getRoot()
	sum := ""
	v, ok := n.Get([]byte(path))
	if !ok {
		sum = path
	} else {
		sum = v.(*fileInfo).sum
	}
	return sum, nil
}

func (c *Tarsum) Root() string {
	return c.root
}

type tarsumHash struct {
	hash.Hash
	h *tar.Header
}

func NewTarsumHash(p string, fi os.FileInfo) (hash.Hash, error) {
	stat, ok := fi.Sys().(*Stat)
	link := ""
	if ok {
		link = stat.Linkname
	}
	if fi.IsDir() {
		p += string(os.PathSeparator)
	}
	h, err := archive.FileInfoHeader(p, fi, link)
	if err != nil {
		return nil, err
	}
	h.Name = p
	if ok {
		h.Uid = int(stat.Uid)
		h.Gid = int(stat.Gid)
		h.Linkname = stat.Linkname
		if stat.Xattrs != nil {
			h.Xattrs = make(map[string]string)
			for k, v := range stat.Xattrs {
				h.Xattrs[k] = string(v)
			}
		}
	}
	tsh := &tarsumHash{h: h, Hash: sha256.New()}
	tsh.Reset()
	return tsh, nil
}

// Reset resets the Hash to its initial state.
func (tsh *tarsumHash) Reset() {
	tsh.Hash.Reset()
	tarsum.WriteV1Header(tsh.h, tsh.Hash)
}

type fileInfo struct {
	os.FileInfo
	sum string
}

func (fi *fileInfo) Hash() string {
	return fi.sum
}

func convertPathError(err error, cleanpath string) error {
	if err, ok := err.(*os.PathError); ok {
		err.Path = cleanpath
		return err
	}
	return err
}
