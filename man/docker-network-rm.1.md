% DOCKER(1) Docker User Manuals
% Docker Community
% OCT 2015
# NAME
docker-network-rm - remove a new network

# SYNOPSIS
**docker network rm NETWORK**

[**--help**]

# DESCRIPTION

Removes a network by name or identifier. To remove a network, you must first disconnect any containers connected to it.

```
  $ docker network rm my-network
```


# OPTIONS
**NETWORK**
  Specify network name

**--help**
  Print usage statement

# HISTORY
OCT 2015, created by Mary Anthony <mary@docker.com>
