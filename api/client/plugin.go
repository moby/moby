package client

import (
	"fmt"
)

const (
	// Continuing the command allows the flow of the command to continue
	// once the flow exits the plugin.
	PLUGIN_CONTINUE_CMD = 0

	// Rewriting the command arguments allows the plugin to influence the
	// command being executed. This is useful for things such as naming,
	// addresses and more. Plugins further down the chain will see the
	// modified args.
	//
	// The payload of the associated PluginResult is expected to be of type
	// string[].
	PLUGIN_ARGS_REWRITE = 1

	// Exiting the command allows a plugin to cancel the flow of a command
	// and returning to the user. This is useful in error conditions.
	PLUGIN_EXIT_CMD = 2
)

// A PluginResult is a type for communicating actions a plugin wishes to be
// taken as well as any required information for the action in the form of
// a payload of type interface{}.
type PluginResult struct {
	Action  int
	Payload interface{}
}

// A CliPlugin type implements two methods that represent call hooks that
// may be executed either before or after the execution of the command
// being invoked by the CLI user.
type CliPlugin interface {
	Before(cli *DockerCli, cmd string, args []string) (result *PluginResult, err error)
	After(cli *DockerCli, callError error, cmd string, args []string) (err error)
}

// A CliPlugin needs a portable initialization method to allow for easier
// registration and management.
type CliPluginInit func() (plugin CliPlugin, err error)

var (
	// All registered plugins
	plugins map[string]CliPluginInit
)

func init() {
	plugins = make(map[string]CliPluginInit)
}

// Finds a CliPlugin with a given name. If there is no regisitered plugin
// with a matching name then an error is returned.
func findPlugin(name string) (CliPluginInit, error) {
	if initFunc, exists := plugins[name]; exists {
		return initFunc, nil
	}

	return nil, fmt.Errorf("No such plugin: %s", name)
}

// Registers a CliPlugin in order to make it available for activation and
// use.
func RegisterPlugin(name string, initFunc CliPluginInit) error {
	if _, exists := plugins[name]; exists {
		return fmt.Errorf("Name already registered %s", name)
	}

	plugins[name] = initFunc
	return nil
}

// Creates a new CliPlugin instance of the registered plugin with a name
// matching the user supplied name argument. If no plugin is found with
// a matching name, an error is returned.
func NewCliPlugin(name string) (CliPlugin, error) {
	if initFunc, err := findPlugin(name); err == nil {
		return initFunc()
	} else {
		return nil, err
	}
}
