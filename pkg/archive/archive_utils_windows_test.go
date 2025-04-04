package archive

import (
	"archive/tar"
	"os"

	"github.com/docker/docker/pkg/idtools"
)

// chmodTarEntry is used to adjust the file permissions used in tar header based
// on the platform the archival is done.
func chmodTarEntry(perm os.FileMode) os.FileMode {
	// Remove group- and world-writable bits.
	perm &= 0o755

	// Add the x bit: make everything +x on Windows
	return perm | 0o111
}

func getInodeFromStat(stat interface{}) (uint64, error) {
	// do nothing. no notion of Inode in stat on Windows
	return 0, nil
}

// handleTarTypeBlockCharFifo is an OS-specific helper function used by
// createTarFile to handle the following types of header: Block; Char; Fifo
func handleTarTypeBlockCharFifo(hdr *tar.Header, path string) error {
	return nil
}

func handleLChmod(hdr *tar.Header, path string, hdrInfo os.FileInfo) error {
	return nil
}

func getFileUIDGID(stat interface{}) (idtools.Identity, error) {
	// no notion of file ownership mapping yet on Windows
	return idtools.Identity{UID: 0, GID: 0}, nil
}
