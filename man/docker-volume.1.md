% DOCKER(1) Docker User Manuals
% Docker Community
% Feb 2016
# NAME
docker-volume - Create a new volume

# SYNOPSIS
**docker volume** [OPTIONS] COMMAND
[**--help**]

# DESCRIPTION

docker volume has subcommands for managing data volumes.

## Data volumes

The `docker volume` command has subcommands for managing data volumes. A data volume is a specially-designated directory that by-passes storage driver management.

Data volumes persist data independent of a container's life cycle. When you delete a container, the Engine daemon does not delete any data volumes. You can share volumes across multiple containers. Moreover, you can share data volumes with other computing resources in your system.

To see help for a subcommand, use:

```
docker volume CMD help
```

For full details on using docker volume visit Docker's online documentation.

# OPTIONS
**--help**
  Print usage statement

# COMMANDS
**create**
  Create a volume
  See **docker-volume-create(1)** for full documentation on the **create** command.

**inspect**
  Display detailed information on one or more volumes
  See **docker-volume-inspect(1)** for full documentation on the **inspect** command.

**ls**
  List volumes
  See **docker-volume-ls(1)** for full documentation on the **ls** command.

**rm**
  Remove a volume
  See **docker-volume-rm(1)** for full documentation on the **rm** command.

# HISTORY
Feb 2016, created by Dan Walsh <dwalsh@redhat.com>
