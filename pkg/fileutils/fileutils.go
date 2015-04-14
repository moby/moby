package fileutils

import (
	"path/filepath"

	"github.com/Sirupsen/logrus"
)

// Matches returns true if relFilePath matches any of the patterns
// and isn't excluded by any of the subsequent patterns.
func Matches(relFilePath string, patterns []string) (bool, error) {

	matched := false

	for _, pattern := range patterns {

		var negative bool
		if pattern == "" {
			continue
		}

		if pattern[0] == '!' {
			if len(pattern) == 1 {
				continue
			}
			negative = true
			pattern = pattern[1:]
		}

		match, err := filepath.Match(pattern, relFilePath)
		if err != nil {
			logrus.Errorf("Error matching: %s (pattern: %s)", relFilePath, pattern)
			return false, err
		}

		if match {

			if negative {
				matched = false
				continue
			}

			if filepath.Clean(relFilePath) == "." {
				logrus.Errorf("Can't exclude whole path, excluding pattern: %s", pattern)
				continue
			}
			matched = true
		}
	}

	if matched {
		logrus.Debugf("Skipping excluded path: %s", relFilePath)
	}

	return matched, nil
}
