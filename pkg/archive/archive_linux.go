package archive // import "github.com/docker/docker/pkg/archive"

import (
	"archive/tar"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/pkg/system"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

func getWhiteoutConverter(format WhiteoutFormat, inUserNS bool) (tarWhiteoutConverter, error) {
	if format == OverlayWhiteoutFormat {
		if inUserNS {
			return nil, errors.New("specifying OverlayWhiteoutFormat is not allowed in userns")
		}
		return overlayWhiteoutConverter{}, nil
	}
	return nil, nil
}

type overlayWhiteoutConverter struct {
}

func (overlayWhiteoutConverter) ConvertWrite(hdr *tar.Header, path string, fi os.FileInfo) (wo *tar.Header, err error) {
	// convert whiteouts to AUFS format
	if fi.Mode()&os.ModeCharDevice != 0 && hdr.Devmajor == 0 && hdr.Devminor == 0 {
		// we just rename the file and make it normal
		dir, filename := filepath.Split(hdr.Name)
		hdr.Name = filepath.Join(dir, WhiteoutPrefix+filename)
		hdr.Mode = 0600
		hdr.Typeflag = tar.TypeReg
		hdr.Size = 0
	}

	if fi.Mode()&os.ModeDir != 0 {
		// convert opaque dirs to AUFS format by writing an empty file with the prefix
		opaque, err := system.Lgetxattr(path, "trusted.overlay.opaque")
		if err != nil {
			return nil, err
		}
		if len(opaque) == 1 && opaque[0] == 'y' {
			if hdr.Xattrs != nil {
				delete(hdr.Xattrs, "trusted.overlay.opaque")
			}

			// create a header for the whiteout file
			// it should inherit some properties from the parent, but be a regular file
			wo = &tar.Header{
				Typeflag:   tar.TypeReg,
				Mode:       hdr.Mode & int64(os.ModePerm),
				Name:       filepath.Join(hdr.Name, WhiteoutOpaqueDir),
				Size:       0,
				Uid:        hdr.Uid,
				Uname:      hdr.Uname,
				Gid:        hdr.Gid,
				Gname:      hdr.Gname,
				AccessTime: hdr.AccessTime,
				ChangeTime: hdr.ChangeTime,
			}
		}
	}

	return
}

func (c overlayWhiteoutConverter) ConvertRead(hdr *tar.Header, path string) (bool, error) {
	base := filepath.Base(path)
	dir := filepath.Dir(path)

	// if a directory is marked as opaque by the AUFS special file, we need to translate that to overlay
	if base == WhiteoutOpaqueDir {
		err := unix.Setxattr(dir, "trusted.overlay.opaque", []byte{'y'}, 0)
		if err != nil {
			return false, errors.Wrapf(err, "setxattr(%q, trusted.overlay.opaque=y)", dir)
		}
		// don't write the file itself
		return false, err
	}

	// if a file was deleted and we are using overlay, we need to create a character device
	if strings.HasPrefix(base, WhiteoutPrefix) {
		originalBase := base[len(WhiteoutPrefix):]
		originalPath := filepath.Join(dir, originalBase)

		if err := unix.Mknod(originalPath, unix.S_IFCHR, 0); err != nil {
			return false, errors.Wrapf(err, "failed to mknod(%q, S_IFCHR, 0)", originalPath)
		}
		if err := os.Chown(originalPath, hdr.Uid, hdr.Gid); err != nil {
			return false, err
		}

		// don't write the file itself
		return false, nil
	}

	return true, nil
}
