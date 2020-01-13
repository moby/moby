package fs

import (
	"io"
	"os"

	"github.com/pkg/errors"
)

func (c *copier) copyFileInfo(fi os.FileInfo, name string) error {
	if err := os.Chmod(name, fi.Mode()); err != nil {
		return errors.Wrapf(err, "failed to chmod %s", name)
	}

	// TODO: copy windows specific metadata

	return nil
}

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

func copyXAttrs(dst, src string, xeh XAttrErrorHandler) error {
	return nil
}

func copyDevice(dst string, fi os.FileInfo) error {
	return errors.New("device copy not supported")
}
