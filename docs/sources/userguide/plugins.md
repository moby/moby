# Plugins

Plugins provides a method for hooking into the Docker engine container lifecyle and extending engine behavior.

Libchan is used to communicate between the core engine and plugins and between plugins.  Plugins may either be in process, a sub process, or a container (future version) managed by the daemon.

## Usage

Plugins can be specified through command line flags to the daemon

- **Builtin plugin** `--plugin name "option"`
- **Subprocess plugin** `--plugin-exec "/bin/command args"`

## Example

~~~~
# Always mount /var/data in containers to a directory in /data
docker -d --plugin host-mount "/data /var/data"
~~~~

## Administrative interface
An administrative client which connects to the daemon through a unix socket using libchan can be used to configure daemon plugins at runtime without restarting the daemon process.

## Architecture
Each plugin will run separate from the engine core and only communicate through libchan.  The plugin interface for in process or external process will be the same allowing for converting plugins to be out of process without any logical code changes.  External plugins will need a process initializer which establishes the libchan connection on FD 3 and 4.  FD 3 will be reserved for inbound channels and FD 4 for outband channels.  Although libchan allows both inbound and outbound channels on the same connections, the logical separation will be used as the protocol for which type of channel to expect across each transport.

### Plugin Communication
Two models for plugin communication have been proposed.  Neither option has been decided as the final communication model and the final decision may be a combination of the two or something different.  

#### Pipelining (Option #1)
Every plugin registers a list of message types which it expects to receive.  The engine core will route messages to each plugin which has registered for that type (ordering not yet described).  Each plugin will have a handler loop which routes received messages to the appropriate code handler.  The code handler will then process the message and return a list of message back to the engine core router.  The engine core router will then continue routing returned messages.  If the returned message is the same type as that passed along, it will pass along the returned message to the next plugin instead of the original message.  This allows for easily piping the plugins together.  When a returned messaged is the same type, it will never route to itself or a plugin which has already been processed in the current pipeline.

** Challenges **

** Advantages**

#### Channel Hub (Option #2)
Every plugin registers endpoints similar to how docker/engine works today.  Every endpoint is mapped to a single function within a single plugin.  A daemon plugin to handle lifecycle hooks runs at a specific endpoint (e.g. lifecycle_hook) which can be used to register libchan receivers to lifecycle events.  Each lifecycle event may have a unique structure which the receiver will decode and a return channel to return structure modifications.  The hooks are called in order on a lifecycle event and the output of each call will be fed into the next.  Additional plugins to handle pub/sub and state would also register endpoints.  When a plugin attempts to register an endpoint which is already registered, it will receive an error, keeping conflicting plugins to run simultaneously.

** Challenges **

** Advantages**