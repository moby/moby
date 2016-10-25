% DOCKER(1) Docker User Manuals
% Docker Community
% JUNE 2014
# NAME
docker-pause - Pause all processes within a container

# SYNOPSIS
**docker pause**
CONTAINER [CONTAINER...]

# DESCRIPTION

The `docker pause` command suspends all processes in a container. On Linux,
this uses the cgroups freezer. Traditionally, when suspending a process the
`SIGSTOP` signal is used, which is observable by the process being suspended.
With the cgroups freezer the process is unaware, and unable to capture,
that it is being suspended, and subsequently resumed. On Windows, only Hyper-V
containers can be paused.

See the [cgroups freezer documentation]
(https://www.kernel.org/doc/Documentation/cgroups/freezer-subsystem.txt) for
further details.

# OPTIONS
There are no available options.

# See also
**docker-unpause(1)** to unpause all processes within a container.

# HISTORY
June 2014, updated by Sven Dowideit <SvenDowideit@home.org.au>
