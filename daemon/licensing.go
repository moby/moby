package daemon

import (
	"github.com/docker/docker/api/types/system"
	"github.com/docker/docker/daemon/dockerversion"
)

func (daemon *Daemon) fillLicense(v *system.Info) {
	v.ProductLicense = dockerversion.DefaultProductLicense
}
