Container DNS surface area
==========================

Scope: name resolution inside a container on a user-defined network, whatever the network's
driver: the `/etc/resolv.conf` the daemon writes for the container, and the embedded DNS
resolver behind it, as observable from outside the daemon.

This is a stub. It holds the expectations that came up while writing the
[Swarm networking surface area](swarm-networking.md) but do not depend on any one driver, and it is
not yet a complete account. Its conventions for tiers, vantage points and dependencies are in
[README.md](README.md).

On Linux nodes, a container attached only to the default bridge network, or one using the host's
network, does not use the embedded resolver, and nothing here applies to it.

## 1. `resolv.conf`

This section holds on Linux nodes.

| Expectation | Tier | Observed from | Depends on |
| --- | --- | --- | --- |
| The `/etc/resolv.conf` nameserver is `127.0.0.11`. | C | `ctr` | `none` |
| A valid `ndots` in the container's DNS options is kept. | B | `ctr` | `none` |
| If the container's DNS options are empty, a valid `ndots` from the host's `resolv.conf` is kept. | B | `ctr` | `none` |
| If the container's DNS options include any option, none of the host's `resolv.conf` options are used, `ndots` included. | C | `ctr` | `none` |
| `/etc/resolv.conf` contains `options ndots:0` when the options in effect, per the rows above, include no valid `ndots`, whether none was set or all were invalid. | C | `ctr` | `none` |
| `/etc/resolv.conf` has the search domains the container was created with, or else the daemon's `dns-search`, or else the host's `resolv.conf` search list. | B | `ctr` | `none` |
| A search domain of `.` given with `--dns-search` or in the daemon's `dns-search` is left out of `/etc/resolv.conf`. When it is the only one given, `/etc/resolv.conf` does not have a `search` line, even if the host's `resolv.conf` has one. | B | `ctr` | `none` |
| Apart from `ndots`, which the rows above cover, `/etc/resolv.conf` lists the options in effect as they were given. | B | `ctr` | `none` |

A container's DNS options are those it was created with, or the daemon's `dns-opts` if it was
created without any. A valid `ndots` is an integer of 0 or more. Values above 15 are kept, although
glibc treats any `ndots` above 15 as 15.

## 2. Names and answers

| Expectation | Tier | Observed from | Depends on |
| --- | --- | --- | --- |
| A container's name resolves to its address on a network it shares with the querying container. | B | `ctr` | `none` |
| A container's name and its network aliases resolve whatever the case of the query: a container named `Web` resolves as `web` and as `WEB`. | B | `ctr` | `none` |
| A network alias given to a container's endpoint resolves to the container's address on that network, for containers on that network only. | B | `ctr` | `none` |
| An alias that several containers on a network share resolves to all of their addresses. | B | `ctr` | `none` |
| A container's short ID, its first 12 hex digits, and its hostname resolve as its name does. | C | `ctr` | `none` |
| A name resolves for endpoints on any network the querying container is attached to. | B | `ctr` | `os` (Windows) |
| A name does not resolve for endpoints on a network the querying container is not attached to. | B | `ctr` | `none` |
| A standalone container's records are published when it connects to a network, whether or not it has a healthcheck. | B | `ctr` | `none` |
| A container's records are withdrawn when it disconnects from the network or stops. | B | `ctr` | `none` |
| On a node not in a Swarm, and on any node for an overlay network, once `POST /containers/{id}/rename` responds 204, the container's new name resolves and its old name does not, for containers on its node; see [§5](#5-known-divergences--non-goals). | B | `ctr` | `none` |
| An A or AAAA query for a name the embedded resolver has records for is answered from those records whatever `ndots` the options in effect set, a single-label name included. | B | `ctr` | `none` |
| An AAAA query for the name of a container returns its IPv6 address on a network it shares with the querying container when none of the querying container's other networks has a record for the name. | B | `ctr` | `none` |
| An A or AAAA query for a name the embedded resolver has records for, but none of that address family, gets an empty `NOERROR` response and is not forwarded upstream. | B | `ctr` | `none` |
| When a name has records on more than one of the querying container's networks, an A or AAAA query is answered from one of them only, and gets an empty `NOERROR` response when that network has no address of the family asked for, even if another does. | C | `ctr` | `none` |
| Over UDP, an answer from the embedded resolver's own records lists all of the name's addresses when it fits in the larger of 512 bytes and the query's EDNS0 buffer size, and is otherwise truncated with the TC bit set. Over TCP, it lists all of them. | B | `ctr` | `none` |
| The order of the addresses in an answer from the embedded resolver's own records is shuffled per query, leaving the choice to the client. | C | `ctr` | `none` |
| Reverse (PTR) lookups resolve for the addresses of containers on the querying container's networks. | B | `ctr` | `os` (Windows) |
| Forward-resolving the name a reverse lookup returns for a container's address yields that address. | B | `ctr` | `none` |
| On Linux nodes, an alias given as `<name>:<alias>` in the `Links` of a container's endpoint settings for a user-defined network resolves to the named container's address on that network, for queries from the container that has the link and from no other container. | B | `ctr` | `none` |

## 3. Forwarding

| Expectation | Tier | Observed from | Depends on |
| --- | --- | --- | --- |
| Public DNS records resolve when the container has external access. | B | `ctr` | `os` |
| On Linux nodes, the embedded resolver does not forward queries from a container attached only to `internal` networks to the nameservers in the host's `resolv.conf`. | B | `ctr` | `none` |
| When the container has external access, the embedded resolver forwards to the nameservers set with `--dns`, or else to those in the daemon's `dns` setting, in place of the host's. | B | `ctr` | `os` |
| A container attached only to `internal` networks resolves names through a nameserver on one of those networks, set with `--dns` or in the daemon's `dns` setting. | B | `ctr` | `os` |
| On Linux nodes, when the container has external access and no nameservers are set with `--dns` or in the daemon's `dns` setting, the embedded resolver forwards to the nameservers in the host's `resolv.conf`, loopback ones included, and queries them from the host's network namespace. | B | `ctr` | `os` |
| On Linux nodes, the embedded resolver queries nameservers set with `--dns` or in the daemon's `dns` setting from the container's network namespace. | B | `ctr` | `os` |
| When a nameserver the embedded resolver forwards to does not respond, or responds SERVFAIL or REFUSED, the resolver forwards the query to the next one. When none gives another response, the client gets SERVFAIL. | B | `ctr` | `none` |
| A response other than SERVFAIL or REFUSED, NXDOMAIN included, goes back to the client without the next nameserver being asked. | B | `ctr` | `none` |
| The embedded resolver forwards to at most the first three nameservers. | C | `ctr` | `none` |
| On Windows nodes, and on Linux nodes when the options in effect, per [§1](#1-resolvconf), do not set `ndots`, a single-label A or AAAA query for a name the embedded resolver does not have a record for is forwarded upstream, as a multi-label name would be. | B | `ctr` | `none` |
| On Linux nodes, when the options in effect, per [§1](#1-resolvconf), set `ndots` to 1 or more, a single-label A or AAAA query for a name the embedded resolver does not have a record for gets an empty `NOERROR` response instead of being forwarded upstream; see [§5](#5-known-divergences--non-goals). | C | `ctr` | `none` |

The row for an `ndots` of 1 or more is C because the behavior itself is in question:
[moby/moby#53157](https://github.com/moby/moby/issues/53157) proposes putting container names in a
domain on the search list instead, which would make it unnecessary. An `ndots` of 0, or an invalid
one, has the same effect, which is a bug; see [§5](#5-known-divergences--non-goals).

## 4. Firewall anchors

This section holds on Linux nodes. Under the nftables backend, the embedded resolver redirects
`127.0.0.11:53` to the ports it actually listens on with rules in a table of its own, inside the
container's network namespace. The hooks and priorities of that table's base chains are an anchor:
they decide evaluation order against rules that anything else programs in the same namespace, such
as a container with `NET_ADMIN` running its own firewall or VPN. The names of the table and its
chains are not anchors.

| Expectation | Tier | Observed from | Depends on |
| --- | --- | --- | --- |
| The embedded resolver's nftables base chains are on the output hook at dstnat priority (−100) and on the postrouting hook at srcnat priority (100). | C\* | `ctr` (its network namespace) | `none` |

\* Tier C while the nftables backend is experimental. These expectations become tier B once the
nftables backend is a supported feature.

## 5. Known divergences & non-goals

| Topic | Current behavior | Depends on |
| --- | --- | --- |
| Single-label forwarding with an `ndots` of 0 or an invalid `ndots` | Not forwarded. When the options in effect set `ndots` to 0 or to an invalid value, a single-label A or AAAA query for a name the embedded resolver does not have a record for gets an empty `NOERROR` response, as it does for an `ndots` of 1 or more ([§3](#3-forwarding)). Only 1 or more is meant to have that effect ([moby/moby#53104](https://github.com/moby/moby/issues/53104)). It shows in Docker-in-Docker: the inner daemon reads the outer container's `resolv.conf` as its host's, finds the `ndots:0` the outer daemon added, and its containers cannot resolve names that only the outer resolver knows. | `none` |
| Single-label queries after a live restore | Forwarded. After the daemon restarts with `--live-restore`, a single-label A or AAAA query from a container that kept running is forwarded upstream, whatever `ndots` the options in effect set ([§3](#3-forwarding)). | `none` |
| Renaming a container on a Swarm node | Not applied. On a node in a Swarm, `POST /containers/{id}/rename` does not change the container's records on networks other than overlay networks. Its new name does not resolve there, and its old name keeps resolving, also after the container is removed ([§2](#2-names-and-answers)). | `none` |
| Reverse-lookup round trip on Windows | Fails. A reverse lookup returns `<name>.<network>`, and the embedded resolver on a Windows node does not answer names in that form. It looks the whole name up in the network's records, finds nothing, and forwards the query upstream ([§2](#2-names-and-answers)). | `none` |
| Nameservers on `internal` networks on Windows | Not applied. The daemon does not give a Windows endpoint on an `internal` network the nameservers set with `--dns` or in its `dns` setting. A Windows container attached only to `internal` networks cannot resolve names through a nameserver on one of them ([§3](#3-forwarding)). | `none` |

## Deliberately not asserted

- The comments the daemon writes into `resolv.conf`, such as its header and its record of where
  `ndots` came from.
- The ports the embedded resolver actually listens on behind `127.0.0.11:53`, and the rules that
  redirect to them, except for the hooks and priorities in [§4](#4-firewall-anchors).
