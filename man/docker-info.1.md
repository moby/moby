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
Information displayed includes the kernel version, number of containers and images.
The number of images shown is the number of unique images. The same image tagged
under different names is counted only once.

Depending on the storage driver in use, additional information can be shown, such
as pool name, data file, metadata file, data space used, total data space, metadata
space used, and total metadata space.

The data file is where the images are stored and the metadata file is where the
meta data regarding those images are stored. When run for the first time Docker
allocates a certain amount of data space and meta data space from the space
available on the volume where `/var/lib/docker` is mounted.

# OPTIONS
**--help**
  Print usage statement

# EXAMPLES

## Display Docker system information

Here is a sample output for a daemon running on Ubuntu, using the overlay
storage driver:

    $ docker -D info
    Containers: 14
     Running: 3
     Paused: 1
     Stopped: 10
    Images: 52
    Server Version: 1.11.1
    Storage Driver: overlay
     Backing Filesystem: extfs
    Logging Driver: json-file
    Cgroup Driver: cgroupfs
    Plugins:
     Volume: local
     Network: bridge null host
    Kernel Version: 4.4.0-21-generic
    Operating System: Ubuntu 16.04 LTS
    OSType: linux
    Architecture: x86_64
    CPUs: 24
    Total Memory: 62.86 GiB
    Name: docker
    ID: I54V:OLXT:HVMM:TPKO:JPHQ:CQCD:JNLC:O3BZ:4ZVJ:43XJ:PFHZ:6N2S
    Docker Root Dir: /var/lib/docker
    Debug mode (client): true
    Debug mode (server): true
     File Descriptors: 59
     Goroutines: 159
     System Time: 2016-04-26T10:04:06.14689342-04:00
     EventsListeners: 0
    Http Proxy: http://test:test@localhost:8080
    Https Proxy: https://test:test@localhost:8080
    No Proxy: localhost,127.0.0.1,docker-registry.somecorporation.com
    Username: svendowideit
    Registry: https://index.docker.io/v1/
    WARNING: No swap limit support
    Labels:
     storage=ssd
     staging=true
    Insecure registries:
     myinsecurehost:5000
     127.0.0.0/8

The global `-D` option tells all `docker` commands to output debug information.

The example below shows the output for a daemon running on Red Hat Enterprise Linux,
using the devicemapper storage driver. As can be seen in the output, additional
information about the devicemapper storage driver is shown:

    $ docker info
    Containers: 14
     Running: 3
     Paused: 1
     Stopped: 10
    Untagged Images: 52
    Server Version: 1.10.3
    Storage Driver: devicemapper
     Pool Name: docker-202:2-25583803-pool
     Pool Blocksize: 65.54 kB
     Base Device Size: 10.74 GB
     Backing Filesystem: xfs
     Data file: /dev/loop0
     Metadata file: /dev/loop1
     Data Space Used: 1.68 GB
     Data Space Total: 107.4 GB
     Data Space Available: 7.548 GB
     Metadata Space Used: 2.322 MB
     Metadata Space Total: 2.147 GB
     Metadata Space Available: 2.145 GB
     Udev Sync Supported: true
     Deferred Removal Enabled: false
     Deferred Deletion Enabled: false
     Deferred Deleted Device Count: 0
     Data loop file: /var/lib/docker/devicemapper/devicemapper/data
     Metadata loop file: /var/lib/docker/devicemapper/devicemapper/metadata
     Library Version: 1.02.107-RHEL7 (2015-12-01)
    Execution Driver: native-0.2
    Logging Driver: json-file
    Plugins:
     Volume: local
     Network: null host bridge
    Kernel Version: 3.10.0-327.el7.x86_64
    Operating System: Red Hat Enterprise Linux Server 7.2 (Maipo)
    OSType: linux
    Architecture: x86_64
    CPUs: 1
    Total Memory: 991.7 MiB
    Name: ip-172-30-0-91.ec2.internal
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
