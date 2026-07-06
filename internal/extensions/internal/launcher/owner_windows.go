package launcher

import "io/fs"

// fileUID cannot be determined portably on Windows, where file access is
// governed by ACLs that the Unix-style owner does not reflect; ownership is not
// enforced there (see untrustedOwner).
func fileUID(fs.FileInfo) (int, bool) { return 0, false }
