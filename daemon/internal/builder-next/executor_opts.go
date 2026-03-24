package buildkit

import (
	"github.com/moby/buildkit/executor/oci"
	"github.com/moby/buildkit/solver/llbsolver/cdidevices"
	"github.com/moby/moby/v2/daemon/libnetwork"
	"github.com/moby/sys/user"
)

// executorOpts holds options for constructing an executor. It contains fields
// used on Linux, Windows, or both.
type executorOpts struct {
	// common fields
	root              string
	networkController *libnetwork.Controller
	dnsConfig         *oci.DNSConfig
	cdiManager        *cdidevices.Manager

	// linux-only fields
	cgroupParent    string
	apparmorProfile string
	rootless        bool
	identityMapping user.IdentityMapping

	// windows-only fields
	containerdAddr      string
	containerdNamespace string
	hypervIsolation     bool
}
