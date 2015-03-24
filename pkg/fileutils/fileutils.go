package fileutils

import (
	log "github.com/Sirupsen/logrus"
	"path/filepath"
)

// Matches returns true if relFilePath matches any of the patterns
// and isn't excluded by any of the subsequent patterns.
func Matches(relFilePath string, patterns []string) (bool, error) {

	var matched = false

	for _, pattern := range patterns {

		var negative bool
		if pattern != "" && pattern[0] == '!' {
			negative = true
			pattern = pattern[1:]
		}

		match, err := filepath.Match(pattern, relFilePath)
		if err != nil {
			log.Errorf("Error matching: %s (pattern: %s)", relFilePath, pattern)
			return false, err
		}

		if match {

			if negative {
				matched = false
				continue
			}

			if filepath.Clean(relFilePath) == "." {
				log.Errorf("Can't exclude whole path, excluding pattern: %s", pattern)
				continue
			}
			matched = true
		}
	}

	if matched {
		log.Debugf("Skipping excluded path: %s", relFilePath)
	}

	return matched, nil
}
