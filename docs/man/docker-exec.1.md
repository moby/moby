% DOCKER(1) Docker User Manuals
% Docker Community
% SEPT 2014
# NAME
docker-exec - Run a command in a running container

# SYNOPSIS
**docker exec**
[**-d**|**--detach**[=*false*]]
[**-i**|**--interactive**[=*false*]]
[**-t**|**--tty**[=*false*]]
 CONTAINER COMMAND [ARG...]

# DESCRIPTION

Run a process in a running container. 

# Options

**-d**, **--detach**=*true*|*false*
   Detached mode. This runs the new process in the background.

**-i**, **--interactive**=*true*|*false*
   When set to true, keep STDIN open even if not attached. The default is false.

**-t**, **--tty**=*true*|*false*
   When set to true Docker can allocate a pseudo-tty and attach to the standard
input of the process. This can be used, for example, to run a throwaway
interactive shell. The default value is false.
