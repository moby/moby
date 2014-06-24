% DOCKER(1) Docker User Manuals
% William Henry
% APRIL 2014
# NAME
docker-rmi \- Remove one or more images.

# SYNOPSIS

**docker rmi** [**-f**|**--force**[=*false*] IMAGE [IMAGE...]

# DESCRIPTION

This will remove one or more images from the host node. This does not
remove images from a registry. You cannot remove an image of a running
container unless you use the **-f** option. To see all images on a host
use the **docker images** command.

# OPTIONS

**-f**, **--force**=*true*|*false*
   When set to true, force the removal of the image. The default is
*false*.

# EXAMPLES

## Removing an image

Here is an example of removing and image:

    docker rmi fedora/httpd

# HISTORY

April 2014, Originally compiled by William Henry (whenry at redhat dot com)
based on docker.io source material and internal work.
