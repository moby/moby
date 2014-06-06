% DOCKER(1) Docker User Manuals
% William Henry
% APRIL 2014
# NAME
docker-start - Restart a stopped container

# SYNOPSIS
**docker start** [**a**|**--attach**[=*false*]] [**-i**|**--interactive**
[=*true*] CONTAINER [CONTAINER...]

# DESCRIPTION

Start a stopped container.

# OPTION
**-a**, **--attach**=*true*|*false*
   When true attach to container's stdout/stderr and forward all signals to
the process

**-i**, **--interactive**=*true*|*false*
   When true attach to container's stdin

# NOTES
If run on a started container, start takes no action and succeeds
unconditionally.

# HISTORY
April 2014, Originally compiled by William Henry (whenry at redhat dot com)
based on docker.io source material and internal work.
