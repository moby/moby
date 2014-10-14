% DOCKER(1) Docker User Manuals
% Dan Walsh
% JUNE  2014
# NAME
docker-daemon \- Run the Docker daemon

# SYNOPSIS
**docker daemon** [OPTIONS]

# DESCRIPTION
Run the docker daemon.  Docker daemon is usually run within the init system in 
either an init script or in a systemd unit file.

# OPTIONS

**-H**, **--host**=[unix:///var/run/docker.sock]: tcp://[host:port] to bind or
unix://[/path/to/socket] to use.
   The socket(s) to bind to in daemon mode specified using one or more
   tcp://host:port, unix:///path/to/socket, fd://* or fd://socketfd.

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

**--selinux-enabled**=*true*|*false*
  Enable selinux support. Default is false.

# EXAMPLES

For specific examples please see the man page for the specific Docker command.
For example:

    man docker run

# HISTORY
June 2014, Originally compiled by Dan Walsh (dwalsh at redhat dot com) based
 on docker.io source material and internal work.
