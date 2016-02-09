package builder

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/docker/docker/pkg/fileutils"
)

// ValidateContextDirectory checks if all the contents of the directory
// can be read and returns an error if some files can't be read
// symlinks which point to non-existing files don't trigger an error
func ValidateContextDirectory(srcPath string, excludes []string) error {
	contextRoot, err := getContextRoot(srcPath)
	if err != nil {
		return err
	}
	return filepath.Walk(contextRoot, func(filePath string, f os.FileInfo, err error) error {
		// skip this directory/file if it's not in the path, it won't get added to the context
		if relFilePath, err := filepath.Rel(contextRoot, filePath); err != nil {
			return err
		} else if skip, err := fileutils.Matches(relFilePath, excludes); err != nil {
			return err
		} else if skip {
			if f.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if err != nil {
			if os.IsPermission(err) {
				return fmt.Errorf("can't stat '%s'", filePath)
			}
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}

		// skip checking if symlinks point to non-existing files, such symlinks can be useful
		// also skip named pipes, because they hanging on open
		if f.Mode()&(os.ModeSymlink|os.ModeNamedPipe) != 0 {
			return nil
		}

		if !f.IsDir() {
			currentFile, err := os.Open(filePath)
			if err != nil && os.IsPermission(err) {
				return fmt.Errorf("no permission to read from '%s'", filePath)
			}
			currentFile.Close()
		}
		return nil
	})
}
