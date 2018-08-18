package daemon // import "github.com/docker/docker/daemon"

import (
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/dockerversion"
)

func (daemon *Daemon) fillLicense(v *types.Info) {
	v.ProductLicense = dockerversion.DefaultProductLicense
}
