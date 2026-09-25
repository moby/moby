// SPDX-FileCopyrightText: Copyright The Moby Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !unix && !windows

package launcher

import "io/fs"

func fileUID(fs.FileInfo) (int, bool) {
	return 0, false
}
