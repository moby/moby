package registry

import (
	"os"
	"path/filepath"
	"strings"
)

// DefaultV2Registry is the URI of the default (official) v2 registry.
// This is the windows-specific endpoint.
//
// Currently it is a TEMPORARY link that allows Microsoft to continue
// development of Docker Engine for Windows.
const DefaultV2Registry = "https://ms-tp3.registry-1.docker.io"

// CertsDir is the directory where certificates are stored
var CertsDir = os.Getenv("programdata") + `\docker\certs.d`

// cleanPath is used to ensure that a directory name is valid on the target
// platform. It will be passed in something *similar* to a URL such as
// https:\index.docker.io\v1. Not all platforms support directory names
// which contain those characters (such as : on Windows)
func cleanPath(s string) string {
	return filepath.FromSlash(strings.Replace(s, ":", "", -1))
}
