page_title: Redirect Ports
page_description: usage about port redirection
page_keywords: Usage, basic port, docker, documentation, examples

# Redirect Ports

## Introduction

Interacting with a service is commonly done through a connection to a
port. When this service runs inside a container, one can connect to the
port after finding the IP address of the container as follows:

    # Find IP address of container with ID <container_id>
    docker inspect <container_id> | grep IPAddress | cut -d '"' -f 4

However, this IP address is local to the host system and the container
port is not reachable by the outside world. Furthermore, even if the
port is used locally, e.g. by another container, this method is tedious
as the IP address of the container changes every time it starts.

Docker addresses these two problems and give a simple and robust way to
access services running inside containers.

To allow non-local clients to reach the service running inside the
container, Docker provide ways to bind the container port to an
interface of the host system. To simplify communication between
containers, Docker provides the linking mechanism.

## Auto map all exposed ports on the host

To bind all the exposed container ports to the host automatically, use
`docker run -P <imageid>`. The mapped host ports
will be auto-selected from a pool of unused ports (49000..49900), and
you will need to use `docker ps`,
`docker inspect <container_id>` or
`docker port <container_id> <port>` to determine
what they are.

## Binding a port to a host interface

To bind a port of the container to a specific interface of the host
system, use the `-p` parameter of the
`docker run` command:

    # General syntax
    docker run -p [([<host_interface>:[host_port]])|(<host_port>):]<container_port>[/udp] <image> <cmd>

When no host interface is provided, the port is bound to all available
interfaces of the host machine (aka INADDR\_ANY, or 0.0.0.0).When no
host port is provided, one is dynamically allocated. The possible
combinations of options for TCP port are the following:

    # Bind TCP port 8080 of the container to TCP port 80 on 127.0.0.1 of the host machine.
    docker run -p 127.0.0.1:80:8080 <image> <cmd>

    # Bind TCP port 8080 of the container to a dynamically allocated TCP port on 127.0.0.1 of the host machine.
    docker run -p 127.0.0.1::8080 <image> <cmd>

    # Bind TCP port 8080 of the container to TCP port 80 on all available interfaces of the host machine.
    docker run -p 80:8080 <image> <cmd>

    # Bind TCP port 8080 of the container to a dynamically allocated TCP port on all available interfaces of the host machine.
    docker run -p 8080 <image> <cmd>

UDP ports can also be bound by adding a trailing `/udp`. All the
combinations described for TCP work. Here is only one example:

    # Bind UDP port 5353 of the container to UDP port 53 on 127.0.0.1 of the host machine.
    docker run -p 127.0.0.1:53:5353/udp <image> <cmd>

The command `docker port` lists the interface and
port on the host machine bound to a given container port. It is useful
when using dynamically allocated ports:

    # Bind to a dynamically allocated port
    docker run -p 127.0.0.1::8080 --name dyn-bound <image> <cmd>

    # Lookup the actual port
    docker port dyn-bound 8080
    127.0.0.1:49160

## Linking a container

Communication between two containers can also be established in a
docker-specific way called linking.

To briefly present the concept of linking, let us consider two
containers: `server`, containing the service, and
`client`, accessing the service. Once
`server` is running, `client` is
started and links to server. Linking sets environment variables in
`client` giving it some information about
`server`. In this sense, linking is a method of
service discovery.

Let us now get back to our topic of interest; communication between the
two containers. We mentioned that the tricky part about this
communication was that the IP address of `server`
was not fixed. Therefore, some of the environment variables are going to
be used to inform `client` about this IP address.
This process called exposure, is possible because `client`
is started after `server` has been
started.

Here is a full example. On `server`, the port of
interest is exposed. The exposure is done either through the
`--expose` parameter to the `docker run`
command, or the `EXPOSE` build command in
a Dockerfile:

    # Expose port 80
    docker run --expose 80 --name server <image> <cmd>

The `client` then links to the `server`:

    # Link
    docker run --name client --link server:linked-server <image> <cmd>

`client` locally refers to `server`
as `linked-server`. The following
environment variables, among others, are available on `client`:

    # The default protocol, ip, and port of the service running in the container
    LINKED-SERVER_PORT=tcp://172.17.0.8:80

    # A specific protocol, ip, and port of various services
    LINKED-SERVER_PORT_80_TCP=tcp://172.17.0.8:80
    LINKED-SERVER_PORT_80_TCP_PROTO=tcp
    LINKED-SERVER_PORT_80_TCP_ADDR=172.17.0.8
    LINKED-SERVER_PORT_80_TCP_PORT=80

This tells `client` that a service is running on
port 80 of `server` and that `server`
is accessible at the IP address 172.17.0.8

Note: Using the `-p` parameter also exposes the
port..
