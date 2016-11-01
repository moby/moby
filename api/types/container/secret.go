package container

import "os"

type ContainerSecret struct {
	Name   string
	Target string
	Data   []byte
	UID    int
	GID    int
	Mode   os.FileMode
}
