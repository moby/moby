//go:build linux
// +build linux

package loopback // import "github.com/docker/docker/pkg/loopback"

import "golang.org/x/sys/unix"

// IOCTL consts
const (
	LoopSetFd       = unix.LOOP_SET_FD
	LoopCtlGetFree  = unix.LOOP_CTL_GET_FREE
	LoopGetStatus64 = unix.LOOP_GET_STATUS64
	LoopSetStatus64 = unix.LOOP_SET_STATUS64
	LoopClrFd       = unix.LOOP_CLR_FD
	LoopSetCapacity = unix.LOOP_SET_CAPACITY
)

// LOOP consts.
const (
	LoFlagsAutoClear = unix.LO_FLAGS_AUTOCLEAR
	LoFlagsReadOnly  = unix.LO_FLAGS_READ_ONLY
	LoFlagsPartScan  = unix.LO_FLAGS_PARTSCAN
	LoKeySize        = unix.LO_KEY_SIZE
	LoNameSize       = unix.LO_NAME_SIZE
)
