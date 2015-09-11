# Runtime and Lifecycle

## Lifecycle

### Create

Creates the container: file system, namespaces, cgroups, capabilities.

### Start (process)

Runs a process in a container. Can be invoked several times.

### Stop (process)

Not sure we need that from runc cli. Process is killed from the outside.

This event needs to be captured by runc to run onstop event handlers.

## Hooks
Hooks allow one to run code before/after various lifecycle events of the container.
The state of the container is passed to the hooks over stdin, so the hooks could get the information they need to do their work.

Hook paths are absolute and are executed from the host's filesystem.

### Pre-start
The pre-start hooks are called after the container process is spawned, but before the user supplied command is executed.
They are called after the container namespaces are created on Linux, so they provide an opportunity to customize the container.
In Linux, for e.g., the network namespace could be configured in this hook.

If a hook returns a non-zero exit code, then an error including the exit code and the stderr is returned to the caller and the container is torn down.

### Post-stop
The post-stop hooks are called after the container process is stopped. Cleanup or debugging could be performed in such a hook.
If a hook returns a non-zero exit code, then an error is logged and the remaining hooks are executed.

*Example*

```json
    "hooks" : {
        "prestart": [
            {
                "path": "/usr/bin/fix-mounts",
                "args": ["arg1", "arg2"],
                "env":  [ "key1=value1"]
            },
            {
                "path": "/usr/bin/setup-network"
            }
        ],
        "poststop": [
            {
                "path": "/usr/sbin/cleanup.sh",
                "args": ["-f"]
            }
        ]
    }
```

`path` is required for a hook. `args` and `env` are optional.
