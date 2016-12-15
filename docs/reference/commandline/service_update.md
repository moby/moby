---
title: "service update"
description: "The service update command description and usage"
keywords: "service, update"
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
      --constraint-add list              Add or update a placement constraint (default [])
      --constraint-rm list               Remove a constraint (default [])
      --container-label-add list         Add or update a container label (default [])
      --container-label-rm list          Remove a container label by its key (default [])
      --dns-add list                     Add or update a custom DNS server (default [])
      --dns-option-add list              Add or update a DNS option (default [])
      --dns-option-rm list               Remove a DNS option (default [])
      --dns-rm list                      Remove a custom DNS server (default [])
      --dns-search-add list              Add or update a custom DNS search domain (default [])
      --dns-search-rm list               Remove a DNS search domain (default [])
      --endpoint-mode string             Endpoint mode (vip or dnsrr)
      --env-add list                     Add or update an environment variable (default [])
      --env-rm list                      Remove an environment variable (default [])
      --force                            Force update even if no changes require it
      --group-add list                   Add an additional supplementary user group to the container (default [])
      --group-rm list                    Remove a previously added supplementary user group from the container (default [])
      --health-cmd string                Command to run to check health
      --health-interval duration         Time between running the check (ns|us|ms|s|m|h)
      --health-retries int               Consecutive failures needed to report unhealthy
      --health-timeout duration          Maximum time to allow one check to run (ns|us|ms|s|m|h)
      --help                             Print usage
      --host-add list                    Add or update a custom host-to-IP mapping (host:ip) (default [])
      --host-rm list                     Remove a custom host-to-IP mapping (host:ip) (default [])
      --hostname string                  Container hostname
      --image string                     Service image tag
      --label-add list                   Add or update a service label (default [])
      --label-rm list                    Remove a label by its key (default [])
      --limit-cpu decimal                Limit CPUs (default 0.000)
      --limit-memory bytes               Limit Memory (default 0 B)
      --log-driver string                Logging driver for service
      --log-opt list                     Logging driver options (default [])
      --mount-add mount                  Add or update a mount on a service
      --mount-rm list                    Remove a mount by its target path (default [])
      --no-healthcheck                   Disable any container-specified HEALTHCHECK
      --publish-add port                 Add or update a published port
      --publish-rm port                  Remove a published port by its target port
      --replicas uint                    Number of tasks
      --reserve-cpu decimal              Reserve CPUs (default 0.000)
      --reserve-memory bytes             Reserve Memory (default 0 B)
      --restart-condition string         Restart when condition is met (none, on-failure, or any)
      --restart-delay duration           Delay between restart attempts (ns|us|ms|s|m|h)
      --restart-max-attempts uint        Maximum number of restarts before giving up
      --restart-window duration          Window used to evaluate the restart policy (ns|us|ms|s|m|h)
      --rollback                         Rollback to previous specification
      --secret-add secret                Add or update a secret on a service
      --secret-rm list                   Remove a secret (default [])
      --stop-grace-period duration       Time to wait before force killing a container (ns|us|ms|s|m|h)
  -t, --tty                              Allocate a pseudo-TTY
      --update-delay duration            Delay between updates (ns|us|ms|s|m|h) (default 0s)
      --update-failure-action string     Action on update failure (pause|continue) (default "pause")
      --update-max-failure-ratio float   Failure rate to tolerate during an update
      --update-monitor duration          Duration after each task update to monitor for failure (ns|us|ms|s|m|h) (default 0s)
      --update-parallelism uint          Maximum number of tasks updated simultaneously (0 to update all at once) (default 1)
  -u, --user string                      Username or UID (format: <name|uid>[:<group|gid>])
      --with-registry-auth               Send registry authentication details to swarm agents
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

### Adding and removing secrets

Use the `--secret-add` or `--secret-rm` options add or remove a service's
secrets.

The following example adds a secret named `ssh-2` and removes `ssh-1`:

```bash
$ docker service update \
    --secret-add source=ssh-2,target=ssh-2 \
    --secret-rm ssh-1 \
    myservice
```

### Update services using templates

Some flags of `service update` support the use of templating.
See [`service create`](./service_create.md#templating) for the reference.

## Related information

* [service create](service_create.md)
* [service inspect](service_inspect.md)
* [service logs](service_logs.md)
* [service ls](service_ls.md)
* [service ps](service_ps.md)
* [service rm](service_rm.md)
* [service scale](service_scale.md)
