package mounts // import "github.com/moby/moby/volume/mounts"

func (p *linuxParser) HasResource(m *MountPoint, absolutePath string) bool {
	return false
}
