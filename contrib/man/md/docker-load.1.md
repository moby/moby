% DOCKER(1) Docker User Manuals
% William Henry
% APRIL 2014
# NAME
docker-load - Load an image from a tar archive on STDIN

# SYNOPSIS
**docker load**  **--input**=""

# DESCRIPTION

Loads a tarred repository from a file or the standard input stream.
Restores both images and tags.

# OPTIONS

**-i**, **--input**=""
   Read from a tar archive file, instead of STDIN

# EXAMPLE

    $ sudo docker images
    REPOSITORY          TAG                 IMAGE ID            CREATED             VIRTUAL SIZE
    busybox             latest              769b9341d937        7 weeks ago         2.489 MB
    $ sudo docker load --input fedora.tar
    $ sudo docker images
    REPOSITORY          TAG                 IMAGE ID            CREATED             VIRTUAL SIZE
    busybox             latest              769b9341d937        7 weeks ago         2.489 MB
    fedora              rawhide             0d20aec6529d        7 weeks ago         387 MB
    fedora              20                  58394af37342        7 weeks ago         385.5 MB
    fedora              heisenbug           58394af37342        7 weeks ago         385.5 MB
    fedora              latest              58394af37342        7 weeks ago         385.5 MB

# HISTORY
April 2014, Originally compiled by William Henry (whenry at redhat dot com)
based on docker.io source material and internal work.
