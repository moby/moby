% DOCKER(1) Docker User Manuals
% Docker Community
% JUNE 2014
# NAME
docker-stats - Display a live stream of one or more containers' resource usage statistics

# SYNOPSIS
**docker stats**
[**-a**|**--all**[=*false*]]
[**--help**]
[**--no-stream**[=*false*]]
[CONTAINER...]

# DESCRIPTION

Display a live stream of one or more containers' resource usage statistics

# OPTIONS
**-a**, **--all**=*true*|*false*
   Show all containers. Only running containers are shown by default. The default is *false*.

**--help**
  Print usage statement

**--no-stream**=*true*|*false*
  Disable streaming stats and only pull the first result, default setting is false.

# EXAMPLES

Running `docker stats` on all running containers

    $ docker stats
    CONTAINER           CPU %               MEM USAGE / LIMIT     MEM %               NET I/O             BLOCK I/O
    redis1              0.07%               796 KB / 64 MB        1.21%               788 B / 648 B       3.568 MB / 512 KB
    redis2              0.07%               2.746 MB / 64 MB      4.29%               1.266 KB / 648 B    12.4 MB / 0 B
    nginx1              0.03%               4.583 MB / 64 MB      6.30%               2.854 KB / 648 B    27.7 MB / 0 B

Running `docker stats` on multiple containers by name and id.

    $ docker stats fervent_panini 5acfcb1b4fd1
    CONTAINER           CPU %               MEM USAGE/LIMIT     MEM %               NET I/O
    5acfcb1b4fd1        0.00%               115.2 MB/1.045 GB   11.03%              1.422 kB/648 B
    fervent_panini      0.02%               11.08 MB/1.045 GB   1.06%               648 B/648 B
