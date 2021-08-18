package fs

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/containerd/continuity/fs"
	"github.com/docker/docker/pkg/fileutils"
	"github.com/pkg/errors"
)

var bufferPool = &sync.Pool{
	New: func() interface{} {
		buffer := make([]byte, 32*1024)
		return &buffer
	},
}

func rootPath(root, p string, followLinks bool) (string, error) {
	p = filepath.Join("/", p)
	if p == "/" {
		return root, nil
	}
	if followLinks {
		return fs.RootPath(root, p)
	}
	d, f := filepath.Split(p)
	ppath, err := fs.RootPath(root, d)
	if err != nil {
		return "", err
	}
	return filepath.Join(ppath, f), nil
}

func ResolveWildcards(root, src string, followLinks bool) ([]string, error) {
	d1, d2 := splitWildcards(src)
	if d2 != "" {
		p, err := rootPath(root, d1, followLinks)
		if err != nil {
			return nil, err
		}
		matches, err := resolveWildcards(p, d2)
		if err != nil {
			return nil, err
		}
		for i, m := range matches {
			p, err := rel(root, m)
			if err != nil {
				return nil, err
			}
			matches[i] = p
		}
		return matches, nil
	}
	return []string{d1}, nil
}

// Copy copies files using `cp -a` semantics.
// Copy is likely unsafe to be used in non-containerized environments.
func Copy(ctx context.Context, srcRoot, src, dstRoot, dst string, opts ...Opt) error {
	var ci CopyInfo
	for _, o := range opts {
		o(&ci)
	}
	ensureDstPath := dst
	if d, f := filepath.Split(dst); f != "" && f != "." {
		ensureDstPath = d
	}
	if ensureDstPath != "" {
		ensureDstPath, err := fs.RootPath(dstRoot, ensureDstPath)
		if err != nil {
			return err
		}
		if err := MkdirAll(ensureDstPath, 0755, ci.Chown, ci.Utime); err != nil {
			return err
		}
	}

	dst, err := fs.RootPath(dstRoot, filepath.Clean(dst))
	if err != nil {
		return err
	}

	c, err := newCopier(ci.Chown, ci.Utime, ci.Mode, ci.XAttrErrorHandler, ci.IncludePatterns, ci.ExcludePatterns)
	if err != nil {
		return err
	}
	srcs := []string{src}

	if ci.AllowWildcards {
		matches, err := ResolveWildcards(srcRoot, src, ci.FollowLinks)
		if err != nil {
			return err
		}
		if len(matches) == 0 {
			return errors.Errorf("no matches found: %s", src)
		}
		srcs = matches
	}

	for _, src := range srcs {
		srcFollowed, err := rootPath(srcRoot, src, ci.FollowLinks)
		if err != nil {
			return err
		}
		dst, err := c.prepareTargetDir(srcFollowed, src, dst, ci.CopyDirContents)
		if err != nil {
			return err
		}
		if err := c.copy(ctx, srcFollowed, "", dst, false, false, false); err != nil {
			return err
		}
	}

	return nil
}

func (c *copier) prepareTargetDir(srcFollowed, src, destPath string, copyDirContents bool) (string, error) {
	fiSrc, err := os.Lstat(srcFollowed)
	if err != nil {
		return "", err
	}

	fiDest, err := os.Stat(destPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return "", errors.Wrap(err, "failed to lstat destination path")
		}
	}

	if (!copyDirContents && fiSrc.IsDir() && fiDest != nil) || (!fiSrc.IsDir() && fiDest != nil && fiDest.IsDir()) {
		destPath = filepath.Join(destPath, filepath.Base(src))
	}

	target := filepath.Dir(destPath)

	if copyDirContents && fiSrc.IsDir() && fiDest == nil {
		target = destPath
	}
	if err := MkdirAll(target, 0755, c.chown, c.utime); err != nil {
		return "", err
	}

	return destPath, nil
}

type User struct {
	UID, GID int
}

type Chowner func(*User) (*User, error)

type XAttrErrorHandler func(dst, src, xattrKey string, err error) error

type CopyInfo struct {
	Chown             Chowner
	Utime             *time.Time
	AllowWildcards    bool
	Mode              *int
	XAttrErrorHandler XAttrErrorHandler
	CopyDirContents   bool
	FollowLinks       bool
	// Include only files/dirs matching at least one of these patterns
	IncludePatterns []string
	// Exclude files/dir matching any of these patterns (even if they match an include pattern)
	ExcludePatterns []string
}

type Opt func(*CopyInfo)

func WithCopyInfo(ci CopyInfo) func(*CopyInfo) {
	return func(c *CopyInfo) {
		*c = ci
	}
}

func WithChown(uid, gid int) Opt {
	return func(ci *CopyInfo) {
		ci.Chown = func(*User) (*User, error) {
			return &User{UID: uid, GID: gid}, nil
		}
	}
}

func AllowWildcards(ci *CopyInfo) {
	ci.AllowWildcards = true
}

func WithXAttrErrorHandler(h XAttrErrorHandler) Opt {
	return func(ci *CopyInfo) {
		ci.XAttrErrorHandler = h
	}
}

func AllowXAttrErrors(ci *CopyInfo) {
	h := func(string, string, string, error) error {
		return nil
	}
	WithXAttrErrorHandler(h)(ci)
}

func WithIncludePattern(includePattern string) Opt {
	return func(ci *CopyInfo) {
		ci.IncludePatterns = append(ci.IncludePatterns, includePattern)
	}
}

func WithExcludePattern(excludePattern string) Opt {
	return func(ci *CopyInfo) {
		ci.ExcludePatterns = append(ci.ExcludePatterns, excludePattern)
	}
}

type copier struct {
	chown                 Chowner
	utime                 *time.Time
	mode                  *int
	inodes                map[uint64]string
	xattrErrorHandler     XAttrErrorHandler
	includePatternMatcher *fileutils.PatternMatcher
	excludePatternMatcher *fileutils.PatternMatcher
	parentDirs            []parentDir
}

type parentDir struct {
	srcPath string
	dstPath string
	copied  bool
}

func newCopier(chown Chowner, tm *time.Time, mode *int, xeh XAttrErrorHandler, includePatterns, excludePatterns []string) (*copier, error) {
	if xeh == nil {
		xeh = func(dst, src, key string, err error) error {
			return err
		}
	}

	var includePatternMatcher *fileutils.PatternMatcher
	if len(includePatterns) != 0 {
		var err error
		includePatternMatcher, err = fileutils.NewPatternMatcher(includePatterns)
		if err != nil {
			return nil, errors.Wrapf(err, "invalid includepatterns: %s", includePatterns)
		}
	}

	var excludePatternMatcher *fileutils.PatternMatcher
	if len(excludePatterns) != 0 {
		var err error
		excludePatternMatcher, err = fileutils.NewPatternMatcher(excludePatterns)
		if err != nil {
			return nil, errors.Wrapf(err, "invalid excludepatterns: %s", excludePatterns)
		}
	}

	return &copier{
		inodes:                map[uint64]string{},
		chown:                 chown,
		utime:                 tm,
		xattrErrorHandler:     xeh,
		mode:                  mode,
		includePatternMatcher: includePatternMatcher,
		excludePatternMatcher: excludePatternMatcher,
	}, nil
}

// dest is always clean
func (c *copier) copy(ctx context.Context, src, srcComponents, target string, overwriteTargetMetadata, parentMatchedInclude, parentMatchedExclude bool) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	fi, err := os.Lstat(src)
	if err != nil {
		return errors.Wrapf(err, "failed to stat %s", src)
	}

	include := true
	matchesIncludePattern := false
	matchesExcludePattern := false
	if srcComponents != "" {
		matchesIncludePattern, err = c.include(srcComponents, fi, parentMatchedInclude)
		if err != nil {
			return err
		}
		include = matchesIncludePattern

		matchesExcludePattern, err = c.exclude(srcComponents, fi, parentMatchedExclude)
		if err != nil {
			return err
		}
		if matchesExcludePattern {
			include = false
		}
	}

	if include {
		if err := c.createParentDirs(src, srcComponents, target, overwriteTargetMetadata); err != nil {
			return err
		}
	}

	if !fi.IsDir() {
		if !include {
			return nil
		}

		if err := ensureEmptyFileTarget(target); err != nil {
			return err
		}
	}

	copyFileInfo := true

	switch {
	case fi.IsDir():
		if created, err := c.copyDirectory(
			ctx, src, srcComponents, target, fi, overwriteTargetMetadata,
			include, matchesIncludePattern, matchesExcludePattern,
		); err != nil {
			return err
		} else if !overwriteTargetMetadata || c.includePatternMatcher != nil {
			copyFileInfo = created
		}
	case (fi.Mode() & os.ModeType) == 0:
		link, err := getLinkSource(target, fi, c.inodes)
		if err != nil {
			return errors.Wrap(err, "failed to get hardlink")
		}
		if link != "" {
			if err := os.Link(link, target); err != nil {
				return errors.Wrap(err, "failed to create hard link")
			}
		} else if err := copyFile(src, target); err != nil {
			return errors.Wrap(err, "failed to copy files")
		}
	case (fi.Mode() & os.ModeSymlink) == os.ModeSymlink:
		link, err := os.Readlink(src)
		if err != nil {
			return errors.Wrapf(err, "failed to read link: %s", src)
		}
		if err := os.Symlink(link, target); err != nil {
			return errors.Wrapf(err, "failed to create symlink: %s", target)
		}
	case (fi.Mode() & os.ModeDevice) == os.ModeDevice:
		if err := copyDevice(target, fi); err != nil {
			return errors.Wrapf(err, "failed to create device")
		}
	default:
		// TODO: Support pipes and sockets
		return errors.Wrapf(err, "unsupported mode %s", fi.Mode())
	}

	if copyFileInfo {
		if err := c.copyFileInfo(fi, target); err != nil {
			return errors.Wrap(err, "failed to copy file info")
		}

		if err := copyXAttrs(target, src, c.xattrErrorHandler); err != nil {
			return errors.Wrap(err, "failed to copy xattrs")
		}
	}
	return nil
}

func (c *copier) include(path string, fi os.FileInfo, parentMatchedInclude bool) (bool, error) {
	if c.includePatternMatcher == nil {
		return true, nil
	}

	m, err := c.includePatternMatcher.MatchesUsingParentResult(path, parentMatchedInclude)
	if err != nil {
		return false, errors.Wrap(err, "failed to match includepatterns")
	}
	return m, nil
}

func (c *copier) exclude(path string, fi os.FileInfo, parentMatchedExclude bool) (bool, error) {
	if c.excludePatternMatcher == nil {
		return false, nil
	}

	m, err := c.excludePatternMatcher.MatchesUsingParentResult(path, parentMatchedExclude)
	if err != nil {
		return false, errors.Wrap(err, "failed to match excludepatterns")
	}
	return m, nil
}

// Delayed creation of parent directories when a file or dir matches an include
// pattern.
func (c *copier) createParentDirs(src, srcComponents, target string, overwriteTargetMetadata bool) error {
	for i, parentDir := range c.parentDirs {
		if parentDir.copied {
			continue
		}

		fi, err := os.Stat(parentDir.srcPath)
		if err != nil {
			return errors.Wrapf(err, "failed to stat %s", src)
		}
		if !fi.IsDir() {
			return errors.Errorf("%s is not a directory", parentDir.srcPath)
		}

		created, err := copyDirectoryOnly(parentDir.srcPath, parentDir.dstPath, fi, overwriteTargetMetadata)
		if err != nil {
			return err
		}
		if created {
			if err := c.copyFileInfo(fi, parentDir.dstPath); err != nil {
				return errors.Wrap(err, "failed to copy file info")
			}

			if err := copyXAttrs(parentDir.dstPath, parentDir.srcPath, c.xattrErrorHandler); err != nil {
				return errors.Wrap(err, "failed to copy xattrs")
			}
		}

		c.parentDirs[i].copied = true
	}
	return nil
}

func (c *copier) copyDirectory(
	ctx context.Context,
	src string,
	srcComponents string,
	dst string,
	stat os.FileInfo,
	overwriteTargetMetadata bool,
	include bool,
	matchesIncludePattern bool,
	matchesExcludePattern bool,
) (bool, error) {
	if !stat.IsDir() {
		return false, errors.Errorf("source is not directory")
	}

	created := false

	parentDir := parentDir{
		srcPath: src,
		dstPath: dst,
	}

	// If this directory passed include/exclude matching directly, go ahead
	// and create the directory. Otherwise, delay to handle include
	// patterns like a/*/c where we do not want to create a/b until we
	// encounter a/b/c.
	if include {
		var err error
		created, err = copyDirectoryOnly(src, dst, stat, overwriteTargetMetadata)
		if err != nil {
			return created, err
		}
		parentDir.copied = true
	}

	c.parentDirs = append(c.parentDirs, parentDir)

	defer func() {
		c.parentDirs = c.parentDirs[:len(c.parentDirs)-1]
	}()

	fis, err := ioutil.ReadDir(src)
	if err != nil {
		return false, errors.Wrapf(err, "failed to read %s", src)
	}

	for _, fi := range fis {
		if err := c.copy(
			ctx,
			filepath.Join(src, fi.Name()), filepath.Join(srcComponents, fi.Name()),
			filepath.Join(dst, fi.Name()),
			true, matchesIncludePattern, matchesExcludePattern,
		); err != nil {
			return false, err
		}
	}

	return created, nil
}

func copyDirectoryOnly(src, dst string, stat os.FileInfo, overwriteTargetMetadata bool) (bool, error) {
	if st, err := os.Lstat(dst); err != nil {
		if !os.IsNotExist(err) {
			return false, err
		}
		if err := os.Mkdir(dst, stat.Mode()); err != nil {
			return false, errors.Wrapf(err, "failed to mkdir %s", dst)
		}
		return true, nil
	} else if !st.IsDir() {
		return false, errors.Errorf("cannot copy to non-directory: %s", dst)
	} else if overwriteTargetMetadata {
		if err := os.Chmod(dst, stat.Mode()); err != nil {
			return false, errors.Wrapf(err, "failed to chmod on %s", dst)
		}
	}
	return false, nil
}

func ensureEmptyFileTarget(dst string) error {
	fi, err := os.Lstat(dst)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return errors.Wrap(err, "failed to lstat file target")
	}
	if fi.IsDir() {
		return errors.Errorf("cannot replace to directory %s with file", dst)
	}
	return os.Remove(dst)
}

func containsWildcards(name string) bool {
	isWindows := runtime.GOOS == "windows"
	for i := 0; i < len(name); i++ {
		ch := name[i]
		if ch == '\\' && !isWindows {
			i++
		} else if ch == '*' || ch == '?' || ch == '[' {
			return true
		}
	}
	return false
}

func splitWildcards(p string) (d1, d2 string) {
	parts := strings.Split(filepath.Join(p), string(filepath.Separator))
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
	return filepath.Join(p1...), filepath.Join(p2...)
}

func resolveWildcards(basePath, comp string) ([]string, error) {
	var out []string
	err := filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := rel(basePath, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if match, _ := filepath.Match(comp, rel); !match {
			return nil
		}
		out = append(out, path)
		if info.IsDir() {
			return filepath.SkipDir
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// rel makes a path relative to base path. Same as `filepath.Rel` but can also
// handle UUID paths in windows.
func rel(basepath, targpath string) (string, error) {
	// filepath.Rel can't handle UUID paths in windows
	if runtime.GOOS == "windows" {
		pfx := basepath + `\`
		if strings.HasPrefix(targpath, pfx) {
			p := strings.TrimPrefix(targpath, pfx)
			if p == "" {
				p = "."
			}
			return p, nil
		}
	}
	return filepath.Rel(basepath, targpath)
}
