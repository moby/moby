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
	"os"
	"syscall"

	"github.com/containerd/containerd/v2/pkg/sys"

	"golang.org/x/sys/unix"
)

// getUsernsFD returns pinnable user namespace's file descriptor.
func getUsernsFD(uidMaps, gidMaps []syscall.SysProcIDMap) (_ *os.File, retErr error) {
	if !sys.SupportsPidFD() {
		return nil, fmt.Errorf("kernel doesn't support pidfd")

	}

	var pidfd int

	proc, err := os.StartProcess("/proc/self/exe", []string{"containerd[getUsernsFD]"}, &os.ProcAttr{
		Sys: &syscall.SysProcAttr{
			Cloneflags:  unix.CLONE_NEWUSER,
			UidMappings: uidMaps,
			GidMappings: gidMaps,
			// NOTE: It's reexec but it's not heavy because subprocess
			// be in PTRACE_TRACEME mode before performing execve.
			Ptrace:    true,
			Pdeathsig: syscall.SIGKILL,
			PidFD:     &pidfd,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start noop process for unshare: %w", err)
	}

	if pidfd == -1 {
		proc.Kill()
		proc.Wait()
		return nil, fmt.Errorf("failed to prevent pid reused issue because pidfd isn't supported")
	}

	pidFD := os.NewFile(uintptr(pidfd), "pidfd")
	defer func() {
		unix.PidfdSendSignal(int(pidFD.Fd()), unix.SIGKILL, nil, 0)

		pidfdWaitid(pidFD)

		pidFD.Close()
	}()

	// NOTE:
	//
	// The usernsFD will hold the userns reference in kernel. Even if the
	// child process is reaped, the usernsFD is still valid.
	usernsFD, err := os.Open(fmt.Sprintf("/proc/%d/ns/user", proc.Pid))
	if err != nil {
		return nil, fmt.Errorf("failed to get userns file descriptor for /proc/%d/user/ns: %w", proc.Pid, err)
	}
	defer func() {
		if retErr != nil {
			usernsFD.Close()
		}
	}()

	// Ensure the child process is still alive. If the err is ESRCH, we
	// should return error because we can't guarantee the usernsFD and
	// u[g]idmapFile are valid. It's safe to return error and retry.
	if err := unix.PidfdSendSignal(int(pidFD.Fd()), 0, nil, 0); err != nil {
		return nil, fmt.Errorf("failed to ensure child process is alive: %w", err)
	}
	return usernsFD, nil
}

func pidfdWaitid(pidFD *os.File) error {
	return sys.IgnoringEINTR(func() error {
		return unix.Waitid(unix.P_PIDFD, int(pidFD.Fd()), nil, unix.WEXITED, nil)
	})
}
