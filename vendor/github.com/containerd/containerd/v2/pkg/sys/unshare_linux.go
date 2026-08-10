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
	"errors"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

// UnshareAfterEnterUserns allows to disassociate parts of its execution context
// within a user namespace.
func UnshareAfterEnterUserns(uidMap, gidMap string, unshareFlags uintptr, f func(pid int) error) (retErr error) {
	if unshareFlags&syscall.CLONE_NEWUSER == syscall.CLONE_NEWUSER {
		return fmt.Errorf("unshare flags should not include user namespace")
	}

	if !SupportsPidFD() {
		return fmt.Errorf("kernel doesn't support pidfd")
	}

	uidMaps, err := parseIDMapping(uidMap)
	if err != nil {
		return err
	}

	gidMaps, err := parseIDMapping(gidMap)
	if err != nil {
		return err
	}

	targetUID := uidMaps[0].HostID
	pid, pidfd, err := startProcessWithUserNamespace(targetUID, unshareFlags, uidMaps, gidMaps)
	if err != nil {
		return err
	}

	defer unix.Close(pidfd)
	defer func() {
		derr := unix.PidfdSendSignal(pidfd, unix.SIGKILL, nil, 0)
		if derr != nil {
			if !errors.Is(derr, unix.ESRCH) {
				retErr = derr
			}
			return
		}
		pidfdWaitid(pidfd)
	}()

	if f != nil {
		if err := f(pid); err != nil {
			return err
		}
	}

	// Ensure the child process is still alive. If the err is ESRCH, we
	// should return error because the pid could be reused. It's safe to
	// return error and retry.
	if err := unix.PidfdSendSignal(pidfd, 0, nil, 0); err != nil {
		return fmt.Errorf("failed to ensure child process is alive: %w", err)
	}
	return nil
}

// TODO: Support multiple mappings in future
func parseIDMapping(mapping string) ([]syscall.SysProcIDMap, error) {
	parts := strings.Split(mapping, ":")
	if len(parts) != 3 {
		return nil, fmt.Errorf("user namespace mappings require the format `container-id:host-id:size`")
	}

	cID, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid container id for user namespace remapping, %w", err)
	}

	hID, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid host id for user namespace remapping, %w", err)
	}

	size, err := strconv.Atoi(parts[2])
	if err != nil {
		return nil, fmt.Errorf("invalid size for user namespace remapping, %w", err)
	}

	if cID < 0 || hID < 0 || size < 0 {
		return nil, fmt.Errorf("invalid mapping %s, all IDs and size must be positive integers", mapping)
	}

	return []syscall.SysProcIDMap{
		{
			ContainerID: cID,
			HostID:      hID,
			Size:        size,
		},
	}, nil
}

// startProcessWithUserNamespace starts a ptraced dummy process to create
// a user namespace. It runs a goroutine on a single thread as targetHostUID
// when creating the process and user namespace, ensuring that the kernel
// attributes user limits to targetHostUID and not containerd's user. On success
// it returns the pid, pidfd and no error.
func startProcessWithUserNamespace(targetHostUID int, unshareFlags uintptr, uidMaps, gidMaps []syscall.SysProcIDMap) (int, int, error) {
	type result struct {
		pid   int
		pidfd int
		err   error
	}
	resultChan := make(chan result)

	go func() {
		runtime.LockOSThread()
		pid, pidfd, err := startProcessWithUsernsLocked(targetHostUID, unshareFlags, uidMaps, gidMaps)

		// If this errored out let the go runtime reap the thread by not unlocking
		if err == nil {
			runtime.UnlockOSThread()
		}
		resultChan <- result{pid, pidfd, err}
	}()

	res := <-resultChan
	return res.pid, res.pidfd, res.err
}

// startProcessWithUsernsLocked expects the os thread to be locked already. It does
//  1. setresuid() to the targetHostUID user to attribute further user namespace creations to it
//  2. sets the thread's effective capabilities back to the original user's to allow writing
//     to /proc/uid_map as well as to create the user namespace as a "privileged" process in
//     some distro's eyes,
//  3. spawns a new ptraced process in a new user namespace
//  4. unshares into unshareFlags within that user namespace
//
// returns the pid, pidfd, and error information of that process. On error the process is already
// killed, and the thread is *not* setresuid()'ed back to the original user. In this case the
// thread should be killed off by the caller to avoid using a thread in a bad state
func startProcessWithUsernsLocked(targetHostUID int, unshareFlags uintptr, uidMaps, gidMaps []syscall.SysProcIDMap) (int, int, error) {
	originalEUID := os.Geteuid()

	originalCaps, err := getCurrentCaps()
	if err != nil {
		return -1, -1, fmt.Errorf("failed to read current capabilities: %w", err)
	}

	if _, _, errno := syscall.RawSyscall(unix.SYS_SETRESUID, ^uintptr(0), uintptr(targetHostUID), ^uintptr(0)); errno != 0 {
		return -1, -1, fmt.Errorf("failed to set effective UID: %w", errno)
	}

	err = setCurrentCaps(originalCaps)
	if err != nil {
		return -1, -1, fmt.Errorf("failed to restore capabilities: %w", err)
	}

	var pidfd int
	proc, err := os.StartProcess("/proc/self/exe", []string{"UnshareAfterEnterUserns"}, &os.ProcAttr{
		Sys: &syscall.SysProcAttr{
			// clone new user namespace first and then unshare
			Cloneflags:                 unix.CLONE_NEWUSER,
			Unshareflags:               unshareFlags,
			UidMappings:                uidMaps,
			GidMappings:                gidMaps,
			GidMappingsEnableSetgroups: true,
			// NOTE: It's reexec but it's not heavy because subprocess
			// be in PTRACE_TRACEME mode before performing execve.
			Ptrace:    true,
			Pdeathsig: syscall.SIGKILL,
			PidFD:     &pidfd,
		},
	})
	if err != nil {
		return -1, -1, fmt.Errorf("failed to start noop process for unshare: %w", err)
	}

	if pidfd == -1 {
		proc.Kill()
		proc.Wait()
		return -1, -1, fmt.Errorf("kernel doesn't support CLONE_PIDFD")
	}

	if _, _, errno := syscall.RawSyscall(unix.SYS_SETRESUID, ^uintptr(0), uintptr(originalEUID), ^uintptr(0)); errno != 0 {
		proc.Kill()
		proc.Wait()
		unix.Close(pidfd)
		return -1, -1, fmt.Errorf("failed to restore UID: %w", errno)
	}

	return proc.Pid, pidfd, nil
}

func pidfdWaitid(pidfd int) error {
	return IgnoringEINTR(func() error {
		return unix.Waitid(unix.P_PIDFD, pidfd, nil, unix.WEXITED, nil)
	})
}

type capSnapshot struct {
	hdr  unix.CapUserHeader
	data [2]unix.CapUserData
}

func getCurrentCaps() (*capSnapshot, error) {
	caps := &capSnapshot{}
	caps.hdr.Version = unix.LINUX_CAPABILITY_VERSION_3

	err := unix.Capget(&caps.hdr, &caps.data[0])
	if err != nil {
		return nil, err
	}
	return caps, nil
}

func setCurrentCaps(caps *capSnapshot) error {
	return unix.Capset(&caps.hdr, &caps.data[0])
}
