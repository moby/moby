<!--[metadata]>
+++
title = "service update"
description = "The service update command description and usage"
keywords = ["service, update"]
advisory = "rc"
[menu.main]
parent = "smn_cli"
+++
<![end-metadata]-->

# service update

```Markdown
Usage:	docker service update [OPTIONS] SERVICE

Update a service

Options:
      --arg value                    Service command args (default [])
      --command value                Service command (default [])
      --constraint value             Placement constraints (default [])
      --endpoint-mode string         Endpoint mode (vip or dnsrr)
  -e, --env value                    Set environment variables (default [])
      --help                         Print usage
      --image string                 Service image tag
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

Updates a service as described by the specified parameters. This command has to be run targeting a manager node.
The parameters are the same as [`docker service create`](service_create.md). Please look at the description there
for further information.

## Examples

### Update a service

```bash
$ docker service update --limit-cpu 2 redis 
```

## Related information

* [service create](service_create.md)
* [service inspect](service_inspect.md)
* [service tasks](service_tasks.md)
* [service ls](service_ls.md)
* [service rm](service_rm.md)
