# Runtime and Lifecycle

## State

Runtime MUST store container metadata on disk so that external tools can consume and act on this information.
It is recommended that this data be stored in a temporary filesystem so that it can be removed on a system reboot.
On Linux/Unix based systems the metadata MUST be stored under `/run/opencontainer/containers`.
For non-Linux/Unix based systems the location of the root metadata directory is currently undefined.
Within that directory there MUST be one directory for each container created, where the name of the directory MUST be the ID of the container.
For example: for a Linux container with an ID of `173975398351`, there will be a corresponding directory: `/run/opencontainer/containers/173975398351`.
Within each container's directory, there MUST be a JSON encoded file called `state.json` that contains the runtime state of the container.
For example: `/run/opencontainer/containers/173975398351/state.json`.

The `state.json` file MUST contain all of the following properties:

* **`version`**: (string) is the OCF specification version used when creating the container.
* **`id`**: (string) is the container's ID.
This MUST be unique across all containers on this host.
There is no requirement that it be unique across hosts.
The ID is provided in the state because hooks will be executed with the state as the payload.
This allows the hooks to perform cleanup and teardown logic after the runtime destroys its own state.
* **`pid`**: (int) is the ID of the main process within the container, as seen by the host.
* **`bundlePath`**: (string) is the absolute path to the container's bundle directory.
This is provided so that consumers can find the container's configuration and root filesystem on the host.

*Example*

```json
{
    "id": "oc-container",
    "pid": 4422,
    "root": "/containers/redis"
}
```

## Lifecycle

### Create

Creates the container: file system, namespaces, cgroups, capabilities.

### Start (process)

Runs a process in a container.
Can be invoked several times.

### Stop (process)

Not sure we need that from runc cli.
Process is killed from the outside.

This event needs to be captured by runc to run onstop event handlers.

## Hooks

See [runtime configuration for hooks](./runtime-config.md)
