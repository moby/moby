% DOCKER(1) Docker User Manuals
% Docker Community
% JUNE 2014
# NAME
docker-unpause - Unpause all processes within one or more containers

# SYNOPSIS
**docker unpause**
CONTAINER [CONTAINER...]

# DESCRIPTION

The `docker unpause` command un-suspends all processes in the specified containers.
On Linux, it does this using the cgroups freezer.

See the [cgroups freezer documentation]
(https://www.kernel.org/doc/Documentation/cgroup-v1/freezer-subsystem.txt) for
further details.

# OPTIONS
**--help**
  Print usage statement

# See also
**docker-pause(1)** to pause all processes within one or more containers.

# HISTORY
June 2014, updated by Sven Dowideit <SvenDowideit@home.org.au>
