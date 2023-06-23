//go:build linux
// +build linux

package loopback // import "github.com/docker/docker/pkg/loopback"

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/containerd/containerd/log"
	"golang.org/x/sys/unix"
)

// Loopback related errors
var (
	ErrAttachLoopbackDevice   = errors.New("loopback attach failed")
	ErrGetLoopbackBackingFile = errors.New("unable to get loopback backing file")
	ErrSetCapacity            = errors.New("unable set loopback capacity")
)

func stringToLoopName(src string) [unix.LO_NAME_SIZE]uint8 {
	var dst [unix.LO_NAME_SIZE]uint8
	copy(dst[:], src[:])
	return dst
}

func getNextFreeLoopbackIndex() (int, error) {
	f, err := os.OpenFile("/dev/loop-control", os.O_RDONLY, 0644)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	return unix.IoctlRetInt(int(f.Fd()), unix.LOOP_CTL_GET_FREE)
}

func openNextAvailableLoopback(index int, sparseFile *os.File) (loopFile *os.File, err error) {
	// Start looking for a free /dev/loop
	for {
		target := fmt.Sprintf("/dev/loop%d", index)
		index++

		fi, err := os.Stat(target)
		if err != nil {
			if os.IsNotExist(err) {
				log.G(context.TODO()).Error("There are no more loopback devices available.")
			}
			return nil, ErrAttachLoopbackDevice
		}

		if fi.Mode()&os.ModeDevice != os.ModeDevice {
			log.G(context.TODO()).Errorf("Loopback device %s is not a block device.", target)
			continue
		}

		// OpenFile adds O_CLOEXEC
		loopFile, err = os.OpenFile(target, os.O_RDWR, 0644)
		if err != nil {
			log.G(context.TODO()).Errorf("Error opening loopback device: %s", err)
			return nil, ErrAttachLoopbackDevice
		}

		// Try to attach to the loop file
		if err = unix.IoctlSetInt(int(loopFile.Fd()), unix.LOOP_SET_FD, int(sparseFile.Fd())); err != nil {
			loopFile.Close()

			// If the error is EBUSY, then try the next loopback
			if err != unix.EBUSY {
				log.G(context.TODO()).Errorf("Cannot set up loopback device %s: %s", target, err)
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
		log.G(context.TODO()).Errorf("Unreachable code reached! Error attaching %s to a loopback device.", sparseFile.Name())
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
		log.G(context.TODO()).Debugf("Error retrieving the next available loopback: %s", err)
	}

	// OpenFile adds O_CLOEXEC
	sparseFile, err := os.OpenFile(sparseName, os.O_RDWR, 0644)
	if err != nil {
		log.G(context.TODO()).Errorf("Error opening sparse file %s: %s", sparseName, err)
		return nil, ErrAttachLoopbackDevice
	}
	defer sparseFile.Close()

	loopFile, err := openNextAvailableLoopback(startIndex, sparseFile)
	if err != nil {
		return nil, err
	}

	// Set the status of the loopback device
	loopInfo := &unix.LoopInfo64{
		File_name: stringToLoopName(loopFile.Name()),
		Offset:    0,
		Flags:     unix.LO_FLAGS_AUTOCLEAR,
	}

	if err = unix.IoctlLoopSetStatus64(int(loopFile.Fd()), loopInfo); err != nil {
		log.G(context.TODO()).Errorf("Cannot set up loopback device info: %s", err)

		// If the call failed, then free the loopback device
		if err = unix.IoctlSetInt(int(loopFile.Fd()), unix.LOOP_CLR_FD, 0); err != nil {
			log.G(context.TODO()).Error("Error while cleaning up the loopback device")
		}
		loopFile.Close()
		return nil, ErrAttachLoopbackDevice
	}

	return loopFile, nil
}
