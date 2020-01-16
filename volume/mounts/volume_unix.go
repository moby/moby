// +build linux freebsd darwin

package mounts // import "github.com/moby/moby/volume/mounts"

import (
	"fmt"
	"path/filepath"
	"strings"
)

func (p *linuxParser) HasResource(m *MountPoint, absolutePath string) bool {
	relPath, err := filepath.Rel(m.Destination, absolutePath)
	return err == nil && relPath != ".." && !strings.HasPrefix(relPath, fmt.Sprintf("..%c", filepath.Separator))
}

func (p *windowsParser) HasResource(m *MountPoint, absolutePath string) bool {
	return false
}
