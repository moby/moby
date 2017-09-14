package volume

import (
	"errors"
	"fmt"
	"path"

	"github.com/docker/docker/api/types/mount"
)

var lcowSpecificValidators mountValidator = func(m *mount.Mount) error {
	if path.Clean(m.Target) == "/" {
		return fmt.Errorf("invalid specification: destination can't be '/'")
	}
	if m.Type == mount.TypeNamedPipe {
		return errors.New("Linux containers on Windows do not support named pipe mounts")
	}
	return nil
}

type lcowParser struct {
	windowsParser
}

func (p *lcowParser) validateMountConfig(mnt *mount.Mount) error {
	return p.validateMountConfigReg(mnt, rxLCOWDestination, lcowSpecificValidators)
}

func (p *lcowParser) ParseMountRaw(raw, volumeDriver string) (*MountPoint, error) {
	return p.parseMountRaw(raw, volumeDriver, rxLCOWDestination, false, lcowSpecificValidators)
}

func (p *lcowParser) ParseMountSpec(cfg mount.Mount) (*MountPoint, error) {
	return p.parseMountSpec(cfg, rxLCOWDestination, false, lcowSpecificValidators)
}
