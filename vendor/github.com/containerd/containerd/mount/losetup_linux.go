/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package mount

import (
	"fmt"
	"math/rand"
	"os"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

const (
	loopControlPath = "/dev/loop-control"
	loopDevFormat   = "/dev/loop%d"

	ebusyString = "device or resource busy"
)

// LoopParams parameters to control loop device setup
type LoopParams struct {
	// Loop device should forbid write
	Readonly bool
	// Loop device is automatically cleared by kernel when the
	// last opener closes it
	Autoclear bool
	// Use direct IO to access the loop backing file
	Direct bool
}

func ioctl(fd, req, args uintptr) (uintptr, uintptr, error) {
	r1, r2, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, req, args)
	if errno != 0 {
		return 0, 0, errno
	}

	return r1, r2, nil
}

func getFreeLoopDev() (uint32, error) {
	ctrl, err := os.OpenFile(loopControlPath, os.O_RDWR, 0)
	if err != nil {
		return 0, errors.Errorf("could not open %v: %v", loopControlPath, err)
	}
	defer ctrl.Close()
	num, _, err := ioctl(ctrl.Fd(), unix.LOOP_CTL_GET_FREE, 0)
	if err != nil {
		return 0, errors.Wrap(err, "could not get free loop device")
	}
	return uint32(num), nil
}

// setupLoopDev attaches the backing file to the loop device and returns
// the file handle for the loop device. The caller is responsible for
// closing the file handle.
func setupLoopDev(backingFile, loopDev string, param LoopParams) (_ *os.File, retErr error) {
	// 1. Open backing file and loop device
	flags := os.O_RDWR
	if param.Readonly {
		flags = os.O_RDONLY
	}

	back, err := os.OpenFile(backingFile, flags, 0)
	if err != nil {
		return nil, errors.Wrapf(err, "could not open backing file: %s", backingFile)
	}
	defer back.Close()

	loop, err := os.OpenFile(loopDev, flags, 0)
	if err != nil {
		return nil, errors.Wrapf(err, "could not open loop device: %s", loopDev)
	}
	defer func() {
		if retErr != nil {
			loop.Close()
		}
	}()

	// 2. Set FD
	if _, _, err = ioctl(loop.Fd(), unix.LOOP_SET_FD, back.Fd()); err != nil {
		return nil, errors.Wrapf(err, "could not set loop fd for device: %s", loopDev)
	}

	// 3. Set Info
	info := unix.LoopInfo64{}
	copy(info.File_name[:], backingFile)
	if param.Readonly {
		info.Flags |= unix.LO_FLAGS_READ_ONLY
	}

	if param.Autoclear {
		info.Flags |= unix.LO_FLAGS_AUTOCLEAR
	}

	if param.Direct {
		info.Flags |= unix.LO_FLAGS_DIRECT_IO
	}

	_, _, err = ioctl(loop.Fd(), unix.LOOP_SET_STATUS64, uintptr(unsafe.Pointer(&info)))
	if err == nil {
		return loop, nil
	}

	if param.Direct {
		// Retry w/o direct IO flag in case kernel does not support it. The downside is that
		// it will suffer from double cache problem.
		info.Flags &= ^(uint32(unix.LO_FLAGS_DIRECT_IO))
		_, _, err = ioctl(loop.Fd(), unix.LOOP_SET_STATUS64, uintptr(unsafe.Pointer(&info)))
		if err == nil {
			return loop, nil
		}
	}

	_, _, _ = ioctl(loop.Fd(), unix.LOOP_CLR_FD, 0)
	return nil, errors.Errorf("failed to set loop device info: %v", err)
}

// setupLoop looks for (and possibly creates) a free loop device, and
// then attaches backingFile to it.
//
// When autoclear is true, caller should take care to close it when
// done with the loop device. The loop device file handle keeps
// loFlagsAutoclear in effect and we rely on it to clean up the loop
// device. If caller closes the file handle after mounting the device,
// kernel will clear the loop device after it is umounted. Otherwise
// the loop device is cleared when the file handle is closed.
//
// When autoclear is false, caller should be responsible to remove
// the loop device when done with it.
//
// Upon success, the file handle to the loop device is returned.
func setupLoop(backingFile string, param LoopParams) (*os.File, error) {
	for retry := 1; retry < 100; retry++ {
		num, err := getFreeLoopDev()
		if err != nil {
			return nil, err
		}

		loopDev := fmt.Sprintf(loopDevFormat, num)
		file, err := setupLoopDev(backingFile, loopDev, param)
		if err != nil {
			// Per util-linux/sys-utils/losetup.c:create_loop(),
			// free loop device can race and we end up failing
			// with EBUSY when trying to set it up.
			if strings.Contains(err.Error(), ebusyString) {
				// Fallback a bit to avoid live lock
				time.Sleep(time.Millisecond * time.Duration(rand.Intn(retry*10)))
				continue
			}
			return nil, err
		}

		return file, nil
	}

	return nil, errors.New("timeout creating new loopback device")
}

func removeLoop(loopdev string) error {
	file, err := os.Open(loopdev)
	if err != nil {
		return err
	}
	defer file.Close()

	_, _, err = ioctl(file.Fd(), unix.LOOP_CLR_FD, 0)
	return err
}

// AttachLoopDevice attaches a specified backing file to a loop device
func AttachLoopDevice(backingFile string) (string, error) {
	file, err := setupLoop(backingFile, LoopParams{})
	if err != nil {
		return "", err
	}
	defer file.Close()
	return file.Name(), nil
}

// DetachLoopDevice detaches the provided loop devices
func DetachLoopDevice(devices ...string) error {
	for _, dev := range devices {
		if err := removeLoop(dev); err != nil {
			return errors.Wrapf(err, "failed to remove loop device: %s", dev)
		}
	}

	return nil
}
