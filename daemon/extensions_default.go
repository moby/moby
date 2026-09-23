package daemon

import (
	"github.com/moby/extensions"
	"github.com/moby/extensions/clientpoint"
	"github.com/moby/extensions/serverpoint"
	"github.com/moby/moby/v2/daemon/internal/extensionconfig"

	containernamegeneratorpb "github.com/moby/moby/v2/extpoints/containernamegenerator/v0/protogen"
	daemonconfigpb "github.com/moby/moby/v2/extpoints/daemonconfig/v0/protogen"
	servicenamegeneratorpb "github.com/moby/moby/v2/extpoints/servicenamegenerator/v0/protogen"
	namesgeneratorlegacy "github.com/moby/moby/v2/internal/namesgenerator/legacy"
)

// builtinExtensions lists in-tree extensions that should be available in the
// engine by default.
var builtinExtensions = []extensions.Extension{
	namesgeneratorlegacy.Extension,
}

// daemonExtensions declares extensions that need daemon-owned state.
// Ordinary builtin definitions obtain their bootstrap settings through points.
func (daemon *Daemon) daemonExtensions() []extensions.Extension {
	cfg := daemon.Config()
	return []extensions.Extension{
		extensionconfig.New(&cfg),
	}
}

// clientProviders lists generated client wiring for points that launched
// extensions may provide.
//
// A point that's simply exposed as a service by an extension DOES NOT need to
// be in this list.
// This list only contains points that the daemon calls directly.
func clientProviders() []clientpoint.Registration {
	return []clientpoint.Registration{
		containernamegeneratorpb.ClientPoint,
		servicenamegeneratorpb.ClientPoint,
	}
}

// dependencyProviders lists points that external extensions may call back
// into.
// Points not in this list will not be able to used as dependencies.
func dependencyProviders() []serverpoint.Registration {
	return []serverpoint.Registration{
		daemonconfigpb.ServerPoint,
	}
}
