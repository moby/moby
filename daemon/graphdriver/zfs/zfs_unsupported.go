// +build !linux,!freebsd

package zfs // import "github.com/docker/docker/daemon/graphdriver/zfs"

func checkRootdirFs(rootdir string) error {
	return nil
}

func getMountpoint(id string) string {
	return id
}
