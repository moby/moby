% DOCKER(1) Docker User Manuals
% William Henry
% APRIL 2014
# NAME
docker-pull - Pull an image or a repository from the registry

# SYNOPSIS
**docker pull** NAME[:TAG]

# DESCRIPTION

This command pulls down an image or a repository from the registry. If
there is more than one image for a repository (e.g. fedora) then all
images for that repository name are pulled down including any tags.

# EXAMPLE

# Pull a reposiotry with multiple images

    $ sudo docker pull fedora
    Pulling repository fedora
    ad57ef8d78d7: Download complete
    105182bb5e8b: Download complete
    511136ea3c5a: Download complete
    73bd853d2ea5: Download complete

    $ sudo docker images
    REPOSITORY   TAG         IMAGE ID        CREATED      VIRTUAL SIZE
    fedora       rawhide     ad57ef8d78d7    5 days ago   359.3 MB
    fedora       20          105182bb5e8b    5 days ago   372.7 MB
    fedora       heisenbug   105182bb5e8b    5 days ago   372.7 MB
    fedora       latest      105182bb5e8b    5 days ago   372.7 MB

# HISTORY
April 2014, Originally compiled by William Henry (whenry at redhat dot com)
based on docker.io source material and internal work.

