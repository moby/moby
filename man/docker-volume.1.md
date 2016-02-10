% DOCKER(1) Docker User Manuals
% Docker Community
% Feb 2016
# NAME
docker-volume - Create a new volume

# SYNOPSIS
**docker volume** [OPTIONS] COMMAND
[**--help**]

# DESCRIPTION

docker volume command manages content volumes for docker containers.

## Data volumes

A *data volume* is a specially-designated directory within one or more
containers.

Data volumes provide several useful features for persistent or shared data:

Volumes are initialized when a container is created. If the container's
base image contains data at the specified mount point, that existing data is
copied into the new volume upon volume initialization. (Note that this does
not apply when [mounting a host directory](#mount-a-host-directory-as-a-data-volume).)

Data volumes can be shared and reused among containers.

Changes to a data volume are made directly.

Changes to a data volume will not be included when you update an image.

Data volumes persist even if the container itself is deleted.

Data volumes are designed to persist data, independent of the container's life
cycle. Docker therefore *never* automatically deletes volumes when you remove
a container, nor will it "garbage collect" volumes that are no longer
referenced by a container.

# OPTIONS
**--help**
  Print usage statement

# COMMANDS
**create**
  Create a volume
  See **docker-volume-create(1)** for full documentation on the **create** command.

**inspect**
  Return low-level information on a volume
  See **docker-volume-inspect(1)** for full documentation on the **inspect** command.

**ls**
  List volumes
  See **docker-volume-ls(1)** for full documentation on the **ls** command.

**rm**
  Remove a volume
  See **docker-volume-rm(1)** for full documentation on the **rm** command.

# HISTORY
Feb 2016, created by Dan Walsh <dwalsh@redhat.com>
