package container

import "os"

// ContainerSecret represents a secret in a container.  This gets realized
// in the container tmpfs
type ContainerSecret struct {
	Name   string
	Target string
	Data   []byte
	UID    string
	GID    string
	Mode   os.FileMode
}
