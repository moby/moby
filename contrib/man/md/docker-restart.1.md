% DOCKER(1) Docker User Manuals
% William Henry
% APRIL 2014
# NAME
docker-restart - Restart a running container

# SYNOPSIS
**docker restart** [**-t**|**--time**[=*10*]] CONTAINER [CONTAINER...]

# DESCRIPTION
Restart each container listed.

# OPTIONS
**-t**, **--time**=NUM
   Number of seconds to try to stop for before killing the container. Once
killed it will then be restarted. Default=10

# HISTORY
April 2014, Originally compiled by William Henry (whenry at redhat dot com)
based on docker.io source material and internal work.

