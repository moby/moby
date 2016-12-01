// +build !windows

package dockerfile

import (
	"fmt"
	"os"
	"path/filepath"
)

// normaliseWorkdir normalises a user requested working directory in a
// platform sematically consistent way.
func normaliseWorkdir(current string, requested string) (string, error) {
	if requested == "" {
		return "", fmt.Errorf("cannot normalise nothing")
	}
	current = filepath.FromSlash(current)
	requested = filepath.FromSlash(requested)
	if !filepath.IsAbs(requested) {
		return filepath.Join(string(os.PathSeparator), current, requested), nil
	}
	return requested, nil
}

func errNotJSON(command, _ string) error {
	return fmt.Errorf("%s requires the arguments to be in JSON form", command)
}

// GETENV
//
// GETENV gets the environment variables from the container to synchronise
// back to the image configuration. This is not implemented on *nix platforms.
//
func getenv(b *Builder, args []string, attributes map[string]bool, original string) error {
	return fmt.Errorf("GETENV not implemented")
}
