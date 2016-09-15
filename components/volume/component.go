package volume

import (
	apirouter "github.com/docker/docker/api/server/router"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/component"
	"github.com/docker/docker/pkg/component/registry"
	"github.com/spf13/cobra"
)

const (
	// ComponentType is the name identifying this type of component
	ComponentType = "volumes"
)

// Volumes is the public interface for the volume component. It is used by other
// compoennts.
type Volumes interface {
	Create(name, driverName string, opts, labels map[string]string) (*types.Volume, error)
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
func (c *Component) CommandLine() []*cobra.Command {
	return nil
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
