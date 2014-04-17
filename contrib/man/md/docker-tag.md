% DOCKER(1) Docker User Manuals
% William Henry
% APRIL 2014
# NAME
docker-tag - Tag an image in the repository

# SYNOPSIS
**docker tag** [**-f**|**--force**[=*false*]
IMAGE [REGISTRYHOST/][USERNAME/]NAME[:TAG]

# DESCRIPTION
This will tag an image in the repository.

# "OPTIONS"
**-f**, **--force**=*true*|*false*
   When set to true, force the tag name. The default is *false*.

**REGISTRYHOST**
   The hostname of the registry if required. This may also include the port
separated by a ':'

**USERNAME**
   The username or other qualifying identifier for the image.

**NAME**
   The image name.

**TAG**
   The tag you are assigning to the image.

# EXAMPLES

## Tagging an image

Here is an example of tagging an image with the tag version1.0 :

    docker tag 0e5574283393 fedora/httpd:version1.0

## Tagging an image for a private repository

To push an image to an private registry and not the central Docker
registry you must tag it with the registry hostname and port (if needed).

    docker tag 0e5574283393 myregistryhost:5000/fedora/httpd:version1.0

# HISTORY
April 2014, Originally compiled by William Henry (whenry at redhat dot com)
based on docker.io source material and internal work.
