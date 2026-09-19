// SPDX-FileCopyrightText: Copyright The Moby Authors
// SPDX-License-Identifier: Apache-2.0

package launcher

import "io/fs"

// fileUID cannot be determined portably on Windows, where ACLs govern access.
func fileUID(fs.FileInfo) (int, bool) { return 0, false }
