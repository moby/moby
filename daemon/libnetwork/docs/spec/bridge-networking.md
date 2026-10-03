Bridge networking surface area
==============================

Scope: the bridge network driver, and the parts of the daemon's firewall integration that are not
specific to Swarm, as observable from outside the daemon. Everything here holds on Linux nodes.

This is a stub. It holds expectations that came up while writing the
[Swarm networking surface area](swarm-networking.md) but belong to the bridge driver, and it is not
yet a complete account of it. Its conventions for tiers, vantage points and dependencies are in
[README.md](README.md).

## 1. Firewall anchors & host integration

The content of Moby's firewall rules is incidental: it is exactly what a backend change replaces.
Two things are an interface, and they are this section's anchors:

1. The `DOCKER-USER` iptables chain. Operators attach rules to it: it is documented as the place
   to put policy that survives daemon restarts, and the daemon never deletes or modifies its
   contents.
2. The hooks and priorities of the bridge driver's nftables base chains. They decide evaluation
   order against everything else on the host, such as firewalld, kube-proxy or an operator's own
   ruleset. A changed priority silently changes who sees a packet first.

No other name is an anchor: not Moby's other iptables chains, and nothing in nftables. An
operator's rules can only reach Moby's nftables chains by being programmed into Moby's own
tables, and that is not a supported configuration.

An enabled IP version is IPv4 while the daemon's `iptables` option is true, and IPv6 while its
`ip6tables` option is true, under either firewall backend and whether or not `ipv6` is set.

### 1.1 Base-chain priorities

The embedded resolver's base chains are in each container's network namespace rather than the
host's, and are specified in [container-dns.md](container-dns.md#4-firewall-anchors).

| Expectation | Tier | Observed from | Depends on |
| --- | --- | --- | --- |
| The bridge driver's nftables base chains are on the prerouting hook at raw priority (−300) and at dstnat priority (−100), on the output hook at dstnat priority (−100), on the forward hook at filter priority (0), and on the postrouting hook at srcnat priority (100). | C\* | `node` | `none` |

\* Tier C while the nftables backend is experimental. These expectations become tier B once the
nftables backend is a supported feature.

### 1.2 `DOCKER-USER`

`DOCKER-USER` belongs to the iptables backend, and these rows hold under it.

| Expectation | Tier | Observed from | Depends on |
| --- | --- | --- | --- |
| `DOCKER-USER` exists in the filter table and is jumped to from `FORWARD`, for each enabled IP version. | B | `node` | `os` |
| A rule an operator places in `DOCKER-USER` is evaluated before Moby's own forwarding rules, and can drop traffic to a published port, including a rule placed after a firewalld reload. | B | `client` | `os` |
| The daemon never deletes or modifies pre-existing `DOCKER-USER` rules. They survive a daemon restart, including one with `--iptables=false`. | B | `node`, `client` | `none` |
| After a firewalld reload, `DOCKER-USER` exists and is jumped to from `FORWARD`, for each enabled IP version. | B | `node` | `os` |

### 1.3 Cross-generation cleanup

A node running release *N−1* is upgraded to *N*, possibly with a different firewall backend, and
must not be left with two generations of live rules. Cleanup finds a previous generation's chains
and tables by name, which makes the names load-bearing for the daemon itself, though not an
interface for operators. A rename therefore needs a cleanup path for the old name, or an upgraded
node keeps the previous generation's rules. A test that runs a single binary never meets the
previous release's names. The C row that pins the names cleanup relies on makes a rename visible.
The rows in this section hold for each enabled IP version.

| Expectation | Tier | Observed from | Depends on |
| --- | --- | --- | --- |
| Once the daemon has started, the host netns does not contain any of the tables and chains a previous daemon generation using the other backend left there, except `DOCKER-USER` and the `FORWARD` jump to it, which hold the operator's rules and stay in place, and the overlay driver's `docker-overlay` table, which [swarm-networking.md §12.3](swarm-networking.md#123-cross-generation-cleanup) covers. | B | `node` | `os` |
| This holds in both directions, and across a package upgrade from the previous release as well as a same-binary restart. | B | `node` | `os`, `upgrade` |
| Chains and tables that an older generation created and this one does not, such as `DOCKER-ISOLATION`, other than `DOCKER-INGRESS`, which [swarm-networking.md §12.3](swarm-networking.md#123-cross-generation-cleanup) covers, do not remain either, when they are left alongside the previous release's chains. | B | `node` | `os` |
| After a switch from iptables to nftables, a rule the previous generation left in a built-in chain for a network, endpoint or port binding does not remain once the daemon has restored that network, endpoint or port binding, across a package upgrade from the previous release as well as a same-binary restart; see [§3](#3-known-divergences--non-goals). | B | `node` | `os`, `upgrade` |
| The daemon removes a previous generation's tables and chains when it starts: after a switch to iptables, at every start, it deletes the `docker-bridges` table; after a switch to nftables, at the first start, it deletes the chains `DOCKER` (in the filter and nat tables), `DOCKER-FORWARD`, `DOCKER-BRIDGE`, `DOCKER-CT`, `DOCKER-INTERNAL`, `DOCKER-ISOLATION-STAGE-1`, `DOCKER-ISOLATION-STAGE-2` and `DOCKER-ISOLATION`, and the jumps to them from `FORWARD` and from the nat table's `PREROUTING` and `OUTPUT`. | C | `node` | `os` |
| After a switch from iptables to nftables, the daemon removes the previous generation's rules in built-in chains for a network, endpoint or port binding when it restores that network, endpoint or port binding. | C | `node` | `os` |
| The names cleanup relies on are unchanged: the bridge driver's nftables table, `docker-bridges`; its iptables chains `DOCKER` (in the filter and nat tables), `DOCKER-FORWARD`, `DOCKER-BRIDGE`, `DOCKER-CT`, `DOCKER-INTERNAL`, `DOCKER-ISOLATION-STAGE-1` and `DOCKER-ISOLATION-STAGE-2`; and the old `DOCKER-ISOLATION` chain. | C | `node` | `none` |
| After a switch from iptables to nftables, the daemon leaves the policy of the filter table's `FORWARD` chain as it was, including a policy of DROP. | C\* | `node` | `none` |
| When that policy is DROP, the daemon logs a warning that contains `Network traffic for published ports may be dropped, iptables chain FORWARD has policy DROP.` on the first start after the switch. | C\* | `log` | `none` |
| After a switch from iptables to nftables, a DROP rule an operator placed in `DOCKER-USER` still drops the forwarded traffic it matches, including traffic to published ports. | C\* | `client` | `os` |

\* Tier C while the nftables backend is experimental. These expectations become tier B once the
nftables backend is a supported feature.

### 1.4 Firewall backend and firewalld

| Expectation | Tier | Observed from | Depends on |
| --- | --- | --- | --- |
| The daemon does not start with a `firewall-backend` other than `iptables` or `nftables`, and prints an error that contains `invalid firewall-backend`. | B | `log` | `none` |
| With firewalld running, a published port answers after a firewalld reload. | B | `client` | `os` |
| Under the iptables backend, with `ip-forward` and `iptables` true and `ip-forward-no-drop` false, once the daemon has created a bridge network with IPv4 on a host where `net.ipv4.ip_forward` was 0, `net.ipv4.ip_forward` is 1 and the filter table's `FORWARD` chain has policy DROP. | B | `node` | `os` |
| Likewise with `ip6tables` true, once the daemon has created a bridge network with IPv6 on a host where `net.ipv6.conf.all.forwarding` or `net.ipv6.conf.default.forwarding` was 0, both are 1 and the ip6tables filter table's `FORWARD` chain has policy DROP. | B | `node` | `os` |
| Under the nftables backend, with `ip-forward` true, unless the default bridge is disabled or `--live-restore` is enabled and containers are running when it starts, the daemon does not start when `net.ipv4.ip_forward` is 0, and prints an error that contains `IPv4 forwarding is disabled`. | C\* | `log` | `none` |

\* Tier C while the nftables backend is experimental. This expectation becomes tier B once the
nftables backend is a supported feature.

## 2. Network and endpoint options

| Expectation | Tier | Observed from | Depends on |
| --- | --- | --- | --- |
| `POST /networks/create` for a bridge network whose `com.docker.network.driver.mtu` is not an integer from 0 to 2⁶³ − 1 responds 400, with a message that includes the value. | B | `api` | `none` |
| `POST /networks/create` for a bridge network with an MTU the kernel refuses responds 500, and `GET /networks` then does not list the network; see [§3](#3-known-divergences--non-goals). | C | `api` | `none` |
| When the daemon refuses one of the requests in this section, the `docker network create`, `docker run -d` or `docker network connect` command that made it exits non-zero and prints the daemon's error. | B | `api` | `cli` |
| A bridge network's positive `com.docker.network.driver.mtu` is the MTU of its containers' interfaces; see [§3](#3-known-divergences--non-goals). | B | `ctr` | `os` |
| A bridge network's `com.docker.network.driver.mtu` of 0 leaves the MTU of the bridge and of its containers' interfaces as the kernel set it. | C | `node`, `ctr` | `none` |
| The daemon does not start with a negative daemon-wide `mtu`, and prints an error that contains `invalid default MTU`. | B | `log` | `none` |
| Unless `--live-restore` is enabled and containers are running when it starts, the daemon does not start with a daemon-wide `mtu` that the kernel cannot apply to the default bridge; see [§3](#3-known-divergences--non-goals). | B | `log` | `none` |
| With `--live-restore` enabled and containers running when it starts, the daemon starts with a daemon-wide `mtu` that the kernel cannot apply to the default bridge, and the default bridge keeps its MTU. | B | `log`, `node` | `none` |
| The daemon-wide `mtu` is the MTU of the interfaces of containers on the default bridge. | B | `ctr` | `os` |
| The interface name follows `com.docker.network.endpoint.ifname` when set; see [§3](#3-known-divergences--non-goals). | B | `ctr` | `os` |
| `POST /containers/{id}/start` and `POST /networks/{id}/connect` for an endpoint whose `com.docker.network.endpoint.ifname` the OS refuses respond 500, with a message that contains `error renaming interface` and the requested name; see [§3](#3-known-divergences--non-goals). | C | `api` | `none` |
| When two of a container's endpoints ask for the same interface name, or one asks for `lo`, the start fails, or else the connect that attaches the second of them fails. The response is 500 with a message that names the interface; see [§3](#3-known-divergences--non-goals). | C | `api` | `none` |

## 3. Known divergences & non-goals

| Topic | Current behavior | Depends on |
| --- | --- | --- |
| Rules left in built-in iptables chains after a switch to nftables | Removed only for the networks, endpoints and port bindings the daemon restores. Rules for anything it does not restore stay in the built-in chains ([§1.3](#13-cross-generation-cleanup)). | `none` |
| Out-of-range bridge MTU error | The error does not name the option. `POST /networks/create` with an MTU the kernel does not accept responds 500 with `invalid argument`. With IPv6 enabled, one the kernel accepts for IPv4 but not for IPv6 responds 500 with a message that contains `Cannot read IPv6 setup for bridge` and `no such file or directory` ([§2](#2-network-and-endpoint-options)). | `none` |
| Bridge MTU from 2³² to 2⁶³ − 1 | Applied modulo 2³². `POST /networks/create` accepts a value whose remainder the kernel accepts, such as 4294968796, applied as 1500, and `GET /networks/{id}` then reports the requested value in `Options`. One whose remainder the kernel refuses is refused as in the entry above ([§2](#2-network-and-endpoint-options)). | `none` |
| Daemon-wide `mtu` error | A value the kernel cannot apply stops the daemon with an error that ends in `error creating default "bridge" network: invalid argument`, which does not name the setting. ([§2](#2-network-and-endpoint-options)). | `none` |
| HTTP 500 for refused requests | As in [swarm-networking.md §13](swarm-networking.md#13-known-divergences--non-goals), `POST /networks/create` responds 500 for a bridge MTU the kernel refuses, and `POST /containers/{id}/start` and `POST /networks/{id}/connect` respond 500 for an `ifname` the OS refuses or that another of the container's endpoints already uses ([§2](#2-network-and-endpoint-options)). | `none` |
| Interface left behind after a failed connect | As in [swarm-networking.md §13](swarm-networking.md#13-known-divergences--non-goals). | `none` |
| Interface names the kernel rewrites | As in [swarm-networking.md §13](swarm-networking.md#13-known-divergences--non-goals). | `none` |
| Interface-name collisions depend on attach order | As in [swarm-networking.md §13](swarm-networking.md#13-known-divergences--non-goals). | `none` |

## Deliberately not asserted

- The content of Moby's own chains and tables, and their names other than `DOCKER-USER` except as
  [§1.3](#13-cross-generation-cleanup) pins them for cleanup: individual match/target lines, their
  ordering within a chain, counters, and `iptables-save` / `nft list ruleset` diffs. The rules are
  exactly what a backend change replaces; the surface is their packet-level effect. The anchors in
  [§1](#1-firewall-anchors--host-integration) are the exception.
- Which packet-filtering technology is in use, except where
  [§1](#1-firewall-anchors--host-integration) pins a backend-specific anchor.
- Whether an operator's `DOCKER-USER` rules survive a firewalld reload. That is firewalld's
  behavior: the daemon recreates the chain and the jump to it after a reload, and does not restore
  the chain's rules.
