// Package symlink implements [FollowSymlinkInScope] which is an extension
// of [path/filepath.EvalSymlinks], as well as a Windows long-path aware
// version of [path/filepath.EvalSymlinks] from the Go standard library.
//
// The code from [path/filepath.EvalSymlinks] has been adapted in fs.go.
// Read the [LICENSE.BSD] file that governs fs.go and [LICENSE.APACHE] for
// fs_unix_test.go.
//
// [LICENSE.APACHE]: https://github.com/moby/sys/blob/symlink/v0.2.0/symlink/LICENSE.APACHE
// [LICENSE.BSD]: https://github.com/moby/sys/blob/symlink/v0.2.0/symlink/LICENSE.APACHE
package symlink
