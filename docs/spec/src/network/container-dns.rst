############################
 Container DNS surface area
############################

Scope: name resolution inside a container on a user-defined network, whatever the network's driver.
This covers the :file:`/etc/resolv.conf` that the daemon writes for the container and the embedded
DNS resolver behind it, as observable from outside the daemon. In this spec, the user-defined
networks are the networks other than the predefined networks, ingress networks and
:network:`docker_gwbridge`. On a Swarm-scoped network of a node-local driver, such as
:driver:`bridge`, the expectations hold for containers on the same node. Each node attaches its
containers to a network of its own, whose subnet can overlap the subnets on other nodes. On Linux
nodes, a :api-field:`Links` alias of the querying container takes precedence over the records of its
networks, as :need:`DNS-NAMES-LINK-ALIAS-PRECEDENCE` says, and over forwarding. The other
expectations of this spec hold for a name, and for the ``<name>`` of a :dns-name:`<name>.<network>`
query, that is not such an alias.

This spec holds expectations that do not depend on any one driver. It is not yet a complete account.
:doc:`../README.md <../README>` gives the model that this spec follows: the items of a spec, their
categories and what their verdicts depend on. :doc:`README.md <README>` gives the conventions of the
networking specs, such as vantage points, daemon settings, timing and convergence ceilings, and host
requirements.

A container sends its DNS queries to the embedded DNS resolver, which is a DNS server that the
daemon runs for the container. The expectations state what the embedded resolver answers, and what
it forwards to upstream nameservers. A name resolves to an address when the embedded resolver
answers a DNS query for the name with that address. A container has external access when its default
route is on a network that is not ``internal``, and replies to its egress traffic reach it. A
container on a bridge network whose :drvopt:`com.docker.network.bridge.enable_ip_masquerade` is
``false`` does not have external access where :need:`BR-EGRESS-REPLY-DIRECT-ROUTING-OFF` applies. An
expectation that says that the embedded resolver forwards a query upstream does not cover a
container whose upstream nameservers are all the embedded resolver's own address, ``127.0.0.11`` on
a Linux node: a container created with :cli-flag:`--dns` nameservers that are all that address, or
one created without :cli-flag:`--dns` nameservers whose daemon's :daemon-json:`dns` setting names
only that address. The host's :file:`resolv.conf` is :file:`/etc/resolv.conf`, also on a host that
runs systemd-resolved. An expectation that takes nameservers, search domains or options from the
host's :file:`resolv.conf` takes them from that file as it is when the container starts, unless the
expectation says that the daemon reads it when the daemon started. Each start of the container reads
the file again and writes the container's :file:`/etc/resolv.conf` again, also when a process in the
container has changed it.

On Linux nodes, the containers that follow do not use the embedded resolver, and nothing here
applies to them, other than the expectations whose state names the default bridge:

- A container attached only to the default bridge network, that has not had an endpoint on a
  user-defined network since it started.
- A container that uses the host's network.
- A container attached to the :network:`none` network.

On Windows nodes, the expectations of this spec that state what the embedded resolver forwards, or
how it answers a query that it forwards, hold for a container whose endpoints are all on
:driver:`overlay` networks, or on :driver:`nat` networks created without
:drvopt:`com.docker.network.windowsshim.disable_gatewaydns` set to true.

.. _container-dns-requirements:

**************
 Requirements
**************

.. req:: User-defined networks use the embedded DNS server at 127.0.0.11
    :id: REQ-DNS-EMBEDDED-RESOLVER
    :source_kind: docs
    :source: https://docs.docker.com/engine/network/#dns-services, https://github.com/docker/docs/blob/0e064795f2b8996655643315ab6ba82d72594e8a/engine/userguide/networking/configure-dns.md

    A container attached to a user-defined network uses Docker's embedded DNS server, whose address
    is 127.0.0.11, and applications that need an explicit DNS server address can use it. It has no
    IPv6 address. The IPv4 address works in IPv6-only containers too.

.. req:: Lookups the embedded DNS server cannot answer are forwarded upstream
    :id: REQ-DNS-FWD-EXTERNAL
    :source_kind: docs
    :source: https://docs.docker.com/engine/network/#dns-services, https://docs.docker.com/engine/release-notes/27/#270, https://github.com/docker/docs/blob/0e064795f2b8996655643315ab6ba82d72594e8a/engine/userguide/networking/configure-dns.md

    The embedded DNS server forwards external DNS lookups, those it cannot resolve itself, to
    upstream DNS servers. On Windows it does so by default.

.. req:: Internal-only containers use DNS servers on their networks, not external
    :id: REQ-DNS-FWD-INTERNAL
    :source_kind: docs
    :source: https://github.com/moby/moby/security/advisories/GHSA-mq39-4gv4-mvpx, https://docs.docker.com/engine/release-notes/26.0/#2600, https://docs.docker.com/engine/release-notes/26.1/#2613
    :refines: PRIN-LEAST-ACCESS

    For a container connected only to internal networks, the daemon does not forward requests to
    external DNS servers, even when the host's server is on a loopback address. DNS servers within
    an internal network stay usable.

.. req:: --dns servers are queried from the container, host servers from the host
    :id: REQ-DNS-FWD-NAMESPACE
    :source_kind: docs
    :source: https://docs.docker.com/engine/network/#dns-services, https://docs.docker.com/engine/release-notes/28/#2800

    Requests to --dns servers are forwarded from the container's network namespace, so
    --dns=127.0.0.1 is the container's own loopback. Nameservers read from the host's
    /etc/resolv.conf are always accessed from the host's network namespace.

.. req:: Upstream servers are tried in order until a success or NXDOMAIN
    :id: REQ-DNS-FWD-ORDER
    :source_kind: docs
    :source: https://docs.docker.com/engine/network/#dns-services, https://docs.docker.com/engine/release-notes/18.03/#18031-ce, https://docs.docker.com/engine/release-notes/24.0/#2401

    On a custom network the embedded DNS server queries upstream servers in order and stops after a
    successful or NXDOMAIN response. A SERVFAIL moves on to the next server, and an NXDOMAIN is
    returned to the client as NXDOMAIN.

.. req:: A REFUSED from an upstream server moves on to the next server
    :id: REQ-DNS-FWD-REFUSED
    :source_kind: issue
    :source: https://github.com/moby/libnetwork/pull/2171: a maintainer's merged change and its commit message

    The embedded DNS server queries the next upstream server after a REFUSED response, as it does
    after a SERVFAIL.

.. req:: With ndots set above 0, an unknown single-label name is not forwarded
    :id: REQ-DNS-FWD-SINGLE-LABEL-NDOTS
    :source_kind: issue
    :source: moby/moby daemon/libnetwork/resolver.go: code comment on ndots in the forwarding path

    When the user sets ndots above 0 and an A or AAAA query is for a name in the root domain, a
    single-label name, that the embedded resolver cannot answer from its records, the resolver does
    not forward it. It answers without an address and lets the client retry with a search domain
    attached.

.. req:: Upstream servers come from --dns, else the daemon's dns, else the host
    :id: REQ-DNS-FWD-UPSTREAM-SERVERS
    :source_kind: docs
    :source: https://docs.docker.com/engine/network/#dns-services, https://docs.docker.com/reference/cli/dockerd/#daemon-dns-options, https://docs.docker.com/engine/release-notes/28/#2800, https://docs.docker.com/engine/release-notes/26.1/#2613

    By default containers use the same DNS servers as the host, from its /etc/resolv.conf. --dns
    overrides them for a container, and the daemon's dns setting sets them for all containers. The
    embedded DNS server ignores its own address when it is given as an upstream server.

.. req:: A live restore keeps the embedded resolver's configuration for running containers
    :id: REQ-DNS-LIVE-RESTORE-RESOLVER
    :source_kind: issue
    :source: https://github.com/moby/libnetwork/pull/1620, moby/moby daemon/libnetwork/sandbox_store.go: code comment on sbState.ExtDNS

    After the daemon restarts with live-restore, the embedded resolver of a container that kept
    running forwards queries as it did before the restart.

.. req:: A network alias resolves on its network, a shared one to every holder
    :id: REQ-DNS-NAMES-ALIAS
    :source_kind: docs
    :source: https://docs.docker.com/reference/cli/docker/network/connect/#alias, https://github.com/docker/docs/blob/0e064795f2b8996655643315ab6ba82d72594e8a/engine/userguide/networking/work-with-networks.md, https://docs.docker.com/engine/release-notes/prior-releases/#1110-2016-04-13

    A network alias given with --network-alias or docker network connect --alias resolves to the
    container on the network where it is defined. Several containers can share an alias on a
    network; the answer then carries the records of all of them, and which of them comes first
    varies between answers (DNS round robin).

.. req:: A container name resolves before a search domain is tried
    :id: REQ-DNS-NAMES-BEFORE-SEARCH-DOMAINS
    :source_kind: observable
    :source: Users who reach containers by single-label names on hosts whose search domains answer for many names (wildcard records or NXDOMAIN redirection). They rely on the name resolving to the container without knowing that ndots decides the order of the lookups

    In a container whose resolv.conf gets no valid ndots, neither from the container's or the
    daemon's DNS options nor from the host's resolv.conf, a single-label name that the embedded
    resolver has records for resolves to the container. This holds when a search domain of the
    container has a record for that name too.

.. req:: Container names and aliases resolve whatever the case of the query
    :id: REQ-DNS-NAMES-CASE
    :source_kind: issue
    :source: https://github.com/moby/moby/issues/21169, https://github.com/moby/moby/issues/28689

    The embedded DNS server matches names case-insensitively, as DNS does (RFC 4343).

.. req:: Containers on a user-defined network resolve each other by name
    :id: REQ-DNS-NAMES-CTR-NAME
    :source_kind: docs
    :source: https://docs.docker.com/engine/network/drivers/bridge/#differences-between-user-defined-bridges-and-the-default-bridge, https://docs.docker.com/engine/network/#user-defined-networks, https://docs.docker.com/reference/cli/docker/container/run/#name

    On a user-defined network, containers can reach each other by container name through automatic
    DNS resolution, unlike on the default bridge network.

.. req:: DNSNames lists an endpoint's DNS names, qualified as <name>.<network>
    :id: REQ-DNS-NAMES-DNSNAMES
    :source_kind: api
    :source: https://docs.docker.com/reference/api/engine/version/v1.54/#tag/Container/operation/ContainerInspect, api/openapi.yaml: definition EndpointSettings (DNSNames)

    The API's DNSNames lists all DNS names a container has on a network: its name, network aliases,
    short ID and hostname. Appending .<network-name> to one of them gives its fully qualified form.

.. req:: Names follow connects, disconnects, starts, stops and renames
    :id: REQ-DNS-NAMES-DYNAMIC
    :source_kind: docs
    :source: https://docs.docker.com/engine/network/drivers/bridge/#differences-between-user-defined-bridges-and-the-default-bridge, https://github.com/docker/docs/blob/0e064795f2b8996655643315ab6ba82d72594e8a/engine/userguide/networking/work-with-networks.md, https://github.com/moby/moby/issues/22466: a rename whose new name did not resolve that maintainers treated as a bug

    Containers can be connected to and disconnected from user-defined networks on the fly, and name
    resolution follows: names and aliases stop resolving after a disconnect or stop, and resolve
    again after a start. A renamed container resolves by its new name.

.. req:: A link alias resolves only for the container that has the link
    :id: REQ-DNS-NAMES-LINK-ALIAS
    :source_kind: docs-history
    :source: https://github.com/docker/docs/blob/0e064795f2b8996655643315ab6ba82d72594e8a/engine/userguide/networking/configure-dns.md, https://github.com/docker/docs/blob/0e064795f2b8996655643315ab6ba82d72594e8a/engine/userguide/networking/work-with-networks.md

    --link=CONTAINER:ALIAS on a user-defined network gives the embedded DNS server an extra name
    ALIAS for that container, and the lookup result is local to the container that has the link.

.. req:: Names resolve only among containers that share a network
    :id: REQ-DNS-NAMES-NETWORK-SCOPE
    :source_kind: docs
    :source: https://docs.docker.com/engine/network/drivers/bridge/#differences-between-user-defined-bridges-and-the-default-bridge, https://docs.docker.com/engine/network/drivers/bridge/#use-user-defined-bridge-networks, https://github.com/docker/docs/blob/0e064795f2b8996655643315ab6ba82d72594e8a/engine/userguide/networking/work-with-networks.md
    :refines: PRIN-LEAST-ACCESS

    A container's names and aliases are scoped to the networks it is attached to: containers on
    those networks resolve them, containers on other networks do not, and a container attached to
    several networks resolves the names on each of them.

.. req:: The embedded DNS server answers A, AAAA, PTR and MX for its names
    :id: REQ-DNS-NAMES-QUERY-TYPES
    :source_kind: docs
    :source: https://docs.docker.com/engine/release-notes/prior-releases/#1110-2016-04-13, https://docs.docker.com/engine/release-notes/18.02/#18020-ce, https://github.com/moby/moby/issues/21562

    The embedded DNS server answers AAAA queries (IPv6 service discovery) as well as A queries,
    reverse (PTR) queries for container addresses, and MX queries, for the names it has records for.

.. req:: An MX query for a known name gets an empty NOERROR answer
    :id: REQ-DNS-NAMES-MX-IMPLICIT
    :source_kind: issue
    :source: moby/moby daemon/libnetwork/resolver.go: code comment in handleMXQuery

    When the embedded resolver resolves a name, it answers an MX query for that name with an empty
    list and NOERROR, so that email clients can treat the name as an implicit MX (RFC 5321 section
    5.1) and issue an A or AAAA query for it.

.. req:: A process in a container can serve DNS on port 53
    :id: REQ-DNS-PORT53-SERVER
    :source_kind: observable
    :source: Users who run DNS servers such as dnsmasq or Pi-hole or AdGuard Home or CoreDNS or unbound in containers on user-defined networks. These servers bind port 53 on the wildcard address by default

    A process in a container on a user-defined network can bind TCP and UDP port 53, on the wildcard
    address too. The container's queries to 127.0.0.11 still reach the embedded resolver.

.. req:: A container's resolv.conf is editable at runtime
    :id: REQ-DNS-RESOLV-EDITABLE
    :source_kind: docs
    :source: https://docs.docker.com/engine/release-notes/prior-releases/#120-2014-08-20

    /etc/resolv.conf is editable at runtime inside the container.

.. req:: A search domain of . sets no search domain
    :id: REQ-DNS-RESOLV-SEARCH-DOT
    :source_kind: docs
    :source: https://github.com/docker/cli/blob/e5ef05054cb2215f5256cef460fd01f47091b04a/man/docker-run.1.md: the --dns-search flag help, https://github.com/moby/moby/blob/c61fa33c23330b134e4107b138d3141424c8dc22/docs/userguide/networking/default_network/configure-dns.md: the --dns-search entry of its options table, moby/moby daemon/pkg/opts/opts.go: code comment on ValidateDNSSearch that validates the daemon's dns-search and reads a dot as a zero-length domain

    A search domain of . given with --dns-search or in the daemon's dns-search sets no search
    domain: when it is the only search domain given, the container has none.

.. req:: Search domains and DNS options come from the container, daemon or host
    :id: REQ-DNS-RESOLV-SEARCH-OPTIONS
    :source_kind: docs
    :source: https://docs.docker.com/engine/network/#dns-services, https://docs.docker.com/reference/cli/dockerd/#daemon-dns-options

    A container gets the search domains and DNS options given with --dns-search and --dns-opt;
    otherwise those set for all containers in the daemon's dns-search and dns-opts; otherwise it
    inherits the settings in the host's /etc/resolv.conf.

.. _container-dns-rationales:

************
 Rationales
************

.. rationale:: The daemon's dns nameservers are reached from each container's namespace
    :id: RAT-DNS-DAEMON-NAMESERVERS-NAMESPACE

    The daemon's :daemon-json:`dns` setting gives every container that has no :cli-flag:`--dns`
    nameservers the same upstream nameservers. The embedded resolver queries them from each
    container's network namespace, as it queries :cli-flag:`--dns` nameservers, so a loopback
    address in the setting is each container's own loopback, not the host's. A move to the host's
    network namespace would send the queries of every container that the setting covers to a service
    on the host in place of one in the container, at once, and a move back would do the reverse. It
    should be made as a decision.

.. rationale:: A host resolv.conf that cannot be read as a file fails at the start, or at the first query
    :id: RAT-DNS-HOST-RESOLV-UNREADABLE

    The daemon reads the host's :file:`/etc/resolv.conf` to find the upstream servers of a container
    that uses the embedded resolver. A directory, a socket or a FIFO at that path cannot be read as
    a file. Today the read's error, or a read that does not return, stops the container's start or
    the daemon's own. The other choice is to treat the path as a file without nameservers, as
    :need:`DNS-FWD-HOST-NAMESERVERS-NONE` states for a missing file. The container then starts, and
    the embedded resolver forwards to the loopback addresses in the host's network namespace, where
    a resolver may not listen. One choice fails at the start. The other fails later, at the
    container's first query for a name that the embedded resolver has no record for. A change from
    one to the other should be deliberate.

.. rationale:: A link alias is the linking container's own name for a container
    :id: RAT-DNS-LINK-ALIAS-PRECEDENCE

    A link alias names the container that the linking container chose. Today it wins over every name
    and network alias on that container's networks, so the linking container reaches the same
    container whatever other containers join those networks. If network names won instead, a
    container that joined later with the alias as its name would take the linking container's
    traffic. A change to that order should be deliberate.

.. rationale:: A later rewrite of a running container's resolv.conf would replace edits made in the container
    :id: RAT-DNS-RESOLV-WRITTEN-ONCE

    Once the embedded resolver has started for a container, the daemon does not write the
    container's :file:`/etc/resolv.conf` again while the container runs. From then on, the file and
    the resolver do not follow the container's networks: a container that leaves its last
    user-defined network keeps :resolv-opt:`nameserver 127.0.0.11`, and the resolver keeps answering
    there. A process in the container can edit the file while the container runs. A daemon that
    wrote the file again, for example to restore the host's nameservers after such a disconnect,
    would replace those edits. It would also have to stop the resolver at the same moment, or leave
    the file and the resolver in disagreement. When the daemon writes the file should change by
    decision.

.. rationale:: The rcode and flags of a response decide what the client does next
    :id: RAT-DNS-RESPONSE-STEERS-CLIENT

    The response that the embedded resolver gives decides how a client's lookup goes on. The TC bit
    sends the client to TCP, where it gets every address (:need:`DNS-NAMES-ANSWER-SIZE`).
    :rcode:`SERVFAIL` reads as a temporary failure, which clients retry or take to another resolver.
    Another rcode passed through from a nameserver ends the lookup with that nameserver's verdict. A
    change to which response a client gets in these cases changes how its lookups end, and should be
    deliberate.

.. rationale:: When no configured upstream is left, a fallback would send queries to servers that nobody configured
    :id: RAT-DNS-NO-UPSTREAM-LEFT

    The embedded resolver does not forward to a configured nameserver ``127.0.0.11``
    (:need:`DNS-FWD-NAMESERVER-SELF`, :need:`DNS-FWD-NAMESERVER-SELF-DAEMON`). When that is the
    container's only nameserver, no upstream is left. Three outcomes are possible for a name without
    records: :rcode:`SERVFAIL`, as today; a fallback to the host's nameservers; or a query that the
    resolver forwards to itself. A fallback would send the container's queries to servers that its
    configuration does not name, and a loop would hold every such query until it times out. The
    choice among the three should be made on purpose.

.. rationale:: For a name on the network, the resolver's choice for other query types decides which queries go upstream
    :id: RAT-DNS-OTHER-TYPES-LEAVE-THE-NODE

    The embedded resolver answers a name that it has address records for itself only for A, AAAA and
    MX queries. A query of another type, for a name that it has records for, takes the path of a
    query for a name without records (:need:`DNS-FWD-UNKNOWN-NAME`): it goes upstream. An empty
    answer from the resolver itself is as defensible, because the name belongs to the container's
    network. The two differ for a name that exists both on the network and upstream: today its
    addresses come from the network and every other record from upstream. They also differ in
    whether queries that carry the names of containers and aliases leave the node. A change to that
    split, in either direction, should be deliberate.

.. rationale:: The TTL sets how long a client may keep an address that no longer belongs to the name
    :id: RAT-DNS-TTL-STALE-ADDRESSES

    The TTL of an answer from the embedded resolver's own records is the time for which a client or
    a caching resolver in the container may keep using an address after the container behind the
    name stops, restarts with another address, or is replaced. A longer TTL lengthens that time. A
    shorter one sends more queries to the resolver. The value trades one against the other, and a
    change to it should be a decision about that trade.

.. _container-dns-host-requirements:

*******************
 Host requirements
*******************

The kernel facilities and programs hold on Linux nodes. The host configuration holds on Linux and
Windows nodes. The conventions for this section are in :ref:`README.md <README-host-requirements>`.
The embedded resolver needs NAT rules in the container's network namespace under the active firewall
backend. It needs them whether or not the daemon's :daemon-json:`iptables` setting is true.

.. _container-dns-kernel-facilities:

Kernel facilities
=================

.. hostreq:: nftables with the NAT expression
    :id: DNS-HOST-NFTABLES-NAT
    :kind: os
    :platforms: linux
    :kernel_options: NF_TABLES, NF_TABLES_IPV4, NFT_NAT, NFT_CHAIN_NAT_IPV4
    :when: nftables-backend

    *Needed when:* The nftables backend, for a container or a task that sends a query to the
    embedded resolver

    *Kernel options:* :kconfig:`NF_TABLES`, :kconfig:`NF_TABLES_IPV4`, :kconfig:`NFT_NAT`. Before
    Linux 5.1, also :kconfig:`NFT_CHAIN_NAT_IPV4`.

.. needextend:: type == "expectation" and docname in ['network/container-dns', 'network/swarm-networking', 'network/swarm-traffic', 'network/swarm-lifecycle'] and ("linux" in platforms) and ("Names and answers" in sections or "Forwarding" in sections or "Port 53 in the container" in sections or "Service discovery" in sections) and ("ctr" in vantage or "task" in vantage) and "DNS-HOST-NFTABLES-NAT" not in pins
    :+needs: DNS-HOST-NFTABLES-NAT

.. _container-dns-programs:

Programs
========

.. hostreq:: iptables with the nat table and the DNAT and SNAT targets
    :id: DNS-HOST-IPTABLES-COMMAND
    :kind: program
    :platforms: linux
    :when: iptables-backend

    *Facility:* :program:`iptables`, accepting the nat table and the :ipt-target:`DNAT` and
    :ipt-target:`SNAT` targets

    *Needed when:* The iptables backend, for a container or a task that sends a query to the
    embedded resolver

.. needextend:: type == "expectation" and docname in ['network/container-dns', 'network/swarm-networking', 'network/swarm-traffic', 'network/swarm-lifecycle'] and ("linux" in platforms) and ("Names and answers" in sections or "Forwarding" in sections or "Port 53 in the container" in sections or "Service discovery" in sections) and ("ctr" in vantage or "task" in vantage) and "DNS-HOST-IPTABLES-COMMAND" not in pins
    :+needs: DNS-HOST-IPTABLES-COMMAND

.. hostreq:: The nft command
    :id: DNS-HOST-NFT-COMMAND
    :kind: program
    :platforms: linux
    :when: nftables-backend

    *Facility:* :program:`nft`

    *Needed when:* The nftables backend, on a daemon built without the ``libnftables`` build tag,
    for a container or a task that sends a query to the embedded resolver. A daemon built with it
    uses the libnftables library and runs no :program:`nft` command.

.. needextend:: type == "expectation" and docname in ['network/container-dns', 'network/swarm-networking', 'network/swarm-traffic', 'network/swarm-lifecycle'] and ("linux" in platforms) and ("Names and answers" in sections or "Forwarding" in sections or "Port 53 in the container" in sections or "Service discovery" in sections) and ("ctr" in vantage or "task" in vantage) and "DNS-HOST-NFT-COMMAND" not in pins
    :+needs: DNS-HOST-NFT-COMMAND

The daemon does not depend on which implementation of :program:`iptables` the host has. The kernel
options that the program needs depend on that implementation:

.. hostreq:: Kernel options for iptables
    :id: DNS-HOST-IPTABLES-KERNEL
    :kind: os
    :platforms: linux
    :kernel_options: NETFILTER, NETFILTER_XTABLES, NF_CONNTRACK, NF_NAT, NETFILTER_XT_NAT
    :when: iptables-backend

    *Needed when:* The iptables backend, for a container or a task that sends a query to the
    embedded resolver

    The kernel options that :program:`iptables` needs, whichever implementation provides it. *Kernel
    options:* :kconfig:`NETFILTER`, :kconfig:`NETFILTER_XTABLES`, :kconfig:`NF_CONNTRACK`,
    :kconfig:`NF_NAT`, :kconfig:`NETFILTER_XT_NAT`

.. needextend:: type == "expectation" and docname in ['network/container-dns', 'network/swarm-networking', 'network/swarm-traffic', 'network/swarm-lifecycle'] and ("linux" in platforms) and ("Names and answers" in sections or "Forwarding" in sections or "Port 53 in the container" in sections or "Service discovery" in sections) and ("ctr" in vantage or "task" in vantage) and "DNS-HOST-IPTABLES-KERNEL" not in pins
    :+needs: DNS-HOST-IPTABLES-KERNEL

.. hostreq:: Kernel options for iptables from iptables-legacy
    :id: DNS-HOST-IPTABLES-LEGACY-KERNEL
    :kind: os
    :platforms: linux
    :kernel_options: IP_NF_IPTABLES, IP_NF_NAT, IP_NF_IPTABLES_LEGACY, NETFILTER_XTABLES_LEGACY
    :when: iptables-backend, iptables-legacy

    *Needed when:* The iptables backend with iptables-legacy providing :program:`iptables`, for a
    container or a task that sends a query to the embedded resolver

    The kernel options that :program:`iptables` needs when iptables-legacy provides it. *Kernel
    options:* :kconfig:`IP_NF_IPTABLES`, :kconfig:`IP_NF_NAT`. :kconfig:`IP_NF_IPTABLES_LEGACY` from
    Linux 6.9, and :kconfig:`NETFILTER_XTABLES_LEGACY` from Linux 6.17.

.. needextend:: type == "expectation" and docname in ['network/container-dns', 'network/swarm-networking', 'network/swarm-traffic', 'network/swarm-lifecycle'] and ("linux" in platforms) and ("Names and answers" in sections or "Forwarding" in sections or "Port 53 in the container" in sections or "Service discovery" in sections) and ("ctr" in vantage or "task" in vantage) and "DNS-HOST-IPTABLES-LEGACY-KERNEL" not in pins
    :+needs: DNS-HOST-IPTABLES-LEGACY-KERNEL

.. hostreq:: Kernel options for iptables from iptables-nft
    :id: DNS-HOST-IPTABLES-NFT-KERNEL
    :kind: os
    :platforms: linux
    :kernel_options: NF_TABLES, NF_TABLES_IPV4, NFT_COMPAT, NFT_NAT, NFT_CHAIN_NAT_IPV4
    :when: iptables-backend, iptables-nft

    *Needed when:* The iptables backend with iptables-nft providing :program:`iptables`, for a
    container or a task that sends a query to the embedded resolver

    The kernel options that :program:`iptables` needs when iptables-nft provides it. *Kernel
    options:* :kconfig:`NF_TABLES`, :kconfig:`NF_TABLES_IPV4`, :kconfig:`NFT_COMPAT`,
    :kconfig:`NFT_NAT`. Before Linux 5.1, :kconfig:`NFT_CHAIN_NAT_IPV4` in place of
    :kconfig:`NFT_NAT`.

.. needextend:: type == "expectation" and docname in ['network/container-dns', 'network/swarm-networking', 'network/swarm-traffic', 'network/swarm-lifecycle'] and ("linux" in platforms) and ("Names and answers" in sections or "Forwarding" in sections or "Port 53 in the container" in sections or "Service discovery" in sections) and ("ctr" in vantage or "task" in vantage) and "DNS-HOST-IPTABLES-NFT-KERNEL" not in pins
    :+needs: DNS-HOST-IPTABLES-NFT-KERNEL

.. _container-dns-host-configuration:

Host configuration
==================

.. hostreq:: Host nameservers that the node reaches and that resolve the queried names
    :id: DNS-HOST-EXTERNAL-ACCESS
    :kind: config
    :platforms: linux, windows

    *Facility:* Host nameservers that the node reaches, and that resolve the names that an
    expectation queries, such as public names. On a Linux node, these are the nameservers in the
    host's :file:`resolv.conf`. On a Windows node, these are the nameservers in the DNS server list
    that the Host Networking Service gives the container's endpoint, other than the endpoint's
    gateway address, where the embedded resolver listens. A nameserver on a network that the node
    shares with it meets this as well as one on the internet.

    *Needed when:* A query that the embedded resolver forwards to the host's nameservers

.. _container-dns-file-resolv-conf:

*********************
 :file:`resolv.conf`
*********************

This section holds on Linux nodes. Its expectations, other than the expectation about a process's
changes to the file, hold for a :file:`/etc/resolv.conf` that has not changed since the daemon wrote
it.

A container's DNS options are the options that it was created with. If it was created without DNS
options, they are the daemon's :daemon-json:`dns-opts`. The options in effect are the container's
DNS options when it has any. Otherwise, they are the options in the host's :file:`resolv.conf`. A
valid :resolv-opt:`ndots` is an integer of 0 or more.

.. expectation:: The resolv.conf nameserver is 127.0.0.11
    :id: DNS-RESOLV-NAMESERVER
    :observed_from: ctr, task
    :depends_on: none
    :platforms: linux
    :refines: REQ-DNS-EMBEDDED-RESOLVER

    The :file:`/etc/resolv.conf` nameserver is ``127.0.0.11``. The file names no other nameserver,
    IPv4 or IPv6.

.. expectation:: In an IPv6-only container, 127.0.0.11 answers for names on its networks
    :id: DNS-RESOLV-IPV6-ONLY
    :observed_from: ctr, task
    :depends_on: none
    :platforms: linux
    :refines: REQ-DNS-EMBEDDED-RESOLVER
    :needs: DNS-HOST-NFTABLES-NAT, DNS-HOST-IPTABLES-COMMAND, DNS-HOST-NFT-COMMAND, DNS-HOST-IPTABLES-KERNEL, DNS-HOST-IPTABLES-LEGACY-KERNEL, DNS-HOST-IPTABLES-NFT-KERNEL

    *Applies to:* A container whose user-defined networks all have :api-field:`EnableIPv4` false and
    :api-field:`EnableIPv6` true, and another container on one of those networks, with IPv6 enabled
    in its network namespace and on its interfaces on those networks, where no endpoint on those
    networks other than that container's has that container's name or a name that differs from it
    only in the case of its letters

    The :file:`/etc/resolv.conf` nameserver is ``127.0.0.11``. An AAAA query that the container
    sends to ``127.0.0.11`` for the other container's name gets that container's IPv6 address on one
    of those networks.

.. expectation:: resolv.conf has the ndots from the container's DNS options
    :id: DNS-RESOLV-NDOTS-CTR
    :observed_from: ctr, task
    :depends_on: none
    :platforms: linux
    :refines: REQ-DNS-RESOLV-SEARCH-OPTIONS

    *Applies to:* A container whose DNS options include a valid :resolv-opt:`ndots`

    :file:`/etc/resolv.conf` has the :resolv-opt:`ndots` value from the container's DNS options. A
    value above 15 is kept as given.

.. expectation:: Without DNS options, resolv.conf has the host's ndots
    :id: DNS-RESOLV-NDOTS-HOST
    :observed_from: ctr, task
    :depends_on: none
    :platforms: linux
    :refines: REQ-DNS-RESOLV-SEARCH-OPTIONS

    *Applies to:* A container whose DNS options are empty, and a host :file:`resolv.conf` with a
    valid :resolv-opt:`ndots`

    :file:`/etc/resolv.conf` has the :resolv-opt:`ndots` value from the host's :file:`resolv.conf`.

.. expectation:: With container DNS options, resolv.conf has no host option they lack
    :id: DNS-RESOLV-OPTIONS-NO-HOST
    :observed_from: ctr, task
    :depends_on: none
    :platforms: linux
    :refines: REQ-DNS-RESOLV-SEARCH-OPTIONS

    *Applies to:* A container whose DNS options include any option, and a host :file:`resolv.conf`
    with an option that the container's DNS options do not include, such as :resolv-opt:`edns0`

    :file:`/etc/resolv.conf` has no option that the host's :file:`resolv.conf` has and the
    container's DNS options do not, apart from an :resolv-opt:`ndots:0` that the daemon adds when
    the container's DNS options set no valid :resolv-opt:`ndots`.

.. expectation:: Without ndots, a container name resolves before the search domains
    :id: DNS-RESOLV-CTR-NAME-BEFORE-SEARCH
    :observed_from: ctr, task
    :depends_on: none
    :platforms: linux
    :refines: REQ-DNS-NAMES-BEFORE-SEARCH-DOMAINS
    :needs: DNS-HOST-NFTABLES-NAT, DNS-HOST-IPTABLES-COMMAND, DNS-HOST-NFT-COMMAND, DNS-HOST-IPTABLES-KERNEL, DNS-HOST-IPTABLES-LEGACY-KERNEL, DNS-HOST-IPTABLES-NFT-KERNEL
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A container whose options in effect do not include a valid :resolv-opt:`ndots`,
    with a search domain ``<domain>`` in its :file:`/etc/resolv.conf`, and an upstream nameserver
    that has a record for :dns-name:`<name>.<domain>`, where ``<name>`` is a single-label name that
    the embedded resolver has records for

    A lookup of ``<name>`` in the container, through a resolver that follows its
    :file:`/etc/resolv.conf`, such as the C library's, gets the addresses in the embedded resolver's
    records for ``<name>``. It does not get the address of :dns-name:`<name>.<domain>`.

.. expectation:: Without a valid ndots in effect, resolv.conf has options ndots:0
    :id: DNS-RESOLV-NDOTS-ZERO
    :observed_from: ctr, task
    :depends_on: none
    :platforms: linux
    :exact_form_of: DNS-RESOLV-CTR-NAME-BEFORE-SEARCH

    *Applies to:* A container whose options in effect do not include a valid :resolv-opt:`ndots`

    :file:`/etc/resolv.conf` contains :resolv-opt:`options ndots:0`. This applies when the options
    do not set :resolv-opt:`ndots`, and when all of the :resolv-opt:`ndots` values in them are
    invalid.

.. expectation:: resolv.conf has the container's search domains
    :id: DNS-RESOLV-SEARCH-CTR
    :observed_from: ctr, task
    :depends_on: none
    :platforms: linux
    :refines: REQ-DNS-RESOLV-SEARCH-OPTIONS

    *Applies to:* A container created with search domains

    :file:`/etc/resolv.conf` has the search domains that the container was created with, other than
    ``.``.

.. expectation:: Without its own search domains, resolv.conf has the daemon's dns-search
    :id: DNS-RESOLV-SEARCH-DAEMON
    :observed_from: ctr, task
    :depends_on: none
    :platforms: linux
    :refines: REQ-DNS-RESOLV-SEARCH-OPTIONS

    *Applies to:* A container created without search domains, and a daemon with
    :daemon-json:`dns-search` set

    :file:`/etc/resolv.conf` has the search domains in the daemon's :daemon-json:`dns-search`, other
    than ``.``.

.. expectation:: Without any dns-search, resolv.conf has the host's search list
    :id: DNS-RESOLV-SEARCH-HOST
    :observed_from: ctr, task
    :depends_on: none
    :platforms: linux
    :refines: REQ-DNS-RESOLV-SEARCH-OPTIONS

    *Applies to:* A container created without search domains, and a daemon without
    :daemon-json:`dns-search`

    :file:`/etc/resolv.conf` has the search list from the host's :file:`resolv.conf`.

.. expectation:: A search domain of . is left out of resolv.conf
    :id: DNS-RESOLV-SEARCH-DOT
    :observed_from: ctr, task
    :depends_on: none
    :platforms: linux
    :refines: REQ-DNS-RESOLV-SEARCH-DOT

    :file:`/etc/resolv.conf` does not contain a search domain of ``.`` given with
    :cli-flag:`--dns-search` or in the daemon's :daemon-json:`dns-search`.

.. expectation:: With . as the only search domain, resolv.conf has no search line
    :id: DNS-RESOLV-SEARCH-DOT-ONLY
    :observed_from: ctr, task
    :depends_on: none
    :platforms: linux
    :refines: REQ-DNS-RESOLV-SEARCH-DOT

    *Applies to:* A container whose only search domain, given with :cli-flag:`--dns-search` or in
    the daemon's :daemon-json:`dns-search`, is ``.``

    :file:`/etc/resolv.conf` does not have a :resolv-opt:`search` line, even if the host's
    :file:`resolv.conf` has one.

.. expectation:: Apart from ndots, resolv.conf lists the options as given
    :id: DNS-RESOLV-OPTIONS-AS-GIVEN
    :observed_from: ctr, task
    :depends_on: none
    :platforms: linux
    :refines: REQ-DNS-RESOLV-SEARCH-OPTIONS

    Apart from :resolv-opt:`ndots`, :file:`/etc/resolv.conf` lists the options in effect as they
    were given.

.. expectation:: A bridge-mode container gets search and options from systemd's resolv.conf
    :id: DNS-RESOLV-DEFAULT-BRIDGE-SYSTEMD-SOURCE
    :observed_from: ctr, task
    :depends_on: os-linux
    :platforms: linux
    :narrows: DNS-RESOLV-OPTIONS-AS-GIVEN, DNS-RESOLV-SEARCH-HOST, DNS-RESOLV-NDOTS-HOST, DNS-RESOLV-NDOTS-ZERO, DNS-RESOLV-CTR-NAME-BEFORE-SEARCH
    :violates: REQ-DNS-RESOLV-SEARCH-OPTIONS

    *Applies to:* A Linux node whose :file:`/etc/resolv.conf` named only ``127.0.0.53`` when the
    daemon started, and a container that has an endpoint on a user-defined network, from its start
    or after a connect, and whose :api-field:`HostConfig.NetworkMode` is the default bridge, such as
    a container that :api-op:`POST /containers/create` created without a :api-field:`NetworkMode`,
    even when its :api-field:`EndpointsConfig` names only a user-defined network, or the container
    of a service task whose first network is not one of the Swarm's predefined networks

    When neither the container nor the daemon gives search domains, the container's
    :file:`/etc/resolv.conf` has the search list of :file:`/run/systemd/resolve/resolv.conf`. Where
    the search list of the host's :file:`/etc/resolv.conf` differs from it, the container's file
    does not have that list. When the container's DNS options are empty, the container's file has
    the options of :file:`/run/systemd/resolve/resolv.conf` other than an invalid
    :resolv-opt:`ndots`, with an :resolv-opt:`ndots:0` added when they set no valid
    :resolv-opt:`ndots`. Apart from that :resolv-opt:`ndots:0`, it has no option of the host's
    :file:`/etc/resolv.conf` that :file:`/run/systemd/resolve/resolv.conf` lacks.

.. expectation:: A process's change to resolv.conf is kept, across connects too
    :id: DNS-RESOLV-CTR-EDIT-KEPT
    :observed_from: ctr
    :depends_on: none
    :platforms: linux
    :refines: REQ-DNS-RESOLV-EDITABLE

    *Applies to:* A running container, also one attached only to the default bridge network

    After a process in the container changes the container's :file:`/etc/resolv.conf`, the file
    keeps the process's content while the container runs. A connect or disconnect of the container
    does not change the file. This includes a connect of a container that was attached only to the
    default bridge network to a user-defined network.

.. expectation:: After a connect from the default bridge, the container resolves names
    :id: DNS-RESOLV-DEFAULT-BRIDGE-CONNECT
    :observed_from: ctr
    :depends_on: none
    :platforms: linux
    :refines: REQ-DNS-EMBEDDED-RESOLVER, REQ-DNS-FWD-UPSTREAM-SERVERS, REQ-DNS-NAMES-DYNAMIC, REQ-DNS-FWD-NAMESPACE
    :needs: DNS-HOST-NFTABLES-NAT, DNS-HOST-IPTABLES-COMMAND, DNS-HOST-NFT-COMMAND, DNS-HOST-IPTABLES-KERNEL, DNS-HOST-IPTABLES-LEGACY-KERNEL, DNS-HOST-IPTABLES-NFT-KERNEL, DNS-HOST-EXTERNAL-ACCESS

    *Applies to:* A running container attached only to the default bridge, created without
    :cli-flag:`--dns` nameservers, whose daemon does not have a :daemon-json:`dns` setting, and
    whose :file:`/etc/resolv.conf` has not changed since the daemon wrote it

    After :api-op:`POST /networks/{id}/connect` to a user-defined network responds 200, the
    container resolves the names of the containers on that network. The embedded resolver forwards
    the queries that :need:`DNS-FWD-UNKNOWN-NAME` says it forwards upstream to the nameservers in
    the host's :file:`resolv.conf`, loopback nameservers included. It queries them from the host's
    network namespace.

.. expectation:: After a connect from the default bridge, the nameserver is 127.0.0.11
    :id: DNS-RESOLV-DEFAULT-BRIDGE-CONNECT-NAMESERVER
    :observed_from: ctr
    :depends_on: none
    :platforms: linux
    :refines: REQ-DNS-EMBEDDED-RESOLVER, REQ-DNS-NAMES-DYNAMIC

    *Applies to:* A running container attached only to the default bridge, whose
    :file:`/etc/resolv.conf` has not changed since the daemon wrote it

    After :api-op:`POST /networks/{id}/connect` to a user-defined network responds 200, the
    :file:`/etc/resolv.conf` nameserver is ``127.0.0.11``.

.. expectation:: resolv.conf is not rewritten once the embedded resolver starts
    :id: DNS-RESOLV-NOT-UPDATED
    :observed_from: ctr
    :depends_on: none
    :platforms: linux
    :rationale: RAT-DNS-RESOLV-WRITTEN-ONCE

    After the embedded resolver starts for a container, :file:`/etc/resolv.conf` does not change
    again while the container runs. After a disconnect of the container from its last user-defined
    network, the file still names the embedded resolver's address as its nameserver.

.. expectation:: A container started on a user-defined network keeps its resolv.conf
    :id: DNS-RESOLV-UNCHANGED-USER-DEFINED-START
    :observed_from: ctr
    :depends_on: none
    :platforms: linux
    :pins: DNS-RESOLV-NOT-UPDATED

    *Applies to:* A running container that started with an endpoint on a user-defined network

    The container's :file:`/etc/resolv.conf` keeps the content that it had at start. This holds
    after later connects and disconnects. After a disconnect of the container from its last
    user-defined network, the file still names the embedded resolver's address as its nameserver.

.. expectation:: After a connect from the default bridge, resolv.conf stops changing
    :id: DNS-RESOLV-UNCHANGED-AFTER-CONNECT
    :observed_from: ctr
    :depends_on: none
    :platforms: linux
    :pins: DNS-RESOLV-NOT-UPDATED

    *Applies to:* A running container that started attached only to the default bridge, and that a
    later connect attached to a user-defined network

    The container's :file:`/etc/resolv.conf` keeps the content that it had after that connect. This
    holds after later connects and disconnects. After a disconnect of the container from its last
    user-defined network, the file still names the embedded resolver's address as its nameserver.

.. expectation:: The embedded resolver keeps answering after the last disconnect
    :id: DNS-RESOLV-RESOLVER-AFTER-DISCONNECT
    :observed_from: ctr
    :depends_on: none
    :platforms: linux
    :needs: DNS-HOST-NFTABLES-NAT, DNS-HOST-IPTABLES-COMMAND, DNS-HOST-NFT-COMMAND, DNS-HOST-IPTABLES-KERNEL, DNS-HOST-IPTABLES-LEGACY-KERNEL, DNS-HOST-IPTABLES-NFT-KERNEL
    :rationale: RAT-DNS-RESOLV-WRITTEN-ONCE

    *Applies to:* A running container that has had an endpoint on a user-defined network at any time
    since it started

    After the container disconnects from every user-defined network, the embedded resolver continues
    to answer the container's queries.

.. _container-dns-names-and-answers:

*******************
 Names and answers
*******************

In this section, the querying container is the container that sends the DNS query. The embedded
resolver has records for a name on a network when an endpoint on that network has that name. On an
overlay network, the endpoint of a service's task counts only while the task is running, as
:ref:`swarm-traffic-service-discovery` in :doc:`swarm-traffic.md <swarm-traffic>` defines it. An
endpoint's names are its container's name, short ID and hostname, and its network aliases. On a
Linux node, the names of an endpoint on the default bridge are instead the names, if any, that
:api-field:`NetworkSettings.Networks.bridge.DNSNames` lists for its container. An endpoint on an
ingress network, and the endpoint that the daemon adds on :network:`docker_gwbridge` for a
container's gateway, have no names. An endpoint also has every name that differs from one of these
only in the case of its letters. An expectation about a name holds where, on the querying
container's networks, no endpoint other than the one that the expectation names has that name,
unless the expectation names that case. In :dns-name:`<name>.<network>`, ``<name>`` stands for a
name and ``<network>`` for the name of a network. Both are placeholders, not literal text. On a
Swarm-scoped network of a multi-host driver, such as :driver:`overlay`, a request's effect on what
containers on other nodes resolve holds within the ceiling that the timing rules in :ref:`README.md
<readme-timing>` give it.

An expectation in this section names ``none`` when the embedded DNS resolver answers the query from
its own records. It does this the same way on every platform.

.. expectation:: A container's name resolves to its address on a shared network
    :id: DNS-NAMES-CTR-NAME
    :observed_from: ctr, task
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-DNS-NAMES-CTR-NAME
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A running container with an endpoint on a user-defined network that it shares with
    the querying container, where no other network of the querying container has an endpoint with
    that container's name

    The container's name resolves to its address on the shared network.

.. expectation:: Names and aliases resolve whatever the case of the query
    :id: DNS-NAMES-CASE
    :observed_from: ctr, task
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-DNS-NAMES-CASE
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A running container with an endpoint on a user-defined network that it shares with
    the querying container

    Its name and its network aliases resolve whatever the case of the query: a container named
    :dns-name:`Web` resolves as :dns-name:`web` and as :dns-name:`WEB`.

.. expectation:: A network alias resolves on its network only
    :id: DNS-NAMES-ALIAS
    :observed_from: ctr, task
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-DNS-NAMES-ALIAS, REQ-DNS-NAMES-NETWORK-SCOPE
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A network alias given to a container's endpoint, where no other network of the
    querying container has an endpoint with that alias

    The alias resolves to the container's address on that network, for containers on that network
    only.

.. expectation:: An alias that several containers share resolves to all of them
    :id: DNS-NAMES-ALIAS-SHARED
    :observed_from: ctr, task
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-DNS-NAMES-ALIAS
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* Several containers on a network whose endpoints have the same network alias, where
    no other network of the querying container has an endpoint with that alias

    The alias resolves to the addresses of all of those containers on that network. Over UDP, this
    holds when the answer fits in 512 bytes, or in the query's EDNS0 buffer size when that is
    larger.

.. expectation:: A container's short ID and hostname resolve as its name does
    :id: DNS-NAMES-SHORT-ID-HOSTNAME
    :observed_from: ctr, task
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-DNS-NAMES-DNSNAMES, REQ-DNS-NAMES-CTR-NAME
    :within: SWARM-CEIL-REASSIGNMENT

    A container's short ID (the first 12 hex digits of its ID) and its hostname resolve as its name
    does.

.. expectation:: DNSNames lists the name, aliases, short ID and hostname, which all resolve
    :id: DNS-NAMES-DNSNAMES
    :observed_from: api, ctr, task
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-DNS-NAMES-DNSNAMES
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A running container with an endpoint on a user-defined network

    :api-op:`GET /containers/{id}/json` lists in
    :api-field:`NetworkSettings.Networks.<network>.DNSNames` the container's name, the aliases of
    its endpoint on that network, its short ID and its hostname. Each of them resolves on that
    network.

.. expectation:: DNSNames lists the name, the aliases, the short ID, then the hostname
    :id: DNS-NAMES-DNSNAMES-SHORT-ID
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows
    :exact_form_of: DNS-NAMES-DNSNAMES

    *Applies to:* A running container with an endpoint on a user-defined network

    :api-field:`NetworkSettings.Networks.<network>.DNSNames` in :api-op:`GET /containers/{id}/json`
    lists the container's name, then the aliases of its endpoint on that network, then the
    container's short ID and then its hostname. It does not list a name more than once.

.. expectation:: After a rename, DNSNames has the new name and not the old one
    :id: DNS-NAMES-DNSNAMES-RENAME
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-DNS-NAMES-DNSNAMES, REQ-DNS-NAMES-DYNAMIC

    *Applies to:* A running container whose hostname and network aliases are not its name

    After :api-op:`POST /containers/{id}/rename` for the container responds 204,
    :api-field:`NetworkSettings.Networks.<network>.DNSNames` in :api-op:`GET /containers/{id}/json`
    has the new name for each user-defined network. It does not have the old name. It still lists
    the aliases, the short ID and the hostname, and no other name.

.. expectation:: DNSNames on the default bridge is empty
    :id: DNS-NAMES-DNSNAMES-DEFAULT-BRIDGE
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :refines: REQ-DNS-NAMES-DNSNAMES

    *Applies to:* Linux nodes with a container attached to the default bridge

    :api-field:`NetworkSettings.Networks.bridge.DNSNames` in :api-op:`GET /containers/{id}/json` is
    empty.

.. expectation:: After a rename, DNSNames on the default bridge lists names
    :id: DNS-NAMES-DNSNAMES-DEFAULT-BRIDGE-RENAME-FILLED
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :narrows: DNS-NAMES-DNSNAMES-DEFAULT-BRIDGE
    :concerns: DNS-NAMES-DNSNAMES-RENAME
    :violates: REQ-DNS-NAMES-DNSNAMES

    *Applies to:* A running container attached to the default bridge

    After :api-op:`POST /containers/{id}/rename` for the container responds 204,
    :api-field:`NetworkSettings.Networks.bridge.DNSNames` in :api-op:`GET /containers/{id}/json`
    lists the new name, the short ID and, when it differs from the short ID, the hostname. The list
    stays after the container stops and starts again. A rename while the container is stopped does
    not change it.

.. expectation:: DNSNames that a request gives for the default bridge are reported as given
    :id: DNS-NAMES-DNSNAMES-DEFAULT-BRIDGE-REQUESTED
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :narrows: DNS-NAMES-DNSNAMES-DEFAULT-BRIDGE
    :violates: REQ-DNS-NAMES-DNSNAMES

    :api-op:`POST /containers/create` with :api-field:`HostConfig.NetworkMode` ``bridge`` and a list
    of names in :api-field:`NetworkingConfig.EndpointsConfig.bridge.DNSNames` responds 201, and
    :api-op:`POST /networks/{id}/connect` that connects a container to the default bridge with such
    a list in :api-field:`EndpointConfig.DNSNames` responds 200. :api-op:`GET /containers/{id}/json`
    then reports that list in :api-field:`NetworkSettings.Networks.bridge.DNSNames`, also when its
    names are not the container's name, short ID or hostname. A start of the container does not
    change the list.

.. expectation:: A name resolves if it exists on any of the querying container's networks
    :id: DNS-NAMES-ANY-NETWORK
    :observed_from: ctr, task
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-DNS-NAMES-NETWORK-SCOPE
    :within: SWARM-CEIL-REASSIGNMENT

    A name resolves when an endpoint with that name exists on any user-defined network that the
    querying container is attached to.

.. expectation:: A query gets no address from a network the querying container is not on
    :id: DNS-NAMES-OTHER-NETWORK
    :observed_from: ctr, task
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-DNS-NAMES-NETWORK-SCOPE
    :within: SWARM-CEIL-REASSIGNMENT

    A query for a name does not get the address of an endpoint on a network that the querying
    container is not attached to. This holds also when an endpoint on one of the querying
    container's networks has that name.

.. expectation:: After a start, a container's name and aliases resolve on its networks
    :id: DNS-NAMES-START
    :observed_from: ctr
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-DNS-NAMES-DYNAMIC
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A created or stopped standalone container

    After :api-op:`POST /containers/{id}/start` for the container responds 204, its name and its
    aliases on each of its user-defined networks resolve for the other containers on that network.
    This holds whether or not the container has a healthcheck.

.. expectation:: After a connect, a container's name and aliases resolve on the network
    :id: DNS-NAMES-CONNECT
    :observed_from: ctr
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-DNS-NAMES-DYNAMIC
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A running standalone container

    After :api-op:`POST /networks/{id}/connect` to a user-defined network for the container responds
    200, its name and its aliases on that network resolve for the other containers on the network.
    This holds whether or not the container has a healthcheck.

.. expectation:: After a disconnect, a container's name and aliases stop resolving there
    :id: DNS-NAMES-DISCONNECT
    :observed_from: ctr
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-DNS-NAMES-DYNAMIC
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A running container with an endpoint on a user-defined network

    After :api-op:`POST /networks/{id}/disconnect` for the container from that network responds 200,
    the other containers on that network do not resolve its name or aliases to its address on that
    network.

.. expectation:: After a stop, a container's name and aliases stop resolving
    :id: DNS-NAMES-STOP
    :observed_from: ctr, task
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-DNS-NAMES-DYNAMIC
    :within: SWARM-CEIL-ALLOCATION, SWARM-CEIL-REASSIGNMENT

    *Applies to:* A running container with endpoints on user-defined networks

    After :api-op:`POST /containers/{id}/stop` for the container responds 204, the other containers
    on each of those networks do not resolve its name or aliases to its address on that network.

.. expectation:: After its process exits, a container's name and aliases stop resolving
    :id: DNS-NAMES-EXIT
    :observed_from: ctr, task
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-DNS-NAMES-DYNAMIC
    :within: SWARM-CEIL-ALLOCATION, SWARM-CEIL-REASSIGNMENT

    *Applies to:* A running container with endpoints on user-defined networks, whose restart policy
    does not restart it

    After the daemon emits the container's :api-event:`die` event, whether its process exited or
    :api-op:`POST /containers/{id}/kill` ended it, the other containers on each of those networks do
    not resolve its name or aliases to its address on that network. On a network that is not
    Swarm-scoped, this holds once the daemon emits the event.

.. expectation:: After a rename, the new name resolves and the old one does not
    :id: DNS-NAMES-RENAME
    :observed_from: ctr
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-DNS-NAMES-DYNAMIC

    *Applies to:* A running container whose hostname and network aliases differ from its name in
    more than the case of their letters

    After :api-op:`POST /containers/{id}/rename` for the container, with a new name that differs
    from its old name in more than the case of its letters, responds 204, its new name resolves and
    its old name does not resolve to its address, for containers on each of its user-defined
    networks on its node.

.. expectation:: A default-bridge container's names do not resolve to its default-bridge address
    :id: DNS-NAMES-DEFAULT-BRIDGE-NO-NAMES
    :observed_from: ctr
    :depends_on: none
    :platforms: linux
    :refines: REQ-DNS-NAMES-CTR-NAME

    *Applies to:* A node that is not in a Swarm, a running container attached only to the default
    bridge, created and started without :api-field:`DNSNames` for it, and another container attached
    to the default bridge and to a user-defined network

    The other container does not resolve the first container's name, short ID or hostname to the
    first container's address on the default bridge.

.. expectation:: After a rename, default-bridge names resolve on a user-defined network
    :id: DNS-NAMES-DEFAULT-BRIDGE-RENAME-RESOLVES
    :observed_from: ctr
    :depends_on: none
    :platforms: linux
    :narrows: DNS-NAMES-DEFAULT-BRIDGE-NO-NAMES
    :concerns: DNS-NAMES-RENAME
    :violates: REQ-DNS-NAMES-CTR-NAME

    *Applies to:* A node that is not in a Swarm, a running container attached to the default bridge,
    and another container attached to the default bridge and to a user-defined network

    After :api-op:`POST /containers/{id}/rename` for the first container responds 204, the other
    container resolves the new name and the short ID to the renamed container's address on the
    default bridge. After the renamed container stops and starts again, the other container resolves
    the names that :api-field:`NetworkSettings.Networks.bridge.DNSNames` then lists to the renamed
    container's address on the default bridge.

.. expectation:: Names requested for the default bridge resolve on a user-defined network
    :id: DNS-NAMES-DEFAULT-BRIDGE-REQUESTED-RESOLVES
    :observed_from: ctr
    :depends_on: none
    :platforms: linux
    :concerns: DNS-NAMES-DEFAULT-BRIDGE-NO-NAMES, DNS-NAMES-DNSNAMES-DEFAULT-BRIDGE-REQUESTED
    :violates: REQ-DNS-NAMES-CTR-NAME

    *Applies to:* A node that is not in a Swarm, a running container attached only to the default
    bridge, created with :api-field:`HostConfig.NetworkMode` ``bridge`` and a list of names that
    includes its name in :api-field:`NetworkingConfig.EndpointsConfig.bridge.DNSNames`, and another
    container attached to the default bridge and to a user-defined network

    The other container resolves each name in that list, the first container's name included, to the
    first container's address on the default bridge.

.. expectation:: A and AAAA queries for a known name are answered from its records
    :id: DNS-NAMES-ADDRESS-RECORDS
    :observed_from: ctr, task
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-DNS-NAMES-CTR-NAME, REQ-DNS-NAMES-QUERY-TYPES
    :within: SWARM-CEIL-REASSIGNMENT

    The embedded resolver answers an A or AAAA query for a name that it has records for from those
    records. This holds for a single-label name too, whatever :resolv-opt:`ndots` is set by the
    options in effect, which the :ref:`container-dns-file-resolv-conf` section defines.

.. expectation:: A query gets no address when the first network lacks that family
    :id: DNS-NAMES-NETWORK-ORDER-FAMILY-MISSING
    :observed_from: ctr, task
    :depends_on: none
    :platforms: linux
    :within: SWARM-CEIL-REASSIGNMENT
    :narrows: DNS-NAMES-ADDRESS-RECORDS
    :violates: REQ-DNS-NAMES-QUERY-TYPES, REQ-DNS-NAMES-CTR-NAME

    *Applies to:* A Linux node, and a name that several of the querying container's networks have
    records for, where the first of those networks that the embedded resolver consults has no
    address of one family for the name and another of them has one. On a node that is not in a
    Swarm, the embedded resolver consults the networks in the order that
    :need:`DNS-NAMES-NETWORK-ORDER` gives. On a node in a Swarm, it consults the Swarm-scoped
    networks other than the ingress network first, then the ingress network, then the networks that
    are not Swarm-scoped, and this expectation covers a name that, in the first of these groups that
    has records for it, only one network has records for.

    An A or AAAA query for the name, of that family, gets an empty :rcode:`NOERROR` response.

.. expectation:: An AAAA query returns the container's IPv6 address on a shared network
    :id: DNS-NAMES-AAAA
    :observed_from: ctr, task
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-DNS-NAMES-CTR-NAME, REQ-DNS-NAMES-QUERY-TYPES
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A container with an IPv6 address on a user-defined network that it shares with the
    querying container, where no other network of the querying container has an endpoint with that
    container's name

    An AAAA query for that name returns the container's IPv6 address on the shared network.

.. expectation:: A query of the address type a name lacks is answered without an address
    :id: DNS-NAMES-ADDRESS-TYPE-MISSING
    :observed_from: ctr, task
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-DNS-NAMES-QUERY-TYPES
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* Records in the embedded resolver for a name that are all A records, or all AAAA
    records, and an upstream nameserver of the container that has a record of the other type for
    that name

    A query of the other type for that name gets a response without an address. The embedded
    resolver does not forward this query upstream.

.. expectation:: A query of the address type a name lacks gets an empty NOERROR
    :id: DNS-NAMES-ADDRESS-TYPE-MISSING-NOERROR
    :observed_from: ctr, task
    :depends_on: none
    :platforms: linux, windows
    :exact_form_of: DNS-NAMES-ADDRESS-TYPE-MISSING
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* Records in the embedded resolver for a name that are all A records, or all AAAA
    records

    A query of the other type for that name gets an empty :rcode:`NOERROR` response.

.. expectation:: An MX query for a known name gets an empty NOERROR response
    :id: DNS-NAMES-MX
    :observed_from: ctr, task
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-DNS-NAMES-QUERY-TYPES, REQ-DNS-NAMES-MX-IMPLICIT
    :within: SWARM-CEIL-REASSIGNMENT

    An MX query for a name that the embedded resolver has A or AAAA records for gets an empty
    :rcode:`NOERROR` response. The embedded resolver does not forward this query upstream.

.. expectation:: Other query types for a name with records are forwarded upstream
    :id: DNS-NAMES-QUERY-TYPE-FORWARDED
    :observed_from: ctr, task
    :depends_on: none
    :platforms: linux, windows
    :within: SWARM-CEIL-REASSIGNMENT
    :rationale: RAT-DNS-OTHER-TYPES-LEAVE-THE-NODE

    *Applies to:* A name that the embedded resolver has address records for, and a query type other
    than A, AAAA and MX for which the first of the querying container's upstream nameservers has a
    record for that name

    The embedded resolver forwards upstream a query of a type other than A, AAAA and MX for a name
    that it has address records for. The query gets that nameserver's answer.

.. expectation:: Outside a Swarm, A and AAAA answers come from the first network only
    :id: DNS-NAMES-NETWORK-ORDER
    :observed_from: ctr
    :depends_on: none
    :platforms: linux
    :exact_form_of: DNS-NAMES-ANY-NETWORK

    *Applies to:* A Linux node that is not in a Swarm

    When several of the querying container's networks have records for a name, the embedded resolver
    answers an A or AAAA query for that name from the records of the first network only. The order
    is: (1) Endpoints with a higher :api-field:`GwPriority` first. (2) Networks that are not
    ``internal`` first. (3) Endpoints that give the container a gateway or a default route for more
    IP versions first, whether or not the container's default route for a version goes through them.
    (4) Network names in byte order. Each step orders only the endpoints that the steps before it
    leave equal.

.. expectation:: Answers list all addresses unless the UDP size limit truncates them
    :id: DNS-NAMES-ANSWER-SIZE
    :observed_from: ctr, task
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-DNS-NAMES-ALIAS
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A network alias that the endpoints of several containers on one network share

    Over UDP, an answer from the embedded resolver's own records lists all of the alias's addresses
    on the network that it answers from when it fits in a size limit. The limit is the larger of 512
    bytes and the query's EDNS0 buffer size. Over TCP, the answer lists all of those addresses.

.. expectation:: An answer over the UDP size limit is truncated with the TC bit set
    :id: DNS-NAMES-ANSWER-TRUNCATED
    :observed_from: ctr, task
    :depends_on: none
    :platforms: linux, windows
    :within: SWARM-CEIL-REASSIGNMENT
    :rationale: RAT-DNS-RESPONSE-STEERS-CLIENT

    Over UDP, when an answer from the embedded resolver's own records does not fit in the larger of
    512 bytes and the query's EDNS0 buffer size, the embedded resolver truncates it and sets the TC
    bit.

.. expectation:: Answers vary which of a shared alias's addresses comes first
    :id: DNS-NAMES-ADDRESS-ORDER
    :observed_from: ctr, task
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-DNS-NAMES-ALIAS
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A network alias that the endpoints of several containers on one network share

    Each answer from the embedded resolver's own records lists the alias's addresses in an order
    chosen for that answer. For an alias with two addresses, each of them comes first in at least
    one of 20 consecutive answers.

.. expectation:: Answers from the embedded resolver's records have a TTL of 600 seconds
    :id: DNS-NAMES-TTL
    :observed_from: ctr, task
    :depends_on: none
    :platforms: linux, windows
    :within: SWARM-CEIL-REASSIGNMENT
    :rationale: RAT-DNS-TTL-STALE-ADDRESSES

    The records in an answer from the embedded resolver's own records have a TTL of 600 seconds.
    This holds for A, AAAA and PTR answers.

.. expectation:: Reverse lookups resolve the addresses of containers on shared networks
    :id: DNS-NAMES-PTR
    :observed_from: ctr, task
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-DNS-NAMES-QUERY-TYPES
    :within: SWARM-CEIL-REASSIGNMENT

    Reverse (PTR) lookups resolve for the IPv4 and IPv6 addresses of containers on the querying
    container's user-defined networks.

.. expectation:: A reverse lookup's <name>.<network> resolves forward to the address
    :id: DNS-NAMES-PTR-ROUND-TRIP
    :observed_from: ctr, task
    :depends_on: none
    :platforms: linux
    :refines: REQ-DNS-NAMES-DNSNAMES, REQ-DNS-NAMES-QUERY-TYPES
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A container's address whose reverse lookup returns :dns-name:`<name>.<network>.`,
    where no network of the querying container has records for :dns-name:`<name>.<network>`

    A forward lookup of :dns-name:`<name>.<network>` returns that address. This holds also when
    another of the querying container's networks has records for ``<name>``.

.. expectation:: Reverse lookups fail for IPv6 addresses with :: for 3+ groups at an end
    :id: DNS-NAMES-PTR-IPV6-ZERO-RUN
    :observed_from: ctr, task
    :depends_on: none
    :platforms: linux, windows
    :within: SWARM-CEIL-REASSIGNMENT
    :narrows: DNS-NAMES-PTR, DNS-NAMES-PTR-FORM
    :violates: REQ-DNS-NAMES-QUERY-TYPES

    *Applies to:* A container with an IPv6 address on a user-defined network that it shares with the
    querying container, where the address's text form (RFC 5952) starts or ends with ``::`` that
    stands for three or more groups of zeros, such as ``fd00:0:0:1:1::``

    The embedded resolver does not answer a PTR query for that address from its records. It handles
    the query as it handles a query for a name that it has no records for.

.. expectation:: A reverse lookup answers <name>.<network>. with the container name
    :id: DNS-NAMES-PTR-FORM
    :observed_from: ctr, task
    :depends_on: none
    :platforms: linux, windows
    :exact_form_of: DNS-NAMES-PTR
    :within: SWARM-CEIL-REASSIGNMENT

    The answer to a reverse lookup of a container's address on a user-defined network is
    :dns-name:`<name>.<network>.`, where ``<name>`` is the container's name and ``<network>`` is the
    network's name. It is never an alias, the short ID or the hostname.

.. expectation:: A reverse lookup matches only in lower case
    :id: DNS-NAMES-PTR-CASE-SENSITIVE
    :observed_from: ctr, task
    :depends_on: none
    :platforms: linux, windows
    :within: SWARM-CEIL-REASSIGNMENT
    :narrows: DNS-NAMES-PTR, DNS-NAMES-PTR-FORM
    :violates: REQ-DNS-NAMES-QUERY-TYPES

    The embedded resolver answers a PTR query for the address of a container on the querying
    container's networks from its records only when the query's :dns-name:`in-addr.arpa` or
    :dns-name:`ip6.arpa` suffix, and for an IPv6 address the hex digits of the name, are in lower
    case. It handles a PTR query with an upper-case letter in them as it handles a query for a name
    that it has no records for.

.. expectation:: <name>.<network> resolves to the addresses of <name> on that network
    :id: DNS-NAMES-NETWORK-QUALIFIED
    :observed_from: ctr, task
    :depends_on: none
    :platforms: linux
    :refines: REQ-DNS-NAMES-DNSNAMES
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A querying container attached to the network ``<network>``, and records for
    ``<name>`` on that network, where no network of the querying container has records for
    :dns-name:`<name>.<network>`

    A query for :dns-name:`<name>.<network>` resolves to the addresses of ``<name>`` on that network
    only. This holds also when another of the querying container's networks has records for
    ``<name>``.

.. expectation:: The network part of <name>.<network> matches only in its own case
    :id: DNS-NAMES-NETWORK-QUALIFIED-CASE-SENSITIVE
    :observed_from: ctr, task
    :depends_on: none
    :platforms: linux
    :within: SWARM-CEIL-REASSIGNMENT
    :narrows: DNS-NAMES-NETWORK-QUALIFIED
    :concerns: DNS-NAMES-CASE
    :violates: REQ-DNS-NAMES-CASE, REQ-DNS-NAMES-DNSNAMES

    In a :dns-name:`<name>.<network>` query, ``<network>`` matches a network only when its case is
    the same as the case of the network's name. The ``<name>`` part matches whatever its case.

.. expectation:: On Windows, <name>.<network> is not answered from <name>'s records
    :id: DNS-NAMES-NETWORK-QUALIFIED-WINDOWS
    :observed_from: ctr, task
    :depends_on: none
    :platforms: windows
    :concerns: DNS-NAMES-PTR-ROUND-TRIP, DNS-NAMES-NETWORK-QUALIFIED
    :violates: REQ-DNS-NAMES-DNSNAMES

    *Applies to:* A querying container attached to the network ``<network>``, and records for
    ``<name>`` on that network, where no network of the querying container has records for
    :dns-name:`<name>.<network>`, and a record of the query's type for :dns-name:`<name>.<network>`
    on the first of the querying container's upstream nameservers

    The embedded resolver on a Windows node does not answer a query for :dns-name:`<name>.<network>`
    from the records of ``<name>`` on ``<network>``. It forwards the query upstream. The query gets
    that nameserver's answer.

.. expectation:: <name>.<network> is forwarded when the querying container is not on it
    :id: DNS-NAMES-NETWORK-QUALIFIED-NOT-ATTACHED
    :observed_from: ctr, task
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-DNS-NAMES-NETWORK-SCOPE, REQ-DNS-FWD-EXTERNAL

    *Applies to:* A network whose name is a valid DNS label and that the querying container is not
    attached to, where no network of the querying container has records for
    :dns-name:`<name>.<network>`, ``<network>`` being that network's name, and a record of the
    query's type for :dns-name:`<name>.<network>` on the first of the querying container's upstream
    nameservers

    The embedded resolver forwards a query for :dns-name:`<name>.<network>` upstream. The query gets
    that nameserver's answer.

.. expectation:: An endpoint named <name>.<network> resolves by its whole name
    :id: DNS-NAMES-NETWORK-QUALIFIED-WHOLE-NAME
    :observed_from: ctr, task
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-DNS-NAMES-CTR-NAME, REQ-DNS-NAMES-ALIAS
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A querying container attached to the network ``<network>``, where an endpoint is
    named ``<name>``, and records for :dns-name:`<name>.<network>` on one of the querying
    container's networks

    :dns-name:`<name>.<network>` resolves to the addresses in the records for the whole name.

.. expectation:: An endpoint named <name>.<network> wins over <name> on <network>
    :id: DNS-NAMES-NETWORK-QUALIFIED-WHOLE-NAME-ONLY
    :observed_from: ctr, task
    :depends_on: none
    :platforms: linux, windows
    :exact_form_of: DNS-NAMES-NETWORK-QUALIFIED-WHOLE-NAME
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A querying container attached to the network ``<network>``, where an endpoint is
    named ``<name>``, and records for :dns-name:`<name>.<network>` on one of the querying
    container's networks

    :dns-name:`<name>.<network>` resolves only to the addresses in the records for the whole name.
    It does not resolve to the address of ``<name>`` on the network ``<network>``.

.. expectation:: On Windows, names on networks other than nat and overlay are not answered from records
    :id: DNS-NAMES-WINDOWS-OTHER-DRIVERS
    :observed_from: ctr, task
    :depends_on: none
    :platforms: windows
    :narrows: DNS-NAMES-CTR-NAME, DNS-NAMES-SHORT-ID-HOSTNAME, DNS-NAMES-ADDRESS-RECORDS, DNS-NAMES-CASE, DNS-NAMES-ALIAS, DNS-NAMES-ALIAS-SHARED, DNS-NAMES-DNSNAMES, DNS-NAMES-ANY-NETWORK, DNS-NAMES-START, DNS-NAMES-CONNECT, DNS-NAMES-RENAME, DNS-NAMES-AAAA, DNS-NAMES-ADDRESS-TYPE-MISSING, DNS-NAMES-ADDRESS-TYPE-MISSING-NOERROR, DNS-NAMES-MX, DNS-NAMES-ANSWER-SIZE, DNS-NAMES-ADDRESS-ORDER, DNS-NAMES-TTL, DNS-NAMES-PTR, DNS-NAMES-PTR-FORM, DNS-NAMES-NETWORK-QUALIFIED-WHOLE-NAME, DNS-NAMES-NETWORK-QUALIFIED-WHOLE-NAME-ONLY
    :violates: REQ-DNS-NAMES-CTR-NAME, REQ-DNS-NAMES-CASE, REQ-DNS-NAMES-ALIAS, REQ-DNS-NAMES-NETWORK-SCOPE, REQ-DNS-NAMES-DNSNAMES, REQ-DNS-NAMES-DYNAMIC, REQ-DNS-NAMES-QUERY-TYPES, REQ-DNS-NAMES-MX-IMPLICIT

    On a Windows node, the daemon gives the embedded resolver only to a container's endpoints on
    :driver:`overlay` networks, and on :driver:`nat` networks other than one created with
    :drvopt:`com.docker.network.windowsshim.disable_gatewaydns` set to true. On a user-defined
    network of another driver, such as :driver:`transparent` or :driver:`l2bridge`, a query from a
    container for the name of another container on the network is not answered from the daemon's
    records: a query for a network alias of another container on the network does not resolve to
    that container's address on the network.

.. expectation:: A link alias resolves to the linked container for the linking one only
    :id: DNS-NAMES-LINK-ALIAS
    :observed_from: ctr
    :depends_on: none
    :platforms: linux
    :refines: REQ-DNS-NAMES-LINK-ALIAS

    *Applies to:* Linux nodes with a container with ``<name>:<alias>`` in the :api-field:`Links` of
    its endpoint settings for a user-defined network

    The alias resolves to the named container's address on that network. It resolves only for
    queries from the container that has the link.

.. expectation:: A link alias matches only in its own case
    :id: DNS-NAMES-LINK-ALIAS-CASE
    :observed_from: ctr
    :depends_on: none
    :platforms: linux
    :narrows: DNS-NAMES-LINK-ALIAS
    :violates: REQ-DNS-NAMES-CASE, REQ-DNS-NAMES-LINK-ALIAS

    A query matches a link alias only when its case is the same as the case of the alias in
    :api-field:`Links`. The embedded resolver answers a query for the alias in another case as it
    answers a name that is not a link alias: from the records of the querying container's networks,
    or upstream.

.. expectation:: A link alias takes precedence over names on the querying container's networks
    :id: DNS-NAMES-LINK-ALIAS-PRECEDENCE
    :observed_from: ctr
    :depends_on: none
    :platforms: linux
    :rationale: RAT-DNS-LINK-ALIAS-PRECEDENCE

    *Applies to:* Linux nodes with a container with ``<name>:<alias>`` in the :api-field:`Links` of
    its endpoint settings for a user-defined network

    The alias takes precedence over the names and network aliases on each of that container's
    networks. It resolves to the named container even when another container on one of those
    networks has the alias as its name or as a network alias.

.. _container-dns-names-and-answers-in-a-swarm:

In a Swarm
==========

.. expectation:: In a Swarm, a rename applies on a multi-host network on its own node
    :id: DNS-NAMES-RENAME-SWARM
    :observed_from: ctr
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-DNS-NAMES-DYNAMIC, REQ-SWARM-SD-CONTAINER-NAME

    *Applies to:* A node in a Swarm, and a running container whose hostname and network aliases
    differ from its name in more than the case of their letters, with an endpoint on a Swarm-scoped
    network of a multi-host driver, such as :driver:`overlay`

    After :api-op:`POST /containers/{id}/rename` for the container, with a new name that differs
    from its old name in more than the case of its letters, responds 204, its new name resolves and
    its old name does not resolve to its address, for containers on that network on its node.

.. expectation:: On a Swarm node, a rename leaves the records on other networks unchanged
    :id: DNS-NAMES-RENAME-SWARM-NODE
    :observed_from: ctr
    :depends_on: none
    :platforms: linux, windows
    :narrows: DNS-NAMES-CTR-NAME, DNS-NAMES-DNSNAMES, DNS-NAMES-PTR-FORM, DNS-NAMES-RENAME, DNS-NAMES-ANY-NETWORK, DNS-NAMES-CASE, DNS-NAMES-ADDRESS-RECORDS, DNS-NAMES-AAAA, DNS-NAMES-MX, DNS-NAMES-ADDRESS-TYPE-MISSING, DNS-NAMES-ADDRESS-TYPE-MISSING-NOERROR, DNS-NAMES-NETWORK-QUALIFIED, DNS-FWD-UNKNOWN-NAME, DNS-NAMES-SHORT-ID-HOSTNAME, DNS-RESOLV-IPV6-ONLY, DNS-RESOLV-CTR-NAME-BEFORE-SEARCH, DNS-FWD-SINGLE-LABEL, DNS-FWD-SINGLE-LABEL-NDOTS, DNS-FWD-SINGLE-LABEL-NDOTS-NOERROR, DNS-FWD-PUBLIC-NAMES, DNS-FWD-INTERNAL-SERVFAIL, DNS-FWD-NAMESERVER-SELF-ONLY
    :concerns: DNS-NAMES-RENAME-SWARM
    :violates: REQ-DNS-NAMES-DYNAMIC, PRIN-NO-LEAKS, REQ-DNS-NAMES-CTR-NAME, REQ-DNS-NAMES-DNSNAMES, REQ-DNS-NAMES-CASE, REQ-DNS-NAMES-NETWORK-SCOPE, REQ-DNS-NAMES-QUERY-TYPES, REQ-DNS-NAMES-MX-IMPLICIT, REQ-DNS-FWD-EXTERNAL, REQ-DNS-EMBEDDED-RESOLVER, REQ-DNS-NAMES-BEFORE-SEARCH-DOMAINS, REQ-DNS-FWD-SINGLE-LABEL-NDOTS

    On a node in a Swarm, :api-op:`POST /containers/{id}/rename` changes the container's records
    only on Swarm-scoped networks of multi-host drivers, such as :driver:`overlay`. It does not
    change them on other networks, such as user-defined bridge networks and Swarm-scoped networks of
    node-local drivers. The container's new name does not resolve there. Its old name continues to
    resolve there after the rename, and a reverse lookup of its address there returns the old name.
    The old name continues to resolve there after the container is removed.

.. expectation:: After a Swarm leave, running containers' names stop resolving
    :id: DNS-NAMES-SWARM-LEAVE-RECORDS
    :observed_from: ctr
    :depends_on: none
    :platforms: linux, windows
    :within: SWARM-CEIL-REJOIN
    :narrows: DNS-NAMES-CTR-NAME, DNS-NAMES-ALIAS, DNS-NAMES-PTR, DNS-NAMES-CASE, DNS-NAMES-ALIAS-SHARED, DNS-NAMES-SHORT-ID-HOSTNAME, DNS-NAMES-DNSNAMES, DNS-NAMES-ANY-NETWORK, DNS-NAMES-ADDRESS-RECORDS, DNS-NAMES-AAAA, DNS-NAMES-NETWORK-QUALIFIED, DNS-NAMES-LINK-ALIAS, DNS-RESOLV-IPV6-ONLY, DNS-NAMES-MX, DNS-NAMES-ADDRESS-TYPE-MISSING, DNS-NAMES-ADDRESS-TYPE-MISSING-NOERROR, DNS-NAMES-PTR-FORM, DNS-NAMES-NETWORK-ORDER, DNS-NAMES-NETWORK-QUALIFIED-WHOLE-NAME, DNS-NAMES-NETWORK-QUALIFIED-WHOLE-NAME-ONLY, DNS-NAMES-LINK-ALIAS-PRECEDENCE, DNS-RESOLV-CTR-NAME-BEFORE-SEARCH
    :violates: REQ-DNS-NAMES-CTR-NAME, REQ-DNS-NAMES-QUERY-TYPES, REQ-DNS-NAMES-LINK-ALIAS, REQ-DNS-NAMES-ALIAS, REQ-DNS-NAMES-NETWORK-SCOPE, REQ-DNS-NAMES-DNSNAMES, REQ-DNS-NAMES-CASE, REQ-DNS-EMBEDDED-RESOLVER, REQ-DNS-NAMES-MX-IMPLICIT, REQ-DNS-NAMES-BEFORE-SEARCH-DOMAINS

    Take a node in a Swarm, whether it created the Swarm with :api-op:`POST /swarm/init` or joined
    it, that leaves it with :api-op:`POST /swarm/leave`. After the leave, the names of the
    containers that were running on the node's user-defined networks before the leave stop resolving
    there, and so do reverse lookups of their addresses. The embedded resolver handles a query of
    any type for one of those names, and a PTR query for one of their addresses, as it handles a
    query for a name that no endpoint on the querying container's networks has. Link aliases for
    those containers stop resolving too. A container that starts after the leave gets records that
    resolve.

.. _container-dns-forwarding:

************
 Forwarding
************

.. expectation:: Names without a record resolve through the host's nameservers
    :id: DNS-FWD-PUBLIC-NAMES
    :observed_from: ctr, task
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-DNS-FWD-EXTERNAL
    :needs: DNS-HOST-EXTERNAL-ACCESS

    *Applies to:* A container that has external access and was created without :cli-flag:`--dns`
    nameservers, whose daemon does not have a :daemon-json:`dns` setting

    A name that the embedded resolver has no record for resolves to the addresses that the host's
    nameservers give for it, such as a public DNS name.

.. expectation:: An internal-only container never queries unreachable or host nameservers
    :id: DNS-FWD-INTERNAL
    :observed_from: ctr, node, task
    :depends_on: none
    :platforms: linux
    :refines: REQ-DNS-FWD-INTERNAL

    *Applies to:* Linux nodes with a container attached only to ``internal`` networks

    No nameserver that the container cannot reach from its own network namespace receives the
    container's queries, such as a nameserver set with :cli-flag:`--dns` or in the daemon's
    :daemon-json:`dns` setting at an address that the node reaches and the container does not. The
    embedded resolver also does not forward them to the nameservers in the host's
    :file:`resolv.conf`.

.. expectation:: Unknown names get SERVFAIL in an internal-only container without --dns
    :id: DNS-FWD-INTERNAL-SERVFAIL
    :observed_from: ctr, task
    :depends_on: none
    :platforms: linux
    :exact_form_of: DNS-FWD-INTERNAL

    *Applies to:* Linux nodes with a container attached only to ``internal`` networks and created
    without :cli-flag:`--dns` nameservers, whose daemon does not have a :daemon-json:`dns` setting

    A query for a name that the embedded resolver does not have a record for gets :rcode:`SERVFAIL`.
    When the options in effect set :resolv-opt:`ndots` to 1 or more, a single-label A or AAAA query
    gets an empty :rcode:`NOERROR` response instead.

.. expectation:: The embedded resolver forwards to the --dns nameservers
    :id: DNS-FWD-CTR-NAMESERVERS
    :observed_from: ctr, task
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-DNS-FWD-UPSTREAM-SERVERS

    *Applies to:* A container that has external access and was created with :cli-flag:`--dns`
    nameservers, and a name that the embedded resolver does not have a record for, for which the
    first nameserver set with :cli-flag:`--dns` other than the embedded resolver's own address and
    the host's nameservers give different addresses

    The embedded resolver forwards to the nameservers set with :cli-flag:`--dns`, other than its own
    address (``127.0.0.11`` on a Linux node, the network's gateway address on a Windows node), in
    place of the host's nameservers. A query for that name resolves to the address that the first of
    those :cli-flag:`--dns` nameservers gives.

.. expectation:: Without --dns, the embedded resolver forwards to the daemon's dns
    :id: DNS-FWD-DAEMON-NAMESERVERS
    :observed_from: ctr, task
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-DNS-FWD-UPSTREAM-SERVERS

    *Applies to:* A daemon with a :daemon-json:`dns` setting, and a container that has external
    access and was created without :cli-flag:`--dns` nameservers, and a name that the embedded
    resolver does not have a record for, for which the first nameserver in the daemon's
    :daemon-json:`dns` setting other than the embedded resolver's own address and the host's
    nameservers give different addresses

    The embedded resolver forwards to the nameservers in the daemon's :daemon-json:`dns` setting,
    other than its own address (``127.0.0.11`` on a Linux node, the network's gateway address on a
    Windows node), in place of the host's nameservers. A query for that name resolves to the address
    that the first of those nameservers gives.

.. expectation:: An internal-only container resolves through a --dns nameserver
    :id: DNS-FWD-INTERNAL-CTR-NAMESERVER
    :observed_from: ctr, task
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-DNS-FWD-INTERNAL

    *Applies to:* A container attached only to ``internal`` networks and created with
    :cli-flag:`--dns` set to the address of a nameserver in another container on one of those
    networks

    The container created with :cli-flag:`--dns` resolves names through that nameserver.

.. expectation:: An internal-only container resolves through the daemon's dns nameserver
    :id: DNS-FWD-INTERNAL-DAEMON-NAMESERVER
    :observed_from: ctr, task
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-DNS-FWD-INTERNAL

    *Applies to:* A daemon with a :daemon-json:`dns` setting that holds the address of a nameserver
    in a container on an ``internal`` network, and another container attached only to ``internal``
    networks, that network among them, and created without :cli-flag:`--dns` nameservers

    The container created without :cli-flag:`--dns` nameservers resolves names through that
    nameserver.

.. expectation:: On Windows, internal endpoints do not get --dns or dns nameservers
    :id: DNS-FWD-WINDOWS-INTERNAL-NAMESERVERS
    :observed_from: ctr, task
    :depends_on: none
    :platforms: windows
    :concerns: DNS-FWD-INTERNAL-CTR-NAMESERVER, DNS-FWD-INTERNAL-DAEMON-NAMESERVER
    :violates: REQ-DNS-FWD-INTERNAL, REQ-DNS-FWD-UPSTREAM-SERVERS

    A Windows endpoint on an ``internal`` network does not get the nameservers set with
    :cli-flag:`--dns` or in the daemon's :daemon-json:`dns` setting. A Windows container attached
    only to ``internal`` networks does not resolve names through a nameserver on one of them.

.. expectation:: Without dns settings, the host's nameservers are queried from the host
    :id: DNS-FWD-HOST-NAMESERVERS
    :observed_from: ctr, task
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-DNS-FWD-UPSTREAM-SERVERS, REQ-DNS-FWD-NAMESPACE
    :needs: DNS-HOST-EXTERNAL-ACCESS

    *Applies to:* Linux nodes with a host :file:`resolv.conf` that names a nameserver by a valid IP
    address, and a container that started with an endpoint on a user-defined network, has external
    access and was created without :cli-flag:`--dns` nameservers, whose daemon does not have a
    :daemon-json:`dns` setting

    The embedded resolver forwards to the nameservers in the host's :file:`resolv.conf`, loopback
    nameservers included. It queries them from the host's network namespace.

.. expectation:: With NetworkMode bridge on a systemd-resolved host, queries skip 127.0.0.53
    :id: DNS-FWD-DEFAULT-BRIDGE-CONNECT-UPSTREAM
    :observed_from: ctr, task
    :depends_on: os-linux
    :platforms: linux
    :narrows: DNS-FWD-HOST-NAMESERVERS, DNS-RESOLV-DEFAULT-BRIDGE-CONNECT
    :violates: REQ-DNS-FWD-UPSTREAM-SERVERS

    *Applies to:* A Linux node whose :file:`/etc/resolv.conf` named only ``127.0.0.53`` when the
    daemon started, and a container that was created without :cli-flag:`--dns` nameservers, whose
    daemon does not have a :daemon-json:`dns` setting, that has an endpoint on a user-defined
    network, from its start, or after a connect while its :file:`/etc/resolv.conf` had not changed
    since the daemon wrote it, and whose :api-field:`HostConfig.NetworkMode` is the default bridge,
    such as a container that :api-op:`POST /containers/create` created without a
    :api-field:`NetworkMode`, even when its :api-field:`EndpointsConfig` names only a user-defined
    network, or the container of a service task whose first network is not one of the Swarm's
    predefined networks

    When :file:`/run/systemd/resolve/resolv.conf` names a nameserver by a valid IP address, the
    embedded resolver forwards the container's queries to the nameservers in that file, not to
    ``127.0.0.53``. When that file does not name a nameserver by a valid IP address, or does not
    exist, the embedded resolver forwards the container's queries to ``127.0.0.1`` and ``::1``, and
    queries them from the host's network namespace. Then, when a nameserver that listens on port 53
    at ``127.0.0.1`` or at ``::1`` in the host's network namespace has a record for a name that the
    embedded resolver does not have a record for, a query from the container for that name resolves
    to that record's address.

.. expectation:: With no host nameserver, the embedded resolver uses 127.0.0.1 and ::1
    :id: DNS-FWD-HOST-NAMESERVERS-NONE
    :observed_from: ctr, task
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-DNS-FWD-UPSTREAM-SERVERS, REQ-DNS-FWD-NAMESPACE

    *Applies to:* Linux nodes with a host :file:`resolv.conf` that does not name a nameserver by a
    valid IP address, because the file has no :resolv-opt:`nameserver` line with one or does not
    exist, a broken symlink included, and a container that has external access, that was created
    without :cli-flag:`--dns` nameservers, whose daemon does not have a :daemon-json:`dns` setting,
    and that started with an endpoint on a user-defined network or started attached only to the
    default bridge and was then connected to a user-defined network while its
    :file:`/etc/resolv.conf` had not changed since the daemon wrote it, and a nameserver that
    listens on port 53 at ``127.0.0.1`` or at ``::1`` in the host's network namespace and has a
    record for a name that the embedded resolver does not have a record for

    The embedded resolver forwards to ``127.0.0.1`` and ``::1``. It queries them from the host's
    network namespace: a query from the container for that name resolves to that record's address.

.. expectation:: A host resolv.conf that is a directory or a socket fails the container's start
    :id: DNS-FWD-HOST-RESOLV-NOT-REGULAR
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :rationale: RAT-DNS-HOST-RESOLV-UNREADABLE

    *Applies to:* A Linux node whose :file:`/etc/resolv.conf` is a directory or a socket, and a
    created or stopped container that uses the embedded resolver

    :api-op:`POST /containers/{id}/start` for the container responds with a 4xx or 5xx status, with
    a message that contains :api-error:`is a directory` when the file is a directory, and
    :api-error:`no such device or address` when it is a socket.

.. expectation:: A host resolv.conf that is a FIFO holds the daemon's start
    :id: DNS-FWD-HOST-RESOLV-FIFO
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :rationale: RAT-DNS-HOST-RESOLV-UNREADABLE

    *Applies to:* A Linux node whose :file:`/etc/resolv.conf` is a FIFO

    When the daemon starts, its API does not respond before a process opens the FIFO for writing.
    Once a process has written a resolv.conf to the FIFO and closed it each time the daemon opens
    the FIFO, the daemon completes its start and its API responds.

.. expectation:: --dns nameservers are queried from the container's namespace
    :id: DNS-FWD-CONFIGURED-NAMESERVERS-CTR-NS
    :observed_from: ctr, task
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-DNS-FWD-NAMESPACE

    *Applies to:* A container whose only :cli-flag:`--dns` nameserver is ``127.0.0.1``, a nameserver
    that listens on port 53 at ``127.0.0.1`` in the container's network namespace, and one that
    listens on port 53 at ``127.0.0.1`` in the host's network namespace, which give different
    addresses for a name that the embedded resolver does not have a record for

    The embedded resolver queries nameservers set with :cli-flag:`--dns` from the container's
    network namespace: a query from the container for that name resolves to the address that the
    nameserver in the container's network namespace gives.

.. expectation:: The daemon's dns nameservers are queried from the container's namespace
    :id: DNS-FWD-DAEMON-NAMESERVERS-CTR-NS
    :observed_from: ctr, task
    :depends_on: os-linux
    :platforms: linux
    :rationale: RAT-DNS-DAEMON-NAMESERVERS-NAMESPACE

    *Applies to:* A daemon whose :daemon-json:`dns` setting has the only nameserver ``127.0.0.1``, a
    container created without :cli-flag:`--dns` nameservers, a nameserver that listens on port 53 at
    ``127.0.0.1`` in the container's network namespace, and one that listens on port 53 at
    ``127.0.0.1`` in the host's network namespace, which give different addresses for a name that
    the embedded resolver does not have a record for

    The embedded resolver queries the nameservers in the daemon's :daemon-json:`dns` setting from
    the container's network namespace: a query from the container for that name resolves to the
    address that the nameserver in the container's network namespace gives.

.. expectation:: After a live restore, the embedded resolver forwards to the same servers
    :id: DNS-FWD-LIVE-RESTORE-UPSTREAM
    :observed_from: ctr
    :depends_on: none
    :platforms: linux
    :refines: REQ-DNS-LIVE-RESTORE-RESOLVER
    :needs: DNS-HOST-EXTERNAL-ACCESS

    *Applies to:* A running container that uses the embedded resolver, created without
    :cli-flag:`--dns` nameservers, on a Linux node whose daemon has :daemon-json:`live-restore` true
    and no :daemon-json:`dns` setting, and a name of two or more labels that the embedded resolver
    does not have a record for and that the nameservers in the host's :file:`/etc/resolv.conf` have
    records for

    The daemon stops, the nameservers in the host's :file:`/etc/resolv.conf` are replaced with
    others that have records for that name with other addresses, and the daemon starts again. The
    embedded resolver then forwards a query for that name to the upstream servers that it forwarded
    to before the restart, and not to the nameservers now in the host's :file:`resolv.conf`: the
    name resolves to the addresses that the replaced nameservers give.

.. expectation:: No response, SERVFAIL or REFUSED moves the query to the next nameserver
    :id: DNS-FWD-FAILOVER
    :observed_from: ctr, task
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-DNS-FWD-ORDER, REQ-DNS-FWD-REFUSED

    *Applies to:* A container with two or more upstream nameservers, and a query for a name that the
    embedded resolver does not have a record for, which the first of those nameservers does not
    respond to, or answers with :rcode:`SERVFAIL` or :rcode:`REFUSED`, and for which the second has
    a record of the query's type

    The embedded resolver forwards a query to the next nameserver when the current nameserver does
    not respond, or responds :rcode:`SERVFAIL` or :rcode:`REFUSED`. The query gets the second
    nameserver's answer.

.. expectation:: When every nameserver fails, the client gets SERVFAIL
    :id: DNS-FWD-ALL-FAILED-SERVFAIL
    :observed_from: ctr, task
    :depends_on: none
    :platforms: linux, windows
    :rationale: RAT-DNS-RESPONSE-STEERS-CLIENT

    When each nameserver that the embedded resolver forwards to does not respond, or responds
    :rcode:`SERVFAIL` or :rcode:`REFUSED`, the client gets :rcode:`SERVFAIL`.

.. expectation:: Over UDP, SERVFAIL comes after 4 seconds per silent nameserver
    :id: DNS-FWD-FAILOVER-TIMEOUT
    :observed_from: ctr, task
    :depends_on: none
    :platforms: linux, windows
    :exact_form_of: DNS-FWD-ALL-FAILED-SERVFAIL

    Over UDP, when each nameserver that the embedded resolver forwards a query to gives no response
    at all, neither a DNS response nor an ICMP error, the embedded resolver responds
    :rcode:`SERVFAIL` to the client after 4 seconds for each of those nameservers.

.. expectation:: A successful or NXDOMAIN response goes to the client
    :id: DNS-FWD-RESPONSE-RETURNED
    :observed_from: ctr, task
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-DNS-FWD-ORDER

    *Applies to:* A container with two or more upstream nameservers, where the second has a
    different answer for the query than the first

    The embedded resolver returns a nameserver's successful or :rcode:`NXDOMAIN` response to the
    client. It does not ask the next nameserver.

.. expectation:: Any other rcode but SERVFAIL and REFUSED goes to the client
    :id: DNS-FWD-OTHER-RCODE-RETURNED
    :observed_from: ctr, task
    :depends_on: none
    :platforms: linux, windows
    :rationale: RAT-DNS-RESPONSE-STEERS-CLIENT

    *Applies to:* A container with two or more upstream nameservers, where the second has a
    different answer for the query than the first

    The embedded resolver returns a nameserver's response with an rcode other than :rcode:`NOERROR`,
    :rcode:`NXDOMAIN`, :rcode:`SERVFAIL` and :rcode:`REFUSED` to the client. It does not ask the
    next nameserver.

.. expectation:: The embedded resolver forwards to at most the first three nameservers
    :id: DNS-FWD-NAMESERVER-LIMIT
    :observed_from: ctr, task
    :depends_on: none
    :platforms: linux, windows
    :exact_form_of: DNS-FWD-FAILOVER

    *Applies to:* A container with upstream nameservers, set with :cli-flag:`--dns`, in the daemon's
    :daemon-json:`dns` setting or in the host's :file:`resolv.conf`, of which four or more are not a
    nameserver ``127.0.0.11`` set with :cli-flag:`--dns` or in the daemon's :daemon-json:`dns`
    setting, where the first three of those four or more do not respond to the query

    The embedded resolver forwards the query to each of those first three, and not to the fourth of
    them or to any later one.

.. expectation:: A --dns nameserver 127.0.0.11 is skipped
    :id: DNS-FWD-NAMESERVER-SELF
    :observed_from: ctr, task
    :depends_on: none
    :platforms: linux
    :exact_form_of: DNS-FWD-CTR-NAMESERVERS

    *Applies to:* A container whose nameservers set with :cli-flag:`--dns` are ``127.0.0.11`` and
    then a nameserver that has a record for a name that the embedded resolver does not have a record
    for

    The embedded resolver does not forward to a nameserver ``127.0.0.11`` set with
    :cli-flag:`--dns`. A query for that name that the container sends once to ``127.0.0.11`` gets
    the other nameserver's answer, and the container's loopback interface carries no other DNS query
    from when the container sends it until the container gets the answer.

.. expectation:: A daemon dns nameserver 127.0.0.11 is skipped
    :id: DNS-FWD-NAMESERVER-SELF-DAEMON
    :observed_from: ctr, task
    :depends_on: none
    :platforms: linux
    :exact_form_of: DNS-FWD-DAEMON-NAMESERVERS

    *Applies to:* A daemon whose :daemon-json:`dns` setting has ``127.0.0.11`` and then a nameserver
    that has a record for a name that the embedded resolver does not have a record for, and a
    container created without :cli-flag:`--dns` nameservers

    The embedded resolver does not forward to a nameserver ``127.0.0.11`` in the daemon's
    :daemon-json:`dns` setting. A query for that name that the container sends once to
    ``127.0.0.11`` gets the other nameserver's answer, and the container's loopback interface
    carries no other DNS query from when the container sends it until the container gets the answer.

.. expectation:: With 127.0.0.11 as the only nameserver, unknown names get SERVFAIL
    :id: DNS-FWD-NAMESERVER-SELF-ONLY
    :observed_from: ctr, task
    :depends_on: none
    :platforms: linux
    :rationale: RAT-DNS-NO-UPSTREAM-LEFT

    *Applies to:* Linux nodes with a container whose only nameserver, set with :cli-flag:`--dns` or
    in the daemon's :daemon-json:`dns` setting, is ``127.0.0.11``

    A query for a name that the embedded resolver does not have a record for gets :rcode:`SERVFAIL`.
    When the options in effect set :resolv-opt:`ndots` to 1 or more, a single-label A or AAAA query
    gets an empty :rcode:`NOERROR` response instead.

.. expectation:: A query for a name without records is forwarded upstream
    :id: DNS-FWD-UNKNOWN-NAME
    :observed_from: ctr, task
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-DNS-FWD-EXTERNAL

    *Applies to:* A name and a query type for which the first of the querying container's upstream
    nameservers has a record

    The embedded resolver forwards upstream a query for a name that it does not have records for on
    the querying container's networks, whatever the type of the query, other than an A, AAAA or MX
    query for :dns-name:`<name>.<network>` where the querying container is attached to a network
    named ``<network>`` and the embedded resolver has records for ``<name>`` on it, and other than a
    PTR query for the reverse-lookup name of an address of an endpoint on those networks that has
    names. A single-label A or AAAA query is the exception when the options in effect, which the
    :ref:`container-dns-file-resolv-conf` section defines, set :resolv-opt:`ndots` to 1 or more. A
    query for such a name and type gets that nameserver's answer.

.. expectation:: An unknown multi-label A or AAAA query is forwarded upstream
    :id: DNS-FWD-MULTI-LABEL
    :observed_from: ctr, task
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-DNS-FWD-EXTERNAL

    *Applies to:* A multi-label name that does not match any container, service or alias on the
    querying container's networks, other than :dns-name:`<name>.<network>` where the querying
    container is attached to a network named ``<network>`` and the embedded resolver has records for
    ``<name>`` on it, and that the first of the querying container's upstream nameservers has A and
    AAAA records for

    The embedded resolver forwards an A or AAAA query for the name upstream. The query gets that
    nameserver's answer.

.. expectation:: An unknown single-label name is forwarded unless ndots is 1 or more
    :id: DNS-FWD-SINGLE-LABEL
    :observed_from: ctr, task
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-DNS-FWD-EXTERNAL

    *Applies to:* A container on a Windows node, or a container on a Linux node whose options in
    effect, which the :ref:`container-dns-file-resolv-conf` section defines, do not set
    :resolv-opt:`ndots` to 1 or more, and a single-label name that the embedded resolver does not
    have a record for and that the first of the container's upstream nameservers has A and AAAA
    records for

    The embedded resolver forwards a single-label A or AAAA query upstream when it does not have a
    record for the name. The query gets that nameserver's answer.

.. expectation:: With ndots of 1 or more, an unknown single-label name is not forwarded
    :id: DNS-FWD-SINGLE-LABEL-NDOTS
    :observed_from: ctr, task
    :depends_on: none
    :platforms: linux
    :refines: REQ-DNS-FWD-SINGLE-LABEL-NDOTS

    *Applies to:* A container on a Linux node whose options in effect, which the
    :ref:`container-dns-file-resolv-conf` section defines, set :resolv-opt:`ndots` to 1 or more, and
    an upstream nameserver of the container that has A and AAAA records for a single-label name that
    the embedded resolver does not have a record for

    A single-label A or AAAA query for a name that the embedded resolver does not have a record for
    gets a response without an address. The embedded resolver does not forward the query upstream.

.. expectation:: With ndots of 1 or more, an unknown single-label name gets an empty NOERROR
    :id: DNS-FWD-SINGLE-LABEL-NDOTS-NOERROR
    :observed_from: ctr, task
    :depends_on: none
    :platforms: linux
    :exact_form_of: DNS-FWD-SINGLE-LABEL-NDOTS

    *Applies to:* A container on a Linux node whose options in effect, which the
    :ref:`container-dns-file-resolv-conf` section defines, set :resolv-opt:`ndots` to 1 or more

    A single-label A or AAAA query for a name that the embedded resolver does not have a record for
    gets an empty :rcode:`NOERROR` response.

.. expectation:: With ndots 0 or invalid, an unknown single-label name gets an empty NOERROR
    :id: DNS-FWD-NDOTS-ZERO-SINGLE-LABEL
    :observed_from: ctr, task
    :depends_on: none
    :platforms: linux
    :narrows: DNS-FWD-SINGLE-LABEL, DNS-FWD-UNKNOWN-NAME, DNS-FWD-INTERNAL-SERVFAIL, DNS-FWD-NAMESERVER-SELF-ONLY
    :concerns: DNS-FWD-SINGLE-LABEL-NDOTS, DNS-RESOLV-NDOTS-ZERO
    :violates: REQ-DNS-FWD-EXTERNAL

    When the options in effect set :resolv-opt:`ndots` to 0 or to an invalid value, a single-label A
    or AAAA query gets an empty :rcode:`NOERROR` response when the embedded resolver does not have a
    record for the name, as it does for an :resolv-opt:`ndots` of 1 or more. The embedded resolver
    does not forward the query upstream (`moby/moby#53104
    <https://github.com/moby/moby/issues/53104>`__).

.. expectation:: After a live restore, ndots no longer affects unknown single-label names
    :id: DNS-FWD-LIVE-RESTORE-SINGLE-LABEL
    :observed_from: ctr
    :depends_on: none
    :platforms: linux
    :narrows: DNS-FWD-SINGLE-LABEL-NDOTS, DNS-FWD-SINGLE-LABEL-NDOTS-NOERROR, DNS-FWD-INTERNAL-SERVFAIL, DNS-FWD-NAMESERVER-SELF-ONLY, DNS-FWD-NDOTS-ZERO-SINGLE-LABEL
    :concerns: DNS-FWD-SINGLE-LABEL, DNS-FWD-UNKNOWN-NAME
    :violates: REQ-DNS-LIVE-RESTORE-RESOLVER, REQ-DNS-FWD-SINGLE-LABEL-NDOTS

    After the daemon restarts with :daemon-json:`live-restore` true, the embedded resolver handles a
    single-label A or AAAA query that it does not have a record for, from a container that kept
    running, as it handles a query for a name of two or more labels, whatever :resolv-opt:`ndots`
    the options in effect set. It forwards the query upstream. When it has no upstream server to
    forward to, as for a container attached only to ``internal`` networks and created without
    :cli-flag:`--dns` nameservers, whose daemon does not have a :daemon-json:`dns` setting, or a
    container whose only nameserver, set with :cli-flag:`--dns` or in the daemon's
    :daemon-json:`dns` setting, is ``127.0.0.11``, the query gets :rcode:`SERVFAIL`.

.. expectation:: With NetworkMode bridge, the resolver takes ndots from systemd's resolv.conf
    :id: DNS-FWD-DEFAULT-BRIDGE-SYSTEMD-NDOTS
    :observed_from: ctr, task
    :depends_on: os-linux
    :platforms: linux
    :narrows: DNS-FWD-SINGLE-LABEL-NDOTS, DNS-FWD-SINGLE-LABEL-NDOTS-NOERROR, DNS-FWD-NDOTS-ZERO-SINGLE-LABEL, DNS-FWD-INTERNAL-SERVFAIL, DNS-FWD-NAMESERVER-SELF-ONLY, DNS-FWD-SINGLE-LABEL
    :concerns: DNS-FWD-UNKNOWN-NAME
    :violates: REQ-DNS-FWD-SINGLE-LABEL-NDOTS

    *Applies to:* A Linux node whose :file:`/etc/resolv.conf` named only ``127.0.0.53`` when the
    daemon started, where only one of that file and :file:`/run/systemd/resolve/resolv.conf` sets an
    :resolv-opt:`ndots`, and a container whose DNS options are empty, that has an endpoint on a
    user-defined network, from its start or after a connect, and whose
    :api-field:`HostConfig.NetworkMode` is the default bridge, such as a container that
    :api-op:`POST /containers/create` created without a :api-field:`NetworkMode`, even when its
    :api-field:`EndpointsConfig` names only a user-defined network, or the container of a service
    task whose first network is not one of the Swarm's predefined networks

    Where only :file:`/etc/resolv.conf` sets an :resolv-opt:`ndots`, the embedded resolver handles a
    single-label A or AAAA query that it does not have a record for as it handles a query for a name
    of two or more labels. Where only :file:`/run/systemd/resolve/resolv.conf` sets one, the
    embedded resolver does not forward such a query upstream, whatever value that file sets.

.. _container-dns-port-53-in-the-container:

**************************
 Port 53 in the container
**************************

This section holds on Linux nodes.

.. expectation:: A process binds port 53 while 127.0.0.11 still reaches the resolver
    :id: DNS-PORT53-BIND
    :observed_from: ctr, task
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-DNS-PORT53-SERVER

    A process in the container binds UDP and TCP port 53 on the wildcard address, or on
    ``127.0.0.11``, without an error. A query that the container sends to ``127.0.0.11`` does not
    reach that process. The embedded resolver handles the query.

.. expectation:: Without the iptables command, queries to 127.0.0.11 get no answer
    :id: DNS-PORT53-NO-IPTABLES
    :observed_from: ctr
    :depends_on: os-linux
    :platforms: linux
    :pins: DNS-HOST-IPTABLES-COMMAND

    *Applies to:* A daemon with the iptables backend and :daemon-json:`iptables` false, on a host
    without the :program:`iptables` command, and a running container with an endpoint on a
    user-defined network

    A DNS query from the container to ``127.0.0.11`` does not get an answer.

.. expectation:: Without the iptables command, the daemon logs the DNAT/SNAT failure
    :id: DNS-PORT53-NO-IPTABLES-LOG
    :observed_from: log
    :depends_on: none
    :platforms: linux
    :pins: DNS-HOST-IPTABLES-COMMAND

    *Applies to:* A daemon with the iptables backend and :daemon-json:`iptables` false, on a host
    without the :program:`iptables` command, and a running container with an endpoint on a
    user-defined network

    The daemon log contains :log:`setting up DNAT/SNAT rules failed`.

.. _container-dns-deliberately-not-asserted:

***************************
 Deliberately not asserted
***************************

- The comments the daemon writes into :file:`resolv.conf`, such as its header and its record of
  where :resolv-opt:`ndots` came from.
- The ports that the embedded resolver listens on.
- The firewall rules that the daemon programs in a container's network namespace: whether they
  exist, their content and names, and the hooks and priorities of their base chains.
