//go:build freebsd
// +build freebsd

package fs

import (
	"io"
	"os"

	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

func copyFile(source, target string) error {
	src, err := os.Open(source)
	if err != nil {
		return errors.Wrapf(err, "failed to open source %s", source)
	}
	defer src.Close()
	tgt, err := os.Create(target)
	if err != nil {
		return errors.Wrapf(err, "failed to open target %s", target)
	}
	defer tgt.Close()

	return copyFileContent(tgt, src)
}

func copyFileContent(dst, src *os.File) error {
	buf := bufferPool.Get().(*[]byte)
	_, err := io.CopyBuffer(dst, src, *buf)
	bufferPool.Put(buf)
	return err
}

func mknod(dst string, mode uint32, rDev int) error {
	return unix.Mknod(dst, uint32(mode), uint64(rDev))
}
