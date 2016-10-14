---
title: "service update"
description: "The service update command description and usage"
keywords: ["service, update"]
---

# service update

```Markdown
Usage:  docker service update [OPTIONS] SERVICE

Update a service

Options:
      --args string                    Service command args
      --constraint-add value           Add or update placement constraints (default [])
      --constraint-rm value            Remove a constraint (default [])
      --container-label-add value      Add or update container labels (default [])
      --container-label-rm value       Remove a container label by its key (default [])
      --endpoint-mode string           Endpoint mode (vip or dnsrr)
      --env-add value                  Add or update environment variables (default [])
      --env-rm value                   Remove an environment variable (default [])
      --group-add value                Add additional user groups to the container (default [])
      --group-rm value                 Remove previously added user groups from the container (default [])
      --help                           Print usage
      --image string                   Service image tag
      --label-add value                Add or update service labels (default [])
      --label-rm value                 Remove a label by its key (default [])
      --limit-cpu value                Limit CPUs (default 0.000)
      --limit-memory value             Limit Memory (default 0 B)
      --log-driver string              Logging driver for service
      --log-opt value                  Logging driver options (default [])
      --mount-add value                Add or update a mount on a service
      --mount-rm value                 Remove a mount by its target path (default [])
      --name string                    Service name
      --publish-add value              Add or update a published port (default [])
      --publish-rm value               Remove a published port by its target port (default [])
      --replicas value                 Number of tasks (default none)
      --reserve-cpu value              Reserve CPUs (default 0.000)
      --reserve-memory value           Reserve Memory (default 0 B)
      --restart-condition string       Restart when condition is met (none, on-failure, or any)
      --restart-delay value            Delay between restart attempts (default none)
      --restart-max-attempts value     Maximum number of restarts before giving up (default none)
      --restart-window value           Window used to evaluate the restart policy (default none)
      --stop-grace-period value        Time to wait before force killing a container (default none)
      --update-delay duration          Delay between updates
      --update-failure-action string   Action on update failure (pause|continue) (default "pause")
      --update-parallelism uint        Maximum number of tasks updated simultaneously (0 to update all at once) (default 1)
  -u, --user string                    Username or UID (format: <name|uid>[:<group|gid>])
      --with-registry-auth             Send registry authentication details to Swarm agents
  -w, --workdir string                 Working directory inside the container
```

Updates a service as described by the specified parameters. This command has to be run targeting a manager node.
The parameters are the same as [`docker service create`](service_create.md). Please look at the description there
for further information.

## Examples

### Update a service

```bash
$ docker service update --limit-cpu 2 redis
```

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
