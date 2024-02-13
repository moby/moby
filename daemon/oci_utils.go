package daemon // import "github.com/docker/docker/daemon"

import (
	"github.com/docker/docker/container"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

func setLinuxDomainname(c *container.Container, s *specs.Spec) {
	// There isn't a field in the OCI for the NIS domainname, but luckily there
	// is a sysctl which has an identical effect to setdomainname(2) so there's
	// no explicit need for runtime support.
	if s.Linux == nil {
		s.Linux = &specs.Linux{}
	}
	if s.Linux.Sysctl == nil {
		s.Linux.Sysctl = make(map[string]string)
	}
	if c.Config.Domainname != "" {
		s.Linux.Sysctl["kernel.domainname"] = c.Config.Domainname
	}
}
