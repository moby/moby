// +build cgo

package devices

//#include <sys/mkdev.h>
import "C"

func getmajor(dev uint64) uint64 {
	return uint64(C.major(C.dev_t(dev)))
}

func getminor(dev uint64) uint64 {
	return uint64(C.minor(C.dev_t(dev)))
}

func makedev(major int, minor int) int {
	return int(C.makedev(C.major_t(major), C.minor_t(minor)))
}
