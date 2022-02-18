package registry // import "github.com/moby/moby/registry"

import (
	"os"
	"path/filepath"
	"strings"
)

// CertsDir is the directory where certificates are stored
func CertsDir() string {
	return os.Getenv("programdata") + `\docker\certs.d`
}

// cleanPath is used to ensure that a directory name is valid on the target
// platform. It will be passed in something *similar* to a URL such as
// https:\index.docker.io\v1. Not all platforms support directory names
// which contain those characters (such as : on Windows)
func cleanPath(s string) string {
	return filepath.FromSlash(strings.Replace(s, ":", "", -1))
}
