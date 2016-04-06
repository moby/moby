package dockerignore

import (
	"bufio"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

// ReadAll reads a .dockerignore file and returns the list of file patterns
// to ignore. Note this will trim whitespace from each line as well
// as use GO's "clean" func to get the shortest/cleanest path for each.
func ReadAll(reader io.ReadCloser) ([]string, error) {
	if reader == nil {
		return nil, nil
	}
	defer reader.Close()
	scanner := bufio.NewScanner(reader)
	var excludes []string

	for scanner.Scan() {
		pattern := strings.TrimSpace(scanner.Text())
		if pattern == "" {
			continue
		}
		pattern = filepath.Clean(pattern)
		pattern = filepath.ToSlash(pattern)
		excludes = append(excludes, pattern)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("Error reading .dockerignore: %v", err)
	}
	return excludes, nil
}
