package container

import "os"

type ContainerSecret struct {
	Name   string
	Target string
	Data   []byte
	Uid    int
	Gid    int
	Mode   os.FileMode
}
