// +build linux,cgo

package loopback

import (
	"fmt"
	"os"
	"syscall"

	"github.com/Sirupsen/logrus"
)

func getLoopbackBackingFile(file *os.File, quiet bool) (uint64, uint64, error) {
	loopInfo, err := ioctlLoopGetStatus64(file.Fd())
	if err != nil {
		if ! quiet {
			logrus.Errorf("Error getting loopback backing file: %s", err)
		}
		return 0, 0, ErrGetLoopbackBackingFile
	}
	return loopInfo.loDevice, loopInfo.loInode, nil
}

// SetCapacity reloads the size for the loopback device.
func SetCapacity(file *os.File) error {
	if err := ioctlLoopSetCapacity(file.Fd(), 0); err != nil {
		logrus.Errorf("Error loopbackSetCapacity: %s", err)
		return ErrSetCapacity
	}
	return nil
}

// FindLoopDeviceFor returns a loopback device file for the specified file which
// is backing file of a loop back device.
func FindLoopDeviceFor(file *os.File) *os.File {
	stat, err := file.Stat()
	if err != nil {
		return nil
	}
	targetInode := stat.Sys().(*syscall.Stat_t).Ino
	targetDevice := stat.Sys().(*syscall.Stat_t).Dev

	for i := 0; true; i++ {
		path := fmt.Sprintf("/dev/loop%d", i)

		file, err := os.OpenFile(path, os.O_RDWR, 0)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}

			// Ignore all errors until the first not-exist
			// we want to continue looking for the file
			continue
		}

		dev, inode, err := getLoopbackBackingFile(file, false)
		if err == nil && dev == targetDevice && inode == targetInode {
			return file
		}
		file.Close()
	}

	return nil
}

// FindLoopDeviceForPath returns a loopback device file if the specified path
// is backed by a loop back device.
func FindLoopDeviceForPath(filePath string) *os.File {
	// Try opening filePath
	file, err := os.Open(filePath)
	if err != nil {
		return nil
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return nil
	}
	targetInode := stat.Sys().(*syscall.Stat_t).Ino
	targetDevice := stat.Sys().(*syscall.Stat_t).Dev

	for i := 0; true; i++ {
		path := fmt.Sprintf("/dev/loop%d", i)

		file, err := os.OpenFile(path, os.O_RDWR, 0)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}

			// Ignore all errors until the first not-exist
			// we want to continue looking for the file
			continue
		}

		dev, inode, err := getLoopbackBackingFile(file, true)
		if err == nil && dev == targetDevice && inode == targetInode {
			return file
		}
		file.Close()
	}

	return nil
}
// GetLoopDeviceFor returns a loopback device file for the specified file which
// is backing file of the loop back device passed as a paramter.
func GetLoopDeviceFor(file *os.File, loopName string) *os.File {
	stat, err := file.Stat()
	if err != nil {
		return nil
	}
	targetInode := stat.Sys().(*syscall.Stat_t).Ino
	targetDevice := stat.Sys().(*syscall.Stat_t).Dev

	file, err = os.OpenFile(loopName, os.O_RDWR, 0)
	if err != nil {
		return nil
	}

	dev, inode, err := getLoopbackBackingFile(file, false)
	if err == nil && dev == targetDevice && inode == targetInode {
		return file
	}
	file.Close()

	return nil
}
