package component

import (
	"github.com/docker/docker/api/server/router"
	"github.com/spf13/cobra"
)

// Component is a functional unit of the Docker Engine. It provides interfaces
// for both users and others components to access the functionality.
type Component interface {
	CompType
	// Routes returns the API routes provided by the Component
	Routes() []router.Route
	// CommandLine returns the CLI commands provided by the Component
	CommandLine(*command.DockerCli) []*cobra.Command
	// Interface returns the interfaced used by other Components to access the
	// functionality of the Component
	Interface() interface{}

	// Init the Component. Init is the first lifecycle hook called on a
	// Component. Init should initialize the Component, and store references to
	// other Components by retrieving them from the registry. Other Components
	// may not have been initialized yet, so methods on the Component should not
	// be called as part of Init().
	Init(*Context, Config) error
	// Start the Component. The second lifecycle hook, Start is called
	// after all Components have been initialized.
	Start(*Context) error
	// Reload the Component. This lifecycle hook is called when the
	// daemon reloads its configuration. The new Config is passed to the
	// Component so it can update itself.
	Reload(*Context, Config) error
	// Shutdown the Component. This lifecycle hook is called when the daemon is
	// shutting down. The Component should perform any necessary cleanup.
	Shutdown(*Context) error
}

// CompType is a unique identifier for the Component. Only one component
// can be registered for each CompType.
type CompType interface {
	Provides() string
}

// Context is the context used by Componenets to run
type Context struct {
	Events Events
}

// Config is the configuration data provided to components
type Config struct {
	Filesystem FilesystemConfig

// FilesystemConfig is the configuration for the root filesystem used by
// components
type FilesystemConfig struct {
	Root string
	UID  int
	GID  int
}

// Events interface used by components to log events
type Events interface {
	Log(string, string, events.Actor)
}
