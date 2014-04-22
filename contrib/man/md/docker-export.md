% DOCKER(1) Docker User Manuals
% William Henry
% APRIL 2014
# NAME
docker-export - Export the contents of a filesystem as a tar archive to
STDOUT.

# SYNOPSIS
**docker export** CONTAINER

# DESCRIPTION
Export the contents of a container's filesystem using the full or shortened
container ID or container name. The output is exported to STDOUT and can be
redirected to a tar file.

# EXAMPLE
Export the contents of the container called angry_bell to a tar file
called test.tar:

    # docker export angry_bell > test.tar
    # ls *.tar
    test.tar

# HISTORY
April 2014, Originally compiled by William Henry (whenry at redhat dot com)
based on docker.io source material and internal work.
