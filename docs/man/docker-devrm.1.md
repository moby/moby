% DOCKER(1) Docker User Manuals
% Chris Alfonso
% Ian Main
% APRIL 2014
# NAME
docker-devrm - Detach a device from a running container.

# SYNOPSIS
**docker devrm** CONTAINER DEVICE

# DESCRIPTION
The device will be removed from the cgroup allowed devices. The docker process
will enter the namespace of the container and unlink the device from the
container.

# EXAMPLE
The /dev/loop0 is detached from a running container:

    #docker devrm 4386fb97867d /dev/loop0

# HISTORY
July 2014, Authored by Ian Main and Chris Alfonso, based upon work from Timothy
Hobbs.
