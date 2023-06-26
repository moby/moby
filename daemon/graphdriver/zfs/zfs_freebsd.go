package zfs // import "github.com/docker/docker/daemon/graphdriver/zfs"

import (
	"strings"

	"github.com/docker/docker/daemon/graphdriver"
	"github.com/containerd/containerd/log"
	"golang.org/x/sys/unix"
)

func checkRootdirFs(rootdir string) error {
	var buf unix.Statfs_t
	if err := unix.Statfs(rootdir, &buf); err != nil {
		return err
	}

	// on FreeBSD buf.Fstypename contains ['z', 'f', 's', 0 ... ]
	if (buf.Fstypename[0] != 122) || (buf.Fstypename[1] != 102) || (buf.Fstypename[2] != 115) || (buf.Fstypename[3] != 0) {
		log.G(ctx).WithField("storage-driver", "zfs").Debugf("no zfs dataset found for rootdir '%s'", rootdir)
		return graphdriver.ErrPrerequisites
	}

	return nil
}

const maxlen = 12

func getMountpoint(id string) string {
	id, suffix, _ := strings.Cut(id, "-")
	id = id[:maxlen]
	if suffix != "" {
		// preserve filesystem suffix.
		id += "-" + suffix
	}
	return id
}
