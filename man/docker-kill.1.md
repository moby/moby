% DOCKER(1) Docker User Manuals
% Docker Community
% JUNE 2014
# NAME
docker-kill - Kill a running container using SIGKILL or a specified signal

# SYNOPSIS
**docker kill**
[**--help**]
[**-s**|**--signal**[=*"KILL"*]]
CONTAINER [CONTAINER...]

# DESCRIPTION

The main process inside each container specified will be sent SIGKILL,
 or any signal specified with option --signal.

If a restart policy is set on the container, the restart policy is disabled until
the next time the container is started.
When using `--signal`, if the signal is considered a "non-fatal" signal, such as
`SIGUSR1`, the restart policy is left in place.

The container's `--stop-signal` is always considered a fatal signal even when it
is normally consided non-fatal, such as `SIGUSR1`.

On Linux fatal signals are:

- SIGABRT
- SIGINT
- SIGKILL
- SIGQUIT
- SIGTERM

# OPTIONS
**--help**
  Print usage statement

**-s**, **--signal**="KILL"
   Signal to send to the container

# HISTORY
April 2014, Originally compiled by William Henry (whenry at redhat dot com)
 based on docker.com source material and internal work.
June 2014, updated by Sven Dowideit <SvenDowideit@home.org.au>
October 2015, updated by Brian Goff <cpuguy83@gmail.com>
