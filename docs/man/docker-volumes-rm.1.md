
% Docker Community
% JUNE 2014
# NAME
docker-volumes-rm - Remove one or more volumes

# SYNOPSIS
**docker volumes rm**

# DESCRIPTION

**docker rm** will remove one or more volumes from the host node. You cannot
remove a volume which is being used by one or more volumes. Docker can also
only remove volumes which it created within the Docker root path.

# OPTIONS

# EXAMPLES

##Removing a volume##

To remove a volume, use the **docker volumes rm cmmand**:

    docker volumes rm morose_torvalds

# HISTORY
November 2014, updated by Brian Goff <cpuguy83@mail.com>
