% DOCKER(1) Docker User Manuals
% Docker Community
% JUNE 2014
# NAME
docker-start - Restart a stopped container

# SYNOPSIS
**docker start**
[**-a**|**--attach**[=*false*]]
[**-i**|**--interactive**[=*false*]]
CONTAINER [CONTAINER...]

# DESCRIPTION

Start a stopped container.

# OPTIONS
**-a**, **--attach**=*true*|*false*
   Attach container's STDOUT and STDERR and forward all signals to the process. The default is *false*.

**-i**, **--interactive**=*true*|*false*
   Attach container's STDIN. The default is *false*.

# HISTORY
April 2014, Originally compiled by William Henry (whenry at redhat dot com)
based on docker.com source material and internal work.
June 2014, updated by Sven Dowideit <SvenDowideit@home.org.au>
