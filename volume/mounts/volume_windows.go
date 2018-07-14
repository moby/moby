package mounts // import "github.com/docker/docker/volume/mounts"

func (p *windowsParser) HasResource(m *MountPoint, absolutePath string) bool {
	return false
}

// NewParser creates a parser for a given container OS, depending on the current host OS (linux on a windows host will resolve to an lcowParser)
func NewParser(containerOS string) Parser {
	switch containerOS {
	case OSWindows:
		return &windowsParser{}
	}
	return &lcowParser{}
}
