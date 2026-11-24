package chrootarchive

import (
	"errors"
	"io"
	"os"
	"path/filepath"
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

// Handler for teasing out the automatic decompression
func untarHandler(tarArchive io.Reader, dest string, options *archive.TarOptions, decompress bool, root string) error {
	if tarArchive == nil {
		return errors.New("empty archive")
	}

	// Create dest here only if it is the root itself; paths below the root are
	// created by the extractor after entering the chroot.
	// This case is only currently used by cp.
	if dest == root {
		dest = filepath.Clean(dest)
		if _, err := os.Stat(dest); os.IsNotExist(err) {
			if err := os.MkdirAll(dest, 0); err != nil {
				return err
			}
		}
	}

	// Windows is different to Linux here because Windows does not support
	// chroot. Hence there is no point sandboxing a chrooted process to
	// do the unpack. We call inline instead within the daemon process.
	if decompress {
		return archive.Untar(tarArchive, addLongPathPrefix(dest), options)
	}
	return archive.UntarUncompressed(tarArchive, addLongPathPrefix(dest), options)
}

func invokePack(srcPath string, options *archive.TarOptions, _ string) (io.ReadCloser, error) {
	// Windows is different to Linux here because Windows does not support
	// chroot. Hence there is no point sandboxing a chrooted process to
	// do the pack. We call inline instead within the daemon process.
	return archive.TarWithOptions(srcPath, options)
}
