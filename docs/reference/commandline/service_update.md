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
      --args command                       Service command args
      --constraint-add list                Add or update a placement constraint
      --constraint-rm list                 Remove a constraint
      --container-label-add list           Add or update a container label
      --container-label-rm list            Remove a container label by its key
  -d, --detach                             Exit immediately instead of waiting for the service to converge (default true)
      --dns-add list                       Add or update a custom DNS server
      --dns-option-add list                Add or update a DNS option
      --dns-option-rm list                 Remove a DNS option
      --dns-rm list                        Remove a custom DNS server
      --dns-search-add list                Add or update a custom DNS search domain
      --dns-search-rm list                 Remove a DNS search domain
      --endpoint-mode string               Endpoint mode (vip or dnsrr)
      --entrypoint command                 Overwrite the default ENTRYPOINT of the image
      --env-add list                       Add or update an environment variable
      --env-rm list                        Remove an environment variable
      --force                              Force update even if no changes require it
      --group-add list                     Add an additional supplementary user group to the container
      --group-rm list                      Remove a previously added supplementary user group from the container
      --health-cmd string                  Command to run to check health
      --health-interval duration           Time between running the check (ms|s|m|h)
      --health-retries int                 Consecutive failures needed to report unhealthy
      --health-start-period duration       Start period for the container to initialize before counting retries towards unstable (ms|s|m|h)
      --health-timeout duration            Maximum time to allow one check to run (ms|s|m|h)
      --help                               Print usage
      --host-add list                      Add or update a custom host-to-IP mapping (host:ip)
      --host-rm list                       Remove a custom host-to-IP mapping (host:ip)
      --hostname string                    Container hostname
      --image string                       Service image tag
      --label-add list                     Add or update a service label
      --label-rm list                      Remove a label by its key
      --limit-cpu decimal                  Limit CPUs
      --limit-memory bytes                 Limit Memory
      --log-driver string                  Logging driver for service
      --log-opt list                       Logging driver options
      --mount-add mount                    Add or update a mount on a service
      --mount-rm list                      Remove a mount by its target path
      --network-add list                   Add a network
      --network-rm list                    Remove a network
      --no-healthcheck                     Disable any container-specified HEALTHCHECK
      --placement-pref-add pref            Add a placement preference
      --placement-pref-rm pref             Remove a placement preference
      --publish-add port                   Add or update a published port
      --publish-rm port                    Remove a published port by its target port
  -q, --quiet                              Suppress progress output
      --read-only                          Mount the container's root filesystem as read only
      --replicas uint                      Number of tasks
      --reserve-cpu decimal                Reserve CPUs
      --reserve-memory bytes               Reserve Memory
      --restart-condition string           Restart when condition is met ("none"|"on-failure"|"any")
      --restart-delay duration             Delay between restart attempts (ns|us|ms|s|m|h)
      --restart-max-attempts uint          Maximum number of restarts before giving up
      --restart-window duration            Window used to evaluate the restart policy (ns|us|ms|s|m|h)
      --rollback                           Rollback to previous specification
      --rollback-delay duration            Delay between task rollbacks (ns|us|ms|s|m|h)
      --rollback-failure-action string     Action on rollback failure ("pause"|"continue")
      --rollback-max-failure-ratio float   Failure rate to tolerate during a rollback
      --rollback-monitor duration          Duration after each task rollback to monitor for failure (ns|us|ms|s|m|h)
      --rollback-order string              Rollback order ("start-first"|"stop-first") (default "stop-first")
      --rollback-parallelism uint          Maximum number of tasks rolled back simultaneously (0 to roll back all at once)
      --secret-add secret                  Add or update a secret on a service
      --secret-rm list                     Remove a secret
      --stop-grace-period duration         Time to wait before force killing a container (ns|us|ms|s|m|h)
      --stop-signal string                 Signal to stop the container
  -t, --tty                                Allocate a pseudo-TTY
      --update-delay duration              Delay between updates (ns|us|ms|s|m|h)
      --update-failure-action string       Action on update failure ("pause"|"continue"|"rollback")
      --update-max-failure-ratio float     Failure rate to tolerate during an update
      --update-monitor duration            Duration after each task update to monitor for failure (ns|us|ms|s|m|h)
      --update-order string                Update order ("start-first"|"stop-first")
      --update-parallelism uint            Maximum number of tasks updated simultaneously (0 to update all at once)
  -u, --user string                        Username or UID (format: <name|uid>[:<group|gid>])
      --with-registry-auth                 Send registry authentication details to swarm agents
  -w, --workdir string                     Working directory inside the container
```

## Description

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

### Add or remove mounts

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

### Rolling back to the previous version of a service 

Use the `--rollback` option to roll back to the previous version of the service. 

This will revert the service to the configuration that was in place before the most recent `docker service update` command.

The following example updates the number of replicas for the service from 4 to 5, and then rolls back to the previous configuration.

```bash
$ docker service update --replicas=5 web

web

$ docker service ls

ID            NAME  MODE        REPLICAS  IMAGE
80bvrzp6vxf3  web   replicated  0/5       nginx:alpine

```
Roll back the `web` service... 

```bash
$ docker service update --rollback web

web

$ docker service ls

ID            NAME  MODE        REPLICAS  IMAGE
80bvrzp6vxf3  web   replicated  0/4       nginx:alpine

```

Other options can be combined with `--rollback` as well, for example, `--update-delay 0s` to execute the rollback without a delay between tasks:

```bash
$ docker service update \
  --rollback \
  --update-delay 0s
  web

web

```

Services can also be set up to roll back to the previous version automatically
when an update fails. To set up a service for automatic rollback, use
`--update-failure-action=rollback`. A rollback will be triggered if the fraction
of the tasks which failed to update successfully exceeds the value given with
`--update-max-failure-ratio`.

The rate, parallelism, and other parameters of a rollback operation are
determined by the values passed with the following flags:

- `--rollback-delay`
- `--rollback-failure-action`
- `--rollback-max-failure-ratio`
- `--rollback-monitor`
- `--rollback-parallelism`

For example, a service set up with `--update-parallelism 1 --rollback-parallelism 3`
will update one task at a time during a normal update, but during a rollback, 3
tasks at a time will get rolled back. These rollback parameters are respected both
during automatic rollbacks and for rollbacks initiated manually using `--rollback`.

### Add or remove secrets

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

## Related commands

* [service create](service_create.md)
* [service inspect](service_inspect.md)
* [service logs](service_logs.md)
* [service ls](service_ls.md)
* [service ps](service_ps.md)
* [service rm](service_rm.md)
* [service scale](service_scale.md)
