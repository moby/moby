// SPDX-License-Identifier: Apache-2.0
/*
 * Copyright (C) 2024-2025 Aleksa Sarai <cyphar@cyphar.com>
 * Copyright (C) 2024-2025 SUSE LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package pathrs

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/cyphar/filepath-securejoin/pathrs-lite"
	"golang.org/x/sys/unix"
)

// OpenInRoot opens the given path inside the root with the provided flags. It
// is effectively shorthand for [securejoin.OpenInRoot] followed by
// [securejoin.Reopen].
func OpenInRoot(root, subpath string, flags int) (*os.File, error) {
	handle, err := retryEAGAIN(func() (*os.File, error) {
		return pathrs.OpenInRoot(root, subpath)
	})
	if err != nil {
		return nil, err
	}
	defer handle.Close()

	return Reopen(handle, flags)
}

// CreateInRoot creates a new file inside a root (as well as any missing parent
// directories) and returns a handle to said file. This effectively has
// open(O_CREAT|O_NOFOLLOW) semantics. If you want the creation to use O_EXCL,
// include it in the passed flags. The fileMode argument uses unix.* mode bits,
// *not* os.FileMode.
func CreateInRoot(root, subpath string, flags int, fileMode uint32) (*os.File, error) {
	dir, filename := filepath.Split(subpath)
	if filepath.Join("/", filename) == "/" {
		return nil, fmt.Errorf("create in root subpath %q has bad trailing component %q", subpath, filename)
	}

	dirFd, err := MkdirAllInRootOpen(root, dir, 0o755)
	if err != nil {
		return nil, err
	}
	defer dirFd.Close()

	// We know that the filename does not have any "/" components, and that
	// dirFd is inside the root. O_NOFOLLOW will stop us from following
	// trailing symlinks, so this is safe to do. libpathrs's Root::create_file
	// works the same way.
	flags |= unix.O_CREAT | unix.O_NOFOLLOW
	fd, err := unix.Openat(int(dirFd.Fd()), filename, flags, fileMode)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), root+"/"+subpath), nil
}
