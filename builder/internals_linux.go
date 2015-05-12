package builder

import (
	"github.com/docker/docker/runconfig"
)

func setPlatformSpecificHostConfig(b *Builder, hostConfig *runconfig.HostConfig) {
	hostConfig.CgroupParent = b.cgroupParent
}
