% DOCKER(1) Docker User Manuals
% Docker Community
% JUNE 2014
# NAME
docker-info - Display system-wide information

# SYNOPSIS
**docker info**
[**--help**]


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
**--help**
  Print usage statement

# EXAMPLES

## Display Docker system information

Here is a sample output:

    # docker info
    Containers: 14
     Running: 3
     Paused: 1
     Stopped: 10
    Images: 52
    Server Version: 1.9.0
    Storage Driver: aufs
     Root Dir: /var/lib/docker/aufs
     Dirs: 80
    Execution Driver: native-0.2
    Logging Driver: json-file
    Cgroup Driver: cgroupfs
    Plugins:
     Volume: local
     Network: bridge null host
    Kernel Version: 3.13.0-24-generic
    Operating System: Ubuntu 14.04 LTS
    OSType: linux
    Architecture: x86_64
    CPUs: 1
    Total Memory: 2 GiB
    Name: docker
    ID: I54V:OLXT:HVMM:TPKO:JPHQ:CQCD:JNLC:O3BZ:4ZVJ:43XJ:PFHZ:6N2S
    Docker Root Dir: /var/lib/docker
    Debug mode (client): false
    Debug mode (server): false
    Username: xyz
    Registry: https://index.docker.io/v1/
    Insecure registries:
     myinsecurehost:5000
     127.0.0.0/8
	
# HISTORY
April 2014, Originally compiled by William Henry (whenry at redhat dot com)
based on docker.com source material and internal work.
June 2014, updated by Sven Dowideit <SvenDowideit@home.org.au>
