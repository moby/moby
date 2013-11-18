package aufs

import (
	"fmt"
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
			if err := tryRelocate(path.Join(pth, fi.Name(), "layer"), path.Join(a.rootPath(), "diff", fi.Name())); err != nil {
				return err
			}
			if err := a.Create(fi.Name(), ""); err != nil {
				return err
			}
		}
	}
	return nil
}

// tryRelocate will try to rename the old path to the new pack and if
// the operation fails, it will fallback to a symlink
func tryRelocate(oldPath, newPath string) error {
	if err := os.Rename(oldPath, newPath); err != nil {
		if sErr := os.Symlink(oldPath, newPath); sErr != nil {
			return fmt.Errorf("Unable to relocate %s to %s: Rename err %s Symlink err %s", oldPath, newPath, err, sErr)
		}
	}
	return nil
}
