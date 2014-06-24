% DOCKER(1) Docker User Manuals
% William Henry
% APRIL 2014
# NAME
docker-save - Save an image to a tar archive (streamed to STDOUT by default)

# SYNOPSIS
**docker save** [**-o**|**--output**=""] IMAGE

# DESCRIPTION
Produces a tarred repository to the standard output stream. Contains all
parent layers, and all tags + versions, or specified repo:tag.

Stream to a file instead of STDOUT by using **-o**.

# OPTIONS
**-o**, **--output**=""
   Write to an file, instead of STDOUT

# EXAMPLE

Save all fedora repository images to a fedora-all.tar and save the latest
fedora image to a fedora-latest.tar:

    $ sudo docker save fedora > fedora-all.tar
    $ sudo docker save --output=fedora-latest.tar fedora:latest
    $ ls -sh fedora-all.tar
    721M fedora-all.tar
    $ ls -sh fedora-latest.tar
    367M fedora-latest.tar

# HISTORY
April 2014, Originally compiled by William Henry (whenry at redhat dot com)
based on docker.io source material and internal work.

