% DOCKER(1) Docker User Manuals
% Docker Community
% JUNE 2014
# NAME
docker-pause - Pause all processes within a container

# SYNOPSIS
**docker pause**
[**-D**|**--debug**[=*false*]]
[**--help**]/
CONTAINER [CONTAINER...]

# DESCRIPTION

The `docker pause` command uses the cgroups freezer to suspend all processes in
a container.  Traditionally when suspending a process the `SIGSTOP` signal is
used, which is observable by the process being suspended. With the cgroups freezer
the process is unaware, and unable to capture, that it is being suspended,
and subsequently resumed.

See the [cgroups freezer documentation]
(https://www.kernel.org/doc/Documentation/cgroups/freezer-subsystem.txt) for
further details.

# OPTIONS
**-D**, **--debug**=*true*|*false*
   Enable debug mode. Default is false.

**--help**
   Print usage statement

# See also
**docker-unpause(1)** to unpause all processes within a container.

# HISTORY
June 2014, updated by Sven Dowideit <SvenDowideit@home.org.au>
