% DOCKER(1) Docker User Manuals
% William Henry
% APRIL 2014
# NAME
docker-kill - Kill a running container (send SIGKILL, or specified signal)

# SYNOPSIS
**docker kill** **--signal**[=*"KILL"*] CONTAINER [CONTAINER...]

# DESCRIPTION

The main process inside each container specified will be sent SIGKILL,
 or any signal specified with option --signal.

# OPTIONS
**-s**, **--signal**=*"KILL"*
   Signal to send to the container

# HISTORY
April 2014, Originally compiled by William Henry (whenry at redhat dot com)
 based on docker.io source material and internal work.
