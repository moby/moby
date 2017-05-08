Removes one or more images from the host node. This does not remove images from
a registry. You cannot remove an image of a running container unless you use the
**-f** option. To see all images on a host use the **docker image ls** command.

# EXAMPLES

## Removing an image

Here is an example of removing an image:

    docker image rm fedora/httpd
