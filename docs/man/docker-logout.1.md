% DOCKER(1) Docker User Manuals
% Daniel, Dao Quang Minh
% JUNE 2014
# NAME
docker-logout - Log out from a Docker registry

# SYNOPSIS
**docker logout** [SERVER]

# DESCRIPTION
Log the user out from a Docker registry, if no server is
specified "https://index.docker.io/v1/" is the default. If you want to
log out from a private registry you can specify this by adding the server name.

# EXAMPLE

## Log out from a local registry

    # docker logout localhost:8080

# HISTORY
June 2014, Originally compiled by Daniel, Dao Quang Minh (daniel at nitrous dot io)

