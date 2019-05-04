// +build linux,cgo

package loopback // import "github.com/docker/docker/pkg/loopback"

import (
	"bufio"
	"fmt"
	"math/bits"
	"os"
	"strconv"
	"syscall"

	"github.com/docker/docker/pkg/system"
)

const (
	loopMajorDev = 7 // see /usr/include/linux/major.h
	loopControl = "/dev/loop-control"
	loopFormat = "/dev/loop%d"
	loopZero = "/dev/loop0"
	sysfsModuleFormat = "/sys/module/loop/parameters/%s"
)

type attachErrorState = int

const (
	attachErrorStateNextFree = attachErrorState(iota)
	attachErrorStateMknod // Only occurs when creating a new loop device
	attachErrorStateStat
	attachErrorStateModeCheck
	attachErrorStateOpenBlock
	attachErrorStateAttachFd
)

type attachError struct {
	atState attachErrorState
	underlying error
}

func (attachErr *attachError) Error() string {
	return attachErr.underlying.Error()
}

func (attachErr *attachError) Underlying() error {
	return attachErr.underlying
}

type loopModuleContext interface {
	performPathStat(path string) (os.FileInfo, error)
	getNextFreeDeviceIndex() (int, error)
	getBaseDeviceNodeStat() (*syscall.Stat_t, error)
	performMknod(path string, mode uint32, dev int) error
	openDeviceFile(path string) (*os.File, error)
	setLoopFileFd(loopFile *os.File, sparseFile *os.File) error
}

func getNextFreeDeviceIndexViaLoopControl() (int, error) {
	f, err := os.OpenFile("/dev/loop-control", os.O_RDONLY, 0644)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	index, err := ioctlLoopCtlGetFree(f.Fd())
	if index < 0 {
		index = 0
	}
	return index, err
}

// getBaseDeviceNodeStatVfs inspects /dev/loop0 to collect uid,gid, and mode for
// the loop0 device on the system. If it does not exist we assume 0,0,0660 for
// the stat data (the defaults at least for Ubuntu 18.10).
//
// Stolen from daemon/devmapper/graphdriver/devmapper/devmapper_test.go.
func getBaseDeviceNodeStatVfs(ctx loopModuleContext) (*syscall.Stat_t, error) {
	loop0, err := ctx.performPathStat(loopZero)
	if err != nil {
		if os.IsNotExist(err) {
			return &syscall.Stat_t{
				Uid:  0,
				Gid:  0,
				Mode: 0660,
			}, nil
		}
		return nil, err
	}
	return loop0.Sys().(*syscall.Stat_t), nil
}

func getPartitionShift() (uint, error) {
	fp, err := os.Open(fmt.Sprintf(sysfsModuleFormat, "max_part"))
	if err != nil {
		// This parameter is expected to exist for the forseseeable future
		// but it wouldn't hurt to handle the case where it's missing.
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	defer fp.Close()

	scanner := bufio.NewScanner(fp)
	scanner.Scan()
	// io.EOF isn't treated as an error by scanner.Err()
	if err = scanner.Err(); err != nil {
		return 0, err
	}

	maxPart, err := strconv.ParseUint(scanner.Text(), 10, 0)
	if err != nil {
		return 0, err
	}
	// see drivers/block/loop.c in the Linux kernel sources
	// part_shift = fls(max_part) as set at module init time,
	// i.e. part_shift is the offset of the most significant
	// set bit of max_part as passed as a module parameter.
	return uint(bits.Len(uint(maxPart))), nil
}

func getMknodDeviceNumber(index int) (int, error) {
	partShift, err := getPartitionShift()
	if err != nil {
		return 0, err
	}
	minorDev := int64(index << partShift)
	return int(system.Mkdev(loopMajorDev, minorDev)), nil
}

func directIndexMknod(ctx loopModuleContext, index int) (os.FileInfo, error) {
	loopPath := fmt.Sprintf(loopFormat, index)
	// If the file already exists we don't need to create it
	if incumbentStat, err := ctx.performPathStat(loopPath); err == nil {
		return incumbentStat, nil
	}

	baseStats, err := ctx.getBaseDeviceNodeStat()
	if err != nil {
		return nil, err
	}

	deviceNumber, err := getMknodDeviceNumber(index)
	if err != nil {
		return nil, err
	}

	if err = ctx.performMknod(loopPath, uint32(baseStats.Mode|syscall.S_IFBLK), deviceNumber); err != nil {
		// If the mknod call failed because it already exists, we're fine
		if asErrno, ok := err.(syscall.Errno); !ok || asErrno != syscall.EEXIST {
			return nil, err
		}
	}
	return ctx.performPathStat(loopPath)
}

func openDeviceFileVfs(path string) (*os.File, error) {
	// OpenFile adds O_CLOEXEC
	return os.OpenFile(path, os.O_RDWR, 0644)
}

func setLoopFileFdIoctl(loopFile *os.File, sparseFile *os.File) error {
	return ioctlLoopSetFd(loopFile.Fd(), sparseFile.Fd())
}

// Returns:
// 1. The open loop device file, if applicable
// 2. The index of the loop device file that was created, or -1 if
//    no device file was created.
// 3. The error that occurred, or nil if no error occurred.
//    Note that err.atState == attachErrorStateMknod implies that the index
//    of the loop device is >= 0.
func attachToNextAvailableDevice(ctx loopModuleContext, sparseFile *os.File) (loopFile *os.File, createdIndex int, err *attachError) {
	createdIndex = -1
	index, underlying := ctx.getNextFreeDeviceIndex()
	if underlying != nil {
		err = &attachError{
			atState: attachErrorStateNextFree,
			underlying: underlying,
		}
		return
	}

	target := fmt.Sprintf(loopFormat, index)
	fi, underlying := ctx.performPathStat(target)
	if underlying != nil && os.IsNotExist(underlying) {
		createdIndex = index
		fi, underlying = directIndexMknod(ctx, index)
	}
	if underlying != nil {
		if createdIndex >= 0 {
			err = &attachError{
				atState: attachErrorStateMknod,
				underlying: underlying,
			}
		} else {
			err = &attachError{
				atState: attachErrorStateStat,
				underlying: underlying,
			}
		}
		return
	}

	// If, for some reason, we end up with a non-device file, we can't use it
	// and have to bail out now.
	if fi.Mode()&os.ModeDevice != os.ModeDevice {
		err = &attachError{
			atState: attachErrorStateModeCheck,
			underlying: syscall.EINVAL,
		}
		return
	}

	// OpenFile adds O_CLOEXEC
	loopFile, underlying = ctx.openDeviceFile(target)
	if underlying != nil {
		err = &attachError{
			atState: attachErrorStateOpenBlock,
			underlying: underlying,
		}
		return
	}

	if underlying = ctx.setLoopFileFd(loopFile, sparseFile); underlying != nil {
		loopFile.Close()
		loopFile = nil
		err = &attachError{
			atState: attachErrorStateAttachFd,
			underlying: underlying,
		}
	}

	return
}

type concreteLoopModuleContext struct {}

func (ctx *concreteLoopModuleContext) performPathStat(path string) (os.FileInfo, error) {
	return os.Stat(path)
}

func (ctx *concreteLoopModuleContext) getNextFreeDeviceIndex() (int, error) {
	return getNextFreeDeviceIndexViaLoopControl()
}

func (ctx *concreteLoopModuleContext) getBaseDeviceNodeStat() (*syscall.Stat_t, error) {
	return getBaseDeviceNodeStatVfs(ctx)
}

func (ctx *concreteLoopModuleContext) performMknod(path string, mode uint32, dev int) error {
	return system.Mknod(path, mode, dev)
}

func (ctx *concreteLoopModuleContext) openDeviceFile(path string) (*os.File, error) {
	return openDeviceFileVfs(path)
}

func (ctx *concreteLoopModuleContext) setLoopFileFd(loopFile *os.File, sparseFile *os.File) error {
	return setLoopFileFdIoctl(loopFile, sparseFile)
}