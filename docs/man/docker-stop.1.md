% DOCKER(1) Docker User Manuals
% William Henry
% APRIL 2014
# NAME
docker-stop - Stop a running container
 grace period)

# SYNOPSIS
**docker stop** [**-t**|**--time**[=*10*]] CONTAINER [CONTAINER...]

# DESCRIPTION
Stop a running container (Send SIGTERM, and then SIGKILL after
 grace period)

# OPTIONS
**-t**, **--time**=NUM
   Wait NUM number of seconds for the container to stop before killing it.
The default is 10 seconds.

# HISTORY
April 2014, Originally compiled by William Henry (whenry at redhat dot com)
based on docker.io source material and internal work.
