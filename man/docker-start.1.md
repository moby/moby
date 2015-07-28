% DOCKER(1) Docker User Manuals
% Docker Community
% JUNE 2014
# NAME
docker-start - Start one or more stopped containers

# SYNOPSIS
**docker start**
[**-a**|**--attach**[=*false*]]
[**-D**|**--debug**[=*false*]]
[**--help**]
[**-i**|**--interactive**[=*false*]]
CONTAINER [CONTAINER...]

# DESCRIPTION

Start one or more stopped containers.

# OPTIONS
**-a**, **--attach**=*true*|*false*
   Attach container's STDOUT and STDERR and forward all signals to the process. The default is *false*.

**-D**, **--debug**=*true*|*false*
   Enable debug mode. Default is false.

**--help**
   Print usage statement

**-i**, **--interactive**=*true*|*false*
   Attach container's STDIN. The default is *false*.

# See also
**docker-stop(1)** to stop a running container.

# HISTORY
April 2014, Originally compiled by William Henry (whenry at redhat dot com)
based on docker.com source material and internal work.
June 2014, updated by Sven Dowideit <SvenDowideit@home.org.au>
