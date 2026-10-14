// Package dropin finds containerd configuration drop-in files.
package dropin

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

// Files returns the containerd configuration files (*.toml) in dir, in
// lexical order.
// A missing directory has no files.
// Files with non-empty imports are rejected.
func Files(dir string) ([]string, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.toml"))
	if err != nil {
		return nil, fmt.Errorf("failed to find containerd config drop-ins: %w", err)
	}
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read containerd config drop-in %q: %w", path, err)
		}
		var dropIn struct {
			Imports []string `toml:"imports"`
		}
		if err := toml.Unmarshal(data, &dropIn); err != nil {
			return nil, fmt.Errorf("failed to parse containerd config drop-in %q: %w", path, err)
		}
		// containerd loads nested imports after the remaining imports of the
		// main configuration file, so they could replace settings that dockerd
		// imports after the drop-in files.
		if len(dropIn.Imports) != 0 {
			return nil, fmt.Errorf("containerd config drop-in %q must not contain imports", path)
		}
	}
	return files, nil
}
