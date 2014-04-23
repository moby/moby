% DOCKER(1) Docker User Manuals
% William Henry
% APRIL 2014
# NAME
docker-import - Create an empty filesystem image and import the contents
of the tarball into it.

# SYNOPSIS
**docker import** URL|- [REPOSITORY[:TAG]]

# DESCRIPTION
Create a new filesystem image from the contents of a tarball (.tar,
.tar.gz, .tgz, .bzip, .tar.xz, .txz) into it, then optionally tag it.

# EXAMPLES

## Import from a remote location

    # docker import http://example.com/exampleimage.tgz example/imagerepo

## Import from a local file

Import to docker via pipe and stdin:

    # cat exampleimage.tgz | docker import - example/imagelocal

## Import from a local file and tag

Import to docker via pipe and stdin:

    # cat exampleimageV2.tgz | docker import - example/imagelocal:V-2.0

## Import from a local directory

    # tar -c . | docker import - exampleimagedir

# HISTORY
April 2014, Originally compiled by William Henry (whenry at redhat dot com)
based on docker.io source material and internal work.
