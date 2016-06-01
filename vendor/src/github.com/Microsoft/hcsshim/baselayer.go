package hcsshim

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"

	"github.com/Microsoft/go-winio"
)

type baseLayerWriter struct {
	root         string
	f            *os.File
	bw           *winio.BackupFileWriter
	err          error
	hasUtilityVM bool
}

func (w *baseLayerWriter) closeCurrentFile() error {
	if w.f != nil {
		err := w.bw.Close()
		err2 := w.f.Close()
		w.f = nil
		w.bw = nil
		if err != nil {
			return err
		}
		if err2 != nil {
			return err2
		}
	}
	return nil
}

func (w *baseLayerWriter) Add(name string, fileInfo *winio.FileBasicInfo) (err error) {
	defer func() {
		if err != nil {
			w.err = err
		}
	}()

	err = w.closeCurrentFile()
	if err != nil {
		return err
	}

	if filepath.ToSlash(name) == `UtilityVM/Files` {
		w.hasUtilityVM = true
	}

	path := filepath.Join(w.root, name)
	path, err = makeLongAbsPath(path)
	if err != nil {
		return err
	}

	var f *os.File
	defer func() {
		if f != nil {
			f.Close()
		}
	}()

	createmode := uint32(syscall.CREATE_NEW)
	if fileInfo.FileAttributes&syscall.FILE_ATTRIBUTE_DIRECTORY != 0 {
		err := os.Mkdir(path, 0)
		if err != nil && !os.IsExist(err) {
			return err
		}
		createmode = syscall.OPEN_EXISTING
	}

	mode := uint32(syscall.GENERIC_READ | syscall.GENERIC_WRITE | winio.WRITE_DAC | winio.WRITE_OWNER | winio.ACCESS_SYSTEM_SECURITY)
	f, err = winio.OpenForBackup(path, mode, syscall.FILE_SHARE_READ, createmode)
	if err != nil {
		return err
	}

	err = winio.SetFileBasicInfo(f, fileInfo)
	if err != nil {
		return err
	}

	w.f = f
	w.bw = winio.NewBackupFileWriter(f, true)
	f = nil
	return nil
}

func (w *baseLayerWriter) AddLink(name string, target string) (err error) {
	defer func() {
		if err != nil {
			w.err = err
		}
	}()

	err = w.closeCurrentFile()
	if err != nil {
		return err
	}

	linkpath, err := makeLongAbsPath(filepath.Join(w.root, name))
	if err != nil {
		return err
	}

	linktarget, err := makeLongAbsPath(filepath.Join(w.root, target))
	if err != nil {
		return err
	}

	return os.Link(linktarget, linkpath)
}

func (w *baseLayerWriter) Remove(name string) error {
	return errors.New("base layer cannot have tombstones")
}

func (w *baseLayerWriter) Write(b []byte) (int, error) {
	n, err := w.bw.Write(b)
	if err != nil {
		w.err = err
	}
	return n, err
}

func (w *baseLayerWriter) Close() error {
	err := w.closeCurrentFile()
	if err != nil {
		return err
	}
	if w.err == nil {
		err = ProcessBaseLayer(w.root)
		if err != nil {
			return err
		}

		if w.hasUtilityVM {
			err = ProcessUtilityVMImage(filepath.Join(w.root, "UtilityVM"))
			if err != nil {
				return err
			}
		}
	}
	return w.err
}
