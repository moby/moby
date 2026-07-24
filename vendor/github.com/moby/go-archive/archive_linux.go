package archive

import (
	"archive/tar"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/moby/sys/userns"
	"golang.org/x/sys/unix"
)

func getWhiteoutConverter(format WhiteoutFormat) tarWhiteoutConverter {
	if format == OverlayWhiteoutFormat {
		return newOverlayWhiteoutConverter()
	}
	return nil
}

type overlayWhiteoutConverter struct {
	opaqueXattr string
}

func newOverlayWhiteoutConverter() overlayWhiteoutConverter {
	opaqueXattr := "trusted.overlay.opaque"
	if userns.RunningInUserNS() {
		opaqueXattr = "user.overlay.opaque"
	}
	return overlayWhiteoutConverter{
		opaqueXattr: opaqueXattr,
	}
}

func (c overlayWhiteoutConverter) ConvertWrite(hdr *tar.Header, filePath string, fi os.FileInfo) (wo *tar.Header, _ error) {
	// convert whiteouts to AUFS format
	if fi.Mode()&os.ModeCharDevice != 0 && hdr.Devmajor == 0 && hdr.Devminor == 0 {
		// we just rename the file and make it normal
		dir, filename := path.Split(hdr.Name)
		hdr.Name = path.Join(dir, WhiteoutPrefix+filename)
		hdr.Mode = 0o600
		hdr.Typeflag = tar.TypeReg
		hdr.Size = 0
	}

	if !fi.IsDir() {
		// FIXME(thaJeztah): return a sentinel error instead of nil, nil
		return nil, nil
	}

	// convert opaque dirs to AUFS format by writing an empty file with the prefix
	opaque, err := lgetxattr(filePath, c.opaqueXattr)
	if err != nil {
		return nil, err
	}
	if len(opaque) != 1 || opaque[0] != 'y' {
		// FIXME(thaJeztah): return a sentinel error instead of nil, nil
		return nil, nil
	}
	delete(hdr.PAXRecords, paxSchilyXattr+c.opaqueXattr)

	// create a header for the whiteout file
	// it should inherit some properties from the parent, but be a regular file
	return &tar.Header{
		Typeflag:   tar.TypeReg,
		Mode:       hdr.Mode & int64(os.ModePerm),
		Name:       path.Join(hdr.Name, WhiteoutOpaqueDir), // #nosec G305 -- An archive is being created, not extracted.
		Size:       0,
		Uid:        hdr.Uid,
		Uname:      hdr.Uname,
		Gid:        hdr.Gid,
		Gname:      hdr.Gname,
		AccessTime: hdr.AccessTime,
		ChangeTime: hdr.ChangeTime,
	}, nil
}

func (c overlayWhiteoutConverter) ConvertRead(hdr *tar.Header, filePath string) (bool, error) {
	base := filepath.Base(filePath)
	dir := filepath.Dir(filePath)

	switch base {
	case WhiteoutPrefix, WhiteoutPrefix + ".", WhiteoutPrefix + "..":
		return false, fmt.Errorf("invalid whiteout entry %q", hdr.Name)

	case WhiteoutOpaqueDir:
		// If a directory is marked as opaque by the AUFS special file, we need to translate that to overlay.
		if err := unix.Setxattr(dir, c.opaqueXattr, []byte{'y'}, 0); err != nil {
			return false, fmt.Errorf("setxattr('%s', %s=y): %w", dir, c.opaqueXattr, err)
		}
		// Don't write the whiteout file itself.
		return false, nil

	default:
		originalBase, ok := strings.CutPrefix(base, WhiteoutPrefix)
		if !ok {
			// Regular file.
			return true, nil
		}
		// If a file was deleted, and we are using overlay, we need to create a character device.
		originalPath := filepath.Join(dir, originalBase)
		if err := unix.Mknod(originalPath, unix.S_IFCHR, 0); err != nil {
			return false, fmt.Errorf("failed to mknod('%s', S_IFCHR, 0): %w", originalPath, err)
		}

		// Header IDs have already been remapped. Optimize the common non-remapped
		// root-owned (0:0) case by assuming the created whiteout has the expected
		// ownership, rather than comparing against the effective UID/GID or stat'ing
		// the created node to verify it.
		if hdr.Uid != 0 || hdr.Gid != 0 {
			// TODO(thaJeztah): Revisit whether whiteout ownership needs to be preserved.
			//
			// This was added in the original overlay whiteout implementation:
			// https://github.com/moby/moby/pull/18560 / https://github.com/moby/moby/pull/22126
			//
			// OverlayFS documents whiteouts in terms of a character device with device
			// number 0:0, not ownership: https://docs.kernel.org/filesystems/overlayfs.html#whiteouts-and-opaque-directories
			//
			// If ownership is not required, this Lchown can be removed to avoid the remaining TOCTOU window.
			if err := os.Lchown(originalPath, hdr.Uid, hdr.Gid); err != nil {
				return false, err
			}
		}

		// Don't write the whiteout file itself.
		return false, nil
	}
}
