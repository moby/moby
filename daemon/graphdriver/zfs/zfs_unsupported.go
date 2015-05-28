// +build !linux,!freebsd

package zfs

func checkRootdirFs(rootdir string) error {
	return nil
}
