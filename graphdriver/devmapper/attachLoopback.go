package devmapper

import (
	"fmt"
	"github.com/dotcloud/docker/utils"
	"unsafe"
)

func ioctlLoopCtlGetFree(fd uintptr) (int, error) {
	index, _, err := sysSyscall(sysSysIoctl, fd, LoopCtlGetFree, 0)
	if err != 0 {
		return 0, err
	}
	return int(index), nil
}

func ioctlLoopSetFd(loopFd, sparseFd uintptr) error {
	if _, _, err := sysSyscall(sysSysIoctl, loopFd, LoopSetFd, sparseFd); err != 0 {
		return err
	}
	return nil
}

func ioctlLoopSetStatus64(loopFd uintptr, loopInfo *LoopInfo64) error {
	_, _, err := sysSyscall(sysSysIoctl, loopFd, LoopSetStatus64, uintptr(unsafe.Pointer(loopInfo)))
	if err != 0 {
		return err
	}
	return nil
}

func ioctlLoopClrFd(loopFd uintptr) error {
	_, _, err := sysSyscall(sysSysIoctl, loopFd, LoopClrFd, 0)
	if err != 0 {
		return err
	}
	return nil
}

// //func dmGetLoopbackBackingFileFct(fd uintptr) (uint64, uint64, sysErrno) {
// func ioctlLoopGetStatus64(loopFd uintptr) (*LoopInfo64, error) {
// 	var lo64 C.struct_loop_info64
// 	_, _, err := sysSyscall(sysSysIoctl, fd, C.LOOP_GET_STATUS64, uintptr(unsafe.Pointer(&lo64)))
// 	return uint64(lo64.lo_device), uint64(lo64.lo_inode), sysErrno(err)
// }

// func dmLoopbackSetCapacityFct(fd uintptr) sysErrno {
// 	_, _, err := sysSyscall(sysSysIoctl, fd, C.LOOP_SET_CAPACITY, 0)
// 	return sysErrno(err)
// }

// func dmGetBlockSizeFct(fd uintptr) (int64, sysErrno) {
// 	var size int64
// 	_, _, err := sysSyscall(sysSysIoctl, fd, C.BLKGETSIZE64, uintptr(unsafe.Pointer(&size)))
// 	return size, sysErrno(err)
// }

// func getBlockSizeFct(fd uintptr, size *uint64) sysErrno {
// 	_, _, err := sysSyscall(sysSysIoctl, fd, C.BLKGETSIZE64, uintptr(unsafe.Pointer(&size)))
// 	return sysErrno(err)
// }

func getNextFreeLoopbackIndex() (int, error) {
	f, err := osOpenFile("/dev/loop-control", osORdOnly, 0644)
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

func openNextAvailableLoopback(index int, sparseFile *osFile) (loopFile *osFile, err error) {
	// Start looking for a free /dev/loop
	for {
		target := fmt.Sprintf("/dev/loop%d", index)
		index++

		fi, err := osStat(target)
		if err != nil {
			if osIsNotExist(err) {
				utils.Errorf("There are no more loopback device available.")
			}
			return nil, ErrAttachLoopbackDevice
		}

		// FIXME: Check here if target is a block device (in C: S_ISBLK(mode))
		if fi.IsDir() {
		}

		// Open the targeted loopback (use OpenFile because Open sets O_CLOEXEC)
		loopFile, err = osOpenFile(target, osORdWr, 0644)
		if err != nil {
			utils.Errorf("Error openning loopback device: %s", err)
			return nil, ErrAttachLoopbackDevice
		}

		// Try to attach to the loop file
		if err := ioctlLoopSetFd(loopFile.Fd(), sparseFile.Fd()); err != nil {
			loopFile.Close()

			// If the error is EBUSY, then try the next loopback
			if err != sysEBusy {
				utils.Errorf("Cannot set up loopback device %s: %s", target, err)
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
		utils.Errorf("Unreachable code reached! Error attaching %s to a loopback device.", sparseFile.Name())
		return nil, ErrAttachLoopbackDevice
	}

	return loopFile, nil
}

func stringToLoopName(src string) [LoNameSize]uint8 {
	var dst [LoNameSize]uint8
	copy(dst[:], src[:])
	return dst
}

// attachLoopDevice attaches the given sparse file to the next
// available loopback device. It returns an opened *osFile.
func attachLoopDevice(sparseName string) (loop *osFile, err error) {

	// Try to retrieve the next available loopback device via syscall.
	// If it fails, we discard error and start loopking for a
	// loopback from index 0.
	startIndex, err := getNextFreeLoopbackIndex()
	if err != nil {
		utils.Debugf("Error retrieving the next available loopback: %s", err)
	}

	// Open the given sparse file (use OpenFile because Open sets O_CLOEXEC)
	sparseFile, err := osOpenFile(sparseName, osORdWr, 0644)
	if err != nil {
		utils.Errorf("Error openning sparse file %s: %s", sparseName, err)
		return nil, ErrAttachLoopbackDevice
	}
	defer sparseFile.Close()

	loopFile, err := openNextAvailableLoopback(startIndex, sparseFile)
	if err != nil {
		return nil, err
	}

	// Set the status of the loopback device
	loopInfo := &LoopInfo64{
		loFileName: stringToLoopName(loopFile.Name()),
		loOffset:   0,
		loFlags:    LoFlagsAutoClear,
	}

	if err := ioctlLoopSetStatus64(loopFile.Fd(), loopInfo); err != nil {
		utils.Errorf("Cannot set up loopback device info: %s", err)

		// If the call failed, then free the loopback device
		if err := ioctlLoopClrFd(loopFile.Fd()); err != nil {
			utils.Errorf("Error while cleaning up the loopback device")
		}
		loopFile.Close()
		return nil, ErrAttachLoopbackDevice
	}

	return loopFile, nil
}
