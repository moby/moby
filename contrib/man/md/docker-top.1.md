% DOCKER(1) Docker User Manuals
% William Henry
% APRIL 2014
# NAME
docker-top - Lookup the running processes of a container

# SYNOPSIS
**docker top** CONTAINER [ps-OPTION]

# DESCRIPTION

Look up the running process of the container. ps-OPTION can be any of the
 options you would pass to a Linux ps command.

# EXAMPLE

Run **docker top** with the ps option of -x:

    $ sudo docker top 8601afda2b -x
    PID      TTY       STAT       TIME         COMMAND
    16623    ?         Ss         0:00         sleep 99999


# HISTORY
April 2014, Originally compiled by William Henry (whenry at redhat dot com)
based on docker.io source material and internal work.

