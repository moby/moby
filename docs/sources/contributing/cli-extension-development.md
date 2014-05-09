page_title: Writing CLI Extensions
page_description: Guides on how to write and contribute Docker CLI extensions
page_keywords: Docker, documentation, developers, contributing, plugins, remotes, extensions, cli

# Writing a Docker CLI Extensions

## Writing Docker CLI Plugins

Docker CLI plugins allow developers to extend the client side behaviour of
Docker by hooking into a command before and after it executes. This allows
for the addition of functionality such as scheduling of resources.

Plugins are organized into an array and are executed in the order they are
specified by the user. When executed, each plugin's hook is called and its
result or error handled accordingly.

Plugin writing begins with the two plugin interfaces:

    // A CliPlugin type implements two methods that represent call hooks that
    // may be executed either before or after the execution of the command
    // being invoked by the CLI user.

    type CliPlugin interface {
        Before(cli *DockerCli, cmd string, args []string) (result *PluginResult, err error)
        After(cli *DockerCli, callError error, cmd string, args []string) (err error)
    }

    // A CliPlugin needs a portable initialization method to allow for easier
    // registration and management.
    type CliPluginInit func() (plugin *CliPlugin, err error)

## Plugin Registration

Before a plugin can be referenced and used, it must be registered with the
Docker CLI. This is done by first adding the plugin to an imports file
located at [api/client_extensions/plugins.go](https://github.com/dotcloud/docker/api/client_extensions/plugins.go).
This will pull in the plugin code during compile time.

## Plugin - Init

After the package import for the plugin has been added to Docker, the plugin
must register itself. This may be done easily from an init function.

    package example

    import (
        client "github.com/dotcloud/docker/api/client"
    )

    type MyPlugin struct {}

    func init() {
        client.RegisterPlugin("my_plugin", &MyPlugin {})
    }

## Plugin - Before

The Before method returns a PluginResult, a type that allows a plugin to
inform Docker of its desired impact on the command execution life cycle. The
following actions are available to developers.

#### client.PLUGIN_CONTINUE_CMD

    func (mp *MyPlugin) Before(cli *client.DockerCli, cmd string, args []string) (result *PluginResult, err error) {
        return &client.PluginResult {
            Action: PLUGIN_CONTINUE_CMD,
        }
    }

Continues the command execution normally. If there are plugins after this
one then their hooks are also called normally.

#### client.PLUGIN_ARGS_REWRITE

    func (mp *MyPlugin) Before(cli *client.DockerCli, cmd string, args []string) (result *PluginResult, err error) {
        return &client.PluginResult {
            Action: client.PLUGIN_ARGS_REWRITE,
            Payload: append(args, "-v")
        }
    }

Rewrites the arguments slice with the returned payload of the plugin result.

#### client.PLUGIN_EXIT_CMD

    func (mp *MyPlugin) Before(cli *client.DockerCli, cmd string, args []string) (result *PluginResult, err error) {
        return &client.PluginResult {
            Action: PLUGIN_EXIT_CMD,
        }, fmt.Errorf("MyPlugin failed.")
    }

Exits the command and returns an error, if any.

## Plugin - After

The After method returns only an error. The effects of this method are more
limited than Before simply due to the fact that After happens after the
command has executed. The After method is given the results of the command
execution as well as the command and arguments involved.

    func (mp *MyPlugin) After(cli *DockerCli, callError error, cmd string, args []string) (err error) {
        return nil
    }

## Complete Plugin Example

Putting everything together from the previous sections gives us a nice, complete example plugin.

    package example

    import (
        client "github.com/dotcloud/docker/api/client"
    )

    type MyPlugin struct {}

    func init() {
        client.RegisterPlugin("my_plugin", &MyPlugin {})
    }

    func (mp *MyPlugin) Before(cli *client.DockerCli, cmd string, args []string) (result *PluginResult, err error) {
        return &client.PluginResult {
            Action: client.PLUGIN_ARGS_REWRITE,
            Payload: append(args, "-v")
        }
    }


    func (mp *MyPlugin) After(cli *DockerCli, callError error, cmd string, args []string) (err error) {
        return nil
    }

The plugin can then be enabled by calling the Docker CLI binary with the
`DOCKER_CLI_PLUGINS` environment variable set.

    $ DOCKER_CLI_PLUGINS="my_plugin" docker ps


## Writing Docker CLI Remotes

Docker CLI Remotes are pluggable components that abstract away both the
connection and communication logic for command execution in the Docker CLI.
By writing your own remote implementation, you can control how the Docker CLI
connects to remote daemon instances and how it communicates its intent.

## Registration

Before a remote can be referenced and used, it must be registered with the
Docker CLI. This is done by first adding the remote to an imports file
located at [api/client_extensions/remotes.go](https://github.com/dotcloud/docker/api/client_extensions/remotes.go).
This will pull in the remote code during compile time.

Remote writing begins with the two plugin interfaces:

    // A CliRemote type represents a plugable component that can both create
    // network connections via the Dial method and act upon them via the Call
    // method.
    //
    // By implementing the hooks, a remote plugin can successfully route and
    // manage the outgoing command.
    type CliRemote interface {
        Call(cli *DockerCli, callDetails *CallDetails) (io.ReadCloser, int, error)
        Dial(cli *DockerCli) (net.Conn, error)
    }

    // A CliRemote needs a portable initialization method to allow for easier
    // registration and management.
    type CliRemoteInit func() (CliRemote, error)

## Remote - Init

After the package import for the remote has been added to Docker, the remote
must register itself. This may be done easily from an init function.

    package example

    import (
        "io"
        "net"
        myproto "github.com/me/myproject/myproto"
        client "github.com/dotcloud/docker/api/client"
    )

    type MyRemote struct {}

    func init() {
        client.RegisterRemote("my_remote", &MyRemote {})
    }

## Remote - Dial

The Dial method of a remote returns a live network connection or an error.
This allows for custom connection logic, wrapping and other options that
have an interest in controlling how the outgoing connection for the command
is made.

This method may be deprecated in the future when more responsibility is
refactored from the Docker CLI to plugins and remotes.

    func (mr *MyRemote) Dial(cli *client.DockerCli) (net.Conn, error) {
        if cli.TlsConfig != nil && cli.Proto != "unix" {
            return tls.Dial(cli.Proto, cli.Address, cli.TlsConfig)
        }

        return net.Dial(cli.Proto, cli.Address)
    }

## Remote - Call

The Call method of a remote handles the actual execution of the command on
behalf of the Docker CLI. The Call method receives all of the call details
through a struct that packages them up nicely.

    func (mr *MyRemote) Call(cli *client.DockerCli, callDetails *client.CallDetails) (io.ReadCloser, int, error) {
        call := myproto.Prepare(cli, callDetails)
        return call.Execute()
    }

## Complete Remote Example

Putting everything together from the previous sections gives us a nice,
complete example remote.

    package example

    import (
        "io"
        "net"
        myproto "github.com/me/myproject/myproto"
        client "github.com/dotcloud/docker/api/client"
    )

    type MyRemote struct {}

    func init() {
        client.RegisterRemote("my_remote", &MyRemote {})
    }

    func (mr *MyRemote) Dial(cli *client.DockerCli) (net.Conn, error) {
        if cli.TlsConfig != nil && cli.Proto != "unix" {
            return tls.Dial(cli.Proto, cli.Address, cli.TlsConfig)
        }

        return net.Dial(cli.Proto, cli.Address)
    }

    func (mr *MyRemote) Call(cli *client.DockerCli, callDetails *client.CallDetails) (io.ReadCloser, int, error) {
        call := myproto.Prepare(cli, callDetails)
        return call.Execute()
    }

The remote can then be enabled by calling the Docker CLI binary with the
`DOCKER_CLI_REMOTE` environment variable set.

    $ DOCKER_CLI_REMOTE="my_remote" docker ps
