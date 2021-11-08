// +build darwin freebsd openbsd solaris

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
	"io"
	"os"
	"syscall"

	"github.com/containerd/continuity/sysx"
	"github.com/pkg/errors"
)

func copyFileInfo(fi os.FileInfo, name string) error {
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
			return errors.Wrapf(err, "failed to chown %s", name)
		}
	}

	if (fi.Mode() & os.ModeSymlink) != os.ModeSymlink {
		if err := os.Chmod(name, fi.Mode()); err != nil {
			return errors.Wrapf(err, "failed to chmod %s", name)
		}
	}

	if err := utimesNano(name, StatAtime(st), StatMtime(st)); err != nil {
		return errors.Wrapf(err, "failed to utime %s", name)
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
		e := errors.Wrapf(err, "failed to list xattrs on %s", src)
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
			e := errors.Wrapf(err, "failed to get xattr %q on %s", xattr, src)
			if errorHandler != nil {
				if e = errorHandler(dst, src, xattr, e); e == nil {
					continue
				}
			}
			return e
		}
		if err := sysx.LSetxattr(dst, xattr, data, 0); err != nil {
			e := errors.Wrapf(err, "failed to set xattr %q on %s", xattr, dst)
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
