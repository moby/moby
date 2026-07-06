package daemon

import (
	"context"

	"github.com/moby/moby/v2/daemon/config"
	"github.com/moby/moby/v2/internal/extensions"
	"github.com/moby/moby/v2/internal/extensions/host"
)

// setupExtensionHost builds the daemon's extension host from its own in-process
// extensions (see [builtinExtensions]). The daemon is just another host -- a
// built-in extension registers exactly like any other.
//
// Out-of-process extension binaries are layered on top of this in a later
// change; here the daemon runs only the extensions compiled into it.
func setupExtensionHost(ctx context.Context, cfg *config.Config) (*host.Host, error) {
	return host.New(ctx, host.Options{
		Extensions:      builtinExtensions(cfg),
		ExtensionConfig: extensionConfig(cfg),
	})
}

// extensionConfig is the daemon's per-extension configuration (the
// "extension-config" key in daemon.json), keyed by extension id, in the host's
// form. The host delivers each in-process extension its entry by id at Init.
func extensionConfig(cfg *config.Config) map[extensions.ExtensionID]extensions.Config {
	if len(cfg.ExtensionConfig) == 0 {
		return nil
	}
	out := make(map[extensions.ExtensionID]extensions.Config, len(cfg.ExtensionConfig))
	for id, c := range cfg.ExtensionConfig {
		out[extensions.ExtensionID(id)] = c
	}
	return out
}

// builtinExtensions returns the in-process extensions the daemon registers
// itself, selected from the daemon config. Each is an ordinary
// [extensions.Extension] value -- no func init(), no global registry -- so the
// active set is exactly this list, and config reaches each one by id through
// host.Options.ExtensionConfig.
//
// It is currently empty. NRI ([github.com/moby/moby/v2/daemon/extproviders/nri]
// .Extension) is the obvious first built-in, but it stays on the legacy
// daemon/internal/nri path for now: its create-spec bridge does not yet deliver
// container lifecycle events or state sync to plugins, exposes neither `docker
// info` status nor live reload, and still rejects the spec adjustments it cannot
// map (see the package TODOs). Routing NRI through the extension today would
// regress those, so it moves here only once the bridge reaches that parity.
func builtinExtensions(*config.Config) []extensions.Extension {
	return nil
}
