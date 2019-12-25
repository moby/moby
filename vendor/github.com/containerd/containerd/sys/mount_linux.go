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

package sys

import (
	"runtime"
	"syscall"
	"unsafe"

	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

// FMountat performs mount from the provided directory.
func FMountat(dirfd uintptr, source, target, fstype string, flags uintptr, data string) error {
	var (
		sourceP, targetP, fstypeP, dataP *byte
		pid                              uintptr
		ws                               unix.WaitStatus
		err                              error
		errno                            syscall.Errno
	)

	sourceP, err = syscall.BytePtrFromString(source)
	if err != nil {
		return err
	}

	targetP, err = syscall.BytePtrFromString(target)
	if err != nil {
		return err
	}

	fstypeP, err = syscall.BytePtrFromString(fstype)
	if err != nil {
		return err
	}

	if data != "" {
		dataP, err = syscall.BytePtrFromString(data)
		if err != nil {
			return err
		}
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	pid, errno = forkAndMountat(dirfd,
		uintptr(unsafe.Pointer(sourceP)),
		uintptr(unsafe.Pointer(targetP)),
		uintptr(unsafe.Pointer(fstypeP)),
		flags,
		uintptr(unsafe.Pointer(dataP)))

	if errno != 0 {
		return errors.Wrap(errno, "failed to fork thread")
	}

	_, err = unix.Wait4(int(pid), &ws, 0, nil)
	for err == syscall.EINTR {
		_, err = unix.Wait4(int(pid), &ws, 0, nil)
	}

	if err != nil {
		return errors.Wrapf(err, "failed to find pid=%d process", pid)
	}

	errno = syscall.Errno(ws.ExitStatus())
	if errno != 0 {
		return errors.Wrap(errno, "failed to mount")
	}
	return nil
}

// forkAndMountat will fork thread, change working dir and mount.
//
// precondition: the runtime OS thread must be locked.
func forkAndMountat(dirfd uintptr, source, target, fstype, flags, data uintptr) (pid uintptr, errno syscall.Errno) {
	// block signal during clone
	beforeFork()

	// the cloned thread shares the open file descriptor, but the thread
	// never be reused by runtime.
	pid, _, errno = syscall.RawSyscall6(syscall.SYS_CLONE, uintptr(syscall.SIGCHLD)|syscall.CLONE_FILES, 0, 0, 0, 0, 0)
	if errno != 0 || pid != 0 {
		// restore all signals
		afterFork()
		return
	}

	// restore all signals
	afterForkInChild()

	// change working dir
	_, _, errno = syscall.RawSyscall(syscall.SYS_FCHDIR, dirfd, 0, 0)
	if errno != 0 {
		goto childerr
	}
	_, _, errno = syscall.RawSyscall6(syscall.SYS_MOUNT, source, target, fstype, flags, data, 0)

childerr:
	syscall.RawSyscall(syscall.SYS_EXIT, uintptr(errno), 0, 0)
	panic("unreachable")
}
