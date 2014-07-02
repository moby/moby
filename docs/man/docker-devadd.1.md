% DOCKER(1) Docker User Manuals
% Chris Alfonso
% Ian Main
% APRIL 2014
# NAME
docker-devadd - Attach a device to a running container.

# SYNOPSIS
**docker devadd** CONTAINER DEVICE

# DESCRIPTION
The device will be added to the cgroup allowed devices. The docker process will
enter the namespace of the container and call mknod to create the device in the
container. The device will have the same mode as the host system device mode
setting.

# EXAMPLE
The /dev/loop0 is attached to a running container:

    #docker devadd 4386fb97867d /dev/loop0

# HISTORY
July 2014, Authored by Ian Main and Chris Alfonso, based upon work from Timothy
Hobbs.
