package safepath

/*
Copyright 2014 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/containerd/log"
	"github.com/docker/docker/internal/unix_noeintr"
	"golang.org/x/sys/unix"
)

// kubernetesSafeOpen open path formed by concatenation of the base directory
// and its subpath and return its fd.
// Symlinks are disallowed (pathname must already resolve symlinks) and the
// path must be within the base directory.
// This is minimally modified code from https://github.com/kubernetes/kubernetes/blob/55fb1805a1217b91b36fa8fe8f2bf3a28af2454d/pkg/volume/util/subpath/subpath_linux.go#L530
func kubernetesSafeOpen(base, subpath string) (int, error) {
	// syscall.Openat flags used to traverse directories not following symlinks
	const nofollowFlags = unix.O_RDONLY | unix.O_NOFOLLOW
	// flags for getting file descriptor without following the symlink
	const openFDFlags = unix.O_NOFOLLOW | unix.O_PATH

	pathname := filepath.Join(base, subpath)
	segments := strings.Split(subpath, string(filepath.Separator))

	// Assumption: base is the only directory that we have under control.
	// Base dir is not allowed to be a symlink.
	parentFD, err := unix_noeintr.Open(base, nofollowFlags|unix.O_CLOEXEC, 0)
	if err != nil {
		return -1, &ErrNotAccessible{Path: base, Cause: err}
	}
	defer func() {
		if parentFD != -1 {
			if err = unix_noeintr.Close(parentFD); err != nil {
				log.G(context.TODO()).Errorf("Closing FD %v failed for safeopen(%v): %v", parentFD, pathname, err)
			}
		}
	}()

	childFD := -1
	defer func() {
		if childFD != -1 {
			if err = unix_noeintr.Close(childFD); err != nil {
				log.G(context.TODO()).Errorf("Closing FD %v failed for safeopen(%v): %v", childFD, pathname, err)
			}
		}
	}()

	currentPath := base

	// Follow the segments one by one using openat() to make
	// sure the user cannot change already existing directories into symlinks.
	for _, seg := range segments {
		var deviceStat unix.Stat_t

		currentPath = filepath.Join(currentPath, seg)
		if !isLocalTo(currentPath, base) {
			return -1, &ErrEscapesBase{Base: currentPath, Subpath: seg}
		}

		// Trigger auto mount if it's an auto-mounted directory, ignore error if not a directory.
		// Notice the trailing slash is mandatory, see "automount" in openat(2) and open_by_handle_at(2).
		unix_noeintr.Fstatat(parentFD, seg+"/", &deviceStat, unix.AT_SYMLINK_NOFOLLOW)

		log.G(context.TODO()).Debugf("Opening path %s", currentPath)
		childFD, err = unix_noeintr.Openat(parentFD, seg, openFDFlags|unix.O_CLOEXEC, 0)
		if err != nil {
			return -1, &ErrNotAccessible{Path: currentPath, Cause: err}
		}

		err := unix_noeintr.Fstat(childFD, &deviceStat)
		if err != nil {
			return -1, fmt.Errorf("error running fstat on %s with %v", currentPath, err)
		}
		fileFmt := deviceStat.Mode & unix.S_IFMT
		if fileFmt == unix.S_IFLNK {
			return -1, fmt.Errorf("unexpected symlink found %s", currentPath)
		}

		// Close parentFD
		if err = unix_noeintr.Close(parentFD); err != nil {
			return -1, fmt.Errorf("closing fd for %q failed: %v", filepath.Dir(currentPath), err)
		}
		// Set child to new parent
		parentFD = childFD
		childFD = -1
	}

	// We made it to the end, return this fd, don't close it
	finalFD := parentFD
	parentFD = -1

	return finalFD, nil
}
