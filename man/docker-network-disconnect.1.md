% DOCKER(1) Docker User Manuals
% Docker Community
% OCT 2015
# NAME
docker-network-disconnect - disconnect a container from a network

# SYNOPSIS
**docker network disconnect**
[**--help**]
[**--force**]
NETWORK CONTAINER

# DESCRIPTION

Disconnects a container from a network.

```bash
  $ docker network disconnect multi-host-network container1
```


# OPTIONS
**NETWORK**
  Specify network name

**CONTAINER**
    Specify container name

**--force**
  Force the container to disconnect from a network

**--help**
  Print usage statement

# HISTORY
OCT 2015, created by Mary Anthony <mary@docker.com>
