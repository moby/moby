package contenthash

import (
	"errors"
	"os"
	"path/filepath"
)

var (
	errTooManyLinks = errors.New("too many links")
)

type onSymlinkFunc func(string, string) error

// rootPath joins a path with a root, evaluating and bounding any
// symlink to the root directory.
// This is containerd/continuity/fs RootPath implementation with a callback on
// resolving the symlink.
func rootPath(root, path string, cb onSymlinkFunc) (string, error) {
	if path == "" {
		return root, nil
	}
	var linksWalked int // to protect against cycles
	for {
		i := linksWalked
		newpath, err := walkLinks(root, path, &linksWalked, cb)
		if err != nil {
			return "", err
		}
		path = newpath
		if i == linksWalked {
			newpath = filepath.Join("/", newpath)
			if path == newpath {
				return filepath.Join(root, newpath), nil
			}
			path = newpath
		}
	}
}

func walkLink(root, path string, linksWalked *int, cb onSymlinkFunc) (newpath string, islink bool, err error) {
	if *linksWalked > 255 {
		return "", false, errTooManyLinks
	}

	path = filepath.Join("/", path)
	if path == "/" {
		return path, false, nil
	}
	realPath := filepath.Join(root, path)

	fi, err := os.Lstat(realPath)
	if err != nil {
		// If path does not yet exist, treat as non-symlink
		if os.IsNotExist(err) {
			return path, false, nil
		}
		return "", false, err
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		return path, false, nil
	}
	newpath, err = os.Readlink(realPath)
	if err != nil {
		return "", false, err
	}
	if cb != nil {
		if err := cb(path, newpath); err != nil {
			return "", false, err
		}
	}
	*linksWalked++
	return newpath, true, nil
}

func walkLinks(root, path string, linksWalked *int, cb onSymlinkFunc) (string, error) {
	switch dir, file := filepath.Split(path); {
	case dir == "":
		newpath, _, err := walkLink(root, file, linksWalked, cb)
		return newpath, err
	case file == "":
		if os.IsPathSeparator(dir[len(dir)-1]) {
			if dir == "/" {
				return dir, nil
			}
			return walkLinks(root, dir[:len(dir)-1], linksWalked, cb)
		}
		newpath, _, err := walkLink(root, dir, linksWalked, cb)
		return newpath, err
	default:
		newdir, err := walkLinks(root, dir, linksWalked, cb)
		if err != nil {
			return "", err
		}
		newpath, islink, err := walkLink(root, filepath.Join(newdir, file), linksWalked, cb)
		if err != nil {
			return "", err
		}
		if !islink {
			return newpath, nil
		}
		if filepath.IsAbs(newpath) {
			return newpath, nil
		}
		return filepath.Join(newdir, newpath), nil
	}
}
