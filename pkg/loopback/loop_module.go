// +build linux,cgo

package loopback // import "github.com/docker/docker/pkg/loopback"

import (
	"bufio"
	"fmt"
	"io"
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

type loopModuleContext interface {
	PerformPathStat(path string) (os.FileInfo, error)
	GetNextFreeDeviceIndex() (int, error)
	GetBaseDeviceNodeStat() (*syscall.Stat_t, error)
	OpenSysfsParameterFile(param string) (io.ReadCloser, error)
	GetMaxPartitionParameter() (uint, error)
	GetPartitionShift() (uint, error)
	GetMknodDeviceNumber(index int) (int, error)
	PerformMknod(path string, mode uint32, dev int) error
	MakeIndexNode(index int) (os.FileInfo, error)
}

func getNextFreeDeviceIndex() (int, error) {
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

// getBaseDeviceNodeStat inspects /dev/loop0 to collect uid,gid, and mode for
// the loop0 device on the system. If it does not exist we assume 0,0,0660 for
// the stat data (the defaults at least for Ubuntu 18.10).
//
// Stolen from daemon/devmapper/graphdriver/devmapper/devmapper_test.go.
func getBaseDeviceNodeStat(ctx loopModuleContext) (*syscall.Stat_t, error) {
	loop0, err := ctx.PerformPathStat(loopZero)
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

func openLoopModuleSysfsParameter(param string) (io.ReadCloser, error) {
	return os.Open(fmt.Sprintf(sysfsModuleFormat, param))
}

func getMaxPartitionParameter(ctx loopModuleContext) (uint, error) {
	fp, err := ctx.OpenSysfsParameterFile("max_part")
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
	return uint(maxPart), err
}

func getPartitionShift(ctx loopModuleContext) (uint, error) {
	maxPart, err := ctx.GetMaxPartitionParameter()
	if err != nil {
		return 0, err
	}
	// see drivers/block/loop.c in the Linux kernel sources
	// part_shift = fls(max_part) as set at module init time,
	// i.e. part_shift is the offset of the most significant
	// set bit of max_part as passed as a module parameter.
	return uint(bits.Len(maxPart)), nil
}

func directIndexMknod(ctx loopModuleContext, index int) (os.FileInfo, error) {
	loopPath := fmt.Sprintf(loopFormat, index)
	// If the file already exists we don't need to create it
	if incumbentStat, err := ctx.PerformPathStat(loopPath); err == nil {
		return incumbentStat, nil
	}

	baseStats, err := ctx.GetBaseDeviceNodeStat()
	if err != nil {
		return nil, err
	}

	deviceNumber, err := ctx.GetMknodDeviceNumber(index)
	if err != nil {
		return nil, err
	}

	if err = ctx.PerformMknod(loopPath, uint32(baseStats.Mode|syscall.S_IFBLK), deviceNumber); err != nil {
		// If the mknod call failed because it already exists, we're fine
		if asErrno, ok := err.(syscall.Errno); !ok || asErrno != syscall.EEXIST {
			return nil, err
		}
	}
	return ctx.PerformPathStat(loopPath)
}

type concreteLoopModuleContext struct {}

func (ctx *concreteLoopModuleContext) PerformPathStat(path string) (os.FileInfo, error) {
	return os.Stat(path)
}

func (ctx *concreteLoopModuleContext) GetNextFreeDeviceIndex() (int, error) {
	return getNextFreeDeviceIndex()
}

func (ctx *concreteLoopModuleContext) GetBaseDeviceNodeStat() (*syscall.Stat_t, error) {
	return getBaseDeviceNodeStat(ctx)
}

func (ctx *concreteLoopModuleContext) OpenSysfsParameterFile(param string) (io.ReadCloser, error) {
	return openLoopModuleSysfsParameter(param)
}

func (ctx *concreteLoopModuleContext) GetMaxPartitionParameter() (uint, error) {
	return getMaxPartitionParameter(ctx)
}

func (ctx *concreteLoopModuleContext) GetPartitionShift() (uint, error) {
	return getPartitionShift(ctx)
}

func (ctx *concreteLoopModuleContext) GetMknodDeviceNumber(index int) (int, error) {
	partShift, err := ctx.GetPartitionShift()
	if err != nil {
		return 0, err
	}
	minorDev := int64(index << partShift)
	return int(system.Mkdev(loopMajorDev, minorDev)), nil
}

func (ctx *concreteLoopModuleContext) PerformMknod(path string, mode uint32, dev int) error {
	return system.Mknod(path, mode, dev)
}

func (ctx *concreteLoopModuleContext) MakeIndexNode(index int) (os.FileInfo, error) {
	return directIndexMknod(ctx, index)
}