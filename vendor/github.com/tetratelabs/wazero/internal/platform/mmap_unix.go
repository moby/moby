//go:build (linux || darwin || freebsd || netbsd || dragonfly || solaris) && !tinygo

package platform

import (
	"syscall"
)

const (
	mmapProtAMD64 = syscall.PROT_READ | syscall.PROT_WRITE | syscall.PROT_EXEC
	mmapProtARM64 = syscall.PROT_READ | syscall.PROT_WRITE
)

func munmapCodeSegment(code []byte) error {
	return syscall.Munmap(code)
}

// mmapCodeSegmentAMD64 gives all read-write-exec permission to the mmap region
// to enter the function. Otherwise, segmentation fault exception is raised.
func mmapCodeSegmentAMD64(size int) ([]byte, error) {
	// The region must be RWX: RW for writing native codes, X for executing the region.
	return mmapCodeSegment(size, mmapProtAMD64)
}

// mmapCodeSegmentARM64 cannot give all read-write-exec permission to the mmap region.
// Otherwise, the mmap systemcall would raise an error. Here we give read-write
// to the region so that we can write contents at call-sites. Callers are responsible to
// execute MprotectRX on the returned buffer.
func mmapCodeSegmentARM64(size int) ([]byte, error) {
	// The region must be RW: RW for writing native codes.
	return mmapCodeSegment(size, mmapProtARM64)
}
