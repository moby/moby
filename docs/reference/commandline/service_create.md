<!--[metadata]>
+++
title = "service create"
description = "The service create command description and usage"
keywords = ["service, create"]
advisory = "rc"
[menu.main]
parent = "smn_cli"
+++
<![end-metadata]-->

# service create

```Markdown
Usage:	docker service create [OPTIONS] IMAGE [COMMAND] [ARG...]

Create a new service

Options:
      --constraint value             Placement constraints (default [])
      --endpoint-mode string         Endpoint mode (vip or dnsrr)
  -e, --env value                    Set environment variables (default [])
      --help                         Print usage
  -l, --label value                  Service labels (default [])
      --limit-cpu value              Limit CPUs (default 0.000)
      --limit-memory value           Limit Memory (default 0 B)
      --mode string                  Service mode (replicated or global) (default "replicated")
  -m, --mount value                  Attach a mount to the service
      --name string                  Service name
      --network value                Network attachments (default [])
  -p, --publish value                Publish a port as a node port (default [])
      --replicas value               Number of tasks (default none)
      --reserve-cpu value            Reserve CPUs (default 0.000)
      --reserve-memory value         Reserve Memory (default 0 B)
      --restart-condition string     Restart when condition is met (none, on_failure, or any)
      --restart-delay value          Delay between restart attempts (default none)
      --restart-max-attempts value   Maximum number of restarts before giving up (default none)
      --restart-window value         Window used to evaluate the restart policy (default none)
      --stop-grace-period value      Time to wait before force killing a container (default none)
      --update-delay duration        Delay between updates
      --update-parallelism uint      Maximum number of tasks updated simultaneously
  -u, --user string                  Username or UID
  -w, --workdir string               Working directory inside the container
```

Creates a service as described by the specified parameters. This command has to
be run targeting a manager node.

## Examples

### Create a service

```bash
$ docker service create --name redis redis:3.0.6
dmu1ept4cxcfe8k8lhtux3ro3

$ docker service ls
ID            NAME   REPLICAS  IMAGE        COMMAND
dmu1ept4cxcf  redis  1/1       redis:3.0.6
```

### Create a service with 5 tasks

You can set the number of tasks for a service using the `--replicas` option. The
following command creates a `redis` service with `5` tasks:

```bash
$ docker service create --name redis --replicas=5 redis:3.0.6
4cdgfyky7ozwh3htjfw0d12qv
```

The above command sets the *desired* number of tasks for the service. Even
though the command returns directly, actual scaling of the service may take
some time. The `REPLICAS` column shows both the *actual* and *desired* number
of tasks for the service.

In the following example, the desired number of tasks is set to `5`, but the
*actual* number is `3`

```bash
$ docker service ls
ID            NAME    REPLICAS  IMAGE        COMMAND
4cdgfyky7ozw  redis   3/5       redis:3.0.7
```

Once all the tasks are created, the actual number of tasks is equal to the
desired number:

```bash
$ docker service ls
ID            NAME    REPLICAS  IMAGE        COMMAND
4cdgfyky7ozw  redis   5/5       redis:3.0.7
```


### Create a service with a rolling update constraints


```bash
$ docker service create \
  --replicas 10 \
  --name redis \
  --update-delay 10s \
  --update-parallelism 2 \
  redis:3.0.6
```

When this service is [updated](service_update.md), a rolling update will update
tasks in batches of `2`, with `10s` between batches.

### Setting environment variables (-e --env)

This sets environmental variables for all tasks in a service. For example:

```bash
$ docker service create --name redis_2 --replicas 5 --env MYVAR=foo redis:3.0.6
```

### Set metadata on a service (-l --label)

A label is a `key=value` pair that applies metadata to a service. To label a
service with two labels:

```bash
$ docker service create \
  --name redis_2 \
  --label com.example.foo="bar"
  --label bar=baz \
  redis:3.0.6
```

For more information about labels, refer to [apply custom
metadata](../../userguide/labels-custom-metadata.md)

### Service mode

Is this a replicated service or a global service. A replicated service runs as
many tasks as specified, while a global service runs on each active node in the
swarm.

The following command creates a "global" service:

```bash
$ docker service create --name redis_2 --mode global redis:3.0.6
```


## Related information

* [service inspect](service_inspect.md)
* [service ls](service_ls.md)
* [service rm](service_rm.md)
* [service scale](service_scale.md)
* [service tasks](service_tasks.md)
* [service update](service_update.md)
