package containerfs // import "github.com/moby/moby/pkg/containerfs"

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/moby/moby/pkg/archive"
	"github.com/moby/moby/pkg/idtools"
	"github.com/moby/moby/pkg/system"
	"github.com/sirupsen/logrus"
)

// TarFunc provides a function definition for a custom Tar function
type TarFunc func(string, *archive.TarOptions) (io.ReadCloser, error)

// UntarFunc provides a function definition for a custom Untar function
type UntarFunc func(io.Reader, string, *archive.TarOptions) error

// Archiver provides a similar implementation of the archive.Archiver package with the rootfs abstraction
type Archiver struct {
	SrcDriver Driver
	DstDriver Driver
	Tar       TarFunc
	Untar     UntarFunc
	IDMapping *idtools.IdentityMapping
}

// TarUntar is a convenience function which calls Tar and Untar, with the output of one piped into the other.
// If either Tar or Untar fails, TarUntar aborts and returns the error.
func (archiver *Archiver) TarUntar(src, dst string) error {
	logrus.Debugf("TarUntar(%s %s)", src, dst)
	tarArchive, err := archiver.Tar(src, &archive.TarOptions{Compression: archive.Uncompressed})
	if err != nil {
		return err
	}
	defer tarArchive.Close()
	options := &archive.TarOptions{
		UIDMaps: archiver.IDMapping.UIDs(),
		GIDMaps: archiver.IDMapping.GIDs(),
	}
	return archiver.Untar(tarArchive, dst, options)
}

// UntarPath untar a file from path to a destination, src is the source tar file path.
func (archiver *Archiver) UntarPath(src, dst string) error {
	tarArchive, err := archiver.SrcDriver.Open(src)
	if err != nil {
		return err
	}
	defer tarArchive.Close()
	options := &archive.TarOptions{
		UIDMaps: archiver.IDMapping.UIDs(),
		GIDMaps: archiver.IDMapping.GIDs(),
	}
	return archiver.Untar(tarArchive, dst, options)
}

// CopyWithTar creates a tar archive of filesystem path `src`, and
// unpacks it at filesystem path `dst`.
// The archive is streamed directly with fixed buffering and no
// intermediary disk IO.
func (archiver *Archiver) CopyWithTar(src, dst string) error {
	srcSt, err := archiver.SrcDriver.Stat(src)
	if err != nil {
		return err
	}
	if !srcSt.IsDir() {
		return archiver.CopyFileWithTar(src, dst)
	}

	// if this archiver is set up with ID mapping we need to create
	// the new destination directory with the remapped root UID/GID pair
	// as owner

	identity := idtools.Identity{UID: archiver.IDMapping.RootPair().UID, GID: archiver.IDMapping.RootPair().GID}

	// Create dst, copy src's content into it
	if err := idtools.MkdirAllAndChownNew(dst, 0755, identity); err != nil {
		return err
	}
	logrus.Debugf("Calling TarUntar(%s, %s)", src, dst)
	return archiver.TarUntar(src, dst)
}

// CopyFileWithTar emulates the behavior of the 'cp' command-line
// for a single file. It copies a regular file from path `src` to
// path `dst`, and preserves all its metadata.
func (archiver *Archiver) CopyFileWithTar(src, dst string) (retErr error) {
	logrus.Debugf("CopyFileWithTar(%s, %s)", src, dst)
	srcDriver := archiver.SrcDriver
	dstDriver := archiver.DstDriver

	srcSt, retErr := srcDriver.Stat(src)
	if retErr != nil {
		return retErr
	}

	if srcSt.IsDir() {
		return fmt.Errorf("Can't copy a directory")
	}

	// Clean up the trailing slash. This must be done in an operating
	// system specific manner.
	if dst[len(dst)-1] == dstDriver.Separator() {
		dst = dstDriver.Join(dst, srcDriver.Base(src))
	}

	// The original call was system.MkdirAll, which is just
	// os.MkdirAll on not-Windows and changed for Windows.
	if dstDriver.OS() == "windows" {
		// Now we are WCOW
		if err := system.MkdirAll(filepath.Dir(dst), 0700); err != nil {
			return err
		}
	} else {
		// We can just use the driver.MkdirAll function
		if err := dstDriver.MkdirAll(dstDriver.Dir(dst), 0700); err != nil {
			return err
		}
	}

	r, w := io.Pipe()
	errC := make(chan error, 1)

	go func() {
		defer close(errC)
		errC <- func() error {
			defer w.Close()

			srcF, err := srcDriver.Open(src)
			if err != nil {
				return err
			}
			defer srcF.Close()

			hdr, err := tar.FileInfoHeader(srcSt, "")
			if err != nil {
				return err
			}
			hdr.Format = tar.FormatPAX
			hdr.ModTime = hdr.ModTime.Truncate(time.Second)
			hdr.AccessTime = time.Time{}
			hdr.ChangeTime = time.Time{}
			hdr.Name = dstDriver.Base(dst)
			if dstDriver.OS() == "windows" {
				hdr.Mode = int64(chmodTarEntry(os.FileMode(hdr.Mode)))
			} else {
				hdr.Mode = int64(os.FileMode(hdr.Mode))
			}

			if err := remapIDs(archiver.IDMapping, hdr); err != nil {
				return err
			}

			tw := tar.NewWriter(w)
			defer tw.Close()
			if err := tw.WriteHeader(hdr); err != nil {
				return err
			}
			if _, err := io.Copy(tw, srcF); err != nil {
				return err
			}
			return nil
		}()
	}()
	defer func() {
		if err := <-errC; retErr == nil && err != nil {
			retErr = err
		}
	}()

	retErr = archiver.Untar(r, dstDriver.Dir(dst), nil)
	if retErr != nil {
		r.CloseWithError(retErr)
	}
	return retErr
}

// IdentityMapping returns the IdentityMapping of the archiver.
func (archiver *Archiver) IdentityMapping() *idtools.IdentityMapping {
	return archiver.IDMapping
}

func remapIDs(idMapping *idtools.IdentityMapping, hdr *tar.Header) error {
	ids, err := idMapping.ToHost(idtools.Identity{UID: hdr.Uid, GID: hdr.Gid})
	hdr.Uid, hdr.Gid = ids.UID, ids.GID
	return err
}

// chmodTarEntry is used to adjust the file permissions used in tar header based
// on the platform the archival is done.
func chmodTarEntry(perm os.FileMode) os.FileMode {
	// perm &= 0755 // this 0-ed out tar flags (like link, regular file, directory marker etc.)
	permPart := perm & os.ModePerm
	noPermPart := perm &^ os.ModePerm
	// Add the x bit: make everything +x from windows
	permPart |= 0111
	permPart &= 0755

	return noPermPart | permPart
}
