% DOCKER(1) Docker User Manuals
% Docker Community
% SEPT 2014
# NAME
docker-modify - Modify a running container

# SYNOPSIS
**docker modify**
 CONTAINER ACTION ARGUMENTS...

# DESCRIPTION

Modifies the configuration of a running container.  At present this only supports
adding and removing devices from the container.

# ACTION

The action can be one of:

**device-add** Adds a new device to the container.  The permissions and device
information are gathered from the host filesystem.

**device-remove** Removes a device from the container.

# ARGUMENTS

The arguments are a list of one or more devices, separated by a comma, that will
be added or removed from the container.

<host device file>:[<container device file>]:[<cgroup permissions>]

Note that the host device file is required even when removing because
the device information must be gathered from the host.

# EXAMPLES

docker modify happy_bell device-add /dev/loop0

docker modify happy_bell device-add /dev/loop0:/dev/sd0

docker modify happy_bell device-add /dev/loop0:/dev/sd0:rwm

docker modify happy_bell device-add /dev/loop0:/dev/sd0:rwm,/dev/loop1:/dev/sd1:rwm

docker modify happy_bell device-remove /dev/loop0,/dev/loop1


