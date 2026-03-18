package fstype

import "golang.org/x/sys/unix"

// getFSMagic returns the filesystem id given the path.
func getFSMagic(rootpath string) (FsMagic, error) {
	var buf unix.Statfs_t
	if err := unix.Statfs(rootpath, &buf); err != nil {
		return 0, err
	}
	return FsMagic(buf.Type), nil
}
