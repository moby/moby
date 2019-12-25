package chrootarchive // import "github.com/docker/docker/pkg/chrootarchive"

import (
	"io"

	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/longpath"
)

// chroot is not supported by Windows
func chroot(path string) error {
	return nil
}

func invokeUnpack(decompressedArchive io.ReadCloser,
	dest string,
	options *archive.TarOptions, root string) error {
	// Windows is different to Linux here because Windows does not support
	// chroot. Hence there is no point sandboxing a chrooted process to
	// do the unpack. We call inline instead within the daemon process.
	return archive.Unpack(decompressedArchive, longpath.AddPrefix(dest), options)
}

func invokePack(srcPath string, options *archive.TarOptions, root string) (io.ReadCloser, error) {
	// Windows is different to Linux here because Windows does not support
	// chroot. Hence there is no point sandboxing a chrooted process to
	// do the pack. We call inline instead within the daemon process.
	return archive.TarWithOptions(srcPath, options)
}
