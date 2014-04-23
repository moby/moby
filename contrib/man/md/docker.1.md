% DOCKER(1) Docker User Manuals
% William Henry
% APRIL 2014
# NAME
docker \- Docker image and container command line interface

# SYNOPSIS
**docker** [OPTIONS] COMMAND [arg...]

# DESCRIPTION
**docker** has two distinct functions. It is used for starting the Docker
daemon and to run the CLI (i.e., to command the daemon to manage images,
containers etc.) So **docker** is both a server, as a deamon, and a client
to the daemon, through the CLI.

To run the Docker deamon you do not specify any of the commands listed below but
must specify the **-d** option.  The other options listed below are for the
daemon only.

The Docker CLI has over 30 commands. The commands are listed below and each has
its own man page which explain usage and arguements.

To see the man page for a command run **man docker <command>**.

# OPTIONS
**-D**=*ture*|*false*
   Enable debug mode. Default is false.

**-H**, **--host**=[unix:///var/run/docker.sock]: tcp://[host[:port]] to bind or
unix://[/path/to/socket] to use.
   Enable both the socket support and TCP on localhost. When host=[0.0.0.0],
port=[4243] or path =[/var/run/docker.sock] is omitted, default values are used.

**--api-enable-cors**=*true*|*false*
  Enable CORS headers in the remote API. Default is false.

**-b**=""
  Attach containers to a pre\-existing network bridge; use 'none' to disable container networking

**--bip**=""
  Use the provided CIDR notation address for the dynamically created bridge (docker0); Mutually exclusive of \-b

**-d**=*true*|*false*
  Enable daemon mode. Default is false.

**--dns**=""
  Force Docker to use specific DNS servers

**-g**=""
  Path to use as the root of the Docker runtime. Default is `/var/lib/docker`.

**--icc**=*true*|*false*
  Enable inter\-container communication. Default is true.

**--ip**=""
  Default IP address to use when binding container ports. Default is `0.0.0.0`.

**--iptables**=*true*|*false*
  Disable Docker's addition of iptables rules. Default is true.

**--mtu**=VALUE
  Set the containers network mtu. Default is `1500`.

**-p**=""
  Path to use for daemon PID file. Default is `/var/run/docker.pid`

**-r**=*true*|*false*
  Restart previously running containers. Default is true.

**-s**=""
  Force the Docker runtime to use a specific storage driver.

**-v**=*true*|*false*
  Print version information and quit. Default is false.

# COMMANDS
**docker-attach(1)**
  Attach to a running container

**docker-build(1)**
  Build a container from a Dockerfile

**docker-commit(1)**
  Create a new image from a container's changes

**docker-cp(1)**
  Copy files/folders from the containers filesystem to the host at path

**docker-diff(1)**
  Inspect changes on a container's filesystem


**docker-events(1)**
  Get real time events from the server

**docker-export(1)**
  Stream the contents of a container as a tar archive

**docker-history(1)**
  Show the history of an image

**docker-images(1)**
  List images

**docker-import(1)**
  Create a new filesystem image from the contents of a tarball

**docker-info(1)**
  Display system-wide information

**docker-inspect(1)**
  Return low-level information on a container

**docker-kill(1)**
  Kill a running container (which includes the wrapper process and everything
inside it)

**docker-load(1)**
  Load an image from a tar archive

**docker-login(1)**
  Register or Login to a Docker registry server

**docker-logs(1)**
  Fetch the logs of a container

**docker-port(1)**
  Lookup the public-facing port which is NAT-ed to PRIVATE_PORT

**docker-ps(1)**
  List containers

**docker-pull(1)**
  Pull an image or a repository from a Docker registry server

**docker-push(1)**
  Push an image or a repository to a Docker registry server

**docker-restart(1)**
  Restart a running container

**docker-rm(1)**
  Remove one or more containers

**docker-rmi(1)**
  Remove one or more images

**docker-run(1)**
  Run a command in a new container

**docker-save(1)**
  Save an image to a tar archive

**docker-search(1)**
  Search for an image in the Docker index

**docker-start(1)**
  Start a stopped container

**docker-stop(1)**
  Stop a running container

**docker-tag(1)**
  Tag an image into a repository

**docker-top(1)**
  Lookup the running processes of a container

**version**
  Show the Docker version information

**docker-wait(1)**
  Block until a container stops, then print its exit code

# EXAMPLES

For specific examples please see the man page for the specific Docker command.
For example:

    man docker run

# HISTORY
April 2014, Originally compiled by William Henry (whenry at redhat dot com) based
 on docker.io source material and internal work.
