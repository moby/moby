% DOCKER(1) Docker User Manuals
% Docker Community
% JUNE 2014
# NAME
docker-info - Display system-wide information

# SYNOPSIS
**docker info**


# DESCRIPTION
This command displays system wide information regarding the Docker installation.
Information displayed includes the number of containers and images, pool name,
data file, metadata file, data space used, total data space, metadata space used
, total metadata space, execution driver, and the kernel version.

The data file is where the images are stored and the metadata file is where the
meta data regarding those images are stored. When run for the first time Docker
allocates a certain amount of data space and meta data space from the space
available on the volume where `/var/lib/docker` is mounted.

# OPTIONS
There are no available options.

# EXAMPLES

## Display Docker system information

Here is a sample output:

    # docker info
    Client version: 1.1.2
    Client API version: 1.13
    Go version (client): go1.2.1
    Git commit (client): d84a070
    Server version: 1.1.2
    Server API version: 1.13
    Go version (server): go1.2.1
    Git commit (server): d84a070
    Containers: 0
    Images: 4
    Storage Driver: aufs
     Root Dir: /var/lib/docker/aufs
     Dirs: 4
    Execution Driver: native-0.2
    Kernel Version: 3.15.3-tinycore64
    Debug mode (server): true
    Debug mode (client): false
    Fds: 9
    Goroutines: 10
    EventsListeners: 0
    Init Path: /usr/local/bin/docker
    Sockets: [unix:///var/run/docker.sock]

# HISTORY
April 2014, Originally compiled by William Henry (whenry at redhat dot com)
based on docker.com source material and internal work.
June 2014, updated by Sven Dowideit <SvenDowideit@home.org.au>
