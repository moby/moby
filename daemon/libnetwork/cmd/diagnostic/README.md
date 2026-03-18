---
description: Learn to use the built-in network debugger to debug overlay networking problems
keywords: network, troubleshooting, debug
title: Debug overlay or swarm networking issues
---

**WARNING**
This tool can change the internal state of the libnetwork API, be really mindful
on its use and read carefully the following guide. Improper use of it will damage
or permanently destroy the network configuration.


Docker CE 17.12 and higher introduce a network debugging tool designed to help
debug issues with overlay networks and swarm services running on Linux hosts.
When enabled, a network diagnostic server listens on the specified port and
provides diagnostic information. The network debugging tool should only be
started to debug specific issues, and should not be left running all the time.

Information about networks is stored in the database, which can be examined using
the API. Currently the database contains information about the overlay network
as well as the service discovery data.

The Docker API exposes endpoints to query and control the network debugging
tool. CLI integration is provided as a preview, but the implementation is not
yet considered stable and commands and options may change without notice.

The tool is available into 2 forms:
1) client only: dockereng/network-diagnostic:onlyclient
2) docker in docker version: dockereng/network-diagnostic:17.12-dind
The latter allows to use the tool with a cluster running an engine older than 17.12

## Enable the diagnostic server

The tool currently only works on Docker hosts running on Linux. To enable it on a node
follow the step below.

1.  Set the `network-diagnostic-port` to a port which is free on the Docker
    host, in the `/etc/docker/daemon.json` configuration file.

    ```json
    “network-diagnostic-port”: <port>
    ```

2.  Get the process ID (PID) of the `dockerd` process. It is the second field in
    the output, and is typically a number from 2 to 6 digits long.

    ```bash
    $ ps aux |grep dockerd | grep -v grep
    ```

3.  Reload the Docker configuration without restarting Docker, by sending the
    `HUP` signal to the PID you found in the previous step.

    ```bash
    kill -HUP <pid-of-dockerd>
    ```

If systemd is used the command `systemctl reload docker` will be enough


A message like the following will appear in the Docker host logs:

```none
Starting the diagnostic server listening on <port> for commands
```

## Disable the diagnostic tool

Repeat these steps for each node participating in the swarm.

1.  Remove the `network-diagnostic-port` key from the `/etc/docker/daemon.json`
    configuration file.

2.  Get the process ID (PID) of the `dockerd` process. It is the second field in
    the output, and is typically a number from 2 to 6 digits long.

    ```bash
    $ ps aux |grep dockerd | grep -v grep
    ```

3.  Reload the Docker configuration without restarting Docker, by sending the
    `HUP` signal to the PID you found in the previous step.

    ```bash
    kill -HUP <pid-of-dockerd>
    ```

A message like the following will appear in the Docker host logs:

```none
Disabling the diagnostic server
```

## Access the diagnostic tool's API

The network diagnostic tool exposes its own RESTful API. To access the API,
send a HTTP request to the port where the tool is listening. The following
commands assume the tool is listening on port 2000.

Examples are not given for every endpoint.

### Get help

```bash
$ curl localhost:2000/help

OK
/updateentry
/getentry
/gettable
/leavenetwork
/createentry
/help
/clusterpeers
/ready
/joinnetwork
/deleteentry
/networkpeers
/
/join
```

### Join or leave the network database cluster

```bash
$ curl localhost:2000/join?members=ip1,ip2,...
```

```bash
$ curl localhost:2000/leave?members=ip1,ip2,...
```

`ip1`, `ip2`, ... are the swarm node ips (usually one is enough)

### Join or leave a network

```bash
$ curl localhost:2000/joinnetwork?nid=<network id>
```

```bash
$ curl localhost:2000/leavenetwork?nid=<network id>
```

`network id` can be retrieved on the manager with `docker network ls --no-trunc` and has
to be the full length identifier

### List cluster peers

```bash
$ curl localhost:2000/clusterpeers
```

### List nodes connected to a given network

```bash
$ curl localhost:2000/networkpeers?nid=<network id>
```
`network id` can be retrieved on the manager with `docker network ls --no-trunc` and has
to be the full length identifier

### Dump database tables

The tables are called `endpoint_table` and `overlay_peer_table`.
The `overlay_peer_table` contains all the overlay forwarding information
The `endpoint_table` contains all the service discovery information

```bash
$ curl localhost:2000/gettable?nid=<network id>&tname=<table name>
```

### Interact with a specific database table

The tables are called `endpoint_table` and `overlay_peer_table`.

```bash
$ curl localhost:2000/<method>?nid=<network id>&tname=<table name>&key=<key>[&value=<value>]
```

Note:
operations on tables have node ownership, this means that are going to remain persistent till
the node that inserted them is part of the cluster

## Access the diagnostic tool's CLI

The CLI is provided as a preview and is not yet stable. Commands or options may
change at any time.

The CLI executable is called `diagnosticClient` and is made available using a
standalone container.

`docker run --net host dockereng/network-diagnostic:onlyclient -v -net <full network id> -t sd`

The following flags are supported:

| Flag          | Description                                     |
|---------------|-------------------------------------------------|
| -t <string>   | Table one of `sd` or `overlay`.                 |
| -ip <string>  | The IP address to query. Defaults to 127.0.0.1. |
| -net <string> | The target network ID.                          |
| -port <int>   | The target port. (default port is 2000)         |
| -a            | Join/leave network                              |
| -v            | Enable verbose output.                          |

*NOTE*
By default the tool won't try to join the network. This is following the intent to not change
the state on which the node is when the diagnostic client is run. This means that it is safe
to run the diagnosticClient against a running daemon because it will just dump the current state.
When using instead the diagnosticClient in the containerized version the flag `-a` MUST be passed
to avoid retrieving empty results. On the other side using the `-a` flag against a loaded daemon
will have the undesirable side effect to leave the network and so cutting down the data path for
that daemon.

### Container version of the diagnostic tool

The CLI is provided as a container with a 17.12 engine that needs to run using privileged mode.
*NOTE*
Remember that table operations have ownership, so any `create entry` will be persistent till
the diagnostic container is part of the swarm.

1.  Make sure that the node where the diagnostic client will run is not part of the swarm, if so do `docker swarm leave -f`

2.  To run the container, use a command like the following:

    ```bash
    $ docker container run --name net-diagnostic -d --privileged --network host dockereng/network-diagnostic:17.12-dind
    ```

3.  Connect to the container using `docker exec -it <container-ID> sh`,
    and start the server using the following command:

    ```bash
    $ kill -HUP 1
    ```

4.  Join the diagnostic container to the swarm, then run the diagnostic CLI within the container.

    ```bash
    $ ./diagnosticClient <flags>...
    ```

4.  When finished debugging, leave the swarm and stop the container.

### Examples

The following commands dump the service discovery table and verify node
ownership.

*NOTE*
Remember to use the full network ID, you can easily find that with `docker network ls --no-trunc`

**Service discovery and load balancer:**

```bash
$ diagnostiClient -t sd -v -net n8a8ie6tb3wr2e260vxj8ncy4 -a
```

**Overlay network:**

```bash
$ diagnostiClient -port 2001 -t overlay -v -net n8a8ie6tb3wr2e260vxj8ncy4 -a
```
