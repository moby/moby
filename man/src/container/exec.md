Run a process in a running container.

The command started using `docker exec` will only run while the container's primary
process (`PID 1`) is running, and will not be restarted if the container is restarted.

If the container is paused, then the `docker exec` command will wait until the
container is unpaused, and then run

# CAPABILITIES

`privileged` gives the process extended
[Linux capabilities](http://man7.org/linux/man-pages/man7/capabilities.7.html)
when running in a container. 

Without this flag, the process run by `docker exec` in a running container has
the same capabilities as the container, which may be limited. Set
`--privileged` to give all capabilities to the process.

# USER
`user` sets the username or UID used and optionally the groupname or GID for the specified command.

   The followings examples are all valid:
   --user [user | user:group | uid | uid:gid | user:gid | uid:group ]

   Without this argument the command will be run as root in the container.
