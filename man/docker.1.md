% DOCKER(1) Docker User Manuals
% William Henry
% APRIL 2014
# NAME
docker \- Docker image and container command line interface

# SYNOPSIS
**docker** [OPTIONS] COMMAND [arg...]

**docker** daemon [--help|...]

**docker** [--help|-v|--version]

# DESCRIPTION
is a client for interacting with the daemon (see **dockerd(8)**) through the CLI.

The Docker CLI has over 30 commands. The commands are listed below and each has
its own man page which explain usage and arguments.

To see the man page for a command run **man docker <command>**.

# OPTIONS
**--help**
  Print usage statement

**--config**=""
  Specifies the location of the Docker client configuration files. The default is '~/.docker'.

**-D**, **--debug**=*true*|*false*
  Enable debug mode. Default is false.

**-H**, **--host**=[*unix:///var/run/docker.sock*]: tcp://[host]:[port][path] to bind or
unix://[/path/to/socket] to use.
  The socket(s) to bind to in daemon mode specified using one or more
  tcp://host:port/path, unix:///path/to/socket, fd://* or fd://socketfd.
  If the tcp port is not specified, then it will default to either `2375` when
  `--tls` is off, or `2376` when `--tls` is on, or `--tlsverify` is specified.

**-l**, **--log-level**="*debug*|*info*|*warn*|*error*|*fatal*"
  Set the logging level. Default is `info`.

**--tls**=*true*|*false*
  Use TLS; implied by --tlsverify. Default is false.

**--tlscacert**=*~/.docker/ca.pem*
  Trust certs signed only by this CA.

**--tlscert**=*~/.docker/cert.pem*
  Path to TLS certificate file.

**--tlskey**=*~/.docker/key.pem*
  Path to TLS key file.

**--tlsverify**=*true*|*false*
  Use TLS and verify the remote (daemon: verify client, client: verify daemon).
  Default is false.

**-v**, **--version**=*true*|*false*
  Print version information and quit. Default is false.

# COMMANDS
**attach**
  Attach to a running container
  See **docker-attach(1)** for full documentation on the **attach** command.

**build**
  Build an image from a Dockerfile
  See **docker-build(1)** for full documentation on the **build** command.

**commit**
  Create a new image from a container's changes
  See **docker-commit(1)** for full documentation on the **commit** command.

**cp**
  Copy files/folders between a container and the local filesystem
  See **docker-cp(1)** for full documentation on the **cp** command.

**create**
  Create a new container
  See **docker-create(1)** for full documentation on the **create** command.

**diff**
  Inspect changes on a container's filesystem
  See **docker-diff(1)** for full documentation on the **diff** command.

**events**
  Get real time events from the server
  See **docker-events(1)** for full documentation on the **events** command.

**exec**
  Run a command in a running container
  See **docker-exec(1)** for full documentation on the **exec** command.

**export**
  Stream the contents of a container as a tar archive
  See **docker-export(1)** for full documentation on the **export** command.

**history**
  Show the history of an image
  See **docker-history(1)** for full documentation on the **history** command.

**images**
  List images
  See **docker-images(1)** for full documentation on the **images** command.

**import**
  Create a new filesystem image from the contents of a tarball
  See **docker-import(1)** for full documentation on the **import** command.

**info**
  Display system-wide information
  See **docker-info(1)** for full documentation on the **info** command.

**inspect**
  Return low-level information on a container or image
  See **docker-inspect(1)** for full documentation on the **inspect** command.

**kill**
  Kill a running container (which includes the wrapper process and everything
inside it)
  See **docker-kill(1)** for full documentation on the **kill** command.

**load**
  Load an image from a tar archive
  See **docker-load(1)** for full documentation on the **load** command.

**login**
  Log in to a Docker Registry
  See **docker-login(1)** for full documentation on the **login** command.

**logout**
  Log the user out of a Docker Registry
  See **docker-logout(1)** for full documentation on the **logout** command.

**logs**
  Fetch the logs of a container
  See **docker-logs(1)** for full documentation on the **logs** command.

**pause**
  Pause all processes within a container
  See **docker-pause(1)** for full documentation on the **pause** command.

**port**
  Lookup the public-facing port which is NAT-ed to PRIVATE_PORT
  See **docker-port(1)** for full documentation on the **port** command.

**ps**
  List containers
  See **docker-ps(1)** for full documentation on the **ps** command.

**pull**
  Pull an image or a repository from a Docker Registry
  See **docker-pull(1)** for full documentation on the **pull** command.

**push**
  Push an image or a repository to a Docker Registry
  See **docker-push(1)** for full documentation on the **push** command.

**rename**
  Rename a container.
  See **docker-rename(1)** for full documentation on the **rename** command.

**restart**
  Restart a container
  See **docker-restart(1)** for full documentation on the **restart** command.

**rm**
  Remove one or more containers
  See **docker-rm(1)** for full documentation on the **rm** command.

**rmi**
  Remove one or more images
  See **docker-rmi(1)** for full documentation on the **rmi** command.

**run**
  Run a command in a new container
  See **docker-run(1)** for full documentation on the **run** command.

**save**
  Save an image to a tar archive
  See **docker-save(1)** for full documentation on the **save** command.

**search**
  Search for an image in the Docker index
  See **docker-search(1)** for full documentation on the **search** command.

**start**
  Start a container
  See **docker-start(1)** for full documentation on the **start** command.

**stats**
  Display a live stream of one or more containers' resource usage statistics
  See **docker-stats(1)** for full documentation on the **stats** command.

**stop**
  Stop a container
  See **docker-stop(1)** for full documentation on the **stop** command.

**tag**
  Tag an image into a repository
  See **docker-tag(1)** for full documentation on the **tag** command.

**top**
  Lookup the running processes of a container
  See **docker-top(1)** for full documentation on the **top** command.

**unpause**
  Unpause all processes within a container
  See **docker-unpause(1)** for full documentation on the **unpause** command.

**version**
  Show the Docker version information
  See **docker-version(1)** for full documentation on the **version** command.

**wait**
  Block until a container stops, then print its exit code
  See **docker-wait(1)** for full documentation on the **wait** command.


# RUNTIME EXECUTION OPTIONS

Use the **--exec-opt** flags to specify options to the execution driver.
The following options are available:

#### native.cgroupdriver
Specifies the management of the container's `cgroups`. You can specify `cgroupfs`
or `systemd`. If you specify `systemd` and it is not available, the system errors
out.

#### Client
For specific client examples please see the man page for the specific Docker
command. For example:

    man docker-run

# HISTORY
April 2014, Originally compiled by William Henry (whenry at redhat dot com) based on docker.com source material and internal work.
