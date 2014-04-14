package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// TreeSize walks a directory tree and returns its total size in bytes.
func TreeSize(dir string) (size int64, err error) {
	data := make(map[uint64]bool)
	err = filepath.Walk(dir, func(d string, fileInfo os.FileInfo, e error) error {
		// Ignore directory sizes
		if fileInfo == nil {
			return nil
		}

		s := fileInfo.Size()
		if fileInfo.IsDir() || s == 0 {
			return nil
		}

		// Check inode to handle hard links correctly
		inode := fileInfo.Sys().(*syscall.Stat_t).Ino
		// inode is not a uint64 on all platforms. Cast it to avoid issues.
		if _, exists := data[uint64(inode)]; exists {
			return nil
		}
		// inode is not a uint64 on all platforms. Cast it to avoid issues.
		data[uint64(inode)] = false

		size += s

		return nil
	})
	return
}

// FollowSymlink will follow an existing link and scope it to the root
// path provided.
func FollowSymlinkInScope(link, root string) (string, error) {
	prev := "/"

	root, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}

	link, err = filepath.Abs(link)
	if err != nil {
		return "", err
	}

	if !strings.HasPrefix(filepath.Dir(link), root) {
		return "", fmt.Errorf("%s is not within %s", link, root)
	}

	for _, p := range strings.Split(link, "/") {
		prev = filepath.Join(prev, p)
		prev = filepath.Clean(prev)

		for {
			if !strings.HasPrefix(prev, root) {
				// Don't resolve symlinks outside of root. For example,
				// we don't have to check /home in the below.
				//
				//   /home -> usr/home
				//   FollowSymlinkInScope("/home/bob/foo/bar", "/home/bob/foo")
				break
			}

			stat, err := os.Lstat(prev)
			if err != nil {
				if os.IsNotExist(err) {
					break
				}
				return "", err
			}
			if stat.Mode()&os.ModeSymlink == os.ModeSymlink {
				dest, err := os.Readlink(prev)
				if err != nil {
					return "", err
				}

				switch dest[0] {
				case '/':
					prev = filepath.Join(root, dest)
				case '.':
					prev, _ = filepath.Abs(prev)

					if prev = filepath.Clean(filepath.Join(filepath.Dir(prev), dest)); len(prev) < len(root) {
						prev = filepath.Join(root, filepath.Base(dest))
					}
				}
			} else {
				break
			}
		}
	}
	return prev, nil
}
