package plugins

import (
	"os"
	"path/filepath"
)

var specPaths = []string{filepath.Join(os.Getenv("programdata"), "docker", "plugins")}
