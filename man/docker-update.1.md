% DOCKER(1) Docker User Manuals
% Docker Community
% JUNE 2014
# NAME
docker-update - Update configuration of one or more containers

# SYNOPSIS
**docker update**
[**--blkio-weight**[=*[BLKIO-WEIGHT]*]]
[**--cpu-shares**[=*0*]]
[**--cpu-period**[=*0*]]
[**--cpu-quota**[=*0*]]
[**--cpuset-cpus**[=*CPUSET-CPUS*]]
[**--cpuset-mems**[=*CPUSET-MEMS*]]
[**--help**]
[**--kernel-memory**[=*KERNEL-MEMORY*]]
[**-m**|**--memory**[=*MEMORY*]]
[**--memory-reservation**[=*MEMORY-RESERVATION*]]
[**--memory-swap**[=*MEMORY-SWAP*]]
[**--restart**[=*""*]]
CONTAINER [CONTAINER...]

# DESCRIPTION

The `docker update` command dynamically updates container configuration.
you can Use this command to prevent containers from consuming too many 
resources from their Docker host.  With a single command, you can place 
limits on a single container or on many. To specify more than one container,
provide space-separated list of container names or IDs.

With the exception of the `--kernel-memory` value, you can specify these
options on a running or a stopped container. You can only update
`--kernel-memory` on a stopped container. When you run `docker update` on
stopped container, the next time you restart it, the container uses those
values.

Another configuration you can change with this command is restart policy,
new restart policy will take effect instantly after you run `docker update`
on a container.

# OPTIONS
**--blkio-weight**=0
   Block IO weight (relative weight) accepts a weight value between 10 and 1000.

**--cpu-shares**=0
   CPU shares (relative weight)

**--cpu-period**=0
   Limit the CPU CFS (Completely Fair Scheduler) period

**--cpu-quota**=0
   Limit the CPU CFS (Completely Fair Scheduler) quota

**--cpuset-cpus**=""
   CPUs in which to allow execution (0-3, 0,1)

**--cpuset-mems**=""
   Memory nodes(MEMs) in which to allow execution (0-3, 0,1). Only effective on NUMA systems.

**--help**
   Print usage statement

**--kernel-memory**=""
   Kernel memory limit (format: `<number>[<unit>]`, where unit = b, k, m or g)

   Note that you can not update kernel memory to a running container, it can only
be updated to a stopped container, and affect after it's started.

**-m**, **--memory**=""
   Memory limit (format: <number><optional unit>, where unit = b, k, m or g)

**--memory-reservation**=""
   Memory soft limit (format: <number>[<unit>], where unit = b, k, m or g)

**--memory-swap**=""
   Total memory limit (memory + swap)

**--restart**=""
   Restart policy to apply when a container exits (no, on-failure[:max-retry], always, unless-stopped).

# EXAMPLES

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
