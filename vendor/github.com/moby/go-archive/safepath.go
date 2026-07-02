package archive

import (
	"errors"
	"os"
	"path"
	"path/filepath"
)

// maxSymlinkDepth is the maximum number of symlinks that safeResolve will
// follow before returning errTooManyLinks.
const maxSymlinkDepth = 255

// errTooManyLinks is returned when safeResolve encounters more than
// maxSymlinkDepth symlinks while resolving a path.
var errTooManyLinks = errors.New("too many symlinks")

// safeResolve joins p to root, resolving each path component on-disk to
// detect and bound any symlinks within root. This prevents tar path-traversal
// attacks where a malicious archive creates a symlink pointing outside root
// and then writes files through it.
//
// Unlike filepath.Join(root, p), safeResolve calls os.Lstat on each path
// component. When a component is a symlink, its target is resolved relative
// to root, so absolute symlinks and relative symlinks that escape root are
// silently redirected to stay within root.
//
// Internally safeResolve operates on slash-separated paths so that resolution
// is consistent across platforms regardless of the input separator. Symlink
// targets read via os.Readlink are normalised to forward slashes too, and
// absolute targets — both forward-slash ("/foo") and platform-specific
// ("C:\foo" on Windows) — are bounded within root.
//
// This is ported from github.com/containerd/continuity/fs.RootPath.
func safeResolve(root, p string) (string, error) {
	if p == "" {
		return root, nil
	}
	// Normalise the input to forward slashes so that all internal path
	// operations have consistent semantics on every platform.
	p = filepath.ToSlash(p)
	var linksWalked int
	for {
		prevLinks := linksWalked
		newpath, err := walkLinks(root, p, &linksWalked)
		if err != nil {
			return "", err
		}
		p = newpath
		if prevLinks == linksWalked {
			// No new symlinks resolved; bound the path under root and
			// check for stability.
			newpath = path.Join("/", newpath)
			if p == newpath {
				return filepath.Join(root, filepath.FromSlash(newpath)), nil
			}
			p = newpath
		}
	}
}

// walkLinks resolves symlinks in a slash-separated path one component at a
// time, bounding any symlink targets within root.
func walkLinks(root, p string, linksWalked *int) (string, error) {
	dir, file := path.Split(p)
	switch {
	case dir == "":
		newpath, _, err := walkLink(root, file, linksWalked)
		return newpath, err
	case file == "":
		if dir[len(dir)-1] == '/' {
			if dir == "/" {
				return dir, nil
			}
			// Strip trailing separator and recurse.
			return walkLinks(root, dir[:len(dir)-1], linksWalked)
		}
		newpath, _, err := walkLink(root, dir, linksWalked)
		return newpath, err
	default:
		newdir, err := walkLinks(root, dir, linksWalked)
		if err != nil {
			return "", err
		}
		newpath, islink, err := walkLink(
			root, path.Join(newdir, file), linksWalked,
		)
		if err != nil {
			return "", err
		}
		if !islink {
			return newpath, nil
		}
		// path.IsAbs only recognises forward-slash absolute paths ("/foo");
		// filepath.IsAbs additionally catches the platform-specific cases
		// (e.g. "C:\foo" on Windows). Together they cover both so an
		// absolute symlink target read on either platform is treated as
		// absolute regardless of which separator style it uses.
		if path.IsAbs(newpath) || filepath.IsAbs(newpath) {
			return newpath, nil
		}
		return path.Join(newdir, newpath), nil
	}
}

// walkLink checks whether the component at filepath.Join(root, p) is a
// symlink. If it is, it reads the link target, increments linksWalked, and
// returns the target. Non-existent components are treated as non-symlinks,
// which allows safeResolve to be used for paths that do not yet exist.
//
// p is a slash-separated path; the returned newpath is also slash-separated.
func walkLink(root, p string, linksWalked *int) (newpath string, islink bool, err error) {
	if *linksWalked > maxSymlinkDepth {
		return "", false, errTooManyLinks
	}

	// Normalise to a slash-rooted path so that filepath.Join(root, p)
	// never escapes root regardless of whether p is absolute or relative.
	p = path.Join("/", p)
	if p == "/" {
		return p, false, nil
	}
	realPath := filepath.Join(root, filepath.FromSlash(p))

	fi, err := os.Lstat(realPath)
	if err != nil {
		if os.IsNotExist(err) {
			// The component does not exist yet; treat as a non-symlink so
			// that safeResolve works for paths being created for the first
			// time.
			return p, false, nil
		}
		return "", false, err
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		return p, false, nil
	}

	target, err := os.Readlink(realPath)
	if err != nil {
		return "", false, err
	}
	*linksWalked++
	// Normalise the link target to forward slashes for consistent internal
	// processing; on Windows os.Readlink may return backslash-separated
	// paths.
	return filepath.ToSlash(target), true, nil
}
