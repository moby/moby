---
title: "service update"
description: "The service update command description and usage"
keywords: ["service, update"]
---

<!-- This file is maintained within the docker/docker Github
     repository at https://github.com/docker/docker/. Make all
     pull requests against that repo. If you see this file in
     another repository, consider it read-only there, as it will
     periodically be overwritten by the definitive file. Pull
     requests which include edits to this file in other repositories
     will be rejected.
-->

# service update

```Markdown
Usage:  docker service update [OPTIONS] SERVICE

Update a service

Options:
      --args string                      Service command args
      --constraint-add value             Add or update placement constraints (default [])
      --constraint-rm value              Remove a constraint (default [])
      --container-label-add value        Add or update container labels (default [])
      --container-label-rm value         Remove a container label by its key (default [])
      --endpoint-mode string             Endpoint mode (vip or dnsrr)
      --env-add value                    Add or update environment variables (default [])
      --env-rm value                     Remove an environment variable (default [])
      --force                            Force update even if no changes require it
      --group-add value                  Add additional user groups to the container (default [])
      --group-rm value                   Remove previously added user groups from the container (default [])
      --help                             Print usage
      --image string                     Service image tag
      --label-add value                  Add or update service labels (default [])
      --label-rm value                   Remove a label by its key (default [])
      --limit-cpu value                  Limit CPUs (default 0.000)
      --limit-memory value               Limit Memory (default 0 B)
      --log-driver string                Logging driver for service
      --log-opt value                    Logging driver options (default [])
      --mount-add value                  Add or update a mount on a service
      --mount-rm value                   Remove a mount by its target path (default [])
      --name string                      Service name
      --publish-add value                Add or update a published port (default [])
      --publish-rm value                 Remove a published port by its target port (default [])
      --replicas value                   Number of tasks (default none)
      --reserve-cpu value                Reserve CPUs (default 0.000)
      --reserve-memory value             Reserve Memory (default 0 B)
      --restart-condition string         Restart when condition is met (none, on-failure, or any)
      --restart-delay value              Delay between restart attempts (default none)
      --restart-max-attempts value       Maximum number of restarts before giving up (default none)
      --restart-window value             Window used to evaluate the restart policy (default none)
      --rollback                         Rollback to previous specification
      --stop-grace-period value          Time to wait before force killing a container (default none)
      --update-delay duration            Delay between updates
      --update-failure-action string     Action on update failure (pause|continue) (default "pause")
      --update-max-failure-ratio value   Failure rate to tolerate during an update
      --update-monitor duration          Duration after each task update to monitor for failure (default 0s)
      --update-parallelism uint          Maximum number of tasks updated simultaneously (0 to update all at once) (default 1)
  -u, --user string                      Username or UID (format: <name|uid>[:<group|gid>])
      --with-registry-auth               Send registry authentication details to Swarm agents
  -w, --workdir string                   Working directory inside the container
```

Updates a service as described by the specified parameters. This command has to be run targeting a manager node.
The parameters are the same as [`docker service create`](service_create.md). Please look at the description there
for further information.

Normally, updating a service will only cause the service's tasks to be replaced with new ones if a change to the
service requires recreating the tasks for it to take effect. For example, only changing the
`--update-parallelism` setting will not recreate the tasks, because the individual tasks are not affected by this
setting. However, the `--force` flag will cause the tasks to be recreated anyway. This can be used to perform a
rolling restart without any changes to the service parameters.

## Examples

### Update a service

```bash
$ docker service update --limit-cpu 2 redis
```

### Perform a rolling restart with no parameter changes

```bash
$ docker service update --force --update-parallelism 1 --update-delay 30s redis
```

In this example, the `--force` flag causes the service's tasks to be shut down
and replaced with new ones even though none of the other parameters would
normally cause that to happen. The `--update-parallelism 1` setting ensures
that only one task is replaced at a time (this is the default behavior). The
`--update-delay 30s` setting introduces a 30 second delay between tasks, so
that the rolling restart happens gradually.

### Adding and removing mounts

Use the `--mount-add` or `--mount-rm` options add or remove a service's bind-mounts
or volumes.

The following example creates a service which mounts the `test-data` volume to
`/somewhere`. The next step updates the service to also mount the `other-volume`
volume to `/somewhere-else`volume, The last step unmounts the `/somewhere` mount
point, effectively removing the `test-data` volume. Each command returns the
service name.

- The `--mount-add` flag takes the same parameters as the `--mount` flag on
  `service create`. Refer to the [volumes and
  bind-mounts](service_create.md#volumes-and-bind-mounts-mount) section in the
  `service create` reference for details.

- The `--mount-rm` flag takes the `target` path of the mount.

```bash
$ docker service create \
    --name=myservice \
    --mount \
      type=volume,source=test-data,target=/somewhere \
    nginx:alpine \
    myservice

myservice

$ docker service update \
    --mount-add \
      type=volume,source=other-volume,target=/somewhere-else \
    myservice

myservice

$ docker service update --mount-rm /somewhere myservice

myservice
```

## Related information

* [service create](service_create.md)
* [service inspect](service_inspect.md)
* [service ps](service_ps.md)
* [service ls](service_ls.md)
* [service rm](service_rm.md)
