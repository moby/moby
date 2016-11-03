package container

import "os"

type ContainerSecret struct {
	Name   string
	Target string
	Data   []byte
	UID    string
	GID    string
	Mode   os.FileMode
}
