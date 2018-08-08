package daemon // import "github.com/docker/docker/daemon"

import (
	"github.com/docker/docker/api/types"
)

func (daemon *Daemon) fillLicense(v *types.Info) {
	v.ProductLicense = "Community Engine"
}
