// +build solaris,cgo

package graphdriver

/*
#include <sys/statvfs.h>
#include <stdlib.h>

static inline struct statvfs *getstatfs(char *s) {
        struct statvfs *buf;
        int err;
        buf = (struct statvfs *)malloc(sizeof(struct statvfs));
        err = statvfs(s, buf);
        return buf;
}
*/
import "C"
import (
	"path/filepath"
	"unsafe"

	"github.com/Sirupsen/logrus"
)

const (
	// FsMagicZfs filesystem id for Zfs
	FsMagicZfs = FsMagic(0x2fc12fc1)
)

var (
	// Slice of drivers that should be used in an order
	priority = []string{
		"zfs",
	}

	// FsNames maps filesystem id to name of the filesystem.
	FsNames = map[FsMagic]string{
		FsMagicZfs: "zfs",
	}
)

// GetFSMagic returns the filesystem id given the path.
func GetFSMagic(rootpath string) (FsMagic, error) {
	return 0, nil
}

// Mounted checks if the given path is mounted as the fs type
//Solaris supports only ZFS for now
func Mounted(fsType FsMagic, mountPath string) (bool, error) {

	cs := C.CString(filepath.Dir(mountPath))
	buf := C.getstatfs(cs)

	// on Solaris buf.f_basetype contains ['z', 'f', 's', 0 ... ]
	if (buf.f_basetype[0] != 122) || (buf.f_basetype[1] != 102) || (buf.f_basetype[2] != 115) ||
		(buf.f_basetype[3] != 0) {
		logrus.Debugf("[zfs] no zfs dataset found for rootdir '%s'", mountPath)
		C.free(unsafe.Pointer(buf))
		return false, ErrPrerequisites
	}

	C.free(unsafe.Pointer(buf))
	C.free(unsafe.Pointer(cs))
	return true, nil
}
