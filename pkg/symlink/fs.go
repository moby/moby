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
// The role of this function is to return an absolute path in the root
// or normalize to the root if the symlink leads to a path which is
// outside of the root.
// Errors encountered while attempting to follow the symlink in path
// will be reported.
// Normalizations to the root don't constitute errors.
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

			// let's break if we're not dealing with a symlink
			if stat.Mode()&os.ModeSymlink != os.ModeSymlink {
				break
			}

			// process the symlink
			dest, err := os.Readlink(prev)
			if err != nil {
				return "", err
			}

			if path.IsAbs(dest) {
				prev = filepath.Join(root, dest)
			} else {
				prev, _ = filepath.Abs(prev)

				dir := filepath.Dir(prev)
				prev = filepath.Join(dir, dest)
				if dir == root && !strings.HasPrefix(prev, root) {
					prev = root
				}
				if len(prev) < len(root) || (len(prev) == len(root) && prev != root) {
					prev = filepath.Join(root, filepath.Base(dest))
				}
			}
		}
	}
	if prev == "/" {
		prev = root
	}
	return prev, nil
}
