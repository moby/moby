// Package extensionconfig provides the daemon configuration snapshot to extensions.
package extensionconfig

import (
	"context"

	"github.com/moby/extensions"
	"github.com/moby/moby/v2/daemon/config"
	daemonconfigv0 "github.com/moby/moby/v2/extpoints/daemonconfig/v0"
)

// New declares a config provider with an immutable persistent-root snapshot.
func New(cfg *config.Config) extensions.Extension {
	return extensions.New(extensions.Declaration{
		ID: "org.mobyproject.daemon.config.v0",
		Providers: []extensions.Provider{
			daemonconfigv0.Point.Provide(snapshot{rootDir: cfg.Root}),
		},
	})
}

type snapshot struct {
	rootDir string
}

func (s snapshot) Get(context.Context, *daemonconfigv0.GetRequest) (*daemonconfigv0.GetResponse, error) {
	return &daemonconfigv0.GetResponse{RootDir: s.rootDir}, nil
}
