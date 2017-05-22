// +build !windows

package dockerfile

import (
	"errors"
	"os"
	"path/filepath"
)

// normalizeWorkdir normalizes a user requested working directory in a
// platform semantically consistent way.
func normalizeWorkdir(_ string, current string, requested string) (string, error) {
	if requested == "" {
		return "", errors.New("cannot normalize nothing")
	}
	current = filepath.FromSlash(current)
	requested = filepath.FromSlash(requested)
	if !filepath.IsAbs(requested) {
		return filepath.Join(string(os.PathSeparator), current, requested), nil
	}
	return requested, nil
}

// equalEnvKeys compare two strings and returns true if they are equal. On
// Windows this comparison is case insensitive.
func equalEnvKeys(from, to string) bool {
	return from == to
}
