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
	"context"
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/containerd/log"
	"golang.org/x/sys/unix"
)

var (
	pidfdSupported     bool
	pidfdSupportedOnce sync.Once
)

// SupportsPidFD returns true if current kernel supports pidfd.
func SupportsPidFD() bool {
	pidfdSupportedOnce.Do(func() {
		logger := log.G(context.Background())

		if err := checkPidFD(); err != nil {
			logger.WithError(err).Error("failed to ensure the kernel supports pidfd")

			pidfdSupported = false
			return
		}
		pidfdSupported = true
	})
	return pidfdSupported
}

func checkPidFD() error {
	// Linux kernel supports pidfd_open(2) since v5.3.
	//
	// https://man7.org/linux/man-pages/man2/pidfd_open.2.html
	pidfd, err := unix.PidfdOpen(os.Getpid(), 0)
	if err != nil {
		return fmt.Errorf("failed to invoke pidfd_open: %w", err)
	}
	defer unix.Close(pidfd)

	// Linux kernel supports pidfd_send_signal(2) since v5.1.
	//
	// https://man7.org/linux/man-pages/man2/pidfd_send_signal.2.html
	if err := unix.PidfdSendSignal(pidfd, 0, nil, 0); err != nil {
		return fmt.Errorf("failed to invoke pidfd_send_signal: %w", err)
	}

	// The waitid(2) supports P_PIDFD since Linux kernel v5.4.
	//
	// https://man7.org/linux/man-pages/man2/waitid.2.html
	werr := IgnoringEINTR(func() error {
		return unix.Waitid(unix.P_PIDFD, pidfd, nil, unix.WEXITED, nil)
	})

	// The waitid returns ECHILD since current process isn't the child of current process.
	if !errors.Is(werr, unix.ECHILD) {
		return fmt.Errorf("failed to invoke waitid with P_PIDFD: wanted error %v, but got %v",
			unix.ECHILD, werr)
	}

	// NOTE: The CLONE_PIDFD flag has been supported since Linux kernel v5.2.
	// So assumption is that if waitid(2) supports P_PIDFD, current kernel
	// should support CLONE_PIDFD as well.
	return nil
}
