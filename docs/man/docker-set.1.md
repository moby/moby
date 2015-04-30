% DOCKER(1) Docker User Manuals
% Docker Community
% JUNE 2014
# NAME
docker-set - Setup or update resource configs for one or more containers

# SYNOPSIS
**docker set**
[**-c**|**--cpu-shares**[=*0*]]
[**--cpuset-cpus**[=*CPUSET-CPUS*]]
[**--help**]
[**-m**|**--memory**[=*MEMORY*]]
[**--memory-swap**[=*MEMORY-SWAP*]]
CONTAINER [CONTAINER...]

# DESCRIPTION

Set resource configs for one or more containers. We can only set
containers which are running.

# OPTIONS
**-c**, **--cpu-shares**=0
   CPU shares (relative weight)

**--cpuset-cpus**=""
   CPUs in which to allow execution (0-3, 0,1)

**--cpuset-mems**=""
   MEMs in which to allow execution (0-3, 0,1). Only effective on NUMA systems.

**--cpu-quota**=0
   Limit the CPU CFS (Completely Fair Scheduler) quota

**--help**
   Print usage statement

**-m**, **--memory**=""
   Memory limit (format: <number><optional unit>, where unit = b, k, m or g)

   Allows you to constrain the memory available to a container. If the host
supports swap memory, then the **-m** memory setting can be larger than physical
RAM. If a limit of 0 is specified (not using **-m**), the container's memory is
not limited. The actual limit may be rounded up to a multiple of the operating
system's page size (the value would be very large, that's millions of trillions).

**--memory-swap**=""
   Total memory limit (memory + swap)

   Set `-1` to disable swap (format: <number><optional unit>, where unit = b, k, m or g).
This value should always larger than **-m**, so you should alway use this with **-m**.

# EXAMPLES

##Set a container with cpu-shares=512##

The container's cpu-shares could be 1024 or any other value, we can update
it after the container is created. Find container ID either from a
**docker ps**, or use the ID returned from the **docker run** command:

    # docker set -c 512 abebf7571666

##Set a container with cpu-shares and memory##

We can set multiple resource configs for one or more containers.

    # docker set -c 512 -m 300M abebf7571666 hopeful_morse
