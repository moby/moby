package daemon

import (
	"github.com/moby/moby/api/types/system"
	"github.com/moby/moby/daemon/dockerversion"
)

func (daemon *Daemon) fillLicense(v *system.Info) {
	v.ProductLicense = dockerversion.DefaultProductLicense
}
