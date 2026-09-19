// SPDX-FileCopyrightText: Copyright The Moby Authors
// SPDX-License-Identifier: Apache-2.0

package launcher

import "os"

func shutdownSignal() os.Signal {
	return os.Interrupt
}
