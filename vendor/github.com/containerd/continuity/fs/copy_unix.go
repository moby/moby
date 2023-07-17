//go:build darwin || freebsd || openbsd || netbsd || solaris
// +build darwin freebsd openbsd netbsd solaris

/*
   Copyright The containerd Authors.

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

package fs

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"syscall"

	"github.com/containerd/continuity/sysx"
)

func copyFileInfo(fi os.FileInfo, src, name string) error {
	st := fi.Sys().(*syscall.Stat_t)
	if err := os.Lchown(name, int(st.Uid), int(st.Gid)); err != nil {
		if os.IsPermission(err) {
			// Normally if uid/gid are the same this would be a no-op, but some
			// filesystems may still return EPERM... for instance NFS does this.
			// In such a case, this is not an error.
			if dstStat, err2 := os.Lstat(name); err2 == nil {
				st2 := dstStat.Sys().(*syscall.Stat_t)
				if st.Uid == st2.Uid && st.Gid == st2.Gid {
					err = nil
				}
			}
		}
		if err != nil {
			return fmt.Errorf("failed to chown %s: %w", name, err)
		}
	}

	if (fi.Mode() & os.ModeSymlink) != os.ModeSymlink {
		if err := os.Chmod(name, fi.Mode()); err != nil {
			return fmt.Errorf("failed to chmod %s: %w", name, err)
		}
	}

	if err := utimesNano(name, StatAtime(st), StatMtime(st)); err != nil {
		return fmt.Errorf("failed to utime %s: %w", name, err)
	}

	return nil
}

func copyFileContent(dst, src *os.File) error {
	buf := bufferPool.Get().(*[]byte)
	_, err := io.CopyBuffer(dst, src, *buf)
	bufferPool.Put(buf)

	return err
}

func copyXAttrs(dst, src string, excludes map[string]struct{}, errorHandler XAttrErrorHandler) error {
	xattrKeys, err := sysx.LListxattr(src)
	if err != nil {
		if os.IsPermission(err) && runtime.GOOS == "darwin" {
			// On darwin, character devices do not permit listing xattrs
			return nil
		}
		e := fmt.Errorf("failed to list xattrs on %s: %w", src, err)
		if errorHandler != nil {
			e = errorHandler(dst, src, "", e)
		}
		return e
	}
	for _, xattr := range xattrKeys {
		if _, exclude := excludes[xattr]; exclude {
			continue
		}
		data, err := sysx.LGetxattr(src, xattr)
		if err != nil {
			e := fmt.Errorf("failed to get xattr %q on %s: %w", xattr, src, err)
			if errorHandler != nil {
				if e = errorHandler(dst, src, xattr, e); e == nil {
					continue
				}
			}
			return e
		}
		if err := sysx.LSetxattr(dst, xattr, data, 0); err != nil {
			e := fmt.Errorf("failed to set xattr %q on %s: %w", xattr, dst, err)
			if errorHandler != nil {
				if e = errorHandler(dst, src, xattr, e); e == nil {
					continue
				}
			}
			return e
		}
	}

	return nil
}
