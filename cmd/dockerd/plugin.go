package main

import (
	"context"
	"encoding/json"

	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/plugin"
	"github.com/docker/docker/daemon/config"
	"github.com/pkg/errors"
)

// initPlugins initializes plugins using containerd's plugin system,
// registering any services with the daemon's grpc servers
func (cli *DaemonCli) initPlugins(ctx context.Context, c *config.Config) error {
	plugins := plugin.Graph(nil) // TODO: Allow disabled plugins
	initialized := plugin.NewPluginSet()
	for _, p := range plugins {
		id := p.URI()
		log.G(ctx).WithField("type", p.Type).Infof("loading plugin %q...", id)

		initContext := plugin.NewContext(
			ctx,
			p,
			initialized,
			c.Root,
			"",
		)

		// load the plugin specific configuration if it is provided
		if p.Config != nil {
			cc, ok := c.Plugins[id]
			if ok {
				err := json.Unmarshal([]byte(cc), p.Config)
				if err != nil {
					return err
				}
			}
			initContext.Config = p.Config
		}
		result := p.Init(initContext)
		if err := initialized.Add(result); err != nil {
			return errors.Wrapf(err, "could not add plugin result to plugin set")
		}

		instance, err := result.Instance()
		if err != nil {
			if plugin.IsSkipPlugin(err) {
				log.G(ctx).WithError(err).WithField("type", p.Type).Infof("skip loading plugin %q...", id)
			} else {
				log.G(ctx).WithError(err).Warnf("failed to load plugin %s", id)
			}

			// TODO: Check if required
			continue
		}

		if svc, ok := instance.(plugin.Service); ok {
			if cli.grpcServer != nil {
				svc.Register(cli.grpcServer)
			} else {
				log.G(ctx).Warnf("no gRPC server to register %s", id)
			}
		}
		if svc, ok := instance.(plugin.TCPService); ok {
			svc.RegisterTCP(cli.tcpServer)
		}

		cli.plugins = append(cli.plugins, result)
	}
	// TODO: Check if any unloaded required plugins

	return nil
}
