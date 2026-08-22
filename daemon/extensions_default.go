package daemon

import (
	"github.com/moby/extensions"
	"github.com/moby/extensions/clientpoint"
	"github.com/moby/moby/v2/daemon/config"
	namesgeneratorpb "github.com/moby/moby/v2/extpoints/namesgenerator/v0/protogen"
	"github.com/moby/moby/v2/internal/namesgenerator"
)

// clientProviders lists generated client wiring for points that launched
// extensions may provide. Socket exposure is resolved locally and is not listed.
func clientProviders() []clientpoint.Registration {
	return []clientpoint.Registration{
		namesgeneratorpb.ClientPoint,
	}
}

// builtinExtensions returns the in-process extensions selected by daemon config.
func builtinExtensions(*config.Config) []extensions.Extension {
	return []extensions.Extension{
		namesgenerator.Extension,
	}
}
