<!--[metadata]>
+++
title = "update"
description = "The update command description and usage"
keywords = ["resources, update, dynamically"]
[menu.main]
parent = "smn_cli"
+++
<![end-metadata]-->

## update

    Usage: docker update [OPTIONS] CONTAINER [CONTAINER...]

    Update configuration of one or more containers

      --help=false               Print usage
      --blkio-weight=0           Block IO (relative weight), between 10 and 1000
      --cpu-shares=0             CPU shares (relative weight)
      --cpu-period=0             Limit the CPU CFS (Completely Fair Scheduler) period
      --cpu-quota=0              Limit the CPU CFS (Completely Fair Scheduler) quota
      --cpuset-cpus=""           CPUs in which to allow execution (0-3, 0,1)
      --cpuset-mems=""           Memory nodes (MEMs) in which to allow execution (0-3, 0,1)
      -m, --memory=""            Memory limit
      --memory-reservation=""    Memory soft limit
      --memory-swap=""           A positive integer equal to memory plus swap. Specify -1 to enable unlimited swap
      --kernel-memory=""         Kernel memory limit: container must be stopped
      --restart                  Restart policy to apply when a container exits

The `docker update` command dynamically updates container configuration.
You can use this command to prevent containers from consuming too many resources
from their Docker host.  With a single command, you can place limits on
a single container or on many. To specify more than one container, provide
space-separated list of container names or IDs.

With the exception of the `--kernel-memory` value, you can specify these
options on a running or a stopped container. You can only update
`--kernel-memory` on a stopped container. When you run `docker update` on
stopped container, the next time you restart it, the container uses those
values.

Another configuration you can change with this command is restart policy,
new restart policy will take effect instantly after you run `docker update`
on a container.

## EXAMPLES

The following sections illustrate ways to use this command.

### Update a container with cpu-shares=512

To limit a container's cpu-shares to 512, first identify the container
name or ID. You can use **docker ps** to find these values. You can also
use the ID returned from the **docker run** command.  Then, do the following:

```bash
$ docker update --cpu-shares 512 abebf7571666
```

### Update a container with cpu-shares and memory

To update multiple resource configurations for multiple containers:

```bash
$ docker update --cpu-shares 512 -m 300M abebf7571666 hopeful_morse
```

### Update a container's restart policy

To update restart policy for one or more containers:
```bash
$ docker update --restart=on-failure:3 abebf7571666 hopeful_morse
```
