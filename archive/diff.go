package archive

import (
	"log"
	"os"
	"path/filepath"
	"strings"
)

// ApplyLayer parses a diff in the standard layer format from `layer`, and
// applies it to the directory `dest`.
func ApplyLayer(dest string, layer Archive) error {
	// Poor man's diff applyer in 2 steps:

	// Step 1: untar everything in place
	if err := Untar(layer, dest); err != nil {
		return err
	}

	// Step 2: walk for whiteouts and apply them, removing them in the process
	err := filepath.Walk(dest, func(fullPath string, f os.FileInfo, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				// This happens in the case of whiteouts in parent dir removing a directory
				// We just ignore it
				return filepath.SkipDir
			}
			return err
		}

		// Rebase path
		path, err := filepath.Rel(dest, fullPath)
		if err != nil {
			return err
		}
		path = filepath.Join("/", path)

		// Skip AUFS metadata
		if matched, err := filepath.Match("/.wh..wh.*", path); err != nil {
			return err
		} else if matched {
			log.Printf("Removing aufs metadata %s", fullPath)
			_ = os.RemoveAll(fullPath)
		}

		filename := filepath.Base(path)
		if strings.HasPrefix(filename, ".wh.") {
			rmTargetName := filename[len(".wh."):]
			rmTargetPath := filepath.Join(filepath.Dir(fullPath), rmTargetName)
			// Remove the file targeted by the whiteout
			log.Printf("Removing whiteout target %s", rmTargetPath)
			_ = os.Remove(rmTargetPath)
			// Remove the whiteout itself
			log.Printf("Removing whiteout %s", fullPath)
			_ = os.RemoveAll(fullPath)
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}
