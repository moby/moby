package contenthash

import (
	"bytes"
	"context"
	"crypto/sha256"
	"io"
	"os"
	"path"
	"path/filepath"
	"sync"

	"github.com/containerd/continuity/fs"
	"github.com/docker/docker/pkg/locker"
	iradix "github.com/hashicorp/go-immutable-radix"
	"github.com/hashicorp/golang-lru/simplelru"
	"github.com/moby/buildkit/cache"
	"github.com/moby/buildkit/cache/metadata"
	"github.com/moby/buildkit/snapshot"
	digest "github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/tonistiigi/fsutil"
)

var errNotFound = errors.Errorf("not found")

var defaultManager *cacheManager
var defaultManagerOnce sync.Once

const keyContentHash = "buildkit.contenthash.v0"

func getDefaultManager() *cacheManager {
	defaultManagerOnce.Do(func() {
		lru, _ := simplelru.NewLRU(20, nil) // error is impossible on positive size
		defaultManager = &cacheManager{lru: lru, locker: locker.New()}
	})
	return defaultManager
}

// Layout in the radix tree: Every path is saved by cleaned absolute unix path.
// Directories have 2 records, one contains digest for directory header, other
// the recursive digest for directory contents. "/dir/" is the record for
// header, "/dir" is for contents. For the root node "" (empty string) is the
// key for root, "/" for the root header

func Checksum(ctx context.Context, ref cache.ImmutableRef, path string) (digest.Digest, error) {
	return getDefaultManager().Checksum(ctx, ref, path)
}

func GetCacheContext(ctx context.Context, md *metadata.StorageItem) (CacheContext, error) {
	return getDefaultManager().GetCacheContext(ctx, md)
}

func SetCacheContext(ctx context.Context, md *metadata.StorageItem, cc CacheContext) error {
	return getDefaultManager().SetCacheContext(ctx, md, cc)
}

type CacheContext interface {
	Checksum(ctx context.Context, ref cache.Mountable, p string) (digest.Digest, error)
	HandleChange(kind fsutil.ChangeKind, p string, fi os.FileInfo, err error) error
}

type Hashed interface {
	Digest() digest.Digest
}

type cacheManager struct {
	locker *locker.Locker
	lru    *simplelru.LRU
	lruMu  sync.Mutex
}

func (cm *cacheManager) Checksum(ctx context.Context, ref cache.ImmutableRef, p string) (digest.Digest, error) {
	cc, err := cm.GetCacheContext(ctx, ensureOriginMetadata(ref.Metadata()))
	if err != nil {
		return "", nil
	}
	return cc.Checksum(ctx, ref, p)
}

func (cm *cacheManager) GetCacheContext(ctx context.Context, md *metadata.StorageItem) (CacheContext, error) {
	cm.locker.Lock(md.ID())
	cm.lruMu.Lock()
	v, ok := cm.lru.Get(md.ID())
	cm.lruMu.Unlock()
	if ok {
		cm.locker.Unlock(md.ID())
		return v.(*cacheContext), nil
	}
	cc, err := newCacheContext(md)
	if err != nil {
		cm.locker.Unlock(md.ID())
		return nil, err
	}
	cm.lruMu.Lock()
	cm.lru.Add(md.ID(), cc)
	cm.lruMu.Unlock()
	cm.locker.Unlock(md.ID())
	return cc, nil
}

func (cm *cacheManager) SetCacheContext(ctx context.Context, md *metadata.StorageItem, cci CacheContext) error {
	cc, ok := cci.(*cacheContext)
	if !ok {
		return errors.Errorf("invalid cachecontext: %T", cc)
	}
	if md.ID() != cc.md.ID() {
		cc = &cacheContext{
			md:       md,
			tree:     cci.(*cacheContext).tree,
			dirtyMap: map[string]struct{}{},
		}
	} else {
		if err := cc.save(); err != nil {
			return err
		}
	}
	cm.lruMu.Lock()
	cm.lru.Add(md.ID(), cc)
	cm.lruMu.Unlock()
	return nil
}

type cacheContext struct {
	mu    sync.RWMutex
	md    *metadata.StorageItem
	tree  *iradix.Tree
	dirty bool // needs to be persisted to disk

	// used in HandleChange
	txn      *iradix.Txn
	node     *iradix.Node
	dirtyMap map[string]struct{}
}

type mount struct {
	mountable cache.Mountable
	mountPath string
	unmount   func() error
}

func (m *mount) mount(ctx context.Context) (string, error) {
	if m.mountPath != "" {
		return m.mountPath, nil
	}
	mounts, err := m.mountable.Mount(ctx, true)
	if err != nil {
		return "", err
	}

	lm := snapshot.LocalMounter(mounts)

	mp, err := lm.Mount()
	if err != nil {
		return "", err
	}

	m.mountPath = mp
	m.unmount = lm.Unmount
	return mp, nil
}

func (m *mount) clean() error {
	if m.mountPath != "" {
		if err := m.unmount(); err != nil {
			return err
		}
		m.mountPath = ""
	}
	return nil
}

func newCacheContext(md *metadata.StorageItem) (*cacheContext, error) {
	cc := &cacheContext{
		md:       md,
		tree:     iradix.New(),
		dirtyMap: map[string]struct{}{},
	}
	if err := cc.load(); err != nil {
		return nil, err
	}
	return cc, nil
}

func (cc *cacheContext) load() error {
	dt, err := cc.md.GetExternal(keyContentHash)
	if err != nil {
		return nil
	}

	var l CacheRecords
	if err := l.Unmarshal(dt); err != nil {
		return err
	}

	txn := cc.tree.Txn()
	for _, p := range l.Paths {
		txn.Insert([]byte(p.Path), p.Record)
	}
	cc.tree = txn.Commit()
	return nil
}

func (cc *cacheContext) save() error {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	if cc.txn != nil {
		cc.commitActiveTransaction()
	}

	var l CacheRecords
	node := cc.tree.Root()
	node.Walk(func(k []byte, v interface{}) bool {
		l.Paths = append(l.Paths, &CacheRecordWithPath{
			Path:   string(k),
			Record: v.(*CacheRecord),
		})
		return false
	})

	dt, err := l.Marshal()
	if err != nil {
		return err
	}

	return cc.md.SetExternal(keyContentHash, dt)
}

// HandleChange notifies the source about a modification operation
func (cc *cacheContext) HandleChange(kind fsutil.ChangeKind, p string, fi os.FileInfo, err error) (retErr error) {
	p = path.Join("/", filepath.ToSlash(p))
	if p == "/" {
		p = ""
	}
	k := convertPathToKey([]byte(p))

	deleteDir := func(cr *CacheRecord) {
		if cr.Type == CacheRecordTypeDir {
			cc.node.WalkPrefix(append(k, 0), func(k []byte, v interface{}) bool {
				cc.txn.Delete(k)
				return false
			})
		}
	}

	cc.mu.Lock()
	defer cc.mu.Unlock()
	if cc.txn == nil {
		cc.txn = cc.tree.Txn()
		cc.node = cc.tree.Root()

		// root is not called by HandleChange. need to fake it
		if _, ok := cc.node.Get([]byte{0}); !ok {
			cc.txn.Insert([]byte{0}, &CacheRecord{
				Type:   CacheRecordTypeDirHeader,
				Digest: digest.FromBytes(nil),
			})
			cc.txn.Insert([]byte(""), &CacheRecord{
				Type: CacheRecordTypeDir,
			})
		}
	}

	if kind == fsutil.ChangeKindDelete {
		v, ok := cc.txn.Delete(k)
		if ok {
			deleteDir(v.(*CacheRecord))
		}
		d := path.Dir(p)
		if d == "/" {
			d = ""
		}
		cc.dirtyMap[d] = struct{}{}
		return
	}

	stat, ok := fi.Sys().(*fsutil.Stat)
	if !ok {
		return errors.Errorf("%s invalid change without stat information", p)
	}

	h, ok := fi.(Hashed)
	if !ok {
		return errors.Errorf("invalid fileinfo: %s", p)
	}

	v, ok := cc.node.Get(k)
	if ok {
		deleteDir(v.(*CacheRecord))
	}

	cr := &CacheRecord{
		Type: CacheRecordTypeFile,
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		cr.Type = CacheRecordTypeSymlink
		cr.Linkname = filepath.ToSlash(stat.Linkname)
	}
	if fi.IsDir() {
		cr.Type = CacheRecordTypeDirHeader
		cr2 := &CacheRecord{
			Type: CacheRecordTypeDir,
		}
		cc.txn.Insert(k, cr2)
		k = append(k, 0)
		p += "/"
	}
	cr.Digest = h.Digest()
	cc.txn.Insert(k, cr)
	d := path.Dir(p)
	if d == "/" {
		d = ""
	}
	cc.dirtyMap[d] = struct{}{}

	return nil
}

func (cc *cacheContext) Checksum(ctx context.Context, mountable cache.Mountable, p string) (digest.Digest, error) {
	m := &mount{mountable: mountable}
	defer m.clean()

	const maxSymlinkLimit = 255
	i := 0
	for {
		if i > maxSymlinkLimit {
			return "", errors.Errorf("too many symlinks: %s", p)
		}
		cr, err := cc.checksumNoFollow(ctx, m, p)
		if err != nil {
			return "", err
		}
		if cr.Type == CacheRecordTypeSymlink {
			link := cr.Linkname
			if !path.IsAbs(cr.Linkname) {
				link = path.Join(path.Dir(p), link)
			}
			i++
			p = link
		} else {
			return cr.Digest, nil
		}
	}
}

func (cc *cacheContext) checksumNoFollow(ctx context.Context, m *mount, p string) (*CacheRecord, error) {
	p = path.Join("/", filepath.ToSlash(p))
	if p == "/" {
		p = ""
	}

	cc.mu.RLock()
	if cc.txn == nil {
		root := cc.tree.Root()
		cc.mu.RUnlock()
		v, ok := root.Get(convertPathToKey([]byte(p)))
		if ok {
			cr := v.(*CacheRecord)
			if cr.Digest != "" {
				return cr, nil
			}
		}
	} else {
		cc.mu.RUnlock()
	}

	cc.mu.Lock()
	defer cc.mu.Unlock()

	if cc.txn != nil {
		cc.commitActiveTransaction()
	}

	defer func() {
		if cc.dirty {
			go cc.save()
			cc.dirty = false
		}
	}()

	return cc.lazyChecksum(ctx, m, p)
}

func (cc *cacheContext) commitActiveTransaction() {
	for d := range cc.dirtyMap {
		addParentToMap(d, cc.dirtyMap)
	}
	for d := range cc.dirtyMap {
		k := convertPathToKey([]byte(d))
		if _, ok := cc.txn.Get(k); ok {
			cc.txn.Insert(k, &CacheRecord{Type: CacheRecordTypeDir})
		}
	}
	cc.tree = cc.txn.Commit()
	cc.node = nil
	cc.dirtyMap = map[string]struct{}{}
	cc.txn = nil
}

func (cc *cacheContext) lazyChecksum(ctx context.Context, m *mount, p string) (*CacheRecord, error) {
	root := cc.tree.Root()
	if cc.needsScan(root, p) {
		if err := cc.scanPath(ctx, m, p); err != nil {
			return nil, err
		}
	}
	k := convertPathToKey([]byte(p))
	txn := cc.tree.Txn()
	root = txn.Root()
	cr, updated, err := cc.checksum(ctx, root, txn, m, k)
	if err != nil {
		return nil, err
	}
	cc.tree = txn.Commit()
	cc.dirty = updated
	return cr, err
}

func (cc *cacheContext) checksum(ctx context.Context, root *iradix.Node, txn *iradix.Txn, m *mount, k []byte) (*CacheRecord, bool, error) {
	v, ok := root.Get(k)

	if !ok {
		return nil, false, errors.Wrapf(errNotFound, "%s not found", convertKeyToPath(k))
	}
	cr := v.(*CacheRecord)

	if cr.Digest != "" {
		return cr, false, nil
	}
	var dgst digest.Digest

	switch cr.Type {
	case CacheRecordTypeDir:
		h := sha256.New()
		next := append(k, 0)
		iter := root.Seek(next)
		subk := next
		ok := true
		for {
			if !ok || !bytes.HasPrefix(subk, next) {
				break
			}
			h.Write(bytes.TrimPrefix(subk, k))

			subcr, _, err := cc.checksum(ctx, root, txn, m, subk)
			if err != nil {
				return nil, false, err
			}

			h.Write([]byte(subcr.Digest))

			if subcr.Type == CacheRecordTypeDir { // skip subfiles
				next := append(subk, 0, 0xff)
				iter = root.Seek(next)
			}
			subk, _, ok = iter.Next()
		}
		dgst = digest.NewDigest(digest.SHA256, h)

	default:
		p := string(convertKeyToPath(bytes.TrimSuffix(k, []byte{0})))

		target, err := m.mount(ctx)
		if err != nil {
			return nil, false, err
		}

		// no FollowSymlinkInScope because invalid paths should not be inserted
		fp := filepath.Join(target, filepath.FromSlash(p))

		fi, err := os.Lstat(fp)
		if err != nil {
			return nil, false, err
		}

		dgst, err = prepareDigest(fp, p, fi)
		if err != nil {
			return nil, false, err
		}
	}

	cr2 := &CacheRecord{
		Digest:   dgst,
		Type:     cr.Type,
		Linkname: cr.Linkname,
	}

	txn.Insert(k, cr2)

	return cr2, true, nil
}

func (cc *cacheContext) needsScan(root *iradix.Node, p string) bool {
	if p == "/" {
		p = ""
	}
	if _, ok := root.Get(convertPathToKey([]byte(p))); !ok {
		if p == "" {
			return true
		}
		return cc.needsScan(root, path.Clean(path.Dir(p)))
	}
	return false
}

func (cc *cacheContext) scanPath(ctx context.Context, m *mount, p string) (retErr error) {
	p = path.Join("/", p)
	d, _ := path.Split(p)

	mp, err := m.mount(ctx)
	if err != nil {
		return err
	}

	parentPath, err := fs.RootPath(mp, filepath.FromSlash(d))
	if err != nil {
		return err
	}

	n := cc.tree.Root()
	txn := cc.tree.Txn()

	err = filepath.Walk(parentPath, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return errors.Wrapf(err, "failed to walk %s", path)
		}
		rel, err := filepath.Rel(mp, path)
		if err != nil {
			return err
		}
		k := []byte(filepath.Join("/", filepath.ToSlash(rel)))
		if string(k) == "/" {
			k = []byte{}
		}
		k = convertPathToKey(k)
		if _, ok := n.Get(k); !ok {
			cr := &CacheRecord{
				Type: CacheRecordTypeFile,
			}
			if fi.Mode()&os.ModeSymlink != 0 {
				cr.Type = CacheRecordTypeSymlink
				link, err := os.Readlink(path)
				if err != nil {
					return err
				}
				cr.Linkname = filepath.ToSlash(link)
			}
			if fi.IsDir() {
				cr.Type = CacheRecordTypeDirHeader
				cr2 := &CacheRecord{
					Type: CacheRecordTypeDir,
				}
				txn.Insert(k, cr2)
				k = append(k, 0)
			}
			txn.Insert(k, cr)
		}
		return nil
	})
	if err != nil {
		return err
	}

	cc.tree = txn.Commit()
	return nil
}

func prepareDigest(fp, p string, fi os.FileInfo) (digest.Digest, error) {
	h, err := NewFileHash(fp, fi)
	if err != nil {
		return "", errors.Wrapf(err, "failed to create hash for %s", p)
	}
	if fi.Mode().IsRegular() && fi.Size() > 0 {
		// TODO: would be nice to put the contents to separate hash first
		// so it can be cached for hardlinks
		f, err := os.Open(fp)
		if err != nil {
			return "", errors.Wrapf(err, "failed to open %s", p)
		}
		defer f.Close()
		if _, err := poolsCopy(h, f); err != nil {
			return "", errors.Wrapf(err, "failed to copy file data for %s", p)
		}
	}
	return digest.NewDigest(digest.SHA256, h), nil
}

func addParentToMap(d string, m map[string]struct{}) {
	if d == "" {
		return
	}
	d = path.Dir(d)
	if d == "/" {
		d = ""
	}
	m[d] = struct{}{}
	addParentToMap(d, m)
}

func ensureOriginMetadata(md *metadata.StorageItem) *metadata.StorageItem {
	v := md.Get("cache.equalMutable") // TODO: const
	if v == nil {
		return md
	}
	var mutable string
	if err := v.Unmarshal(&mutable); err != nil {
		return md
	}
	si, ok := md.Storage().Get(mutable)
	if ok {
		return si
	}
	return md
}

var pool32K = sync.Pool{
	New: func() interface{} { return make([]byte, 32*1024) }, // 32K
}

func poolsCopy(dst io.Writer, src io.Reader) (written int64, err error) {
	buf := pool32K.Get().([]byte)
	written, err = io.CopyBuffer(dst, src, buf)
	pool32K.Put(buf)
	return
}

func convertPathToKey(p []byte) []byte {
	return bytes.Replace([]byte(p), []byte("/"), []byte{0}, -1)
}

func convertKeyToPath(p []byte) []byte {
	return bytes.Replace([]byte(p), []byte{0}, []byte("/"), -1)
}
