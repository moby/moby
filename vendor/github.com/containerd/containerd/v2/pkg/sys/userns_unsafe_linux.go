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
)

// ForkUserns is to fork child process with user namespace. It returns child
// process's pid and pidfd reference to the child process.
//
// Precondition: The runtime OS thread must be locked, which is GO runtime
// requirement.
//
// Beside this, the child process sets PR_SET_PDEATHSIG with SIGKILL so that
// the parent process's OS thread must be locked. Otherwise, the exit event of
// parent process's OS thread will send kill signal to the child process,
// even if parent process is still running.
//
//go:norace
//go:noinline
func ForkUserns() (_pid uintptr, _pidfd uintptr, _ syscall.Errno) {
	var (
		pidfd     uintptr
		pid, ppid uintptr
		err       syscall.Errno
	)

	ppid, _, err = syscall.RawSyscall(uintptr(syscall.SYS_GETPID), 0, 0, 0)
	if err != 0 {
		return 0, 0, err
	}

	beforeFork()
	if runtime.GOARCH == "s390x" {
		// NOTE:
		//
		// On the s390 architectures, the order of the first two
		// arguments is reversed.
		//
		// REF: https://man7.org/linux/man-pages/man2/clone.2.html
		pid, _, err = syscall.RawSyscall(syscall.SYS_CLONE,
			0,
			uintptr(syscall.CLONE_NEWUSER|syscall.SIGCHLD|syscall.CLONE_PIDFD),
			uintptr(unsafe.Pointer(&pidfd)),
		)
	} else {
		pid, _, err = syscall.RawSyscall(syscall.SYS_CLONE,
			uintptr(syscall.CLONE_NEWUSER|syscall.SIGCHLD|syscall.CLONE_PIDFD),
			0,
			uintptr(unsafe.Pointer(&pidfd)),
		)
	}
	if err != 0 || pid != 0 {
		afterFork()
		return pid, pidfd, err
	}
	afterForkInChild()

	if _, _, err = syscall.RawSyscall(syscall.SYS_PRCTL, syscall.PR_SET_PDEATHSIG, uintptr(syscall.SIGKILL), 0); err != 0 {
		goto err
	}

	pid, _, err = syscall.RawSyscall(syscall.SYS_GETPPID, 0, 0, 0)
	if err != 0 {
		goto err
	}

	// exit if re-parent
	if pid != ppid {
		goto err
	}

	_, _, err = syscall.RawSyscall(syscall.SYS_PPOLL, 0, 0, 0)
err:
	syscall.RawSyscall(syscall.SYS_EXIT, uintptr(err), 0, 0)
	panic("unreachable")
}
