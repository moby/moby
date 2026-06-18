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
	"errors"
	"fmt"
	"os"
	"syscall"

	"github.com/containerd/continuity/sysx"
	"golang.org/x/sys/unix"
)

// maxCopyChunk is the maximum size passed to copy_file_range per call,
// avoiding int overflow on 32-bit architectures.
const maxCopyChunk = 1 << 30 // 1 GiB

// copyFile copies a file from source to target preserving sparse file holes.
//
// If the filesystem does not support SEEK_DATA/SEEK_HOLE, it falls back
// to a plain io.Copy.
func copyFile(target, source string) error {
	src, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("failed to open source %s: %w", source, err)
	}
	defer src.Close()

	fi, err := src.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat source %s: %w", source, err)
	}
	size := fi.Size()

	tgt, err := os.Create(target)
	if err != nil {
		return fmt.Errorf("failed to open target %s: %w", target, err)
	}
	defer tgt.Close()

	if err := tgt.Truncate(size); err != nil {
		return fmt.Errorf("failed to truncate target %s: %w", target, err)
	}

	srcFd := int(src.Fd())

	// Try a SEEK_DATA to check if the filesystem supports it.
	// If not, fall back to a plain copy.
	if _, err := unix.Seek(srcFd, 0, unix.SEEK_DATA); err != nil {
		// ENXIO means no data in the file at all. In other words it's entirely sparse.
		// The truncated target is already correct.
		if errors.Is(err, syscall.ENXIO) {
			return nil
		}

		if errors.Is(err, syscall.EOPNOTSUPP) || errors.Is(err, syscall.ENOTSUP) || errors.Is(err, syscall.EINVAL) {
			// Filesystem doesn't support SEEK_DATA/SEEK_HOLE. Fall back to a plain copy.
			src.Close()
			tgt.Close()
			return openAndCopyFile(target, source)
		}

		return fmt.Errorf("failed to seek data in source %s: %w", source, err)
	}

	// Copy data regions from source to target, skipping holes.
	var offset int64
	tgtFd := int(tgt.Fd())

	for offset < size {
		dataStart, err := unix.Seek(srcFd, offset, unix.SEEK_DATA)
		if err != nil {
			// No more data past offset. Remainder of file is a hole.
			if errors.Is(err, syscall.ENXIO) {
				break
			}
			return fmt.Errorf("SEEK_DATA failed at offset %d: %w", offset, err)
		}

		// Find the end of this data region (start of next hole).
		holeStart, err := unix.Seek(srcFd, dataStart, unix.SEEK_HOLE)
		if err != nil {
			// ENXIO shouldn't happen after a successful SEEK_DATA, but
			// treat it as data extending to end of file.
			if errors.Is(err, syscall.ENXIO) {
				holeStart = size
			} else {
				return fmt.Errorf("SEEK_HOLE failed at offset %d: %w", dataStart, err)
			}
		}

		// Copy the data region [dataStart, holeStart).
		srcOff := dataStart
		tgtOff := dataStart
		remain := holeStart - dataStart

		for remain > 0 {
			chunk := remain
			if chunk > maxCopyChunk {
				chunk = maxCopyChunk
			}

			n, err := unix.CopyFileRange(srcFd, &srcOff, tgtFd, &tgtOff, int(chunk), 0)
			if err != nil {
				// Fall back to a plain copy if copy_file_range is not supported
				// across the source and target filesystems.
				if errors.Is(err, syscall.EXDEV) || errors.Is(err, syscall.ENOSYS) || errors.Is(err, syscall.EOPNOTSUPP) {
					src.Close()
					tgt.Close()
					return openAndCopyFile(target, source)
				}
				return fmt.Errorf("copy_file_range failed: %w", err)
			}
			if n == 0 {
				return fmt.Errorf("copy_file_range returned 0 with %d bytes remaining", remain)
			}
			remain -= int64(n)
		}

		offset = holeStart
	}

	if err := tgt.Sync(); err != nil {
		return fmt.Errorf("failed to sync target %s: %w", target, err)
	}

	return nil
}

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

	timespec := []unix.Timespec{
		unix.NsecToTimespec(syscall.TimespecToNsec(StatAtime(st))),
		unix.NsecToTimespec(syscall.TimespecToNsec(StatMtime(st))),
	}
	if err := unix.UtimesNanoAt(unix.AT_FDCWD, name, timespec, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return fmt.Errorf("failed to utime %s: %w", name, err)
	}

	return nil
}

func copyXAttrs(dst, src string, excludes map[string]struct{}, errorHandler XAttrErrorHandler) error {
	xattrKeys, err := sysx.LListxattr(src)
	if err != nil {
		if errors.Is(err, unix.ENOTSUP) {
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
