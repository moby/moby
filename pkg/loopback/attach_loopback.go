// +build linux,cgo

package loopback // import "github.com/docker/docker/pkg/loopback"

import (
	"bufio"
	"errors"
	"fmt"
	"math/bits"
	"os"
	"strconv"
	"syscall"

	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

// Loopback related errors
var (
	ErrAttachLoopbackDevice   = errors.New("loopback attach failed")
	ErrGetLoopbackBackingFile = errors.New("Unable to get loopback backing file")
	ErrSetCapacity            = errors.New("Unable set loopback capacity")
)

func stringToLoopName(src string) [LoNameSize]uint8 {
	var dst [LoNameSize]uint8
	copy(dst[:], src[:])
	return dst
}

func getNextFreeLoopbackIndex() (int, error) {
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

// getBaseLoopStats inspects /dev/loop0 to collect uid,gid, and mode for the
// loop0 device on the system. If it does not exist we assume 0,0,0660 for the
// stat data (the defaults at least for Ubuntu 18.10).
//
// Stolen from daemon/devmapper/graphdriver/devmapper/devmapper_test.go.
func getBaseLoopStats() (*syscall.Stat_t, error) {
	loop0, err := os.Stat("/dev/loop0")
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

func getMaxPartLoopParameter() (uint, error) {
	fp, err := os.Open("/sys/module/loop/parameters/max_part")
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

func getLoopPartShift() (uint, error) {
	maxPart, err := getMaxPartLoopParameter()
	if err != nil {
		return 0, err
	}
	// see drivers/block/loop.c in the Linux kernel sources
	// part_shift = fls(max_part) as set at module init time,
	// i.e. part_shift is the offset of the most significant
	// set bit of max_part as passed as a module parameter.
	return uint(bits.Len(maxPart)), nil
}

func getLoopMknodDeviceNumber(index int) (int, error) {
	const (
		// Derived from glibc gnu_dev_makedev
		majorShift = 8
		minorHighShift = 12
		minorHighMask = 0xfff00
		minorLowMask = 0x000ff
	)
	const loopMajorMask = 7 << majorShift // see /usr/include/linux/major.h

	partShift, err := getLoopPartShift()
	if err != nil {
		return 0, err
	}

	minor := index << partShift
	return ((minor & minorHighMask) << minorHighShift) | loopMajorMask | (minor & minorLowMask), nil
}

func directLoopMknod(index int) (os.FileInfo, error) {
	loopPath := fmt.Sprintf("/dev/loop%d", index)
	// If the file already exists we don't need to create it
	if incumbentStat, err := os.Stat(loopPath); err == nil {
		return incumbentStat, nil
	}

	baseStats, err := getBaseLoopStats()
	if err != nil {
		return nil, err
	}

	deviceNumber, err := getLoopMknodDeviceNumber(index)
	if err != nil {
		return nil, err
	}

	if err = syscall.Mknod(loopPath, uint32(baseStats.Mode|syscall.S_IFBLK), deviceNumber); err != nil {
		// If the mknod call failed because it already exists, we're fine
		if asErrno, ok := err.(syscall.Errno); !ok || asErrno != syscall.EEXIST {
			return nil, err
		}
	}
	return os.Stat(loopPath)
}

func openNextAvailableLoopback(startIndex int, sparseFile *os.File) (loopFile *os.File, err error) {
	// Start looking for a free /dev/loop from the startIndex
	for index := startIndex;; index++ {
		target := fmt.Sprintf("/dev/loop%d", index)
		fi, err := os.Stat(target)

		// Sometimes we don't have udev managing device nodes for us
		// (e.g. during unit testing inside of another Docker host), or
		// sometimes udev is slow and we managed to get here before it
		// creates the nodes for us. In both cases, since /dev/loop-control
		// advised us that this loop device was free, we'll just directly make
		// a device node for it.
		//
		// The worst case for udev would be that we manage to create the
		// device node before it does; that should merely result in a few more
		// error messages but otherwise shouldn't cause problems.
		if index == startIndex && err != nil && os.IsNotExist(err) {
			logrus.Warnf("Trying to forcibly create loopback device %s", target)
			fi, err = directLoopMknod(index)
		}
		if err != nil {
			if os.IsNotExist(err) {
				logrus.Error("There are no more loopback devices available.")
			}
			return nil, ErrAttachLoopbackDevice
		}

		if fi.Mode()&os.ModeDevice != os.ModeDevice {
			logrus.Errorf("Loopback device %s is not a block device.", target)
			continue
		}

		// OpenFile adds O_CLOEXEC
		loopFile, err = os.OpenFile(target, os.O_RDWR, 0644)
		if err != nil {
			logrus.Errorf("Error opening loopback device: %s", err)
			return nil, ErrAttachLoopbackDevice
		}

		// Try to attach to the loop file
		if err := ioctlLoopSetFd(loopFile.Fd(), sparseFile.Fd()); err != nil {
			loopFile.Close()

			// If the error is EBUSY, then try the next loopback
			if err != unix.EBUSY {
				logrus.Errorf("Cannot set up loopback device %s: %s", target, err)
				return nil, ErrAttachLoopbackDevice
			}

			// Otherwise, we keep going with the loop
			continue
		}
		// In case of success, we finished. Break the loop.
		break
	}

	// This can't happen, but let's be sure
	if loopFile == nil {
		logrus.Errorf("Unreachable code reached! Error attaching %s to a loopback device.", sparseFile.Name())
		return nil, ErrAttachLoopbackDevice
	}

	return loopFile, nil
}

// AttachLoopDevice attaches the given sparse file to the next
// available loopback device. It returns an opened *os.File.
func AttachLoopDevice(sparseName string) (loop *os.File, err error) {

	// Try to retrieve the next available loopback device via syscall.
	// If it fails, we discard error and start looping for a
	// loopback from index 0.
	startIndex, err := getNextFreeLoopbackIndex()
	if err != nil {
		logrus.Debugf("Error retrieving the next available loopback: %s", err)
	}

	// OpenFile adds O_CLOEXEC
	sparseFile, err := os.OpenFile(sparseName, os.O_RDWR, 0644)
	if err != nil {
		logrus.Errorf("Error opening sparse file %s: %s", sparseName, err)
		return nil, ErrAttachLoopbackDevice
	}
	defer sparseFile.Close()

	loopFile, err := openNextAvailableLoopback(startIndex, sparseFile)
	if err != nil {
		return nil, err
	}

	// Set the status of the loopback device
	loopInfo := &loopInfo64{
		loFileName: stringToLoopName(loopFile.Name()),
		loOffset:   0,
		loFlags:    LoFlagsAutoClear,
	}

	if err := ioctlLoopSetStatus64(loopFile.Fd(), loopInfo); err != nil {
		logrus.Errorf("Cannot set up loopback device info: %s", err)

		// If the call failed, then free the loopback device
		if err := ioctlLoopClrFd(loopFile.Fd()); err != nil {
			logrus.Error("Error while cleaning up the loopback device")
		}
		loopFile.Close()
		return nil, ErrAttachLoopbackDevice
	}

	return loopFile, nil
}
