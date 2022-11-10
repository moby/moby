package remotecontext // import "github.com/docker/docker/builder/remotecontext"

import (
	"os"
	"path/filepath"
	"sync"

	iradix "github.com/hashicorp/go-immutable-radix"
	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/tonistiigi/fsutil"
)

type hashed interface {
	Digest() digest.Digest
}

// CachableSource is a source that contains cache records for its contents
type CachableSource struct {
	mu   sync.Mutex
	root string
	tree *iradix.Tree
	txn  *iradix.Txn
}

// NewCachableSource creates new CachableSource
func NewCachableSource(root string) *CachableSource {
	ts := &CachableSource{
		tree: iradix.New(),
		root: root,
	}
	return ts
}

// MarshalBinary marshals current cache information to a byte array
func (cs *CachableSource) MarshalBinary() ([]byte, error) {
	b := TarsumBackup{Hashes: make(map[string]string)}
	root := cs.getRoot()
	root.Walk(func(k []byte, v interface{}) bool {
		b.Hashes[string(k)] = v.(*fileInfo).sum
		return false
	})
	return b.Marshal()
}

// UnmarshalBinary decodes cache information for presented byte array
func (cs *CachableSource) UnmarshalBinary(data []byte) error {
	var b TarsumBackup
	if err := b.Unmarshal(data); err != nil {
		return err
	}
	txn := iradix.New().Txn()
	for p, v := range b.Hashes {
		txn.Insert([]byte(p), &fileInfo{sum: v})
	}
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.tree = txn.Commit()
	return nil
}

// Scan rescans the cache information from the file system
func (cs *CachableSource) Scan() error {
	lc, err := NewLazySource(cs.root)
	if err != nil {
		return err
	}
	txn := iradix.New().Txn()
	err = filepath.WalkDir(cs.root, func(path string, _ os.DirEntry, err error) error {
		if err != nil {
			return errors.Wrapf(err, "failed to walk %s", path)
		}
		rel, err := Rel(cs.root, path)
		if err != nil {
			return err
		}
		h, err := lc.Hash(rel)
		if err != nil {
			return err
		}
		txn.Insert([]byte(rel), &fileInfo{sum: h})
		return nil
	})
	if err != nil {
		return err
	}
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.tree = txn.Commit()
	return nil
}

// HandleChange notifies the source about a modification operation
func (cs *CachableSource) HandleChange(kind fsutil.ChangeKind, p string, fi os.FileInfo, err error) (retErr error) {
	cs.mu.Lock()
	if cs.txn == nil {
		cs.txn = cs.tree.Txn()
	}
	if kind == fsutil.ChangeKindDelete {
		cs.txn.Delete([]byte(p))
		cs.mu.Unlock()
		return
	}

	h, ok := fi.(hashed)
	if !ok {
		cs.mu.Unlock()
		return errors.Errorf("invalid fileinfo: %s", p)
	}

	hfi := &fileInfo{
		sum: h.Digest().Encoded(),
	}
	cs.txn.Insert([]byte(p), hfi)
	cs.mu.Unlock()
	return nil
}

func (cs *CachableSource) getRoot() *iradix.Node {
	cs.mu.Lock()
	if cs.txn != nil {
		cs.tree = cs.txn.Commit()
		cs.txn = nil
	}
	t := cs.tree
	cs.mu.Unlock()
	return t.Root()
}

// Close closes the source
func (cs *CachableSource) Close() error {
	return nil
}

// Hash returns a hash for a single file in the source
func (cs *CachableSource) Hash(path string) (string, error) {
	n := cs.getRoot()
	// TODO: check this for symlinks
	v, ok := n.Get([]byte(path))
	if !ok {
		return path, nil
	}
	return v.(*fileInfo).sum, nil
}

// Root returns a root directory for the source
func (cs *CachableSource) Root() string {
	return cs.root
}

type fileInfo struct {
	sum string
}

func (fi *fileInfo) Hash() string {
	return fi.sum
}
