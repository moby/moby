% DOCKER(1) Docker User Manuals
% Docker Community
% JULY 2015
# NAME
docker-volume-rm - Remove a volume

# SYNOPSIS
**docker volume rm**

[OPTIONS] VOLUME [VOLUME...]

# DESCRIPTION

Removes one or more volumes. You cannot remove a volume that is in use by a container.

    ```
    $ docker volume rm hello
    hello
    ```

# OPTIONS

# HISTORY
July 2015, created by Brian Goff <cpuguy83@gmail.com>
