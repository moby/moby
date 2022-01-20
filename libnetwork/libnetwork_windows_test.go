package libnetwork_test

import (
	"os"
	"path/filepath"
)

const bridgeNetType = "nat"

var specPath = filepath.Join(os.Getenv("programdata"), "docker", "plugins")
