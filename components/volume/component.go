package volume

import (
	apirouter "github.com/docker/docker/api/server/router"
	clicommand "github.com/docker/docker/cli/command"
	"github.com/docker/docker/component"
	"github.com/docker/docker/component/registry"
	"github.com/docker/docker/components/volume/command"
	"github.com/docker/docker/components/volume/types"
	"github.com/spf13/cobra"
)

// volumeComponent provides volume functionality to the engine
type volumeComponent struct {
	backend *backend
	router  *router
}

// Provides returns the component type
func (c *volumeComponent) Provides() string {
	return types.ComponentType
}

// Routes returns the api routes provided by this component
func (c *volumeComponent) Routes() []apirouter.Route {
	return c.router.Routes()
}

// CommandLine returns the cli commands provided by this component
func (c *volumeComponent) CommandLine(dockerCli *clicommand.DockerCli) []*cobra.Command {
	return []*cobra.Command{command.NewVolumeCommand(dockerCli)}
}

// Interface returns the Volumes interface for other components. It must be
// casted to the correct type.
func (c *volumeComponent) Interface() interface{} {
	return c.backend
}

// Init initializes the component
func (c *volumeComponent) Init(context *component.Context, config component.Config) error {
	return c.backend.init(context, config)
}

// Start the component using the context
func (c *volumeComponent) Start(context *component.Context) error {
	return nil
}

// Reload the component
func (c *volumeComponent) Reload(context *component.Context, conf component.Config) error {
	return nil
}

// Shutdown the component
func (c *volumeComponent) Shutdown(context *component.Context) error {
	return nil
}

// New returns a new component
func New() component.Component {
	b := &backend{}
	return &volumeComponent{router: newRouter(b), backend: b}
}

func init() {
	registry.Get().Register(New())
}
