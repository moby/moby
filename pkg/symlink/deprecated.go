package symlink // import "github.com/docker/docker/pkg/symlink"

import "github.com/moby/sys/symlink"

var (
	// EvalSymlinks is deprecated and moved to github.com/moby/sys/symlink
	// Deprecated: use github.com/moby/sys/symlink.EvalSymlinks instead
	EvalSymlinks = symlink.EvalSymlinks
	// FollowSymlinkInScope is deprecated and moved to github.com/moby/sys/symlink
	// Deprecated: use github.com/moby/sys/symlink.FollowSymlinkInScope instead
	FollowSymlinkInScope = symlink.FollowSymlinkInScope
)
