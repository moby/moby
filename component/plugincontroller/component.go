package plugincontroller

import (
	"github.com/docker/docker/component"
	"github.com/docker/docker/plugin"
	"golang.org/x/net/context"
)

const name = "plugincontroller"

// Set sets the plugin controller component
func Set(s *plugin.Manager) (cancel func(), err error) {
	return component.Register(name, s)
}

// Wait waits for the plugin controller component to be available
func Wait(ctx context.Context) *plugin.Manager {
	c := component.Wait(ctx, name)
	if c == nil {
		return nil
	}

	// This could panic... but I think this is ok.
	// This should never be anything else
	return c.(*plugin.Manager)
}

func Get() *plugin.Manager {
	c := component.Get(name)
	if c == nil {
		return nil
	}
	return c.(*plugin.Manager)
}
