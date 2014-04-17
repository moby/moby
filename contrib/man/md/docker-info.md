% DOCKER(1) Docker User Manuals
% William Henry
% APRIL 2014
# NAME
docker-info - Display system wide information

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
    Containers: 18
    Images: 95
    Storage Driver: devicemapper
     Pool Name: docker-8:1-170408448-pool
     Data file: /var/lib/docker/devicemapper/devicemapper/data
     Metadata file: /var/lib/docker/devicemapper/devicemapper/metadata
     Data Space Used: 9946.3 Mb
     Data Space Total: 102400.0 Mb
     Metadata Space Used: 9.9 Mb
     Metadata Space Total: 2048.0 Mb
    Execution Driver: native-0.1
    Kernel Version: 3.10.0-116.el7.x86_64

# HISTORY
April 2014, Originally compiled by William Henry (whenry at redhat dot com)
based on docker.io source material and internal work.
