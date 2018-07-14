// +build linux freebsd darwin

package mounts // import "github.com/docker/docker/volume/mounts"

import (
	"fmt"
	"path/filepath"
	"strings"
)

func (p *linuxParser) HasResource(m *MountPoint, absolutePath string) bool {
	relPath, err := filepath.Rel(m.Destination, absolutePath)
	return err == nil && relPath != ".." && !strings.HasPrefix(relPath, fmt.Sprintf("..%c", filepath.Separator))
}

// NewParser creates a parser for a given container OS, depending on the current host OS (linux on a windows host will resolve to an lcowParser)
func NewParser(containerOS string) Parser {
	return &linuxParser{}
}
