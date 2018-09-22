package continuity

import "os"

type hardlinkKey struct{}

func newHardlinkKey(fi os.FileInfo) (hardlinkKey, error) {
	// NOTE(stevvooe): Obviously, this is not yet implemented. However, the
	// makings of an implementation are available in src/os/types_windows.go. More
	// investigation needs to be done to figure out exactly how to do this.
	return hardlinkKey{}, errNotAHardLink
}
