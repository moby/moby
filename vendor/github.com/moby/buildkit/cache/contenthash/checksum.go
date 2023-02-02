package contenthash

import (
	"bytes"
	"context"
	"crypto/sha256"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	iradix "github.com/hashicorp/go-immutable-radix"
	"github.com/hashicorp/golang-lru/simplelru"
	"github.com/moby/buildkit/cache"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/snapshot"
	"github.com/moby/locker"
	"github.com/moby/patternmatcher"
	digest "github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/tonistiigi/fsutil"
	fstypes "github.com/tonistiigi/fsutil/types"
)

var errNotFound = errors.Errorf("not found")

var defaultManager *cacheManager
var defaultManagerOnce sync.Once

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

type ChecksumOpts struct {
	FollowLinks     bool
	Wildcard        bool
	IncludePatterns []string
	ExcludePatterns []string
}

func Checksum(ctx context.Context, ref cache.ImmutableRef, path string, opts ChecksumOpts, s session.Group) (digest.Digest, error) {
	return getDefaultManager().Checksum(ctx, ref, path, opts, s)
}

func GetCacheContext(ctx context.Context, md cache.RefMetadata) (CacheContext, error) {
	return getDefaultManager().GetCacheContext(ctx, md)
}

func SetCacheContext(ctx context.Context, md cache.RefMetadata, cc CacheContext) error {
	return getDefaultManager().SetCacheContext(ctx, md, cc)
}

func ClearCacheContext(md cache.RefMetadata) {
	getDefaultManager().clearCacheContext(md.ID())
}

type CacheContext interface {
	Checksum(ctx context.Context, ref cache.Mountable, p string, opts ChecksumOpts, s session.Group) (digest.Digest, error)
	HandleChange(kind fsutil.ChangeKind, p string, fi os.FileInfo, err error) error
}

type Hashed interface {
	Digest() digest.Digest
}

type includedPath struct {
	path             string
	record           *CacheRecord
	included         bool
	includeMatchInfo patternmatcher.MatchInfo
	excludeMatchInfo patternmatcher.MatchInfo
}

type cacheManager struct {
	locker *locker.Locker
	lru    *simplelru.LRU
	lruMu  sync.Mutex
}

func (cm *cacheManager) Checksum(ctx context.Context, ref cache.ImmutableRef, p string, opts ChecksumOpts, s session.Group) (digest.Digest, error) {
	if ref == nil {
		if p == "/" {
			return digest.FromBytes(nil), nil
		}
		return "", errors.Errorf("%s: no such file or directory", p)
	}
	cc, err := cm.GetCacheContext(ctx, ensureOriginMetadata(ref))
	if err != nil {
		return "", nil
	}
	return cc.Checksum(ctx, ref, p, opts, s)
}

func (cm *cacheManager) GetCacheContext(ctx context.Context, md cache.RefMetadata) (CacheContext, error) {
	cm.locker.Lock(md.ID())
	cm.lruMu.Lock()
	v, ok := cm.lru.Get(md.ID())
	cm.lruMu.Unlock()
	if ok {
		cm.locker.Unlock(md.ID())
		v.(*cacheContext).linkMap = map[string][][]byte{}
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

func (cm *cacheManager) SetCacheContext(ctx context.Context, md cache.RefMetadata, cci CacheContext) error {
	cc, ok := cci.(*cacheContext)
	if !ok {
		return errors.Errorf("invalid cachecontext: %T", cc)
	}
	if md.ID() != cc.md.ID() {
		cc = &cacheContext{
			md:       cacheMetadata{md},
			tree:     cci.(*cacheContext).tree,
			dirtyMap: map[string]struct{}{},
			linkMap:  map[string][][]byte{},
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

func (cm *cacheManager) clearCacheContext(id string) {
	cm.lruMu.Lock()
	cm.lru.Remove(id)
	cm.lruMu.Unlock()
}

type cacheContext struct {
	mu    sync.RWMutex
	md    cacheMetadata
	tree  *iradix.Tree
	dirty bool // needs to be persisted to disk

	// used in HandleChange
	txn      *iradix.Txn
	node     *iradix.Node
	dirtyMap map[string]struct{}
	linkMap  map[string][][]byte
}

type cacheMetadata struct {
	cache.RefMetadata
}

const keyContentHash = "buildkit.contenthash.v0"

func (md cacheMetadata) GetContentHash() ([]byte, error) {
	return md.GetExternal(keyContentHash)
}

func (md cacheMetadata) SetContentHash(dt []byte) error {
	return md.SetExternal(keyContentHash, dt)
}

type mount struct {
	mountable cache.Mountable
	mountPath string
	unmount   func() error
	session   session.Group
}

func (m *mount) mount(ctx context.Context) (string, error) {
	if m.mountPath != "" {
		return m.mountPath, nil
	}
	mounts, err := m.mountable.Mount(ctx, true, m.session)
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

func newCacheContext(md cache.RefMetadata) (*cacheContext, error) {
	cc := &cacheContext{
		md:       cacheMetadata{md},
		tree:     iradix.New(),
		dirtyMap: map[string]struct{}{},
		linkMap:  map[string][][]byte{},
	}
	if err := cc.load(); err != nil {
		return nil, err
	}
	return cc, nil
}

func (cc *cacheContext) load() error {
	dt, err := cc.md.GetContentHash()
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

	return cc.md.SetContentHash(dt)
}

func keyPath(p string) string {
	p = path.Join("/", filepath.ToSlash(p))
	if p == "/" {
		p = ""
	}
	return p
}

// HandleChange notifies the source about a modification operation
func (cc *cacheContext) HandleChange(kind fsutil.ChangeKind, p string, fi os.FileInfo, err error) (retErr error) {
	p = keyPath(p)
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

	stat, ok := fi.Sys().(*fstypes.Stat)
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

	// if we receive a hardlink just use the digest of the source
	// note that the source may be called later because data writing is async
	if fi.Mode()&os.ModeSymlink == 0 && stat.Linkname != "" {
		ln := path.Join("/", filepath.ToSlash(stat.Linkname))
		v, ok := cc.txn.Get(convertPathToKey([]byte(ln)))
		if ok {
			cp := *v.(*CacheRecord)
			cr = &cp
		}
		cc.linkMap[ln] = append(cc.linkMap[ln], k)
	}

	cc.txn.Insert(k, cr)
	if !fi.IsDir() {
		if links, ok := cc.linkMap[p]; ok {
			for _, l := range links {
				pp := convertKeyToPath(l)
				cc.txn.Insert(l, cr)
				d := path.Dir(string(pp))
				if d == "/" {
					d = ""
				}
				cc.dirtyMap[d] = struct{}{}
			}
			delete(cc.linkMap, p)
		}
	}

	d := path.Dir(p)
	if d == "/" {
		d = ""
	}
	cc.dirtyMap[d] = struct{}{}

	return nil
}

func (cc *cacheContext) Checksum(ctx context.Context, mountable cache.Mountable, p string, opts ChecksumOpts, s session.Group) (digest.Digest, error) {
	m := &mount{mountable: mountable, session: s}
	defer m.clean()

	if !opts.Wildcard && len(opts.IncludePatterns) == 0 && len(opts.ExcludePatterns) == 0 {
		return cc.checksumFollow(ctx, m, p, opts.FollowLinks)
	}

	includedPaths, err := cc.includedPaths(ctx, m, p, opts)
	if err != nil {
		return "", err
	}

	if opts.FollowLinks {
		for i, w := range includedPaths {
			if w.record.Type == CacheRecordTypeSymlink {
				dgst, err := cc.checksumFollow(ctx, m, w.path, opts.FollowLinks)
				if err != nil {
					return "", err
				}
				includedPaths[i].record = &CacheRecord{Digest: dgst}
			}
		}
	}
	if len(includedPaths) == 0 {
		return digest.FromBytes([]byte{}), nil
	}

	if len(includedPaths) == 1 && path.Base(p) == path.Base(includedPaths[0].path) {
		return includedPaths[0].record.Digest, nil
	}

	digester := digest.Canonical.Digester()
	for i, w := range includedPaths {
		if i != 0 {
			digester.Hash().Write([]byte{0})
		}
		digester.Hash().Write([]byte(path.Base(w.path)))
		digester.Hash().Write([]byte(w.record.Digest))
	}
	return digester.Digest(), nil
}

func (cc *cacheContext) checksumFollow(ctx context.Context, m *mount, p string, follow bool) (digest.Digest, error) {
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
		if cr.Type == CacheRecordTypeSymlink && follow {
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

func (cc *cacheContext) includedPaths(ctx context.Context, m *mount, p string, opts ChecksumOpts) ([]*includedPath, error) {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	if cc.txn != nil {
		cc.commitActiveTransaction()
	}

	root := cc.tree.Root()
	scan, err := cc.needsScan(root, "")
	if err != nil {
		return nil, err
	}
	if scan {
		if err := cc.scanPath(ctx, m, ""); err != nil {
			return nil, err
		}
	}

	defer func() {
		if cc.dirty {
			go cc.save()
			cc.dirty = false
		}
	}()

	endsInSep := len(p) != 0 && p[len(p)-1] == filepath.Separator
	p = keyPath(p)

	var includePatternMatcher *patternmatcher.PatternMatcher
	if len(opts.IncludePatterns) != 0 {
		includePatternMatcher, err = patternmatcher.New(opts.IncludePatterns)
		if err != nil {
			return nil, errors.Wrapf(err, "invalid includepatterns: %s", opts.IncludePatterns)
		}
	}

	var excludePatternMatcher *patternmatcher.PatternMatcher
	if len(opts.ExcludePatterns) != 0 {
		excludePatternMatcher, err = patternmatcher.New(opts.ExcludePatterns)
		if err != nil {
			return nil, errors.Wrapf(err, "invalid excludepatterns: %s", opts.ExcludePatterns)
		}
	}

	includedPaths := make([]*includedPath, 0, 2)

	txn := cc.tree.Txn()
	root = txn.Root()
	var (
		updated        bool
		iter           *iradix.Iterator
		k              []byte
		keyOk          bool
		origPrefix     string
		resolvedPrefix string
	)

	iter = root.Iterator()

	if opts.Wildcard {
		origPrefix, k, keyOk, err = wildcardPrefix(root, p)
		if err != nil {
			return nil, err
		}
	} else {
		origPrefix = p
		k = convertPathToKey([]byte(origPrefix))

		// We need to resolve symlinks here, in case the base path
		// involves a symlink. That will match fsutil behavior of
		// calling functions such as stat and walk.
		var cr *CacheRecord
		k, cr, err = getFollowLinks(root, k, true)
		if err != nil {
			return nil, err
		}
		keyOk = (cr != nil)
	}

	if origPrefix != "" {
		if keyOk {
			iter.SeekLowerBound(append(append([]byte{}, k...), 0))
		}

		resolvedPrefix = string(convertKeyToPath(k))
	} else {
		k, _, keyOk = iter.Next()
	}

	var (
		parentDirHeaders []*includedPath
		lastMatchedDir   string
	)

	for keyOk {
		fn := string(convertKeyToPath(k))

		// Convert the path prefix from what we found in the prefix
		// tree to what the argument specified.
		//
		// For example, if the original 'p' argument was /a/b and there
		// is a symlink a->c, we want fn to be /a/b/foo rather than
		// /c/b/foo. This is necessary to ensure correct pattern
		// matching.
		//
		// When wildcards are enabled, this translation applies to the
		// portion of 'p' before any wildcards.
		if strings.HasPrefix(fn, resolvedPrefix) {
			fn = origPrefix + strings.TrimPrefix(fn, resolvedPrefix)
		}

		for len(parentDirHeaders) != 0 {
			lastParentDir := parentDirHeaders[len(parentDirHeaders)-1]
			if strings.HasPrefix(fn, lastParentDir.path+"/") {
				break
			}
			parentDirHeaders = parentDirHeaders[:len(parentDirHeaders)-1]
		}
		var parentDir *includedPath
		if len(parentDirHeaders) != 0 {
			parentDir = parentDirHeaders[len(parentDirHeaders)-1]
		}

		dirHeader := false
		if len(k) > 0 && k[len(k)-1] == byte(0) {
			dirHeader = true
			fn = fn[:len(fn)-1]
			if fn == p && endsInSep {
				// We don't include the metadata header for a source dir which ends with a separator
				k, _, keyOk = iter.Next()
				continue
			}
		}

		maybeIncludedPath := &includedPath{path: fn}
		var shouldInclude bool
		if opts.Wildcard {
			if p != "" && (lastMatchedDir == "" || !strings.HasPrefix(fn, lastMatchedDir+"/")) {
				include, err := path.Match(p, fn)
				if err != nil {
					return nil, err
				}
				if !include {
					k, _, keyOk = iter.Next()
					continue
				}
				lastMatchedDir = fn
			}

			shouldInclude, err = shouldIncludePath(
				strings.TrimSuffix(strings.TrimPrefix(fn+"/", lastMatchedDir+"/"), "/"),
				includePatternMatcher,
				excludePatternMatcher,
				maybeIncludedPath,
				parentDir,
			)
			if err != nil {
				return nil, err
			}
		} else {
			if !strings.HasPrefix(fn+"/", p+"/") {
				break
			}

			shouldInclude, err = shouldIncludePath(
				strings.TrimSuffix(strings.TrimPrefix(fn+"/", p+"/"), "/"),
				includePatternMatcher,
				excludePatternMatcher,
				maybeIncludedPath,
				parentDir,
			)
			if err != nil {
				return nil, err
			}
		}

		if !shouldInclude && !dirHeader {
			k, _, keyOk = iter.Next()
			continue
		}

		cr, upt, err := cc.checksum(ctx, root, txn, m, k, false)
		if err != nil {
			return nil, err
		}
		if upt {
			updated = true
		}

		if cr.Type == CacheRecordTypeDir {
			// We only hash dir headers and files, not dir contents. Hashing
			// dir contents could be wrong if there are exclusions within the
			// dir.
			shouldInclude = false
		}
		maybeIncludedPath.record = cr

		if shouldInclude {
			for _, parentDir := range parentDirHeaders {
				if !parentDir.included {
					includedPaths = append(includedPaths, parentDir)
					parentDir.included = true
				}
			}
			includedPaths = append(includedPaths, maybeIncludedPath)
			maybeIncludedPath.included = true
		}

		if cr.Type == CacheRecordTypeDirHeader {
			// We keep track of parent dir headers whether
			// they are immediately included or not, in case
			// an include pattern matches a file inside one
			// of these dirs.
			parentDirHeaders = append(parentDirHeaders, maybeIncludedPath)
		}

		k, _, keyOk = iter.Next()
	}

	cc.tree = txn.Commit()
	cc.dirty = updated

	return includedPaths, nil
}

func shouldIncludePath(
	candidate string,
	includePatternMatcher *patternmatcher.PatternMatcher,
	excludePatternMatcher *patternmatcher.PatternMatcher,
	maybeIncludedPath *includedPath,
	parentDir *includedPath,
) (bool, error) {
	var (
		m         bool
		matchInfo patternmatcher.MatchInfo
		err       error
	)
	if includePatternMatcher != nil {
		if parentDir != nil {
			m, matchInfo, err = includePatternMatcher.MatchesUsingParentResults(candidate, parentDir.includeMatchInfo)
		} else {
			m, matchInfo, err = includePatternMatcher.MatchesUsingParentResults(candidate, patternmatcher.MatchInfo{})
		}
		if err != nil {
			return false, errors.Wrap(err, "failed to match includepatterns")
		}
		maybeIncludedPath.includeMatchInfo = matchInfo
		if !m {
			return false, nil
		}
	}

	if excludePatternMatcher != nil {
		if parentDir != nil {
			m, matchInfo, err = excludePatternMatcher.MatchesUsingParentResults(candidate, parentDir.excludeMatchInfo)
		} else {
			m, matchInfo, err = excludePatternMatcher.MatchesUsingParentResults(candidate, patternmatcher.MatchInfo{})
		}
		if err != nil {
			return false, errors.Wrap(err, "failed to match excludepatterns")
		}
		maybeIncludedPath.excludeMatchInfo = matchInfo
		if m {
			return false, nil
		}
	}

	return true, nil
}

func wildcardPrefix(root *iradix.Node, p string) (string, []byte, bool, error) {
	// For consistency with what the copy implementation in fsutil
	// does: split pattern into non-wildcard prefix and rest of
	// pattern, then follow symlinks when resolving the non-wildcard
	// prefix.

	d1, d2 := splitWildcards(p)
	if d1 == "/" {
		return "", nil, false, nil
	}

	linksWalked := 0
	k, cr, err := getFollowLinksWalk(root, convertPathToKey([]byte(d1)), true, &linksWalked)
	if err != nil {
		return "", k, false, err
	}

	if d2 != "" && cr != nil && cr.Type == CacheRecordTypeSymlink {
		// getFollowLinks only handles symlinks in path
		// components before the last component, so
		// handle last component in d1 specially.
		resolved := string(convertKeyToPath(k))
		for {
			v, ok := root.Get(k)

			if !ok {
				return d1, k, false, nil
			}
			if v.(*CacheRecord).Type != CacheRecordTypeSymlink {
				break
			}

			linksWalked++
			if linksWalked > 255 {
				return "", k, false, errors.Errorf("too many links")
			}

			resolved := cleanLink(resolved, v.(*CacheRecord).Linkname)
			k = convertPathToKey([]byte(resolved))
		}
	}
	return d1, k, cr != nil, nil
}

func splitWildcards(p string) (d1, d2 string) {
	parts := strings.Split(path.Join(p), "/")
	var p1, p2 []string
	var found bool
	for _, p := range parts {
		if !found && containsWildcards(p) {
			found = true
		}
		if p == "" {
			p = "/"
		}
		if !found {
			p1 = append(p1, p)
		} else {
			p2 = append(p2, p)
		}
	}
	return path.Join(p1...), path.Join(p2...)
}

func containsWildcards(name string) bool {
	for i := 0; i < len(name); i++ {
		ch := name[i]
		if ch == '\\' {
			i++
		} else if ch == '*' || ch == '?' || ch == '[' {
			return true
		}
	}
	return false
}

func (cc *cacheContext) checksumNoFollow(ctx context.Context, m *mount, p string) (*CacheRecord, error) {
	p = keyPath(p)

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
	scan, err := cc.needsScan(root, p)
	if err != nil {
		return nil, err
	}
	if scan {
		if err := cc.scanPath(ctx, m, p); err != nil {
			return nil, err
		}
	}
	k := convertPathToKey([]byte(p))
	txn := cc.tree.Txn()
	root = txn.Root()
	cr, updated, err := cc.checksum(ctx, root, txn, m, k, true)
	if err != nil {
		return nil, err
	}
	cc.tree = txn.Commit()
	cc.dirty = updated
	return cr, err
}

func (cc *cacheContext) checksum(ctx context.Context, root *iradix.Node, txn *iradix.Txn, m *mount, k []byte, follow bool) (*CacheRecord, bool, error) {
	origk := k
	k, cr, err := getFollowLinks(root, k, follow)
	if err != nil {
		return nil, false, err
	}
	if cr == nil {
		return nil, false, errors.Wrapf(errNotFound, "%q", convertKeyToPath(origk))
	}
	if cr.Digest != "" {
		return cr, false, nil
	}
	var dgst digest.Digest

	switch cr.Type {
	case CacheRecordTypeDir:
		h := sha256.New()
		next := append(k, 0)
		iter := root.Iterator()
		iter.SeekLowerBound(append(append([]byte{}, next...), 0))
		subk := next
		ok := true
		for {
			if !ok || !bytes.HasPrefix(subk, next) {
				break
			}
			h.Write(bytes.TrimPrefix(subk, k))

			subcr, _, err := cc.checksum(ctx, root, txn, m, subk, true)
			if err != nil {
				return nil, false, err
			}

			h.Write([]byte(subcr.Digest))

			if subcr.Type == CacheRecordTypeDir { // skip subfiles
				next := append(subk, 0, 0xff)
				iter = root.Iterator()
				iter.SeekLowerBound(next)
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

// needsScan returns false if path is in the tree or a parent path is in tree
// and subpath is missing
func (cc *cacheContext) needsScan(root *iradix.Node, p string) (bool, error) {
	var linksWalked int
	return cc.needsScanFollow(root, p, &linksWalked)
}

func (cc *cacheContext) needsScanFollow(root *iradix.Node, p string, linksWalked *int) (bool, error) {
	if p == "/" {
		p = ""
	}
	v, ok := root.Get(convertPathToKey([]byte(p)))
	if !ok {
		if p == "" {
			return true, nil
		}
		return cc.needsScanFollow(root, path.Clean(path.Dir(p)), linksWalked)
	}
	cr := v.(*CacheRecord)
	if cr.Type == CacheRecordTypeSymlink {
		if *linksWalked > 255 {
			return false, errTooManyLinks
		}
		*linksWalked++
		link := path.Clean(cr.Linkname)
		if !path.IsAbs(cr.Linkname) {
			link = path.Join("/", path.Dir(p), link)
		}
		return cc.needsScanFollow(root, link, linksWalked)
	}
	return false, nil
}

func (cc *cacheContext) scanPath(ctx context.Context, m *mount, p string) (retErr error) {
	p = path.Join("/", p)
	d, _ := path.Split(p)

	mp, err := m.mount(ctx)
	if err != nil {
		return err
	}

	n := cc.tree.Root()
	txn := cc.tree.Txn()

	parentPath, err := rootPath(mp, filepath.FromSlash(d), func(p, link string) error {
		cr := &CacheRecord{
			Type:     CacheRecordTypeSymlink,
			Linkname: filepath.ToSlash(link),
		}
		k := []byte(path.Join("/", filepath.ToSlash(p)))
		k = convertPathToKey(k)
		txn.Insert(k, cr)
		return nil
	})
	if err != nil {
		return err
	}

	err = filepath.Walk(parentPath, func(itemPath string, fi os.FileInfo, err error) error {
		if err != nil {
			return errors.Wrapf(err, "failed to walk %s", itemPath)
		}
		rel, err := filepath.Rel(mp, itemPath)
		if err != nil {
			return err
		}
		k := []byte(path.Join("/", filepath.ToSlash(rel)))
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
				link, err := os.Readlink(itemPath)
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

func getFollowLinks(root *iradix.Node, k []byte, follow bool) ([]byte, *CacheRecord, error) {
	var linksWalked int
	return getFollowLinksWalk(root, k, follow, &linksWalked)
}

func getFollowLinksWalk(root *iradix.Node, k []byte, follow bool, linksWalked *int) ([]byte, *CacheRecord, error) {
	v, ok := root.Get(k)
	if ok {
		return k, v.(*CacheRecord), nil
	}
	if !follow || len(k) == 0 {
		return k, nil, nil
	}

	dir, file := splitKey(k)

	k, parent, err := getFollowLinksWalk(root, dir, follow, linksWalked)
	if err != nil {
		return nil, nil, err
	}
	if parent != nil {
		if parent.Type == CacheRecordTypeSymlink {
			*linksWalked++
			if *linksWalked > 255 {
				return nil, nil, errors.Errorf("too many links")
			}

			link := cleanLink(string(convertKeyToPath(dir)), parent.Linkname)
			return getFollowLinksWalk(root, append(convertPathToKey([]byte(link)), file...), follow, linksWalked)
		}
	}
	k = append(k, file...)
	v, ok = root.Get(k)
	if ok {
		return k, v.(*CacheRecord), nil
	}
	return k, nil, nil
}

func cleanLink(dir, linkname string) string {
	dirPath := path.Clean(dir)
	if dirPath == "." || dirPath == "/" {
		dirPath = ""
	}
	link := path.Clean(linkname)
	if !path.IsAbs(link) {
		return path.Join("/", path.Join(path.Dir(dirPath), link))
	}
	return link
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

func ensureOriginMetadata(md cache.RefMetadata) cache.RefMetadata {
	em, ok := md.GetEqualMutable()
	if !ok {
		em = md
	}
	return em
}

var pool32K = sync.Pool{
	New: func() interface{} {
		buf := make([]byte, 32*1024) // 32K
		return &buf
	},
}

func poolsCopy(dst io.Writer, src io.Reader) (written int64, err error) {
	buf := pool32K.Get().(*[]byte)
	written, err = io.CopyBuffer(dst, src, *buf)
	pool32K.Put(buf)
	return
}

func convertPathToKey(p []byte) []byte {
	return bytes.Replace([]byte(p), []byte("/"), []byte{0}, -1)
}

func convertKeyToPath(p []byte) []byte {
	return bytes.Replace([]byte(p), []byte{0}, []byte("/"), -1)
}

func splitKey(k []byte) ([]byte, []byte) {
	foundBytes := false
	i := len(k) - 1
	for {
		if i <= 0 || foundBytes && k[i] == 0 {
			break
		}
		if k[i] != 0 {
			foundBytes = true
		}
		i--
	}
	return append([]byte{}, k[:i]...), k[i:]
}
