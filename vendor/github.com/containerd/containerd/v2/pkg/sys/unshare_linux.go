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
		return fmt.Errorf("failed to start noop process for unshare: %w", err)
	}

	if pidfd == -1 {
		proc.Kill()
		proc.Wait()
		return fmt.Errorf("kernel doesn't support CLONE_PIDFD")
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
		if err := f(proc.Pid); err != nil {
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

func pidfdWaitid(pidfd int) error {
	return IgnoringEINTR(func() error {
		return unix.Waitid(unix.P_PIDFD, pidfd, nil, unix.WEXITED, nil)
	})
}
