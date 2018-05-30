package graphdriver // import "github.com/docker/docker/daemon/graphdriver"

import (
	"fmt"
	"path/filepath"

	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/pkg/plugingetter"
	"github.com/docker/docker/pkg/plugins"
	"github.com/docker/docker/plugin/v2"
	"github.com/pkg/errors"
)

func lookupPlugin(name string, pg plugingetter.PluginGetter, config Options) (Driver, error) {
	if !config.ExperimentalEnabled {
		return nil, fmt.Errorf("graphdriver plugins are only supported with experimental mode")
	}
	pl, err := pg.Get(name, "GraphDriver", plugingetter.Acquire)
	if err != nil {
		return nil, fmt.Errorf("Error looking up graphdriver plugin %s: %v", name, err)
	}
	return newPluginDriver(name, pl, config)
}

func newPluginDriver(name string, pl plugingetter.CompatPlugin, config Options) (Driver, error) {
	home := config.Root
	if !pl.IsV1() {
		if p, ok := pl.(*v2.Plugin); ok {
			if p.PluginObj.Config.PropagatedMount != "" {
				home = p.PluginObj.Config.PropagatedMount
			}
		}
	}

	var proxy *graphDriverProxy

	switch pt := pl.(type) {
	case plugingetter.PluginWithV1Client:
		proxy = &graphDriverProxy{name, pl, Capabilities{}, pt.Client()}
	case plugingetter.PluginAddr:
		if pt.Protocol() != plugins.ProtocolSchemeHTTPV1 {
			return nil, errors.Errorf("plugin protocol not supported: %s", pt.Protocol())
		}
		addr := pt.Addr()
		client, err := plugins.NewClientWithTimeout(addr.Network()+"://"+addr.String(), nil, pt.Timeout())
		if err != nil {
			return nil, errors.Wrap(err, "error creating plugin client")
		}
		proxy = &graphDriverProxy{name, pl, Capabilities{}, client}
	default:
		return nil, errdefs.System(errors.Errorf("got unknown plugin type %T", pt))
	}

	return proxy, proxy.Init(filepath.Join(home, name), config.DriverOptions, config.UIDMaps, config.GIDMaps)
}
