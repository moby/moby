% DOCKER(1) Docker User Manuals
% William Henry
% APRIL 2014
# NAME
docker-logout - Log the user out of a Docker registry server

# SYNOPSIS
**docker logout** [SERVER]

# DESCRIPTION
Log the user out of a docker registry server, , if no server is
specified "https://index.docker.io/v1/" is the default. If you want to
logout of a private registry you can specify this by adding the server name.

# EXAMPLE

## Logout of a local registry

    # docker logout localhost:8080

# HISTORY
April 2014, Originally compiled by William Henry (whenry at redhat dot com)
based on docker.io source material and internal work.

