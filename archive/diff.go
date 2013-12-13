package archive

import (
	"archive/tar"
	"github.com/dotcloud/docker/utils"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// Linux device nodes are a bit weird due to backwards compat with 16 bit device nodes
// The lower 8 bit is the lower 8 bit in the minor, the following 12 bits are the major,
// and then there is the top 12 bits of then minor
func mkdev(major int64, minor int64) uint32 {
	return uint32(((minor & 0xfff00) << 12) | ((major & 0xfff) << 8) | (minor & 0xff))
}
func timeToTimespec(time time.Time) (ts syscall.Timespec) {
	if time.IsZero() {
		// Return UTIME_OMIT special value
		ts.Sec = 0
		ts.Nsec = ((1 << 30) - 2)
		return
	}
	return syscall.NsecToTimespec(time.UnixNano())
}

// ApplyLayer parses a diff in the standard layer format from `layer`, and
// applies it to the directory `dest`.
func ApplyLayer(dest string, layer Archive) error {
	// We need to be able to set any perms
	oldmask := syscall.Umask(0)
	defer syscall.Umask(oldmask)

	tr := tar.NewReader(layer)

	var dirs []*tar.Header

	// Iterate through the files in the archive.
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			// end of tar archive
			break
		}
		if err != nil {
			return err
		}

		// Skip AUFS metadata dirs
		if strings.HasPrefix(hdr.Name, ".wh..wh.") {
			continue
		}

		path := filepath.Join(dest, hdr.Name)
		base := filepath.Base(path)
		if strings.HasPrefix(base, ".wh.") {
			originalBase := base[len(".wh."):]
			originalPath := filepath.Join(filepath.Dir(path), originalBase)
			if err := os.RemoveAll(originalPath); err != nil {
				return err
			}
		} else {
			// If path exits we almost always just want to remove and replace it
			// The only exception is when it is a directory *and* the file from
			// the layer is also a directory. Then we want to merge them (i.e.
			// just apply the metadata from the layer).
			hasDir := false
			if fi, err := os.Lstat(path); err == nil {
				if fi.IsDir() && hdr.Typeflag == tar.TypeDir {
					hasDir = true
				} else {
					if err := os.RemoveAll(path); err != nil {
						return err
					}
				}
			}

			switch hdr.Typeflag {
			case tar.TypeDir:
				if !hasDir {
					err = os.Mkdir(path, os.FileMode(hdr.Mode))
					if err != nil {
						return err
					}
				}
				dirs = append(dirs, hdr)

			case tar.TypeReg, tar.TypeRegA:
				// Source is regular file
				file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, os.FileMode(hdr.Mode))
				if err != nil {
					return err
				}
				if _, err := io.Copy(file, tr); err != nil {
					file.Close()
					return err
				}
				file.Close()

			case tar.TypeBlock, tar.TypeChar, tar.TypeFifo:
				mode := uint32(hdr.Mode & 07777)
				switch hdr.Typeflag {
				case tar.TypeBlock:
					mode |= syscall.S_IFBLK
				case tar.TypeChar:
					mode |= syscall.S_IFCHR
				case tar.TypeFifo:
					mode |= syscall.S_IFIFO
				}

				if err := syscall.Mknod(path, mode, int(mkdev(hdr.Devmajor, hdr.Devminor))); err != nil {
					return err
				}

			case tar.TypeLink:
				if err := os.Link(filepath.Join(dest, hdr.Linkname), path); err != nil {
					return err
				}

			case tar.TypeSymlink:
				if err := os.Symlink(hdr.Linkname, path); err != nil {
					return err
				}

			default:
				utils.Debugf("unhandled type %d\n", hdr.Typeflag)
			}

			if err = syscall.Lchown(path, hdr.Uid, hdr.Gid); err != nil {
				return err
			}

			// There is no LChmod, so ignore mode for symlink.  Also, this
			// must happen after chown, as that can modify the file mode
			if hdr.Typeflag != tar.TypeSymlink {
				err = syscall.Chmod(path, uint32(hdr.Mode&07777))
				if err != nil {
					return err
				}
			}

			// Directories must be handled at the end to avoid further
			// file creation in them to modify the mtime
			if hdr.Typeflag != tar.TypeDir {
				ts := []syscall.Timespec{timeToTimespec(hdr.AccessTime), timeToTimespec(hdr.ModTime)}
				// syscall.UtimesNano doesn't support a NOFOLLOW flag atm, and
				if hdr.Typeflag != tar.TypeSymlink {
					if err := syscall.UtimesNano(path, ts); err != nil {
						return err
					}
				} else {
					if err := LUtimesNano(path, ts); err != nil {
						return err
					}
				}
			}
		}
	}

	for _, hdr := range dirs {
		path := filepath.Join(dest, hdr.Name)
		ts := []syscall.Timespec{timeToTimespec(hdr.AccessTime), timeToTimespec(hdr.ModTime)}
		if err := syscall.UtimesNano(path, ts); err != nil {
			return err
		}
	}

	return nil
}
