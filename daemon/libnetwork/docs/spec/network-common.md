Common networking surface area
==============================

Scope: the behavior that every network driver must have, and the Engine API requests for networks
and endpoints that do not depend on the network's driver. This spec describes them as observable
from outside the daemon.

This spec is a stub. It holds expectations that every network driver shares. They come from the work
on the other specs and from reviews of the specs' coverage. It is not yet a complete account.
[README.md](README.md) gives its conventions for tiers, vantage points and dependencies.
[swarm-networking.md](swarm-networking.md) and [bridge-networking.md](bridge-networking.md) cover
what is specific to their drivers. [container-dns.md](container-dns.md) covers name resolution.

In this spec, a predefined network is a network that `GET /networks` with the filter
`type=builtin` lists. Requests about Swarm-scoped networks are in
[swarm-networking.md §2](swarm-networking.md#2-network-lifecycle--api-surface).

## 1. Network API

| Applies to | Expectation | Tier | Observed from | Depends on |
| --- | --- | --- | --- | --- |
| An existing network that is not Swarm-scoped and not predefined | `POST /networks/create` with the network's name responds 409. `GET /networks` then lists only one network with that name. | B | `api` | `none` |
| An existing network that is not Swarm-scoped and not predefined | `docker network create` with the network's name exits non-zero and prints the daemon's error. `docker network ls` then lists only one network with that name. | B | `api` | `cli` |
| A Linux node that is not in a Swarm | `POST /networks/create` with the name of a predefined network, `default` or `container` responds 403. `POST /networks/create` with a name that starts with `container:` also responds 403. `GET /networks` then does not list a new network with that name. | B | `api:solo` | `none` |
| A Linux manager | `POST /networks/create` with the name `bridge` or `host` responds 409. It responds 403 when the name is `none`, `default` or `container`, or starts with `container:`. This holds for the default driver and for `overlay`. | C | `api:mgr` | `none` |
| Linux nodes | `docker network create` with the name of a predefined network exits non-zero and prints the daemon's error. | B | `api` | `cli` |
| Linux nodes | `DELETE /networks/{id}` for a predefined network responds 403. `GET /networks` then still lists the network. | B | `api` | `none` |
| Linux nodes | `docker network rm` for a predefined network exits non-zero and prints the daemon's error. `docker network ls` then still lists the network. | B | `api` | `cli` |
| A network that is not Swarm-scoped, with a running container attached to it | `DELETE /networks/{id}` for the network responds 403. `GET /networks` then still lists the network. | B | `api` | `none` |
| A network that is not Swarm-scoped, with a running container attached to it | `docker network rm` for the network exits non-zero and prints the daemon's error. `docker network ls` then still lists the network. | B | `api` | `cli` |
| | `POST /networks/{id}/connect` for a running container that is already attached to network `{id}` responds with a 4xx or 5xx status. The container's addresses on that network do not change. | B | `api` | `none` |
| A network that is not Swarm-scoped | `POST /networks/{id}/connect` for a running container that is already attached to the network responds 403. See [§7](#7-known-divergences--non-goals). | C | `api` | `none` |
| | `docker network connect` for a running container that is already attached to the network exits non-zero and prints the daemon's error. | B | `api` | `cli` |
| A network that is not Swarm-scoped | `POST /networks/{id}/connect` for a created or stopped container that is already attached to the network responds 200. The container's endpoint settings for the network do not change. | C | `api` | `none` |
| A created or stopped container | `POST /networks/{id}/connect` for the container, with an `{id}` that is not the name or an ID prefix of any network, responds 200. `POST /containers/{id}/start` for the container then responds 404 while no network has that name. See [§7](#7-known-divergences--non-goals). | C | `api` | `none` |
| A created or stopped container | `POST /networks/{id}/connect` for the container, with an `{id}` that is not the name or an ID prefix of any network, responds 200. A network with that name is then created. `POST /containers/{id}/start` for the container then responds 204, and the container has an address on that network. See [§7](#7-known-divergences--non-goals). | C | `api` | `none` |
| A network that is not Swarm-scoped | `POST /containers/create` with a static address in `IPAMConfig` that is outside every subnet of that network responds 400, and does not create a container. `POST /networks/{id}/connect` with that address responds 400, and does not attach the container. | B | `api` | `none` |
| Linux nodes | `POST /containers/create` and `POST /networks/{id}/connect` with a static address in `IPAMConfig` for the network `bridge` respond 400. The create does not create a container. The connect does not attach the container. | B | `api` | `none` |
| | `docker run` and `docker network connect` with `--ip` outside every subnet of a network that is not Swarm-scoped exit non-zero and print the daemon's error. On Linux nodes, they do the same with `--ip` for the network `bridge`. | B | `api` | `cli` |
| | At API version 1.43 or earlier, `POST /containers/create` with more than one network in `NetworkingConfig.EndpointsConfig` responds 400, and does not create a container. | B | `api` | `none` |
| | `POST /networks/{id}/disconnect` for a container that is not attached to the network responds with a 4xx or 5xx status. | B | `api` | `none` |
| | `POST /networks/{id}/disconnect` for a container that is not attached to the network responds 500, with a message that contains `is not connected to`. See [§7](#7-known-divergences--non-goals). | C | `api` | `none` |
| | `docker network disconnect` for a container that is not attached to the network exits non-zero and prints the daemon's error. | B | `api` | `cli` |
| A running container with an endpoint on a network, from its start or from a later connect | `POST /networks/{id}/disconnect` for the container responds 200. `GET /containers/{id}/json` then does not list the network in `NetworkSettings.Networks`, and the container still runs. | B | `api` | `none` |
| A running container with an endpoint on a network, from its start or from a later connect | After `POST /networks/{id}/disconnect` for the container responds 200, the container does not have its interface on that network. | B | `ctr` | `os` |
| A running container with an endpoint on a network, from its start or from a later connect | `docker network disconnect` for the container exits zero. `docker inspect` for the container then does not list the network in `NetworkSettings.Networks`. | B | `api` | `cli` |
| A running container | A `POST /networks/{id}/connect` for the container fails because the network's driver does not accept the endpoint. An example is an ipvlan endpoint that sets `MacAddress`, at API version 1.54 or later. `GET /containers/{id}/json` then lists the network in `NetworkSettings.Networks`, and `GET /networks/{id}` does not list the container. See [§7](#7-known-divergences--non-goals). | C | `api` | `none` |
| | `GET /networks/{id}` responds 200 with the network when `{id}` is the network's full ID or its name. It does the same when `{id}` is a prefix of the network's ID that the ID of no other network starts with. | B | `api` | `none` |
| | `docker network inspect` prints the network when it is given the network's full ID or its name. It does the same when it is given a prefix of the network's ID that the ID of no other network starts with. | B | `api` | `cli` |
| | `GET /networks/{id}` responds 404 when `{id}` is not the full ID, the name or a prefix of the ID of any network. | B | `api` | `none` |
| Two or more networks whose IDs start with a prefix that is not a network name | `GET /networks/{id}` with that prefix as `{id}` responds with a 4xx or 5xx status. | B | `api` | `none` |
| Two or more networks whose IDs start with a prefix that is not a network name | `GET /networks/{id}` with that prefix as `{id}` responds 400, with a message that contains `is ambiguous`. | C | `api` | `none` |
| A network whose name is a prefix of the ID of another network | `GET /networks/{id}` with that name as `{id}` returns the network with that name. | C | `api` | `none` |
| | `GET /networks` with the filter `type=custom` lists the networks that the filter `type=builtin` does not list. | B | `api` | `none` |
| Linux nodes | `GET /networks` with the filter `type=builtin` lists `bridge`, `host` and `none`. | C | `api` | `none` |
| | `GET /networks` with a filter term that it does not support responds 400. `GET /networks` with a value that a filter term does not accept also responds 400. | B | `api` | `none` |
| | `docker network ls --filter` exits non-zero and prints the daemon's error for a filter term that the daemon does not support. It does the same for a value that a filter term does not accept. | B | `api` | `cli` |
| | The `type` filter term of `GET /networks` accepts the values `builtin` and `custom`. | C | `api` | `none` |
| | `GET /networks` with both `type=builtin` and `type=custom` lists a subset of the networks. The subset can differ from one request to the next. See [§7](#7-known-divergences--non-goals). | C | `api` | `none` |
| A network that is not Swarm-scoped and not predefined, with no running container attached to it | `GET /networks` with the filter `dangling=true` lists the network. | B | `api` | `none` |
| A network that is not Swarm-scoped, and that is predefined or has a running container attached to it | `GET /networks` with the filter `dangling=false` lists the network. | B | `api` | `none` |
| | `POST /networks/prune` responds 200. It deletes each network that is not Swarm-scoped, not predefined and not config-only, and that no running container is attached to. `NetworksDeleted` in the response lists the names of these networks. | B | `api` | `none` |
| | `docker network prune --force` exits zero, and prints the names of the networks that it deletes. `docker network ls` then does not list those networks. | B | `api` | `cli` |
| A node that is not in a Swarm, and a `docker_gwbridge` network that no running container is attached to | `POST /networks/prune` deletes `docker_gwbridge`. See [§7](#7-known-divergences--non-goals). | C | `api:solo` | `none` |
| | `POST /networks/create` with `ConfigOnly` true responds 201. `GET /networks/{id}` then reports `ConfigOnly` true. | B | `api` | `none` |
| A network created with `ConfigFrom` naming a config-only network | `GET /networks/{id}` for the network reports the config-only network's `IPAM` configuration. | B | `api` | `none` |
| A network created with `ConfigFrom` naming a config-only network | The network's containers get addresses in the subnets of the config-only network's `IPAM` configuration. | B | `ctr` | `none` |
| | `POST /networks/create` with `ConfigFrom` naming a network that does not exist responds with a 4xx or 5xx status. `GET /networks` then does not list a network with the request's `Name`. | B | `api` | `none` |
| | `POST /networks/create` with `ConfigFrom` naming a network that does not exist responds 404, with a message that contains `configuration network "<name>" does not exist`. See [§7](#7-known-divergences--non-goals). | C | `api` | `none` |
| | `docker network create --config-from` naming a network that does not exist exits non-zero and prints the daemon's error. | B | `api` | `cli` |
| A config-only network | After `POST /networks/create` with `ConfigFrom` naming that network responds 201, `DELETE /networks/{id}` for the config-only network responds 403. `GET /networks` then still lists the config-only network. | B | `api` | `none` |
| A config-only network | After `docker network create --config-from` naming that network exits zero, `docker network rm` for the config-only network exits non-zero and prints the daemon's error. | B | `api` | `cli` |
| | `POST /containers/create` for a container on a config-only network responds 201. `POST /networks/{id}/connect` to a config-only network, for a created or stopped container, responds 200. See [§7](#7-known-divergences--non-goals). | C | `api` | `none` |
| | `POST /containers/{id}/start` for a created container on a config-only network responds 403. The container stays in the created state. See [§7](#7-known-divergences--non-goals). | C | `api` | `none` |
| | `docker run -d --network` naming a config-only network exits non-zero and prints the daemon's error. | B | `api` | `cli` |

## 2. Endpoint interfaces

The rows in this section hold for an endpoint of any network driver that adds an interface to the
container ([§5](#5-driver-properties)). An endpoint requests an interface name when its
`com.docker.network.endpoint.ifname` is set to that name.

| Applies to | Expectation | Tier | Observed from | Depends on |
| --- | --- | --- | --- | --- |
| An endpoint that requests an interface name | The interface has that name. See [§7](#7-known-divergences--non-goals). | B | `task`, `ctr` | `os` |
| Linux nodes, and an endpoint that does not request an interface name | The interface name is `eth<N>`. | C | `task`, `ctr` | `os` |
| A created or stopped container with an endpoint whose `com.docker.network.endpoint.ifname` the OS refuses | `POST /containers/{id}/start` for the container responds with a 4xx or 5xx status. | B | `api` | `none` |
| A running container | `POST /networks/{id}/connect` for the container, with an endpoint whose `com.docker.network.endpoint.ifname` the OS refuses, responds with a 4xx or 5xx status. | B | `api` | `none` |
| A created or stopped container with an endpoint whose `com.docker.network.endpoint.ifname` the OS refuses | `POST /containers/{id}/start` for the container responds 500, with a message that contains `error renaming interface` and the requested name. See [§7](#7-known-divergences--non-goals). | C | `api` | `none` |
| A running container | `POST /networks/{id}/connect` for the container, with an endpoint whose `com.docker.network.endpoint.ifname` the OS refuses, responds 500. The message contains `error renaming interface` and the requested name. See [§7](#7-known-divergences--non-goals). | C | `api` | `none` |
| | `docker run -d` with a `com.docker.network.endpoint.ifname` that the OS refuses exits non-zero and prints the daemon's error. | B | `api` | `cli` |
| A running container | `docker network connect` for the container, with a `com.docker.network.endpoint.ifname` that the OS refuses, exits non-zero and prints the daemon's error. | B | `api` | `cli` |
| | `POST /containers/create` for a container with two endpoints that request the same interface name responds 201. So does a request for a container with an endpoint that requests `lo`. | C | `api` | `none` |
| A created or stopped container with two endpoints that request the same interface name | `POST /containers/{id}/start` for the container responds with a 4xx or 5xx status. | C | `api` | `none` |
| A created or stopped container with an endpoint that requests `lo` | `POST /containers/{id}/start` for the container responds with a 4xx or 5xx status. | C | `api` | `none` |
| A running container | `POST /networks/{id}/connect` for the container, with an endpoint that requests `lo`, responds with a 4xx or 5xx status. It does the same with an endpoint that requests the name of one of the container's interfaces. | B | `api` | `none` |
| A created or stopped container with two endpoints that request the same interface name, or with one that requests `lo` | `POST /containers/{id}/start` for the container responds 500, with a message that names the interface. See [§7](#7-known-divergences--non-goals). | C | `api` | `none` |
| A running container | `POST /networks/{id}/connect` for the container, with an endpoint that requests `lo` or the name of one of its interfaces, responds 500. The message names the interface. See [§7](#7-known-divergences--non-goals). | C | `api` | `none` |
| | `docker run -d` exits non-zero and prints the daemon's error when two of the container's endpoints request the same interface name. It does the same when an endpoint requests `lo`. | B | `api` | `cli` |
| A running container | `docker network connect` for the container exits non-zero and prints the daemon's error when the endpoint requests `lo` or the name of one of its interfaces. | B | `api` | `cli` |
| A running standalone container whose endpoint on a network has an `IPv4Address` or `IPv6Address` in `IPAMConfig` | Its interface on that network has that address. | B | `ctr` | `os` |
| A running standalone container whose endpoint on a network has an `IPv4Address` or `IPv6Address` in `IPAMConfig` | `GET /containers/{id}/json` reports the address as `IPAddress` or `GlobalIPv6Address` in `NetworkSettings.Networks.<network>`. | B | `api` | `none` |
| A running container created at API version 1.44 or later with an endpoint `MacAddress` on a network whose driver accepts an endpoint MAC address ([§5](#5-driver-properties)) | Its interface on that network has that MAC address. | B | `ctr` | `os` |
| A running container, and a network whose driver accepts an endpoint MAC address ([§5](#5-driver-properties)) | After `POST /networks/{id}/connect` with `MacAddress` set, at API version 1.54 or later, responds 200, the container's interface on the network has that MAC address. | B | `ctr` | `os` |
| A network whose driver does not accept an endpoint MAC address ([§5](#5-driver-properties)) | `POST /networks/{id}/connect` for a running container, with `MacAddress` set, at API version 1.54 or later, responds with a 4xx or 5xx status. The container does not get an interface on the network. | B | `api` | `none` |
| A network whose driver does not accept an endpoint MAC address ([§5](#5-driver-properties)) | `POST /containers/create` for a container whose endpoint on that network sets `MacAddress` responds 201. `POST /containers/{id}/start` for the container then responds with a 4xx or 5xx status, and the container stays in the created state. | C | `api` | `none` |
| | `docker run -d` with `--mac-address` and with `--network` naming a network whose driver does not accept an endpoint MAC address ([§5](#5-driver-properties)) exits non-zero and prints the daemon's error. | B | `api` | `cli` |

## 3. Default gateway

| Applies to | Expectation | Tier | Observed from | Depends on |
| --- | --- | --- | --- | --- |
| | A container attached only to `internal` networks does not have a default route. See [§7](#7-known-divergences--non-goals). | B | `task`, `ctr` | `os` |
| Linux nodes, and a container with endpoints on two or more networks that are not `internal`, one with a higher `GwPriority` than the others | For each IP version that the endpoint gives the container a gateway or default route for, the container's default route goes through that endpoint. | B | `ctr` | `os` |
| | At API version 1.47 or earlier, `POST /containers/create` ignores `GwPriority` in `EndpointsConfig`. | B | `api`, `ctr` | `none` |
| | `POST /networks/{id}/connect` applies `GwPriority` at every API version. | C | `api`, `ctr` | `none` |

## 4. `/etc/hosts`

This section holds on Linux nodes.

| Applies to | Expectation | Tier | Observed from | Depends on |
| --- | --- | --- | --- | --- |
| A container that does not use the host's network | For each of the container's addresses, `/etc/hosts` has a line that maps the address to the container's hostname. The exceptions are addresses on the ingress network and on `docker_gwbridge`. IPv6 addresses are also exceptions while IPv6 is disabled in the container. | B | `task`, `ctr` | `none` |
| A container whose `Hostname` contains a dot, or that has a `Domainname` | Each line for the container's addresses gives two names. The first is the full name: the hostname, then a dot and the domain name if the container has one. The second is the part of the hostname before its first dot. | B | `ctr` | `none` |
| A running container | After `POST /networks/{id}/connect` for the container responds 200, `/etc/hosts` has the lines for the container's addresses on that network. | B | `ctr` | `none` |
| A running container with an endpoint on a network, from its start or from a later connect | After `POST /networks/{id}/disconnect` for the container responds 200, `/etc/hosts` does not have the lines for the container's addresses on that network. | B | `ctr` | `none` |
| A daemon with `host-gateway-ips` set | An entry `<name>:host-gateway` in `HostConfig.ExtraHosts` adds a line to `/etc/hosts` for each address in `host-gateway-ips`. Each line maps the address to `<name>`. | B | `ctr` | `none` |
| A daemon without `host-gateway-ips` | An entry `<name>:host-gateway` in `HostConfig.ExtraHosts` adds a line to `/etc/hosts` for each gateway address of the default bridge. This holds whatever networks the container is attached to. | C | `ctr` | `none` |
| A daemon without `host-gateway-ips` and with `bridge` set to `none`, and a created container with an entry `<name>:host-gateway` in `HostConfig.ExtraHosts` | `POST /containers/{id}/start` for the container responds 500, with a message that contains `unable to derive the IP value for host-gateway`. The container stays in the created state. See [§7](#7-known-divergences--non-goals). | C | `api` | `none` |
| A daemon without `host-gateway-ips` and with `bridge` set to `none` | `docker run` with `--add-host <name>:host-gateway` exits non-zero and prints the daemon's error. | C | `api` | `cli` |

## 5. Driver properties

Rows in this spec name a property of a network driver, not the driver itself, where the set of
drivers with the property can change. This table gives the properties of the built-in drivers. It
is tier C: it pins which drivers have each property today. A plugin driver has the properties that
it implements.

| Driver | Adds an interface to the container | Accepts an endpoint MAC address |
| --- | --- | --- |
| `bridge` | Yes | Yes |
| `overlay` | Yes | Yes |
| `macvlan` | Yes | Yes |
| `ipvlan` | Yes | No |
| `host` | No | Not applicable |
| `null` (the `none` network) | No | Not applicable |
| Windows: `nat`, `transparent`, `l2bridge`, `l2tunnel`, `ics` | Yes | Yes |

## 6. Daemon restart

| Applies to | Expectation | Tier | Observed from | Depends on |
| --- | --- | --- | --- | --- |
| Linux nodes | After `SIGKILL` and restart, every network namespace that the daemon created before the kill is removed, other than an overlay network's namespace. See [swarm-networking.md §13](swarm-networking.md#13-known-divergences--non-goals). | B | `node` | `os` |

## 7. Known divergences & non-goals

| Topic | Current behavior | Depends on |
| --- | --- | --- |
| HTTP 500 for refused requests | Each request below is refused on purpose, but it responds 500. 500 is the status for a fault in the daemon. The requests are: (1) `POST /networks/{id}/disconnect` for a container that is not attached to the network. (2) `POST /containers/{id}/start` and `POST /networks/{id}/connect` for an endpoint with a `com.docker.network.endpoint.ifname` that the OS refuses, or that another endpoint of the container already uses. (3) `POST /containers/{id}/start` for a created or stopped container with a `host-gateway` entry in `HostConfig.ExtraHosts`, when the daemon does not have an address for `host-gateway` ([§1](#1-network-api), [§2](#2-endpoint-interfaces), [§4](#4-etchosts)). | `none` |
| Networks that a created or stopped container cannot use | Accepted. For a created or stopped container, `POST /networks/{id}/connect` to a network that does not exist, or to a config-only network, responds 200. `POST /containers/create` for a container on a config-only network also responds 201. Only `POST /containers/{id}/start` refuses the network. It responds 404 with `network <name> not found`, or 403 with `cannot create endpoint on configuration-only network`. If a network with the missing name is created before the start, the start attaches the container to it ([§1](#1-network-api)). | `none` |
| Connecting a running container to a network it is already attached to | Refused with two different statuses. The request responds 403 on a network that is not Swarm-scoped, and 409 on an attachable Swarm-scoped network ([§1](#1-network-api), [swarm-networking.md §2.2](swarm-networking.md#22-swarm-scoped-networks)). | `none` |
| HTTP 404 for a missing `ConfigFrom` network | `POST /networks/create` with `ConfigFrom` naming a network that does not exist responds 404. 404 is the status for a request whose own resource does not exist. Here the request is valid, and its body names the missing network ([§1](#1-network-api)). | `none` |
| Several values for the `type` filter | Not combined. With both `type=builtin` and `type=custom` given, `GET /networks` tests each network against one of the two values, chosen at random. The response lists a different subset of the networks each time. As with other filter terms, a network is meant to match when it matches either value ([§1](#1-network-api)). | `none` |
| Default route on `internal` overlays on Windows | A Windows container attached only to `internal` overlays has a default route ([§3](#3-default-gateway)). | `none` |
| `ifname` option on Windows | Ignored. A Windows container's interface name does not follow `com.docker.network.endpoint.ifname` ([§2](#2-endpoint-interfaces)). | `none` |
| Interface left behind after a failed connect | If `POST /networks/{id}/connect` for a running container fails at the rename, the interface stays in the container under its generated `veth` name. The interface is down and does not have addresses. Its host-side peer stays attached. The interface and the peer stay until the container's network namespace is destroyed. A failed connect is meant to leave nothing in place ([§2](#2-endpoint-interfaces)). | `none` |
| Interface names the kernel rewrites | Accepted without an error. For a name that contains `%d`, the kernel gives the interface the first free name that matches, such as `eth0`. A later disconnect responds 200, but the interface stays in the container. The interface stays up and keeps its address ([§2](#2-endpoint-interfaces)). | `none` |
| Interface-name collisions depend on attach order | Take a container with two endpoints. One endpoint asks for `eth<N>`. The name of the other endpoint is generated. The container starts or fails, and the result depends on the order in which its endpoints are attached. This order is random. The endpoint that asks for the name is meant to get it, whatever the order ([§2](#2-endpoint-interfaces)). | `none` |
| Network listed after a failed connect | If the network's driver does not accept the endpoint in `POST /networks/{id}/connect` for a running container, the request fails. `GET /containers/{id}/json` then lists the network in `NetworkSettings.Networks`, but the container does not have an endpoint on it. A failed connect is meant to leave nothing in place ([§1](#1-network-api)). | `none` |
| `docker_gwbridge` outside a Swarm | Pruned. On a node that is not in a Swarm, `docker_gwbridge` is not a predefined network. `POST /networks/prune` deletes it while no running container is attached to it. This includes a `docker_gwbridge` that an operator creates before the node joins a Swarm, to configure the gateway network ([§1](#1-network-api), [swarm-networking.md §8.1](swarm-networking.md#81-gateway-network)). | `none` |
