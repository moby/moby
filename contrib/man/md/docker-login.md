% DOCKER(1) Docker User Manuals
% William Henry
% APRIL 2014
# NAME
docker-login - Register or Login to a docker registry server.

# SYNOPSIS
**docker login** [**-e**|**-email**=""] [**-p**|**--password**=""]
 [**-u**|**--username**=""] [SERVER]

# DESCRIPTION
Register or Login to a docker registry server, if no server is
specified "https://index.docker.io/v1/" is the default. If you want to
login to a private registry you can specify this by adding the server name.

# OPTIONS
**-e**, **--email**=""
   Email address

**-p**, **--password**=""
   Password

**-u**, **--username**=""
   Username

# EXAMPLE

## Login to a local registry

    # docker login localhost:8080

# HISTORY
April 2014, Originally compiled by William Henry (whenry at redhat dot com)
based on docker.io source material and internal work.

