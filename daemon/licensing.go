package daemon // import "github.com/moby/moby/daemon"

import (
	"github.com/moby/moby/api/types"
	"github.com/moby/moby/dockerversion"
)

func (daemon *Daemon) fillLicense(v *types.Info) {
	v.ProductLicense = dockerversion.DefaultProductLicense
}
