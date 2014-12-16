package vfs

import (
	"fmt"
	"os"

	"github.com/docker/docker/volumes/volumedriver"
	"github.com/docker/docker/volumes/volumedriver/host"
)

type Driver struct {
	*host.Driver
}

const DriverName = "vfs"

func init() {
	volumedriver.Register(DriverName, Init)
}

func Init(options map[string]string) (volumedriver.Driver, error) {
	return &Driver{&host.Driver{}}, nil
}

func (d *Driver) Remove(path string) error {
	return os.RemoveAll(path)
}

func (d *Driver) Create(path string) error {
	stat, err := os.Stat(path)
	if err != nil && os.IsNotExist(err) {
		return os.MkdirAll(path, 0700)
	}
	if err != nil {
		return err
	}

	if !stat.IsDir() {
		return fmt.Errorf("file exists at path %s, can't create %s volume", path, DriverName)
	}

	return nil
}
