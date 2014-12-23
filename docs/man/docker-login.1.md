% DOCKER(1) Docker User Manuals
% Docker Community
% JUNE 2014
# NAME
docker-login - Register or log in to a Docker registry server, if no server is specified "https://index.docker.io/v1/" is the default.

# SYNOPSIS
**docker login**
[**-e**|**--email**[=*EMAIL*]]
[**-p**|**--password**[=*PASSWORD*]]
[**-u**|**--username**[=*USERNAME*]]
[SERVER]

# DESCRIPTION
Register or Login to a docker registry server, if no server is
specified "https://index.docker.io/v1/" is the default. If you want to
login to a private registry you can specify this by adding the server name.

# OPTIONS
**-e**, **--email**=""
   Email

**-p**, **--password**=""
   Password

**-u**, **--username**=""
   Username

# EXAMPLES

## Login to a local registry

    # docker login localhost:8080

# HISTORY
April 2014, Originally compiled by William Henry (whenry at redhat dot com)
based on docker.com source material and internal work.
June 2014, updated by Sven Dowideit <SvenDowideit@home.org.au>
