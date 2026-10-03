Swarm networking surface area
=============================

Scope: the Docker overlay network driver and the Swarm service mesh (VIP load balancing,
ingress routing mesh, service discovery), as observable from outside the daemon.

This is a test-surface specification. For how the pieces work, see [design.md](../design.md),
[network.md](../network.md) and [networkdb.md](../networkdb.md).

The conventions it follows for tiers, vantage points and dependencies are shared with the other
specs in this directory, in [README.md](README.md). Unless a row says otherwise, a `ctr` in this
document is a standalone container attached to an `attachable` overlay.

## Interop premise

Wire and protocol compatibility is a hard requirement. Users upgrade Swarm clusters node by node.
During a rollout the cluster is heterogeneous, and nodes with different engine versions and
different firewall backends must share an overlay network and an ingress network. Everything in
[§10 Wire format](#10-wire-format--interop) is contract, and so are the mixed-cluster rows in
[§11](#11-failure--lifecycle).

## 1. Cluster & node configuration

These rows cover cluster configuration.

| Expectation | Tier | Observed from | Depends on |
| --- | --- | --- | --- |
| `AdvertiseAddr` in `POST /swarm/init` or `POST /swarm/join` sets the address peers use to reach this node's control plane. | B | `api:mgr` | `none` |
| `ListenAddr` in `POST /swarm/init` or `POST /swarm/join` sets the local bind address of the control plane. | B | `node` | `none` |
| `DataPathAddr` in `POST /swarm/init` or `POST /swarm/join` sets the address peers send the node's VXLAN traffic to. | B | `underlay` (outer destination address) | `os` |
| Without `DataPathAddr`, peers send the node's VXLAN traffic to its advertise address. | B | `underlay` (outer destination address) | `os` |
| On Linux nodes, a node's VXLAN traffic to a peer carries as its outer source address the address that the node's route to that peer selects, whether or not that is its data-path address; see [§13](#13-known-divergences--non-goals). | C | `underlay` (outer source address) | `os` |
| `DataPathPort` in `POST /swarm/init` moves the VXLAN UDP port cluster-wide. | B | `underlay` | `os` |
| The default data-path port is 4789. | C | `underlay` | `os` (Windows) |
| On a node that is not yet in a Swarm, `docker swarm init` with a data-path port outside the accepted range exits non-zero, and the node stays out of a Swarm. | B | `api` | `cli` |
| On a node that is not yet in a Swarm, `POST /swarm/init` with a data-path port outside the accepted range responds 500; see [§13](#13-known-divergences--non-goals). | C | `api` | `none` |
| The accepted range is 1024 to 49151, and 0 selects the default port. | C | `api` | `none` |
| An overlay network created without a subnet, ingress included, that uses the default IPAM driver is assigned a subnet inside one of the `DefaultAddrPool` prefixes given to `POST /swarm/init`, with the prefix length `SubnetSize`. | B | `api:mgr` | `none` |
| On a node that is not yet in a Swarm, `docker swarm init` with `--default-addr-pool` exits non-zero, and the node stays out of a Swarm, when `--default-addr-pool-mask-length` is greater than 29 or less than the prefix length of one of the pools. | B | `api` | `cli` |
| On a node that is not yet in a Swarm, `POST /swarm/init` with `DefaultAddrPool` set responds 500 when `SubnetSize`, the prefix length of every subnet allocated from the pools, is greater than 29 or less than the prefix length of one of the pools; see [§13](#13-known-divergences--non-goals). | C | `api` | `none` |
| On a manager, `GET /info` includes `Swarm.Cluster`, with the data-path port, and with `DefaultAddrPool` and `SubnetSize`: those given to `POST /swarm/init`, or `10.0.0.0/8` and 24 when it gave no pool. | C | `api:mgr` | `none` |
| On a manager, `docker info` prints the data-path port, and the default address pool and subnet size: those given to `docker swarm init`, or `10.0.0.0/8` and 24 when it gave none. | C | `api:mgr` | `cli` |
| On a worker, `GET /info` does not include `Swarm.Cluster`. | C | `api:wkr` | `none` |
| On a worker, `docker info` does not print the data-path port or a default address pool. | C | `api:wkr` | `cli` |
| The manager control plane listens on TCP 2377 by default. | C | `node` | `none` |
| Gossip uses TCP and UDP port 7946, which is not configurable. | C | `node` | `none` |
| Whether or not any network is `encrypted`, every TCP connection to a manager's control-plane port carries TLS. | B | `underlay` | `none` |
| Whether or not any network is `encrypted`, a capture of gossip traffic on port 7946, over TCP and UDP, does not contain any of the cluster's network IDs, service names or task addresses in cleartext. | B | `underlay` | `none` |
| An IPv6 `--data-path-addr` produces IPv6 outer packets for overlay traffic. | B | `underlay` | `os` |
| On Linux nodes, with `--live-restore`, `docker swarm init` and `docker swarm join` exit non-zero on a node that is not yet in a Swarm, and the node stays out of a Swarm. | B | `api` | `cli` |
| On Linux nodes, with `--live-restore`, `POST /swarm/init` and `POST /swarm/join` respond 500 on a node that is not yet in a Swarm; see [§13](#13-known-divergences--non-goals). | C | `api` | `none` |
| On Linux nodes, with `--live-restore`, the daemon on a node that is already in a Swarm fails to start. | B | `node` | `none` |
| On Linux nodes, with `--firewall-backend=nftables`, with `--iptables` left enabled, and without `features.swarm-nftables`, `docker swarm init` and `docker swarm join` exit non-zero on a node that is not yet in a Swarm, and the node stays out of a Swarm. | C | `api` | `cli` |
| On Linux nodes, with `--firewall-backend=nftables`, with `--iptables` left enabled, and without `features.swarm-nftables`, `POST /swarm/init` and `POST /swarm/join` respond 500 on a node that is not yet in a Swarm; see [§13](#13-known-divergences--non-goals). | C | `api` | `none` |
| On Linux nodes, with `--firewall-backend=nftables`, with `--iptables` left enabled, and without `features.swarm-nftables`, the daemon on a node that is already in a Swarm fails to start. | C | `node` | `none` |
| On Linux nodes, with `--firewall-backend=nftables`, and with `features.swarm-nftables` set or `--iptables=false`, `docker swarm init` and `docker swarm join` exit zero, and the node is then in a Swarm. | C | `api` | `cli` |
| On Linux nodes, with `--firewall-backend=nftables`, and with `features.swarm-nftables` set or `--iptables=false`, `POST /swarm/init` and `POST /swarm/join` respond 200, and the node is then in a Swarm. | C | `api` | `none` |

## 2. Network lifecycle & API surface

A Swarm-scoped network's driver is `overlay`, or a node-local driver such as `bridge` created with
`--scope swarm`. The manager allocates subnets, task addresses and VIPs only for an overlay network.
For a network of a node-local driver, each node allocates addresses itself.

A Swarm-scoped network of a node-local driver serves services whose tasks only talk to other tasks
on the same node, such as sidecars. Leaving it non-attachable keeps containers that the orchestrator
does not manage off it.

### 2.1 Overlay driver options

The Depends on column here covers only whether the option is accepted and honored. The behavior it
selects is specified in the referenced section, which says what that behavior depends on.

| Key | Expectation | Tier | Depends on |
| --- | --- | --- | --- |
| `com.docker.network.driver.mtu` | Sets the base MTU from which encapsulation overhead is subtracted. See [§3](#3-observable-state-inside-a-container). | B | `none` |
| `com.docker.network.driver.overlay.vxlanid_list` | Pins the VNIs, one per IPAM subnet. See [§2.3](#23-overlay-networks). | B | `none` |
| `encrypted` | Selects an encrypted overlay. See [§9](#9-encryption). | B | `none` |
| `dsr` | Selects direct server return for east-west VIP traffic on Linux nodes. See [§6](#6-load-balancing-east-west-vip). | B | `none` |
| `encrypted`, `dsr` | On Linux nodes, each takes effect when the key is present in the network's options, whatever its value, the empty string and `false` included. | C | `none` |

### 2.2 Swarm-scoped networks

The rows in this subsection hold for any Swarm-scoped network, whatever its driver.

| Expectation | Tier | Observed from | Depends on |
| --- | --- | --- | --- |
| On a worker, `POST /networks/create` for a Swarm-scoped network responds 403. | B | `api:wkr` | `none` |
| On a worker, `docker network create` for a Swarm-scoped network exits non-zero and prints the error the daemon returned; see [§13](#13-known-divergences--non-goals). | B | `api:wkr` | `cli` |
| For a Swarm network other than the ingress network ([§2.4](#24-the-ingress-network)), while a service references it, or a task or container that is not shutting down is attached to it, `DELETE /networks/{id}` responds 400, and the network stays listed. | B | `api:mgr` | `none` |
| `POST /networks/create` with the name of an existing Swarm network responds 409, and `GET /networks` on a manager then lists one network with that name. | B | `api:mgr` | `none` |
| When the daemon refuses one of the requests in [§2.2](#22-swarm-scoped-networks) to [§2.4](#24-the-ingress-network), the `docker network create`, `docker network rm`, `docker service create` or `docker service update` command that made it exits non-zero and prints the daemon's error. | B | `api:mgr` | `cli` |
| Once the manager has allocated the network, `GET /networks/{id}` reports `Scope` as `swarm`, `Created`, the `Driver`, `Ingress`, `Attachable`, `Internal`, `EnableIPv6`, `ConfigFrom` and `Labels` given at create, and, for an overlay network, the `IPAM` the manager allocated. | C | `api:mgr` | `none` |
| `docker network inspect` prints what `GET /networks/{id}` returns, and with `--verbose`, what `GET /networks/{id}?verbose=true` returns. | B | `api:mgr`, `api:wkr` | `cli` |
| `GET /networks` on a worker lists a Swarm-scoped network other than ingress only while it has a local attachment there. | C | `api:wkr` | `none` |
| `docker network ls` lists the networks `GET /networks` returns. | B | `api:mgr`, `api:wkr` | `cli` |
| On any node, `POST /containers/{id}/start` for a container on an attachable Swarm-scoped network responds 204, and the container is running with an address on the network. | B | `api:mgr`, `api:wkr` | `none` |
| On any node, `docker run -d --network` naming an attachable Swarm-scoped network exits zero, and the container runs attached to it. | B | `api:mgr`, `api:wkr` | `cli` |
| On any node, `POST /networks/{id}/connect` for a running standalone container and an attachable Swarm-scoped network responds 200, and the container then has an address on the network. | B | `api:mgr`, `api:wkr` | `none` |
| On any node, `docker network connect` naming an attachable Swarm-scoped network exits zero, and the container is then attached to it. | B | `api:mgr`, `api:wkr` | `cli` |
| After a standalone container disconnects from an attachable Swarm-scoped network, or stops, `GET /tasks` on a manager with the `runtime=attachment` filter stops listing its attachment task, and `DELETE /networks/{id}` on a manager then responds 204 if nothing else is attached to the network. | B | `api:mgr`, `api:wkr` | `none` |
| `POST /containers/create` for a container on a non-attachable Swarm-scoped network responds 201, and `GET /containers/{id}` then reports it in the `created` state. | C | `api:wkr` | `none` |
| `POST /containers/{id}/start` for a container on a non-attachable Swarm-scoped network, the ingress network included, responds 500, with a message that contains `not manually attachable`; see [§13](#13-known-divergences--non-goals). | C | `api:wkr` | `none` |
| `docker run -d --network` naming a non-attachable Swarm-scoped network, the ingress network included, exits non-zero and prints an error that contains `not manually attachable`. | B | `api:wkr` | `cli` |
| A container whose start is refused this way is left in the created state, or removed when it was created with auto-remove (`--rm`). | B | `api:wkr` | `none` |
| A service's tasks run on a Swarm-scoped network of a node-local driver, such as `bridge`, that was created with `ConfigFrom` naming a config-only network; see [§13](#13-known-divergences--non-goals). | B | `api:mgr` | `none` |
| `POST /networks/create` for a Swarm-scoped network of a node-local driver with `EnableIPv4` false, at API version 1.48 or later, or with the `com.docker.network.enable_ipv4` option set to `false`, responds 400 with `IPv4 cannot be disabled in a Swarm scoped network`; see [§13](#13-known-divergences--non-goals). | C | `api:mgr` | `none` |
| `POST /networks/create` for a Swarm-scoped network of a node-local driver without `ConfigFrom`, with a subnet and gateway in `IPAM.Config` and with any of the driver's own options, such as `com.docker.network.bridge.*`, in `Options`, responds 201, and `GET /networks/{id}` on a manager then reports an empty `IPAM` and no `Options`; see [§13](#13-known-divergences--non-goals). | C | `api:mgr` | `none` |
| `GET /networks/{id}` on a node running a task on such a network reports a subnet from the node's default address pools and none of the `Options` given at create; see [§13](#13-known-divergences--non-goals). | C | `api:wkr` | `none` |
| `POST /networks/create` for an overlay network with `ConfigFrom` responds 201 when the config-only network exists on the manager that handles the request, and 404 otherwise; see [§13](#13-known-divergences--non-goals). | C | `api:mgr` | `none` |

### 2.3 Overlay networks

The rows in this subsection depend on the manager's allocation, which only an overlay network gets.

| Expectation | Tier | Observed from | Depends on |
| --- | --- | --- | --- |
| On a manager, `POST /networks/create` for an overlay network with `EnableIPv4` false, at API version 1.48 or later, or with the `com.docker.network.enable_ipv4` option set to `false`, at any version, responds 400; see [§13](#13-known-divergences--non-goals). | C | `api:mgr` | `none` |
| A user-specified `vxlanid_list` is honored: subnet *i* uses VNI *i*. | B | `underlay` | `os` |
| `POST /networks/create` for an overlay network with a `vxlanid_list` VNI that is already in use responds 201, and `GET /networks` on a manager then lists the network; see [§13](#13-known-divergences--non-goals). | C | `api:mgr` | `none` |
| `docker network create` with a `vxlanid_list` VNI that is already in use exits zero, and `docker network ls` on a manager then lists the network. | C | `api:mgr` | `cli` |
| `GET /networks` lists, and `GET /networks/{id}` returns, an overlay network the manager has not allocated, such as one whose pinned VNI is already in use or whose address pool is exhausted. | B | `api:mgr` | `none` |
| An overlay network the manager has not allocated does not stop `GET /networks` from listing, or `GET /networks/{id}` from returning, any other Swarm-scoped network. | B | `api:mgr` | `none` |
| Auto-allocated VNIs come from [4096, 2²⁴). The floor keeps clear of the 802.1Q VLAN range for Windows. | C | `underlay` | `none` |
| `GET /networks/{id}` reports the effective `vxlanid_list` in `Options`. | C | `api:mgr` | `none` |
| `POST /networks/create` for an overlay network responds 201 whatever `com.docker.network.driver.mtu` says, and `GET /networks/{id}` on a manager then reports that value in `Options`; see [§13](#13-known-divergences--non-goals). | C | `api:mgr` | `none` |
| `docker network create --driver overlay` exits zero whatever the `mtu` option says, and `docker network inspect` on a manager then shows that value in the network's options. | C | `api:mgr` | `cli` |
| The VIP that `GET /networks/{id}?verbose=true` reports for a service is the address that load-balances to its tasks. | B | `api:mgr`, `api:wkr` (on a node attached to the network), `task` | `os` |
| Within the reassignment ceiling ([§11.2](#112-convergence)) of a change to a service's tasks, the tasks that `GET /networks/{id}?verbose=true` reports for the service are those that `GET /tasks` on a manager reports in the `running` state. | B | `api:mgr`, `api:wkr` (on a node attached to the network), `task` | `os` |
| On a node attached to the network, `GET /networks/{id}?verbose=true` reports `Services`, giving each service's VIP and published ports and each task's endpoint IP. | C | `api:mgr`, `api:wkr` | `none` |
| On a node that does not have a local attachment to the network, `GET /networks/{id}?verbose=true` does not report any services. | C | `api:mgr`, `api:wkr` | `none` |
| On a manager, at API version 1.52 or later, `IPsInUse` in the `Status.IPAM` of `GET /networks/{id}`, where `{id}` is the network's name or a partial ID, rises when a task attaches and falls when it leaves; see [§13](#13-known-divergences--non-goals). | B | `api:mgr` | `none` |
| On a manager, at API version 1.52 or later, for an IPv4 subnet without an IP range, `IPsInUse + DynamicIPsAvailable` in `GET /networks/{id}`, where `{id}` is the network's name or a partial ID, equals the number of addresses in the subnet, 2^(32 − prefix length), at every point during churn; see [§13](#13-known-divergences--non-goals). | B | `api:mgr` | `none` |
| On a manager, at API version 1.52 or later, `GET /networks/{id}` reports `Status.IPAM` with per-subnet `IPsInUse` and `DynamicIPsAvailable`. | C | `api:mgr` | `none` |
| At API versions before 1.52, `GET /networks/{id}` does not include `Status`. | B | `api:mgr`, `api:wkr` | `none` |
| On a worker attached to the network, at API version 1.52 or later, the `Status.IPAM` of `GET /networks/{id}` reflects only the node's own endpoints: it does not change when a task attaches on another node. | C | `api:wkr` | `none` |

The failed-allocation row was revealed by a regression in which one unallocated network made
listing and inspection fail for every Swarm network, fixed in
[moby/moby#53325](https://github.com/moby/moby/pull/53325).

### 2.4 The ingress network

| Expectation | Tier | Observed from | Depends on |
| --- | --- | --- | --- |
| `swarm init` creates an ingress network. | B | `api:mgr` | `none` |
| While an ingress network exists, `POST /networks/create` for an overlay network with `Ingress` set and `Attachable` not set responds 409. | B | `api:mgr` | `none` |
| `POST /networks/create` with `Ingress` set and a local-scope driver, or no `Driver`, responds 403. | B | `api:mgr` | `none` |
| `POST /networks/create` with `Ingress` set, `Attachable` not set, and a global-scope driver other than `overlay` responds 501. | C | `api:mgr` | `none` |
| `POST /networks/create` with both `Ingress` and `Attachable` set and a global-scope driver responds 400. | B | `api:mgr` | `none` |
| `POST /services/create` or `POST /services/{id}/update` that names the ingress network in `TaskTemplate.Networks` responds 400, and the service is not created or not changed. | B | `api:mgr` | `none` |
| While any service publishes a port in ingress mode, `DELETE /networks/{id}` for the ingress network responds 400, and the network stays listed. | B | `api:mgr` | `none` |
| While no ingress network exists, `POST /services/create` and `POST /services/{id}/update` with a port in ingress mode respond 400, and the service is not created or not changed. | B | `api:mgr` | `none` |
| After the ingress network is removed and recreated with a custom subnet and gateway, `GET /networks/ingress` on a manager reports that subnet and gateway. | B | `api:mgr` | `none` |
| A service created after that is reachable on its ingress-mode published ports on every node. | B | `client` | `os` |
| `GET /networks` on every node lists the ingress network. | C | `api:mgr`, `api:wkr` | `none` |

## 3. Observable state inside a container

| Expectation | Tier | Observed from | Depends on |
| --- | --- | --- | --- |
| A sandbox interface exists and passes traffic on the overlay. | B | `task`, `ctr` | `os` |
| The interface name follows `com.docker.network.endpoint.ifname` when set; see [§13](#13-known-divergences--non-goals). | B | `task`, `ctr` | `os` |
| Otherwise, on Linux nodes, the interface is named `eth<N>`. | C | `task`, `ctr` | `os` |
| `POST /containers/{id}/start` and `POST /networks/{id}/connect` for an endpoint whose `com.docker.network.endpoint.ifname` the OS refuses respond 500, with a message that contains `error renaming interface` and the requested name; see [§13](#13-known-divergences--non-goals). | C | `api:wkr` | `none` |
| `docker run -d` and `docker network connect` with such an `ifname` exit non-zero, and print an error that contains the requested name. | B | `api:wkr` | `cli` |
| A service task whose network attachment asks for such an `ifname` fails, and its error in `GET /tasks` contains `error renaming interface` and the requested name. | B | `api:mgr` | `none` |
| `POST /services/create` for a service whose network attachment asks for an `ifname` responds 201, and `GET /services` then lists the service, for any value of `ifname`; see [§13](#13-known-divergences--non-goals). | C | `api:mgr` | `none` |
| When two of a container's endpoints ask for the same interface name, or one asks for `lo`, the start fails, or else the connect that attaches the second of them fails. The response is 500 with a message that names the interface; see [§13](#13-known-divergences--non-goals). | C | `api:wkr` | `none` |
| In that case, `docker run -d` or `docker network connect` exits non-zero, and prints an error that names the interface. | B | `api:wkr` | `cli` |
| The interface holds an IPv4 address from the network's IPAM. | B | `task`, `ctr` | `os` |
| A standalone container's statically configured address is honored. | B | `ctr` | `os` |
| `POST /containers/{id}/start` for a container whose static IPv4 address on an attachable overlay is in use, or, on a node without a local attachment to the network, is outside the network's subnets, responds 500 after 20 seconds with `attaching to network failed, make sure your network options are correct and check manager logs: context deadline exceeded`; see [§13](#13-known-divergences--non-goals). | C | `api:wkr` | `none` |
| `docker run -d --network --ip` with such an address exits non-zero. | B | `api:wkr` | `cli` |
| After `POST /networks/{id}/disconnect` for a running standalone container and an attachable Swarm-scoped network responds 200, the container does not have an interface or address on that network; see [§13](#13-known-divergences--non-goals). | B | `ctr` | `os` |
| The MAC address is honored when the endpoint's `MacAddress` is set, at container create at API version 1.44 or later, or on `POST /networks/{id}/connect` at API version 1.54 or later. | B | `ctr` | `os` |
| Otherwise, on Linux nodes, the MAC is `02:42:` followed by the four octets of the IPv4 address. | C | `task`, `ctr` | `none` |
| The MTU reported on the interface is exactly the largest payload that crosses the overlay between nodes unfragmented, on both cleartext and `encrypted` networks, over an IPv4 underlay. Over an IPv6 underlay it does not hold; see [§13](#13-known-divergences--non-goals). | B | `task` → `task` | `os` |
| On a cleartext network, inner MTU is `base − 50`, where `base` is the `mtu` option, or 1500 when it is 0 or not set. By default it is 1450. | C | `task` | `os` (Windows) |
| On Linux nodes, on an `encrypted` network, inner MTU is `base − 50 − 26`, rounded down to a multiple of 4. By default it is 1424. | C | `task` | `none` |
| Setting `com.docker.network.driver.mtu` moves the reported MTU. | B | `task` | `os` |
| On Windows nodes, on a cleartext overlay without `com.docker.network.driver.mtu`, an endpoint's MTU is the one Linux nodes report for the same network. | B | `task` on two nodes | `os` |
| When nodes' inner MTUs differ, frames from the larger-MTU side that exceed the smaller MTU are dropped at the receiving node, without an ICMP error. TCP is unaffected because each side keeps its segments within the MSS the other advertises. | C | `task` on two nodes | `os` |
| A container on a non-internal overlay has a default route. | B | `task`, `ctr` | `os` |
| A container attached only to `internal` overlays does not have a default route. | B | `task`, `ctr` | `os` |
| On a multi-subnet overlay on a Linux node, static routes to the other subnets are present. | C | `task` | `os` |

The mismatch row documents a failure mode that is hard to diagnose and common in the field,
especially where the overlay runs inside further encapsulation: large TCP transfers work, while
large UDP datagrams and don't-fragment pings disappear without an error.

## 4. East-west reachability

Unless a row says otherwise, every expectation below holds whether the sender and receiver are on
the same node or on different nodes. It also holds in all four combinations of the two, with each
end either a service task or a standalone container on an `attachable` overlay. Any one of these
cases can fail on its own because the two placements take different paths through the overlay and
tasks and standalone containers are attached to it differently.

| Expectation | Tier | Observed from | Depends on |
| --- | --- | --- | --- |
| A unicast frame sent by a container reaches the container it is addressed to on the same overlay subnet. | B | `task`, `ctr` | `os` |
| A broadcast frame sent by a container reaches every other container on the same overlay subnet. | B | `task`, `ctr` | `os` |
| A multicast datagram sent to a group by a container reaches every other container on the same overlay subnet that has joined the group. | B | `task`, `ctr` | `os` |
| A unicast packet sent by a container on subnet A reaches the container it is addressed to on subnet B of the same overlay network. | B | `task`, `ctr` | `os` |
| TCP, UDP and SCTP all pass. | B | `task`, `ctr` | `os` |
| A don't-fragment packet larger than the reported MTU fails at the sender with `EMSGSIZE`. | B | `task`, `ctr` | `os` (Windows) |
| A container cannot reach another container's address on an overlay network it is not attached to. | B | `task`, `ctr` | `os` |
| On Linux nodes, two containers on the same node cannot reach each other via their addresses on a gateway network the daemon created, because inter-container communication is disabled there. This is an isolation property: enabling inter-container communication there would let containers on unrelated overlays reach each other. | B | `task`, `ctr` | `os` |

## 5. Service discovery

A row in this section names `none` when the embedded resolver answers the query from its own
records, which it does the same way on every platform. Every row holds for a query made from a
service task or from a standalone container, whichever kind of workload the name being resolved
belongs to.

How a container resolves names, whatever its network's driver, is specified in
[container-dns.md](container-dns.md): the `resolv.conf` it gets, how answers are built, and what is
forwarded upstream. This section covers the names Swarm adds, on overlay networks unless a row says
otherwise. In this section, a service's running tasks are those that `GET /tasks` reports with
`Status.State` and `DesiredState` both `running`, other than a task whose container has reported
unhealthy. For the rows there that depend on a container's DNS options, a service task is created
with those in the service's DNS config.

| Expectation | Tier | Observed from | Depends on |
| --- | --- | --- | --- |
| The expectations in [container-dns.md](container-dns.md) hold inside service tasks, each with the tier and dependencies it has there, and for the names and addresses of the endpoints in this section, including endpoints on other nodes. | B | `task`, `ctr` | `os` |
| In vip mode, `<service>` resolves to the service's VIP on one of the overlay networks, other than the ingress network, that the querier shares with it. | B | `task`, `ctr` | `none` |
| In dnsrr mode, `<service>` resolves to the addresses of the running tasks. | B | `task`, `ctr` | `none` |
| `GET /services/{id}` for a service in `dnsrr` endpoint mode does not report a VIP in `Endpoint.VirtualIPs`. | B | `api:mgr` | `none` |
| `tasks.<service>` resolves to the addresses of all running tasks. | B | `task`, `ctr` | `none` |
| A service network alias resolves as `<service>` does: to the VIP in vip mode, and to the addresses of the running tasks in dnsrr mode. | B | `task`, `ctr` | `none` |
| `tasks.<alias>` resolves to task addresses. | B | `task`, `ctr` | `none` |
| A task's container name resolves to its overlay address. | B | `task`, `ctr` | `none` |
| That name is `<service>.<slot>.<taskID>`, or `<service>.<nodeID>.<taskID>` for global services. | C | `task`, `ctr` | `none` |
| A task with a healthcheck contributes no records until it first reports healthy. | B | `task`, `ctr` | `none` |
| Once a task's desired state is no longer `running`, its container has reported unhealthy, or its container has stopped, its container name and its addresses under `tasks.<service>` and, in dnsrr mode, under `<service>` stop resolving. | B | `task`, `ctr` | `none` |
| Once no container of a removed service is running, the service's name and aliases stop resolving. | B | `task`, `ctr` | `none` |
| Once `POST /containers/{id}/rename` for a standalone container on an attachable overlay responds 204, containers on other nodes resolve its new name, and not its old one, within the reassignment ceiling ([§11.2](#112-convergence)). | B | `task`, `ctr` | `none` |
| A network alias remains continuously resolvable across a rolling update in which old and new tasks both claim it. | B | `task`, `ctr` | `none` |
| When a name resolves on more than one of the querier's networks, the resolver answers from a Swarm-scoped network in preference to a local one, such as a user-defined bridge network the container is also attached to. | B | `ctr` | `os` (Windows) |
| When two services publish ports in ingress mode and have no other network in common, a task of either does not resolve the other's name or the names of the other's tasks. | B | `task` | `none` |
| On a Swarm-scoped network of a node-local driver, a task's names resolve for containers on its node attached to the network, as a container's names do on any user-defined network. | B | `task`, `ctr` | `none` |
| On such a network, `<service>`, `tasks.<service>` and service aliases do not resolve, and a healthcheck does not delay a task's records. | C | `task`, `ctr` | `none` |

## 6. Load balancing (east-west VIP)

The client sending to a VIP can be a service task or a standalone container on an `attachable`
overlay, and every row in this section holds for either. The backends behind a VIP are always
service tasks. A row observed at the receiving end is observed from `task`.

| Expectation | Tier | Observed from | Depends on |
| --- | --- | --- | --- |
| Traffic to a service VIP is delivered to that service's tasks. | B | `task`, `ctr` | `os` |
| TCP, UDP and SCTP are all load-balanced. | B | `task`, `ctr` | `os` |
| All packets of a flow reach the same task. | B | `task`, `ctr` | `os` |
| Traffic to any port on the VIP is forwarded, whether or not that port is published. | B | `task`, `ctr` | `os` |
| In vip mode, `GET /services/{id}` reports, in `Endpoint.VirtualIPs`, one VIP for each overlay network the service is attached to, from that network's subnet, and one on the ingress network while the service publishes an ingress-mode port. | B | `api:mgr` | `none` |
| After `POST /services/{id}/update` that leaves a service in vip mode and attached to a network responds 200, `GET /services/{id}` reports the same VIP on that network as before. | B | `api:mgr` | `none` |
| After a change of leader, `GET /services/{id}` reports the same VIPs as before. | B | `api:mgr` | `none` |
| `docker service inspect` prints what `GET /services/{id}` returns. | B | `api:mgr` | `cli` |
| Traffic to a service VIP from a task or container on a node that does not run a task of the service is delivered to the service's tasks. | B | `task` or `ctr` on a task-free node | `os` |
| In NAT mode (the default), the task observes a source address that is stable for each client node and is neither the client's nor any task's. | B | `task` | `os` |
| In NAT mode, that source address is the load-balancer address of the client's node on the overlay. | C | `task` | `os` |
| In NAT mode, the destination the task observes is its own address. | B | `task` | `os` |
| In DSR mode (the `dsr` option), the task observes the client's real source address. | B | `task` | `os` |
| In DSR mode, the destination the task observes is the VIP. | B | `task` | `os` |
| A direct connection to another task's own address on the same subnet is not source-rewritten. | B | `task` | `os` |

### 6.1 Distribution

The distribution property admits round-robin, equal-weight random, and 5-tuple hashing, and excludes
least-connections and other schedulers that depend on backend state. Clause (1), a limit, cannot
exclude them on its own because with identical backends least-conn also converges to 1/*k*. Clause
(2), invariance, is what tells them apart. There are defensible reasons to switch to a scheduler
that depends on backend state. The property is tier C for that reason: such a switch should fail its
test and be made deliberately.

> Let *H* be the set of healthy tasks, |*H*| = *k*, held constant. For *n* client flows let *fᵢ(n)*
> be the number delivered to task *i*. Then (1) lim *fᵢ(n)/n* = 1/*k* for every *i* ∈ *H*, and
> (2) that limit is invariant to any observable property of the tasks: connection holding time,
> in-flight concurrency, latency, byte volume, or load.

| Expectation | Tier | Observed from | Depends on |
| --- | --- | --- | --- |
| Properties (1) and (2) above hold for east-west VIP and, separately, for ingress. | C | `task`, `client` | `os` |

### 6.2 Health and drain

Every row in this subsection holds for flows through the service's VIP and for flows that enter
through an ingress-mode published port.

| Expectation | Tier | Observed from | Depends on |
| --- | --- | --- | --- |
| A task with a healthcheck receives no traffic until it first reports healthy. | B | `task` | `os` |
| A task with no healthcheck receives traffic as soon as it is running. | B | `task` | `os` |
| A task that turns unhealthy receives no further traffic. | B | `task` | `os` |
| When a task shuts down gracefully, a connection to it established before its container was sent its stop signal keeps exchanging data in both directions until the task closes it, within the task's stop grace period. | B | `task` | `os` |
| When a task shuts down gracefully, no flow that starts after its container has been sent its stop signal is delivered to it; see [§13](#13-known-divergences--non-goals). | B | `task` | `os` |

## 7. Published ports

A service publishes a port in one of two modes: `mode=ingress`, the default, through the routing
mesh on every node, or `mode=host`, directly on the node running each task. The expectations in this
section hold for both modes. [§7.1](#71-hairpin) covers hairpin paths, [§7.2](#72-modeingress) what
is specific to the routing mesh, and [§7.3](#73-modehost) what is specific to `mode=host`.

Every expectation in this section and its subsections holds whether or not the `dsr` option is
set on the overlay networks a service is attached to, other than the ingress network.

| Expectation | Tier | Observed from | Depends on |
| --- | --- | --- | --- |
| The task receives the flow on its target port. | B | `task` | `os` |
| TCP, UDP and SCTP are all published. | B | `client` | `os` |
| Replies from the task reach the client. | B | `client` | `os` |
| A UDP datagram to a published port gets an ICMP port-unreachable error at the client when nothing in the task listens on the target port. | B | `client` | `os` |
| Ports that are not published are not reachable from outside. | B | `client` | `os` |
| `POST /services/create` or `POST /services/{id}/update` with a spec that lists the same published port and protocol twice responds 400, whatever the publish modes of the two entries, and the service is not created or not changed. | B | `api:mgr` | `none` |
| When the daemon refuses one of the service requests in [§7](#7-published-ports) to [§7.3](#73-modehost), `docker service create` or `docker service update` exits non-zero and prints the daemon's error. | B | `api:mgr` | `cli` |
| `docker service create` and `docker service update` with `--publish`, in its short and long forms, `--endpoint-mode` and `--network` with `alias=` exit zero, and `GET /services/{id}` then reports the ports, publish modes, endpoint mode and aliases the flags name. | B | `api:mgr` | `cli` |
| Once every task of a removed service has stopped, nothing listens on its published ports; see [§13](#13-known-divergences--non-goals). | B | `client`, `node` | `os` |
| Once every task from before a change of a service's published port has stopped, nothing listens on the old port; see [§13](#13-known-divergences--non-goals). | B | `client`, `node` | `os` |
| Changing a service's published port publishes the new one. | B | `client`, `node` | `os` |
| With the userland proxy enabled, a published port is reachable over IPv6 at the node's IPv6 addresses, from off-node, from the node's own address and from `::1`. This holds for TCP and UDP. | B | `client`, `node` | `os` |
| IPv6 traffic to a published port is never accepted and then left unserved. | B | `client`, `node` | `os` |
| With the userland proxy disabled, a published port is closed for IPv6: TCP gets `ECONNREFUSED`, UDP gets ICMPv6 port-unreachable. | C | `client`, `node` | `os` |

These IPv6 rows do not mean the service mesh supports IPv6; it is IPv4-only
([§13](#13-known-divergences--non-goals)). The task does not have an IPv6 address. The proxy accepts
on `[::]` and forwards to the task over IPv4, as it does for any IPv4-only container. So the task
sees the connection coming from the proxy, and the real-source row in [§7.3](#73-modehost) does not
hold for IPv6 clients. The row about traffic left unserved was revealed by a bug in which IPv6
connections to a published port were accepted and never served
([moby/moby#53091](https://github.com/moby/moby/issues/53091)). Closing the port without the proxy
is how that is avoided, and it is tier C because making IPv6 reachable without the proxy would be a
legitimate change.

### 7.1 Hairpin

The six paths below are independent and can fail separately. The first five hold for both publish
modes: for `mode=host`, on the nodes running a task. In this subsection, the node's address is one
of its non-loopback IPv4 addresses, and a task or container that connects to it is attached to at
least one network that is not `internal`.

| Expectation | Tier | Observed from | Depends on |
| --- | --- | --- | --- |
| A published port is reachable from the node itself via `127.0.0.1`. | B | `node` | `os` |
| A published port is reachable from the node itself via the node's address. | B | `node` | `os` |
| A published port is reachable from a container on a local bridge network, via the node's address. | B | `ctr` | `os` |
| A published port is reachable from a task or container on the same node that is attached to one of the service's overlay networks, via the node's address. | B | `task`, `ctr` | `os` |
| A published port is reachable from a task or container on the same node that is attached only to overlay networks the service is not on, via the node's address. | B | `task`, `ctr` | `os` |
| In `mode=host`, a task reaches its own published port via its node's address. | B | `task` | `os` |
| Each of the above holds with and without the userland proxy. | B | `node`, `ctr`, `task` | `os` |

On Linux nodes, the two overlay rows and the `mode=host` self-hairpin row are the paths that go back
into the bridge they came out of. An overlay container reaches the node's address through
`docker_gwbridge`, as does the ingress load balancer or the task that serves the port. Those paths
alone are subject to the bridge's inter-container communication setting. A gateway network the
daemon creates has inter-container communication disabled ([§8.1](#81-gateway-network)). Since a
published port is an explicit export to anything that can route to the node, that setting must not
block it.

The userland-proxy row exists because the proxy hides these paths. With the proxy, the connection
ends in the host's network namespace and never re-enters the bridge; without it, the traffic is
DNATed back across the bridge. Access control should not depend on that setting. Reaching a
published port from an overlay regressed unnoticed in 29.8.0 and 29.8.1 because it broke only with
`--userland-proxy=false` ([moby/moby#53713](https://github.com/moby/moby/issues/53713)).

### 7.2 `mode=ingress`

The routing mesh publishes an ingress-mode port on every node, whichever nodes run the service's
tasks.

| Expectation | Tier | Observed from | Depends on |
| --- | --- | --- | --- |
| A published port is reachable on every node, including nodes running no task of the service. | B | `client` | `os` |
| All packets of a flow reach the same task. | B | `client` | `os` |
| The task does not observe the client's real source address. | C | `task` | `os` |
| A task is not reachable from another task via its ingress-network address, or via its service's VIP on the ingress network, on any port. | B | `task` → `task` | `os` |
| A task cannot reach the outside world through its ingress-network link. | B | `task` → `underlay` | `os` |
| `POST /services/create` or `POST /services/{id}/update` for a service in `dnsrr` endpoint mode that publishes a port in ingress mode responds 400, whether or not the port is chosen, and the service is not created or not changed. | B | `api:mgr` | `none` |

#### 7.2.1 Port allocation

| Expectation | Tier | Observed from | Depends on |
| --- | --- | --- | --- |
| `POST /services/create` with an ingress-mode port and no `PublishedPort` responds 201, and the service is assigned a published port that no other service publishes in ingress mode for the same protocol. | B | `api:mgr` | `none` |
| That port comes from 30000 to 32767. | C | `api:mgr` | `none` |
| `POST /services/create` with an ingress-mode `PublishedPort` responds 201, and the service is published on exactly that port, cluster-wide. | B | `api:mgr` | `none` |
| `POST /services/create` with an ingress-mode `PublishedPort` from 1 to 65535, including one in the auto-assignment range, responds 201, and `GET /services/{id}` then reports that port in `Endpoint.Ports`. | C | `api:mgr` | `none` |
| `POST /services/create` with an ingress-mode `PublishedPort` above 65535 responds 201, and `GET /services/{id}` then does not report the port in `Endpoint.Ports`; see [§13](#13-known-divergences--non-goals). | C | `api:mgr` | `none` |
| A chosen ingress-mode port in the auto-assignment range is not auto-assigned to another service while it is held. | B | `api:mgr` | `none` |
| `GET /services/{id}` reports a service's published port in `Endpoint.Ports`, whether it was auto-assigned or chosen. | B | `api:mgr` | `none` |
| The service is reachable on the port `GET /services/{id}` reports, whether it was auto-assigned or chosen. | B | `api:mgr`, `client` | `os` |
| `POST /services/{id}/update` that leaves the number of ports unchanged responds 200, and `GET /services/{id}` then reports the same auto-assigned published port for each port whose name, protocol and target port are unchanged. | B | `api:mgr` | `none` |
| After `POST /services/{id}/update` adds or removes a published port, in either publish mode, `GET /services/{id}` reports a newly assigned port for each of the service's other auto-assigned ingress-mode ports; see [§13](#13-known-divergences--non-goals). | C | `api:mgr` | `none` |
| `POST /services/create` or `POST /services/{id}/update` that publishes in ingress mode a port and protocol another service publishes in ingress mode responds 400, and the service is not created or not changed; see [§13](#13-known-divergences--non-goals). | B | `api:mgr` | `none` |
| While one service publishes a port number for TCP, `POST /services/create` for another that publishes the same number for UDP responds 201, and both are then published. | B | `api:mgr` | `none` |
| `POST /services/create` or `POST /services/{id}/update` that names a `PublishedPort` and protocol in one publish mode that another service publishes in the other responds 400, whichever service came first, and the service is not created or not changed; see [§13](#13-known-divergences--non-goals). | B | `api:mgr` | `none` |
| On every node, `POST /containers/create` for a container that publishes a port and protocol a service publishes in ingress mode responds 201. | C | `api:wkr` | `none` |
| On every node, `POST /containers/{id}/start` for that container responds 500, with a message that contains `port is already allocated`; see [§13](#13-known-divergences--non-goals). | C | `api:wkr` | `none` |
| On every node, `docker run -d -p` on a port and protocol a service publishes in ingress mode exits non-zero, and prints an error that contains `port is already allocated`. | B | `api:wkr` | `cli` |
| On a node where one of a service's ingress-mode ports is already in use, the routing mesh does not serve any of the service's published ports once no task from an earlier version of its spec remains. | B | `client` | `os` |
| After a service update that adds such a port, the routing mesh on that node keeps serving the previous version's ports to the tasks from before the update until they are gone. | C | `client` | `os` |

### 7.3 `mode=host`

| Expectation | Tier | Observed from | Depends on |
| --- | --- | --- | --- |
| The port is bound only on nodes running a task. | B | `client` | `os` |
| An IPv4 client reaches the task through the published port on the task's node, and the task observes the client's real source address. | B | `client`, `task` | `os` |
| With the userland proxy enabled, the task does not observe an IPv6 client's real source address. | C | `task` | `none` |
| A connection to a published port on a node is delivered to the task on that node that published it. | B | `client` | `os` |
| `POST /services/create` for a service in `dnsrr` endpoint mode that publishes a port in `mode=host` responds 201, and its tasks publish the port on their nodes. | B | `api:mgr` | `none` |
| With `PublishedPort` omitted, an ephemeral host port is assigned. | B | `api:mgr` | `none` |
| `GET /tasks/{id}` reports that port in `Status.PortStatus`, and `GET /containers/json` on the task's node reports it in the container's `Ports`. | B | `api:mgr`, `api:wkr` (on the task's node) | `none` |
| `docker service ps` on a manager, and `docker container ls` on the task's node, print that port. | B | `api:mgr`, `api:wkr` (on the task's node) | `cli` |
| `POST /services/create` for a service that publishes in `mode=host` a port and protocol another service publishes in `mode=host` responds 201, and both services are then listed. | B | `api:mgr` | `none` |
| Tasks that publish the same fixed `mode=host` port and protocol are never placed on the same node, whether they belong to one service or several. | B | `api:mgr` | `none` |
| A task with no eligible node for its fixed `mode=host` port stays pending. | B | `api:mgr` | `none` |

## 8. Egress & the gateway network

| Expectation | Tier | Observed from | Depends on |
| --- | --- | --- | --- |
| A container on a non-internal overlay has egress to the physical network. | B | `task` or `ctr` → `underlay` | `os` |
| The source address an underlay host observes is the node's address, not the container's. | B | `underlay` | `os` |
| A container attached only to `internal` overlays does not have egress. | B | `task`, `ctr` | `os` |
| A container on both an internal and a non-internal overlay has egress, and its default route is not on the internal overlay. | B | `task`, `ctr` | `os` |

### 8.1 Gateway network

Each node has a gateway bridge network, which is how containers on its non-internal overlays reach
the physical network. This subsection holds on Linux nodes.

| Expectation | Tier | Observed from | Depends on |
| --- | --- | --- | --- |
| A gateway bridge network is created automatically when a node joins a Swarm that has an ingress network. | C | `node`, `api:wkr` | `none` |
| In a Swarm with no ingress network, a node's gateway bridge network is created automatically when a container on a non-internal overlay first starts on it. | C | `node`, `api:wkr` | `none` |
| It is named `docker_gwbridge`, as a network and as its bridge interface. | B | `node`, `api:wkr` | `none` |
| When the operator has created `docker_gwbridge` before a node needs a gateway network, `GET /networks/docker_gwbridge` on that node reports the operator's subnet and options after containers on non-internal overlays have started there. | B | `api:wkr` | `none` |
| Those containers' addresses on the gateway network are in the operator's subnet. | B | `task`, `ctr` | `os` |
| On a gateway network the daemon creates, `GET /networks/{id}` reports `com.docker.network.bridge.enable_icc` as `false` in `Options`. | C | `api:mgr`, `api:wkr` | `none` |
| A container's interface on a gateway network the daemon creates does not have an IPv6 address. | C | `task`, `ctr` | `os` |
| Traffic from the underlay that a host routes through the node to a container's address on the gateway network does not reach the container on a port it does not publish. | B | `underlay` → `task` or `ctr` | `os` |

## 9. Encryption

Encrypted overlay networks are implemented on Linux nodes only
([§13](#13-known-divergences--non-goals)), and this section holds on Linux nodes except where a row
names Windows. Which XFRM algorithms a distribution's kernel offers, if any, is up to the OS vendor,
which is why the rows about the traffic an underlay host observes name `os`. The missing-XFRM row
names `none` because its test stands in for the missing support with a failpoint.

| Expectation | Tier | Observed from | Depends on |
| --- | --- | --- | --- |
| No cleartext VXLAN traffic is observable between nodes on the underlay network for the VNIs of `encrypted` overlay networks. | B | `underlay` | `os` |
| Traffic between nodes on the underlay network for the VNIs of `encrypted` overlay networks is observable as IPsec ESP. | C | `underlay` | `os` |
| ESP is in transport mode, protecting the VXLAN datagrams. | C | `underlay` | `os` |
| An encrypted network's IPsec security associations use `rfc4106(gcm(aes))` with a 64-bit ICV. | C | `node` | `none` |
| A forged cleartext VXLAN datagram for the VNI of an `encrypted` overlay network, injected from the underlay, does not reach the network's tasks. | B | `underlay` → `task` | `os` |
| Cleartext VXLAN for a cleartext network's VNI passes between nodes, whether or not they also have `encrypted` networks. | B | `underlay` | `os` |
| After an encrypted network is deleted, a cleartext network allocated the same VNI passes cleartext VXLAN. | B | `underlay` | `os` |
| After a node's daemon is killed while an encrypted network is programmed on it, and that network is deleted before the node programs it again, a cleartext network allocated the same VNI passes traffic to and from the node. | B | `task` | `os` |
| After an encrypted network is deleted, containers on Windows nodes on a cleartext network allocated the same VNI can exchange traffic with containers on the Linux nodes that had containers on the encrypted network. | B | `task`, `ctr` | `os` |
| Across consecutive rotations of the network encryption keys, each of which replaces the gossip and IPsec keys together, traffic between tasks on different nodes of an `encrypted` network does not lose packets, without any action by the operator. | B | `task` | `os` |
| On a node that does not have the network encryption keys, a task attached to an `encrypted` network is rejected, and its error in `GET /tasks` contains `cannot join secure network: encryption keys not present`. | B | `api:mgr` | `none` |
| On a host without kernel XFRM support, a task attached to an `encrypted` network is rejected, and its error in `GET /tasks` contains `cannot join secure network: required modules to install IPSEC rules are missing on host`. | B | `api:mgr` | `none` |
| On such a host, `POST /containers/{id}/start` for a container on an attachable `encrypted` network responds 500 after 20 seconds, with `attaching to network failed, make sure your network options are correct and check manager logs: context deadline exceeded`; see [§13](#13-known-divergences--non-goals). | C | `api:wkr` | `none` |
| On such a host, `docker run -d --network` naming an attachable `encrypted` network exits non-zero. | B | `api:wkr` | `cli` |
| A container whose start is refused this way is left in the created state, or removed when it was created with auto-remove (`--rm`). | B | `api:wkr` | `none` |

## 10. Wire format & interop

Everything here is hard contract; see [Interop premise](#interop-premise).

The rows are grouped by the kind of overlay network they apply to. The rows about what an underlay
host sees without the network's keys apply to cleartext networks only because on an encrypted
network the VXLAN datagram is not visible on the wire.

### 10.1 All overlay networks

| Expectation | Tier | Observed from | Depends on |
| --- | --- | --- | --- |
| A container is reachable from other nodes whether its IP address and its MAC address are each assigned manually or automatically. | B | `task`, `ctr` | `os` |
| In a cluster that mixes nodes of the previous release and the release under test, the rows of [§4](#4-east-west-reachability), [§5](#5-service-discovery), [§6](#6-load-balancing-east-west-vip), [§7.2](#72-modeingress) and [§9](#9-encryption) hold between nodes of different versions, on cleartext and `encrypted` overlays and on the ingress network. | B | `client`, `task`, `underlay` | `os` |
| In a cluster that mixes Linux nodes using the iptables and the nftables firewall backends, the rows of [§4](#4-east-west-reachability), [§6](#6-load-balancing-east-west-vip), [§7.2](#72-modeingress) and [§9](#9-encryption) hold between nodes with different backends, on cleartext and `encrypted` overlays and on the ingress network. | B | `client`, `task`, `underlay` | `os` |

### 10.2 Cleartext overlay networks

The default data-path port is pinned in [§1](#1-cluster--node-configuration), and the range of
auto-allocated VNIs in [§2.3](#23-overlay-networks).

| Expectation | Tier | Observed from | Depends on |
| --- | --- | --- | --- |
| Overlay traffic between hosts is carried in UDP datagrams, over IPv4 or IPv6, to the configured data-path port. | B | `underlay` | `os` |
| The datagram carries a VXLAN header with the network's VNI. | B | `underlay` | `os` |
| The VXLAN payload is an Ethernet frame carrying container-to-container traffic. | B | `underlay` | `os` |

### 10.3 Encrypted overlay networks

Encrypted traffic between hosts is IPsec ESP. [§9](#9-encryption) covers the encryption and the rest
of what an encrypted network promises. The rows below are about the decrypted payload, which is the
same whatever the encryption. An underlay host can only see it by decrypting with a node's keys.
This subsection holds on Linux nodes, as [§9](#9-encryption) does.

| Expectation | Tier | Observed from | Depends on |
| --- | --- | --- | --- |
| The decrypted payload is a UDP datagram to the configured data-path port. | B | `underlay` (decrypted with a node's keys) | `os` |
| That datagram carries a VXLAN header with the network's VNI. | B | `underlay` (decrypted with a node's keys) | `os` |
| Its VXLAN payload is an Ethernet frame carrying container-to-container traffic. | B | `underlay` (decrypted with a node's keys) | `os` |

## 11. Failure & lifecycle

### 11.1 Restart and reclamation

| Expectation | Tier | Observed from | Depends on |
| --- | --- | --- | --- |
| After a graceful daemon restart, the node's tasks are reachable over their overlay networks. | B | `task` | `os` |
| After a graceful daemon restart, the node's published ports answer. | B | `client` | `os` |
| A graceful daemon restart on one node does not interrupt flows between clients and tasks on other nodes, whether they go to a VIP or enter through a published port on another node. | B | `task`, `client` | `os` |
| On Linux nodes, after `SIGKILL` and restart, every network namespace the daemon created before the kill is removed, other than an overlay network's; see [§13](#13-known-divergences--non-goals). | B | `node` | `os` |
| On Linux nodes, while an overlay network is programmed on the node, no namespace, bridge or VXLAN device that a killed daemon left for that network, or for another network with one of its VNIs, remains; see [§13](#13-known-divergences--non-goals). | B | `node` | `os` |
| On Linux nodes, an overlay network's namespace left by a killed daemon, with its bridge and VXLAN device, is removed when the node programs that network again, or another network with one of its VNIs; see [§13](#13-known-divergences--non-goals). | C | `node` | `os` |
| On Windows nodes, while an overlay network exists on the node, no HNS overlay network that a killed daemon left with the same ID, or with an overlapping VNI, remains; see [§13](#13-known-divergences--non-goals). | B | `node` | `os` |
| On Windows nodes, an HNS overlay network left by a killed daemon is removed when the node creates a network with the same ID, or one whose VNI overlaps it; see [§13](#13-known-divergences--non-goals). | C | `node` | `os` |
| After the daemon is killed and restarted, a network allocated a VNI that was in use on the node before the kill is programmed on the node and passes traffic, whether it is the same network or another. | B | `node`, `task` | `os` |
| After the daemon is killed and restarted, the node's tasks are reachable over their overlay networks. | B | `task` | `os` |
| After the daemon is killed and restarted, the node's published ports answer. | B | `client` | `os` |

### 11.2 Convergence

There are three convergence classes, each with a different dominant term. One bound cannot serve all
three.

Each class has a B ceiling, chosen by judgment to encode "a human would call this broken", and it is
never re-baselined. That keeps it free of flakes while it still catches the regression that matters:
convergence going from seconds to minutes, or to never.

| Class | Dominant term | B ceiling |
| --- | --- | --- |
| Reassignment: an address moving to another endpoint, or a task replaced by one on another node | gossip propagation and FDB reprogramming | 300 s |
| Partition heal | reconnection to the failed peers, then full-state syncs | 180 s |
| Node leave and rejoin | full cluster re-formation and network re-programming | 120 s |

Each ceiling is timed from when the change is complete: the new endpoint or task running, the
partition removed, the rejoined node's task running. For a row about a removal that reaches a node
late, the ceiling is timed from when the removal can reach it.

Unless a row says otherwise, each end of the traffic in the rows below can be a service task or a
standalone container on an `attachable` overlay, in all four combinations. Each row states the
node placements it covers.

| Expectation | Tier | Observed from | Depends on |
| --- | --- | --- | --- |
| After overlay address X moves from endpoint S1 to S2, a client that could reach X at S1 reaches it at S2 within the reassignment ceiling, with no manual `ip neigh`, `arp -d` or host flush. This holds whether S2 is on S1's node or another node, and whether the client is on S1's node, S2's node or a third node. | B | `task`, `ctr` | `os` |
| This holds after any number of moves of X, including when a node learns that X's previous endpoint was removed only after X's new endpoint on that node has joined. | B | `task`, `ctr` | `os` |
| An address move converges within the reassignment ceiling whether or not the MAC moves with it. | B | `task`, `ctr` | `os` |
| When a task is replaced by one on another node, a client that could reach the old task reaches the new one within the reassignment ceiling, with no manual intervention. This holds whether the client is on the old task's node, the new task's node or a third node. | B | `task`, `ctr` | `os` |
| During a node partition, once a side declares the other side's nodes failed, `GET /networks/{id}` on its nodes attached to the network does not list them in `Peers`. | C | `api:mgr`, `api:wkr` | `none` |
| During a node partition, once a side declares the other side's nodes failed, its nodes remove their overlay neighbor and FDB entries for the other side's endpoints. | C | `node` | `os` |
| A node's VXLAN device does not hold a neighbor or FDB entry for any of the node's own endpoints. | C | `node` | `os` |
| During a node partition, once a side declares the other side's nodes failed, its VIPs stop balancing flows to tasks on the other side. | C | `task`, `ctr` | `os` |
| After a node partition heals, reachability between the two sides converges within the partition-heal ceiling. | B | `task`, `ctr` | `os` |
| On Linux nodes, reachability restored on heal persists. A connection opened from either side still reaches the other side's containers after they have been silent for longer than the kernel's learned-FDB aging time (300 s by default), including when they were sending as the partition healed. | B | `task`, `ctr` | `os` |
| After a node leaves the Swarm with `POST /swarm/leave` and rejoins with `POST /swarm/join`, the tasks placed on it after the rejoin, such as a global service's, are reachable within the leave-and-rejoin ceiling, from its own node and from others. | B | `task`, `ctr` | `os` |
| After a node leaves the Swarm with `POST /swarm/leave` and rejoins with `POST /swarm/join`, its published ports answer within the leave-and-rejoin ceiling. | B | `client` | `os` |
| After a node leaves the Swarm with `POST /swarm/leave` and rejoins with `POST /swarm/join`, connections to a service's published port, on any node, are delivered to the service's tasks placed on the rejoined node after the rejoin, such as a global service's, within the leave-and-rejoin ceiling. | B | `client` | `os` |
| When a service has a task running on another node throughout, within the leave-and-rejoin ceiling after a node leaves the Swarm with `POST /swarm/leave`, every new flow from another node to the service's VIP or published port reaches a running task. | B | `task`, `ctr`, `client` | `os` |
| When a node leaves the Swarm, the other nodes remove their overlay neighbor and FDB entries for its endpoints. | C | `node` | `os` |

The persistence row was revealed by a bug in which reachability after a heal depended on an entry
the kernel had learned from the peer's traffic, and broke once that entry aged out; it was fixed in
[moby/moby#53663](https://github.com/moby/moby/pull/53663). The row names `os` because how learned
and permanent FDB entries interact is the kernel's VXLAN driver behavior.

### 11.3 Churn

Every row below in which a client reaches or resolves a service holds whether the client is a
service task or a standalone container on an `attachable` overlay. It also holds both for a
client on the same node as one of the service's tasks and for a client on a node running none of
them. The service's own end is always its tasks.

| Expectation | Tier | Observed from | Depends on |
| --- | --- | --- | --- |
| During a rolling update in which at least one task keeps running throughout, as in start-first order, or fewer tasks are updated at a time than are running, the VIP does not stop answering for longer than the reassignment ceiling, and once the update completes every new flow reaches a running task within that ceiling; see [§13](#13-known-divergences--non-goals). | B | `task`, `ctr` | `os` |
| Once rapid scale up and down stops, every new flow to the VIP reaches a running task within the reassignment ceiling; see [§13](#13-known-divergences--non-goals). | B | `task`, `ctr` | `os` |
| After concurrent `POST /networks/create` and `DELETE /networks/{id}` of the same name or ID, `DELETE /networks/{id}` for any network that remains responds 204, and `GET /networks` then does not list it. | B | `api:mgr` | `none` |
| After concurrent `POST /networks/create` and `DELETE /networks/{id}` of the same name or ID, `POST /networks/create` with that name responds 201, and `GET /networks` then lists the network. | B | `api:mgr` | `none` |
| When `DELETE /networks/{id}` races `POST /containers/{id}/start` for a standalone container on the network, either the delete responds 400 and the container starts attached to the network, or the delete responds 204, `GET /networks` on a manager then does not list the network, and the start fails. | B | `api:mgr`, `api:wkr` | `none` |
| When the start fails that way, it responds 500 at once with a message that contains `network <name> not found`, or 500 after 20 seconds with `attaching to network failed, make sure your network options are correct and check manager logs: context deadline exceeded`; see [§13](#13-known-divergences--non-goals). | C | `api:wkr` | `none` |
| When `DELETE /networks/{id}` races the shutdown of the last task or container on the network on a node, either the delete responds 400 and the network stays listed, or it responds 204 and, once the shutdown completes, `GET /networks` on that node does not list the network. | B | `api:mgr`, `api:wkr` | `none` |
| When `docker network rm` races `docker run -d --network` naming the network, either `docker network rm` exits non-zero and the container runs attached to the network, or it exits zero, `docker network ls` on a manager then does not list the network, and `docker run -d` exits non-zero. | B | `api:mgr`, `api:wkr` | `cli` |
| After `POST /services/{id}/update` changes a service's endpoint mode from `vip` to `dnsrr`, `<service>` resolves to the addresses of the running tasks. | B | `task`, `ctr` | `none` |
| After a change from `vip` to `dnsrr`, the old VIP no longer reaches the service. | B | `task`, `ctr` | `os` |
| After `POST /services/{id}/update` changes a service's endpoint mode from `dnsrr` to `vip`, `<service>` resolves to the service VIP. | B | `task`, `ctr` | `none` |
| After a change from `dnsrr` to `vip`, the VIP load-balances across the service's tasks. | B | `task`, `ctr` | `os` |
| After `POST /services/{id}/update` adds a network to `TaskTemplate.Networks`, the service is reachable on the added network. | B | `task`, `ctr` | `os` |
| After `POST /services/{id}/update` removes a network from `TaskTemplate.Networks`, the service is not reachable on the removed network. | B | `task`, `ctr` | `os` |
| `docker service update --network-add` exits zero, and `GET /services/{id}` then lists the added network in `TaskTemplate.Networks`. | B | `api:mgr` | `cli` |
| `docker service update --network-rm` exits zero, and `GET /services/{id}` then does not list the removed network in `TaskTemplate.Networks`. | B | `api:mgr` | `cli` |

### 11.4 Scale and exhaustion

| Expectation | Tier | Observed from | Depends on |
| --- | --- | --- | --- |
| After many create/delete cycles of overlay networks, `POST /networks/create` that pins, with `vxlanid_list`, a VNI a deleted network used responds 201, and `GET /networks/{id}` on a manager then reports that VNI in `Options`. | B | `api:mgr` | `none` |
| After a service that publishes a port in ingress mode is removed, `POST /services/create` for another service that publishes the same port and protocol responds 201, and `GET /services/{id}` then reports that port. | B | `api:mgr` | `none` |
| Across many create/delete cycles, the VNIs the manager assigns to an overlay network created without `vxlanid_list` all differ from the VNIs in the `vxlanid_list` of every other overlay network that existed when it was created. | B | `api:mgr` | `none` |
| At least 100 overlay networks, each with a task on each of two nodes, all pass traffic between the nodes. | B | `task` | `os` |
| `POST /networks/create` for an overlay network the manager cannot allocate, such as one whose address pool is exhausted, responds 201, and `GET /networks` on a manager then lists the network; see [§13](#13-known-divergences--non-goals). | C | `api:mgr` | `none` |
| `docker network create` for such a network exits zero, and `docker network ls` on a manager then lists it. | C | `api:mgr` | `cli` |
| The network's allocation then fails, and the leader manager's daemon log reports it at error level, in an entry that contains `Failed allocation for network` and the network's ID, or, on a manager that has just become leader, `failed allocating network` and the ID. | B | `log` | `none` |
| On a manager, `GET /networks/{id}` for that network responds 200 with an empty `Driver`, `IPAM` and `Options`, without `Status`, even when the request named a driver or a subnet. The response does not report the failure in any field; see [§13](#13-known-divergences--non-goals). | C | `api:mgr` | `none` |
| After a service is added that publishes the same port number as an existing service under a different protocol, both services are reachable on their published ports. | B | `client` | `os` |
| When a service is removed, another service publishing the same port number under a different protocol stays reachable on its published port, throughout the removal and after it. | B | `client` | `os` |

### 11.5 Firewall-backend transition

Interop between nodes with different firewall backends, on cleartext and encrypted overlays and on
the ingress network, is in [§10.1](#101-all-overlay-networks). The firewall backend is a Linux
daemon setting, and this section holds on Linux nodes.

| Expectation | Tier | Observed from | Depends on |
| --- | --- | --- | --- |
| After a node's daemon restarts with the other firewall backend, without the node being drained first, the node's tasks are reachable over their overlay networks. This holds in both directions. | B | `task` | `os` |
| After a node's daemon restarts with the iptables backend in place of nftables, without the node being drained first, the node's published ports answer. | B | `client` | `os` |
| After a node's daemon restarts with the nftables backend in place of iptables, without the node being drained first, the node's published ports answer when the filter table's `FORWARD` chain has policy ACCEPT; see [§13](#13-known-divergences--non-goals). | C\* | `client` | `os` |
| Throughout a rolling backend upgrade (one node at a time: drain it, switch its backend, make it active again), a flow established through a published port on a node not being switched, to a task on a node not being switched, is not interrupted. | B | `client` | `os` |
| Throughout a rolling backend upgrade, every published port answers on the nodes not being switched. | B | `client` | `os` |

\* Tier C while the nftables backend is experimental. This expectation becomes tier B once the
nftables backend is a supported feature.

### 11.6 Failure reporting

When a node cannot program cluster-scoped configuration, such as encryption parameters for a peer, a
load-balancer backend, a firewall rule or an FDB entry, no API surface reports it. The service keeps
looking healthy from `api:mgr` while that node's data plane is wrong. The daemon log is the only
channel, which makes it part of the observable contract.

| Expectation | Tier | Observed from | Depends on |
| --- | --- | --- | --- |
| A node-local failure to program a load-balancer service or backend is logged at error level; see [§13](#13-known-divergences--non-goals). | B | `log` | `none` |
| On Linux nodes, a node-local failure to publish an ingress port is logged at error level, in an entry that contains `Failed to add ingress`. When the cause is a host port already in use, the entry also contains that port number. | B | `log` | `none` |
| A node-local failure to program an overlay peer's FDB or neighbor entry is logged at warning level, in an entry that names the network and the peer's address. | B | `log` | `none` |
| On Linux nodes, a node-local failure to program an IPsec security association or policy for a peer is logged at warning level; see [§13](#13-known-divergences--non-goals). | C | `log` | `none` |
| A further attempt is reported again, not suppressed after the first. | B | `log` | `none` |
| After the condition clears, a backend whose programming starts afterwards receives traffic from the node; see [§13](#13-known-divergences--non-goals). | B | `task`, `client` | `none` |
| After the condition clears, programming that starts afterwards, such as a backend added then, is not reported as a failure. | B | `log` | `none` |
| While that node's data plane is wrong, `GET /tasks` reports the node's tasks as `running`, and, at API version 1.41 or later, `GET /services?status=true` reports the service's `RunningTasks` equal to its `DesiredTasks`. | C | `api:mgr` | `none` |
| `docker service ps --no-trunc` lists the tasks `GET /tasks` returns for the service, with their states and errors. | B | `api:mgr` | `cli` |
| `docker service ls` prints the service's replicas as all running. | C | `api:mgr` | `cli` |

Four gaps in this reporting and recovery are known divergences, listed in
[§13](#13-known-divergences--non-goals): an unchanged condition is not reported again, recovery is
not reported at all, no entry names the service, and recovery is incomplete.

## 12. Firewall anchors & host integration

The content of Moby's firewall rules is incidental: it is exactly what a backend change replaces.
This section covers the overlay driver's own firewall state, and how Swarm traffic meets the bridge
driver's firewall anchors. Those anchors, `DOCKER-USER` among them, are specified in
[bridge-networking.md](bridge-networking.md). This section holds on Linux nodes.

### 12.1 nftables

The overlay driver has one anchor: the hooks and priorities of the base chains in the nftables
table it uses for encrypted networks. They decide evaluation order against everything else on
the host, such as firewalld, kube-proxy or an operator's own ruleset. The names of that table and
its chains are not anchors: an operator's rules can only reach them by being programmed into
Moby's own table, which is not a supported configuration.

| Expectation | Tier | Observed from | Depends on |
| --- | --- | --- | --- |
| The overlay driver's nftables base chains are on the input hook at raw priority (−300) and on the output hook at mangle priority (−150). | C\* | `node` | `none` |

\* Tier C while the nftables backend is experimental. These expectations become tier B once the
nftables backend is a supported feature.

### 12.2 iptables

The `DOCKER-USER` chain itself is specified in
[bridge-networking.md](bridge-networking.md#12-docker-user). Swarm ingress ports do not have a chain
of their own. The bridge driver's anchors cover them too because they are published through the
bridge driver like any other published port.

| Expectation | Tier | Observed from | Depends on |
| --- | --- | --- | --- |
| An operator's `DOCKER-USER` rules apply to Swarm published ports, in ingress mode and `mode=host`, as they do to any other published port. | B | `client` | `os` |
| An operator's rule accepting incoming VXLAN traffic, already in the filter table's `INPUT` chain when an encrypted network is programmed on the node, does not let cleartext VXLAN for that network's VNI through. | B | `underlay` → `task` or `ctr` | `os` |

### 12.3 Cross-generation cleanup

This is the upgrade path a packager exercises: a node running release *N−1* is upgraded to *N*,
possibly with a different firewall backend. The daemon-wide expectations, and why cleanup depends on
names, are in [bridge-networking.md](bridge-networking.md#13-cross-generation-cleanup); the rows
here cover state that belongs to Swarm and the overlay driver.

| Expectation | Tier | Observed from | Depends on |
| --- | --- | --- | --- |
| After a switch of firewall backend, in either direction, while a VNI is programmed on the node, the node does not have the firewall state for overlay encryption that a previous generation left for that VNI: rules for it in filter `INPUT` or mangle `OUTPUT` after a switch to nftables, or the `docker-overlay` table after a switch to iptables; see [§13](#13-known-divergences--non-goals). | B | `node` | `os` |
| This holds across a package upgrade from the previous release as well as a same-binary restart. | B | `node` | `os`, `upgrade` |
| Once the daemon has started, the filter and nat tables do not contain a `DOCKER-INGRESS` chain left by a daemon from before Swarm ingress moved onto bridge port publishing. | B | `node` | `os` |
| After a switch from iptables to nftables, the node removes a previous generation's encryption rules for a VNI from filter `INPUT` and mangle `OUTPUT` when it creates a cleartext overlay network with that VNI, or when it first attaches an endpoint to an encrypted one. | C | `node` | `os` |
| After a switch from nftables to iptables, the daemon deletes the `docker-overlay` table the first time after it starts that the node creates or deletes an overlay network or creates an overlay endpoint; see [§13](#13-known-divergences--non-goals). | C | `node` | `os` |
| The daemon removes a `DOCKER-INGRESS` chain when it starts: at every start under the iptables backend, and at the first start after a switch to nftables. | C | `node` | `os` |
| The names cleanup relies on are unchanged: the overlay driver's nftables table, `docker-overlay`, and the `DOCKER-INGRESS` chain, which cleanup looks for in the filter and nat tables. | C | `node` | `none` |

## 13. Known divergences & non-goals

Each row is either a current behavior that differs from what a user might expect, or a
capability Moby does not provide. Changing any of them is a behavior change, to be made
deliberately.

| Topic | Current behavior | Depends on |
| --- | --- | --- |
| IPv6 addressing inside the overlay | Unsupported. With the default IPAM driver, an overlay network with an IPv6 subnet is accepted but never allocated. `--ipv6` without an IPv6 subnet is accepted, and the network's attachments get IPv4 addresses only. Exactly one address is allocated per attachment, and the service mesh is IPv4-only. | `none` |
| MTU over an IPv6 underlay | The 50-byte encapsulation allowance assumes a 20-byte outer IPv4 header. An IPv6 one is 40, which makes the real overhead 70: the advertised inner MTU overshoots by exactly 20, with 1450 advertised and 1430 achievable on a 1500-byte underlay. The symptom is `EMSGSIZE` at the sender for don't-fragment traffic rather than a silent blackhole. So [§3](#3-observable-state-inside-a-container)'s "the reported MTU is exactly the largest payload that crosses the overlay between nodes unfragmented" does not hold over an IPv6 underlay. | `os` |
| Outer source address of VXLAN traffic | Not guaranteed. The outer source address is whatever the kernel's route selection picks for traffic to each peer, and is meant to be the node's data-path address. When the kernel picks another address, peers drop all of the node's overlay traffic, and neither side logs an error ([§1](#1-cluster--node-configuration), [moby/moby#53004](https://github.com/moby/moby/issues/53004)). | `none` |
| VXLAN on cleartext overlays | Not authenticated. A Linux node delivers VXLAN for a cleartext network's VNI from any host that can reach its data-path port, and the frames reach the network's containers. For an encrypted network it drops VXLAN for the network's VNI that did not arrive under IPsec ([§9](#9-encryption)). | `os` |
| Overlay namespaces after a killed daemon | Not reclaimed at start. On Linux, the namespace of an overlay network that the node does not program again stays, with its bridge, VXLAN device and bind mount under the exec root, until the host reboots or another network with one of its VNIs is programmed on the node ([§11.1](#111-restart-and-reclamation)). | `none` |
| Encrypted overlay on Windows | Not implemented. Traffic between Windows nodes on an `encrypted` overlay is sent unencrypted. Traffic between Windows and Linux nodes on it does not pass, because Linux peers drop cleartext VXLAN for that VNI. A Windows node's endpoint MTU on an `encrypted` network is the same as on a cleartext one ([§9](#9-encryption), [§10.3](#103-encrypted-overlay-networks)). | `none` |
| iptables `FORWARD` policy after a switch to nftables | Left in place. If the filter table's `FORWARD` chain has policy DROP, as the daemon sets it under the iptables backend, then after the daemon switches to nftables the node drops traffic it routes to and from containers, such as traffic to published ports and containers' egress, unless an operator's rule accepts it. The daemon logs a warning and leaves the policy to the operator ([§11.5](#115-firewall-backend-transition), [bridge-networking.md §1.3](bridge-networking.md#13-cross-generation-cleanup)). | `os` |
| Overlay encryption cleanup after a backend switch | After a switch from nftables to iptables, the daemon deletes the `docker-overlay` table once, the first time after it starts that the node creates or deletes an overlay network or creates an overlay endpoint. If the node's data-path address is not known yet at that point, the daemon logs an error that contains `Deleting overlay encryption nftables rules` and leaves the table in place until it next restarts. After a switch from iptables to nftables, the encryption rules a previous generation left for a VNI stay until the node programs that VNI again ([§12.3](#123-cross-generation-cleanup)). | `none` |
| Data-path address on Windows | Not applied. A Windows node's tunnel endpoint is its network adapter's address, whatever `--data-path-addr` or the advertise address says. | `none` |
| Data-path port on Windows | Not applied. Windows nodes do not use the cluster's data-path port, and in a cluster with a non-default port they cannot exchange overlay traffic with Linux nodes. | `none` |
| IPv6 data path on Windows | Not supported. Windows nodes cannot exchange overlay traffic with Linux nodes over an IPv6 underlay. | `none` |
| `mtu` option on Windows | Ignored. On a network with `com.docker.network.driver.mtu` set, Windows nodes do not report the inner MTU that Linux nodes report, and traffic between them can be blackholed ([§3](#3-observable-state-inside-a-container)). | `none` |
| `ifname` option on Windows | Ignored. A Windows container's interface name does not follow `com.docker.network.endpoint.ifname`. | `none` |
| Default route on `internal` overlays on Windows | A Windows container attached only to `internal` overlays has a default route. | `none` |
| Egress from `internal` overlays on Windows | Not prevented. A Windows container attached only to `internal` overlays has egress ([§8](#8-egress--the-gateway-network)). | `none` |
| Egress with both `internal` and non-internal overlays on Windows | Not guaranteed to leave through the non-internal network. A Windows container attached to both has a gateway on each, and its egress can leave through either ([§8](#8-egress--the-gateway-network)). | `none` |
| SCTP on Windows | Not supported. Windows does not have SCTP, and SCTP does not pass to or from containers on Windows nodes, directly or through a VIP ([§4](#4-east-west-reachability), [§6](#6-load-balancing-east-west-vip)). A task on a Windows node that publishes an SCTP port in `mode=host` fails with an error that contains `SCTP is unsupported on windows`. On a Windows node, a port published as SCTP in `mode=ingress` is programmed as TCP, where it collides with a TCP port of the same number ([§7](#7-published-ports), [§11.4](#114-scale-and-exhaustion)). | `os` |
| `dsr` option on Windows | Ignored. Windows nodes load-balance VIP traffic in NAT mode whatever the network's `dsr` option says. A task on a Windows node observes neither the client's real source address nor the VIP ([§6](#6-load-balancing-east-west-vip)). | `none` |
| Isolation between tasks on the ingress network on Windows | Not enforced. A task on a Windows node is reachable from another task via its ingress-network address ([§7.2](#72-modeingress)). | `none` |
| Egress through the ingress link on Windows | Not prevented. A task on a Windows node can reach the outside world through its ingress-network link ([§7.2](#72-modeingress)). | `none` |
| Orphaned HNS networks on Windows | Not reclaimed. When the daemon starts, it adopts every HNS overlay network on the node. It deletes one only when a network with the same ID, or with an overlapping VNI, is created again. The HNS network of a Swarm network that the node stopped using while the daemon was down is left in place ([§11.1](#111-restart-and-reclamation)). The daemon does not enumerate the HNS endpoints on those networks either. | `none` |
| Resolver network preference on Windows | Not applied. Each of a Windows container's networks has its own embedded resolver, which answers only from that network's records. Which network's answer the container gets depends on which adapter's resolver the Windows DNS client asks ([§5](#5-service-discovery)). | `os` |
| Ingress port reservation on Windows | Not reserved. Windows nodes program ingress ports as HNS load-balancer policies without reserving the host port. The daemon does not refuse a `docker run -p` on a port a service publishes, and does not refuse to publish a port that is already in use ([§7.2.1](#721-port-allocation)). | `none` |
| Rootless mode | Not supported, and nothing refuses it. `docker swarm init` and `docker network create --driver overlay` succeed. Tasks of a service on a user-defined overlay network are rejected with a permission error, and a standalone container attached to such a network fails to start after a timeout. A service that publishes a port through the routing mesh reports its task running, but the port does not answer. A node cannot join a Swarm whose manager runs rootless: its connection to the manager is refused. | `os` |
| Network allocation failures are not surfaced to the caller | `POST /networks/create` responds 201 for an overlay network the manager cannot allocate: one whose pinned VNI is in use, whose address pool is exhausted, whose requested subnet overlaps another Swarm network's, or that has an IPv6 subnet under the default IPAM driver. The manager allocates networks asynchronously. When it cannot, it logs the failure and retries later, for example after another network, service or task is removed. The caller is never told. On a manager, `GET /networks/{id}` reports the network with an empty `Driver` and `IPAM`, even when the request named them ([§2.3](#23-overlay-networks), [§11.4](#114-scale-and-exhaustion)). | `none` |
| Network status by full ID on a manager | Node-local. On a manager with a local attachment to an overlay network, `GET /networks/{id}` with the network's full ID reports, in `Status.IPAM`, only that node's own endpoints. With the network's name or a partial ID, it reports the manager's allocation for the whole network ([§2.3](#23-overlay-networks)). | `none` |
| Published-port conflicts accepted in a race | Multiple services with colliding published ports may erroneously be accepted due to a race condition ([§7.2.1](#721-port-allocation)). | `none` |
| Auto-assigned published ports across an update | Reassigned. An update that adds or removes a published port, in either publish mode, reassigns each of the service's other auto-assigned ports, usually to a different number, which clients then have to rediscover ([§7.2.1](#721-port-allocation)). | `none` |
| Auto-assigned ports and `mode=host` ports | Not kept apart. The manager auto-assigns ingress-mode ports without regard to the ports other services publish in `mode=host`, and `POST /services/create` responds 201 when it assigns one of them. The refusal of a port that both modes publish applies only to a port the request names. On a node where both are published, the routing mesh does not serve the service's ports, or the `mode=host` task fails to start, depending on which binds first ([§7.2.1](#721-port-allocation)). | `none` |
| Flows to a stopping task from other nodes | Not drained reliably. A task leaves load balancing on its own node at once and is stopped 2 s later. A node that has not learned of the withdrawal by then keeps sending new flows to the task, through the VIP and through published ports, after it has stopped, and those flows fail until it learns of it ([§6.2](#62-health-and-drain), [§11.3](#113-churn)). | `none` |
| Published ports above 65535 | Accepted. `POST /services/create` responds 201 for a `PublishedPort` above 65535, and the manager never allocates the service ([§7.2.1](#721-port-allocation)). | `none` |
| Node-local failures are not reported again without a new attempt | A node reports a failure to program cluster-scoped configuration only when something triggers a new attempt, such as a change to the service or its tasks. With nothing to trigger one, a node whose data plane has been wrong for an hour has said nothing since its first reports, and an operator who was not watching then cannot discover the condition. | `none` |
| Recovery from a node-local failure is not reported | At the default log level, a successful attempt does not log anything. An operator sees the failures stop and nothing else, with no confirmation that the node's data plane is correct again. | `none` |
| Failure log entries do not name the service | On Linux nodes, load-balancer failure entries name the network, the VIP or a backend's address, and some name only the sandbox. No entry names the service. An entry for a failure to program an IPsec security association prints `%!s(PANIC=String method: runtime error: invalid memory address or nil pointer dereference)` in place of the association, and does not name the peer ([§11.6](#116-failure-reporting)). | `none` |
| Recovery from a node-local failure is incomplete | On Linux nodes, the next attempt programs only the backend that triggered it. A backend whose programming failed, including every backend that arrived while the failure lasted, does not get traffic through that node's load balancer until its task is replaced. When the failure comes after the service's ingress ports are published, the next attempt takes a second reference to them. The host ports then stay published after the service is removed or stops publishing them, until the daemon restarts ([§7](#7-published-ports), [§11.6](#116-failure-reporting)). | `none` |
| HTTP 500 for refused requests | These requests respond 500, the status for a fault in the daemon, although the daemon refuses them on purpose: `POST /swarm/init` with a data-path port outside the accepted range; `POST /swarm/init` with a `SubnetSize` greater than 29, or less than the prefix length of one of its `DefaultAddrPool` prefixes; `POST /swarm/init` and `POST /swarm/join` with `--live-restore`, or with the nftables backend and iptables enabled, without `features.swarm-nftables`; `POST /containers/{id}/start` for a container on a non-attachable Swarm-scoped network; `POST /containers/{id}/start` for a container that publishes a port and protocol a service publishes in ingress mode; `POST /containers/{id}/start` for a container whose network was deleted before the manager looked it up; and `POST /containers/{id}/start` and `POST /networks/{id}/connect` for an endpoint whose `com.docker.network.endpoint.ifname` the OS refuses, or that another of the container's endpoints already uses. Some other invalid `swarm init` arguments, such as an invalid listen address, respond 400 ([§1](#1-cluster--node-configuration), [§2.2](#22-swarm-scoped-networks), [§3](#3-observable-state-inside-a-container), [§7.2.1](#721-port-allocation), [§11.3](#113-churn)). | `none` |
| Error for an overlay network created on a worker | Misleading. On a worker, the error from `network create --driver overlay` says `This node is not a swarm manager. Use "docker swarm init" or "docker swarm join" to connect this node to swarm and try again.`, which tells a node that is already in a Swarm to join one ([§2.2](#22-swarm-scoped-networks)). | `none` |
| `--config-from` on overlay networks | Not usable. `POST /networks/create` for an overlay network with `ConfigFrom` succeeds only if the config-only network exists on the manager that handles the request, and responds 404 otherwise. Every attachment to the network then fails. A service's tasks fail with `user-specified configurations are not supported if the network depends on a configuration network`, and `docker run` fails after 20 seconds with `attaching to network failed, make sure your network options are correct and check manager logs: context deadline exceeded` ([§2.2](#22-swarm-scoped-networks)). | `none` |
| Attachment failures for standalone containers time out | When a standalone container's attachment to an attachable network fails, `POST /containers/{id}/start` waits 20 seconds and responds 500 with `attaching to network failed, make sure your network options are correct and check manager logs: context deadline exceeded`. This happens on an `encrypted` network on a host without kernel XFRM support, on an overlay network created with `ConfigFrom`, when the network is deleted between the manager's lookup of the network and its creation of the attachment task, and when the container's static address is in use or outside the network's subnets. In the first two cases, the reason appears in the attachment task only while the start waits: `GET /tasks` with the `runtime=attachment` filter lists it, rejected, until the daemon gives up and deletes it before responding. Afterwards the reason is only in the node's daemon log. In the third, the attachment task is deleted, and the manager does not log anything. In the fourth, the manager never allocates the attachment task, and the reason appears only in the leader manager's log, in an entry that contains `task allocation failure` ([§2.2](#22-swarm-scoped-networks), [§3](#3-observable-state-inside-a-container), [§9](#9-encryption), [§11.3](#113-churn)). | `none` |
| IPv6-only Swarm-scoped networks of node-local drivers | Refused. `POST /networks/create` for any Swarm-scoped network with IPv4 disabled responds 400 with `IPv4 cannot be disabled in a Swarm scoped network`, whatever the driver. Only `overlay` needs the refusal ([§2.2](#22-swarm-scoped-networks), [§2.3](#23-overlay-networks)). | `none` |
| Subnets and options on Swarm-scoped networks of node-local drivers | Ignored. `POST /networks/create` accepts a valid `IPAM.Config` and `Options` for a Swarm-scoped network of a node-local driver, but the manager does not keep them, and each node creates the network from its own default address pools and the driver's defaults. `ConfigFrom`, naming a config-only network, configures such a network on each node ([§2.2](#22-swarm-scoped-networks)). | `none` |
| `mtu` on Swarm networks is not validated at create | Accepted. The manager does not parse `com.docker.network.driver.mtu`, and `POST /networks/create` responds 201 for any value. A Linux node rejects every task on the network that lands on it when the value is not a non-negative integer, with `failed to parse` or `invalid MTU value` in the task's error. When the kernel cannot create the network's interfaces with the value, every such task fails at start. Windows nodes ignore the option ([§2.3](#23-overlay-networks)). | `none` |
| `ifname` is not validated before a task starts | Accepted. `POST /services/create` and `POST /services/{id}/update` accept any interface name in a network attachment's driver options. Every task whose interface name the OS refuses fails at start. Under the default restart policy, it is replaced indefinitely ([§3](#3-observable-state-inside-a-container)). | `none` |
| Interface left behind after a failed connect | When `POST /networks/{id}/connect` to a running container fails at the rename, the interface stays in the container under its generated `veth` name, down and without addresses, and its host-side peer stays attached, until the container's network namespace is destroyed. A failed connect is meant to leave nothing behind ([§3](#3-observable-state-inside-a-container)). | `none` |
| Interface names the kernel rewrites | Accepted without an error. For a name containing `%d`, the kernel gives the interface the first free name that matches, such as `eth0`. The daemon keeps the requested name. A later disconnect responds 200 but leaves the interface in the container, up and with its address ([§3](#3-observable-state-inside-a-container)). | `none` |
| Interface-name collisions depend on attach order | When one endpoint asks for `eth<N>` and another's name is generated, the container starts or fails depending on the order its endpoints are attached in, which is random. The endpoint that asks for the name is meant to get it, whatever the order ([§3](#3-observable-state-inside-a-container)). | `none` |

## Deliberately not asserted

These are incidental to the implementation, and a change to any of them should not cost anyone a
test fix.

- The content of Moby's own chains and tables, and their names other than `DOCKER-USER` except as
  [§12.3](#123-cross-generation-cleanup) pins them for cleanup: individual match/target lines, their
  ordering within a chain, counters, and `iptables-save` / `nft list ruleset` diffs. The rules are
  exactly what a backend change replaces; the surface is their packet-level effect. The overlay
  driver's base-chain hooks and priorities are the exception; see
  [§12](#12-firewall-anchors--host-integration).
- Which packet-filtering technology is in use, except where
  [§12](#12-firewall-anchors--host-integration) pins a backend-specific anchor.
- The existence, name or topology of internal sandboxes (per-network load-balancer sandbox, ingress
  sandbox), network-namespace paths, and generated `veth` / bridge / VXLAN interface names. A node's
  load-balancer endpoint on a network, as `GET /networks/{id}` on that node lists it with the
  address [§6.1](#61-distribution) pins, is the exception; its key and name are not asserted.
- The load-balancer implementation and its scheduler, beyond the distribution property in
  [§6.1](#61-distribution).
- Internal refcounting, gossip timers, reap and rejoin intervals. The convergence bounds in
  [§11.2](#112-convergence) are the surface instead.
- The order in which subnets are allocated from an address pool, and addresses within a subnet.
- The value of `LocalLBIndex`, which `network inspect --verbose` exposes but which is an internal
  firewall mark.
- Host-port reservation mechanics behind [§7.2.1](#721-port-allocation).
- Whether Windows nodes can use a VNI below 4096.
