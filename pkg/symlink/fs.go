package symlink

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const maxLoopCounter = 100

// FollowSymlink will follow an existing link and scope it to the root
// path provided.
func FollowSymlinkInScope(link, root string) (string, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}

	link, err = filepath.Abs(link)
	if err != nil {
		return "", err
	}

	if link == root {
		return root, nil
	}

	if !strings.HasPrefix(filepath.Dir(link), root) {
		return "", fmt.Errorf("%s is not within %s", link, root)
	}

	prev := "/"

	for _, p := range strings.Split(link, "/") {
		prev = filepath.Join(prev, p)
		prev = filepath.Clean(prev)

		loopCounter := 0
		for {
			loopCounter++

			if loopCounter >= maxLoopCounter {
				return "", fmt.Errorf("loopCounter reached MAX: %v", loopCounter)
			}

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

				if path.IsAbs(dest) {
					prev = filepath.Join(root, dest)
				} else {
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
