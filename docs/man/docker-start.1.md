% DOCKER(1) Docker User Manuals
% Docker Community
% JUNE 2014
# NAME
docker-start - Restart a stopped container

# SYNOPSIS
**docker start**
[**-a**|**--attach**[=*false*]]
[**--help**]
[**-i**|**--interactive**[=*false*]]
CONTAINER [CONTAINER...]

# DESCRIPTION

Start a stopped container.

# OPTIONS
**-a**, **--attach**=*true*|*false*
   Attach container's STDOUT and STDERR and forward all signals to the process. The default is *false*.

**--help**
  Print usage statement

**-i**, **--interactive**=*true*|*false*
   Attach container's STDIN if the container's STDIN is open. The default is *false*.
 
   **-i** option only work when the container's STDIN is open. The container's STDIN is 
   opened by setting the **-i** option when the container is created use **docker create** 
   or **docker run**.

# HISTORY
April 2014, Originally compiled by William Henry (whenry at redhat dot com)
based on docker.com source material and internal work.
June 2014, updated by Sven Dowideit <SvenDowideit@home.org.au>
