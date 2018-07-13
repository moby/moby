package daemon

import (
	"context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"gotest.tools/poll"
)

// PluginIsRunning provides a poller to check if the specified plugin is running
func (d *Daemon) PluginIsRunning(name string) func(poll.LogT) poll.Result {
	return withClient(d, withPluginInspect(name, func(plugin *types.Plugin, t poll.LogT) poll.Result {
		if plugin.Enabled {
			return poll.Success()
		}
		return poll.Continue("plugin %q is not enabled", name)
	}))
}

// PluginIsNotRunning provides a poller to check if the specified plugin is not running
func (d *Daemon) PluginIsNotRunning(name string) func(poll.LogT) poll.Result {
	return withClient(d, withPluginInspect(name, func(plugin *types.Plugin, t poll.LogT) poll.Result {
		if !plugin.Enabled {
			return poll.Success()
		}
		return poll.Continue("plugin %q is enabled", name)
	}))
}

// PluginIsNotPresent provides a poller to check if the specified plugin is not present
func (d *Daemon) PluginIsNotPresent(name string) func(poll.LogT) poll.Result {
	return withClient(d, func(c client.APIClient, t poll.LogT) poll.Result {
		_, _, err := c.PluginInspectWithRaw(context.Background(), name)
		if client.IsErrNotFound(err) {
			return poll.Success()
		}
		if err != nil {
			return poll.Error(err)
		}
		return poll.Continue("plugin %q exists", name)
	})
}

// PluginReferenceIs provides a poller to check if the specified plugin has the specified reference
func (d *Daemon) PluginReferenceIs(name, expectedRef string) func(poll.LogT) poll.Result {
	return withClient(d, withPluginInspect(name, func(plugin *types.Plugin, t poll.LogT) poll.Result {
		if plugin.PluginReference == expectedRef {
			return poll.Success()
		}
		return poll.Continue("plugin %q reference is not %q", name, expectedRef)
	}))
}

func withPluginInspect(name string, f func(*types.Plugin, poll.LogT) poll.Result) func(client.APIClient, poll.LogT) poll.Result {
	return func(c client.APIClient, t poll.LogT) poll.Result {
		plugin, _, err := c.PluginInspectWithRaw(context.Background(), name)
		if client.IsErrNotFound(err) {
			return poll.Continue("plugin %q not found", name)
		}
		if err != nil {
			return poll.Error(err)
		}
		return f(plugin, t)
	}

}

func withClient(d *Daemon, f func(client.APIClient, poll.LogT) poll.Result) func(poll.LogT) poll.Result {
	return func(t poll.LogT) poll.Result {
		c, err := d.NewClient()
		if err != nil {
			poll.Error(err)
		}
		return f(c, t)
	}
}
