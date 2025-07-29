package daemon

import (
	"github.com/moby/moby/api/types/system"

	"github.com/docker/docker/dockerversion"
)

func (daemon *Daemon) fillLicense(v *system.Info) {
	v.ProductLicense = dockerversion.DefaultProductLicense
}
