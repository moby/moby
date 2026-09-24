Bridge networking surface area
==============================

Scope: the bridge network driver, and the parts of the daemon's firewall integration that are not
specific to Swarm. This spec describes them as observable from outside the daemon. Every
expectation here holds on Linux nodes.

This spec is a stub. It holds expectations that belong to the bridge driver. They come from the
work on the [Swarm networking surface area](swarm-networking.md) and from reviews of the specs'
coverage. This spec is not yet a complete account of the bridge driver. [README.md](README.md)
gives its conventions for tiers, vantage points and dependencies.
[network-common.md](network-common.md) covers what every network driver must do, such as naming an
endpoint's interface.

## 1. Firewall anchors & host integration

The content of Moby's firewall rules is not an interface. A backend change replaces exactly this
content. [§1.5](#15-rule-content) pins it at tier C only. Two things are interfaces. They are the
anchors of this section:

1. The `DOCKER-USER` iptables chain. Operators attach rules to it. The documentation names it as
   the place to put policy that survives daemon restarts. The daemon never deletes or modifies its
   contents.
2. The hooks and priorities of the bridge driver's nftables base chains. They decide the evaluation
   order relative to everything else on the host, such as firewalld, kube-proxy or an operator's
   own ruleset. A changed priority silently changes who sees a packet first.

No other name is an anchor. Moby's other iptables chains are not anchors, and no name in nftables
is an anchor. An operator's rules can reach Moby's nftables chains only if they are programmed into
Moby's own tables. That is not a supported configuration.

IPv4 is an enabled IP version while the daemon's `iptables` option is true. IPv6 is an enabled IP
version while the daemon's `ip6tables` option is true. This applies under either firewall backend,
and whether or not `ipv6` is set.

### 1.1 Base-chain priorities

These rows cover the host's network namespace only. The specs do not assert the rules that the
daemon programs in a container's network namespace
([container-dns.md](container-dns.md#deliberately-not-asserted)).

| Applies to | Expectation | Tier | Observed from | Depends on |
| --- | --- | --- | --- | --- |
| | The bridge driver's nftables base chains are on the prerouting hook at raw priority (−300) and at dstnat priority (−100). They are also on the output hook at dstnat priority (−100) and on the forward hook at filter priority (0). They are also on the postrouting hook at srcnat priority (100). | C\* | `node` | `none` |

\* These expectations are tier C while the nftables backend is experimental. They become tier B
when the nftables backend is a supported feature.

### 1.2 `DOCKER-USER`

`DOCKER-USER` belongs to the iptables backend. The rows in this section hold under the iptables
backend.

| Applies to | Expectation | Tier | Observed from | Depends on |
| --- | --- | --- | --- | --- |
| | For each enabled IP version, `DOCKER-USER` exists in the filter table and `FORWARD` jumps to it. | B | `node` | `os` |
| | A DROP rule that an operator places in `DOCKER-USER` drops matching traffic to a published port. This is also true for a rule that an operator places after a firewalld reload. | B | `client` | `os` |
| | `DOCKER-USER` rules that an operator added before a start of the daemon are still there, unchanged, after the start. This includes a restart with `iptables` false. | B | `node`, `client` | `none` |
| | After a firewalld reload, for each enabled IP version, `DOCKER-USER` exists and `FORWARD` jumps to it. | B | `node` | `os` |

### 1.3 Cross-generation cleanup

This section is about a node that runs release *N−1* and is then upgraded to *N*. After the upgrade,
the node can also use a different firewall backend. A generation is the firewall state that one
release creates under one firewall backend. A previous generation is such state that a daemon left
on the node before the current start of the daemon. After the upgrade, the node must not have two
generations of live rules. The rows in this section hold for each enabled IP version. The built-in
chains are the chains that iptables itself provides in each table. Examples are the filter table's
`FORWARD` chain and the nat table's `PREROUTING` chain.

Cleanup finds the chains and tables of a previous generation by their names. Thus the daemon itself
needs these names, but they are not an interface for operators. A rename thus needs a cleanup path
for the old name. Without one, an upgraded node keeps the rules of the previous generation.

In this section, the daemon restores a network, endpoint or port binding when it sets it up again
during its start. It restores each bridge network that exists at the start. With `live-restore`
true, it also restores the endpoints and port bindings of the containers that are still running.

A test that runs a single binary never sees the names of the previous release. The C row that pins
the names that cleanup needs makes a rename visible.

| Applies to | Expectation | Tier | Observed from | Depends on |
| --- | --- | --- | --- | --- |
| | After the daemon has started, the host netns does not contain any table or chain that a previous generation left there with the other backend. The exceptions are `DOCKER-USER` and the `FORWARD` jump to it, which hold the operator's rules and stay in place. The overlay driver's `docker-overlay` table is also an exception, and [swarm-networking.md §12.3](swarm-networking.md#123-cross-generation-cleanup) covers it. This holds for a switch from iptables to nftables and for a switch from nftables to iptables. It holds across a package upgrade from *N−1* to *N* and across a same-binary restart. | B | `node` | `os`, `upgrade` |
| A chain or table that a release older than *N−1* created and that *N* does not create, such as `DOCKER-ISOLATION`, beside the chains of *N−1* | After the daemon has started, that chain or table does not remain. `DOCKER-INGRESS` is an exception, and [swarm-networking.md §12.3](swarm-networking.md#123-cross-generation-cleanup) covers it. | B | `node` | `os` |
| A rule that the previous generation left in a built-in chain for a network, endpoint or port binding | After a switch from iptables to nftables, the rule does not remain after the daemon has restored that network, endpoint or port binding. This holds across a package upgrade from the previous release and across a same-binary restart. See [§4](#4-known-divergences--non-goals). | B | `node` | `os`, `upgrade` |
| | When the daemon starts with the iptables backend, no `docker-bridges` table remains after the start. | C | `node` | `os` |
| A jump from the filter table's `FORWARD` chain to `DOCKER-FORWARD` or to `DOCKER-ISOLATION-STAGE-1` | After the daemon starts with the nftables backend, some chains do not remain. These chains are `DOCKER` (in the filter and nat tables), `DOCKER-FORWARD`, `DOCKER-BRIDGE`, `DOCKER-CT`, `DOCKER-INTERNAL`, `DOCKER-ISOLATION-STAGE-1`, `DOCKER-ISOLATION-STAGE-2` and `DOCKER-ISOLATION`. The jumps to these chains from `FORWARD` and from the nat table's `PREROUTING` and `OUTPUT` do not remain either. | C | `node` | `os` |
| The rules of the previous generation in built-in chains for a network, endpoint or port binding | After a switch from iptables to nftables, these rules remain until that network, endpoint or port binding is restored. They do not remain after that. | C | `node` | `os` |
| | The names that cleanup needs are unchanged. They are the nftables table `docker-bridges`, and the iptables chains `DOCKER` (in the filter and nat tables), `DOCKER-FORWARD`, `DOCKER-BRIDGE`, `DOCKER-CT` and `DOCKER-INTERNAL`. | C | `node` | `none` |
| | The policy of the iptables filter table's `FORWARD` chain is not modified when the daemon starts with the nftables backend. This includes a policy of DROP. | C\* | `node` | `none` |
| The filter table's `FORWARD` chain with policy DROP | On the first start after a switch from iptables to nftables, the daemon logs a warning that contains `Network traffic for published ports may be dropped, iptables chain FORWARD has policy DROP.` | C\* | `log` | `none` |
| | After a switch from iptables to nftables, a DROP rule that an operator placed in `DOCKER-USER` still drops the forwarded traffic that it matches. This includes traffic to published ports. | C\* | `client` | `os` |

\* These expectations are tier C while the nftables backend is experimental. They become tier B
when the nftables backend is a supported feature.

### 1.4 Firewall backend and firewalld

| Applies to | Expectation | Tier | Observed from | Depends on |
| --- | --- | --- | --- | --- |
| | The daemon does not start with a `firewall-backend` other than `iptables` or `nftables`, and prints an error that contains `invalid firewall-backend`. | B | `log` | `none` |
| A node with firewalld running | A published port answers after a firewalld reload. | B | `client` | `os` |
| The iptables backend, with `ip-forward` and `iptables` true and `ip-forward-no-drop` false, and a host where `net.ipv4.ip_forward` is 0 | After the daemon has created a bridge network with IPv4, `net.ipv4.ip_forward` is 1. The filter table's `FORWARD` chain also has policy DROP. | B | `node` | `os` |
| The iptables backend, with `ip-forward` and `ip6tables` true and `ip-forward-no-drop` false, and a host where `net.ipv6.conf.all.forwarding` or `net.ipv6.conf.default.forwarding` is 0 | After the daemon has created a bridge network with IPv6, both are 1. The ip6tables filter table's `FORWARD` chain also has policy DROP. | B | `node` | `os` |
| The nftables backend with `ip-forward` true, a host where `net.ipv4.ip_forward` is 0, the default bridge enabled, and no containers that `live-restore` keeps running | The daemon does not start. It prints an error that contains `IPv4 forwarding is disabled`. | C\* | `log` | `none` |

\* This expectation is tier C while the nftables backend is experimental. It becomes tier B when
the nftables backend is a supported feature.

### 1.5 Rule content

The rows in this subsection pin the content of the bridge driver's rules, so that a change to it is
visible in review. A deliberate change to this content is not a breakage.

| Applies to | Expectation | Tier | Observed from | Depends on |
| --- | --- | --- | --- | --- |
| | For each configuration that the repository keeps a reference iptables ruleset for, the bridge driver's IPv4 iptables rules match that reference. The reference gives the matches and target of each rule, the order of the rules in each chain, and the names of the chains. The configurations include a Swarm service that publishes a port. | C | `node` | `none` |
| | For each configuration that the repository keeps a reference nftables ruleset for, the bridge driver's IPv4 nftables ruleset matches that reference. The reference gives the matches and verdict of each rule, the order of the rules in each chain, and the table and chain names. | C | `node` | `none` |

## 2. Network and endpoint options

| Applies to | Expectation | Tier | Observed from | Depends on |
| --- | --- | --- | --- | --- |
| | `POST /networks/create` for a bridge network whose `com.docker.network.driver.mtu` is not an integer from 0 to 2⁶³ − 1 responds 400. The message includes the value. | B | `api` | `none` |
| | `docker network create --driver bridge` with a `com.docker.network.driver.mtu` option that is not an integer from 0 to 2⁶³ − 1 exits non-zero. It prints an error that includes the value. | B | `api` | `cli` |
| | `POST /networks/create` for a bridge network with an MTU that the kernel refuses responds with a 4xx or 5xx status. `GET /networks` then does not list the network. | B | `api` | `none` |
| | `POST /networks/create` for a bridge network with an MTU that the kernel refuses responds 500. See [§4](#4-known-divergences--non-goals). | C | `api` | `none` |
| | `docker network create --driver bridge` with an MTU that the kernel refuses exits non-zero. `docker network ls` then does not list the network. | B | `api` | `cli` |
| A bridge network whose `com.docker.network.driver.mtu` is positive | The interfaces of the network's containers have that MTU. See [§4](#4-known-divergences--non-goals). | B | `ctr` | `os` |
| | A bridge network's `com.docker.network.driver.mtu` of 0 leaves the MTU of the bridge and of its containers' interfaces as the kernel set it. | C | `node`, `ctr` | `none` |
| | The daemon does not start with a negative daemon-wide `mtu`, and prints an error that contains `invalid default MTU`. | B | `log` | `none` |
| A daemon configured with an `mtu` that the kernel cannot apply to the default bridge, and without containers that `live-restore` keeps running | The daemon does not start. See [§4](#4-known-divergences--non-goals). | B | `log` | `none` |
| A daemon configured with an `mtu` that the kernel cannot apply to the default bridge, with containers that `live-restore` keeps running | The daemon starts. The default bridge keeps its MTU. | B | `log`, `node` | `none` |
| A daemon configured with a positive `mtu` | The interfaces of containers on the default bridge have that MTU. See [§4](#4-known-divergences--non-goals). | B | `ctr` | `os` |
| A daemon configured with an `mtu` of 0 | The default bridge and its containers' interfaces keep the MTU that the kernel gave them. | C | `node`, `ctr` | `none` |
| A daemon configured with an `mtu` from 2³² to 2⁶³ − 1 whose remainder modulo 2³² the kernel accepts | The daemon starts. The default bridge gets that remainder as its MTU. For example, 4294968796 gives an MTU of 1500. See [§4](#4-known-divergences--non-goals). | C | `log`, `node` | `none` |

## 3. Data path

### 3.1 Isolation

| Applies to | Expectation | Tier | Observed from | Depends on |
| --- | --- | --- | --- | --- |
| A daemon with `allow-direct-routing` false, and a target network whose `com.docker.network.bridge.gateway_mode_ipv4` is `nat` or not set | A container on one bridge network cannot reach a container on another bridge network at that container's address, on any port. It reaches the published ports of that container at the node's address. | B | `ctr` → `ctr` | `os` |
| A bridge network with `com.docker.network.bridge.enable_icc` set to `false` | A container cannot reach another container on the network at that container's address, on any port. It reaches the published ports of that container at the node's address. | B | `ctr` → `ctr` | `os` |
| | Traffic between a container on an `internal` bridge network and an address outside the network's subnets does not pass through the node, in either direction. This holds even when the container has a default route through the bridge. | B | `ctr`, `underlay` | `os` |
| An `internal` bridge network whose `com.docker.network.bridge.gateway_mode_ipv4` is not `isolated`, and without `com.docker.network.bridge.inhibit_ipv4` | A process in the node's host network namespace reaches a container on the network at the container's IPv4 address. | B | `node` | `os` |
| An `internal` bridge network whose `com.docker.network.bridge.gateway_mode_ipv4` is not `isolated`, and without `com.docker.network.bridge.inhibit_ipv4` | A container on the network reaches a process in the node's host network namespace that listens on the bridge's IPv4 address. | B | `ctr` | `os` |
| An `internal` bridge network with `com.docker.network.bridge.gateway_mode_ipv4` set to `isolated` | The bridge does not have an IPv4 address. A process in the node's host network namespace cannot reach the network's containers at their IPv4 addresses. | B | `node` | `os` |
| A daemon with `allow-direct-routing` false, a container on a bridge network whose `com.docker.network.bridge.gateway_mode_ipv4` is `nat` or not set, and an underlay host with a route to the container's address via the node's underlay address | Traffic from that host to the container's address arrives on an interface that is not in the network's `com.docker.network.bridge.trusted_host_interfaces`. The traffic does not reach the container on any port. | B | `underlay` → `ctr` | `os` |
| A daemon with `allow-direct-routing` true, a container on a bridge network whose `com.docker.network.bridge.gateway_mode_ipv4` is `nat` or not set, and an underlay host with a route to the container's address via the node's underlay address | Traffic from that host to the container's address reaches the container's published ports only. | B | `underlay` → `ctr` | `os` |
| A container on a bridge network whose `com.docker.network.bridge.gateway_mode_ipv4` is `nat` or not set, and an underlay host with a route to the container's address via the node's underlay address | When traffic from that host to the container's address arrives on an interface in the network's `com.docker.network.bridge.trusted_host_interfaces`, it reaches the container's published ports only. | B | `underlay` → `ctr` | `os` |
| A bridge network whose `com.docker.network.bridge.trusted_host_interfaces` names several interfaces separated by colons, such as `eth1:eth2`, and whose `com.docker.network.bridge.gateway_mode_ipv4` is `nat` or not set, and an underlay host with a route to the address of a container on the network via the node's underlay address | When traffic from that host to the container's address arrives on any of the named interfaces, it reaches the container's published ports. | C | `underlay` → `ctr` | `none` |

### 3.2 Port publishing

| Applies to | Expectation | Tier | Observed from | Depends on |
| --- | --- | --- | --- | --- |
| | `POST /containers/{id}/start` for a created or stopped container that is on a bridge network and publishes a port without a host port responds 204. `GET /containers/{id}/json` then reports the assigned host port in `NetworkSettings.Ports`. A client reaches the container on that host port at the node's address. | B | `api`, `client` | `os` |
| A container that is on a bridge network and listens on a port that it does not publish | A client that connects to the node's address, on any port, does not reach that listener. | B | `client`, `ctr` | `os` |
| | A port published on `127.0.0.1` is reachable from the node at `127.0.0.1`. | B | `node` | `os` |
| | A port published on `127.0.0.1` is not reachable from a client at any address of the node. | B | `client` | `os` |
| | A port published on an address of the node is reachable at that address, from the node and from a client. It is not reachable at the node's other addresses, `127.0.0.1` included. | B | `node`, `client` | `os` |
| A bridge network with IPv6 enabled, and a daemon with `ip6tables` true | A port published without a host address is reachable at the node's IPv6 addresses. | B | `client` | `os` |
| A bridge network with IPv6 enabled, and a daemon with `ip6tables` true | A client connects to a port published without a host address, at one of the node's IPv6 addresses. The container observes the IPv6 address of the client as the source address. | B | `client` → `ctr` | `os` |

### 3.3 Egress

| Applies to | Expectation | Tier | Observed from | Depends on |
| --- | --- | --- | --- | --- |
| A bridge network without the options `com.docker.network.bridge.enable_ip_masquerade`, `com.docker.network.bridge.gateway_mode_ipv4` and `com.docker.network.host_ipv4` | An underlay host observes an address of the node as the source address of IPv4 egress from the network's containers. | B | `underlay` | `os` |
| A bridge network with `com.docker.network.bridge.enable_ip_masquerade` set to `false`, and without `com.docker.network.bridge.gateway_mode_ipv4` | An underlay host observes the container's address as the source address of IPv4 egress from a container on the network. | B | `underlay` | `os` |
| A bridge network with `com.docker.network.bridge.enable_ip_masquerade` set to `false`, and without `com.docker.network.bridge.gateway_mode_ipv4` | An underlay host replies to IPv4 egress from a container on the network. The reply reaches the container only when `allow-direct-routing` is true, or when the reply arrives on an interface in the network's `com.docker.network.bridge.trusted_host_interfaces`. | B | `underlay` → `ctr` | `os` |
| A bridge network with `com.docker.network.host_ipv4` set to an address of the node, `com.docker.network.bridge.enable_ip_masquerade` not `false`, and `com.docker.network.bridge.gateway_mode_ipv4` not `routed` | An underlay host observes that address as the source address of IPv4 egress from the network's containers. | B | `underlay` | `os` |

### 3.4 Gateway modes

| Applies to | Expectation | Tier | Observed from | Depends on |
| --- | --- | --- | --- | --- |
| A bridge network with `com.docker.network.bridge.gateway_mode_ipv4` set to `routed` | An underlay host observes the container's address as the source address of IPv4 egress from a container on the network. | B | `underlay` | `os` |
| A container on a bridge network with `com.docker.network.bridge.gateway_mode_ipv4` set to `routed`, and a client with a route to the container's address via the node's address | Traffic from the client to the container's address reaches the container's published ports, and not its other ports. | B | `client` | `os` |
| A bridge network with `com.docker.network.bridge.gateway_mode_ipv4` set to `routed` | A client that connects to the node's IPv4 address, on a published port of a container on the network, does not reach the container. | B | `client` | `os` |
| A container on a bridge network with `com.docker.network.bridge.gateway_mode_ipv4` set to `nat-unprotected`, and a client with a route to the container's address via the node's address | Traffic from the client to the container's address reaches every port of the container. | B | `client` | `os` |
| | `POST /networks/create` for a bridge network with `Internal` not true and with `com.docker.network.bridge.gateway_mode_ipv4` or `com.docker.network.bridge.gateway_mode_ipv6` set to `isolated` responds with a 4xx or 5xx status. `GET /networks` then does not list the network. | B | `api` | `none` |
| | `POST /networks/create` for a bridge network with `Internal` not true and with `com.docker.network.bridge.gateway_mode_ipv4` or `com.docker.network.bridge.gateway_mode_ipv6` set to `isolated` responds 500. The message contains `gateway mode 'isolated' can only be used for an internal network`. See [§4](#4-known-divergences--non-goals). | C | `api` | `none` |
| | `docker network create --driver bridge --opt com.docker.network.bridge.gateway_mode_ipv4=isolated` without `--internal` exits non-zero and prints the daemon's error. `docker network ls` then does not list the network. | B | `api` | `cli` |
| | `POST /networks/create` for a bridge network with `com.docker.network.bridge.gateway_mode_ipv4` or `com.docker.network.bridge.gateway_mode_ipv6` set to a value other than `nat`, `nat-unprotected`, `routed` and `isolated` responds with a 4xx or 5xx status. `GET /networks` then does not list the network. | B | `api` | `none` |
| | `POST /networks/create` for a bridge network with `com.docker.network.bridge.gateway_mode_ipv4` or `com.docker.network.bridge.gateway_mode_ipv6` set to a value other than `nat`, `nat-unprotected`, `routed` and `isolated` responds 400. The message contains `unknown gateway mode`. | C | `api` | `none` |
| | `docker network create --driver bridge` with the option `com.docker.network.bridge.gateway_mode_ipv4` set to a value other than `nat`, `nat-unprotected`, `routed` and `isolated` exits non-zero and prints the daemon's error. `docker network ls` then does not list the network. | B | `api` | `cli` |

### 3.5 Default gateway

[network-common.md §3](network-common.md#3-default-gateway) covers which endpoint gives a container
its default route, for every network driver.

| Applies to | Expectation | Tier | Observed from | Depends on |
| --- | --- | --- | --- | --- |
| A container with endpoints on two or more bridge networks that are not `internal` and whose `com.docker.network.bridge.gateway_mode_ipv4` is `nat` or not set | An IPv4 client connects to a published port at the node's address. The connection reaches the container at its address on the network of its IPv4 default route. This holds after a connect or disconnect changes that network. | B | `client`, `ctr` | `os` |

## 4. Known divergences & non-goals

| Topic | Current behavior | Depends on |
| --- | --- | --- |
| Rules left in built-in iptables chains after a switch to nftables | The daemon deletes them only for the networks, endpoints and port bindings that it restores. Rules for anything that it does not restore stay in the built-in chains ([§1.3](#13-cross-generation-cleanup)). | `none` |
| Out-of-range bridge MTU error | The error does not name the option. `POST /networks/create` with an MTU that the kernel does not accept responds 500 with `invalid argument`. A request can have IPv6 enabled and an MTU that the kernel accepts for IPv4 but not for IPv6. That request responds 500 with a message that contains `Cannot read IPv6 setup for bridge` and `no such file or directory` ([§2](#2-network-and-endpoint-options)). | `none` |
| Bridge MTU from 2³² to 2⁶³ − 1 | The value is applied modulo 2³². This applies to `com.docker.network.driver.mtu` and to the daemon-wide `mtu`. A daemon-wide `mtu` whose remainder the kernel accepts does not stop the daemon. `POST /networks/create` accepts a value whose remainder the kernel accepts. For example, 4294968796 is applied as 1500. `GET /networks/{id}` then reports the requested value in `Options`. `POST /networks/create` refuses a value whose remainder the kernel refuses, as the *Out-of-range bridge MTU error* entry describes ([§2](#2-network-and-endpoint-options)). | `none` |
| Daemon-wide `mtu` error | A value that the kernel cannot apply stops the daemon with an error that ends in `error creating default "bridge" network: invalid argument`. This error does not name the setting ([§2](#2-network-and-endpoint-options)). | `none` |
| HTTP 500 for refused requests | The daemon refuses each request below on purpose, but the request responds 500. 500 is the status for a fault in the daemon. The requests are: (1) `POST /networks/create` for a bridge network with an MTU that the kernel refuses. (2) `POST /networks/create` for a bridge network with `com.docker.network.bridge.gateway_mode_ipv4` or `com.docker.network.bridge.gateway_mode_ipv6` set to `isolated`, when `Internal` is not true ([§2](#2-network-and-endpoint-options), [§3.4](#34-gateway-modes)). | `none` |

## Deliberately not asserted

- The counters of Moby's own rules.
- The content of Moby's own chains and tables, and their names, outside the configurations that
  [§1.5](#15-rule-content) pins. The exceptions are `DOCKER-USER` and the names that
  [§1.3](#13-cross-generation-cleanup) pins for cleanup. A backend change replaces exactly this
  content. A B row asserts the packet-level effect of the rules, or an anchor in
  [§1](#1-firewall-anchors--host-integration), and never the content.
- Which packet-filtering technology is in use, except where
  [§1](#1-firewall-anchors--host-integration) pins a backend-specific anchor.
- Whether an operator's `DOCKER-USER` rules survive a firewalld reload. That is firewalld's
  behavior. After a reload, the daemon recreates the chain and the jump to it. It does not restore
  the chain's rules.
