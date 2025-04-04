package file

import (
	"archive/tar"
	"os"
	"time"

	"github.com/containerd/continuity/fs"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/chrootarchive"
	"github.com/docker/docker/pkg/idtools"
	"github.com/moby/sys/user"
	copy "github.com/tonistiigi/fsutil/copy"
)

func unpack(srcRoot string, src string, destRoot string, dest string, ch copy.Chowner, tm *time.Time, idmap *user.IdentityMapping) (bool, error) {
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
		// TODO: chrootarchive should be moved into moby/sys like idtools
		// was moved into moby/sys/user. This section can be removed
		// when chrootarchive accepts moby/sys/user arguments.
		for _, uid := range idmap.UIDMaps {
			opts.IDMap.UIDMaps = append(opts.IDMap.UIDMaps, idtools.IDMap{
				ContainerID: int(uid.ID),
				HostID:      int(uid.ParentID),
				Size:        int(uid.Count),
			})
		}
		for _, gid := range idmap.GIDMaps {
			opts.IDMap.GIDMaps = append(opts.IDMap.GIDMaps, idtools.IDMap{
				ContainerID: int(gid.ID),
				HostID:      int(gid.ParentID),
				Size:        int(gid.Count),
			})
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
	rdr, err := archive.DecompressStream(file)
	if err != nil {
		return false
	}
	defer rdr.Close()
	r := tar.NewReader(rdr)
	_, err = r.Next()
	return err == nil
}
