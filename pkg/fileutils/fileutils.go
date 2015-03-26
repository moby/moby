package fileutils

import (
	"github.com/Sirupsen/logrus"
	"path/filepath"
)

// Matches returns true if relFilePath matches any of the patterns
func Matches(relFilePath string, patterns []string) (bool, error) {
	for _, exclude := range patterns {
		matched, err := filepath.Match(exclude, relFilePath)
		if err != nil {
			logrus.Errorf("Error matching: %s (pattern: %s)", relFilePath, exclude)
			return false, err
		}
		if matched {
			if filepath.Clean(relFilePath) == "." {
				logrus.Errorf("Can't exclude whole path, excluding pattern: %s", exclude)
				continue
			}
			logrus.Debugf("Skipping excluded path: %s", relFilePath)
			return true, nil
		}
	}
	return false, nil
}
