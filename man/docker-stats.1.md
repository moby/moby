% DOCKER(1) Docker User Manuals
% Docker Community
% JUNE 2014
# NAME
docker-stats - Display a live stream of one or more containers' resource usage statistics

# SYNOPSIS
**docker stats**
[**-a**|**--all**]
[**--help**]
[**--no-stream**]
[**--totals**]
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

**--totals**=*true*|*false*
  Show total consumption of resources.

# EXAMPLES

Running `docker stats` on all running containers, and display totals in the last line:

    $ docker stats --totals
    CONTAINER           CPU %               MEM USAGE / LIMIT     MEM %               NET I/O             BLOCK I/O
    1285939c1fd3        0.07%               796 kB / 64 MB        1.21%               788 B / 648 B       3.568 MB / 512 kB
    9c76f7834ae2        0.07%               2.746 MB / 64 MB      4.29%               1.266 kB / 648 B    12.4 MB / 0 B
    d1ea048f04e4        0.03%               4.583 MB / 64 MB      6.30%               2.854 kB / 648 B    27.7 MB / 0 B
    Totals              0.17%               8.125 MB / 3.7 GB     0.21%               4.908 kB / 1.944 kB 43.7 MB / 512 kB

Running `docker stats` on multiple containers by name and id.

    $ docker stats fervent_panini 5acfcb1b4fd1
    CONTAINER           CPU %               MEM USAGE/LIMIT     MEM %               NET I/O
    5acfcb1b4fd1        0.00%               115.2 MB/1.045 GB   11.03%              1.422 kB/648 B
    fervent_panini      0.02%               11.08 MB/1.045 GB   1.06%               648 B/648 B
