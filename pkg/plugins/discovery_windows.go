package plugins // import "github.com/moby/moby/pkg/plugins"

import (
	"os"
	"path/filepath"
)

var specsPaths = []string{filepath.Join(os.Getenv("programdata"), "docker", "plugins")}
