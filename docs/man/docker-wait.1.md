% DOCKER(1) Docker User Manuals
% William Henry
% APRIL 2014
# NAME
docker-wait - Block until a container stops, then print its exit code.

# SYNOPSIS
**docker wait** CONTAINER [CONTAINER...]

# DESCRIPTION
Block until a container stops, then print its exit code.

#EXAMPLE

    $ sudo docker run -d fedora sleep 99
    079b83f558a2bc52ecad6b2a5de13622d584e6bb1aea058c11b36511e85e7622
    $ sudo docker wait 079b83f558a2bc
    0

# HISTORY
April 2014, Originally compiled by William Henry (whenry at redhat dot com)
based on docker.io source material and internal work.

