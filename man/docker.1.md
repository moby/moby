% DOCKER(1) Docker User Manuals
% William Henry
% APRIL 2014
# NAME
docker \- Docker image and container command line interface

# SYNOPSIS
**docker** [OPTIONS] COMMAND [ARG...]

**docker** [--help|-v|--version]

# DESCRIPTION
**docker** is a client for interacting with the daemon (see **dockerd(8)**) through the CLI.

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

Use "docker help" or "docker --help" to get an overview of available commands.

# EXAMPLES
For specific client examples please see the man page for the specific Docker
command. For example:

    man docker-run

# HISTORY
April 2014, Originally compiled by William Henry (whenry at redhat dot com) based on docker.com source material and internal work.
