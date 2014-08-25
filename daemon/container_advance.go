package daemon

import (
	"github.com/docker/docker/daemon/graphdriver/devmapper"
	"github.com/docker/docker/pkg/log"
)

func (container *Container) Sweep() error {
	device := "eth0"
	if _, err := container.daemon.ExecIn(container.ID, "ip", []string{"link", "show", device}); err != nil {
		log.Debugf("%s", err)
	} else {
		if _, err := container.daemon.ExecIn(container.ID, "ip", []string{"link", "delete", device}); err != nil {
			return err
		}
	}
	container.releaseNetwork()
	container.Config.Ip = ""
	container.Config.NetworkDisabled = true
	container.hostConfig.NetworkMode = "none"
	if err := container.ToDisk(); err != nil {
		return err
	}

	if err := container.Kill(); err != nil {
		return err
	}

	return nil
}

func (container *Container) DeviceIsBusy() (bool, error) {
	if container.daemon.driver.String() == "devicemapper" {
		driver := container.daemon.driver.(*devmapper.Driver)
		devices := driver.DeviceSet
		if opencount, err := devices.OpenCount(container.ID); err != nil {
			return false, err
		} else {
			container.daemon.eng.Logf("%s: opencount=%d", container.ID, opencount)
			return opencount != 0, nil
		}
	}
	return false, nil
}
