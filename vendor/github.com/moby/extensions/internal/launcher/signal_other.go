// SPDX-FileCopyrightText: Copyright The Moby Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !unix && !windows

package launcher

import "os"

func shutdownSignal() os.Signal {
	return os.Interrupt
}
