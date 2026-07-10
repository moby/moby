package chrootarchive

import (
	"io"
	"strings"

	"github.com/moby/go-archive"
)

// longPathPrefix is the longpath prefix for Windows file paths.
const longPathPrefix = `\\?\`

// addLongPathPrefix adds the Windows long path prefix to the path provided if
// it does not already have it. It is a no-op on platforms other than Windows.
//
// addLongPathPrefix is a copy of [github.com/docker/docker/pkg/longpath.AddPrefix].
func addLongPathPrefix(srcPath string) string {
	if strings.HasPrefix(srcPath, longPathPrefix) {
		return srcPath
	}
	if strings.HasPrefix(srcPath, `\\`) {
		// This is a UNC path, so we need to add 'UNC' to the path as well.
		return longPathPrefix + `UNC` + srcPath[1:]
	}
	return longPathPrefix + srcPath
}

func invokeUnpack(decompressedArchive io.ReadCloser, dest string, options *archive.TarOptions, root string) error {
	// Windows is different to Linux here because Windows does not support
	// chroot. Hence there is no point sandboxing a chrooted process to
	// do the unpack. We call inline instead within the daemon process.
	return archive.Unpack(decompressedArchive, addLongPathPrefix(dest), options)
}

func invokePack(srcPath string, options *archive.TarOptions, root string) (io.ReadCloser, error) {
	// Windows is different to Linux here because Windows does not support
	// chroot. Hence there is no point sandboxing a chrooted process to
	// do the pack. We call inline instead within the daemon process.
	return archive.TarWithOptions(srcPath, options)
}
