// SPDX-FileCopyrightText: Copyright The Moby Authors
// SPDX-License-Identifier: Apache-2.0

//go:build unix

package launcher

import (
	"os"
	"syscall"
)

func shutdownSignal() os.Signal {
	return syscall.SIGTERM
}
