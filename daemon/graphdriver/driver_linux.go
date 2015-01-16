package graphdriver

import (
	"path"
	"syscall"
)

func GetFSMagic(rootpath string) (FsMagic, error) {
	var buf syscall.Statfs_t
	if err := syscall.Statfs(path.Dir(rootpath), &buf); err != nil {
		return 0, err
	}
	return FsMagic(buf.Type), nil
}
