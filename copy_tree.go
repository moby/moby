package docker

import (
	"io"
	"os"
	"path/filepath"
)

type treeCopyingVisitor struct {
	src  string
	dest string

	err error
}

func copyFile(src string, dest string, info os.FileInfo) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()
	destFile, err := os.OpenFile(dest, os.O_RDWR|os.O_CREATE|os.O_CREATE, info.Mode())
	if err != nil {
		return err
	}
	defer destFile.Close()
	_, err = io.Copy(destFile, srcFile)
	if err != nil {
		return err
	}

	return nil
}

func (self *treeCopyingVisitor) visit(src string, info os.FileInfo) error {
	rel, err := filepath.Rel(self.src, src)
	if err != nil {
		return err
	}

	dest := filepath.Join(self.dest, rel)

	if info.IsDir() {
		err = os.Mkdir(dest, info.Mode())
		if err != nil {
			if os.IsExist(err) {
				err = nil
			}
		}
	} else {
		err = copyFile(src, dest, info)
	}

	return err
}

func CopyTree(src string, dest string) error {
	visitor := &treeCopyingVisitor{src: src, dest: dest}

	f := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		return visitor.visit(path, info)
	}

	filepath.Walk(src, f)
	return visitor.err
}
