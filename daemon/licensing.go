package daemon

import (
	"github.com/docker/docker/dockerversion"
	"github.com/moby/moby/api/types/system"
)

func (daemon *Daemon) fillLicense(v *system.Info) {
	v.ProductLicense = dockerversion.DefaultProductLicense
}
