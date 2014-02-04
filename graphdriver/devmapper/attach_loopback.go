// +build linux,amd64

package devmapper

import (
	"fmt"
	"github.com/dotcloud/docker/utils"
)

func stringToLoopName(src string) [LoNameSize]uint8 {
	var dst [LoNameSize]uint8
	copy(dst[:], src[:])
	return dst
}

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

		if fi.Mode()&osModeDevice != osModeDevice {
			utils.Errorf("Loopback device %s is not a block device.", target)
			continue
		}

		// OpenFile adds O_CLOEXEC
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

	// OpenFile adds O_CLOEXEC
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
