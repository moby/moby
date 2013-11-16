package aufs

import (
	"io/ioutil"
	"os"
	"path"
)

func exists(pth string) bool {
	if _, err := os.Stat(pth); err != nil {
		return false
	}
	return true
}

func (a *AufsDriver) Migrate(pth string) error {
	fis, err := ioutil.ReadDir(pth)
	if err != nil {
		return err
	}
	for _, fi := range fis {
		if fi.IsDir() && exists(path.Join(pth, fi.Name(), "layer")) && !a.Exists(fi.Name()) {
			if err := os.Symlink(path.Join(pth, fi.Name(), "layer"), path.Join(a.rootPath(), "diff", fi.Name())); err != nil {
				return err
			}
			if err := a.Create(fi.Name(), ""); err != nil {
				return err
			}
		}
	}
	return nil
}
