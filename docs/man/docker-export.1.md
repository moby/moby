% DOCKER(1) Docker User Manuals
% Docker Community
% JUNE 2014
# NAME
docker-export - Export the contents of a filesystem as a tar archive to STDOUT

# SYNOPSIS
**docker export**
CONTAINER

# DESCRIPTION
Export the contents of a container's filesystem using the full or shortened
container ID or container name. The output is exported to STDOUT and can be
redirected to a tar file.

Stream to a file instead of STDOUT by using **-o**.

# OPTIONS
**-o**, **--output**=""
   Write to a file, instead of STDOUT

# EXAMPLES
Export the contents of the container called angry_bell to a tar file
called angry_bell.tar:

    # docker export angry_bell > angry_bell.tar
    # docker export --output=angry_bell-latest.tar angry_bell
    # ls -sh angry_bell.tar
    321M angry_bell.tar
    # ls -sh angry_bell-latest.tar
    321M angry_bell-latest.tar

# HISTORY
April 2014, Originally compiled by William Henry (whenry at redhat dot com)
based on docker.com source material and internal work.
June 2014, updated by Sven Dowideit <SvenDowideit@home.org.au>
Janurary 2015, updated by Joseph Kern (josephakern at gmail dot com)
