:title: Run Command
:description: Run a command in a new container
:keywords: run, container, docker, documentation 

===========================================
``run`` -- Run a command in a new container
===========================================

::

    Usage: docker run [OPTIONS] IMAGE [COMMAND] [ARG...]

    Run a command in a new container

      -a=map[]: Attach to stdin, stdout or stderr.
      -c=0: CPU shares (relative weight)
      -d=false: Detached mode: leave the container running in the background
      -e=[]: Set environment variables
      -h="": Container host name
      -i=false: Keep stdin open even if not attached
      -m=0: Memory limit (in bytes)
      -p=[]: Map a network port to the container
      -t=false: Allocate a pseudo-tty
      -u="": Username or UID
      -d=[]: Set custom dns servers for the container
      -v=[]: Creates a new volume and mounts it at the specified path.
      -volumes-from="": Mount all volumes from the given container.
      -b=[]: Create a bind mount with: [host-dir]:[container-dir]:[rw|ro]
      -entrypoint="": Overwrite the default entrypoint set by the image.
