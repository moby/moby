% DOCKER(1) Docker User Manuals
% Docker Community
% JUNE 2014
# NAME
docker-stats - Display live container stats based on resource usage.

# SYNOPSIS
**docker top**
[**--help**]
[CONTAINERS]

# DESCRIPTION

Display live container stats based on resource usage.

# OPTIONS
**--help**
  Print usage statement

# EXAMPLES

Run **docker stats** with multiple containers.

    $ sudo docker stats redis1 redis2
    CONTAINER           CPU %               MEM USAGE/LIMIT     MEM %               NET I/O
    redis1              0.07%               796 KiB/64 MiB      1.21%               788 B/648 B
    redis2              0.07%               2.746 MiB/64 MiB    4.29%               1.266 KiB/648 B

# HISTORY
April 2014, Originally compiled by William Henry (whenry at redhat dot com)
based on docker.com source material and internal work.
June 2014, updated by Sven Dowideit <SvenDowideit@home.org.au>
