:title: Command Line Interface
:description: Docker's CLI command description and usage
:keywords: Docker, Docker documentation, CLI, command line


Command Line Interface
======================

Docker Usage
~~~~~~~~~~~~

::

  $ docker
    Usage: docker COMMAND [arg...]

    A self-sufficient runtime for linux containers.

    Commands:
        attach    Attach to a running container
        commit    Create a new image from a container's changes
        diff      Inspect changes on a container's filesystem
        export    Stream the contents of a container as a tar archive
        history   Show the history of an image
        images    List images
        import    Create a new filesystem image from the contents of a tarball
        info      Display system-wide information
        inspect   Return low-level information on a container
        kill      Kill a running container
        login     Register or Login to the docker registry server
        logs      Fetch the logs of a container
        port      Lookup the public-facing port which is NAT-ed to PRIVATE_PORT
        ps        List containers
        pull      Pull an image or a repository to the docker registry server
        push      Push an image or a repository to the docker registry server
        restart   Restart a running container
        rm        Remove a container
        rmi       Remove an image
        run       Run a command in a new container
        start     Start a stopped container
        stop      Stop a running container
        tag       Tag an image into a repository
        version   Show the docker version information
        wait      Block until a container stops, then print its exit code


attach
~~~~~~

::

  Usage: docker attach [OPTIONS]

  Attach to a running container

    -e=true: Attach to stderr
    -i=false: Attach to stdin
    -o=true: Attach to stdout


commit
~~~~~~

::

  Usage: docker commit [OPTIONS] CONTAINER [DEST]

  Create a new image from a container's changes

  -m="": Commit message


diff
~~~~

::

  Usage: docker diff CONTAINER [OPTIONS]

  Inspect changes on a container's filesystem


export
~~~~~~

::

    Usage: docker export CONTAINER

    Export the contents of a filesystem as a tar archive


history
~~~~~~~

::

    Usage: docker history [OPTIONS] IMAGE

    Show the history of an image


images
~~~~~~

::

  Usage: docker images [OPTIONS] [NAME]

  List images

    -a=false: show all images
    -q=false: only show numeric IDs


import
~~~~~~

::

Usage: docker import [OPTIONS] URL|- [REPOSITORY [TAG]]

Create a new filesystem image from the contents of a tarball


info
~~~~

::

  Usage: docker info

  Display system-wide information.


inspect
~~~~~~~

::

  Usage: docker inspect [OPTIONS] CONTAINER

  Return low-level information on a container


kill
~~~~

::

  Usage: docker kill [OPTIONS] CONTAINER [CONTAINER...]

  Kill a running container


login
~~~~~

::

  Usage: docker login

  Register or Login to the docker registry server


logs
~~~~

::

  Usage: docker logs [OPTIONS] CONTAINER

  Fetch the logs of a container


port
~~~~

::

    Usage: docker port [OPTIONS] CONTAINER PRIVATE_PORT

    Lookup the public-facing port which is NAT-ed to PRIVATE_PORT


ps
~~

::

    Usage: docker ps [OPTIONS]

    List containers

      -a=false: Show all containers. Only running containers are shown by default.
      -notrunc=false: Don't truncate output
      -q=false: Only display numeric IDs


pull
~~~~

::

    Usage: docker pull NAME

    Pull an image or a repository from the registry

push
~~~~

::

    Usage: docker push NAME

    Push an image or a repository to the registry


restart
~~~~~~~

::

  Usage: docker restart [OPTIONS] NAME

  Restart a running container


rm
~~

::

  Usage: docker rm [OPTIONS] CONTAINER

  Remove a container


rmi
~~~

::

  Usage: docker rmi [OPTIONS] IMAGE

  Remove an image

    -a=false: Use IMAGE as a path and remove ALL images in this path
    -r=false: Use IMAGE as a regular expression instead of an exact name


run
~~~

::

  Usage: docker run [OPTIONS] IMAGE COMMAND [ARG...]

  Run a command in a new container

    -a=false: Attach stdin and stdout
    -c="": Comment
    -i=false: Keep stdin open even if not attached
    -m=0: Memory limit (in bytes)
    -p=[]: Map a network port to the container
    -t=false: Allocate a pseudo-tty
    -u="": Username or UID


start
~~~~~

::

  Usage: docker start [OPTIONS] NAME

  Start a stopped container


stop
~~~~

::

  Usage: docker stop [OPTIONS] NAME

  Stop a running container


tag
~~~

::

    Usage: docker tag [OPTIONS] IMAGE REPOSITORY [TAG]

    Tag an image into a repository

      -f=false: Force


version
~~~~~~~

::

  Usage: docker version

  Show the docker version information


wait
~~~~

::

  Usage: docker wait [OPTIONS] NAME

  Block until a container stops, then print its exit code.

