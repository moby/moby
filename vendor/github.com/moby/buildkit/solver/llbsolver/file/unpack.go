package file

import (
	"archive/tar"
	"os"
	"time"

	"github.com/containerd/continuity/fs"
	"github.com/moby/go-archive"
	"github.com/moby/go-archive/chrootarchive"
	"github.com/moby/go-archive/compression"
	"github.com/moby/sys/user"
	copy "github.com/tonistiigi/fsutil/copy"
)

func unpack(srcRoot string, src string, destRoot string, dest string, ch copy.Chowner, u *copy.User, tm *time.Time, idmap *user.IdentityMapping) (bool, error) {
	src, err := fs.RootPath(srcRoot, src)
	if err != nil {
		return false, err
	}
	if !isArchivePath(src) {
		return false, nil
	}

	dest, err = fs.RootPath(destRoot, dest)
	if err != nil {
		return false, err
	}
	if _, err := copy.MkdirAll(dest, 0755, ch, tm); err != nil {
		return false, err
	}

	file, err := os.Open(src)
	if err != nil {
		return false, err
	}
	defer file.Close()

	opts := &archive.TarOptions{
		BestEffortXattrs: true,
	}
	if idmap != nil {
		opts.IDMap = *idmap
	}
	if u != nil {
		opts.ChownOpts = &archive.ChownOpts{
			UID: u.UID,
			GID: u.GID,
		}
	}
	return true, chrootarchive.Untar(file, dest, opts)
}

func isArchivePath(path string) bool {
	fi, err := os.Lstat(path)
	if err != nil {
		return false
	}
	if fi.Mode()&os.ModeType != 0 {
		return false
	}
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()
	rdr, err := compression.DecompressStream(file)
	if err != nil {
		return false
	}
	defer rdr.Close()
	r := tar.NewReader(rdr)
	_, err = r.Next()
	return err == nil
}
