package docker

import (
	"os"
	"io"
)

func CopyFile(dstFile, srcFile *os.File) error {
	// No BTRFS reflink suppport, Fall back to normal copy

	// FIXME: Check the return of Copy and compare with dstFile.Stat().Size
	_, err := io.Copy(dstFile, srcFile)
	return err
}
