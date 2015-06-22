% DOCKER(1) Docker User Manuals
% Docker Community
% JUNE 2014
# NAME
docker-exec - Run a command in a running container

# SYNOPSIS
**docker exec**
[**-d**|**--detach**[=*false*]]
[**--help**]
[**-i**|**--interactive**[=*false*]]
[**--privileged**[=*false*]]
[**-t**|**--tty**[=*false*]]
[**-u**|**--user**[=*USER*]]
CONTAINER COMMAND [ARG...]

# DESCRIPTION

Run a process in a running container.

The command started using `docker exec` will only run while the container's primary
process (`PID 1`) is running, and will not be restarted if the container is restarted.

If the container is paused, then the `docker exec` command will wait until the
container is unpaused, and then run

# OPTIONS
**-d**, **--detach**=*true*|*false*
   Detached mode: run command in the background. The default is *false*.

**--help**
  Print usage statement

**-i**, **--interactive**=*true*|*false*
   Keep STDIN open even if not attached. The default is *false*.

**--privileged**=*true*|*false*
   Give the process extended [Linux capabilities](http://man7.org/linux/man-pages/man7/capabilities.7.html)
when running in a container. The default is *false*.

   Without this flag, the process run by `docker exec` in a running container has
the same capabilities as the container, which may be limited. Set
`--privileged` to give all capabilities to the process.

**-t**, **--tty**=*true*|*false*
   Allocate a pseudo-TTY. The default is *false*.

**-u**, **--user**=""
   Sets the username or UID used and optionally the groupname or GID for the specified command.

   The followings examples are all valid:
   --user [user | user:group | uid | uid:gid | user:gid | uid:group ]

   Without this argument the command will be run as root in the container.

The **-t** option is incompatible with a redirection of the docker client
standard input.

# HISTORY
November 2014, updated by Sven Dowideit <SvenDowideit@home.org.au>
