package volume

import (
	apirouter "github.com/docker/docker/api/server/router"
	clicommand "github.com/docker/docker/cli/command"
	"github.com/docker/docker/components/volume/command"
	"github.com/docker/docker/pkg/component"
	"github.com/docker/docker/pkg/component/registry"
	"github.com/spf13/cobra"

	// TODO: move this under the component
	"github.com/docker/docker/volume"
)

const (
	// ComponentType is the name identifying this type of component
	ComponentType = "volumes"
)

// Volumes is the public interface for the volume component. It is used by other
// compoennts.
type Volumes interface {
	Create(name, driverName, ref string, opts, labels map[string]string) (volume.Volume, error)
	GetWithRef(name, driver, ref string) (volume.Volume, error)
	Dereference(volume.Volume, string)
	Remove(volume.Volume) error
}

// Component provides volume functionality to the engine
type Component struct {
	backend *backend
	router  *router
}

// Provides returns the component type
func (c *Component) Provides() string {
	return ComponentType
}

// Routes returns the api routes provided by this component
func (c *Component) Routes() []apirouter.Route {
	return c.router.Routes()
}

// CommandLine returns the cli commands provided by this component
func (c *Component) CommandLine(dockerCli *clicommand.DockerCli) []*cobra.Command {
	return []*cobra.Command{command.NewVolumeCommand(dockerCli)}
}

// Interface returns the Volumes interface for other components. It must be
// casted to the correct type.
func (c *Component) Interface() interface{} {
	return c.backend
}

// Init initializes the component
func (c *Component) Init(context *component.Context, config component.Config) error {
	return c.backend.init(context, config)
}

// Start the component using the context
func (c *Component) Start(context *component.Context) error {
	return nil
}

// Reload the component
func (c *Component) Reload(context *component.Context, conf component.Config) error {
	return nil
}

// Shutdown the component
func (c *Component) Shutdown(context *component.Context) error {
	return nil
}

// New returns a new component
func New() *Component {
	b := &backend{}
	return &Component{router: newRouter(b), backend: b}
}

func init() {
	registry.Get().Register(New())
}
