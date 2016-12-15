% DOCKER(1) Docker User Manuals
% Docker Community
% JUNE 2014
# NAME
docker-pause - Pause all processes within one or more containers

# SYNOPSIS
**docker pause**
CONTAINER [CONTAINER...]

# DESCRIPTION

The `docker pause` command suspends all processes in the specified containers.
On Linux, this uses the cgroups freezer. Traditionally, when suspending a process
the `SIGSTOP` signal is used, which is observable by the process being suspended.
With the cgroups freezer the process is unaware, and unable to capture,
that it is being suspended, and subsequently resumed. On Windows, only Hyper-V
containers can be paused.

See the [cgroups freezer documentation]
(https://www.kernel.org/doc/Documentation/cgroup-v1/freezer-subsystem.txt) for
further details.

# OPTIONS
**--help**
  Print usage statement

# See also
**docker-unpause(1)** to unpause all processes within one or more containers.

# HISTORY
June 2014, updated by Sven Dowideit <SvenDowideit@home.org.au>
