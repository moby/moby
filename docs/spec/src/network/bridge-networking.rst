################################
 Bridge networking surface area
################################

Scope: the bridge network driver, and the parts of the daemon's firewall integration that are not
specific to Swarm. This spec describes them as observable from outside the daemon. Every expectation
here holds on Linux nodes. An expectation about :api-op:`POST /networks/create` or :cli:`docker
network create` holds for a network that is not Swarm-scoped. For a Swarm-scoped network, the node
in an expectation is the node of the container that the expectation names. An expectation about two
containers on a Swarm-scoped network holds for containers on the same node. Each node has a bridge
of its own, without a route to the bridges on other nodes, and their subnets can overlap. An
expectation about reaching a published port at an address of the node holds on a network whose
gateway mode for that address's family is not ``routed``, and whose
:drvopt:`com.docker.network.bridge.host_binding_ipv4` is unset or ``0.0.0.0``, unless the
expectation names them. An expectation about IPv4 traffic through the network's gateway, such as
egress, a published port or a default route, holds for a network without
:drvopt:`com.docker.network.bridge.inhibit_ipv4`, unless the expectation names it. An expectation
about egress from a container on a network, or from a network's containers, holds for a container
whose default route for that traffic's IP version is through that network. An expectation about
reaching a container's published ports at the container's own address on a network whose gateway
mode for that address's family is ``nat`` or not set holds for a container whose default route for
that IP version is through that network. :ref:`swarm-networking-swarm-scoped-networks` in
:doc:`swarm-networking.md <swarm-networking>` covers what is specific to Swarm-scoped bridge
networks, and :ref:`swarm-traffic-gateway-network` in :doc:`swarm-traffic.md <swarm-traffic>` covers
what is specific to :network:`docker_gwbridge`.

This spec holds expectations that belong to the bridge driver. This spec is not yet a complete
account of the bridge driver. :doc:`../README.md <../README>` gives the model that this spec
follows: the items of a spec, their categories and what their verdicts depend on. :doc:`README.md
<README>` gives the conventions of the networking specs, such as vantage points, daemon settings,
timing and convergence ceilings, and host requirements. :doc:`network-common.md <network-common>`
covers what every network driver must do, such as naming an endpoint's interface.

.. _bridge-networking-requirements:

**************
 Requirements
**************

.. req:: Forwarded packets with the bridge-accept-fwmark mark are not dropped
    :id: REQ-BR-ACCEPT-FWMARK
    :source_kind: docs
    :source: https://docs.docker.com/reference/cli/dockerd/, https://docs.docker.com/engine/network/firewall-nftables/#migrating-accept-rules

    The daemon option bridge-accept-fwmark gives a firewall mark, with an optional mask. In bridge
    networks, Docker accepts packets that carry the mark, so an operator who marks packets before
    Docker's rules run stops Docker from dropping them. The mark does not affect the drop of packets
    routed directly to a container from a host that is not allowed direct routing.

.. req:: The bridge name option names the network's Linux bridge
    :id: REQ-BR-BRIDGE-NAME
    :source_kind: docs
    :source: https://docs.docker.com/engine/network/drivers/bridge/#options

    The bridge option com.docker.network.bridge.name is the interface name to use when Docker
    creates the network's Linux bridge.

.. req:: The daemon option bridge set to none leaves out the default bridge
    :id: REQ-BR-DAEMON-BRIDGE-NONE
    :source_kind: docs
    :source: https://docs.docker.com/reference/cli/dockerd/#run-multiple-daemons: the --bridge entry that can be set to none and the example of a daemon without network, https://docs.docker.com/engine/release-notes/prior-releases/#171-2015-07-14: --bridge=none does not configure the network bridge

    With the daemon option bridge set to none, the daemon does not configure the default bridge
    network.

.. req:: The daemon mtu option sets the MTU of the default bridge
    :id: REQ-BR-DAEMON-MTU
    :source_kind: docs
    :source: https://docs.docker.com/reference/cli/dockerd/, https://docs.docker.com/engine/network/drivers/bridge/#configure-the-default-bridge-network, https://docs.docker.com/engine/network/drivers/bridge/#options

    The daemon option mtu (flag --mtu) sets the MTU for the default bridge network, with a default
    of 1500. It is the default bridge's equivalent of com.docker.network.driver.mtu. The option
    table of the bridge driver gives that option's 0 as no limit: a value of 0 leaves the MTU
    unlimited.

.. req:: Daemon options set the default bridge's subnets, ranges and gateways
    :id: REQ-BR-DEFAULT-BRIDGE-ADDRESSING
    :source_kind: docs
    :source: https://docs.docker.com/engine/network/drivers/bridge/#configure-the-default-bridge-network, https://docs.docker.com/engine/network/drivers/bridge/#use-ipv6-with-the-default-bridge-network, https://docs.docker.com/reference/cli/dockerd/, https://docs.docker.com/engine/release-notes/prior-releases/#170-2015-06-16: the daemon-wide default gateway for containers

    The daemon options bip and fixed-cidr configure the default bridge network: bip gives the
    bridge's address and the network's subnet, and fixed-cidr the range that container addresses are
    allocated from. The daemon option default-gateway gives the IPv4 default gateway of the
    containers on the default bridge network. The daemon option ipv6 enables IPv6 on the default
    bridge. bip6 gives the bridge's IPv6 address and the network's IPv6 subnet, and containers use
    that address as their default gateway unless default-gateway-v6 gives another. fixed-cidr-v6
    gives the range that the daemon allocates IPv6 container addresses from.

.. req:: Direct routing to published ports can be allowed per daemon or interface
    :id: REQ-BR-DIRECT-ROUTING-OPT-IN
    :source_kind: docs
    :source: https://docs.docker.com/engine/network/port-publishing/#direct-routing-to-containers-in-bridge-networks, https://docs.docker.com/engine/network/firewall-iptables/#add-iptables-policies-before-dockers-rules, https://docs.docker.com/engine/release-notes/28/#2820: the option's colon-separated list of interface names

    The daemon option allow-direct-routing allows direct access to any published port on any
    container in a bridge network. It turns off the filtering of packets from outside the host that
    are addressed directly to containers, other than packets that open a connection to an
    unpublished port. The network option trusted_host_interfaces, a colon-separated list of
    interface names, allows direct routing to the network through the named host interfaces.
    Unpublished ports stay protected under both options.

.. req:: Operator rules in DOCKER-USER filter forwarded traffic before Docker's
    :id: REQ-BR-DOCKER-USER
    :source_kind: docs
    :source: https://docs.docker.com/engine/network/firewall-iptables/#docker-and-iptables-chains, https://docs.docker.com/engine/network/firewall-iptables/#add-iptables-policies-before-dockers-rules, https://docs.docker.com/engine/network/firewall-iptables/#restrict-external-connections-to-containers: examples that restrict access to published ports with DROP rules, https://docs.docker.com/engine/release-notes/29/#2980

    Under the iptables backend, Docker creates the filter chain DOCKER-USER and makes FORWARD jump
    to it before its own chains, so that operators can filter container traffic there, including
    traffic to published ports. A DROP rule that an operator inserts in DOCKER-USER drops the
    forwarded traffic that it matches, traffic to published ports included. The published ports of
    Swarm services use the same port-publishing rules as containers' published ports, so these rules
    apply to them too.

.. req:: Docker never deletes or modifies an operator's rules in DOCKER-USER
    :id: REQ-BR-DOCKER-USER-KEPT
    :source_kind: issue
    :source: https://github.com/moby/moby/issues/29184, https://github.com/moby/moby/issues/23987, https://docs.docker.com/engine/release-notes/28/#2822
    :refines: PRIN-OWNS-ONLY-ITS-OWN

    Docker does not delete or modify rules that already exist in DOCKER-USER, across Docker
    operations and restarts, also with iptables disabled.

.. req:: Egress is masqueraded by default, with options to disable it or set SNAT
    :id: REQ-BR-EGRESS-NAT
    :source_kind: docs
    :source: https://docs.docker.com/engine/network/drivers/bridge/, https://docs.docker.com/engine/network/port-publishing/#masquerade-or-snat-for-outgoing-packets

    By default outgoing packets from bridge networks are masqueraded, so external hosts see a host
    address. enable_ip_masquerade=false disables this, and com.docker.network.host_ipv4 and
    com.docker.network.host_ipv6 set the source address to use instead of masquerading. The daemon
    option ip-masq sets enable_ip_masquerade for the default bridge.

.. req:: Operator rules ahead of a network's masquerading keep running first
    :id: REQ-BR-NAT-OPERATOR-RULES-FIRST
    :source_kind: observable
    :source: IPsec and VPN setups that send container traffic to a remote subnet through a tunnel whose policy selects traffic by its source subnet, and that insert a rule into the nat table's POSTROUTING chain so that this traffic is not masqueraded, as strongSwan's forwarding guide (https://docs.strongswan.org/docs/latest/howtos/forwarding.html) and the mberner/strongswan image (https://hub.docker.com/r/mberner/strongswan) direct

    With the iptables backend, a rule that an operator inserts into the nat table's POSTROUTING
    chain ahead of a network's masquerading keeps running before it, also after the daemon restarts.

.. req:: GET /info reports the firewall backend driver
    :id: REQ-BR-FIREWALL-BACKEND-INFO
    :source_kind: api
    :source: api/openapi.yaml: definition SystemInfo (FirewallBackend), api/openapi.yaml: definition FirewallInfo (Driver), https://docs.docker.com/reference/api/engine/version-history/#v149-api-changes

    From API 1.49, GET /info on a Linux daemon reports the daemon's firewall configuration in
    FirewallBackend, whose Driver is the name of the firewall backend driver.

.. req:: The firewall-backend option selects iptables or nftables
    :id: REQ-BR-FIREWALL-BACKEND-OPTION
    :source_kind: docs
    :source: https://docs.docker.com/engine/network/packet-filtering-firewalls/#firewall-backend, https://docs.docker.com/reference/cli/dockerd/

    The daemon option firewall-backend selects whether Docker creates its firewall rules with
    iptables, the default, or nftables.

.. req:: A firewalld reload, start or stop does not break container networking
    :id: REQ-BR-FIREWALLD-RELOAD
    :source_kind: docs
    :source: https://docs.docker.com/engine/network/packet-filtering-firewalls/#integration-with-firewalld, https://docs.docker.com/engine/release-notes/prior-releases/#1110-2016-04-13, https://docs.docker.com/engine/release-notes/28/#2833, https://github.com/moby/moby/issues/35043, https://github.com/moby/moby/pull/54012

    When firewalld is running, Docker integrates with it, and a firewalld reload must not break
    container networking: Docker recreates its chains and rules, including DOCKER-USER and the rules
    for published ports, after the reload.

    Firewalld starting or stopping while the daemon runs must not break container networking either.
    When firewalld starts, Docker integrates with it as it does when the daemon starts. When
    firewalld stops and deletes Docker's rules, Docker restores them.

.. req:: Docker puts its bridges in a firewalld zone docker with a forwarding policy
    :id: REQ-BR-FIREWALLD-ZONE
    :source_kind: docs
    :source: https://docs.docker.com/engine/network/packet-filtering-firewalls/#integration-with-firewalld, https://docs.docker.com/engine/network/firewall-nftables/#ip-forwarding, https://docs.docker.com/engine/release-notes/29/#2991
    :refines: PRIN-OWNS-ONLY-ITS-OWN

    When firewalld is running and the daemon option iptables or ip6tables is true, Docker creates a
    firewalld zone called docker, with target ACCEPT, and inserts the bridge interfaces that it
    creates into that zone. It also creates a policy called docker-forwarding that allows forwarding
    from any zone to the docker zone.

    A bridge interface that the user binds to another firewalld zone stays in that zone, and the
    daemon still sets up and restores its network.

.. req:: Routed and nat-unprotected networks accept direct routing
    :id: REQ-BR-GWMODE-DIRECT-ROUTING
    :source_kind: docs
    :source: https://docs.docker.com/engine/network/port-publishing/#gateway-modes, https://docs.docker.com/engine/release-notes/29/#2900: containers in other networks reach every port of a nat-unprotected network

    In routed mode, remote hosts with a route reach a container's published ports at its address and
    nothing else. In nat-unprotected mode, no port filtering is set up, so direct routing, from
    remote hosts and from containers in other bridge networks, reaches every container port.

.. req:: A routed network uses no NAT in either direction
    :id: REQ-BR-GWMODE-ROUTED-NO-NAT
    :source_kind: docs
    :source: https://docs.docker.com/engine/network/port-publishing/#gateway-modes

    In routed mode, no NAT or masquerading rules are set up: outgoing packets keep the container's
    address, and the host port of a mapping is not used, so the port is not opened on the host's
    addresses.

.. req:: Gateway mode is nat, nat-unprotected, routed or isolated (internal only)
    :id: REQ-BR-GWMODE-VALUES
    :source_kind: docs
    :source: https://docs.docker.com/engine/network/port-publishing/#gateway-modes

    The bridge options gateway_mode_ipv4 and gateway_mode_ipv6 take one of nat (the default),
    nat-unprotected, routed and isolated. Mode isolated can only be used on a network created as
    internal.

.. req:: host_binding_ipv4 sets the address of ports published without one
    :id: REQ-BR-HOST-BINDING-ADDRESS
    :source_kind: docs
    :source: https://docs.docker.com/engine/network/drivers/bridge/#default-host-binding-address, https://docs.docker.com/engine/network/drivers/bridge/#options

    The bridge option com.docker.network.bridge.host_binding_ipv4 sets the default host address of
    the port mappings that give none. The daemon option ip sets it for the default bridge. When it
    is an address assigned to a specific interface, a port published without a host address is
    accessible only at that address.

.. req:: Disabling ICC stops inter-container connectivity on a network
    :id: REQ-BR-ICC
    :source_kind: docs
    :source: https://docs.docker.com/engine/network/drivers/bridge/#options, https://docs.docker.com/reference/cli/dockerd/

    The bridge option com.docker.network.bridge.enable_icc, and the daemon flag --icc for the
    default bridge, enable or disable inter-container connectivity on the network.

.. req:: An internal network reaches only the node, through its gateway
    :id: REQ-BR-INTERNAL
    :source_kind: docs
    :source: https://docs.docker.com/reference/cli/docker/network/create/#internal, https://docs.docker.com/engine/network/port-publishing/#gateway-modes
    :violates: PRIN-LEAST-ACCESS

    Containers on an internal network may communicate with each other but not with other networks.
    Containers can still reach host services at the gateway address, and the host can reach any
    container address directly.

.. req:: With iptables, Docker enables forwarding and sets FORWARD to DROP
    :id: REQ-BR-IP-FORWARDING-DROP
    :source_kind: docs
    :source: https://docs.docker.com/engine/network/packet-filtering-firewalls/#docker-on-a-router, https://docs.docker.com/engine/network/firewall-iptables/#docker-and-iptables-chains, https://docs.docker.com/engine/release-notes/28/#2800
    :violates: PRIN-OWNS-ONLY-ITS-OWN

    Under the iptables backend, Docker enables net.ipv4.ip_forward and IPv6 forwarding if they are
    not already enabled, and when it does so it sets the filter FORWARD policy to DROP, unless
    ip-forward-no-drop is set. It sets the ip6tables policy only when it enabled IPv6 forwarding
    itself.

.. req:: Without IPv6 or ip6tables, the daemon runs but refuses IPv6 networks
    :id: REQ-BR-IPV6-UNAVAILABLE
    :source_kind: docs
    :source: https://docs.docker.com/engine/release-notes/27/#2701, https://docs.docker.com/engine/release-notes/28/#2801, https://github.com/moby/moby/issues/49504, https://github.com/moby/moby/issues/42288

    On a host where IPv6 is disabled, or ip6tables does not work, the daemon starts with ip6tables
    enabled and IPv4 bridge networking keeps working, including published ports; only the creation
    of an IPv6-enabled network fails.

.. req:: An isolated internal network has no address on the node
    :id: REQ-BR-ISOLATED
    :source_kind: docs
    :source: https://docs.docker.com/engine/network/port-publishing/#gateway-modes, https://docs.docker.com/engine/release-notes/28/#2800

    Gateway mode isolated, which needs an internal network, assigns no address to the bridge, so the
    host has no address in the network. An internal network otherwise lets the host and its
    containers reach each other through the bridge address.

.. req:: Without ICC, a legacy link opens the linked container's exposed ports
    :id: REQ-BR-LEGACY-LINKS
    :source_kind: docs-history
    :source: https://github.com/moby/moby/blob/8658748ef716e43a5f6d834825d818012ed6e2c4/docs/userguide/networking/default_network/container-communication.md, https://docs.docker.com/engine/network/links/

    With --icc=false, a legacy link (--link) between containers on the default bridge lets the
    linking container reach the ports that the linked container exposes.

.. req:: The network MTU option sets the MTU of the network's containers
    :id: REQ-BR-NETWORK-MTU
    :source_kind: docs
    :source: https://docs.docker.com/engine/network/drivers/bridge/#options

    The bridge option com.docker.network.driver.mtu sets the MTU of the network's container
    interfaces. Its documented default is 0, described as no limit.

.. req:: Docker's nftables base chains use the well-known priority values
    :id: REQ-BR-NFT-BASE-CHAIN-PRIORITIES
    :source_kind: docs
    :source: https://docs.docker.com/engine/network/firewall-nftables/#dockers-nftables-tables, https://docs.docker.com/engine/network/firewall-nftables/#replacing-docker-user-with-an-nftables-table
    :violates: PRIN-STABLE

    Docker's nftables base chains use the well-known priority values for their hooks, so that an
    operator's base chain on the same hook can be ordered before or after Docker's by its priority.
    The page says that support for nftables is experimental: its configuration options, behavior and
    implementation may all change in future releases.

.. req:: With the nftables backend, Docker does not enable IP forwarding
    :id: REQ-BR-NFT-IP-FORWARDING
    :source_kind: docs
    :source: https://docs.docker.com/engine/network/firewall-nftables/#ip-forwarding, https://docs.docker.com/engine/network/firewall-nftables/#migrating-from-iptables-to-nftables
    :violates: PRIN-STABLE

    With its nftables firewall backend, Docker does not enable IP forwarding on the host. It reports
    an error if forwarding is needed and is not enabled. It reports the error when it creates a
    bridge network that needs forwarding. Because of the default bridge, it reports disabled IPv4
    forwarding when the daemon starts. With the daemon option ip-forward false, Docker does not
    check: it starts and creates networks while forwarding is disabled. The page says that support
    for nftables is experimental: its configuration options, behavior and implementation may all
    change in future releases.

.. req:: A restart with the nftables backend replaces most of Docker's iptables rules
    :id: REQ-BR-NFT-MIGRATION
    :source_kind: docs
    :source: https://docs.docker.com/engine/network/firewall-nftables/#migrating-from-iptables-to-nftables, https://docs.docker.com/engine/network/firewall-nftables/#migrating-docker-user
    :violates: PRIN-STABLE

    Restarting the daemon with the nftables backend after it ran with iptables deletes most of
    Docker's iptables chains and rules and creates nftables rules instead. The daemon does not
    remove the jump from FORWARD to DOCKER-USER, so the rules in DOCKER-USER keep running until the
    jump is removed. The daemon does not change the policy of the iptables FORWARD chain. The page
    says that support for nftables is experimental: its configuration options, behavior and
    implementation may all change in future releases.

.. req:: By default, remote hosts cannot route directly to container addresses
    :id: REQ-BR-NO-DIRECT-ROUTING
    :source_kind: docs
    :source: https://docs.docker.com/engine/network/port-publishing/#direct-routing-to-containers-in-bridge-networks, https://docs.docker.com/engine/network/firewall-iptables/#add-iptables-policies-before-dockers-rules, https://docs.docker.com/engine/release-notes/28/#2800
    :refines: PRIN-LEAST-ACCESS

    By default remote hosts are not allowed direct access to container addresses in bridge networks,
    only to ports published on host addresses. Any packet that reaches the host already addressed to
    a container is dropped, and a DOCKER-USER rule cannot allow it.

.. req:: Without a host address or port, a port is published on all addresses
    :id: REQ-BR-PUBLISH-DEFAULTS
    :source_kind: docs
    :source: https://docs.docker.com/engine/network/drivers/bridge/#default-host-binding-address, https://docs.docker.com/engine/daemon/ipv6/#create-an-ipv6-network, https://docs.docker.com/reference/cli/docker/container/run/#publish-all

    A mapping without a host address publishes the port on all host addresses, IPv4 and IPv6. A
    mapping without a host port gets a random port from the ephemeral range, which docker port and
    inspect report.

.. req:: An automatically assigned host port is not a reserved port
    :id: REQ-BR-PUBLISH-SKIP-RESERVED-PORTS
    :source_kind: issue
    :source: https://github.com/moby/moby/pull/52818

    When the daemon assigns a host port automatically, from the default ephemeral range, it skips
    the ports in net.ipv4.ip_local_reserved_ports, as the kernel does for its own automatic port
    assignment. A request for a specific host port or host port range is not affected.

.. req:: Port mappings follow the container's gateway endpoint
    :id: REQ-BR-PUBLISH-GATEWAY-ENDPOINT
    :source_kind: docs
    :source: https://docs.docker.com/engine/release-notes/28/#2800, https://docs.docker.com/engine/release-notes/29/#2900, https://docs.docker.com/engine/network/#container-networks

    Port mappings are applied through the container's gateway endpoint, which can change as networks
    are connected or disconnected. Published ports in a nat network are thus reached at the network
    that gives the container its default route.

.. req:: A port published on a host address is reachable only at that address
    :id: REQ-BR-PUBLISH-HOST-IP
    :source_kind: docs
    :source: https://docs.docker.com/engine/network/port-publishing/#publishing-ports, https://docs.docker.com/engine/network/drivers/bridge/#default-host-binding-address

    A port mapping that names a host address publishes the port on that address only. A port
    published on a loopback address is accessible only from the Docker host.

.. req:: A port mapping can name the SCTP protocol
    :id: REQ-BR-PUBLISH-SCTP
    :source_kind: docs
    :source: https://docs.docker.com/engine/network/links/, https://docs.docker.com/engine/release-notes/18.03/#18030-ce

    A port mapping can bind an SCTP port by adding a trailing /sctp to the container port.

.. req:: Containers on one bridge network reach each other on every port
    :id: REQ-BR-SAME-NETWORK
    :source_kind: docs
    :source: https://docs.docker.com/engine/network/drivers/bridge/, https://docs.docker.com/engine/network/drivers/bridge/#options

    By default, a bridge network allows unrestricted access between the containers connected to it:
    they expose all their ports to each other. The option com.docker.network.bridge.enable_icc,
    which controls inter-container connectivity, is true by default.

.. req:: Over IPv4, a container sees a remote client's own address
    :id: REQ-BR-PUBLISH-V4-CLIENT-SOURCE
    :source_kind: observable
    :source: Services in containers that log or filter or rate-limit clients by source address and that IPv4 clients reach at a published port

    A container that serves a port published without a host address sees the IPv4 address of a
    remote client as the source address of the client's connections.

.. req:: Over IPv6, a container sees a remote client's own address
    :id: REQ-BR-PUBLISH-V6-CLIENT-SOURCE
    :source_kind: observable
    :source: Services in containers that log or filter or rate-limit clients by source address and that IPv6 clients reach at a published port. The daemon enables ip6tables by default since release 27.0

    With ip6tables enabled, a container that serves a port published without a host address sees the
    IPv6 address of a remote client as the source address of the client's connections.

.. req:: Other networks reach only the published ports of nat and routed networks
    :id: REQ-BR-SEPARATE-NETWORKS
    :source_kind: docs
    :source: https://docs.docker.com/engine/network/drivers/bridge/, https://docs.docker.com/engine/network/port-publishing/#gateway-modes, https://docs.docker.com/engine/release-notes/29/#2900: containers in other networks reach every port of a nat-unprotected network, https://github.com/moby/moby/pull/49981: the merged change whose commit message says that direct routing between bridge networks follows the rules for direct routing from outside the host
    :refines: PRIN-LEAST-ACCESS

    The bridge driver installs rules so that a container in another bridge network reaches a
    container in a network whose gateway mode is nat, the default, only on that container's
    published ports. It reaches them only at the host address that a port is published to, unless
    the daemon option allow-direct-routing, or the network option trusted_host_interfaces, allows
    direct routing to the network. In routed mode, a container in another network reaches the
    published ports directly at the container's address.

.. req:: Unpublished container ports are not reachable from outside the node
    :id: REQ-BR-UNPUBLISHED-PORTS
    :source_kind: docs
    :source: https://docs.docker.com/engine/network/port-publishing/, https://docs.docker.com/engine/network/#published-ports, https://docs.docker.com/engine/network/port-publishing/#direct-routing-to-containers-in-bridge-networks
    :refines: PRIN-LEAST-ACCESS

    By default the daemon blocks access to ports that have not been published; they are accessible
    from the host and the same network, but not from outside the host. Opening direct routing to
    published ports leaves unpublished ports protected.

.. _bridge-networking-rationales:

************
 Rationales
************

.. rationale:: Without the daemon's rules, its bridges stay out of an accepting firewalld zone
    :id: RAT-BR-FIREWALLD-ZONE-FOLLOWS-IPTABLES

    With the daemon options :daemon-json:`iptables` and :daemon-json:`ip6tables` false, the daemon
    creates none of its rules, and it does not put its bridge interfaces in firewalld's
    :fwd-zone:`docker` zone, whose target is ``ACCEPT``
    (:need:`BR-FW-FIREWALLD-ZONE-IPTABLES-DISABLED`). Firewalld then applies to them the zone that
    the operator binds or its default zone. A change that put the bridges in the :fwd-zone:`docker`
    zone with both options false would make firewalld accept traffic that the operator's own
    filtering drops. It should be deliberate.

.. rationale:: Only a container's routes keep it from the node's other addresses on an internal network
    :id: RAT-BR-INTERNAL-NODE-ADDRESSES

    :need:`REQ-BR-INTERNAL` says that a container on an internal network reaches host services at
    the gateway address. It does not say whether the node's other addresses are reachable. A
    container on such a network has no route to them, and no rule of the daemon's drops the traffic:
    a container with :cap:`NET_ADMIN` that adds the route reaches them. Whether the daemon filters
    that traffic decides what ``internal`` means for the node's own services. A rule change in
    either direction moves the line between an internal network and the ``isolated`` gateway mode
    (:need:`BR-ISO-ISOLATED-CTR-TO-HOST`), and should be made as a decision about that line.

.. rationale:: ReloadedAt is the only report that the daemon has reapplied its rules after a firewalld reload
    :id: RAT-BR-RELOAD-COMPLETION-REPORTED

    A firewalld reload can remove the daemon's rules, and the daemon applies them again some time
    later. What :need:`BR-FW-FIREWALLD-RELOAD-PORT` and :need:`BR-FW-FIREWALLD-RELOAD-EGRESS`
    promise holds from that point. The :api-field:`ReloadedAt` entry is the only thing the daemon
    reports that says the point has passed. The API reference says that the labels and values of
    :api-field:`FirewallBackend.Info` are not stable, so the entry is not promised. If the entry
    went away, changed its name, or was set before the rules were back, an operator debugging a
    reload could no longer tell rules that are not yet reapplied from rules that were reapplied
    wrongly, so a change to the entry should be deliberate.

.. rationale:: Under nftables, the warnings that a firewalld reload logs report no fault
    :id: RAT-BR-RELOAD-NFT-WARNINGS

    Under the nftables backend, a firewalld reload does not remove the daemon's tables, so the
    daemon has no rules to apply again. It still runs the steps that apply its endpoint and
    port-mapping rules again, as under the iptables backend, and its own nftables layer refuses each
    of those rules because its table already holds it. The warnings that these steps log report no
    fault: the daemon's tables are the same before and after. An operator's log alerts see the
    warnings on every reload, and each time firewalld starts or stops. A change to the entries, to
    their level or to their number changes what those alerts report. A change that makes one of the
    steps succeed changes the daemon's tables on a reload. Each should be deliberate.

.. _bridge-networking-host-requirements:

*******************
 Host requirements
*******************

The conventions for this section are in :ref:`README.md <README-host-requirements>`. IPv6 is not a
requirement: :ref:`bridge-networking-hosts-without-ipv6` states what holds on a host without it.

.. _bridge-networking-kernel-facilities:

Kernel facilities
=================

.. hostreq:: Network namespaces, veth pairs and Linux bridges
    :id: BR-HOST-NETNS-VETH-BRIDGE
    :kind: os
    :platforms: linux
    :kernel_options: NET_NS, VETH, BRIDGE

    *Needed when:* Always

.. needextend:: type == "expectation" and docname in ['network/bridge-networking'] and ("linux" in platforms) and "BR-HOST-NETNS-VETH-BRIDGE" not in pins
    :+needs: BR-HOST-NETNS-VETH-BRIDGE

.. hostreq:: nftables with NAT, masquerade, connection-tracking and FIB expressions
    :id: BR-HOST-NFTABLES
    :kind: os
    :platforms: linux
    :kernel_options: NF_TABLES, NF_TABLES_IPV4, NFT_NAT, NFT_MASQ, NFT_CT, NFT_FIB_IPV4, NFT_CHAIN_NAT_IPV4, NFT_MASQ_IPV4
    :when: nftables-backend

    *Facility:* nftables with the NAT, masquerade, connection-tracking and FIB expressions

    *Needed when:* The nftables backend. Without it, the daemon does not start: the bridge driver
    sets up its nftables table, and the default bridge network, when the daemon starts.

    *Kernel options:* :kconfig:`NF_TABLES`, :kconfig:`NF_TABLES_IPV4`, :kconfig:`NFT_NAT`,
    :kconfig:`NFT_MASQ`, :kconfig:`NFT_CT`, :kconfig:`NFT_FIB_IPV4`. :kconfig:`NFT_CHAIN_NAT_IPV4`
    and :kconfig:`NFT_MASQ_IPV4` on kernels that have them.

.. needextend:: type == "expectation" and docname in ['network/bridge-networking'] and ("linux" in platforms) and "BR-HOST-NFTABLES" not in pins
    :+needs: BR-HOST-NFTABLES

.. hostreq:: nftables for IPv6 with NAT, masquerade and FIB expressions
    :id: BR-HOST-NFTABLES-V6
    :kind: os
    :platforms: linux
    :kernel_options: NF_TABLES_IPV6, NFT_FIB_IPV6, NFT_CHAIN_NAT_IPV6, NFT_MASQ_IPV6
    :when: nftables-backend, ip6tables
    :tags: ipv6

    *Facility:* nftables in the ``ip6`` family, with the NAT, masquerade and FIB expressions

    *Needed when:* The nftables backend, with the daemon option :daemon-json:`ip6tables` true, for a
    bridge network with IPv6 and for the daemon's IPv6 table

    *Kernel options:* :kconfig:`NF_TABLES_IPV6`, :kconfig:`NFT_FIB_IPV6`.
    :kconfig:`NFT_CHAIN_NAT_IPV6` and :kconfig:`NFT_MASQ_IPV6` on kernels that have them.

.. hostreq:: Netfilter hooks for bridged traffic, without inter-container communication
    :id: BR-HOST-BR-NETFILTER-NO-ICC
    :kind: os
    :platforms: linux
    :kernel_options: BRIDGE_NETFILTER
    :when: iptables

    *Facility:* Netfilter hooks for bridged traffic (``br_netfilter``)

    *Needed when:* The daemon option :daemon-json:`iptables` true, and a bridge network that does
    not allow inter-container communication. With :daemon-json:`ip6tables` true, an IPv6 network of
    that kind needs it too. Without it, a daemon with :daemon-json:`icc` false does not start with
    its default bridge network.

.. hostreq:: Netfilter hooks for bridged traffic, without the userland proxy
    :id: BR-HOST-BR-NETFILTER-NO-PROXY
    :kind: os
    :platforms: linux
    :kernel_options: BRIDGE_NETFILTER
    :when: iptables, no-userland-proxy

    *Facility:* Netfilter hooks for bridged traffic (``br_netfilter``)

    *Needed when:* The daemon options :daemon-json:`iptables` true and :daemon-json:`userland-proxy`
    false. Without it, the daemon does not start with its default bridge network.

.. needextend:: type == "expectation" and docname in ['network/bridge-networking'] and ("linux" in platforms) and "BR-HOST-BR-NETFILTER-NO-PROXY" not in pins
    :+needs: BR-HOST-BR-NETFILTER-NO-PROXY

.. hostreq:: SCTP in the kernel
    :id: BR-HOST-SCTP
    :kind: os
    :platforms: linux
    :kernel_options: IP_SCTP, NF_CT_PROTO_SCTP
    :tags: sctp

    *Facility:* SCTP

    *Needed when:* A container that publishes an SCTP port

    *Kernel options:* :kconfig:`IP_SCTP`. :kconfig:`NF_CT_PROTO_SCTP` on kernels that have it.

.. _bridge-networking-programs:

Programs
========

.. hostreq:: iptables with the tables, targets and matches it needs
    :id: BR-HOST-IPTABLES-COMMAND
    :kind: program
    :platforms: linux
    :when: iptables-backend, iptables

    *Facility:* :program:`iptables`, accepting the filter, nat and raw tables, the
    :ipt-target:`MASQUERADE`, :ipt-target:`DNAT` and :ipt-target:`SNAT` targets, and the
    ``addrtype`` and ``conntrack`` matches

    *Needed when:* The iptables backend, with the daemon option :daemon-json:`iptables` true.
    Without it, the daemon does not start (:need:`BR-FW-IPTABLES-MISSING`).

.. needextend:: type == "expectation" and docname in ['network/bridge-networking'] and ("linux" in platforms) and "BR-HOST-IPTABLES-COMMAND" not in pins
    :+needs: BR-HOST-IPTABLES-COMMAND

.. hostreq:: ip6tables with the same tables, targets and matches
    :id: BR-HOST-IP6TABLES-COMMAND
    :kind: program
    :platforms: linux
    :when: iptables-backend, ip6tables

    *Facility:* :program:`ip6tables`, accepting the same tables, targets and matches

    *Needed when:* The iptables backend, with the daemon option :daemon-json:`ip6tables` true, for a
    bridge network with IPv6 and for the daemon's IPv6 chains, such as :ipt-chain:`DOCKER-USER`

.. hostreq:: The nft command
    :id: BR-HOST-NFT-COMMAND
    :kind: program
    :platforms: linux
    :when: nftables-backend

    *Facility:* :program:`nft`

    *Needed when:* The nftables backend, on a daemon built without the ``libnftables`` build tag. A
    daemon built with it uses the libnftables library and runs no :program:`nft` command. Without
    :program:`nft`, a daemon built without that tag does not start (:need:`BR-FW-NFT-MISSING`).

.. needextend:: type == "expectation" and docname in ['network/bridge-networking'] and ("linux" in platforms) and "BR-HOST-NFT-COMMAND" not in pins
    :+needs: BR-HOST-NFT-COMMAND

.. hostreq:: iptables and ip6tables under the nftables backend
    :id: BR-HOST-IPTABLES-CLEANUP-COMMAND
    :kind: program
    :platforms: linux
    :when: nftables-backend

    *Facility:* :program:`iptables` with the daemon option :daemon-json:`iptables` true, and
    :program:`ip6tables` with the daemon option :daemon-json:`ip6tables` true, accepting the filter,
    nat and raw tables

    *Needed when:* The nftables backend, on a node with chains or rules that a daemon left in those
    tables under the iptables backend

.. hostreq:: nft under the iptables backend
    :id: BR-HOST-NFT-CLEANUP-COMMAND
    :kind: program
    :platforms: linux
    :when: iptables-backend

    *Facility:* :program:`nft`

    *Needed when:* The iptables backend, on a node with a :nft-table:`docker-bridges` table that a
    daemon left under the nftables backend, on a daemon built without the ``libnftables`` build tag.
    A daemon built with it uses the libnftables library and runs no :program:`nft` command.

The daemon does not depend on which implementation of :program:`iptables` and :program:`ip6tables`
the host has. The kernel options that the programs need depend on that implementation:

.. hostreq:: Kernel options for iptables
    :id: BR-HOST-IPTABLES-KERNEL
    :kind: os
    :platforms: linux
    :kernel_options: NETFILTER, NETFILTER_ADVANCED, NETFILTER_XTABLES, NF_CONNTRACK, NF_NAT, NETFILTER_XT_NAT, NETFILTER_XT_TARGET_MASQUERADE, NETFILTER_XT_MATCH_ADDRTYPE, NETFILTER_XT_MATCH_CONNTRACK, IP_NF_NAT, IP_NF_TARGET_MASQUERADE
    :when: iptables-backend, iptables

    *Needed when:* The iptables backend, with the daemon option :daemon-json:`iptables` true

    The kernel options that :program:`iptables` needs, whichever implementation provides it. *Kernel
    options:* :kconfig:`NETFILTER`, :kconfig:`NETFILTER_ADVANCED`, :kconfig:`NETFILTER_XTABLES`,
    :kconfig:`NF_CONNTRACK`, :kconfig:`NF_NAT`, :kconfig:`NETFILTER_XT_NAT`,
    :kconfig:`NETFILTER_XT_TARGET_MASQUERADE`, :kconfig:`NETFILTER_XT_MATCH_ADDRTYPE`,
    :kconfig:`NETFILTER_XT_MATCH_CONNTRACK`. Before Linux 5.2, :kconfig:`IP_NF_NAT` and
    :kconfig:`IP_NF_TARGET_MASQUERADE` in place of :kconfig:`NETFILTER_XT_TARGET_MASQUERADE`.

.. needextend:: type == "expectation" and docname in ['network/bridge-networking'] and ("linux" in platforms) and "BR-HOST-IPTABLES-KERNEL" not in pins
    :+needs: BR-HOST-IPTABLES-KERNEL

.. hostreq:: Kernel options for iptables from iptables-legacy
    :id: BR-HOST-IPTABLES-LEGACY-KERNEL
    :kind: os
    :platforms: linux
    :kernel_options: IP_NF_IPTABLES, IP_NF_FILTER, IP_NF_NAT, IP_NF_RAW, IP_NF_IPTABLES_LEGACY, NETFILTER_XTABLES_LEGACY
    :when: iptables-backend, iptables, iptables-legacy

    *Needed when:* The iptables backend with iptables-legacy providing :program:`iptables`, with the
    daemon option :daemon-json:`iptables` true

    The kernel options that :program:`iptables` needs when iptables-legacy provides it. *Kernel
    options:* :kconfig:`IP_NF_IPTABLES`, :kconfig:`IP_NF_FILTER`, :kconfig:`IP_NF_NAT`,
    :kconfig:`IP_NF_RAW`. :kconfig:`IP_NF_IPTABLES_LEGACY` from Linux 6.9, and
    :kconfig:`NETFILTER_XTABLES_LEGACY` from Linux 6.17.

.. needextend:: type == "expectation" and docname in ['network/bridge-networking'] and ("linux" in platforms) and "BR-HOST-IPTABLES-LEGACY-KERNEL" not in pins
    :+needs: BR-HOST-IPTABLES-LEGACY-KERNEL

.. hostreq:: Kernel options for iptables from iptables-nft
    :id: BR-HOST-IPTABLES-NFT-KERNEL
    :kind: os
    :platforms: linux
    :kernel_options: NF_TABLES, NF_TABLES_IPV4, NFT_COMPAT, NFT_NAT, NFT_CHAIN_NAT_IPV4
    :when: iptables-backend, iptables, iptables-nft

    *Needed when:* The iptables backend with iptables-nft providing :program:`iptables`, with the
    daemon option :daemon-json:`iptables` true

    The kernel options that :program:`iptables` needs when iptables-nft provides it. *Kernel
    options:* :kconfig:`NF_TABLES`, :kconfig:`NF_TABLES_IPV4`, :kconfig:`NFT_COMPAT`,
    :kconfig:`NFT_NAT`. Before Linux 5.1, :kconfig:`NFT_CHAIN_NAT_IPV4` in place of
    :kconfig:`NFT_NAT`.

.. needextend:: type == "expectation" and docname in ['network/bridge-networking'] and ("linux" in platforms) and "BR-HOST-IPTABLES-NFT-KERNEL" not in pins
    :+needs: BR-HOST-IPTABLES-NFT-KERNEL

.. hostreq:: Kernel options for ip6tables
    :id: BR-HOST-IP6TABLES-KERNEL
    :kind: os
    :platforms: linux
    :kernel_options: IPV6, IP6_NF_NAT, IP6_NF_TARGET_MASQUERADE, NETFILTER_XT_TARGET_MASQUERADE, NETFILTER, NETFILTER_ADVANCED, NETFILTER_XTABLES, NF_CONNTRACK, NF_NAT, NETFILTER_XT_NAT, NETFILTER_XT_MATCH_ADDRTYPE, NETFILTER_XT_MATCH_CONNTRACK
    :when: iptables-backend, ip6tables

    *Needed when:* The iptables backend, with the daemon option :daemon-json:`ip6tables` true, for a
    bridge network with IPv6 and for the daemon's IPv6 chains, such as :ipt-chain:`DOCKER-USER`

    The kernel options that :program:`ip6tables` needs, whichever implementation provides it.
    *Kernel options:* :kconfig:`IPV6`, and the options that iptables needs, whichever implementation
    provides it, other than its IPv4 options before Linux 5.2. Before Linux 5.2,
    :kconfig:`IP6_NF_NAT` and :kconfig:`IP6_NF_TARGET_MASQUERADE` in place of
    :kconfig:`NETFILTER_XT_TARGET_MASQUERADE`.

.. hostreq:: Kernel options for ip6tables from iptables-legacy
    :id: BR-HOST-IP6TABLES-LEGACY-KERNEL
    :kind: os
    :platforms: linux
    :kernel_options: IP6_NF_IPTABLES, IP6_NF_FILTER, IP6_NF_NAT, IP6_NF_RAW, IP6_NF_IPTABLES_LEGACY, NETFILTER_XTABLES_LEGACY
    :when: iptables-backend, ip6tables, iptables-legacy

    *Needed when:* The iptables backend with iptables-legacy providing :program:`ip6tables`, with
    the daemon option :daemon-json:`ip6tables` true, for a bridge network with IPv6 and for the
    daemon's IPv6 chains, such as :ipt-chain:`DOCKER-USER`

    The kernel options that :program:`ip6tables` needs when iptables-legacy provides it. *Kernel
    options:* :kconfig:`IP6_NF_IPTABLES`, :kconfig:`IP6_NF_FILTER`, :kconfig:`IP6_NF_NAT`,
    :kconfig:`IP6_NF_RAW`. :kconfig:`IP6_NF_IPTABLES_LEGACY` from Linux 6.9, and
    :kconfig:`NETFILTER_XTABLES_LEGACY` from Linux 6.17.

.. hostreq:: Kernel options for ip6tables from iptables-nft
    :id: BR-HOST-IP6TABLES-NFT-KERNEL
    :kind: os
    :platforms: linux
    :kernel_options: NF_TABLES, NF_TABLES_IPV6, NFT_COMPAT, NFT_NAT, NFT_CHAIN_NAT_IPV6
    :when: iptables-backend, ip6tables, iptables-nft

    *Needed when:* The iptables backend with iptables-nft providing :program:`ip6tables`, with the
    daemon option :daemon-json:`ip6tables` true, for a bridge network with IPv6 and for the daemon's
    IPv6 chains, such as :ipt-chain:`DOCKER-USER`

    The kernel options that :program:`ip6tables` needs when iptables-nft provides it. *Kernel
    options:* :kconfig:`NF_TABLES`, :kconfig:`NF_TABLES_IPV6`, :kconfig:`NFT_COMPAT`,
    :kconfig:`NFT_NAT`. Before Linux 5.1, :kconfig:`NFT_CHAIN_NAT_IPV6` in place of
    :kconfig:`NFT_NAT`.

.. hostreq:: The iptables sctp match
    :id: BR-HOST-IPTABLES-SCTP-MATCH
    :kind: os
    :platforms: linux
    :kernel_options: NETFILTER_XT_MATCH_SCTP
    :when: iptables-backend, iptables
    :tags: sctp

    *Facility:* The ``sctp`` match of :program:`iptables`

    *Needed when:* The iptables backend, with the daemon option :daemon-json:`iptables` true, and a
    container that publishes an SCTP port. With :daemon-json:`ip6tables` true, an SCTP port
    published on IPv6 needs it too.

    *Kernel options:* :kconfig:`NETFILTER_XT_MATCH_SCTP`

.. _bridge-networking-host-configuration:

Host configuration
==================

.. hostreq:: IPv4 forwarding enabled (net.ipv4.ip_forward)
    :id: BR-HOST-FORWARDING-V4
    :kind: config
    :platforms: linux
    :when: nftables-backend, ip-forward

    *Facility:* :sysctl:`net.ipv4.ip_forward` is 1

    *Needed when:* The nftables backend, with the daemon option :daemon-json:`ip-forward` true.
    Without it, the daemon does not start with its default bridge network
    (:need:`BR-FW-NFT-NO-FORWARDING`).

.. needextend:: type == "expectation" and docname in ['network/bridge-networking'] and ("linux" in platforms) and "BR-HOST-FORWARDING-V4" not in pins
    :+needs: BR-HOST-FORWARDING-V4

.. hostreq:: IPv6 forwarding enabled for all and default
    :id: BR-HOST-FORWARDING-V6
    :kind: config
    :platforms: linux
    :when: nftables-backend, ip-forward
    :tags: ipv6

    *Facility:* :sysctl:`net.ipv6.conf.all.forwarding` and
    :sysctl:`net.ipv6.conf.default.forwarding` are 1

    *Needed when:* The nftables backend, with the daemon option :daemon-json:`ip-forward` true, and
    a bridge network with IPv6

.. hostreq:: Writable /proc/sys/net in the daemon's network namespace
    :id: BR-HOST-PROC-SYS-NET-WRITABLE
    :kind: config
    :platforms: linux

    *Facility:* :file:`/proc/sys/net` is writable in the daemon's network namespace

    *Needed when:* Always

.. needextend:: type == "expectation" and docname in ['network/bridge-networking'] and ("linux" in platforms) and "BR-HOST-PROC-SYS-NET-WRITABLE" not in pins
    :+needs: BR-HOST-PROC-SYS-NET-WRITABLE

.. hostreq:: No other firewall table drops forwarded traffic
    :id: BR-HOST-NO-FORWARD-DROP
    :kind: config
    :platforms: linux
    :when: nftables-backend

    *Facility:* No other table in the host's firewall drops the forwarded traffic of bridge
    networks. For example, the iptables filter table's :ipt-chain:`FORWARD` chain has policy ACCEPT.

    *Needed when:* The nftables backend, for traffic to or from an endpoint on a bridge network that
    passes the node's forward path, such as egress, traffic to a published port and traffic between
    networks, and traffic within a network where the netfilter hooks for bridged traffic are loaded.
    The endpoints on :network:`docker_gwbridge` that carry the egress and the routing mesh of Swarm
    tasks are such endpoints.

.. _bridge-networking-firewall-anchors-host-integration:

*************************************
 Firewall anchors & host integration
*************************************

The content of Moby's firewall rules is not an interface (:need:`PRIN-STABLE`). A backend change
replaces exactly this content. :ref:`bridge-networking-rule-content` pins it with detectors only.
Two things are interfaces. They are the anchors of this section:

1. The :ipt-chain:`DOCKER-USER` iptables chain. Operators attach rules to it. The documentation
   names it as the place to put policy that survives daemon restarts. The daemon never deletes or
   modifies its contents.
2. The hooks and priorities of the bridge driver's nftables base chains. They decide the evaluation
   order relative to everything else on the host, such as firewalld, kube-proxy or an operator's own
   ruleset. A changed priority silently changes who sees a packet first.

No other name is an anchor. Moby's other iptables chains are not anchors, and no name in nftables is
an anchor. An operator's rules can reach Moby's nftables chains only if they are programmed into
Moby's own tables. That is not a supported configuration.

IPv4 is an enabled IP version while the daemon option :daemon-json:`iptables` is true. IPv6 is an
enabled IP version while the daemon option :daemon-json:`ip6tables` is true. This applies under
either firewall backend, and whether or not :daemon-json:`ipv6` is set.

.. _bridge-networking-base-chain-priorities:

Base-chain priorities
=====================

These expectations cover the host's network namespace only. The specs do not assert the rules that
the daemon programs in a container's network namespace
(:ref:`container-dns-deliberately-not-asserted` in :doc:`container-dns.md <container-dns>`).

.. expectation:: Docker's nftables base chains are at the well-known priorities
    :id: BR-PRIO-WELL-KNOWN
    :observed_from: node
    :depends_on: none
    :platforms: linux
    :provisional_on: REQ-BR-NFT-BASE-CHAIN-PRIORITIES
    :needs: BR-HOST-NFTABLES-V6

    *Applies to:* The nftables backend

    For each enabled IP version, each base chain of the bridge driver's nftables table
    :nft-table:`docker-bridges` in that version's family has the well-known priority of its kind:
    raw (−300) for its chain on the prerouting hook that runs before connection tracking, dstnat
    (−100) for its chains on the prerouting and output hooks that translate destinations, filter (0)
    for its chain on the forward hook, and srcnat (100) for its chain on the postrouting hook.

.. expectation:: Each docker-bridges table has exactly five base chains at set priorities
    :id: BR-PRIO-BASE-CHAINS
    :observed_from: node
    :depends_on: none
    :platforms: linux
    :exact_form_of: BR-PRIO-WELL-KNOWN
    :pins: BR-HOST-NFTABLES
    :needs: BR-HOST-NFTABLES, BR-HOST-NFTABLES-V6

    *Applies to:* The nftables backend

    For each enabled IP version, the bridge driver's nftables table :nft-table:`docker-bridges` in
    that version's family has exactly five base chains. They are on the prerouting hook at raw
    priority (−300) and at dstnat priority (−100), on the output hook at dstnat priority (−100), on
    the forward hook at filter priority (0), and on the postrouting hook at srcnat priority (100).

.. _bridge-networking-ipt-chain-docker-user:

:ipt-chain:`DOCKER-USER`
========================

:ipt-chain:`DOCKER-USER` belongs to the iptables backend. The expectations in this section hold
under the iptables backend.

.. expectation:: DOCKER-USER exists and FORWARD jumps to it for each IP version
    :id: BR-USER-CHAIN-JUMP
    :observed_from: node
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-DOCKER-USER
    :needs: BR-HOST-IP6TABLES-COMMAND, BR-HOST-IP6TABLES-KERNEL, BR-HOST-IP6TABLES-LEGACY-KERNEL, BR-HOST-IP6TABLES-NFT-KERNEL

    For each enabled IP version, :ipt-chain:`DOCKER-USER` exists in the filter table and
    :ipt-chain:`FORWARD` jumps to it.

.. expectation:: An operator's DROP rule in DOCKER-USER drops traffic to a published port
    :id: BR-USER-DROP-RULE
    :observed_from: client
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-DOCKER-USER, REQ-BR-FIREWALLD-RELOAD

    *Applies to:* A DROP rule that an operator places in :ipt-chain:`DOCKER-USER`, which matches
    traffic to a published port by the client's address, or by the container's address and port

    The rule drops the traffic to the published port that it matches and that the node forwards to
    the container, not relaying it through :program:`docker-proxy`. This is also true for a rule
    that an operator places after a firewalld reload.

.. expectation:: An operator's ACCEPT rule in DOCKER-USER does not open direct routing
    :id: BR-USER-ACCEPT-NO-DIRECT-ROUTING
    :observed_from: underlay → ctr
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-NO-DIRECT-ROUTING

    *Applies to:* A daemon with :daemon-json:`allow-direct-routing` false; a container on a bridge
    network whose :drvopt:`com.docker.network.bridge.gateway_mode_ipv4` is ``nat`` or not set; an
    underlay host with a route to the container's IPv4 address via the node's underlay address,
    whose traffic arrives on an interface that is not in the network's
    :drvopt:`com.docker.network.bridge.trusted_host_interfaces`; and an operator's rule in
    :ipt-chain:`DOCKER-USER` that accepts this traffic

    Traffic from that host to the container's IPv4 address does not reach the container on any port.

.. expectation:: An operator's DOCKER-USER rules stay unchanged across a daemon start
    :id: BR-USER-RULES-KEPT
    :observed_from: node
    :depends_on: none
    :platforms: linux
    :refines: REQ-BR-DOCKER-USER-KEPT

    :ipt-chain:`DOCKER-USER` rules that an operator added before a start of the daemon are still
    there, unchanged, after the start. This includes a restart with :daemon-json:`iptables` false.

.. expectation:: After a firewalld reload, DOCKER-USER exists and FORWARD jumps to it
    :id: BR-USER-CHAIN-JUMP-FIREWALLD
    :observed_from: node
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-DOCKER-USER, REQ-BR-FIREWALLD-RELOAD
    :needs: BR-HOST-IP6TABLES-COMMAND, BR-HOST-IP6TABLES-KERNEL, BR-HOST-IP6TABLES-LEGACY-KERNEL, BR-HOST-IP6TABLES-NFT-KERNEL

    After a firewalld reload, for each enabled IP version, :ipt-chain:`DOCKER-USER` exists and
    :ipt-chain:`FORWARD` jumps to it.

.. expectation:: After firewalld stops, DOCKER-USER exists and FORWARD jumps to it
    :id: BR-USER-CHAIN-JUMP-FIREWALLD-STOP
    :observed_from: node
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-DOCKER-USER, REQ-BR-FIREWALLD-RELOAD
    :needs: BR-HOST-IP6TABLES-COMMAND, BR-HOST-IP6TABLES-KERNEL, BR-HOST-IP6TABLES-LEGACY-KERNEL, BR-HOST-IP6TABLES-NFT-KERNEL

    *Applies to:* A node with firewalld running

    After firewalld stops, for each enabled IP version, :ipt-chain:`DOCKER-USER` exists and
    :ipt-chain:`FORWARD` jumps to it.

.. expectation:: After firewalld starts later than the daemon, FORWARD jumps to DOCKER-USER
    :id: BR-USER-CHAIN-JUMP-FIREWALLD-LATE-START
    :observed_from: node
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-DOCKER-USER, REQ-BR-FIREWALLD-RELOAD
    :needs: BR-HOST-IP6TABLES-COMMAND, BR-HOST-IP6TABLES-KERNEL, BR-HOST-IP6TABLES-LEGACY-KERNEL, BR-HOST-IP6TABLES-NFT-KERNEL

    *Applies to:* A daemon that started while firewalld was not running

    After firewalld starts, for each enabled IP version, :ipt-chain:`DOCKER-USER` exists and
    :ipt-chain:`FORWARD` jumps to it.

.. _bridge-networking-cross-generation-cleanup:

Cross-generation cleanup
========================

This section is about a node that runs release *N−1* and is then upgraded to *N*. After the upgrade,
the node can also use a different firewall backend. *N−1* offers both backends for bridge networks,
so the previous generation can be of either backend. A generation is the firewall state that one
release creates under one firewall backend. A previous generation is such state that a daemon left
on the node before the current start of the daemon. After the upgrade, the node must not have two
generations of live rules. The expectations in this section hold for each enabled IP version. The
built-in chains are the chains that iptables itself provides in each table. Examples are the filter
table's :ipt-chain:`FORWARD` chain and the nat table's :ipt-chain:`PREROUTING` chain.

Cleanup finds the chains and tables of a previous generation by their names. Thus the daemon itself
needs these names, but they are not an interface for operators. A rename thus needs a cleanup path
for the old name. Without one, an upgraded node keeps the rules of the previous generation.

In this section, the daemon restores a network, endpoint or port binding when it sets it up again
during its start. It restores each bridge network that exists at the start. It also restores the
endpoints and port bindings of containers that the previous daemon left in place. Those are the
endpoints and port bindings of the containers that :daemon-json:`live-restore` keeps running, and
those of the containers that the daemon stops during its start, after it was killed or after a stop
with :daemon-json:`live-restore` true. The daemon restores them before its API responds. It does not
restore a network whose state it does not have when it starts, such as a network whose state was
deleted from the daemon's data root while the daemon was stopped, or that network's endpoints and
port bindings. :need:`BR-CLEAN-BUILTIN-CHAIN-RULES` states which rules stay for them.

.. expectation:: No table or chain of the other backend remains after the daemon starts
    :id: BR-CLEAN-OTHER-BACKEND
    :observed_from: node
    :depends_on: os-linux, upgrade
    :platforms: linux
    :refines: REQ-BR-DOCKER-USER-KEPT, PRIN-NO-LEAKS
    :provisional_on: REQ-BR-NFT-MIGRATION
    :needs: BR-HOST-IPTABLES-CLEANUP-COMMAND, BR-HOST-NFT-CLEANUP-COMMAND

    After the daemon has started, the host netns does not contain any table or chain that a previous
    generation left there with the other backend. The exceptions are :ipt-chain:`DOCKER-USER` and
    the :ipt-chain:`FORWARD` jump to it, which hold the operator's rules and stay in place. The
    tables and built-in chains that an :program:`iptables` command creates by itself, such as the
    filter table and its :ipt-chain:`FORWARD` chain, are also exceptions. The overlay driver's
    :nft-table:`docker-overlay` table is also an exception. The tables and chains that belong to
    Swarm, such as :ipt-chain:`DOCKER-INGRESS`, are among those that do not remain. This holds for a
    switch from iptables to nftables and for a switch from nftables to iptables. It holds across a
    package upgrade from *N−1* to *N* and across a same-binary restart.

.. expectation:: Neither the old DOCKER-ISOLATION chain nor its FORWARD rule remains
    :id: BR-CLEAN-ISOLATION-CHAIN
    :observed_from: node
    :depends_on: os-linux
    :platforms: linux
    :refines: PRIN-NO-LEAKS
    :needs: BR-HOST-IP6TABLES-COMMAND, BR-HOST-IP6TABLES-KERNEL, BR-HOST-IP6TABLES-LEGACY-KERNEL, BR-HOST-IP6TABLES-NFT-KERNEL, BR-HOST-IPTABLES-CLEANUP-COMMAND

    *Applies to:* The :ipt-chain:`DOCKER-ISOLATION` chain, which releases older than *N−1* created
    and *N* does not create, with the rule :ipt-target:`-j DOCKER-ISOLATION` in the filter table's
    :ipt-chain:`FORWARD` chain that those releases installed, beside the chains of *N−1*. The
    expectation holds for that state whatever created it.

    After the daemon has started, the filter table contains neither that chain nor that rule.

.. expectation:: Restoring a network, endpoint or binding removes its old built-in rules
    :id: BR-CLEAN-BUILTIN-RULES
    :observed_from: node
    :depends_on: os-linux, upgrade
    :platforms: linux
    :refines: PRIN-NO-LEAKS
    :provisional_on: REQ-BR-NFT-MIGRATION
    :needs: BR-HOST-IPTABLES-CLEANUP-COMMAND

    *Applies to:* A rule that the previous generation left in a built-in chain for a network,
    endpoint or port binding: a rule in a built-in chain that matches on the network's bridge or
    subnets, on the endpoint's addresses, or on the binding's host address or port

    After a switch from iptables to nftables, the rule does not remain after the daemon has restored
    that network, endpoint or port binding. This holds across a package upgrade from the previous
    release and across a same-binary restart.

.. expectation:: No docker-bridges table remains after a start with iptables
    :id: BR-CLEAN-OTHER-BACKEND-IPT
    :observed_from: node
    :depends_on: os-linux
    :platforms: linux
    :exact_form_of: BR-CLEAN-OTHER-BACKEND
    :needs: BR-HOST-NFT-CLEANUP-COMMAND

    *Applies to:* A :nft-table:`docker-bridges` table that a daemon left under the nftables backend

    After the daemon starts with the iptables backend, that table does not remain.

.. expectation:: No Docker iptables chain or jump remains after a start with nftables
    :id: BR-CLEAN-OTHER-BACKEND-NFT
    :observed_from: node
    :depends_on: os-linux
    :platforms: linux
    :exact_form_of: BR-CLEAN-OTHER-BACKEND
    :needs: BR-HOST-IPTABLES-CLEANUP-COMMAND

    *Applies to:* A jump from the filter table's :ipt-chain:`FORWARD` chain to
    :ipt-chain:`DOCKER-FORWARD` or to :ipt-chain:`DOCKER-ISOLATION-STAGE-1`

    After the daemon starts with the nftables backend, some chains do not remain. These chains are
    :ipt-chain:`DOCKER` (in the filter and nat tables), :ipt-chain:`DOCKER-FORWARD`,
    :ipt-chain:`DOCKER-BRIDGE`, :ipt-chain:`DOCKER-CT`, :ipt-chain:`DOCKER-INTERNAL`,
    :ipt-chain:`DOCKER-ISOLATION-STAGE-1`, :ipt-chain:`DOCKER-ISOLATION-STAGE-2` and
    :ipt-chain:`DOCKER-ISOLATION`. The jumps to these chains from :ipt-chain:`FORWARD` and from the
    nat table's :ipt-chain:`PREROUTING` and :ipt-chain:`OUTPUT` do not remain either.

.. expectation:: After a switch to nftables, rules for what is not restored stay in built-in chains
    :id: BR-CLEAN-BUILTIN-CHAIN-RULES
    :observed_from: node
    :depends_on: none
    :platforms: linux
    :needs: BR-HOST-IPTABLES-CLEANUP-COMMAND
    :concerns: BR-CLEAN-BUILTIN-RULES, BR-CLEAN-OTHER-BACKEND
    :violates: PRIN-NO-LEAKS

    After a switch from the iptables backend to the nftables backend, the daemon deletes the rules
    that the previous generation added to the built-in iptables chains only for the networks,
    endpoints and port bindings that it restores. Rules for anything that it does not restore stay
    in the built-in chains.

.. expectation:: Old built-in rules stay for a network that the daemon does not restore
    :id: BR-CLEAN-BUILTIN-RULES-UNTIL-RESTORED
    :observed_from: node
    :depends_on: os-linux
    :platforms: linux
    :pins: BR-CLEAN-BUILTIN-CHAIN-RULES
    :needs: BR-HOST-IPTABLES-CLEANUP-COMMAND

    *Applies to:* A bridge network that a previous generation set up under the iptables backend, and
    whose state the daemon does not have when it starts

    After a start with the nftables backend, once the daemon's API responds, the rule in the nat
    table's :ipt-chain:`POSTROUTING` chain that masquerades the network's subnet
    (``-s <subnet> ! -o <bridge> -j MASQUERADE``) remains.

.. expectation:: A start with nftables does not modify the FORWARD chain's policy
    :id: BR-CLEAN-FORWARD-POLICY-KEPT
    :observed_from: node
    :depends_on: none
    :platforms: linux
    :provisional_on: REQ-BR-NFT-MIGRATION
    :pins: BR-HOST-NO-FORWARD-DROP

    The policy of the iptables filter table's :ipt-chain:`FORWARD` chain is not modified when the
    daemon starts with the nftables backend. This includes a policy of DROP.

.. expectation:: A FORWARD policy of DROP logs a warning on the first start with nftables
    :id: BR-CLEAN-FORWARD-DROP-WARNING
    :observed_from: log
    :depends_on: none
    :platforms: linux
    :pins: BR-HOST-NO-FORWARD-DROP
    :needs: BR-HOST-IPTABLES-CLEANUP-COMMAND

    *Applies to:* The filter table's :ipt-chain:`FORWARD` chain with policy DROP

    On the first start after a switch from iptables to nftables, the daemon logs a warning that
    contains :log:`Network traffic for published ports may be dropped, iptables chain FORWARD has
    policy DROP.`

.. expectation:: Without the FORWARD jump, a start with nftables keeps the old iptables state
    :id: BR-CLEAN-NO-FORWARD-JUMP
    :observed_from: node
    :depends_on: none
    :platforms: linux
    :narrows: BR-CLEAN-OTHER-BACKEND, BR-CLEAN-BUILTIN-RULES, BR-CLEAN-FORWARD-DROP-WARNING, BR-CLEAN-ISOLATION-CHAIN
    :concerns: BR-CLEAN-OTHER-BACKEND-NFT, BR-CLEAN-BUILTIN-RULES-UNTIL-RESTORED
    :violates: PRIN-NO-LEAKS, REQ-BR-NFT-MIGRATION

    On a start with the nftables backend, the daemon removes the iptables chains of a previous
    generation for an IP version only when that IP version's filter table :ipt-chain:`FORWARD` chain
    jumps to :ipt-chain:`DOCKER-FORWARD` or to :ipt-chain:`DOCKER-ISOLATION-STAGE-1`. When that jump
    is gone, for example because an operator deleted it, the chains of that IP version,
    :ipt-chain:`DOCKER-ISOLATION` among them, stay, and so do the jumps to that IP version's nat
    table :ipt-chain:`DOCKER` chain from its :ipt-chain:`PREROUTING` and :ipt-chain:`OUTPUT` chains.
    When that IP version's :ipt-chain:`FORWARD` chain also has policy DROP, the daemon logs on that
    start no warning for that IP version that contains :log:`Network traffic for published ports may
    be dropped, iptables chain FORWARD has policy DROP.` When that jump is gone for every enabled IP
    version, the jumps to those chains also stay, and so do the rules that the previous generation
    left in built-in chains, after the daemon restores their networks, endpoints and port bindings.

.. expectation:: After a switch to nftables, a DOCKER-USER DROP rule still drops traffic
    :id: BR-CLEAN-USER-DROP-RULE
    :observed_from: client
    :depends_on: os-linux
    :platforms: linux
    :provisional_on: REQ-BR-NFT-MIGRATION
    :pins: BR-HOST-NO-FORWARD-DROP

    *Applies to:* A filter table :ipt-chain:`FORWARD` chain with policy ACCEPT when the daemon
    starts with the nftables backend, and a DROP rule that an operator placed in
    :ipt-chain:`DOCKER-USER` under the iptables backend, which matches traffic to a published port
    by the client's address, or by the container's address and port

    After a switch from iptables to nftables, the rule still drops the traffic to the published port
    that it matches and that the node forwards to the container.

.. _bridge-networking-firewall-backend-and-firewalld:

Firewall backend and firewalld
==============================

.. expectation:: The daemon does not start with an unknown firewall-backend
    :id: BR-FW-BACKEND-INVALID
    :observed_from: log, node
    :depends_on: none
    :platforms: linux
    :refines: REQ-BR-FIREWALL-BACKEND-OPTION, PRIN-FAILS-CLOSED

    *Applies to:* A daemon configured with a :daemon-json:`firewall-backend` other than ``iptables``
    or ``nftables``

    The daemon does not start. The daemon log contains an error.

.. expectation:: With an unknown firewall-backend, the daemon logs that it is invalid
    :id: BR-FW-BACKEND-INVALID-MSG
    :observed_from: log, node
    :depends_on: none
    :platforms: linux
    :exact_form_of: BR-FW-BACKEND-INVALID

    *Applies to:* A daemon configured with a :daemon-json:`firewall-backend` other than ``iptables``
    or ``nftables``

    The daemon does not start. The daemon log contains an error that contains :log:`invalid
    firewall-backend`.

.. expectation:: GET /info reports the firewall backend that the daemon uses
    :id: BR-FW-BACKEND-INFO
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :refines: REQ-BR-FIREWALL-BACKEND-OPTION, REQ-BR-FIREWALL-BACKEND-INFO

    :api-op:`GET /info` at API version 1.49 or later reports :api-field:`FirewallBackend.Driver`.
    Its value starts with ``iptables`` when the daemon's :daemon-json:`firewall-backend` is
    ``iptables`` or not set, and with ``nftables`` when it is ``nftables``.

.. expectation:: Bridge interfaces are in the firewalld zone docker, with docker-forwarding
    :id: BR-FW-FIREWALLD-ZONE
    :observed_from: node
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-FIREWALLD-ZONE

    *Applies to:* A node with firewalld running, a daemon with :daemon-json:`iptables` or
    :daemon-json:`ip6tables` true, a bridge network whose bridge interface the operator has not bound to another firewalld zone, and a firewalld configuration that has neither a zone
    :fwd-zone:`docker` nor a policy :fwd-policy:`docker-forwarding` before the daemon starts

    Firewalld has a zone :fwd-zone:`docker` with target ``ACCEPT``. The network's bridge interface
    is in that zone. Firewalld also has a policy :fwd-policy:`docker-forwarding` with the ingress
    zone ``ANY``, the egress zone :fwd-zone:`docker` and target ``ACCEPT``.

.. expectation:: A bridge joins the docker zone when firewalld starts after the daemon
    :id: BR-FW-FIREWALLD-LATE-START-ZONE
    :observed_from: node
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-FIREWALLD-ZONE, REQ-BR-FIREWALLD-RELOAD

    *Applies to:* A daemon with :daemon-json:`iptables` or :daemon-json:`ip6tables` true that started while firewalld was not running, a bridge network whose bridge interface the operator has not bound to another firewalld zone, and a firewalld configuration that has neither
    a zone :fwd-zone:`docker` nor a policy :fwd-policy:`docker-forwarding` before firewalld starts

    After firewalld starts, it has the zone :fwd-zone:`docker` and the policy
    :fwd-policy:`docker-forwarding`, and the network's bridge interface is in the zone
    :fwd-zone:`docker`.

.. expectation:: Without iptables and ip6tables, a bridge is not put in the docker zone
    :id: BR-FW-FIREWALLD-ZONE-IPTABLES-DISABLED
    :observed_from: node
    :depends_on: os-linux
    :platforms: linux
    :rationale: RAT-BR-FIREWALLD-ZONE-FOLLOWS-IPTABLES

    *Applies to:* A node with firewalld running, and a daemon with :daemon-json:`iptables` and
    :daemon-json:`ip6tables` false

    After :api-op:`POST /networks/create` for a bridge network responds 201, firewalld does not
    report :fwd-zone:`docker` as the zone of the network's bridge interface.

.. expectation:: A bridge the operator bound to another zone stays there and keeps working
    :id: BR-FW-FIREWALLD-ZONE-USER-BOUND
    :observed_from: node, api
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-FIREWALLD-ZONE

    *Applies to:* A node with firewalld running, and a bridge network whose bridge interface the
    operator has bound to a firewalld zone other than :fwd-zone:`docker`

    After the daemon restarts, firewalld reports the operator's zone as the zone of the network's
    bridge interface. :api-op:`POST /containers/create` for a container on the network then responds
    201, and :api-op:`POST /containers/{id}/start` for it responds 204.

.. expectation:: A published port answers after a firewalld reload
    :id: BR-FW-FIREWALLD-RELOAD-PORT
    :observed_from: client
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-FIREWALLD-RELOAD
    :needs: BR-HOST-NO-FORWARD-DROP

    *Applies to:* A node with firewalld running, and a published port that answers a client at an
    address of the node before a firewalld reload

    After a firewalld reload, the port answers that client at that address.

.. expectation:: After a firewalld reload, GET /info reports a ReloadedAt time
    :id: BR-FW-FIREWALLD-RELOADED-AT
    :observed_from: api
    :depends_on: os-linux
    :platforms: linux
    :rationale: RAT-BR-RELOAD-COMPLETION-REPORTED

    *Applies to:* A node with firewalld running

    After a firewalld reload, once the daemon has applied its firewall configuration again,
    :api-op:`GET /info` at API version 1.49 or later reports a :api-field:`ReloadedAt` entry in
    :api-field:`FirewallBackend.Info`. Its time is no earlier than the second in which the reload
    started.

.. expectation:: A container reaches an underlay IPv4 address after a firewalld reload
    :id: BR-FW-FIREWALLD-RELOAD-EGRESS
    :observed_from: ctr
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-FIREWALLD-RELOAD, REQ-BR-EGRESS-NAT
    :needs: BR-HOST-NO-FORWARD-DROP

    *Applies to:* A node with firewalld running, and a running container on a bridge network that is
    not ``internal``, whose :drvopt:`com.docker.network.bridge.gateway_mode_ipv4` is not ``routed``,
    and whose :drvopt:`com.docker.network.bridge.enable_ip_masquerade` is not ``false``

    After a firewalld reload, a connection from the container to an IPv4 address of an underlay host
    succeeds.

.. expectation:: A published port answers after firewalld stops
    :id: BR-FW-FIREWALLD-STOP-PORT
    :observed_from: client
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-FIREWALLD-RELOAD
    :needs: BR-HOST-NO-FORWARD-DROP

    *Applies to:* A node with firewalld running, and a published port that answers a client at an
    address of the node before firewalld stops

    After firewalld stops, the port answers that client at that address.

.. expectation:: A container reaches an underlay IPv4 address after firewalld stops
    :id: BR-FW-FIREWALLD-STOP-EGRESS
    :observed_from: ctr
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-FIREWALLD-RELOAD, REQ-BR-EGRESS-NAT
    :needs: BR-HOST-NO-FORWARD-DROP

    *Applies to:* A node with firewalld running, and a running container on a bridge network that is
    not ``internal``, whose :drvopt:`com.docker.network.bridge.gateway_mode_ipv4` is not ``routed``,
    and whose :drvopt:`com.docker.network.bridge.enable_ip_masquerade` is not ``false``

    After firewalld stops, a connection from the container to an IPv4 address of an underlay host
    succeeds.

.. expectation:: A published port answers after firewalld starts later than the daemon
    :id: BR-FW-FIREWALLD-LATE-START-PORT
    :observed_from: client
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-FIREWALLD-RELOAD
    :needs: BR-HOST-NO-FORWARD-DROP

    *Applies to:* A daemon that started while firewalld was not running, and a published port that
    answers a client at an address of the node before firewalld starts

    After firewalld starts, the port answers that client at that address.

.. expectation:: A container reaches an underlay IPv4 address after firewalld starts
    :id: BR-FW-FIREWALLD-LATE-START-EGRESS
    :observed_from: ctr
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-FIREWALLD-RELOAD, REQ-BR-EGRESS-NAT
    :needs: BR-HOST-NO-FORWARD-DROP

    *Applies to:* A daemon that started while firewalld was not running, and a running container on
    a bridge network that is not ``internal``, whose
    :drvopt:`com.docker.network.bridge.gateway_mode_ipv4` is not ``routed``, and whose
    :drvopt:`com.docker.network.bridge.enable_ip_masquerade` is not ``false``

    After firewalld starts, a connection from the container to an IPv4 address of an underlay host
    succeeds.

.. expectation:: Under nftables, a firewalld reload logs warnings and leaves the tables unchanged
    :id: BR-FW-NFT-FIREWALLD-RELOAD
    :observed_from: log
    :depends_on: none
    :platforms: linux
    :rationale: RAT-BR-RELOAD-NFT-WARNINGS

    *Applies to:* The nftables backend, and a node with firewalld running

    A firewalld reload does not change the daemon's nftables tables. On each reload, and each time
    firewalld starts or stops while the daemon runs, the daemon log contains these warning-level
    entries, each kind exactly once for each object that it names and for no other object: (1) one
    entry per bridge network that contains :log:`ReapplyNetworkLevelRules is not implemented for
    nftables`; (2) one entry per endpoint that contains :log:`Failed to reconfigure Endpoint` and
    :log:`rule exists`, for an endpoint that has an address of an IP version whose gateway mode is
    ``nat``, the default, on a network that is not ``internal``, on a daemon with
    :daemon-json:`allow-direct-routing` false, whatever the number of its IP versions; (3) one entry
    per bridge network whose endpoints publish a port, that contains
    :log:`Failed to reconfigure NAT` and :log:`rule exists`, whatever the number of its ports and
    their IP versions.

.. expectation:: An IPv4 bridge network turns on ip_forward and sets FORWARD to DROP
    :id: BR-FW-FORWARDING-ENABLED
    :observed_from: node
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-IP-FORWARDING-DROP

    *Applies to:* The iptables backend, with :daemon-json:`ip-forward` and :daemon-json:`iptables`
    true and :daemon-json:`ip-forward-no-drop` false, and a host where :sysctl:`net.ipv4.ip_forward`
    is 0 and the filter table's :ipt-chain:`FORWARD` chain has policy ACCEPT

    After the daemon has created a bridge network with IPv4, :sysctl:`net.ipv4.ip_forward` is 1. The
    filter table's :ipt-chain:`FORWARD` chain also has policy DROP.

.. expectation:: An IPv6 bridge network turns on IPv6 forwarding and sets FORWARD to DROP
    :id: BR-FW-FORWARDING-ENABLED-V6
    :observed_from: node
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-IP-FORWARDING-DROP
    :needs: BR-HOST-IP6TABLES-COMMAND, BR-HOST-IP6TABLES-KERNEL, BR-HOST-IP6TABLES-LEGACY-KERNEL, BR-HOST-IP6TABLES-NFT-KERNEL

    *Applies to:* The iptables backend, with :daemon-json:`ip-forward` and :daemon-json:`ip6tables`
    true and :daemon-json:`ip-forward-no-drop` false, and a host where
    :sysctl:`net.ipv6.conf.all.forwarding` or :sysctl:`net.ipv6.conf.default.forwarding` is 0, and
    the ip6tables filter table's :ipt-chain:`FORWARD` chain has policy ACCEPT

    After the daemon has created a bridge network with IPv6, both are 1. The ip6tables filter
    table's :ipt-chain:`FORWARD` chain also has policy DROP.

.. expectation:: With ip-forward-no-drop, enabling ip_forward leaves the FORWARD policy
    :id: BR-FW-FORWARDING-NO-DROP
    :observed_from: node
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-IP-FORWARDING-DROP

    *Applies to:* The iptables backend, with :daemon-json:`ip-forward`, :daemon-json:`iptables` and
    :daemon-json:`ip-forward-no-drop` true, and a host where :sysctl:`net.ipv4.ip_forward` is 0 and
    the filter table's :ipt-chain:`FORWARD` chain has policy ACCEPT

    After the daemon has created a bridge network with IPv4, :sysctl:`net.ipv4.ip_forward` is 1. The
    filter table's :ipt-chain:`FORWARD` chain still has policy ACCEPT.

.. expectation:: With ip-forward-no-drop, enabling IPv6 forwarding leaves the FORWARD policy
    :id: BR-FW-FORWARDING-NO-DROP-V6
    :observed_from: node
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-IP-FORWARDING-DROP
    :needs: BR-HOST-IP6TABLES-COMMAND, BR-HOST-IP6TABLES-KERNEL, BR-HOST-IP6TABLES-LEGACY-KERNEL, BR-HOST-IP6TABLES-NFT-KERNEL

    *Applies to:* The iptables backend, with :daemon-json:`ip-forward`, :daemon-json:`ip6tables` and
    :daemon-json:`ip-forward-no-drop` true, and a host where :sysctl:`net.ipv6.conf.all.forwarding`
    or :sysctl:`net.ipv6.conf.default.forwarding` is 0, and the ip6tables filter table's
    :ipt-chain:`FORWARD` chain has policy ACCEPT

    After the daemon has created a bridge network with IPv6, both are 1. The ip6tables filter
    table's :ipt-chain:`FORWARD` chain still has policy ACCEPT.

.. expectation:: With ip_forward already 1, an IPv4 network leaves the FORWARD policy
    :id: BR-FW-FORWARDING-ALREADY-ENABLED
    :observed_from: node
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-IP-FORWARDING-DROP

    *Applies to:* The iptables backend, with :daemon-json:`ip-forward` and :daemon-json:`iptables`
    true, and a host where :sysctl:`net.ipv4.ip_forward` is 1 and the filter table's
    :ipt-chain:`FORWARD` chain has policy ACCEPT

    After the daemon has created a bridge network with IPv4, the filter table's :ipt-chain:`FORWARD`
    chain still has policy ACCEPT.

.. expectation:: With IPv6 forwarding already on, an IPv6 network leaves the FORWARD policy
    :id: BR-FW-FORWARDING-ALREADY-ENABLED-V6
    :observed_from: node
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-IP-FORWARDING-DROP
    :needs: BR-HOST-IP6TABLES-COMMAND, BR-HOST-IP6TABLES-KERNEL, BR-HOST-IP6TABLES-LEGACY-KERNEL, BR-HOST-IP6TABLES-NFT-KERNEL

    *Applies to:* The iptables backend, with :daemon-json:`ip-forward` and :daemon-json:`ip6tables`
    true, and a host where :sysctl:`net.ipv6.conf.all.forwarding` and
    :sysctl:`net.ipv6.conf.default.forwarding` are 1 and the ip6tables filter table's
    :ipt-chain:`FORWARD` chain has policy ACCEPT

    After the daemon has created a bridge network with IPv6, the ip6tables filter table's
    :ipt-chain:`FORWARD` chain still has policy ACCEPT.

.. expectation:: The FORWARD chain has policy DROP after a firewalld reload
    :id: BR-FW-FORWARD-DROP-FIREWALLD
    :observed_from: node
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-IP-FORWARDING-DROP, REQ-BR-FIREWALLD-RELOAD

    *Applies to:* The iptables backend, with :daemon-json:`ip-forward` and :daemon-json:`iptables`
    true and :daemon-json:`ip-forward-no-drop` false, firewalld running, and a bridge network with
    IPv4 that the daemon created on a host where :sysctl:`net.ipv4.ip_forward` was 0, after which an
    operator set the policy of the filter table's :ipt-chain:`FORWARD` chain to ACCEPT

    After a firewalld reload, the filter table's :ipt-chain:`FORWARD` chain has policy DROP.

.. expectation:: The ip6tables FORWARD chain has policy DROP after a firewalld reload
    :id: BR-FW-FORWARD-DROP-FIREWALLD-V6
    :observed_from: node
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-IP-FORWARDING-DROP, REQ-BR-FIREWALLD-RELOAD
    :needs: BR-HOST-IP6TABLES-COMMAND, BR-HOST-IP6TABLES-KERNEL, BR-HOST-IP6TABLES-LEGACY-KERNEL, BR-HOST-IP6TABLES-NFT-KERNEL

    *Applies to:* The iptables backend, with :daemon-json:`ip-forward` and :daemon-json:`ip6tables`
    true and :daemon-json:`ip-forward-no-drop` false, firewalld running, and a bridge network with
    IPv6 that the daemon created on a host where :sysctl:`net.ipv6.conf.all.forwarding` or
    :sysctl:`net.ipv6.conf.default.forwarding` was 0, after which an operator set the policy of the
    ip6tables filter table's :ipt-chain:`FORWARD` chain to ACCEPT

    After a firewalld reload, the ip6tables filter table's :ipt-chain:`FORWARD` chain has policy
    DROP.

.. expectation:: The FORWARD chain has policy DROP after firewalld stops
    :id: BR-FW-FORWARD-DROP-FIREWALLD-STOP
    :observed_from: node
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-IP-FORWARDING-DROP, REQ-BR-FIREWALLD-RELOAD

    *Applies to:* The iptables backend, with :daemon-json:`ip-forward` and :daemon-json:`iptables`
    true and :daemon-json:`ip-forward-no-drop` false, firewalld running, and a bridge network with
    IPv4 that the daemon created on a host where :sysctl:`net.ipv4.ip_forward` was 0, after which an
    operator set the policy of the filter table's :ipt-chain:`FORWARD` chain to ACCEPT

    After firewalld stops, the filter table's :ipt-chain:`FORWARD` chain has policy DROP.

.. expectation:: The ip6tables FORWARD chain has policy DROP after firewalld stops
    :id: BR-FW-FORWARD-DROP-FIREWALLD-STOP-V6
    :observed_from: node
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-IP-FORWARDING-DROP, REQ-BR-FIREWALLD-RELOAD
    :needs: BR-HOST-IP6TABLES-COMMAND, BR-HOST-IP6TABLES-KERNEL, BR-HOST-IP6TABLES-LEGACY-KERNEL, BR-HOST-IP6TABLES-NFT-KERNEL

    *Applies to:* The iptables backend, with :daemon-json:`ip-forward` and :daemon-json:`ip6tables`
    true and :daemon-json:`ip-forward-no-drop` false, firewalld running, and a bridge network with
    IPv6 that the daemon created on a host where :sysctl:`net.ipv6.conf.all.forwarding` or
    :sysctl:`net.ipv6.conf.default.forwarding` was 0, after which an operator set the policy of the
    ip6tables filter table's :ipt-chain:`FORWARD` chain to ACCEPT

    After firewalld stops, the ip6tables filter table's :ipt-chain:`FORWARD` chain has policy DROP.

.. expectation:: The FORWARD chain has policy DROP after firewalld starts later
    :id: BR-FW-FORWARD-DROP-FIREWALLD-LATE-START
    :observed_from: node
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-IP-FORWARDING-DROP, REQ-BR-FIREWALLD-RELOAD

    *Applies to:* The iptables backend, with :daemon-json:`ip-forward` and :daemon-json:`iptables`
    true and :daemon-json:`ip-forward-no-drop` false, a daemon that started while firewalld was not
    running, and a bridge network with IPv4 that the daemon created on a host where
    :sysctl:`net.ipv4.ip_forward` was 0, after which an operator set the policy of the filter
    table's :ipt-chain:`FORWARD` chain to ACCEPT

    After firewalld starts, the filter table's :ipt-chain:`FORWARD` chain has policy DROP.

.. expectation:: The ip6tables FORWARD chain has policy DROP after firewalld starts later
    :id: BR-FW-FORWARD-DROP-FIREWALLD-LATE-START-V6
    :observed_from: node
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-IP-FORWARDING-DROP, REQ-BR-FIREWALLD-RELOAD
    :needs: BR-HOST-IP6TABLES-COMMAND, BR-HOST-IP6TABLES-KERNEL, BR-HOST-IP6TABLES-LEGACY-KERNEL, BR-HOST-IP6TABLES-NFT-KERNEL

    *Applies to:* The iptables backend, with :daemon-json:`ip-forward` and :daemon-json:`ip6tables`
    true and :daemon-json:`ip-forward-no-drop` false, a daemon that started while firewalld was not
    running, and a bridge network with IPv6 that the daemon created on a host where
    :sysctl:`net.ipv6.conf.all.forwarding` or :sysctl:`net.ipv6.conf.default.forwarding` was 0,
    after which an operator set the policy of the ip6tables filter table's :ipt-chain:`FORWARD`
    chain to ACCEPT

    After firewalld starts, the ip6tables filter table's :ipt-chain:`FORWARD` chain has policy DROP.

.. expectation:: With nftables, the daemon does not start while ip_forward is 0
    :id: BR-FW-NFT-NO-FORWARDING
    :observed_from: log, node
    :depends_on: none
    :platforms: linux
    :provisional_on: REQ-BR-NFT-IP-FORWARDING
    :pins: BR-HOST-FORWARDING-V4

    *Applies to:* A daemon with the nftables backend, :daemon-json:`ip-forward` true and the default
    bridge enabled, on a host where :sysctl:`net.ipv4.ip_forward` is 0, and without containers that
    :daemon-json:`live-restore` keeps running

    The daemon does not start. The daemon log contains an error that contains :log:`IPv4 forwarding
    is disabled`.

.. expectation:: With nftables, an IPv4 network is refused while ip_forward is 0
    :id: BR-FW-NFT-NO-FORWARDING-NETWORK
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :provisional_on: REQ-BR-NFT-IP-FORWARDING
    :pins: BR-HOST-FORWARDING-V4

    *Applies to:* A daemon with the nftables backend, :daemon-json:`ip-forward` true and
    :daemon-json:`bridge` set to ``none``, on a host where :sysctl:`net.ipv4.ip_forward` is 0

    :api-op:`POST /networks/create` for a bridge network with IPv4 responds with a 4xx or 5xx
    status, with a message that contains :api-error:`IPv4 forwarding is disabled`.
    :api-op:`GET /networks` then does not list a network with the request's :api-field:`Name`.

.. expectation:: With nftables, an IPv6 network is refused while forwarding is off
    :id: BR-FW-NFT-NO-FORWARDING-V6
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :provisional_on: REQ-BR-NFT-IP-FORWARDING
    :pins: BR-HOST-FORWARDING-V6

    *Applies to:* A daemon with the nftables backend and :daemon-json:`ip-forward` true, on a host
    where :sysctl:`net.ipv6.conf.all.forwarding` or :sysctl:`net.ipv6.conf.default.forwarding` is 0

    :api-op:`POST /networks/create` for a bridge network with :api-field:`EnableIPv6` set responds
    with a 4xx or 5xx status, with a message that contains
    :api-error:`IPv6 global forwarding is disabled`. :api-op:`GET /networks` then does not list a
    network with the request's :api-field:`Name`.

.. expectation:: With nftables, an IPv6 network responds 500 while forwarding is off
    :id: BR-FW-NFT-NO-FORWARDING-V6-HTTP500
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :exact_form_of: BR-FW-NFT-NO-FORWARDING-V6

    *Applies to:* A daemon with the nftables backend and :daemon-json:`ip-forward` true, on a host
    where :sysctl:`net.ipv6.conf.all.forwarding` or :sysctl:`net.ipv6.conf.default.forwarding` is 0

    :api-op:`POST /networks/create` for a bridge network with :api-field:`EnableIPv6` set responds
    500.

.. expectation:: With nftables and ip-forward false, the daemon does not check forwarding
    :id: BR-FW-NFT-FORWARDING-UNCHECKED
    :observed_from: api, node
    :depends_on: none
    :platforms: linux
    :provisional_on: REQ-BR-NFT-IP-FORWARDING
    :needs: BR-HOST-NFTABLES-V6

    *Applies to:* A daemon with the nftables backend and :daemon-json:`ip-forward` false, on a host
    where :sysctl:`net.ipv4.ip_forward`, :sysctl:`net.ipv6.conf.all.forwarding` and
    :sysctl:`net.ipv6.conf.default.forwarding` are 0, and without containers that
    :daemon-json:`live-restore` keeps running

    The daemon starts. :api-op:`POST /networks/create` for a bridge network with
    :api-field:`EnableIPv6` set responds 201. The three sysctls are still 0.

.. expectation:: Without the iptables command, the daemon does not start
    :id: BR-FW-IPTABLES-MISSING
    :observed_from: log, node
    :depends_on: none
    :platforms: linux
    :refines: PRIN-FAILS-CLOSED
    :pins: BR-HOST-IPTABLES-COMMAND, SWARM-HOST-IPTABLES-COMMAND

    *Applies to:* A daemon with the iptables backend and :daemon-json:`iptables` true, on a host
    without the :program:`iptables` command

    The daemon does not start. The daemon log contains an error.

.. expectation:: With no iptables command, the daemon logs that iptables is not found
    :id: BR-FW-IPTABLES-MISSING-MSG
    :observed_from: log, node
    :depends_on: none
    :platforms: linux
    :exact_form_of: BR-FW-IPTABLES-MISSING
    :pins: BR-HOST-IPTABLES-COMMAND, SWARM-HOST-IPTABLES-COMMAND

    *Applies to:* A daemon with the iptables backend and :daemon-json:`iptables` true, on a host
    without the :program:`iptables` command

    The daemon does not start. The daemon log contains an error that contains :log:`iptables not
    found`.

.. expectation:: With no ip6tables command, the daemon starts
    :id: BR-FW-IP6TABLES-MISSING
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :refines: REQ-BR-IPV6-UNAVAILABLE
    :pins: BR-HOST-IP6TABLES-COMMAND

    *Applies to:* A daemon with the iptables backend and :daemon-json:`ip6tables` true, on a host
    without the :program:`ip6tables` command

    The daemon starts.

.. expectation:: With no ip6tables command, the daemon logs that it cannot set up chains
    :id: BR-FW-IP6TABLES-MISSING-MSG
    :observed_from: log
    :depends_on: none
    :platforms: linux
    :pins: BR-HOST-IP6TABLES-COMMAND

    *Applies to:* A daemon with the iptables backend and :daemon-json:`ip6tables` true, on a host
    without the :program:`ip6tables` command

    The daemon log contains :log:`ip6tables is enabled, but cannot set up ip6tables chains`.

.. expectation:: Without the ip6tables command, an IPv6 bridge network is refused
    :id: BR-FW-IP6TABLES-MISSING-NETWORK
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :refines: REQ-BR-IPV6-UNAVAILABLE, PRIN-FAILS-CLOSED, REQ-NET-NETWORK-CREATE-ERRORS
    :pins: BR-HOST-IP6TABLES-COMMAND

    *Applies to:* A daemon with the iptables backend and :daemon-json:`ip6tables` true, on a host
    without the :program:`ip6tables` command

    :api-op:`POST /networks/create` for a bridge network with :api-field:`EnableIPv6` set responds
    with a 4xx or 5xx status. :api-op:`GET /networks` then does not list a network with the
    request's :api-field:`Name`.

.. expectation:: Without the ip6tables command, a port is reachable over IPv4
    :id: BR-FW-IP6TABLES-MISSING-PUBLISHED-PORT
    :observed_from: client
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-IPV6-UNAVAILABLE, REQ-BR-PUBLISH-DEFAULTS
    :pins: BR-HOST-IP6TABLES-COMMAND

    *Applies to:* A daemon with the iptables backend and :daemon-json:`ip6tables` true, on a host
    without the :program:`ip6tables` command, and a running container on a bridge network without
    IPv6 that publishes a port without a host address

    A client reaches the container on that port at the node's IPv4 address.

.. expectation:: Without IPv6 nftables support, the nftables backend starts and logs it
    :id: BR-FW-NFT-IPV6-MISSING
    :observed_from: api, log
    :depends_on: none
    :platforms: linux
    :pins: BR-HOST-NFTABLES-V6

    *Applies to:* The nftables backend with :daemon-json:`ip6tables` true, on a kernel without
    :kconfig:`NF_TABLES_IPV6`

    The daemon starts. The daemon log contains :log:`ip6tables is enabled, but cannot set up IPv6
    nftables table`.

.. expectation:: Without IPv6 nftables support, an IPv6 bridge network is refused
    :id: BR-FW-NFT-IPV6-MISSING-NETWORK
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :refines: PRIN-FAILS-CLOSED, REQ-NET-NETWORK-CREATE-ERRORS
    :pins: BR-HOST-NFTABLES-V6

    *Applies to:* The nftables backend with :daemon-json:`ip6tables` true, on a kernel without
    :kconfig:`NF_TABLES_IPV6`

    :api-op:`POST /networks/create` for a bridge network with :api-field:`EnableIPv6` set responds
    with a 4xx or 5xx status. :api-op:`GET /networks` then does not list a network with the
    request's :api-field:`Name`.

.. expectation:: Without the nft command, the daemon does not start with nftables
    :id: BR-FW-NFT-MISSING
    :observed_from: log, node
    :depends_on: none
    :platforms: linux
    :refines: PRIN-FAILS-CLOSED
    :pins: BR-HOST-NFT-COMMAND, SWARM-HOST-NFT-COMMAND, DNS-HOST-NFT-COMMAND

    *Applies to:* A daemon built without the ``libnftables`` build tag, with the nftables backend,
    on a host without the :program:`nft` command

    The daemon does not start. The daemon log contains an error.

.. expectation:: With no nft command, the daemon logs that it failed to find the nft tool
    :id: BR-FW-NFT-MISSING-MSG
    :observed_from: log, node
    :depends_on: none
    :platforms: linux
    :exact_form_of: BR-FW-NFT-MISSING
    :pins: BR-HOST-NFT-COMMAND, SWARM-HOST-NFT-COMMAND, DNS-HOST-NFT-COMMAND

    *Applies to:* A daemon built without the ``libnftables`` build tag, with the nftables backend,
    on a host without the :program:`nft` command

    The daemon does not start. The daemon log contains an error that contains :log:`failed to find
    nft tool`.

.. _bridge-networking-rule-content:

Rule content
============

The expectations in this subsection pin the content of the firewall rules that the daemon programs
in the node's host network namespace, so that a change to it is visible in review.

.. expectation:: The node's IPv4 iptables rules match the reference rulesets
    :id: BR-RULES-REFERENCE
    :observed_from: node
    :depends_on: none
    :platforms: linux
    :pins: BR-HOST-IPTABLES-COMMAND
    :needs: BR-HOST-IPTABLES-COMMAND
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* The iptables backend

    For each configuration that the repository keeps a reference iptables ruleset for in
    :repo-path:`integration/network/bridge/iptablesdoc`, the IPv4 iptables rules and chains in the
    node's host network namespace match that reference, in every table and chain that it shows,
    including those that parts of the daemon other than the bridge driver program, such as Swarm's.
    The reference gives the matches and target of each rule, the order of the rules in each chain,
    and the names of the chains. The configurations include a Swarm service that publishes a port.

.. expectation:: The IPv4 docker-bridges table matches the reference rulesets
    :id: BR-RULES-REFERENCE-NFT
    :observed_from: node
    :depends_on: none
    :platforms: linux
    :pins: BR-HOST-NFTABLES
    :needs: BR-HOST-NFTABLES
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* The nftables backend

    For each configuration that the repository keeps a reference nftables ruleset for in
    :repo-path:`integration/network/bridge/nftablesdoc`, the IPv4 :nft-table:`docker-bridges` table
    in the node's host network namespace matches the reference's listing of that table. The tables
    that the reference lists for other network namespaces are not part of this expectation. The
    reference gives the matches and verdict of each rule, the order of the rules in each chain, and the table and chain names. The configurations include a Swarm service that publishes a port.

.. _bridge-networking-network-and-endpoint-options:

******************************
 Network and endpoint options
******************************

In this section, the parts of expectations about values below −2³¹ or from 2³¹ up hold for a daemon
built for a 64-bit architecture.

.. expectation:: A network MTU that is not an integer in range responds 400
    :id: BR-OPT-MTU-INVALID
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :refines: REQ-BR-NETWORK-MTU, REQ-NET-NETWORK-CREATE-ERRORS, REQ-NET-API-ERROR-STATUS, PRIN-FAILS-CLOSED

    :api-op:`POST /networks/create` for a bridge network whose
    :drvopt:`com.docker.network.driver.mtu` is not an integer from 0 to 2⁶³ − 1 responds 400.

.. expectation:: A refused network MTU says the value is unparsable or an invalid MTU
    :id: BR-OPT-MTU-INVALID-MSG
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :exact_form_of: BR-OPT-MTU-INVALID

    :api-op:`POST /networks/create` for a bridge network whose
    :drvopt:`com.docker.network.driver.mtu` is not an integer from −2⁶³ to 2⁶³ − 1 responds with a
    message that contains :api-error:`failed to parse com.docker.network.driver.mtu value: <value>`.
    For one whose option is an integer from −2⁶³ to −1, the message contains :api-error:`invalid MTU
    number: <value>`. ``<value>`` is the option's value.

.. expectation:: docker network create with an invalid MTU exits non-zero
    :id: BR-OPT-MTU-INVALID-CLI
    :observed_from: api
    :depends_on: cli
    :platforms: linux
    :refines: REQ-BR-NETWORK-MTU, REQ-NET-NETWORK-CREATE-ERRORS, PRIN-FAILS-CLOSED
    :cli_for: BR-OPT-MTU-INVALID

    :cli:`docker network create --driver bridge` with a :drvopt:`com.docker.network.driver.mtu`
    option that is not an integer from 0 to 2⁶³ − 1 exits non-zero.

.. expectation:: A network with an MTU that the kernel or IPv6 cannot use is not created
    :id: BR-OPT-MTU-REFUSED
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :refines: REQ-BR-NETWORK-MTU, REQ-NET-NETWORK-CREATE-ERRORS, PRIN-FAILS-CLOSED

    :api-op:`POST /networks/create` for a bridge network with an MTU from 1 to 2⁶³ − 1 that the
    kernel refuses, or with :api-field:`EnableIPv6` set and an MTU from 1 to 1279, responds with a
    4xx or 5xx status. :api-op:`GET /networks` then does not list a network with the request's
    :api-field:`Name`.

.. expectation:: A network MTU that the kernel or IPv6 cannot use responds 500
    :id: BR-OPT-MTU-REFUSED-HTTP500
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :exact_form_of: BR-OPT-MTU-REFUSED
    :pins: BR-OPT-HTTP500-REFUSED

    :api-op:`POST /networks/create` for a bridge network with an MTU from 1 to 2⁶³ − 1 that the
    kernel refuses, or with :api-field:`EnableIPv6` set and an MTU from 1 to 1279, responds 500.

.. expectation:: docker network create exits non-zero for an MTU the kernel or IPv6 cannot use
    :id: BR-OPT-MTU-REFUSED-CLI
    :observed_from: api
    :depends_on: cli
    :platforms: linux
    :refines: REQ-BR-NETWORK-MTU, REQ-NET-NETWORK-CREATE-ERRORS, PRIN-FAILS-CLOSED
    :cli_for: BR-OPT-MTU-REFUSED

    :cli:`docker network create --driver bridge` with an MTU from 1 to 2⁶³ − 1 that the kernel
    refuses, or with :cli-flag:`--ipv6` and an MTU from 1 to 1279, exits non-zero.
    :cli:`docker network ls` then does not list a network with the name that the command gave.

.. expectation:: A bridge MTU that the kernel or IPv6 cannot use is refused with 500
    :id: BR-OPT-HTTP500-REFUSED
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :concerns: BR-OPT-MTU-REFUSED
    :violates: REQ-NET-API-ERROR-STATUS, REQ-NET-NETWORK-CREATE-ERRORS

    :api-op:`POST /networks/create` for a bridge network with an MTU from 1 to 2³² − 1 that the
    kernel refuses, or from 2³² to 2⁶³ − 1 with a remainder modulo 2³² that the kernel refuses, is
    refused and responds 500. So is a request with :api-field:`EnableIPv6` set and an MTU from 1 to
    1279, or from 2³² to 2⁶³ − 1 with a remainder modulo 2³² from 1 to 1279.

.. expectation:: A refused bridge MTU gives an error that does not name the option
    :id: BR-OPT-MTU-OUT-OF-RANGE
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :concerns: BR-OPT-MTU-REFUSED, BR-OPT-MTU-REFUSED-HTTP500
    :violates: PRIN-ERRORS-ACTIONABLE

    :api-op:`POST /networks/create` for a bridge network with an MTU from 1 to 2³² − 1 that the
    kernel refuses, or from 2³² to 2⁶³ − 1 with a remainder modulo 2³² that the kernel refuses, is
    refused with a message that contains :api-error:`invalid argument`. A request with
    :api-field:`EnableIPv6` set and an MTU below 1280 that the kernel accepts, or a value from 2³²
    to 2⁶³ − 1 whose remainder modulo 2³² is such an MTU, is refused with a message that contains
    :api-error:`Cannot read IPv6 setup for bridge` and :api-error:`no such file or directory`.
    Neither message names the option.

.. expectation:: A network's positive MTU applies to its containers' interfaces
    :id: BR-OPT-MTU-APPLIED
    :observed_from: ctr
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-NETWORK-MTU

    *Applies to:* A bridge network whose :drvopt:`com.docker.network.driver.mtu` is positive

    The interfaces of the network's containers have that MTU.

.. expectation:: A network MTU of 0 leaves an existing bridge's MTU
    :id: BR-OPT-MTU-ZERO
    :observed_from: node, ctr
    :depends_on: none
    :platforms: linux
    :refines: REQ-BR-NETWORK-MTU

    A bridge network's :drvopt:`com.docker.network.driver.mtu` of 0 sets no MTU. When
    :api-op:`POST /networks/create` responds, a bridge device that existed before the request with
    an MTU other than 1500, such as 9000, still has that MTU. A bridge device that the request adds
    has MTU 1500, the kernel's default. The interfaces of the network's containers have MTU 1500,
    the kernel's default for a new veth device.

.. expectation:: The daemon does not start with a negative mtu
    :id: BR-OPT-DAEMON-MTU-NEGATIVE
    :observed_from: log, node
    :depends_on: none
    :platforms: linux
    :refines: PRIN-FAILS-CLOSED

    *Applies to:* A daemon configured with an :daemon-json:`mtu` from −2⁶³ to −1

    The daemon does not start. The daemon log contains an error.

.. expectation:: With a negative mtu, the daemon logs that the default MTU is invalid
    :id: BR-OPT-DAEMON-MTU-NEGATIVE-MSG
    :observed_from: log, node
    :depends_on: none
    :platforms: linux
    :exact_form_of: BR-OPT-DAEMON-MTU-NEGATIVE

    *Applies to:* A daemon configured with an :daemon-json:`mtu` from −2⁶³ to −1

    The daemon does not start. The daemon log contains an error that contains :log:`invalid default
    MTU`.

.. expectation:: The daemon does not start with an mtu that is not a 64-bit integer
    :id: BR-OPT-DAEMON-MTU-INVALID
    :observed_from: log, node
    :depends_on: none
    :platforms: linux
    :refines: PRIN-FAILS-CLOSED

    *Applies to:* A daemon configured with an :daemon-json:`mtu` below −2⁶³ or from 2⁶³, or with one
    that is not an integer, such as ``"1500"`` or ``1500.5``

    The daemon does not start. The daemon log contains an error.

.. expectation:: The daemon does not start with an mtu the default bridge cannot use
    :id: BR-OPT-DAEMON-MTU-REFUSED
    :observed_from: log, node
    :depends_on: none
    :platforms: linux
    :refines: PRIN-FAILS-CLOSED

    *Applies to:* A daemon with the default bridge enabled, configured with an :daemon-json:`mtu`
    from 1 to 2⁶³ − 1 that the kernel cannot apply to the default bridge, or with
    :daemon-json:`ipv6` true and an mtu from 1 to 1279, and without containers that
    :daemon-json:`live-restore` keeps running

    The daemon does not start. The daemon log contains an error.

.. expectation:: An unusable daemon mtu stops the daemon without naming the setting
    :id: BR-OPT-DAEMON-MTU-ERROR
    :observed_from: log, node
    :depends_on: none
    :platforms: linux
    :concerns: BR-OPT-DAEMON-MTU-REFUSED
    :violates: PRIN-ERRORS-ACTIONABLE

    *Applies to:* A daemon with the default bridge enabled, configured with an :daemon-json:`mtu`
    from 1 to 2³² − 1 that the kernel cannot apply to the default bridge, or from 2³² to 2⁶³ − 1
    with a remainder modulo 2³² that the kernel refuses, or with :daemon-json:`ipv6` true and an mtu
    from 1 to 1279 that the kernel can apply or an mtu from 2³² to 2⁶³ − 1 whose remainder modulo
    2³² is such an MTU, and without containers that :daemon-json:`live-restore` keeps running

    The daemon does not start. When the kernel refuses the MTU that the daemon gives it, which for
    an :daemon-json:`mtu` from 2³² to 2⁶³ − 1 is its remainder modulo 2³², the daemon log contains
    an error that ends in :log:`error creating default "bridge" network: invalid argument`.
    Otherwise it contains an error that contains :log:`error creating default "bridge" network:
    Cannot read IPv6 setup for bridge` and :log:`no such file or directory`. Neither error names the
    setting.

.. expectation:: With live-restored containers, the daemon's default-bridge settings are not applied
    :id: BR-OPT-DAEMON-LIVE-RESTORE-UNAPPLIED
    :observed_from: api, node, ctr, ctr → ctr, client
    :depends_on: os-linux
    :platforms: linux
    :narrows: BR-OPT-DAEMON-BIP, BR-OPT-DAEMON-BIP6, BR-ISO-SAME-NETWORK, BR-ISO-NO-ICC-LEGACY-LINK, BR-ISO-DAEMON-NO-ICC, BR-PUB-HOST-BINDING-ADDRESS-DAEMON, BR-EGRESS-DAEMON-NO-MASQUERADE, NET-HOSTS-HOST-GATEWAY-NO-BRIDGE, NET-HOSTS-HOST-GATEWAY-NO-BRIDGE-HTTP500, NET-HOSTS-HOST-GATEWAY-NO-BRIDGE-CLI
    :concerns: BR-OPT-DAEMON-MTU-REFUSED, BR-OPT-DAEMON-MTU-APPLIED, BR-OPT-DAEMON-MTU-DEFAULT, BR-OPT-DAEMON-MTU-ZERO, BR-OPT-DAEMON-MTU-ZERO-FLAG, BR-OPT-DAEMON-MTU-MODULO, BR-OPT-DAEMON-BRIDGE-NONE
    :violates: PRIN-FAILS-CLOSED, REQ-BR-DAEMON-MTU, REQ-BR-ICC, REQ-BR-HOST-BINDING-ADDRESS, REQ-BR-EGRESS-NAT, REQ-BR-DEFAULT-BRIDGE-ADDRESSING, REQ-BR-DAEMON-BRIDGE-NONE

    *Applies to:* A daemon with containers that :daemon-json:`live-restore` keeps running

    After the daemon starts, whatever its :daemon-json:`bridge`, :daemon-json:`bip`,
    :daemon-json:`bip6`, :daemon-json:`default-gateway`, :daemon-json:`default-gateway-v6`,
    :daemon-json:`fixed-cidr`, :daemon-json:`fixed-cidr-v6`, :daemon-json:`icc`, :daemon-json:`ip`,
    :daemon-json:`ip-masq`, :daemon-json:`ipv6` and :daemon-json:`mtu` settings, :api-op:`GET
    /networks` lists :network:`bridge` exactly when the default bridge existed before the start,
    also with :daemon-json:`bridge` set to ``none``, and :api-op:`GET /networks/bridge` reports the
    :api-field:`EnableIPv6`, :api-field:`IPAM` and :api-field:`Options` that the default bridge had
    before the start. The default bridge keeps the MTU that it had before the start, and a container
    that joins it after the start gets the MTU that a container joining it got before the start. Two
    containers on the default bridge that no legacy link joins reach each other at their addresses
    on the network, on any port that they listen on, when the default bridge had ICC enabled before
    the start, and on no port when it had ICC disabled. When the default bridge's
    :drvopt:`com.docker.network.bridge.host_binding_ipv4` was ``0.0.0.0`` before the start, a port
    that a container on it publishes without a host address is reachable at any address of the node,
    also with :daemon-json:`ip` set to one of them. With :daemon-json:`bridge` set to ``none`` and
    without :daemon-json:`host-gateway-ips`, when the default bridge existed before the start, a
    container with an entry ``<name>:host-gateway`` in :api-field:`HostConfig.ExtraHosts` starts,
    and its :file:`/etc/hosts` maps ``<name>`` as :need:`NET-HOSTS-HOST-GATEWAY-DEFAULT-BRIDGE`
    states. When its other settings let it start without such containers, the daemon starts with an
    :daemon-json:`mtu` from 0 to 2⁶³ − 1, or without one, also when the kernel cannot apply the mtu
    to the default bridge and, with :daemon-json:`ipv6` true, for an mtu from 1 to 1279.

.. expectation:: A positive daemon mtu applies to containers on the default bridge
    :id: BR-OPT-DAEMON-MTU-APPLIED
    :observed_from: ctr
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-DAEMON-MTU, REQ-NET-DAEMON-JSON-KEYS

    *Applies to:* A daemon configured with a positive :daemon-json:`mtu` that the kernel can apply
    to the default bridge, and that is 1280 or more when :daemon-json:`ipv6` is true, without
    containers that :daemon-json:`live-restore` keeps running

    The interfaces of containers on the default bridge have that MTU.

.. expectation:: A bridge or daemon MTU from 2³² up is applied modulo 2³²
    :id: BR-OPT-MTU-MODULO
    :observed_from: api, ctr, node
    :depends_on: none
    :platforms: linux
    :narrows: BR-OPT-MTU-APPLIED, BR-OPT-MTU-REFUSED, BR-OPT-MTU-REFUSED-HTTP500, BR-OPT-MTU-REFUSED-CLI, BR-OPT-DAEMON-MTU-REFUSED
    :concerns: BR-OPT-DAEMON-MTU-APPLIED, BR-OPT-MTU-INVALID
    :violates: PRIN-FAILS-CLOSED, REQ-BR-NETWORK-MTU, REQ-BR-DAEMON-MTU, REQ-NET-NETWORK-CREATE-ERRORS

    A value from 2³² to 2⁶³ − 1 of :drvopt:`com.docker.network.driver.mtu` is applied modulo 2³².
    :api-op:`POST /networks/create` accepts such a value when the kernel accepts its remainder as an
    MTU and, with :api-field:`EnableIPv6` set, that remainder is 1280 or more. For example,
    4294968796 is applied as 1500. :api-op:`GET /networks/{id}` then reports the requested value in
    :api-field:`Options`. :api-op:`POST /networks/create` refuses such a value when the kernel
    refuses its remainder as an MTU, or when :api-field:`EnableIPv6` is set and that remainder is
    below 1280 (:need:`BR-OPT-MTU-OUT-OF-RANGE`). A daemon-wide :daemon-json:`mtu` from 2³² to
    2⁶³ − 1 does not stop the daemon when the kernel accepts its remainder as an MTU and, with
    :daemon-json:`ipv6` true, that remainder is 1280 or more. Without containers that
    :daemon-json:`live-restore` keeps running, the daemon then sets that remainder as the MTU of the
    default bridge and of its containers' interfaces.

.. expectation:: Without an mtu setting, the default bridge gets MTU 1500
    :id: BR-OPT-DAEMON-MTU-DEFAULT
    :observed_from: node, ctr
    :depends_on: none
    :platforms: linux
    :refines: REQ-BR-DAEMON-MTU

    *Applies to:* A daemon without :daemon-json:`mtu` in :file:`daemon.json` and without
    :dockerd-flag:`--mtu`, and without containers that :daemon-json:`live-restore` keeps running

    The default bridge and its containers' interfaces have MTU 1500. This includes a default bridge
    device that existed before the start with another MTU.

.. expectation:: With mtu 0 in daemon.json, the default bridge gets MTU 1500
    :id: BR-OPT-DAEMON-MTU-ZERO
    :observed_from: node, ctr
    :depends_on: none
    :platforms: linux
    :pins: BR-OPT-DAEMON-MTU-ZERO-REPLACED

    *Applies to:* A daemon whose :file:`daemon.json` sets :daemon-json:`mtu` to 0, started without
    :dockerd-flag:`--mtu`, and without containers that :daemon-json:`live-restore` keeps running

    The default bridge and its containers' interfaces have MTU 1500. This includes a default bridge
    device that existed before the start with another MTU.

.. expectation:: With --mtu 0, an existing default bridge keeps its MTU
    :id: BR-OPT-DAEMON-MTU-ZERO-FLAG
    :observed_from: node, ctr
    :depends_on: none
    :platforms: linux
    :refines: REQ-BR-DAEMON-MTU

    *Applies to:* A daemon started with :dockerd-flag:`--mtu 0`, without containers that
    :daemon-json:`live-restore` keeps running

    When the daemon's API responds after the start, a default bridge device that existed before the
    start with an MTU other than 1500, such as one whose MTU an operator set with :program:`ip`
    while the daemon was stopped, still has that MTU. A default bridge device that the daemon adds
    at the start has MTU 1500, the kernel's default. The interfaces of the default bridge's
    containers have MTU 1500, the kernel's default for a new veth device.

.. expectation:: An mtu of 0 in daemon.json is replaced by 1500
    :id: BR-OPT-DAEMON-MTU-ZERO-REPLACED
    :observed_from: api, node, ctr
    :depends_on: none
    :platforms: linux
    :concerns: BR-OPT-DAEMON-MTU-ZERO, BR-OPT-DAEMON-MTU-APPLIED, BR-OPT-DAEMON-MTU-ZERO-FLAG
    :violates: REQ-BR-DAEMON-MTU, REQ-NET-DAEMON-JSON-KEYS

    *Applies to:* A daemon without containers that :daemon-json:`live-restore` keeps running

    With :daemon-json:`mtu` 0 in daemon.json, :api-op:`GET /networks/bridge` reports
    :drvopt:`com.docker.network.driver.mtu` ``1500``. The daemon then sets 1500 on the default
    bridge, a default bridge device that existed before the start with an MTU other than 1500
    included, and on its containers' interfaces. Only :dockerd-flag:`--mtu 0` leaves such a bridge
    device's MTU as it was.

.. expectation:: A daemon mtu of 2³² or more sets its remainder modulo 2³² as the MTU
    :id: BR-OPT-DAEMON-MTU-MODULO
    :observed_from: node
    :depends_on: none
    :platforms: linux
    :pins: BR-OPT-MTU-MODULO

    *Applies to:* A daemon configured with an :daemon-json:`mtu` from 2³² to 2⁶³ − 1, where the
    kernel accepts the remainder modulo 2³² as an MTU for the default bridge and, with
    :daemon-json:`ipv6` true, that remainder is 1280 or more, and without containers that
    :daemon-json:`live-restore` keeps running

    The daemon starts. The default bridge gets that remainder as its MTU. For example, 4294968796
    gives an MTU of 1500.

.. expectation:: The daemon's bip, fixed-cidr and default-gateway address the default bridge
    :id: BR-OPT-DAEMON-BIP
    :observed_from: api, node, ctr
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-DEFAULT-BRIDGE-ADDRESSING

    *Applies to:* A daemon with :daemon-json:`bip` set to an IPv4 address and prefix length, such as
    ``192.168.1.1/24``, whose address is neither the first nor the last address of its subnet,
    :daemon-json:`fixed-cidr` set to that subnet or to a subnet within it, such as
    ``192.168.1.0/25``, and :daemon-json:`default-gateway` set to another address of that subnet
    that is neither its first nor its last address, such as ``192.168.1.254``

    :api-op:`GET /networks/bridge` reports an entry in :api-field:`IPAM.Config` whose
    :api-field:`Subnet` is the subnet of :daemon-json:`bip`, whose :api-field:`Gateway` is the
    address of :daemon-json:`bip`, and whose :api-field:`IPRange` is :daemon-json:`fixed-cidr`. The
    Linux bridge of the default bridge network has the address of :daemon-json:`bip`. A container
    that starts on the default bridge gets an IPv4 address in :daemon-json:`fixed-cidr`. When the
    default bridge is the container's only network, its IPv4 default route is via the address of
    :daemon-json:`default-gateway`.

.. expectation:: The daemon's ipv6, bip6 and fixed-cidr-v6 give the default bridge IPv6
    :id: BR-OPT-DAEMON-BIP6
    :observed_from: api, node, ctr
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-DEFAULT-BRIDGE-ADDRESSING
    :needs: BR-HOST-FORWARDING-V6, BR-HOST-IP6TABLES-COMMAND, BR-HOST-IP6TABLES-KERNEL, BR-HOST-IP6TABLES-LEGACY-KERNEL, BR-HOST-IP6TABLES-NFT-KERNEL, BR-HOST-NFTABLES-V6

    *Applies to:* A Linux node that has IPv6 enabled, and a daemon with :daemon-json:`ipv6` true,
    :daemon-json:`bip6` set to a global or unique-local IPv6 address and prefix length, such as
    ``2001:db8::1111/64``, whose address is not the first address of its subnet,
    :daemon-json:`fixed-cidr-v6` set to that subnet or to a subnet within it, and
    :daemon-json:`default-gateway-v6` unset or set to another address of that subnet that is not its
    first address

    :api-op:`GET /networks/bridge` reports :api-field:`EnableIPv6` true and an entry in
    :api-field:`IPAM.Config` whose :api-field:`Subnet` is the subnet of :daemon-json:`bip6`, whose
    :api-field:`Gateway` is the address of :daemon-json:`bip6`, and whose :api-field:`IPRange` is
    :daemon-json:`fixed-cidr-v6`. The Linux bridge of the default bridge network has the address of
    :daemon-json:`bip6`. A container that starts on the default bridge, with IPv6 enabled in its
    network namespace and on its interface on the default bridge, gets an IPv6 address in
    :daemon-json:`fixed-cidr-v6`. When the default bridge is that container's only network, its IPv6
    default route is via the address of :daemon-json:`default-gateway-v6` when that is set, and
    otherwise via the address of :daemon-json:`bip6`.

.. expectation:: With the daemon's bridge set to none, there is no default bridge network
    :id: BR-OPT-DAEMON-BRIDGE-NONE
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :refines: REQ-BR-DAEMON-BRIDGE-NONE

    *Applies to:* A daemon with :daemon-json:`bridge` set to ``none``, without containers that
    :daemon-json:`live-restore` keeps running

    After the daemon starts, :api-op:`GET /networks` does not list :network:`bridge`.

.. expectation:: A network's bridge name option names its bridge interface
    :id: BR-OPT-BRIDGE-NAME
    :observed_from: node
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-BRIDGE-NAME

    *Applies to:* A node without an interface named ``<name>``

    :api-op:`POST /networks/create` for a bridge network with
    :drvopt:`com.docker.network.bridge.name` set to ``<name>`` responds 201. The node then has a
    :link-kind:`bridge` interface named ``<name>``.

.. expectation:: A bridge network created without IPv4 gives its containers no IPv4 address
    :id: BR-OPT-IPV4-DISABLED
    :observed_from: api, ctr
    :depends_on: none
    :platforms: linux
    :refines: REQ-NET-IPV6-ONLY
    :needs: BR-HOST-FORWARDING-V6, BR-HOST-IP6TABLES-COMMAND, BR-HOST-IP6TABLES-KERNEL, BR-HOST-IP6TABLES-LEGACY-KERNEL, BR-HOST-IP6TABLES-NFT-KERNEL, BR-HOST-NFTABLES-V6

    *Applies to:* A Linux node that has IPv6 enabled

    At API version 1.48 or later, :api-op:`POST /networks/create` for a bridge network with
    :api-field:`EnableIPv4` false and :api-field:`EnableIPv6` true responds 201.
    :api-op:`GET /networks/{id}` then reports :api-field:`EnableIPv4` false. The interface of a
    container on the network has no IPv4 address. When IPv6 is enabled in the container's network
    namespace and on that interface, the interface has an IPv6 address in the network's subnet.

.. _bridge-networking-data-path:

***********
 Data path
***********

.. _bridge-networking-isolation:

Isolation
=========

The expectations in this subsection hold with :daemon-json:`userland-proxy` true and with
:daemon-json:`userland-proxy` false, unless an expectation names a value.

In this subsection, a container's addresses on a network are the IPv4 address and the IPv6 address
that :api-op:`GET /containers/{id}/json` reports for that network in
:api-field:`NetworkSettings.Networks`, as :api-field:`IPAddress` and :api-field:`GlobalIPv6Address`,
and the IPv6 link-local addresses of the container's interface on the network, the kernel's and
those in :api-field:`IPAMConfig.LinkLocalIPs`, reached with that interface as the zone. The IPv4
addresses in :api-field:`IPAMConfig.LinkLocalIPs` are not among them.

.. expectation:: Containers on separate networks are isolated
    :id: BR-ISO-SEPARATE-NETWORKS
    :observed_from: ctr → ctr
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-SEPARATE-NETWORKS
    :needs: BR-HOST-NO-FORWARD-DROP

    *Applies to:* A daemon with :daemon-json:`allow-direct-routing` false, and a target network
    whose :drvopt:`com.docker.network.bridge.gateway_mode_ipv4` is ``nat`` or not set, and whose
    :drvopt:`com.docker.network.bridge.trusted_host_interfaces` does not name the bridge of the
    other network

    A container on one bridge network cannot reach a container on another bridge network at that
    container's IPv4 address, on any port. It reaches the published ports of that container at the
    node's address.

.. expectation:: Containers on separate networks are isolated over IPv6
    :id: BR-ISO-SEPARATE-NETWORKS-V6
    :observed_from: ctr → ctr
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-SEPARATE-NETWORKS
    :needs: BR-HOST-FORWARDING-V6, BR-HOST-NO-FORWARD-DROP, BR-HOST-IP6TABLES-COMMAND, BR-HOST-IP6TABLES-KERNEL, BR-HOST-IP6TABLES-LEGACY-KERNEL, BR-HOST-IP6TABLES-NFT-KERNEL, BR-HOST-NFTABLES-V6

    *Applies to:* A daemon with :daemon-json:`ip6tables` true and
    :daemon-json:`allow-direct-routing` false, and two bridge networks with IPv6 enabled, the target
    network's :drvopt:`com.docker.network.bridge.gateway_mode_ipv6` being ``nat`` or not set, and
    its :drvopt:`com.docker.network.bridge.trusted_host_interfaces` not naming the other network's
    bridge

    A container on one of the networks cannot reach a container on the other at that container's
    IPv6 address, on any port. It reaches the published ports of that container at the node's IPv6
    address.

.. expectation:: Containers on other networks reach a routed network's published ports
    :id: BR-ISO-SEPARATE-NETWORKS-ROUTED
    :observed_from: ctr → ctr
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-SEPARATE-NETWORKS, REQ-BR-UNPUBLISHED-PORTS
    :needs: BR-HOST-NO-FORWARD-DROP

    *Applies to:* A target network whose :drvopt:`com.docker.network.bridge.gateway_mode_ipv4` is
    ``routed``, and a container on another bridge network that is not ``internal``

    The container reaches a container on the target network at that container's IPv4 address, on
    each port that the target container publishes. It does not reach the target container on any
    other port.

.. expectation:: Containers on other networks reach any port of a nat-unprotected network
    :id: BR-ISO-SEPARATE-NETWORKS-NAT-UNPROTECTED
    :observed_from: ctr → ctr
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-GWMODE-DIRECT-ROUTING
    :needs: BR-HOST-NO-FORWARD-DROP

    *Applies to:* A target network whose :drvopt:`com.docker.network.bridge.gateway_mode_ipv4` is
    ``nat-unprotected``, and a container on another bridge network that is not ``internal``

    The container reaches a container on the target network at that container's IPv4 address, on
    any port that the target container listens on.

.. expectation:: Containers on one bridge network reach each other on any port
    :id: BR-ISO-SAME-NETWORK
    :observed_from: ctr → ctr
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-SAME-NETWORK, REQ-BR-ICC, REQ-NET-DEFAULT-BRIDGE-ATTACHMENT, REQ-BR-UNPUBLISHED-PORTS, REQ-BR-INTERNAL
    :needs: BR-HOST-NO-FORWARD-DROP

    *Applies to:* A bridge network without :drvopt:`com.docker.network.bridge.enable_icc` set to
    ``false``, other than the default bridge of a daemon with :daemon-json:`icc` false

    A container on the network reaches another container on the network at each of that
    container's addresses on the network, on any port that it listens on.

.. expectation:: A network without ICC isolates its containers
    :id: BR-ISO-NO-ICC
    :observed_from: ctr → ctr
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-ICC
    :needs: BR-HOST-BR-NETFILTER-NO-ICC, BR-HOST-NO-FORWARD-DROP

    *Applies to:* A bridge network with :drvopt:`com.docker.network.bridge.enable_icc` set to
    ``false``

    A container cannot reach another container on the network at any of that container's addresses
    on the network, on any port. This does not apply to two containers that a legacy link joins. A
    container reaches the published ports of another container on the network at the node's address.

.. expectation:: Without ICC, a legacy link reaches only exposed ports over IPv4
    :id: BR-ISO-NO-ICC-LEGACY-LINK
    :observed_from: ctr → ctr
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-LEGACY-LINKS, REQ-BR-ICC
    :needs: BR-HOST-BR-NETFILTER-NO-ICC, BR-HOST-NO-FORWARD-DROP

    *Applies to:* A daemon with :daemon-json:`icc` false, and a container on the default bridge
    network whose :api-field:`HostConfig.Links` names another container on that network

    The container reaches the linked container at the linked container's IPv4 address, on each port,
    with its protocol, that the linked container's :api-field:`Config.ExposedPorts` or
    :api-field:`HostConfig.PortBindings` lists. :api-field:`Config.ExposedPorts` includes the ports
    that the container's image exposes. It does not reach the linked container on any other port.

.. expectation:: With the daemon's icc false, the default bridge isolates its containers
    :id: BR-ISO-DAEMON-NO-ICC
    :observed_from: ctr → ctr
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-ICC
    :needs: BR-HOST-BR-NETFILTER-NO-ICC, BR-HOST-NO-FORWARD-DROP

    *Applies to:* A daemon with :daemon-json:`icc` false, and two containers on the default bridge
    network that no legacy link joins

    Neither container reaches the other at that container's addresses on the network, on any port.

.. expectation:: The node does not forward traffic for an internal network
    :id: BR-ISO-INTERNAL-NO-FORWARD
    :observed_from: ctr, underlay
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-INTERNAL, REQ-NET-INTERNAL-NO-EXTERNAL-ACCESS

    The node does not forward traffic between a container on an ``internal`` bridge network and an
    address outside the network's subnets, in either direction. In the inbound direction, this
    includes traffic from a host outside the node that has a route to the network's subnets through
    an address of the node. This holds even when the container has a default route through the
    bridge, such as one that a container with :cap:`NET_ADMIN` adds itself. Traffic that the node
    delivers to its own addresses is not forwarded, and this expectation does not cover it.

.. expectation:: The node reaches containers on an internal network
    :id: BR-ISO-INTERNAL-HOST-TO-CTR
    :observed_from: node
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-INTERNAL, REQ-BR-ISOLATED

    *Applies to:* An ``internal`` bridge network whose
    :drvopt:`com.docker.network.bridge.gateway_mode_ipv4` is not ``isolated``, and without
    :drvopt:`com.docker.network.bridge.inhibit_ipv4`

    A process in the node's host network namespace reaches a container on the network at the
    container's IPv4 address.

.. expectation:: Containers on an internal network reach the bridge's address
    :id: BR-ISO-INTERNAL-CTR-TO-HOST
    :observed_from: ctr
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-INTERNAL, REQ-BR-ISOLATED

    *Applies to:* An ``internal`` bridge network whose
    :drvopt:`com.docker.network.bridge.gateway_mode_ipv4` is not ``isolated``, and without
    :drvopt:`com.docker.network.bridge.inhibit_ipv4`

    A container on the network reaches a process in the node's host network namespace that listens
    on the bridge's IPv4 address.

.. expectation:: A container's own route reaches other addresses of the node
    :id: BR-ISO-INTERNAL-CTR-ROUTE
    :observed_from: ctr
    :depends_on: os-linux
    :platforms: linux
    :rationale: RAT-BR-INTERNAL-NODE-ADDRESSES

    *Applies to:* An ``internal`` bridge network whose
    :drvopt:`com.docker.network.bridge.gateway_mode_ipv4` is not ``isolated``, without
    :drvopt:`com.docker.network.bridge.inhibit_ipv4`, and a container on the network with
    :cap:`NET_ADMIN` that has added a route via the bridge's IPv4 address to an address of the node
    outside the network's subnets

    The container reaches a process in the node's host network namespace that listens on that
    address.

.. expectation:: An isolated network's bridge has no IPv4 address
    :id: BR-ISO-ISOLATED-NO-GATEWAY
    :observed_from: node
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-ISOLATED

    *Applies to:* An ``internal`` bridge network with
    :drvopt:`com.docker.network.bridge.gateway_mode_ipv4` set to ``isolated``

    The bridge does not have an IPv4 address. A process in the node's host network namespace cannot
    reach the network's containers at their IPv4 addresses.

.. expectation:: Containers on an isolated network cannot reach the node
    :id: BR-ISO-ISOLATED-CTR-TO-HOST
    :observed_from: ctr
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-ISOLATED

    *Applies to:* An ``internal`` bridge network with
    :drvopt:`com.docker.network.bridge.gateway_mode_ipv4` set to ``isolated``

    A container on the network cannot reach a process in the node's host network namespace at any
    IPv4 address of the node. This holds even when the container has a route to that address that a
    container with :cap:`NET_ADMIN` adds itself.

.. expectation:: Direct routing from an untrusted interface does not reach containers
    :id: BR-ISO-DIRECT-ROUTING-OFF
    :observed_from: underlay → ctr
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-NO-DIRECT-ROUTING

    *Applies to:* A daemon with :daemon-json:`allow-direct-routing` false, a container on a bridge
    network whose :drvopt:`com.docker.network.bridge.gateway_mode_ipv4` is ``nat`` or not set, and
    an underlay host with a route to the container's IPv4 address via the node's underlay address,
    whose traffic arrives on an interface that is not in the network's
    :drvopt:`com.docker.network.bridge.trusted_host_interfaces`

    Traffic from that host to the container's IPv4 address does not reach the container on any port.

.. expectation:: Direct routing over IPv6 from an untrusted interface does not reach containers
    :id: BR-ISO-DIRECT-ROUTING-OFF-V6
    :observed_from: underlay → ctr
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-NO-DIRECT-ROUTING
    :needs: BR-HOST-FORWARDING-V6, BR-HOST-IP6TABLES-COMMAND, BR-HOST-IP6TABLES-KERNEL, BR-HOST-IP6TABLES-LEGACY-KERNEL, BR-HOST-IP6TABLES-NFT-KERNEL, BR-HOST-NFTABLES-V6

    *Applies to:* A daemon with :daemon-json:`ip6tables` true and
    :daemon-json:`allow-direct-routing` false, a container on a bridge network with IPv6 enabled
    whose :drvopt:`com.docker.network.bridge.gateway_mode_ipv6` is ``nat`` or not set, and an
    underlay host with a route to the container's IPv6 address via the node's underlay address,
    whose traffic arrives on an interface that is not in the network's
    :drvopt:`com.docker.network.bridge.trusted_host_interfaces`

    Traffic from that host to the container's IPv6 address does not reach the container on any port.

.. expectation:: After a firewalld reload, direct routing still does not reach containers
    :id: BR-ISO-DIRECT-ROUTING-OFF-FIREWALLD
    :observed_from: underlay → ctr
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-NO-DIRECT-ROUTING, REQ-BR-FIREWALLD-RELOAD

    *Applies to:* A node with firewalld running, a daemon with :daemon-json:`allow-direct-routing`
    false, a container on a bridge network whose
    :drvopt:`com.docker.network.bridge.gateway_mode_ipv4` is ``nat`` or not set, and an underlay
    host with a route to the container's IPv4 address via the node's underlay address, whose traffic
    arrives on an interface that is not in the network's
    :drvopt:`com.docker.network.bridge.trusted_host_interfaces`

    After a firewalld reload, traffic from that host to the container's IPv4 address does not reach
    the container on any port, including a port that the container publishes on ``127.0.0.1``.

.. expectation:: With allow-direct-routing, direct routing reaches published ports only
    :id: BR-ISO-DIRECT-ROUTING-ON
    :observed_from: underlay → ctr
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-DIRECT-ROUTING-OPT-IN, REQ-BR-UNPUBLISHED-PORTS
    :needs: BR-HOST-NO-FORWARD-DROP

    *Applies to:* A daemon with :daemon-json:`allow-direct-routing` true, a container on a bridge
    network whose :drvopt:`com.docker.network.bridge.gateway_mode_ipv4` is ``nat`` or not set, and
    an underlay host with a route to the container's IPv4 address via the node's underlay address

    Traffic from that host to the container's IPv4 address reaches the container's published ports
    only. A port counts as published whatever the host address of its binding, a loopback address
    included.

.. expectation:: After a firewalld reload, direct routing still reaches published ports only
    :id: BR-ISO-DIRECT-ROUTING-ON-FIREWALLD
    :observed_from: underlay → ctr
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-DIRECT-ROUTING-OPT-IN, REQ-BR-UNPUBLISHED-PORTS, REQ-BR-FIREWALLD-RELOAD
    :needs: BR-HOST-NO-FORWARD-DROP

    *Applies to:* A node with firewalld running, a daemon with :daemon-json:`allow-direct-routing`
    true, a container on a bridge network whose
    :drvopt:`com.docker.network.bridge.gateway_mode_ipv4` is ``nat`` or not set, and an underlay
    host with a route to the container's IPv4 address via the node's underlay address

    After a firewalld reload, traffic from that host to the container's IPv4 address reaches the
    container's published ports only. A port counts as published whatever the host address of its
    binding, a loopback address included.

.. expectation:: A trusted interface reaches published ports only
    :id: BR-ISO-TRUSTED-IFACE
    :observed_from: underlay → ctr
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-DIRECT-ROUTING-OPT-IN, REQ-BR-UNPUBLISHED-PORTS
    :needs: BR-HOST-NO-FORWARD-DROP

    *Applies to:* A container on a bridge network whose
    :drvopt:`com.docker.network.bridge.gateway_mode_ipv4` is ``nat`` or not set, and an underlay
    host with a route to the container's IPv4 address via the node's underlay address

    When traffic from that host to the container's IPv4 address arrives on an interface in the
    network's :drvopt:`com.docker.network.bridge.trusted_host_interfaces`, it reaches the
    container's published ports only. A port counts as published whatever the host address of its
    binding, a loopback address included.

.. expectation:: A colon-separated list names several trusted interfaces
    :id: BR-ISO-TRUSTED-IFACE-LIST
    :observed_from: underlay → ctr
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-DIRECT-ROUTING-OPT-IN
    :needs: BR-HOST-NO-FORWARD-DROP

    *Applies to:* A bridge network whose :drvopt:`com.docker.network.bridge.trusted_host_interfaces`
    names several interfaces separated by colons, such as ``eth1:eth2``, and whose
    :drvopt:`com.docker.network.bridge.gateway_mode_ipv4` is ``nat`` or not set, and an underlay
    host with a route to the IPv4 address of a container on the network via the node's underlay
    address

    When traffic from that host to the container's IPv4 address arrives on any of the named
    interfaces, it reaches the container's published ports. A port counts as published whatever the
    host address of its binding, a loopback address included.

.. expectation:: With iptables, direct routing does not reach a port published only on IPv6 host addresses
    :id: BR-ISO-DIRECT-ROUTING-V6-HOST-BINDING
    :observed_from: underlay → ctr
    :depends_on: os-linux
    :platforms: linux
    :narrows: BR-ISO-DIRECT-ROUTING-ON, BR-ISO-DIRECT-ROUTING-ON-FIREWALLD, BR-ISO-TRUSTED-IFACE, BR-ISO-TRUSTED-IFACE-LIST
    :violates: REQ-BR-DIRECT-ROUTING-OPT-IN

    *Applies to:* The iptables backend; a container on a bridge network whose
    :drvopt:`com.docker.network.bridge.gateway_mode_ipv4` is ``nat`` or not set, with a port that it
    publishes only on IPv6 host addresses; and an underlay host with a route to the container's IPv4
    address via the node's underlay address, whose traffic arrives on an interface in the network's
    :drvopt:`com.docker.network.bridge.trusted_host_interfaces`, or on any interface with
    :daemon-json:`allow-direct-routing` true

    Traffic from that host to that port at the container's IPv4 address does not reach the
    container.

.. expectation:: Traffic with the bridge-accept-fwmark mark reaches unpublished ports
    :id: BR-ISO-ACCEPT-FWMARK
    :observed_from: underlay → ctr
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-ACCEPT-FWMARK
    :needs: BR-HOST-NO-FORWARD-DROP

    *Applies to:* A daemon with :daemon-json:`bridge-accept-fwmark` set to a mark, or to a mark and
    a mask; a container on a bridge network that is not ``internal`` and whose
    :drvopt:`com.docker.network.bridge.gateway_mode_ipv4` is ``nat`` or not set; an underlay host
    with a route to the container's IPv4 address via the node's underlay address, whose traffic
    arrives on an interface in the network's
    :drvopt:`com.docker.network.bridge.trusted_host_interfaces`; and an operator's rule at the
    prerouting hook that sets that mark on this traffic

    Traffic from that host to the container's IPv4 address reaches the container on every port,
    published or not.

.. expectation:: The bridge-accept-fwmark mark does not open direct routing from an untrusted interface
    :id: BR-ISO-ACCEPT-FWMARK-UNTRUSTED
    :observed_from: underlay → ctr
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-ACCEPT-FWMARK, REQ-BR-NO-DIRECT-ROUTING

    *Applies to:* A daemon with :daemon-json:`bridge-accept-fwmark` set to a mark, or to a mark and
    a mask, and with :daemon-json:`allow-direct-routing` false; a container on a bridge network that
    is not ``internal`` and whose :drvopt:`com.docker.network.bridge.gateway_mode_ipv4` is ``nat``
    or not set; an underlay host with a route to the container's IPv4 address via the node's
    underlay address, whose traffic arrives on an interface that is not in the network's
    :drvopt:`com.docker.network.bridge.trusted_host_interfaces`; and an operator's rule that sets
    that mark on this traffic at the prerouting hook, before the daemon's own rules on that hook

    Traffic from that host to the container's IPv4 address does not reach the container on any port.

.. expectation:: The daemon enables bridge-nf-call-iptables
    :id: BR-ISO-BRIDGE-NF-CALL
    :observed_from: node
    :depends_on: none
    :platforms: linux
    :pins: BR-HOST-BR-NETFILTER-NO-ICC, BR-HOST-BR-NETFILTER-NO-PROXY, BR-HOST-PROC-SYS-NET-WRITABLE, SWARM-HOST-BR-NETFILTER
    :needs: BR-HOST-BR-NETFILTER-NO-ICC, BR-HOST-BR-NETFILTER-NO-PROXY, BR-HOST-PROC-SYS-NET-WRITABLE, SWARM-HOST-BR-NETFILTER

    *Applies to:* A daemon with :daemon-json:`iptables` true, on a host where
    :sysctl:`net.ipv4.ip_forward` is 1, the module ``br_netfilter`` is loaded and
    :sysctl:`net.bridge.bridge-nf-call-iptables` is 0

    Once the daemon has set up a bridge network with :drvopt:`com.docker.network.bridge.enable_icc`
    set to ``false``, or any bridge network when :daemon-json:`userland-proxy` is false, the sysctl
    :sysctl:`net.bridge.bridge-nf-call-iptables` is 1: once the request that creates the network
    responds, or, for a network that exists when the daemon starts, once the daemon's API responds.

.. expectation:: The daemon enables bridge-nf-call-ip6tables
    :id: BR-ISO-BRIDGE-NF-CALL-V6
    :observed_from: node
    :depends_on: none
    :platforms: linux
    :pins: BR-HOST-BR-NETFILTER-NO-ICC, BR-HOST-BR-NETFILTER-NO-PROXY, BR-HOST-PROC-SYS-NET-WRITABLE
    :needs: BR-HOST-BR-NETFILTER-NO-ICC, BR-HOST-BR-NETFILTER-NO-PROXY, BR-HOST-PROC-SYS-NET-WRITABLE, BR-HOST-FORWARDING-V6, BR-HOST-IP6TABLES-COMMAND, BR-HOST-IP6TABLES-KERNEL, BR-HOST-IP6TABLES-LEGACY-KERNEL, BR-HOST-IP6TABLES-NFT-KERNEL, BR-HOST-NFTABLES-V6

    *Applies to:* A daemon with :daemon-json:`ip6tables` true, on a host where the module
    ``br_netfilter`` is loaded and :sysctl:`net.bridge.bridge-nf-call-ip6tables` is 0

    Once the daemon has set up a bridge network with IPv6 enabled and
    :drvopt:`com.docker.network.bridge.enable_icc` set to ``false``, or any bridge network with IPv6
    enabled when :daemon-json:`userland-proxy` is false, the sysctl
    :sysctl:`net.bridge.bridge-nf-call-ip6tables` is 1: once the request that creates the network
    responds, or, for a network that exists when the daemon starts, once the daemon's API responds.

.. expectation:: Without br_netfilter, a network without ICC is refused
    :id: BR-ISO-NO-BR-NETFILTER-NO-ICC
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :refines: PRIN-FAILS-CLOSED, REQ-NET-NETWORK-CREATE-ERRORS
    :pins: BR-HOST-BR-NETFILTER-NO-ICC, BR-HOST-BR-NETFILTER-NO-PROXY

    *Applies to:* A Linux host whose kernel does not offer Netfilter hooks for bridged traffic
    (``br_netfilter``), and a daemon with :daemon-json:`iptables` true

    :api-op:`POST /networks/create` for a bridge network with
    :drvopt:`com.docker.network.bridge.enable_icc` set to ``false`` responds with a 4xx or 5xx
    status. :api-op:`GET /networks` then does not list a network with the request's
    :api-field:`Name`.

.. expectation:: Without br_netfilter, the refusal says ICC cannot be restricted
    :id: BR-ISO-NO-BR-NETFILTER-NO-ICC-MSG
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :exact_form_of: BR-ISO-NO-BR-NETFILTER-NO-ICC
    :pins: BR-HOST-BR-NETFILTER-NO-ICC, BR-HOST-BR-NETFILTER-NO-PROXY

    *Applies to:* A Linux host whose kernel does not offer Netfilter hooks for bridged traffic
    (``br_netfilter``), and a daemon with :daemon-json:`iptables` true

    The error response of :api-op:`POST /networks/create` for a bridge network with
    :drvopt:`com.docker.network.bridge.enable_icc` set to ``false`` has a message that contains
    :api-error:`cannot restrict inter-container communication`.

.. expectation:: Without br_netfilter, a daemon with icc false does not start
    :id: BR-ISO-NO-BR-NETFILTER-DAEMON-ICC
    :observed_from: log, node
    :depends_on: none
    :platforms: linux
    :refines: PRIN-FAILS-CLOSED
    :pins: BR-HOST-BR-NETFILTER-NO-ICC, BR-HOST-BR-NETFILTER-NO-PROXY

    *Applies to:* A Linux host whose kernel does not offer Netfilter hooks for bridged traffic
    (``br_netfilter``), a daemon with :daemon-json:`iptables` true, :daemon-json:`icc` false and the
    default bridge enabled, and without containers that :daemon-json:`live-restore` keeps running

    The daemon does not start. The daemon log contains an error.

.. expectation:: A container does not start without veth support
    :id: BR-ISO-NO-VETH
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :refines: PRIN-FAILS-CLOSED
    :pins: BR-HOST-NETNS-VETH-BRIDGE

    *Applies to:* A Linux host whose kernel does not offer veth pairs, and a created or stopped
    container on a bridge network

    :api-op:`POST /containers/{id}/start` for the container responds with a 4xx or 5xx status. The
    container is not running.

.. expectation:: Without veth support, a container start responds 500
    :id: BR-ISO-NO-VETH-HTTP500
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :exact_form_of: BR-ISO-NO-VETH
    :pins: BR-HOST-NETNS-VETH-BRIDGE

    *Applies to:* A Linux host whose kernel does not offer veth pairs, and a created or stopped
    container on a bridge network

    :api-op:`POST /containers/{id}/start` for the container responds 500, with a message that
    contains :api-error:`pair interfaces`.

.. expectation:: Without veth support, a container on a bridge network is created without a warning
    :id: BR-ISO-NO-VETH-CREATE
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :pins: BR-HOST-NETNS-VETH-BRIDGE
    :concerns: BR-ISO-NO-VETH
    :violates: PRIN-WARN-AHEAD

    *Applies to:* A Linux host whose kernel does not offer veth pairs

    :api-op:`POST /containers/create` for a container on a bridge network responds 201 without a
    warning about the network.

.. expectation:: Without veth support, a stopped container is connected to a bridge network without a warning
    :id: BR-ISO-NO-VETH-CONNECT
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :pins: BR-HOST-NETNS-VETH-BRIDGE
    :concerns: BR-ISO-NO-VETH
    :violates: PRIN-WARN-AHEAD

    *Applies to:* A Linux host whose kernel does not offer veth pairs, and a created or stopped
    container

    :api-op:`POST /networks/{id}/connect` for the container to a bridge network responds 200 without
    a warning about the network.

.. _bridge-networking-port-publishing:

Port publishing
===============

The expectations in this subsection hold with :daemon-json:`userland-proxy` true and with
:daemon-json:`userland-proxy` false, unless an expectation names a value.

.. expectation:: A port published without a host port gets one that clients reach
    :id: BR-PUB-HOST-PORT-ASSIGNED
    :observed_from: api, client
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-PUBLISH-DEFAULTS, REQ-BR-PUBLISH-SKIP-RESERVED-PORTS
    :needs: BR-HOST-NO-FORWARD-DROP

    :api-op:`POST /containers/{id}/start` for a created or stopped container that is on a bridge
    network and publishes a port without a host port responds 204.
    :api-op:`GET /containers/{id}/json` then reports the assigned host port in
    :api-field:`NetworkSettings.Ports`. On a host where neither
    :sysctl:`net.ipv4.ip_local_port_range` nor :sysctl:`net.ipv4.ip_local_reserved_ports` has
    changed since the daemon started, the port is in that range and is not one of those reserved
    ports. This includes a host whose :sysctl:`net.ipv4.ip_local_reserved_ports` lists every port of
    that range but one that nothing on the node uses: the assigned port is that one. A client
    reaches the container on that host port at the node's address.

.. expectation:: docker port prints the host port assigned to a published port
    :id: BR-PUB-HOST-PORT-ASSIGNED-CLI
    :observed_from: api, client
    :depends_on: cli, os-linux
    :platforms: linux
    :refines: REQ-BR-PUBLISH-DEFAULTS
    :cli_for: BR-PUB-HOST-PORT-ASSIGNED
    :needs: BR-HOST-NO-FORWARD-DROP

    *Applies to:* A running container on a bridge network that publishes a port without a host port

    :cli:`docker port` for the container exits 0 and prints the host port that the daemon assigned
    to that port. A client reaches the container on that host port at the node's address.

.. expectation:: A requested host port or range is assigned even when it is reserved
    :id: BR-PUB-HOST-PORT-RESERVED-REQUESTED
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :refines: REQ-BR-PUBLISH-SKIP-RESERVED-PORTS

    *Applies to:* A host whose :sysctl:`net.ipv4.ip_local_reserved_ports` lists a port, or every
    port of a port range, that nothing on the node uses

    :api-op:`POST /containers/{id}/start` for a created container on a bridge network that publishes
    a port with that host port, or with that host port range, responds 204. :api-op:`GET
    /containers/{id}/json` then reports in :api-field:`NetworkSettings.Ports` a host port that
    :sysctl:`net.ipv4.ip_local_reserved_ports` lists.

.. expectation:: A port published with protocol sctp answers
    :id: BR-PUB-SCTP
    :observed_from: client
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-PUBLISH-SCTP
    :needs: BR-HOST-SCTP, BR-HOST-IPTABLES-SCTP-MATCH, BR-HOST-NO-FORWARD-DROP

    *Applies to:* A running container on a bridge network that publishes an SCTP port without a host
    address

    An SCTP association from a client to that port at the node's IPv4 address reaches the container.

.. expectation:: A container that publishes an SCTP port does not start without SCTP
    :id: BR-PUB-NO-SCTP
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :refines: PRIN-FAILS-CLOSED
    :pins: BR-HOST-SCTP, SWARM-HOST-SCTP

    *Applies to:* A Linux host whose kernel does not offer SCTP, and a created or stopped container
    that publishes an SCTP port without a host address, and whose networks are all bridge networks
    that are not ``internal``, that have IPv4, whose
    :drvopt:`com.docker.network.bridge.gateway_mode_ipv4` and
    :drvopt:`com.docker.network.bridge.gateway_mode_ipv6` are not ``routed``, and whose
    :drvopt:`com.docker.network.bridge.host_binding_ipv4` is unset or an IPv4 address

    :api-op:`POST /containers/{id}/start` for the container responds with a 4xx or 5xx status. The
    container is not running.

.. expectation:: Without SCTP, the start error says it cannot create an SCTP socket
    :id: BR-PUB-NO-SCTP-MSG
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :exact_form_of: BR-PUB-NO-SCTP
    :pins: BR-HOST-SCTP, SWARM-HOST-SCTP

    *Applies to:* A Linux host whose kernel does not offer SCTP, and a created or stopped container
    that publishes an SCTP port without a host address, and whose networks are all bridge networks
    that are not ``internal``, that have IPv4, whose
    :drvopt:`com.docker.network.bridge.gateway_mode_ipv4` and
    :drvopt:`com.docker.network.bridge.gateway_mode_ipv6` are not ``routed``, and whose
    :drvopt:`com.docker.network.bridge.host_binding_ipv4` is unset or an IPv4 address

    :api-op:`POST /containers/{id}/start` for the container responds with a 4xx or 5xx status, with
    a message that contains :api-error:`failed to create socket for` and ``/sctp``.

.. expectation:: Without SCTP, a container that publishes an SCTP port is created without a warning
    :id: BR-PUB-NO-SCTP-CREATE
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :pins: BR-HOST-SCTP
    :concerns: BR-PUB-NO-SCTP
    :violates: PRIN-WARN-AHEAD

    *Applies to:* A Linux host whose kernel does not offer SCTP

    :api-op:`POST /containers/create` for a container that publishes an SCTP port without a host
    address, and whose networks are all bridge networks that are not ``internal``, that have IPv4,
    whose :drvopt:`com.docker.network.bridge.gateway_mode_ipv4` and
    :drvopt:`com.docker.network.bridge.gateway_mode_ipv6` are not ``routed``, and whose
    :drvopt:`com.docker.network.bridge.host_binding_ipv4` is unset or an IPv4 address, responds 201
    without a warning about the port.

.. expectation:: Without SCTP, a connect of a stopped container gives no SCTP port warning
    :id: BR-PUB-NO-SCTP-CONNECT
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :pins: BR-HOST-SCTP
    :concerns: BR-PUB-NO-SCTP
    :violates: PRIN-WARN-AHEAD

    *Applies to:* A Linux host whose kernel does not offer SCTP, and a created or stopped container
    that publishes an SCTP port without a host address, and whose networks are all bridge networks
    that are not ``internal``, that have IPv4, whose
    :drvopt:`com.docker.network.bridge.gateway_mode_ipv4` and
    :drvopt:`com.docker.network.bridge.gateway_mode_ipv6` are not ``routed``, and whose
    :drvopt:`com.docker.network.bridge.host_binding_ipv4` is unset or an IPv4 address

    :api-op:`POST /networks/{id}/connect` for the container to another bridge network of that kind
    responds 200 without a warning about the port.

.. expectation:: Without SCTP, repeated starts of one container sometimes fail
    :id: BR-PUB-NO-SCTP-JOIN-ORDER
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :pins: BR-HOST-SCTP
    :needs: BR-HOST-IPTABLES-SCTP-MATCH, BR-HOST-FORWARDING-V6, BR-HOST-IP6TABLES-COMMAND, BR-HOST-IP6TABLES-KERNEL, BR-HOST-IP6TABLES-LEGACY-KERNEL, BR-HOST-IP6TABLES-NFT-KERNEL, BR-HOST-NFTABLES-V6
    :concerns: BR-PUB-NO-SCTP
    :violates: PRIN-REPEATABLE

    *Applies to:* A Linux host whose kernel does not offer SCTP, and a created or stopped container
    with IPv6 enabled in its network namespace and on its interfaces, that publishes an SCTP port
    without a host address, and whose only networks are two bridge networks that are not
    ``internal``, with the same :api-field:`GwPriority`: one with IPv4 and without IPv6, whose
    :drvopt:`com.docker.network.bridge.gateway_mode_ipv4` is not ``routed`` and whose
    :drvopt:`com.docker.network.bridge.host_binding_ipv4` is unset or an IPv4 address, and one with
    IPv4 and IPv6, whose :drvopt:`com.docker.network.bridge.gateway_mode_ipv4` and
    :drvopt:`com.docker.network.bridge.gateway_mode_ipv6` are ``routed``

    Repeated :api-op:`POST /containers/{id}/start` for the container, each after the container
    stops, give both outcomes: some respond 204, and some respond with a 4xx or 5xx status, with a
    message that contains :api-error:`failed to create socket for` and ``/sctp``. This holds
    whatever the names of the container and of its networks.

.. expectation:: A container's unpublished port is not reachable at the node's address
    :id: BR-PUB-UNPUBLISHED-PORT
    :observed_from: client
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-UNPUBLISHED-PORTS

    *Applies to:* A container that is on a bridge network and listens on a port that it does not
    publish

    A client that connects to the node's address, on any port, does not reach that listener.

.. expectation:: The node reaches a container's unpublished port at its address
    :id: BR-PUB-UNPUBLISHED-PORT-NODE
    :observed_from: node
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-UNPUBLISHED-PORTS

    *Applies to:* A container on a bridge network that is not ``internal`` and has no
    :drvopt:`com.docker.network.bridge.inhibit_ipv4`, and that listens on a port that it does not
    publish

    A process in the node's host network namespace reaches that listener at the container's IPv4
    address.

.. expectation:: A port published on 127.0.0.1 is reachable from the node
    :id: BR-PUB-LOOPBACK-NODE
    :observed_from: node
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-PUBLISH-HOST-IP

    A port published on ``127.0.0.1`` is reachable from the node at ``127.0.0.1``.

.. expectation:: A port published on 127.0.0.1 is not reachable from a client
    :id: BR-PUB-LOOPBACK-CLIENT
    :observed_from: client
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-PUBLISH-HOST-IP

    A port published on ``127.0.0.1`` is not reachable from a client at any address of the node.
    This includes a packet to ``127.0.0.1`` that a client on the node's link sends in an Ethernet
    frame addressed to the node's MAC address.

.. expectation:: After a firewalld reload, a port published on 127.0.0.1 stays unreachable
    :id: BR-PUB-LOOPBACK-CLIENT-FIREWALLD
    :observed_from: client
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-PUBLISH-HOST-IP, REQ-BR-FIREWALLD-RELOAD

    *Applies to:* A node with firewalld running, and a container that publishes a port on
    ``127.0.0.1``

    After a firewalld reload, the port is not reachable from a client at any address of the node.
    This includes a packet to ``127.0.0.1`` that a client on the node's link sends in an Ethernet
    frame addressed to the node's MAC address.

.. expectation:: A port published on a node address is reachable at that address only
    :id: BR-PUB-HOST-ADDRESS
    :observed_from: node, client
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-PUBLISH-HOST-IP
    :needs: BR-HOST-NO-FORWARD-DROP

    A port published on an address of the node is reachable at that address, from the node and from
    a client. It is not reachable at the node's other addresses, ``127.0.0.1`` included.

.. expectation:: With host_binding_ipv4, a port is reachable at that address only
    :id: BR-PUB-HOST-BINDING-ADDRESS
    :observed_from: node, client
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-HOST-BINDING-ADDRESS
    :needs: BR-HOST-NO-FORWARD-DROP

    *Applies to:* A bridge network with :drvopt:`com.docker.network.bridge.host_binding_ipv4` set to
    an IPv4 address of the node, and a container on the network that publishes a port without a
    host address

    The port is reachable at that address, from the node and from a client. It is not reachable at
    the node's other addresses, ``127.0.0.1`` included.

.. expectation:: With the daemon option ip, a default-bridge port is reachable there only
    :id: BR-PUB-HOST-BINDING-ADDRESS-DAEMON
    :observed_from: node, client
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-HOST-BINDING-ADDRESS
    :needs: BR-HOST-NO-FORWARD-DROP

    *Applies to:* A daemon with :daemon-json:`ip` set to an IPv4 address of the node, and a
    container on the default bridge network that publishes a port without a host address

    The port is reachable at that address, from the node and from a client. It is not reachable at
    the node's other addresses, ``127.0.0.1`` included.

.. expectation:: A port published with no host address answers at node IPv6 addresses
    :id: BR-PUB-V6-REACHABLE
    :observed_from: client
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-PUBLISH-DEFAULTS
    :needs: BR-HOST-FORWARDING-V6, BR-HOST-NO-FORWARD-DROP, BR-HOST-IP6TABLES-COMMAND, BR-HOST-IP6TABLES-KERNEL, BR-HOST-IP6TABLES-LEGACY-KERNEL, BR-HOST-IP6TABLES-NFT-KERNEL, BR-HOST-NFTABLES-V6

    *Applies to:* A bridge network with IPv6 enabled, and a daemon with :daemon-json:`ip6tables`
    true

    A port published without a host address is reachable at the node's IPv6 addresses.

.. expectation:: Without the userland proxy, a port published on all addresses does not answer at link-local
    :id: BR-PUB-V6-LINK-LOCAL
    :observed_from: client
    :depends_on: os-linux
    :platforms: linux
    :needs: BR-HOST-FORWARDING-V6, BR-HOST-IP6TABLES-COMMAND, BR-HOST-IP6TABLES-KERNEL, BR-HOST-IP6TABLES-LEGACY-KERNEL, BR-HOST-IP6TABLES-NFT-KERNEL, BR-HOST-NFTABLES-V6
    :concerns: BR-PUB-V6-REACHABLE
    :violates: REQ-BR-PUBLISH-DEFAULTS

    *Applies to:* A bridge network with IPv6 enabled, and a daemon with :daemon-json:`ip6tables`
    true and :daemon-json:`userland-proxy` false

    A port published without a host address is not reachable from a client at an IPv6 link-local
    address of the node.

.. expectation:: Over IPv4, the container sees the client's address as the source
    :id: BR-PUB-V4-CLIENT-SOURCE
    :observed_from: client → ctr
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-PUBLISH-V4-CLIENT-SOURCE
    :needs: BR-HOST-NO-FORWARD-DROP

    A client connects to a port published without a host address, at one of the node's IPv4
    addresses. The container observes the IPv4 address of the client as the source address.

.. expectation:: Over IPv6, the container sees the client's address as the source
    :id: BR-PUB-V6-CLIENT-SOURCE
    :observed_from: client → ctr
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-PUBLISH-V6-CLIENT-SOURCE
    :needs: BR-HOST-FORWARDING-V6, BR-HOST-NO-FORWARD-DROP, BR-HOST-IP6TABLES-COMMAND, BR-HOST-IP6TABLES-KERNEL, BR-HOST-IP6TABLES-LEGACY-KERNEL, BR-HOST-IP6TABLES-NFT-KERNEL, BR-HOST-NFTABLES-V6

    *Applies to:* A bridge network with IPv6 enabled, and a daemon with :daemon-json:`ip6tables`
    true

    A client connects to a port published without a host address, at one of the node's IPv6
    addresses. The container observes the IPv6 address of the client as the source address.

.. expectation:: Without the userland proxy, the bridge's route_localnet is 1
    :id: BR-PUB-ROUTE-LOCALNET
    :observed_from: node
    :depends_on: none
    :platforms: linux
    :pins: BR-HOST-PROC-SYS-NET-WRITABLE
    :needs: BR-HOST-PROC-SYS-NET-WRITABLE

    *Applies to:* A daemon with :daemon-json:`userland-proxy` false, on a host where
    :sysctl:`net.ipv4.conf.default.route_localnet` is 0, and a bridge network whose bridge
    interface, where it exists before the daemon sets up the network, has
    :sysctl:`net.ipv4.conf.<bridge>.route_localnet` 0, such as a default bridge device left by a
    daemon with :daemon-json:`userland-proxy` true

    Once the daemon has set up the network, the sysctl
    :sysctl:`net.ipv4.conf.<bridge>.route_localnet` is 1, where ``<bridge>`` is the network's bridge
    interface: once the request that creates the network responds, or, for a network that exists
    when the daemon starts, once the daemon's API responds.

.. _bridge-networking-egress:

Egress
======

.. expectation:: A container on a bridge network reaches an underlay host
    :id: BR-EGRESS-UNDERLAY-REACH
    :observed_from: ctr
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-EGRESS-NAT, REQ-NET-DEFAULT-BRIDGE-ATTACHMENT
    :needs: BR-HOST-NO-FORWARD-DROP

    *Applies to:* A running container on a bridge network that is not ``internal``, whose
    :drvopt:`com.docker.network.bridge.gateway_mode_ipv4` is not ``routed`` and whose
    :drvopt:`com.docker.network.bridge.enable_ip_masquerade` is not ``false``. The default bridge is
    such a network.

    A connection from the container to an IPv4 address of an underlay host succeeds.

.. expectation:: IPv4 egress from containers has the node's address as its source
    :id: BR-EGRESS-MASQUERADE
    :observed_from: underlay
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-EGRESS-NAT, REQ-BR-GWMODE-VALUES
    :needs: BR-HOST-NO-FORWARD-DROP

    *Applies to:* A bridge network whose :drvopt:`com.docker.network.bridge.enable_ip_masquerade` is
    not ``false`` and whose :drvopt:`com.docker.network.bridge.gateway_mode_ipv4` is ``nat`` or not
    set, without :drvopt:`com.docker.network.host_ipv4`. The default bridge is such a network.

    An underlay host observes an address of the node as the source address of IPv4 egress from the
    network's containers.

.. expectation:: IPv6 egress from containers has the node's address as its source
    :id: BR-EGRESS-MASQUERADE-V6
    :observed_from: underlay
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-EGRESS-NAT, REQ-BR-GWMODE-VALUES
    :needs: BR-HOST-FORWARDING-V6, BR-HOST-NO-FORWARD-DROP, BR-HOST-IP6TABLES-COMMAND, BR-HOST-IP6TABLES-KERNEL, BR-HOST-IP6TABLES-LEGACY-KERNEL, BR-HOST-IP6TABLES-NFT-KERNEL, BR-HOST-NFTABLES-V6

    *Applies to:* A daemon with :daemon-json:`ip6tables` true, and a bridge network with IPv6
    enabled whose :drvopt:`com.docker.network.bridge.enable_ip_masquerade` is not ``false`` and
    whose :drvopt:`com.docker.network.bridge.gateway_mode_ipv6` is ``nat`` or not set, without
    :drvopt:`com.docker.network.host_ipv6`

    An underlay host observes an address of the node as the source address of IPv6 egress from the
    network's containers.

.. expectation:: An operator's nat rule ahead of masquerading survives a restart
    :id: BR-EGRESS-OPERATOR-NAT-RULE-FIRST
    :observed_from: underlay
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-NAT-OPERATOR-RULES-FIRST

    *Applies to:* The iptables backend, a user-defined bridge network whose
    :drvopt:`com.docker.network.bridge.enable_ip_masquerade` is not ``false`` and whose
    :drvopt:`com.docker.network.bridge.gateway_mode_ipv4` is ``nat`` or not set, without
    :drvopt:`com.docker.network.host_ipv4`, and a rule that an operator inserted at the start of
    the nat table's :ipt-chain:`POSTROUTING` chain after the network was created, which returns IPv4
    packets from the network's subnet to a given destination

    An underlay host at that destination observes the container's address as the source address of
    IPv4 egress from the network's containers, and still does after the daemon restarts.

.. expectation:: After a restart, the default bridge masquerades ahead of operator rules
    :id: BR-EGRESS-OPERATOR-RULE-DEFAULT-BRIDGE
    :observed_from: underlay
    :depends_on: os-linux
    :platforms: linux
    :concerns: BR-EGRESS-OPERATOR-NAT-RULE-FIRST
    :violates: REQ-BR-NAT-OPERATOR-RULES-FIRST

    *Applies to:* The iptables backend, the default bridge with masquerading, and a rule that an
    operator inserted at the start of the nat table's :ipt-chain:`POSTROUTING` chain, which returns
    IPv4 packets from the default bridge's subnet to a given destination

    After the daemon restarts, an underlay host at that destination observes an address of the node
    as the source address of IPv4 egress from the default bridge's containers.

.. expectation:: Without masquerade, IPv4 egress has the container's address as source
    :id: BR-EGRESS-NO-MASQUERADE
    :observed_from: underlay
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-EGRESS-NAT
    :needs: BR-HOST-NO-FORWARD-DROP

    *Applies to:* A bridge network with :drvopt:`com.docker.network.bridge.enable_ip_masquerade` set
    to ``false``, and without :drvopt:`com.docker.network.bridge.gateway_mode_ipv4`

    An underlay host observes the container's address as the source address of IPv4 egress from a
    container on the network.

.. expectation:: With the daemon's ip-masq false, default-bridge egress has the container's address
    :id: BR-EGRESS-DAEMON-NO-MASQUERADE
    :observed_from: underlay
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-EGRESS-NAT
    :needs: BR-HOST-NO-FORWARD-DROP

    *Applies to:* A daemon with :daemon-json:`ip-masq` false, and a container on the default bridge

    An underlay host observes the container's address as the source address of IPv4 egress from the
    container.

.. expectation:: Without masquerade, IPv6 egress has the container's address as source
    :id: BR-EGRESS-NO-MASQUERADE-V6
    :observed_from: underlay
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-EGRESS-NAT
    :needs: BR-HOST-FORWARDING-V6, BR-HOST-NO-FORWARD-DROP, BR-HOST-IP6TABLES-COMMAND, BR-HOST-IP6TABLES-KERNEL, BR-HOST-IP6TABLES-LEGACY-KERNEL, BR-HOST-IP6TABLES-NFT-KERNEL, BR-HOST-NFTABLES-V6

    *Applies to:* A daemon with :daemon-json:`ip6tables` true, and a bridge network with IPv6
    enabled, with :drvopt:`com.docker.network.bridge.enable_ip_masquerade` set to ``false`` and
    without :drvopt:`com.docker.network.bridge.gateway_mode_ipv6`

    An underlay host observes the container's IPv6 address as the source address of IPv6 egress
    from a container on the network.

.. expectation:: An unmasqueraded egress reply on an untrusted interface is not delivered
    :id: BR-EGRESS-REPLY-DIRECT-ROUTING-OFF
    :observed_from: underlay → ctr
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-NO-DIRECT-ROUTING

    *Applies to:* A daemon with :daemon-json:`allow-direct-routing` false, a bridge network with
    :drvopt:`com.docker.network.bridge.enable_ip_masquerade` set to ``false`` and without
    :drvopt:`com.docker.network.bridge.gateway_mode_ipv4`, and an underlay host with a route to the
    container's IPv4 address via the node's underlay address, whose replies to the container arrive
    on an interface that is not in the network's
    :drvopt:`com.docker.network.bridge.trusted_host_interfaces`

    An underlay host replies to IPv4 egress from a container on the network. The reply does not
    reach the container.

.. expectation:: With allow-direct-routing, a reply to unmasqueraded egress is delivered
    :id: BR-EGRESS-REPLY-DIRECT-ROUTING-ON
    :observed_from: underlay → ctr
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-DIRECT-ROUTING-OPT-IN
    :needs: BR-HOST-NO-FORWARD-DROP

    *Applies to:* A daemon with :daemon-json:`allow-direct-routing` true, a bridge network with
    :drvopt:`com.docker.network.bridge.enable_ip_masquerade` set to ``false`` and without
    :drvopt:`com.docker.network.bridge.gateway_mode_ipv4`, and an underlay host with a route to the
    container's IPv4 address via the node's underlay address

    An underlay host replies to IPv4 egress from a container on the network. The reply reaches the
    container.

.. expectation:: A reply to unmasqueraded egress on a trusted interface is delivered
    :id: BR-EGRESS-REPLY-TRUSTED-IFACE
    :observed_from: underlay → ctr
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-DIRECT-ROUTING-OPT-IN
    :needs: BR-HOST-NO-FORWARD-DROP

    *Applies to:* A bridge network with :drvopt:`com.docker.network.bridge.enable_ip_masquerade` set
    to ``false`` and without :drvopt:`com.docker.network.bridge.gateway_mode_ipv4`, and an underlay
    host with a route to the container's IPv4 address via the node's underlay address, whose replies
    to the container arrive on an interface in the network's
    :drvopt:`com.docker.network.bridge.trusted_host_interfaces`

    An underlay host replies to IPv4 egress from a container on the network. The reply reaches the
    container.

.. expectation:: With host_ipv4 set, IPv4 egress has that address as its source
    :id: BR-EGRESS-HOST-IPV4
    :observed_from: underlay
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-EGRESS-NAT
    :needs: BR-HOST-NO-FORWARD-DROP

    *Applies to:* A bridge network with :drvopt:`com.docker.network.host_ipv4` set to an address of
    the node, :drvopt:`com.docker.network.bridge.enable_ip_masquerade` not ``false``, and
    :drvopt:`com.docker.network.bridge.gateway_mode_ipv4` not ``routed``

    An underlay host observes that address as the source address of IPv4 egress from the network's
    containers.

.. expectation:: With host_ipv6 set, IPv6 egress has that address as its source
    :id: BR-EGRESS-HOST-IPV6
    :observed_from: underlay
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-EGRESS-NAT
    :needs: BR-HOST-FORWARDING-V6, BR-HOST-NO-FORWARD-DROP, BR-HOST-IP6TABLES-COMMAND, BR-HOST-IP6TABLES-KERNEL, BR-HOST-IP6TABLES-LEGACY-KERNEL, BR-HOST-IP6TABLES-NFT-KERNEL, BR-HOST-NFTABLES-V6

    *Applies to:* A daemon with :daemon-json:`ip6tables` true, and a bridge network with IPv6
    enabled, with :drvopt:`com.docker.network.host_ipv6` set to an IPv6 address of the node,
    :drvopt:`com.docker.network.bridge.enable_ip_masquerade` not ``false``, and
    :drvopt:`com.docker.network.bridge.gateway_mode_ipv6` not ``routed``

    An underlay host observes that address as the source address of IPv6 egress from the network's
    containers.

.. _bridge-networking-gateway-modes:

Gateway modes
=============

.. expectation:: In routed mode, IPv4 egress has the container's address as source
    :id: BR-GWMODE-ROUTED-EGRESS
    :observed_from: underlay
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-GWMODE-ROUTED-NO-NAT
    :needs: BR-HOST-NO-FORWARD-DROP

    *Applies to:* A bridge network with :drvopt:`com.docker.network.bridge.gateway_mode_ipv4` set to
    ``routed``

    An underlay host observes the container's address as the source address of IPv4 egress from a
    container on the network.

.. expectation:: In routed mode, IPv6 egress has the container's address as source
    :id: BR-GWMODE-ROUTED-EGRESS-V6
    :observed_from: underlay
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-GWMODE-ROUTED-NO-NAT
    :needs: BR-HOST-FORWARDING-V6, BR-HOST-NO-FORWARD-DROP, BR-HOST-IP6TABLES-COMMAND, BR-HOST-IP6TABLES-KERNEL, BR-HOST-IP6TABLES-LEGACY-KERNEL, BR-HOST-IP6TABLES-NFT-KERNEL, BR-HOST-NFTABLES-V6

    *Applies to:* A daemon with :daemon-json:`ip6tables` true, and a bridge network with IPv6
    enabled, with :drvopt:`com.docker.network.bridge.gateway_mode_ipv6` set to ``routed``

    An underlay host observes the container's IPv6 address as the source address of IPv6 egress
    from a container on the network.

.. expectation:: In routed mode, a client with a route reaches published ports only
    :id: BR-GWMODE-ROUTED-INBOUND
    :observed_from: client
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-GWMODE-DIRECT-ROUTING, REQ-BR-UNPUBLISHED-PORTS
    :needs: BR-HOST-NO-FORWARD-DROP

    *Applies to:* A container on a bridge network with
    :drvopt:`com.docker.network.bridge.gateway_mode_ipv4` set to ``routed``, and a client with a
    route to the container's IPv4 address via the node's address

    Traffic from the client to the container's IPv4 address reaches the container's published ports,
    and not its other ports.

.. expectation:: In routed mode, a client with an IPv6 route reaches published ports only
    :id: BR-GWMODE-ROUTED-INBOUND-V6
    :observed_from: client
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-GWMODE-DIRECT-ROUTING, REQ-BR-UNPUBLISHED-PORTS
    :needs: BR-HOST-FORWARDING-V6, BR-HOST-NO-FORWARD-DROP, BR-HOST-IP6TABLES-COMMAND, BR-HOST-IP6TABLES-KERNEL, BR-HOST-IP6TABLES-LEGACY-KERNEL, BR-HOST-IP6TABLES-NFT-KERNEL, BR-HOST-NFTABLES-V6

    *Applies to:* A daemon with :daemon-json:`ip6tables` true, a container on a bridge network with
    IPv6 enabled and :drvopt:`com.docker.network.bridge.gateway_mode_ipv6` set to ``routed``, and a
    client with a route to the container's IPv6 address via the node's address

    Traffic from the client to the container's IPv6 address reaches the container's published ports,
    and not its other ports.

.. expectation:: In routed mode, published ports are not reachable at the node's address
    :id: BR-GWMODE-ROUTED-NODE-ADDRESS
    :observed_from: client
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-GWMODE-ROUTED-NO-NAT

    *Applies to:* A container on a bridge network with
    :drvopt:`com.docker.network.bridge.gateway_mode_ipv4` set to ``routed``, whose IPv4 default
    route is through that network

    A client that connects to the node's IPv4 address, on a published port of the container, does
    not reach the container.

.. expectation:: In nat-unprotected mode, a client with a route reaches every port
    :id: BR-GWMODE-NAT-UNPROTECTED
    :observed_from: client
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-GWMODE-DIRECT-ROUTING
    :needs: BR-HOST-NO-FORWARD-DROP

    *Applies to:* A container on a bridge network with
    :drvopt:`com.docker.network.bridge.gateway_mode_ipv4` set to ``nat-unprotected``, and a client
    with a route to the container's IPv4 address via the node's address

    Traffic from the client to the container's IPv4 address reaches every port of the container.

.. expectation:: Isolated gateway mode on a network that is not internal is refused
    :id: BR-GWMODE-ISOLATED-NOT-INTERNAL
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :refines: REQ-BR-GWMODE-VALUES, REQ-NET-NETWORK-CREATE-ERRORS, PRIN-FAILS-CLOSED

    :api-op:`POST /networks/create` for a bridge network with :api-field:`Internal` not true and
    with :drvopt:`com.docker.network.bridge.gateway_mode_ipv4` or
    :drvopt:`com.docker.network.bridge.gateway_mode_ipv6` set to ``isolated`` responds with a 4xx or
    5xx status. :api-op:`GET /networks` then does not list a network with the request's
    :api-field:`Name`.

.. expectation:: Isolated gateway mode on a network that is not internal responds 500
    :id: BR-GWMODE-ISOLATED-NOT-INTERNAL-HTTP500
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :exact_form_of: BR-GWMODE-ISOLATED-NOT-INTERNAL
    :pins: BR-GWMODE-HTTP500-REFUSED

    :api-op:`POST /networks/create` for a bridge network with :api-field:`Internal` not true and
    with :drvopt:`com.docker.network.bridge.gateway_mode_ipv4` or
    :drvopt:`com.docker.network.bridge.gateway_mode_ipv6` set to ``isolated`` responds 500. The
    message contains :api-error:`gateway mode 'isolated' can only be used for an internal network`.

.. expectation:: docker network create without --internal refuses isolated mode
    :id: BR-GWMODE-ISOLATED-NOT-INTERNAL-CLI
    :observed_from: api
    :depends_on: cli
    :platforms: linux
    :refines: REQ-BR-GWMODE-VALUES, REQ-NET-NETWORK-CREATE-ERRORS, PRIN-FAILS-CLOSED
    :cli_for: BR-GWMODE-ISOLATED-NOT-INTERNAL

    :cli:`docker network create --driver bridge --opt
    com.docker.network.bridge.gateway_mode_ipv4=isolated` without :cli-flag:`--internal` exits
    non-zero and prints the daemon's error. :cli:`docker network ls` then does not list a network
    with the name that the command gave.

.. expectation:: Isolated gateway mode on a network that is not internal is refused with 500
    :id: BR-GWMODE-HTTP500-REFUSED
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :concerns: BR-GWMODE-ISOLATED-NOT-INTERNAL
    :violates: REQ-NET-API-ERROR-STATUS, REQ-NET-NETWORK-CREATE-ERRORS

    :api-op:`POST /networks/create` for a bridge network with
    :drvopt:`com.docker.network.bridge.gateway_mode_ipv4` or
    :drvopt:`com.docker.network.bridge.gateway_mode_ipv6` set to ``isolated``, when
    :api-field:`Internal` is not true, is refused and responds 500.

.. expectation:: An unknown gateway mode responds 400
    :id: BR-GWMODE-UNKNOWN
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :refines: REQ-BR-GWMODE-VALUES, REQ-NET-NETWORK-CREATE-ERRORS, REQ-NET-API-ERROR-STATUS, PRIN-FAILS-CLOSED

    :api-op:`POST /networks/create` for a bridge network with
    :drvopt:`com.docker.network.bridge.gateway_mode_ipv4` or
    :drvopt:`com.docker.network.bridge.gateway_mode_ipv6` set to a value other than ``nat``,
    ``nat-unprotected``, ``routed`` and ``isolated`` responds 400. :api-op:`GET /networks` then does
    not list a network with the request's :api-field:`Name`.

.. expectation:: An unknown gateway mode responds 400 and says the mode is unknown
    :id: BR-GWMODE-UNKNOWN-MSG
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :exact_form_of: BR-GWMODE-UNKNOWN

    :api-op:`POST /networks/create` for a bridge network with
    :drvopt:`com.docker.network.bridge.gateway_mode_ipv4` or
    :drvopt:`com.docker.network.bridge.gateway_mode_ipv6` set to a value other than ``nat``,
    ``nat-unprotected``, ``routed`` and ``isolated`` responds 400. The message contains
    :api-error:`unknown gateway mode`.

.. expectation:: docker network create with an unknown gateway mode exits non-zero
    :id: BR-GWMODE-UNKNOWN-CLI
    :observed_from: api
    :depends_on: cli
    :platforms: linux
    :refines: REQ-BR-GWMODE-VALUES, REQ-NET-NETWORK-CREATE-ERRORS, PRIN-FAILS-CLOSED
    :cli_for: BR-GWMODE-UNKNOWN

    :cli:`docker network create --driver bridge` with the option
    :drvopt:`com.docker.network.bridge.gateway_mode_ipv4` set to a value other than ``nat``,
    ``nat-unprotected``, ``routed`` and ``isolated`` exits non-zero and prints the daemon's error.
    :cli:`docker network ls` then does not list a network with the name that the command gave.

.. _bridge-networking-default-gateway:

Default gateway
===============

:ref:`network-common-default-gateway` in :doc:`network-common.md <network-common>` covers which
endpoint gives a container its default route, for every network driver. The expectations in this
subsection hold with :daemon-json:`userland-proxy` true and with :daemon-json:`userland-proxy`
false, unless an expectation names a value.

.. expectation:: A published port reaches the container on its default route's network
    :id: BR-GW-DEFAULT-ROUTE-NETWORK
    :observed_from: client, ctr
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-PUBLISH-GATEWAY-ENDPOINT
    :needs: BR-HOST-NO-FORWARD-DROP

    *Applies to:* A container with endpoints on two or more bridge networks that are not
    ``internal``, whose IPv4 default route is through a network whose
    :drvopt:`com.docker.network.bridge.gateway_mode_ipv4` is ``nat`` or not set

    An IPv4 client connects to a published port at the node's address. The connection reaches the
    container at its address on the network of its IPv4 default route. This holds whatever the
    gateway modes of the container's other networks. It holds after a connect or disconnect moves
    the default route to another such network.

.. _bridge-networking-hosts-without-ipv6:

Hosts without IPv6
==================

These expectations hold on a Linux host booted with ``ipv6.disable=1``. Such a host has no IPv6
stack, so it does not have :file:`/proc/sys/net/ipv6`.

.. expectation:: The daemon starts on a host booted with ipv6.disable=1
    :id: BR-NOV6-DAEMON-START
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :refines: REQ-BR-IPV6-UNAVAILABLE

    *Applies to:* A Linux host booted with ``ipv6.disable=1``

    The daemon starts. :api-op:`GET /info` responds 200.

.. expectation:: On a host without IPv6, a container on a bridge network starts
    :id: BR-NOV6-CTR-START
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :refines: REQ-BR-IPV6-UNAVAILABLE

    *Applies to:* A Linux host booted with ``ipv6.disable=1``, and a created container on a bridge
    network without IPv6

    :api-op:`POST /containers/{id}/start` for the container responds 204.

.. expectation:: On a host without IPv6, a published port is reachable over IPv4
    :id: BR-NOV6-PUBLISHED-PORT
    :observed_from: client
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-IPV6-UNAVAILABLE, REQ-BR-PUBLISH-DEFAULTS
    :needs: BR-HOST-NO-FORWARD-DROP

    *Applies to:* A Linux host booted with ``ipv6.disable=1``, and a running container on a bridge
    network without IPv6 that publishes a port without a host address

    A client reaches the container on that port at the node's IPv4 address.

.. expectation:: On a host without IPv6, an IPv6 bridge network is refused
    :id: BR-NOV6-V6-NETWORK-REFUSED
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :refines: REQ-BR-IPV6-UNAVAILABLE, PRIN-FAILS-CLOSED, REQ-NET-NETWORK-CREATE-ERRORS

    *Applies to:* A Linux host booted with ``ipv6.disable=1``

    :api-op:`POST /networks/create` for a bridge network with :api-field:`EnableIPv6` set responds
    with a 4xx or 5xx status. :api-op:`GET /networks` then does not list a network with the
    request's :api-field:`Name`.

.. expectation:: An IPv6 network's refusal without host IPv6 says a /proc file is missing
    :id: BR-NOV6-V6-NETWORK-REFUSED-MSG
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :concerns: BR-NOV6-V6-NETWORK-REFUSED
    :violates: PRIN-ERRORS-ACTIONABLE

    *Applies to:* A Linux host booted with ``ipv6.disable=1``

    The error response of :api-op:`POST /networks/create` for a bridge network with
    :api-field:`EnableIPv6` set has a message that contains :api-error:`Cannot read IPv6 setup for
    bridge` and :api-error:`no such file or directory`, with the path of a file under
    :file:`/proc/sys/net/ipv6`. The message does not say that IPv6 is disabled on the host.

.. _bridge-networking-deliberately-not-asserted:

***************************
 Deliberately not asserted
***************************

- The counters of Moby's own rules.
- The content of Moby's own chains and tables, and their names, outside the configurations that
  :ref:`bridge-networking-rule-content` pins. The exceptions are :ipt-chain:`DOCKER-USER` and the
  names that :ref:`bridge-networking-cross-generation-cleanup` pins for cleanup. A backend change
  replaces exactly this content. A contract asserts the packet-level effect of the rules, or an
  anchor in :ref:`bridge-networking-firewall-anchors-host-integration`, and never the content.
- Which packet-filtering technology is in use, beyond what the expectations of this spec assert for
  each backend.
- Whether an operator's :ipt-chain:`DOCKER-USER` rules survive a firewalld reload. That is
  firewalld's behavior.
