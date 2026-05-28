package mounts

func (p *linuxParser) HasResource(m *MountPoint, absolutePath string) bool {
	return false
}
