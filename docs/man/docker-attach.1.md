% DOCKER(1) Docker User Manuals
% Docker Community
% JUNE 2014
# NAME
docker-attach - Attach to a running container

# SYNOPSIS
**docker attach**
[**--no-stdin**[=*false*]]
[**--sig-proxy**[=*true*]]
CONTAINER

# DESCRIPTION
If you **docker run** a container in detached mode (**-d**), you can reattach to
the detached container with **docker attach** using the container's ID or name.

You can detach from the container again (and leave it running) with `CTRL-p 
CTRL-q` (for a quiet exit), or `CTRL-c`  which will send a SIGKILL to the
container, or `CTRL-\` to get a stacktrace of the Docker client when it quits.
When you detach from a container the exit code will be returned to
the client.

# OPTIONS
**--no-stdin**=*true*|*false*
   Do not attach STDIN. The default is *false*.

**--sig-proxy**=*true*|*false*
   Proxy all received signals to the process (non-TTY mode only). SIGCHLD, SIGKILL, and SIGSTOP are not proxied. The default is *true*.

# EXAMPLES

## Attaching to a container

In this example the top command is run inside a container, from an image called
fedora, in detached mode. The ID from the container is passed into the **docker
attach** command:

    # ID=$(sudo docker run -d fedora /usr/bin/top -b)
    # sudo docker attach $ID
    top - 02:05:52 up  3:05,  0 users,  load average: 0.01, 0.02, 0.05
    Tasks:   1 total,   1 running,   0 sleeping,   0 stopped,   0 zombie
    Cpu(s):  0.1%us,  0.2%sy,  0.0%ni, 99.7%id,  0.0%wa,  0.0%hi,  0.0%si,  0.0%st
    Mem:    373572k total,   355560k used,    18012k free,    27872k buffers
    Swap:   786428k total,        0k used,   786428k free,   221740k cached

    PID USER      PR  NI  VIRT  RES  SHR S %CPU %MEM    TIME+  COMMAND
    1 root      20   0 17200 1116  912 R    0  0.3   0:00.03 top

    top - 02:05:55 up  3:05,  0 users,  load average: 0.01, 0.02, 0.05
    Tasks:   1 total,   1 running,   0 sleeping,   0 stopped,   0 zombie
    Cpu(s):  0.0%us,  0.2%sy,  0.0%ni, 99.8%id,  0.0%wa,  0.0%hi,  0.0%si,  0.0%st
    Mem:    373572k total,   355244k used,    18328k free,    27872k buffers
    Swap:   786428k total,        0k used,   786428k free,   221776k cached

    PID USER      PR  NI  VIRT  RES  SHR S %CPU %MEM    TIME+  COMMAND
    1 root      20   0 17208 1144  932 R    0  0.3   0:00.03 top

# HISTORY
April 2014, Originally compiled by William Henry (whenry at redhat dot com)
based on docker.com source material and internal work.
June 2014, updated by Sven Dowideit <SvenDowideit@home.org.au>
