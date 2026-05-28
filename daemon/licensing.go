package daemon

import (
	"github.com/moby/moby/api/types/system"
	"github.com/moby/moby/v2/dockerversion"
)

func (daemon *Daemon) fillLicense(v *system.Info) {
	v.ProductLicense = dockerversion.DefaultProductLicense
}
