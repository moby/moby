% DOCKER(1) Docker User Manuals
% Docker Community
% JUNE 2014
# NAME
docker-cp - Copy files/folders from the PATH of the container to the HOSTPATH
of the host running the command.

# SYNOPSIS
**docker cp**
[**--help**]
CONTAINER:PATH HOSTPATH

# DESCRIPTION
Copy files/folders from a container's filesystem to the host
path running the command. Paths are relative to the root of
the filesystem. Files can be copied from a running or stopped
container.

# OPTIONS
**--help**
  Print usage statement

# EXAMPLES
An important shell script file, created in a bash shell, is copied from
the exited container to the current dir on the host:

    # docker cp c071f3c3ee81:setup.sh .

# HISTORY
April 2014, Originally compiled by William Henry (whenry at redhat dot com)
based on docker.com source material and internal work.
June 2014, updated by Sven Dowideit <SvenDowideit@home.org.au>
