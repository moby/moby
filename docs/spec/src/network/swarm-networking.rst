###############################
 Swarm networking surface area
###############################

Scope: Swarm networking: the configuration of the cluster and its nodes, Swarm-scoped networks, the
overlay network driver's options and networks, and the ingress network, as observable from outside
the daemon.

This document is a behavior specification. For how the parts work, see `design.md
<../../../../daemon/libnetwork/docs/design.md>`__, `network.md
<../../../../daemon/libnetwork/docs/network.md>`__ and `networkdb.md
<../../../../daemon/libnetwork/docs/networkdb.md>`__.

:doc:`../README.md <../README>` gives the model that this spec follows: the items of a spec, their
categories and what their verdicts depend on. :doc:`README.md <README>` gives the conventions of the
networking specs, such as vantage points, daemon settings, timing and convergence ceilings, and host
requirements. The other specs in this directory use the same conventions. Unless an expectation says
otherwise, a ``ctr`` in this document is a standalone container attached to an ``attachable``
overlay.

In this spec, an overlay network is an overlay network other than the ingress network, unless an
expectation includes the ingress network. A node has a local attachment to a network while a task, a
standalone container or the node's load-balancer endpoint on that network is on the node. A network
is programmed on a node while the node has a local attachment to it. A node creates its
load-balancer endpoint on a network with the network, and deletes it with the network. Except on the
ingress network, the node deletes the network once no task or standalone container of the node is on
it, so the load-balancer endpoint does not outlast them. Unless an expectation or its section names
other values, an expectation holds in a Swarm with the cluster settings that :api-op:`POST
/swarm/init` sets by default. An expectation holds whatever data-path, advertise and listen
addresses each node gives in :api-op:`POST /swarm/init` or :api-op:`POST /swarm/join`, when the
data-path addresses of the Swarm's nodes are all of one IP version, and when each node's route for
its VXLAN traffic to the other nodes selects its data-path address as the source address
(:ref:`swarm-lifecycle-known-divergences-non-goals`), unless the expectation names that case.

An expectation that involves the ingress network holds for an ingress network that was created with
:api-field:`Ingress` set, that is not ``internal`` and that does not have the option :drvopt:`dsr`.
Only an expectation that names an ingress network without :api-field:`Ingress` covers such a network
(:ref:`swarm-networking-the-ingress-network`). An expectation that involves a node's gateway network
holds for one that the daemon created, or that the operator created with the options of one that the
daemon creates and no others but a subnet and a gateway (:ref:`swarm-traffic-gateway-network`),
unless the expectation names other options. The daemon creates its gateway network with
:drvopt:`com.docker.network.bridge.name` set to :network:`docker_gwbridge`,
:drvopt:`com.docker.network.bridge.enable_icc` set to ``false`` and
:drvopt:`com.docker.network.bridge.enable_ip_masquerade` set to ``true``. On an overlay network with
more than one subnet, an expectation about traffic to a VIP holds for the VIP and a backend task
only when both are in the subnet of the load-balancer endpoint of the client's node on that network
(:ref:`swarm-lifecycle-known-divergences-non-goals`). On Linux nodes, an expectation about a
:api-value:`mode=host` published port holds for a service that is attached to at least one network
that is not ``internal``, and for a service whose spec names no network and that publishes no port
in ingress mode, since its tasks run on the default bridge network
(:ref:`swarm-traffic-egress-the-gateway-network`). An expectation about reaching a
:api-value:`mode=host` published port at an address of the node holds on a network whose gateway
mode for that address's family is not ``routed``, without
:drvopt:`com.docker.network.bridge.host_binding_ipv4` and
:drvopt:`com.docker.network.bridge.inhibit_ipv4`, unless the expectation names them
(:doc:`bridge-networking.md <bridge-networking>`).

.. _swarm-networking-host-requirements:

*******************
 Host requirements
*******************

The conventions for this section are in :ref:`README.md <readme-host-requirements>`. The kernel
facilities and programs are requirements of each Linux node. The Windows facilities are requirements
of each Windows node.

.. _swarm-networking-kernel-facilities:

Kernel facilities
=================

.. hostreq:: XFRM with ESP in transport mode and rfc4106(gcm(aes))
    :id: SWARM-HOST-XFRM
    :kind: os
    :platforms: linux
    :kernel_options: XFRM, XFRM_USER, XFRM_ALGO, INET_ESP, CRYPTO_AES, CRYPTO_GCM, CRYPTO_SEQIV, INET_XFRM_MODE_TRANSPORT
    :tags: encrypted

    *Facility:* XFRM, with ESP in transport mode and the AEAD algorithm ``rfc4106(gcm(aes))`` with a
    64-bit ICV

    *Needed when:* An :drvopt:`encrypted` overlay network

    *Kernel options:* :kconfig:`XFRM`, :kconfig:`XFRM_USER`, :kconfig:`XFRM_ALGO`,
    :kconfig:`INET_ESP`, :kconfig:`CRYPTO_AES`, :kconfig:`CRYPTO_GCM`, :kconfig:`CRYPTO_SEQIV`.
    :kconfig:`INET_XFRM_MODE_TRANSPORT` on kernels that have it.

.. hostreq:: VXLAN devices
    :id: SWARM-HOST-VXLAN
    :kind: os
    :platforms: linux
    :kernel_options: VXLAN

    *Needed when:* Always

.. needextend:: type == "expectation" and docname in ['network/swarm-networking', 'network/swarm-traffic', 'network/swarm-lifecycle'] and ("linux" in platforms) and "SWARM-HOST-VXLAN" not in pins
    :+needs: SWARM-HOST-VXLAN

.. hostreq:: IPv6
    :id: SWARM-HOST-IPV6
    :kind: os
    :platforms: linux
    :kernel_options: IPV6
    :tags: ipv6

    *Needed when:* A node with an IPv6 data-path address

.. hostreq:: ESP over IPv6
    :id: SWARM-HOST-ESP-IPV6
    :kind: os
    :platforms: linux
    :kernel_options: INET6_ESP
    :tags: encrypted, ipv6

    *Needed when:* An :drvopt:`encrypted` overlay network, on a node with an IPv6 data-path address

.. hostreq:: Network namespaces, veth pairs and Linux bridges
    :id: SWARM-HOST-NETNS-VETH-BRIDGE
    :kind: os
    :platforms: linux
    :kernel_options: NET_NS, VETH, BRIDGE

    *Needed when:* Always

.. needextend:: type == "expectation" and docname in ['network/swarm-networking', 'network/swarm-traffic', 'network/swarm-lifecycle'] and ("linux" in platforms) and "SWARM-HOST-NETNS-VETH-BRIDGE" not in pins
    :+needs: SWARM-HOST-NETNS-VETH-BRIDGE

.. hostreq:: Netfilter hooks for bridged traffic
    :id: SWARM-HOST-BR-NETFILTER
    :kind: os
    :platforms: linux
    :kernel_options: BRIDGE_NETFILTER
    :when: iptables

    *Facility:* Netfilter hooks for bridged traffic (``br_netfilter``)

    *Needed when:* The daemon option :daemon-json:`iptables` true

.. needextend:: type == "expectation" and docname in ['network/swarm-networking', 'network/swarm-traffic', 'network/swarm-lifecycle'] and ("linux" in platforms) and "SWARM-HOST-BR-NETFILTER" not in pins
    :+needs: SWARM-HOST-BR-NETFILTER

.. hostreq:: IPVS with firewall marks, round-robin and connection tracking
    :id: SWARM-HOST-IPVS
    :kind: os
    :platforms: linux
    :kernel_options: IP_VS, IP_VS_RR, IP_VS_NFCT, IP_VS_PROTO_TCP, IP_VS_PROTO_UDP, IP_VS_PROTO_SCTP

    *Facility:* IPVS with firewall-mark virtual services, the round-robin scheduler and connection
    tracking

    *Needed when:* Always

.. hostreq:: SCTP
    :id: SWARM-HOST-SCTP
    :kind: os
    :platforms: linux
    :kernel_options: IP_SCTP, NF_CT_PROTO_SCTP

    *Needed when:* Always

    *Kernel options:* :kconfig:`IP_SCTP`. :kconfig:`NF_CT_PROTO_SCTP` on kernels that have it.

.. _swarm-networking-programs:

Programs
========

.. hostreq:: iptables with the tables, targets and matches it needs
    :id: SWARM-HOST-IPTABLES-COMMAND
    :kind: program
    :platforms: linux

    *Facility:* :program:`iptables`, accepting the filter, mangle and nat tables, the
    :ipt-target:`MARK`, :ipt-target:`DNAT`, :ipt-target:`SNAT` and :ipt-target:`REDIRECT` targets,
    and the ``ipvs``, ``conntrack``, ``sctp``, ``bpf``, ``policy``, ``owner`` and ``mark`` matches

    *Needed when:* Always

.. hostreq:: ip6tables with the tables, targets and matches it needs
    :id: SWARM-HOST-IP6TABLES-COMMAND
    :kind: program
    :platforms: linux
    :when: iptables-backend
    :tags: encrypted, ipv6

    *Facility:* :program:`ip6tables`, accepting the filter and mangle tables, the :ipt-target:`MARK`
    target, and the ``bpf``, ``policy``, ``owner`` and ``mark`` matches

    *Needed when:* The iptables backend, and an :drvopt:`encrypted` overlay network on a node with
    an IPv6 data-path address

The daemon does not depend on which implementation of :program:`iptables` and :program:`ip6tables`
the host has. The kernel options that the programs need depend on that implementation:

.. hostreq:: Kernel options for iptables
    :id: SWARM-HOST-IPTABLES-KERNEL
    :kind: os
    :platforms: linux
    :kernel_options: NETFILTER, NETFILTER_ADVANCED, NETFILTER_XTABLES, NF_CONNTRACK, NF_NAT, NETFILTER_XT_MARK, NETFILTER_XT_NAT, NETFILTER_XT_TARGET_REDIRECT, NETFILTER_XT_MATCH_IPVS, NETFILTER_XT_MATCH_CONNTRACK, NETFILTER_XT_MATCH_SCTP, NETFILTER_XT_MATCH_BPF, NETFILTER_XT_MATCH_POLICY, NETFILTER_XT_MATCH_OWNER
    :when: iptables-backend

    *Needed when:* The iptables backend

    The kernel options that :program:`iptables` needs, whichever implementation provides it. *Kernel
    options:* :kconfig:`NETFILTER`, :kconfig:`NETFILTER_ADVANCED`, :kconfig:`NETFILTER_XTABLES`,
    :kconfig:`NF_CONNTRACK`, :kconfig:`NF_NAT`, :kconfig:`NETFILTER_XT_MARK`,
    :kconfig:`NETFILTER_XT_NAT`, :kconfig:`NETFILTER_XT_TARGET_REDIRECT`,
    :kconfig:`NETFILTER_XT_MATCH_IPVS`, :kconfig:`NETFILTER_XT_MATCH_CONNTRACK`,
    :kconfig:`NETFILTER_XT_MATCH_SCTP`, :kconfig:`NETFILTER_XT_MATCH_BPF`,
    :kconfig:`NETFILTER_XT_MATCH_POLICY`, :kconfig:`NETFILTER_XT_MATCH_OWNER`

.. needextend:: type == "expectation" and docname in ['network/swarm-networking', 'network/swarm-traffic', 'network/swarm-lifecycle'] and ("linux" in platforms) and "SWARM-HOST-IPTABLES-KERNEL" not in pins
    :+needs: SWARM-HOST-IPTABLES-KERNEL

.. hostreq:: Kernel options for iptables from iptables-legacy
    :id: SWARM-HOST-IPTABLES-LEGACY-KERNEL
    :kind: os
    :platforms: linux
    :kernel_options: IP_NF_IPTABLES, IP_NF_FILTER, IP_NF_MANGLE, IP_NF_NAT, IP_NF_IPTABLES_LEGACY, NETFILTER_XTABLES_LEGACY
    :when: iptables-backend, iptables-legacy

    *Needed when:* The iptables backend with iptables-legacy providing :program:`iptables`

    The kernel options that :program:`iptables` needs when iptables-legacy provides it. *Kernel
    options:* :kconfig:`IP_NF_IPTABLES`, :kconfig:`IP_NF_FILTER`, :kconfig:`IP_NF_MANGLE`,
    :kconfig:`IP_NF_NAT`. :kconfig:`IP_NF_IPTABLES_LEGACY` from Linux 6.9, and
    :kconfig:`NETFILTER_XTABLES_LEGACY` from Linux 6.17.

.. needextend:: type == "expectation" and docname in ['network/swarm-networking', 'network/swarm-traffic', 'network/swarm-lifecycle'] and ("linux" in platforms) and "SWARM-HOST-IPTABLES-LEGACY-KERNEL" not in pins
    :+needs: SWARM-HOST-IPTABLES-LEGACY-KERNEL

.. hostreq:: Kernel options for iptables from iptables-nft
    :id: SWARM-HOST-IPTABLES-NFT-KERNEL
    :kind: os
    :platforms: linux
    :kernel_options: NF_TABLES, NF_TABLES_IPV4, NFT_COMPAT, NFT_NAT, NFT_CHAIN_NAT_IPV4, NFT_CHAIN_ROUTE_IPV4
    :when: iptables-backend, iptables-nft

    *Needed when:* The iptables backend with iptables-nft providing :program:`iptables`

    The kernel options that :program:`iptables` needs when iptables-nft provides it. *Kernel
    options:* :kconfig:`NF_TABLES`, :kconfig:`NF_TABLES_IPV4`, :kconfig:`NFT_COMPAT`,
    :kconfig:`NFT_NAT`. Before Linux 5.1, :kconfig:`NFT_CHAIN_NAT_IPV4` in place of
    :kconfig:`NFT_NAT`. Before Linux 5.2, :kconfig:`NFT_CHAIN_ROUTE_IPV4` for the
    :ipt-chain:`OUTPUT` chain of the mangle table.

.. needextend:: type == "expectation" and docname in ['network/swarm-networking', 'network/swarm-traffic', 'network/swarm-lifecycle'] and ("linux" in platforms) and "SWARM-HOST-IPTABLES-NFT-KERNEL" not in pins
    :+needs: SWARM-HOST-IPTABLES-NFT-KERNEL

.. hostreq:: Kernel options for ip6tables
    :id: SWARM-HOST-IP6TABLES-KERNEL
    :kind: os
    :platforms: linux
    :kernel_options: NETFILTER, NETFILTER_ADVANCED, NETFILTER_XTABLES, NF_CONNTRACK, NF_NAT, NETFILTER_XT_MARK, NETFILTER_XT_NAT, NETFILTER_XT_TARGET_REDIRECT, NETFILTER_XT_MATCH_IPVS, NETFILTER_XT_MATCH_CONNTRACK, NETFILTER_XT_MATCH_SCTP, NETFILTER_XT_MATCH_BPF, NETFILTER_XT_MATCH_POLICY, NETFILTER_XT_MATCH_OWNER
    :when: iptables-backend

    The kernel options that :program:`ip6tables` needs, whichever implementation provides it.
    *Kernel options:* The options that iptables needs, whichever implementation provides it

.. hostreq:: Kernel options for ip6tables from iptables-legacy
    :id: SWARM-HOST-IP6TABLES-LEGACY-KERNEL
    :kind: os
    :platforms: linux
    :kernel_options: IP6_NF_IPTABLES, IP6_NF_FILTER, IP6_NF_MANGLE, IP6_NF_IPTABLES_LEGACY, NETFILTER_XTABLES_LEGACY
    :when: iptables-backend, iptables-legacy

    The kernel options that :program:`ip6tables` needs when iptables-legacy provides it. *Kernel
    options:* :kconfig:`IP6_NF_IPTABLES`, :kconfig:`IP6_NF_FILTER`, :kconfig:`IP6_NF_MANGLE`.
    :kconfig:`IP6_NF_IPTABLES_LEGACY` from Linux 6.9, and :kconfig:`NETFILTER_XTABLES_LEGACY` from
    Linux 6.17.

.. hostreq:: Kernel options for ip6tables from iptables-nft
    :id: SWARM-HOST-IP6TABLES-NFT-KERNEL
    :kind: os
    :platforms: linux
    :kernel_options: NF_TABLES, NF_TABLES_IPV6, NFT_COMPAT, NFT_CHAIN_ROUTE_IPV6
    :when: iptables-backend, iptables-nft

    The kernel options that :program:`ip6tables` needs when iptables-nft provides it. *Kernel
    options:* :kconfig:`NF_TABLES`, :kconfig:`NF_TABLES_IPV6`, :kconfig:`NFT_COMPAT`. Before Linux
    5.2, :kconfig:`NFT_CHAIN_ROUTE_IPV6` for the :ipt-chain:`OUTPUT` chain of the mangle table.

.. _swarm-networking-windows-facilities:

Windows facilities
==================

.. hostreq:: Host Networking Service overlay networks with VSID policies
    :id: SWARM-HOST-HNS-OVERLAY
    :kind: os
    :platforms: windows

    *Needed when:* Always

.. needextend:: type == "expectation" and docname in ['network/swarm-networking', 'network/swarm-traffic', 'network/swarm-lifecycle'] and ("windows" in platforms) and "SWARM-HOST-HNS-OVERLAY" not in pins
    :+needs: SWARM-HOST-HNS-OVERLAY

.. _swarm-networking-host-configuration:

Host configuration
==================

These are requirements of each node, and of the physical network between nodes.

.. hostreq:: Firewalls pass the manager, gossip and data-path ports
    :id: SWARM-HOST-FIREWALL-CLUSTER-PORTS
    :kind: config
    :platforms: linux, windows
    :tags: swarm

    *Facility:* Each node's firewall and the physical network between nodes pass TCP to port 2377 of
    each manager, TCP and UDP on port 7946, and UDP on the data-path port, 4789 unless :api-op:`POST
    /swarm/init` set another

    *Needed when:* A node in a Swarm

.. needextend:: type == "expectation" and docname in ['network/swarm-networking', 'network/swarm-traffic', 'network/swarm-lifecycle'] and ("linux" in platforms or "windows" in platforms) and "SWARM-HOST-FIREWALL-CLUSTER-PORTS" not in pins
    :+needs: SWARM-HOST-FIREWALL-CLUSTER-PORTS

.. hostreq:: Firewalls pass IP protocol 50 (ESP)
    :id: SWARM-HOST-FIREWALL-ESP
    :kind: config
    :platforms: linux, windows
    :tags: encrypted

    *Facility:* Each node's firewall and the physical network between nodes pass IP protocol 50
    (ESP)

    *Needed when:* An :drvopt:`encrypted` overlay network

.. _swarm-networking-interop-premise:

*****************
 Interop premise
*****************

Wire and protocol compatibility is a hard requirement. Users upgrade a Swarm node by node. During a
rollout, the cluster is heterogeneous. Nodes with different engine versions must share an overlay
network and an ingress network. Everything in :ref:`swarm-lifecycle-wire-format-interop` is
contract.

.. _swarm-networking-cluster-node-configuration:

******************************
 Cluster & node configuration
******************************

These expectations cover cluster configuration. The accepted range of the data-path port is 1024 to
49151. The reference for :cli:`docker swarm init --data-path-port` documents this range.

.. expectation:: DataPathAddr sets the address peers send the node's VXLAN traffic to
    :id: SWARM-CLUSTER-DATA-PATH-ADDRESS
    :observed_from: underlay (outer destination address)
    :depends_on: os-linux, os-windows
    :platforms: linux, windows

    :api-field:`DataPathAddr` in :api-op:`POST /swarm/init` or :api-op:`POST /swarm/join` sets the
    address peers send the node's VXLAN traffic to.

.. expectation:: Without DataPathAddr, peers send VXLAN traffic to the advertise address
    :id: SWARM-CLUSTER-DATA-PATH-ADDRESS-DEFAULT
    :observed_from: underlay (outer destination address)
    :depends_on: os-linux, os-windows
    :platforms: linux, windows

    Without :api-field:`DataPathAddr`, peers send the node's VXLAN traffic to its advertise address.

.. expectation:: Peers keep sending to the address a DataPathAddr interface first had
    :id: SWARM-CLUSTER-DATA-PATH-ADDRESS-IFACE
    :observed_from: underlay (outer destination address)
    :depends_on: none
    :platforms: linux, windows

    :api-field:`DataPathAddr` in :api-op:`POST /swarm/init` or :api-op:`POST /swarm/join` names a
    network interface. Peers then send the node's VXLAN traffic to the address that the interface
    had when the request was made. They continue to send it to that address after the interface's
    address changes, and after the daemon restarts. See
    :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: VXLAN traffic's outer source address is the one the route selects
    :id: SWARM-CLUSTER-VXLAN-SOURCE-ADDRESS
    :observed_from: underlay (outer source address)
    :depends_on: os-linux
    :platforms: linux

    A node's VXLAN traffic to a peer has the outer source address that the node's route to that peer
    selects. This holds whether or not that address is the node's data-path address. See
    :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: DataPathPort in POST /swarm/init sets the cluster's VXLAN UDP port
    :id: SWARM-CLUSTER-DATA-PATH-PORT
    :observed_from: underlay
    :depends_on: os-linux, os-windows
    :platforms: linux, windows

    :api-field:`DataPathPort` in :api-op:`POST /swarm/init` sets the VXLAN UDP port for the whole
    cluster. See :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: Linux and Windows nodes exchange overlay traffic on the default port
    :id: SWARM-CLUSTER-DATA-PATH-PORT-DEFAULT
    :observed_from: task
    :depends_on: os-windows
    :platforms: linux, windows

    *Applies to:* A Swarm with the default data-path port

    Linux and Windows nodes in that Swarm exchange overlay traffic. The default data-path port is
    the UDP port that Windows nodes always use for VXLAN.

.. expectation:: docker swarm init refuses a data-path port outside the accepted range
    :id: SWARM-CLUSTER-DATA-PATH-PORT-INVALID-CLI
    :observed_from: api:solo
    :depends_on: cli
    :platforms: linux, windows
    :cli_for: SWARM-CLUSTER-DATA-PATH-PORT-INVALID

    *Applies to:* A node that is not yet in a Swarm

    :cli:`docker swarm init` with a data-path port outside the accepted range exits non-zero. The
    node is then still not in a Swarm.

.. expectation:: POST /swarm/init refuses a data-path port outside the accepted range
    :id: SWARM-CLUSTER-DATA-PATH-PORT-INVALID
    :observed_from: api:solo
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A node that is not yet in a Swarm

    :api-op:`POST /swarm/init` with a data-path port outside the accepted range responds with a 4xx
    or 5xx status. The node is then still not in a Swarm.

.. expectation:: A data-path port outside the accepted range responds 500
    :id: SWARM-CLUSTER-DATA-PATH-PORT-INVALID-HTTP500
    :observed_from: api:solo
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A node that is not yet in a Swarm

    :api-op:`POST /swarm/init` with a data-path port outside the accepted range responds 500. See
    :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: A data-path port of 0 in POST /swarm/init selects 4789
    :id: SWARM-CLUSTER-DATA-PATH-PORT-ZERO
    :observed_from: api:solo, api:mgr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A node that is not yet in a Swarm

    :api-op:`POST /swarm/init` with a data-path port of 0 responds 200. :api-op:`GET /info` on the
    node then reports 4789 as the data-path port in :api-field:`Swarm.Cluster`.

.. expectation:: An overlay without a subnet gets one of SubnetSize from DefaultAddrPool
    :id: SWARM-CLUSTER-ADDRESS-POOL-ALLOCATION
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* An overlay network, the ingress network included, that uses the default IPAM
    driver and is created without a subnet

    Once the network is allocated (:ref:`swarm-networking-network-lifecycle-api-surface`),
    :api-op:`GET /networks/{id}` on a manager reports a subnet for it. That subnet is inside one of
    the :api-field:`DefaultAddrPool` prefixes that :api-op:`GET /info` on a manager reports in
    :api-field:`Swarm.Cluster`. The prefix length of that subnet is the :api-field:`SubnetSize` that
    :api-op:`GET /info` reports.

.. expectation:: docker swarm init refuses a mask length over 29 or below a pool's prefix
    :id: SWARM-CLUSTER-SUBNET-SIZE-INVALID-CLI
    :observed_from: api:solo
    :depends_on: cli
    :platforms: linux, windows
    :cli_for: SWARM-CLUSTER-SUBNET-SIZE-INVALID

    *Applies to:* A node that is not yet in a Swarm

    :cli:`docker swarm init` with :cli-flag:`--default-addr-pool` exits non-zero when
    :cli-flag:`--default-addr-pool-mask-length` is greater than 29. It also exits non-zero when that
    mask length is less than the prefix length of one of the pools. In both cases, the node is then
    still not in a Swarm.

.. expectation:: POST /swarm/init refuses a SubnetSize over 29 or below a pool's prefix
    :id: SWARM-CLUSTER-SUBNET-SIZE-INVALID
    :observed_from: api:solo
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A node that is not yet in a Swarm

    :api-op:`POST /swarm/init` with :api-field:`DefaultAddrPool` set responds with a 4xx or 5xx
    status when the request's :api-field:`SubnetSize` is greater than 29. It also responds with a
    4xx or 5xx status when the request's :api-field:`SubnetSize` is less than the prefix length of one of the pools, or is 0 and one of the pools has a prefix longer than 24 bits. In both cases, the node is then still not in a Swarm.

.. expectation:: A SubnetSize over 29 or below a pool's prefix responds 500
    :id: SWARM-CLUSTER-SUBNET-SIZE-INVALID-HTTP500
    :observed_from: api:solo
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A node that is not yet in a Swarm

    :api-op:`POST /swarm/init` with :api-field:`DefaultAddrPool` set responds 500 when the request's
    :api-field:`SubnetSize` is greater than 29. It also responds 500 when the request's
    :api-field:`SubnetSize` is less than the prefix length of one of the pools, or is 0 and one of the pools has a prefix longer than 24 bits. See
    :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: POST /swarm/init accepts a SubnetSize of 0, and GET /info reports 0
    :id: SWARM-CLUSTER-SUBNET-SIZE-ZERO
    :observed_from: api:solo, api:mgr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A node that is not yet in a Swarm

    :api-op:`POST /swarm/init` with :api-field:`DefaultAddrPool` set, with no prefix longer than 24 bits, and a :api-field:`SubnetSize` of 0 responds 200. :api-op:`GET /info` on the node then reports a :api-field:`SubnetSize` of 0
    in :api-field:`Swarm.Cluster`. See :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: A manager's GET /info reports the data-path port, pool and subnet size
    :id: SWARM-CLUSTER-INFO-MANAGER
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    On a manager, :api-op:`GET /info` includes :api-field:`Swarm.Cluster`.
    :api-field:`Swarm.Cluster` includes the data-path port, :api-field:`DefaultAddrPool` and
    :api-field:`SubnetSize`. When :api-op:`POST /swarm/init` gave a pool, the pool and size are
    those that the request gave. When the request gave no pool, they are ``10.0.0.0/8`` and 24,
    whatever :api-field:`SubnetSize` the request gave.

.. expectation:: A manager's docker info prints the data-path port, pool and subnet size
    :id: SWARM-CLUSTER-INFO-MANAGER-CLI
    :observed_from: api:mgr
    :depends_on: cli
    :platforms: linux, windows
    :cli_for: SWARM-CLUSTER-INFO-MANAGER

    On a manager, :cli:`docker info` prints the data-path port, the default address pool and the
    subnet size. When :cli:`docker swarm init` gave :cli-flag:`--default-addr-pool`, the pool and
    size are those that the command gave. When the command gave no :cli-flag:`--default-addr-pool`,
    they are ``10.0.0.0/8`` and 24, even if it gave :cli-flag:`--default-addr-pool-mask-length`.

.. expectation:: A worker's GET /info does not include Swarm.Cluster
    :id: SWARM-CLUSTER-INFO-WORKER
    :observed_from: api:wkr
    :depends_on: none
    :platforms: linux, windows

    On a worker, :api-op:`GET /info` does not include :api-field:`Swarm.Cluster`.

.. expectation:: A worker's docker info prints no data-path port or address pool
    :id: SWARM-CLUSTER-INFO-WORKER-CLI
    :observed_from: api:wkr
    :depends_on: cli
    :platforms: linux, windows
    :cli_for: SWARM-CLUSTER-INFO-WORKER

    On a worker, :cli:`docker info` does not print the data-path port or a default address pool.

.. expectation:: Gossip uses TCP and UDP port 7946, which is not configurable
    :id: SWARM-CLUSTER-GOSSIP-PORT
    :observed_from: node
    :depends_on: none
    :platforms: linux, windows

    Gossip uses TCP and UDP port 7946. Neither port is configurable.

.. expectation:: Peers send a node's gossip traffic to its advertise address
    :id: SWARM-CLUSTER-GOSSIP-ADVERTISE-ADDRESS
    :observed_from: underlay
    :depends_on: none
    :platforms: linux, windows

    Peers send this node's gossip traffic, on TCP and UDP port 7946, to its advertise address.

.. expectation:: A node listens for gossip on port 7946 at its listen address's host
    :id: SWARM-CLUSTER-GOSSIP-LISTEN-ADDRESS
    :observed_from: node
    :depends_on: none
    :platforms: linux, windows

    A node listens for gossip on TCP and UDP port 7946, on the host part of its listen address.

.. expectation:: Gossip traffic shows no network IDs, service names or task addresses
    :id: SWARM-CLUSTER-GOSSIP-NO-CLEARTEXT
    :observed_from: underlay
    :depends_on: none
    :platforms: linux, windows

    A capture of gossip traffic, on TCP and UDP port 7946, does not show the cluster's network IDs,
    service names or task addresses in cleartext. This holds whether or not any network is
    :drvopt:`encrypted`.

.. expectation:: An IPv6 DataPathAddr produces IPv6 outer packets for overlay traffic
    :id: SWARM-CLUSTER-DATA-PATH-ADDRESS-V6
    :observed_from: underlay
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :needs: SWARM-HOST-IPV6

    An IPv6 :api-field:`DataPathAddr` in :api-op:`POST /swarm/init` or :api-op:`POST /swarm/join`
    produces IPv6 outer packets for overlay traffic. See
    :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: A join with a data-path address of the other IP version succeeds
    :id: SWARM-CLUSTER-DATA-PATH-ADDRESS-MIXED-VERSION
    :observed_from: api:solo
    :depends_on: none
    :platforms: linux, windows
    :needs: SWARM-HOST-IPV6

    *Applies to:* A Swarm whose nodes have data-path addresses of one IP version

    :api-op:`POST /swarm/join` to that Swarm with a data-path address of the other IP version
    responds 200. The node is then in the Swarm. See
    :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: docker swarm init and join fail on a node with live-restore
    :id: SWARM-CLUSTER-LIVE-RESTORE-REFUSED-CLI
    :observed_from: api:solo
    :depends_on: cli
    :platforms: linux
    :cli_for: SWARM-CLUSTER-LIVE-RESTORE-REFUSED

    *Applies to:* A Linux node that is not yet in a Swarm, with :daemon-json:`live-restore` true

    :cli:`docker swarm init` and :cli:`docker swarm join` exit non-zero. The node is then still not
    in a Swarm.

.. expectation:: POST /swarm/init and /swarm/join fail on a node with live-restore
    :id: SWARM-CLUSTER-LIVE-RESTORE-REFUSED
    :observed_from: api:solo
    :depends_on: none
    :platforms: linux

    *Applies to:* A Linux node that is not yet in a Swarm, with :daemon-json:`live-restore` true

    :api-op:`POST /swarm/init` and :api-op:`POST /swarm/join` respond with a 4xx or 5xx status. The
    node is then still not in a Swarm.

.. expectation:: POST /swarm/init and /swarm/join respond 500 with live-restore
    :id: SWARM-CLUSTER-LIVE-RESTORE-REFUSED-HTTP500
    :observed_from: api:solo
    :depends_on: none
    :platforms: linux

    *Applies to:* A Linux node that is not yet in a Swarm, with :daemon-json:`live-restore` true

    :api-op:`POST /swarm/init` and :api-op:`POST /swarm/join` respond 500. See
    :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: A daemon in a Swarm with live-restore does not start
    :id: SWARM-CLUSTER-LIVE-RESTORE-NO-START
    :observed_from: log
    :depends_on: none
    :platforms: linux

    *Applies to:* A Linux node that is already in a Swarm, with :daemon-json:`live-restore` true

    The daemon does not start. The daemon log contains an error.

.. expectation:: A Swarm node with live-restore logs incompatible with swarm mode
    :id: SWARM-CLUSTER-LIVE-RESTORE-NO-START-MSG
    :observed_from: log
    :depends_on: none
    :platforms: linux

    *Applies to:* A Linux node that is already in a Swarm, with :daemon-json:`live-restore` true

    The daemon log contains an error that contains :log:`incompatible with swarm mode`.

.. _swarm-networking-network-lifecycle-api-surface:

*********************************
 Network lifecycle & API surface
*********************************

A Swarm-scoped network is a network that the Swarm managers keep. Its driver is a multi-host driver,
such as :driver:`overlay`, or a node-local driver, such as :driver:`bridge`. A network of a
node-local driver is Swarm-scoped when it is created with :api-field:`Scope` set to ``swarm``
(:cli-flag:`--scope swarm`).

A Swarm-scoped network is allocated after the network is created, and asynchronously. In this spec,
a network is allocated once :api-op:`GET /networks/{id}` on a manager reports the network's
:api-field:`Driver`.

.. _swarm-networking-overlay-driver-options:

Overlay driver options
======================

An expectation in this section that refers to other expectations or to another section covers only
whether the daemon accepts and honors the option. Its ``depends_on`` field also covers only that.
The expectations or the section that it refers to specify the behavior that the option selects, and
what that behavior depends on.

.. expectation:: The mtu option sets the base MTU before encapsulation overhead
    :id: SWARM-OVLOPT-MTU
    :key: com.docker.network.driver.mtu
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows

    Sets the base MTU from which encapsulation overhead is subtracted. See
    :need:`SWARM-CTR-MTU-CLEARTEXT`, :need:`SWARM-CTR-MTU-CLEARTEXT-V6`,
    :need:`SWARM-CTR-MTU-ENCRYPTED` and :need:`SWARM-CTR-MTU-ENCRYPTED-V6`.

.. expectation:: The vxlanid_list option sets one VNI per IPAM subnet
    :id: SWARM-OVLOPT-VNI-LIST
    :key: com.docker.network.driver.overlay.vxlanid_list
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows

    Sets the VNIs, one per IPAM subnet. See :need:`SWARM-OVL-VNI-LIST`.

.. expectation:: The encrypted option selects an encrypted overlay
    :id: SWARM-OVLOPT-ENCRYPTED
    :key: encrypted
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows
    :needs: SWARM-HOST-XFRM, SWARM-HOST-IPTABLES-COMMAND

    Selects an encrypted overlay. See :ref:`swarm-lifecycle-encryption`.

.. expectation:: The dsr option selects direct server return for east-west VIP traffic
    :id: SWARM-OVLOPT-DSR
    :key: dsr
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND

    Selects direct server return for east-west VIP traffic on Linux nodes. See
    :need:`SWARM-LB-DSR-SOURCE` and :need:`SWARM-LB-DSR-DESTINATION`.

.. expectation:: On Linux, encrypted and dsr take effect whatever their value
    :id: SWARM-OVLOPT-FLAG-ANY-VALUE
    :key: encrypted, dsr
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows
    :needs: SWARM-HOST-XFRM, SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND

    On Linux nodes, each key takes effect when it is present in the network's options, whatever its
    value. This includes the empty string and ``false`` as values.

.. _swarm-networking-swarm-scoped-networks:

Swarm-scoped networks
=====================

The expectations in this subsection hold for any Swarm-scoped network, whatever its driver.

.. expectation:: A manager creates a network with Scope swarm or a multi-host driver
    :id: SWARM-SCOPED-CREATE
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A manager, and a network driver that is built in or that a managed plugin provides

    On that manager, :api-op:`POST /networks/create` with a name that no Swarm-scoped network has,
    and no network on that manager has, responds 201 when it sets :api-field:`Scope` to ``swarm`` or
    names a multi-host driver, such as :driver:`overlay`. :api-op:`GET /networks` on a manager then
    lists the network.

.. expectation:: docker network create --scope=swarm or --driver overlay makes a network
    :id: SWARM-SCOPED-CREATE-CLI
    :observed_from: api:mgr
    :depends_on: cli
    :platforms: linux, windows
    :cli_for: SWARM-SCOPED-CREATE

    *Applies to:* A manager

    On that manager, :cli:`docker network create --scope=swarm` with a name that no Swarm-scoped
    network has, and no network on that manager has, exits zero. :cli:`docker network create
    --driver overlay` with such a name, in place of the first command, also exits zero. :cli:`docker
    network ls` on a manager then lists the network.

.. expectation:: A Swarm-scoped network reports its Driver within the allocation ceiling
    :id: SWARM-SCOPED-CREATE-ALLOCATED
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows
    :within: SWARM-CEIL-ALLOCATION

    Within the :need:`[[title|lower]] ceiling ([[seconds]] s) <SWARM-CEIL-ALLOCATION>` after
    :api-op:`POST /networks/create` for a Swarm-scoped network responds 201, :api-op:`GET
    /networks/{id}` on a manager reports the network's :api-field:`Driver`. This does not apply to a
    network that cannot be allocated (:need:`SWARM-SCALE-UNALLOCATABLE-INSPECT`).

.. expectation:: Creating a Swarm-scoped network on a worker responds 403
    :id: SWARM-SCOPED-CREATE-WORKER-HTTP403
    :observed_from: api:wkr
    :depends_on: none
    :platforms: linux, windows

    On a worker, :api-op:`POST /networks/create` with :api-field:`Scope` set to ``swarm`` responds 403. It also
    responds 403 with a multi-host driver, such as :driver:`overlay`, whatever :api-field:`Scope` the request
    gives.

.. expectation:: docker network create of a Swarm-scoped network fails on a worker
    :id: SWARM-SCOPED-CREATE-WORKER-CLI
    :observed_from: api:wkr
    :depends_on: cli
    :platforms: linux, windows
    :cli_for: SWARM-SCOPED-CREATE-WORKER-HTTP403

    On a worker, :cli:`docker network create --scope=swarm` exits non-zero and prints the daemon's
    error. :cli:`docker network create --driver overlay` also exits non-zero and prints the daemon's
    error. See :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: A network that a service references is not deleted
    :id: SWARM-SCOPED-DELETE-IN-USE-SERVICE
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A Swarm with a Swarm-scoped network other than the ingress network
    (:ref:`swarm-networking-the-ingress-network`), and a service that references the network

    :api-op:`DELETE /networks/{id}` for the network responds with a 4xx or 5xx status. The network
    stays listed.

.. expectation:: Deleting a network that a service references responds 400
    :id: SWARM-SCOPED-DELETE-IN-USE-SERVICE-HTTP400
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A Swarm with a Swarm-scoped network other than the ingress network
    (:ref:`swarm-networking-the-ingress-network`), and a service that references the network

    :api-op:`DELETE /networks/{id}` for the network responds 400, with a message that contains
    :api-error:`is in use by service`. See :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: A network with an attached task or container is not deleted
    :id: SWARM-SCOPED-DELETE-IN-USE-TASK
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A Swarm with a Swarm-scoped network other than the ingress network
    (:ref:`swarm-networking-the-ingress-network`), and a task or container attached to it that is
    not shutting down

    :api-op:`DELETE /networks/{id}` for the network responds with a 4xx or 5xx status. The network
    stays listed.

.. expectation:: Deleting a network with an attached task or container responds 400
    :id: SWARM-SCOPED-DELETE-IN-USE-TASK-HTTP400
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A Swarm with a Swarm-scoped network other than the ingress network
    (:ref:`swarm-networking-the-ingress-network`), and a task or container attached to it that is
    not shutting down

    :api-op:`DELETE /networks/{id}` for the network responds 400, with a message that contains
    :api-error:`is in use by task`. See :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: docker network rm fails for a network that a service references
    :id: SWARM-SCOPED-DELETE-IN-USE-SERVICE-CLI
    :observed_from: api:mgr
    :depends_on: cli
    :platforms: linux, windows
    :cli_for: SWARM-SCOPED-DELETE-IN-USE-SERVICE

    *Applies to:* A Swarm with a Swarm-scoped network other than the ingress network, and a service
    that references the network

    :cli:`docker network rm` for the network exits non-zero and prints the daemon's error.
    :cli:`docker network ls` on a manager then still lists the network.

.. expectation:: docker network rm fails for a network with an attached task or container
    :id: SWARM-SCOPED-DELETE-IN-USE-TASK-CLI
    :observed_from: api:mgr
    :depends_on: cli
    :platforms: linux, windows
    :cli_for: SWARM-SCOPED-DELETE-IN-USE-TASK

    *Applies to:* A Swarm with a Swarm-scoped network other than the ingress network, and a task or
    container attached to it that is not shutting down

    :cli:`docker network rm` for the network exits non-zero and prints the daemon's error.
    :cli:`docker network ls` on a manager then still lists the network.

.. expectation:: Creating a network with a Swarm-scoped network's name responds 409
    :id: SWARM-SCOPED-NAME-CONFLICT
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A Swarm with a Swarm-scoped network

    :api-op:`POST /networks/create` with the name of that network responds 409. :api-op:`GET
    /networks` on a manager then lists only one network with that name.

.. expectation:: docker network create with a Swarm-scoped network's name fails
    :id: SWARM-SCOPED-NAME-CONFLICT-CLI
    :observed_from: api:mgr
    :depends_on: cli
    :platforms: linux, windows
    :cli_for: SWARM-SCOPED-NAME-CONFLICT

    *Applies to:* A Swarm with a Swarm-scoped network

    :cli:`docker network create` with the name of that network exits non-zero and prints the
    daemon's error. :cli:`docker network ls` on a manager then lists only one network with that
    name.

.. expectation:: A manager lists a local and a Swarm-scoped network of the same name
    :id: SWARM-SCOPED-NAME-COLLISION-LIST
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A manager with a network that is not Swarm-scoped and that has the name of a
    Swarm-scoped network

    On that manager, :api-op:`GET /networks` lists both networks.

.. expectation:: A name shared with a local network fails to inspect or delete either
    :id: SWARM-SCOPED-NAME-COLLISION-AMBIGUOUS
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A manager with a network that is not Swarm-scoped and that has the name of a
    Swarm-scoped network

    On that manager, :api-op:`GET /networks/{id}` and :api-op:`DELETE /networks/{id}` with that name
    respond with a 4xx or 5xx status. :api-op:`DELETE /networks/{id}` does not delete either
    network.

.. expectation:: A shared name responds 400 with a message that it is ambiguous
    :id: SWARM-SCOPED-NAME-COLLISION-AMBIGUOUS-HTTP400
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A manager with a network that is not Swarm-scoped and that has the name of a
    Swarm-scoped network

    On that manager, :api-op:`GET /networks/{id}` and :api-op:`DELETE /networks/{id}` with that name
    respond 400, with a message that contains :api-error:`is ambiguous`. See
    :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: docker network inspect and rm fail for a name a local network shares
    :id: SWARM-SCOPED-NAME-COLLISION-AMBIGUOUS-CLI
    :observed_from: api:mgr
    :depends_on: cli
    :platforms: linux, windows
    :cli_for: SWARM-SCOPED-NAME-COLLISION-AMBIGUOUS

    *Applies to:* A manager with a network that is not Swarm-scoped and that has the name of a
    Swarm-scoped network

    On that manager, :cli:`docker network inspect` and :cli:`docker network rm` with that name exit
    non-zero and print the daemon's error.

.. expectation:: The scope parameter selects which network a shared name returns
    :id: SWARM-SCOPED-NAME-COLLISION-SCOPE-FILTER
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A manager with a network that is not Swarm-scoped and that has the name of a
    Swarm-scoped network

    On that manager, :api-op:`GET /networks/{id}` with that name and with :api-query:`scope` set to
    ``local`` returns the network that is not Swarm-scoped. With :api-query:`scope` set to
    ``swarm``, it returns the Swarm-scoped network.

.. expectation:: A Swarm network's ID prefix wins over a local network of that name
    :id: SWARM-SCOPED-NAME-ID-PREFIX
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A manager with a network that is not Swarm-scoped whose name is a prefix of the ID
    of a Swarm-scoped network

    On that manager, :api-op:`GET /networks/{id}` with that name as ``{id}`` returns the
    Swarm-scoped network. See :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: A node with a same-named local network rejects the Swarm network's tasks
    :id: SWARM-SCOPED-NAME-COLLISION-TASKS
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A node with a network that is not Swarm-scoped and that has the name of a
    Swarm-scoped network

    On that node, the tasks of a service on the Swarm-scoped network are rejected. See
    :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: A container naming the Swarm network by ID fails to start with 500
    :id: SWARM-SCOPED-NAME-COLLISION-START-BY-ID
    :observed_from: api:active
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A node with a network that is not Swarm-scoped and that has the name of a
    Swarm-scoped network, with a created or stopped standalone container whose endpoint settings for
    the Swarm-scoped network carry its ID

    On that node, :api-op:`POST /containers/{id}/start` for the container responds 500, with a
    message that contains :api-error:`could not find network`. See
    :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: A container naming the shared name starts on the local network
    :id: SWARM-SCOPED-NAME-COLLISION-START-BY-NAME
    :observed_from: api:active
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A node with a network that is not Swarm-scoped and that has the name of a
    Swarm-scoped network, with a created standalone container whose endpoint settings name that
    network by its name

    On that node, :api-op:`POST /containers/{id}/start` for the container responds 204. :api-op:`GET
    /containers/{id}/json` then reports the ID of the network that is not Swarm-scoped as the
    :api-field:`NetworkID` for that name. See :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: GET /networks/{id} reports Scope swarm and the network's create fields
    :id: SWARM-SCOPED-INSPECT
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    Once a Swarm-scoped network is allocated
    (:ref:`swarm-networking-network-lifecycle-api-surface`), :api-op:`GET /networks/{id}` for that
    network reports :api-field:`Scope` as ``swarm``, and reports :api-field:`Created`. It also
    reports the :api-field:`Driver`, :api-field:`Ingress`, :api-field:`Attachable`,
    :api-field:`Internal`, :api-field:`EnableIPv6`, :api-field:`ConfigFrom` and :api-field:`Labels`
    given at create. For an overlay network, it also reports the :api-field:`IPAM` of the
    allocation.

.. expectation:: docker network inspect prints a Swarm-scoped network's Scope and Driver
    :id: SWARM-SCOPED-INSPECT-CLI
    :observed_from: api:mgr
    :depends_on: cli
    :platforms: linux, windows

    *Applies to:* An allocated Swarm-scoped network

    On a manager, :cli:`docker network inspect` for the network prints ``swarm`` as its
    :api-field:`Scope`, and prints its :api-field:`Driver`.

.. expectation:: docker network inspect --verbose prints Services on an attached node
    :id: SWARM-OVL-VERBOSE-SERVICES-CLI
    :observed_from: api:mgr, api:wkr
    :depends_on: cli
    :platforms: linux, windows

    *Applies to:* A node with a local attachment to an overlay network

    On that node, :cli:`docker network inspect --verbose` for that network prints
    :api-field:`Services`.

.. expectation:: A worker lists a Swarm-scoped network only while it has an attachment
    :id: SWARM-SCOPED-LIST-WORKER
    :observed_from: api:wkr
    :depends_on: none
    :platforms: linux, windows

    On a worker, :api-op:`GET /networks` lists a Swarm-scoped network other than the ingress network
    only while the worker has a local attachment to the network.

.. expectation:: dangling=true lists every Swarm-scoped network, and false attached ones
    :id: SWARM-SCOPED-LIST-DANGLING
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A manager

    :api-op:`GET /networks` with the filter :api-query:`dangling=true` lists every Swarm-scoped
    network, including the ingress network and networks that services use. :api-op:`GET /networks`
    with the filter :api-query:`dangling=false` lists the Swarm-scoped networks that have a local
    attachment on that manager. Those networks are then in both responses. See
    :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: An unattached worker's GET /networks/{id} responds 404
    :id: SWARM-SCOPED-INSPECT-WORKER-UNATTACHED
    :observed_from: api:wkr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A worker that does not have a local attachment to a Swarm-scoped network other
    than the ingress network

    On that worker, :api-op:`GET /networks/{id}` for that network responds 404.

.. expectation:: An unattached manager reports IPAM, empty Containers and no Peers
    :id: SWARM-SCOPED-INSPECT-MANAGER-UNATTACHED
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A manager that does not have a local attachment to an overlay network

    On that manager, :api-op:`GET /networks/{id}` for that network responds 200. The response
    reports the :api-field:`IPAM` of the network's allocation and an empty :api-field:`Containers`.
    It does not include :api-field:`Peers`.

.. expectation:: docker network ls on a manager lists each Swarm-scoped network
    :id: SWARM-SCOPED-LIST-MANAGER-CLI
    :observed_from: api:mgr
    :depends_on: cli
    :platforms: linux, windows

    :cli:`docker network ls` on a manager lists each Swarm-scoped network.

.. expectation:: A container on an attachable network starts on any active node
    :id: SWARM-SCOPED-ATTACHABLE-START
    :observed_from: api:active
    :depends_on: none
    :platforms: linux, windows

    On any node whose availability is ``active``, :api-op:`POST /containers/{id}/start` for a
    created or stopped container on an attachable Swarm-scoped network responds 204. The container
    is then running and has an address on the network. For a network created with
    :api-field:`ConfigFrom`, this holds for a network of a node-local driver, on a node that has the
    config-only network that :api-field:`ConfigFrom` names.

.. expectation:: docker run on an attachable network succeeds on any active node
    :id: SWARM-SCOPED-ATTACHABLE-START-CLI
    :observed_from: api:active
    :depends_on: cli
    :platforms: linux, windows
    :cli_for: SWARM-SCOPED-ATTACHABLE-START

    On any node whose availability is ``active``, :cli:`docker run -d --network` naming an
    attachable Swarm-scoped network exits zero. The container then runs and is attached to the
    network. For a network created with :api-field:`ConfigFrom`, this holds for a network of a
    node-local driver, on a node that has the config-only network that :api-field:`ConfigFrom`
    names.

.. expectation:: A running container connects to an attachable network on any active node
    :id: SWARM-SCOPED-ATTACHABLE-CONNECT
    :observed_from: api:active
    :depends_on: none
    :platforms: linux, windows

    On any node whose availability is ``active``, :api-op:`POST /networks/{id}/connect` to an
    attachable Swarm-scoped network, for a running standalone container, responds 200. The container
    then has an address on the network. For a network created with :api-field:`ConfigFrom`, this
    holds for a network of a node-local driver, on a node that has the config-only network that
    :api-field:`ConfigFrom` names.

.. expectation:: docker network connect to an attachable network succeeds on active nodes
    :id: SWARM-SCOPED-ATTACHABLE-CONNECT-CLI
    :observed_from: api:active
    :depends_on: cli
    :platforms: linux, windows
    :cli_for: SWARM-SCOPED-ATTACHABLE-CONNECT

    On any node whose availability is ``active``, :cli:`docker network connect` naming an attachable
    Swarm-scoped network and a running standalone container exits zero. The container then has an
    address on the network. For a network created with :api-field:`ConfigFrom`, this holds for a
    network of a node-local driver, on a node that has the config-only network that
    :api-field:`ConfigFrom` names.

.. expectation:: Connecting an already attached container again responds 409
    :id: SWARM-SCOPED-CONNECT-ALREADY-ATTACHED-HTTP409
    :observed_from: api:active
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A running standalone container on any node whose availability is ``active`` that
    is already attached to an attachable Swarm-scoped network

    :api-op:`POST /networks/{id}/connect` to that network, for that container, responds 409. The
    container's addresses on that network do not change. See
    :ref:`network-common-known-divergences-non-goals`.

.. expectation:: A container's attachment task disappears once it disconnects or stops
    :id: SWARM-SCOPED-ATTACHMENT-TASK-REMOVED
    :observed_from: api:mgr, api:active
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A standalone container on an attachable Swarm-scoped network

    The container disconnects from the network, or stops. The container's attachment task then
    disappears from :api-op:`GET /tasks` on a manager with the :api-query:`runtime=attachment`
    filter.

.. expectation:: A network can be deleted once its last standalone container leaves
    :id: SWARM-SCOPED-ATTACHABLE-LAST-DETACH-DELETE
    :observed_from: api:mgr, api:active
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A standalone container on an attachable Swarm-scoped network to which nothing else
    is attached

    The container disconnects from the network, or stops. :api-op:`DELETE /networks/{id}` for the
    network on a manager then responds 204.

.. expectation:: A container on an attachable network fails to start on a drained node
    :id: SWARM-SCOPED-DRAINED-START
    :observed_from: api:mgr, api:wkr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A created or stopped container on an attachable Swarm-scoped network, on a node
    whose availability is ``drain`` or ``pause``

    :api-op:`POST /containers/{id}/start` for the container responds with a 4xx or 5xx status.

.. expectation:: Starting on a drained or paused node responds 500 after 20 seconds
    :id: SWARM-SCOPED-DRAINED-START-HTTP500
    :observed_from: api:mgr, api:wkr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A created or stopped container on an attachable Swarm-scoped network, on a node
    whose availability is ``drain`` or ``pause``

    :api-op:`POST /containers/{id}/start` for the container responds 500 after 20 seconds, with a
    message that contains :api-error:`attaching to network failed, make sure your network options
    are correct and check manager logs: context deadline exceeded`. See
    :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: Connecting to an attachable network fails on a drained or paused node
    :id: SWARM-SCOPED-DRAINED-CONNECT
    :observed_from: api:mgr, api:wkr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A running standalone container on a node whose availability is ``drain`` or
    ``pause``, and an attachable Swarm-scoped network

    :api-op:`POST /networks/{id}/connect` to that network, for the container, responds with a 4xx or
    5xx status. :api-op:`GET /containers/{id}/json` then does not list the network in
    :api-field:`NetworkSettings.Networks`.

.. expectation:: Connecting on a drained or paused node responds 500 after 20 seconds
    :id: SWARM-SCOPED-DRAINED-CONNECT-HTTP500
    :observed_from: api:mgr, api:wkr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A running standalone container on a node whose availability is ``drain`` or
    ``pause``, and an attachable Swarm-scoped network

    :api-op:`POST /networks/{id}/connect` to that network, for the container, responds 500 after 20
    seconds, with a message that contains :api-error:`attaching to network failed, make sure your
    network options are correct and check manager logs: context deadline exceeded`. See
    :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: docker run on an attachable network fails on a drained or paused node
    :id: SWARM-SCOPED-DRAINED-START-CLI
    :observed_from: api:mgr, api:wkr
    :depends_on: cli
    :platforms: linux, windows
    :cli_for: SWARM-SCOPED-DRAINED-START

    *Applies to:* A node whose availability is ``drain`` or ``pause``, and an attachable
    Swarm-scoped network

    On that node, :cli:`docker run -d --network` naming that network exits non-zero and prints the
    daemon's error.

.. expectation:: A container on a non-attachable network is created
    :id: SWARM-SCOPED-NOT-ATTACHABLE-CREATE
    :observed_from: api:mgr, api:wkr
    :depends_on: none
    :platforms: linux, windows

    :api-op:`POST /containers/create` for a container on a non-attachable Swarm-scoped network
    responds 201. :api-op:`GET /containers/{id}/json` then reports the container in the ``created``
    state.

.. expectation:: A container on a non-attachable network fails to start
    :id: SWARM-SCOPED-NOT-ATTACHABLE-START
    :observed_from: api:mgr, api:wkr
    :depends_on: none
    :platforms: linux, windows

    :api-op:`POST /containers/{id}/start` for a created or stopped container on a non-attachable
    Swarm-scoped network, the ingress network included, responds with a 4xx or 5xx status.

.. expectation:: Starting a container on a non-attachable network responds 500
    :id: SWARM-SCOPED-NOT-ATTACHABLE-START-HTTP500
    :observed_from: api:mgr, api:wkr
    :depends_on: none
    :platforms: linux, windows

    :api-op:`POST /containers/{id}/start` for a created or stopped container on a non-attachable
    Swarm-scoped network, the ingress network included, responds 500, with a message that contains
    :api-error:`not manually attachable`. See :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: docker run on a non-attachable network fails
    :id: SWARM-SCOPED-NOT-ATTACHABLE-START-CLI
    :observed_from: api:mgr, api:wkr
    :depends_on: cli
    :platforms: linux, windows
    :cli_for: SWARM-SCOPED-NOT-ATTACHABLE-START

    :cli:`docker run -d --network` naming a non-attachable Swarm-scoped network, the ingress network
    included, exits non-zero and prints the daemon's error.

.. expectation:: A container without --rm stays created after a failed start
    :id: SWARM-SCOPED-NOT-ATTACHABLE-STAYS-CREATED
    :observed_from: api:mgr, api:wkr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A created container without auto-remove (:cli-flag:`--rm`) on a non-attachable
    Swarm-scoped network

    :api-op:`POST /containers/{id}/start` for the container fails. The container stays in the
    created state.

.. expectation:: A container with --rm is removed after a failed start
    :id: SWARM-SCOPED-NOT-ATTACHABLE-AUTO-REMOVE
    :observed_from: api:mgr, api:wkr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A created container with auto-remove (:cli-flag:`--rm`) on a non-attachable
    Swarm-scoped network

    :api-op:`POST /containers/{id}/start` for the container fails. The container is removed.

.. expectation:: Connecting a container to a non-attachable network fails
    :id: SWARM-SCOPED-NOT-ATTACHABLE-CONNECT
    :observed_from: api:mgr, api:wkr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A running standalone container, and a non-attachable Swarm-scoped network, the
    ingress network included

    :api-op:`POST /networks/{id}/connect` to that network, for the container, responds with a 4xx or
    5xx status. :api-op:`GET /containers/{id}/json` then does not list the network in
    :api-field:`NetworkSettings.Networks`.

.. expectation:: Connecting a container to a non-attachable network responds 500
    :id: SWARM-SCOPED-NOT-ATTACHABLE-CONNECT-HTTP500
    :observed_from: api:mgr, api:wkr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A running standalone container, and a non-attachable Swarm-scoped network, the
    ingress network included

    :api-op:`POST /networks/{id}/connect` to that network, for the container, responds 500, with a
    message that contains :api-error:`not manually attachable`. See
    :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: docker network connect to a non-attachable network fails
    :id: SWARM-SCOPED-NOT-ATTACHABLE-CONNECT-CLI
    :observed_from: api:mgr, api:wkr
    :depends_on: cli
    :platforms: linux, windows
    :cli_for: SWARM-SCOPED-NOT-ATTACHABLE-CONNECT

    *Applies to:* A running standalone container, and a non-attachable Swarm-scoped network, the
    ingress network included

    :cli:`docker network connect` naming that network and the container exits non-zero and prints
    the daemon's error.

.. expectation:: Connecting to an unknown network succeeds, and the start then fails
    :id: SWARM-SCOPED-CONNECT-UNKNOWN-NETWORK
    :observed_from: api:mgr, api:wkr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A node in a Swarm, and a created or stopped container

    :api-op:`POST /networks/{id}/connect` for the container, with an ``{id}`` that is not the name
    or an ID prefix of any network, responds 200. :api-op:`POST /containers/{id}/start` for the
    container then responds 500, with a message that contains :api-error:`network <name> not found`.
    ``<name>`` is the ``{id}`` that the connect gave. See
    :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: Tasks on a ConfigFrom network run where the config-only network exists
    :id: SWARM-SCOPED-CONFIG-FROM-TASKS
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A Swarm-scoped network of a node-local driver, such as :driver:`bridge`, with
    :api-field:`ConfigFrom` naming a config-only network that exists on each node that runs a task

    :api-op:`GET /tasks` reports the tasks of a service on the network as ``running``. See
    :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: A task's address is in its node's config-only network's subnet
    :id: SWARM-SCOPED-CONFIG-FROM-SUBNET
    :observed_from: task
    :depends_on: os-linux, os-windows
    :platforms: linux, windows

    *Applies to:* A Swarm-scoped network of a node-local driver, such as :driver:`bridge`, with
    :api-field:`ConfigFrom` naming a config-only network that exists on each node that runs a task

    Each task of a service on the network has an address on the network in the subnet of its node's
    config-only network. See :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: A node without the config-only network rejects the network's tasks
    :id: SWARM-SCOPED-CONFIG-FROM-MISSING-TASKS
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A Swarm-scoped network of a node-local driver, created with
    :api-field:`ConfigFrom` naming a config-only network, and a node that does not have that
    config-only network

    On that node, the tasks of a service on the network are rejected. Their error in :api-op:`GET
    /tasks` contains the name of the config-only network.

.. expectation:: A rejected task's error says the configuration network does not exist
    :id: SWARM-SCOPED-CONFIG-FROM-MISSING-TASKS-MSG
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A Swarm-scoped network of a node-local driver, created with
    :api-field:`ConfigFrom` naming a config-only network, and a node that does not have that
    config-only network

    On that node, the error of a rejected task in :api-op:`GET /tasks` contains
    :api-error:`configuration network "<name>" does not exist`.

.. expectation:: A node without the config-only network fails a container start
    :id: SWARM-SCOPED-CONFIG-FROM-MISSING-START
    :observed_from: api:active
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A Swarm-scoped network of a node-local driver, created with
    :api-field:`ConfigFrom` naming a config-only network, and a node whose availability is
    ``active`` that does not have that config-only network, with a created or stopped container on
    the network

    On that node, :api-op:`POST /containers/{id}/start` for the container responds with a 4xx or 5xx
    status.

.. expectation:: A node without the config-only network fails a connect
    :id: SWARM-SCOPED-CONFIG-FROM-MISSING-CONNECT
    :observed_from: api:active
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A Swarm-scoped network of a node-local driver, created with
    :api-field:`ConfigFrom` naming a config-only network, and a node whose availability is
    ``active`` that does not have that config-only network, with a running standalone container that
    is not on the network

    On that node, :api-op:`POST /networks/{id}/connect` to the network, for the container, responds
    with a 4xx or 5xx status. :api-op:`GET /containers/{id}/json` then does not list the network in
    :api-field:`NetworkSettings.Networks`.

.. expectation:: Without the config-only network, start responds 500 after 20 seconds
    :id: SWARM-SCOPED-CONFIG-FROM-MISSING-START-HTTP500
    :observed_from: api:active
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A Swarm-scoped network of a node-local driver, created with
    :api-field:`ConfigFrom` naming a config-only network, and a node whose availability is
    ``active`` that does not have that config-only network, with a created or stopped container on
    the network

    On that node, :api-op:`POST /containers/{id}/start` for the container responds 500 after 20
    seconds, with a message that contains :api-error:`attaching to network failed, make sure your
    network options are correct and check manager logs: context deadline exceeded`. :api-op:`POST
    /networks/{id}/connect` to the network for a running standalone container on that node responds
    the same way. See :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: An attached node reports its config-only network's IPAM configuration
    :id: SWARM-SCOPED-CONFIG-FROM-INSPECT-ATTACHED
    :observed_from: api:mgr, api:wkr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A Swarm-scoped network of a node-local driver, created with
    :api-field:`ConfigFrom` naming a config-only network, and a node with a local attachment to the
    network

    On that node, :api-op:`GET /networks/{id}` for the network reports the :api-field:`IPAM`
    configuration of that node's config-only network.

.. expectation:: An unattached manager reports an empty IPAM and the network's ConfigFrom
    :id: SWARM-SCOPED-CONFIG-FROM-INSPECT-UNATTACHED
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A Swarm-scoped network of a node-local driver, created with
    :api-field:`ConfigFrom` naming a config-only network, and a manager that does not have a local
    attachment to the network

    On that manager, :api-op:`GET /networks/{id}` for the network reports an empty :api-field:`IPAM`
    and reports the network's :api-field:`ConfigFrom`.

.. expectation:: Deleting a config-only network in use on the node responds 403
    :id: SWARM-SCOPED-CONFIG-ONLY-DELETE-REFUSED
    :observed_from: api:mgr, api:wkr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A Swarm-scoped network of a node-local driver, created with
    :api-field:`ConfigFrom` naming a config-only network, and a node with a local attachment to the
    network

    On that node, :api-op:`DELETE /networks/{id}` for the config-only network responds 403.
    :api-op:`GET /networks` on that node then still lists the config-only network.

.. expectation:: docker network rm fails for a config-only network in use on the node
    :id: SWARM-SCOPED-CONFIG-ONLY-DELETE-REFUSED-CLI
    :observed_from: api:mgr, api:wkr
    :depends_on: cli
    :platforms: linux, windows
    :cli_for: SWARM-SCOPED-CONFIG-ONLY-DELETE-REFUSED

    *Applies to:* A Swarm-scoped network of a node-local driver, created with
    :api-field:`ConfigFrom` naming a config-only network, and a node with a local attachment to the
    network

    On that node, :cli:`docker network rm` for the config-only network exits non-zero and prints the
    daemon's error.

.. expectation:: A config-only network the node does not use can be deleted
    :id: SWARM-SCOPED-CONFIG-ONLY-DELETE-UNUSED
    :observed_from: api:mgr, api:wkr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A Swarm-scoped network of a node-local driver, created with
    :api-field:`ConfigFrom` naming a config-only network, and a node that has the config-only
    network and does not have a local attachment to the network

    On that node, :api-op:`DELETE /networks/{id}` for the config-only network responds 204.

.. expectation:: A node-local Swarm-scoped network with IPv4 disabled responds 400
    :id: SWARM-SCOPED-NODE-LOCAL-V4-DISABLED
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    :api-op:`POST /networks/create` for a Swarm-scoped network of a node-local driver with
    :api-field:`EnableIPv4` false, at API version 1.48 or later, responds 400 with :api-error:`IPv4
    cannot be disabled in a Swarm scoped network`. The request also responds 400 with that message
    when the :drvopt:`com.docker.network.enable_ipv4` option is set to ``false``. See
    :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: docker network create --scope=swarm fails with IPv4 disabled
    :id: SWARM-SCOPED-NODE-LOCAL-V4-DISABLED-CLI
    :observed_from: api:mgr
    :depends_on: cli
    :platforms: linux, windows
    :cli_for: SWARM-SCOPED-NODE-LOCAL-V4-DISABLED

    :cli:`docker network create --scope=swarm` with a node-local driver and :cli-flag:`--ipv4=false`
    exits non-zero and prints the daemon's error. So does the same command with :cli-flag:`--opt
    com.docker.network.enable_ipv4=false` in place of :cli-flag:`--ipv4=false`.

.. expectation:: A manager reports empty IPAM and no Options for a node-local network
    :id: SWARM-SCOPED-NODE-LOCAL-IPAM-MANAGER
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    A :api-op:`POST /networks/create` request for a Swarm-scoped network of a node-local driver
    without :api-field:`ConfigFrom` has a subnet and gateway in :api-field:`IPAM.Config`. Its
    :api-field:`Options` has only the driver's own options, such as
    :drvopt:`com.docker.network.bridge.*`, or no options. The request responds 201. :api-op:`GET
    /networks/{id}` on a manager then reports an empty :api-field:`IPAM` and does not report any
    :api-field:`Options`. See :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: A task node reports a default-pool subnet and no create-time Options
    :id: SWARM-SCOPED-NODE-LOCAL-IPAM-TASK-NODE
    :observed_from: api:wkr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A Swarm-scoped network of a node-local driver that was created without
    :api-field:`ConfigFrom`, with a subnet and gateway in :api-field:`IPAM.Config`

    On a node that runs a task on that network, :api-op:`GET /networks/{id}` reports a subnet from
    the node's default address pools. It does not report any of the :api-field:`Options` given at
    create. See :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: A manager accepts ConfigFrom naming a config-only network only it has
    :id: SWARM-SCOPED-CONFIG-FROM-MANAGER-ONLY
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A manager with a config-only network that no other node has

    On that manager, :api-op:`POST /networks/create` for a Swarm-scoped network of a node-local
    driver, with :api-field:`ConfigFrom` naming the config-only network, responds 201. See
    :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: A manager without the config-only network responds 404 to ConfigFrom
    :id: SWARM-SCOPED-CONFIG-FROM-CREATE-MISSING
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A manager that does not have a given config-only network, and another manager that
    has it

    On the first manager, :api-op:`POST /networks/create` for a Swarm-scoped network of any driver,
    with :api-field:`ConfigFrom` naming that config-only network, responds 404. See
    :ref:`swarm-lifecycle-known-divergences-non-goals` and
    :ref:`network-common-known-divergences-non-goals`.

.. expectation:: An overlay network created with ConfigFrom responds 201
    :id: SWARM-SCOPED-CONFIG-FROM-OVERLAY
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A manager with a config-only network

    On that manager, :api-op:`POST /networks/create` for an overlay network, with
    :api-field:`ConfigFrom` naming the config-only network, responds 201. See
    :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: docker network create --config-from fails without the config-only one
    :id: SWARM-SCOPED-CONFIG-FROM-CREATE-MISSING-CLI
    :observed_from: api:mgr
    :depends_on: cli
    :platforms: linux, windows
    :cli_for: SWARM-SCOPED-CONFIG-FROM-CREATE-MISSING

    *Applies to:* A manager that does not have a given config-only network

    On that manager, :cli:`docker network create --scope=swarm --config-from` naming that network
    exits non-zero and prints the daemon's error. So does :cli:`docker network create --driver
    overlay --config-from` naming that network.

.. expectation:: A manager without the driver plugin responds 404 plugin not found
    :id: SWARM-SCOPED-PLUGIN-CREATE-MISSING-HTTP404
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A manager that does not have a given managed plugin

    On that manager, :api-op:`POST /networks/create` for a Swarm-scoped network whose driver is that
    plugin responds 404, with a message that contains :api-error:`plugin "<name>" not found`.

.. expectation:: A Swarm-scoped network on a legacy v1 driver plugin responds 400
    :id: SWARM-SCOPED-PLUGIN-LEGACY-HTTP400
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A manager with a legacy network plugin, a v1 plugin that the daemon finds in its
    plugin directories

    On that manager, :api-op:`POST /networks/create` for a Swarm-scoped network whose driver is that
    plugin responds 400.

.. expectation:: A legacy plugin's refusal says it is not supported in swarm mode
    :id: SWARM-SCOPED-PLUGIN-LEGACY-MSG
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A manager with a legacy network plugin, a v1 plugin that the daemon finds in its
    plugin directories

    On that manager, :api-op:`POST /networks/create` for a Swarm-scoped network whose driver is that
    plugin responds 400, with a message that contains :api-error:`is not supported in swarm mode`.

.. expectation:: Without the plugin on the leader, another manager's create responds 400
    :id: SWARM-SCOPED-PLUGIN-LEADER-MISSING
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A Swarm whose leader manager does not have a given managed plugin, and another
    manager that has it

    On that other manager, :api-op:`POST /networks/create` for a Swarm-scoped network whose driver
    or IPAM driver is that plugin responds 400, with a message that contains :api-error:`error
    during lookup of plugin <name>`. See :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: An overlay with the leader's IPAM plugin is allocated from any manager
    :id: SWARM-SCOPED-PLUGIN-IPAM-ON-LEADER
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A Swarm whose leader manager has a given managed plugin

    :api-op:`POST /networks/create` for an overlay network whose IPAM driver is that plugin responds 201.
    This holds whether or not the manager that handles the request has the plugin. The network is
    then allocated (:ref:`swarm-networking-network-lifecycle-api-surface`).

.. expectation:: A new leader without the plugin still reports the allocation
    :id: SWARM-SCOPED-PLUGIN-LEADER-CHANGE-ALLOCATION
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* An allocated network, and a manager that does not have the driver or IPAM plugin
    of that network

    Leadership moves to that manager. :api-op:`GET /networks/{id}` still reports the network's
    allocation. See :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: Under a new leader without the plugin, tasks stay in state new
    :id: SWARM-SCOPED-PLUGIN-LEADER-CHANGE-TASKS
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* An allocated network, and a manager that does not have the driver or IPAM plugin
    of that network

    Leadership moves to that manager. The tasks of a service on the network stay in the ``new``
    state. See :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: A new leader without the plugin logs failed allocating network
    :id: SWARM-SCOPED-PLUGIN-LEADER-CHANGE-LOG
    :observed_from: log
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* An allocated network, and a manager that does not have the driver or IPAM plugin
    of that network

    Leadership moves to that manager. The daemon log of the leader manager contains :log:`failed
    allocating network` and the network's ID. See
    :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: A driver plugin named without its tag responds 500
    :id: SWARM-SCOPED-PLUGIN-UNTAGGED-DRIVER
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A manager with a managed plugin that provides a network driver

    On that manager, :api-op:`POST /networks/create` for a Swarm-scoped network whose
    :api-field:`Driver` is the plugin's name without its tag, such as ``name`` for ``name:latest``,
    responds 500, with a message that contains :api-error:`could not resolve driver`. See
    :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: An IPAM plugin named without its tag responds 201 but is never allocated
    :id: SWARM-SCOPED-PLUGIN-UNTAGGED-IPAM
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A Swarm whose leader manager has a managed plugin that provides an IPAM driver

    :api-op:`POST /networks/create` for an overlay network whose IPAM driver is the plugin's name
    without its tag responds 201. This holds whether or not the manager that handles the request has
    the plugin. The network is never allocated
    (:ref:`swarm-networking-network-lifecycle-api-surface`). See
    :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: A task whose ifname the OS refuses fails with that name in its error
    :id: SWARM-SCOPED-IFNAME-REFUSED
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    A service task whose network attachment requests an :drvopt:`ifname` that the OS refuses fails.
    Its error in :api-op:`GET /tasks` contains the requested name.

.. expectation:: A service that requests any ifname is created
    :id: SWARM-SCOPED-IFNAME-ANY-VALUE
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    For any value of :drvopt:`ifname`, :api-op:`POST /services/create` for a service whose network
    attachment requests an :drvopt:`ifname` responds 201, and :api-op:`GET /services` then lists the
    service. See :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: A task with a refused ifname reports error renaming interface
    :id: SWARM-SCOPED-IFNAME-REFUSED-MSG
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A service whose network attachment requests an :drvopt:`ifname` that the OS
    refuses

    The error of each task of the service in :api-op:`GET /tasks` contains :api-error:`error
    renaming interface`. See :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: A service on host with overlay or ingress ports is created, tasks fail
    :id: SWARM-SCOPED-HOST-NETWORK-COMBINED
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    :api-op:`POST /services/create` for a service attached to the :network:`host` network and to an
    overlay network responds 201. So does a request for a service on :network:`host` that publishes
    a port in ingress mode. Each task of such a service fails. See
    :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. _swarm-networking-overlay-networks:

Overlay networks
================

The expectations in this subsection depend on the addresses that are allocated for a network: its
subnets, task addresses and VIPs. These addresses are allocated only for an overlay network. Where
an expectation says that :api-op:`GET /services/{id}` then reports a VIP, it reports it within the
allocation ceiling (:ref:`swarm-lifecycle-convergence`) after the request responds.

.. expectation:: An overlay network with IPv4 disabled responds 400
    :id: SWARM-OVL-V4-DISABLED
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    On a manager, :api-op:`POST /networks/create` for an overlay network with
    :api-field:`EnableIPv4` false, at API version 1.48 or later, responds 400. At any version, the
    request also responds 400 when the :drvopt:`com.docker.network.enable_ipv4` option is set to
    ``false``. See :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: docker network create --driver overlay fails with IPv4 disabled
    :id: SWARM-OVL-V4-DISABLED-CLI
    :observed_from: api:mgr
    :depends_on: cli
    :platforms: linux, windows
    :cli_for: SWARM-OVL-V4-DISABLED

    On a manager, :cli:`docker network create --driver overlay --ipv4=false` exits non-zero and
    prints the daemon's error. So does the same command with :cli-flag:`--opt
    com.docker.network.enable_ipv4=false` in place of :cli-flag:`--ipv4=false`.

.. expectation:: A user-specified vxlanid_list gives subnet i the VNI i
    :id: SWARM-OVL-VNI-LIST
    :observed_from: underlay
    :depends_on: os-linux, os-windows
    :platforms: linux, windows

    A user-specified :drvopt:`vxlanid_list` is honored: subnet *i* uses VNI *i*.

.. expectation:: A short vxlanid_list gets auto-allocated VNIs for the remaining subnets
    :id: SWARM-OVL-VNI-LIST-SHORT
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    :api-op:`POST /networks/create` for an overlay network whose :drvopt:`vxlanid_list` has fewer
    VNIs than the network has subnets responds 201. Once the network is allocated
    (:ref:`swarm-networking-network-lifecycle-api-surface`), :api-op:`GET /networks/{id}` on a
    manager reports the same number of VNIs in :drvopt:`vxlanid_list` as the overlay has subnets.
    These are the requested VNIs, then an auto-allocated VNI for each remaining subnet.

.. expectation:: A long vxlanid_list keeps its first VNIs and leaves the rest free
    :id: SWARM-OVL-VNI-LIST-LONG
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    :api-op:`POST /networks/create` for an overlay network whose :drvopt:`vxlanid_list` has more
    VNIs than the network has subnets responds 201. Once the network is allocated
    (:ref:`swarm-networking-network-lifecycle-api-surface`), :api-op:`GET /networks/{id}` on a
    manager reports the same number of VNIs in :drvopt:`vxlanid_list` as the overlay has subnets.
    These are the first VNIs of the requested list. :api-op:`POST /networks/create` for another
    overlay network whose :drvopt:`vxlanid_list` has one of the remaining VNIs responds 201, and
    that network is then allocated.

.. expectation:: An overlay network with another overlay's VNI is created
    :id: SWARM-OVL-VNI-DUPLICATE
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* An allocated overlay network

    :api-op:`POST /networks/create` for another overlay network, with a VNI in its
    :drvopt:`vxlanid_list` that :api-op:`GET /networks/{id}` reports in the first network's
    :drvopt:`vxlanid_list`, responds 201. :api-op:`GET /networks` on a manager then lists the new
    network. See :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: docker network create with another overlay's VNI succeeds
    :id: SWARM-OVL-VNI-DUPLICATE-CLI
    :observed_from: api:mgr
    :depends_on: cli
    :platforms: linux, windows
    :cli_for: SWARM-OVL-VNI-DUPLICATE

    *Applies to:* An allocated overlay network

    :cli:`docker network create --driver overlay --opt
    com.docker.network.driver.overlay.vxlanid_list=<VNI>`, with a VNI that :cli:`docker network
    inspect` prints in the first network's :drvopt:`vxlanid_list`, exits zero. :cli:`docker network
    ls` on a manager then lists the new network.

.. expectation:: An overlay network that is not allocated is listed and returned
    :id: SWARM-OVL-UNALLOCATED-LISTED
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* An overlay network that is not allocated

    :api-op:`GET /networks` lists the network. :api-op:`GET /networks/{id}` returns the network.

.. expectation:: An unallocated overlay does not hide other Swarm-scoped networks
    :id: SWARM-OVL-UNALLOCATED-OTHERS-LISTED
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    An overlay network that is not allocated does not prevent :api-op:`GET /networks` from listing
    any other Swarm-scoped network. It also does not prevent :api-op:`GET /networks/{id}` from
    returning any other Swarm-scoped network.

.. expectation:: Auto-allocated VNIs come from [4096, 2²⁴)
    :id: SWARM-OVL-VNI-AUTO-RANGE
    :observed_from: underlay
    :depends_on: none
    :platforms: linux, windows

    Auto-allocated VNIs come from [4096, 2²⁴).

.. expectation:: GET /networks/{id} reports the effective vxlanid_list in Options
    :id: SWARM-OVL-VNI-LIST-REPORTED
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    :api-op:`GET /networks/{id}` reports the effective :drvopt:`vxlanid_list` in
    :api-field:`Options`.

.. expectation:: An overlay network accepts and reports any mtu option value
    :id: SWARM-OVL-MTU-ANY-VALUE
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    :api-op:`POST /networks/create` for an overlay network responds 201 whatever value
    :drvopt:`com.docker.network.driver.mtu` has. :api-op:`GET /networks/{id}` on a manager then
    reports that value in :api-field:`Options`. See
    :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: docker network create accepts and inspect prints any overlay mtu value
    :id: SWARM-OVL-MTU-ANY-VALUE-CLI
    :observed_from: api:mgr
    :depends_on: cli
    :platforms: linux, windows
    :cli_for: SWARM-OVL-MTU-ANY-VALUE

    :cli:`docker network create --driver overlay` exits zero whatever value the :daemon-json:`mtu`
    option has. :cli:`docker network inspect` on a manager then prints that value in the network's
    options.

.. expectation:: A task or VIP can get one of an overlay's AuxiliaryAddresses
    :id: SWARM-OVL-AUXILIARY-ADDRESSES
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    :api-op:`POST /networks/create` for an overlay network with :api-field:`AuxiliaryAddresses` in
    :api-field:`IPAM.Config` responds 201. :api-op:`GET /networks/{id}` on a manager then does not
    report these addresses. A task or VIP on the network can get one of these addresses. See
    :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: A container does not start with a static address another one uses
    :id: SWARM-OVL-STATIC-ADDRESS-IN-USE
    :observed_from: api:active
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A created or stopped container with a static IPv4 address on an attachable
    overlay, and another container on the overlay that uses that address

    :api-op:`POST /containers/{id}/start` responds with a 4xx or 5xx status.

.. expectation:: A static address in use responds 500 after 20 seconds
    :id: SWARM-OVL-STATIC-ADDRESS-IN-USE-HTTP500
    :observed_from: api:active
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A created or stopped container with a static IPv4 address on an attachable
    overlay, and another container on the overlay that uses that address

    :api-op:`POST /containers/{id}/start` responds 500 after 20 seconds. The response's message
    contains :api-error:`attaching to network failed, make sure your network options are correct and
    check manager logs: context deadline exceeded`. See
    :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: docker run --ip with an address in use on the overlay fails
    :id: SWARM-OVL-STATIC-ADDRESS-IN-USE-CLI
    :observed_from: api:active
    :depends_on: cli
    :platforms: linux, windows
    :cli_for: SWARM-OVL-STATIC-ADDRESS-IN-USE

    *Applies to:* A container on an attachable overlay

    :cli:`docker run -d --network --ip` with that overlay and the container's IPv4 address on it
    exits non-zero and prints the daemon's error.

.. expectation:: An attached node refuses an off-subnet static address at create with 400
    :id: SWARM-OVL-STATIC-ADDRESS-OUTSIDE-ATTACHED-HTTP400
    :observed_from: api:active
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* An attachable overlay, and a node with a local attachment to it

    On that node, :api-op:`POST /containers/create` for a container on the overlay, with a static
    IPv4 address outside the overlay's subnets, responds 400. The response's message contains
    :api-error:`no configured subnet contains IP address`. See
    :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: An unattached node creates with an off-subnet address and start fails
    :id: SWARM-OVL-STATIC-ADDRESS-OUTSIDE-UNATTACHED
    :observed_from: api:active
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* An attachable overlay, and a node without a local attachment to it

    On that node, :api-op:`POST /containers/create` for a container on the overlay, with a static
    IPv4 address outside the overlay's subnets, responds 201. :api-op:`POST /containers/{id}/start`
    for that container then responds 500 after 20 seconds, with a message that contains
    :api-error:`attaching to network failed`. See
    :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: docker run --ip with an address outside the overlay's subnets fails
    :id: SWARM-OVL-STATIC-ADDRESS-OUTSIDE-CLI
    :observed_from: api:active
    :depends_on: cli
    :platforms: linux, windows

    *Applies to:* An attachable overlay

    :cli:`docker run -d --network --ip` with that overlay and an IPv4 address outside its subnets
    exits non-zero and prints the daemon's error.

.. expectation:: A container's address is free for a static address once it leaves
    :id: SWARM-OVL-STATIC-ADDRESS-RELEASED
    :observed_from: api:mgr, api:active
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A standalone container on an attachable overlay

    The container's attachment task disappears from :api-op:`GET /tasks` on a manager. The
    attachment task disappears after the container disconnects from the overlay or stops
    (:need:`SWARM-SCOPED-ATTACHMENT-TASK-REMOVED`). On any node whose availability is ``active``,
    :api-op:`POST /containers/{id}/start` for another container on the overlay, with the first
    container's address as its static IPv4 address, then responds 204.

.. expectation:: A vip service has one VIP from each attached overlay's subnet
    :id: SWARM-OVL-VIP-PER-NETWORK
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A service in vip mode

    :api-op:`GET /services/{id}` reports one VIP in :api-field:`Endpoint.VirtualIPs` for each
    overlay network that the service is attached to. Each of these VIPs is from that network's
    subnet.

.. expectation:: A vip service with an ingress-mode port has one VIP on ingress
    :id: SWARM-OVL-VIP-INGRESS
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A service in vip mode that publishes an ingress-mode port

    :api-op:`GET /services/{id}` for the service includes one VIP on the ingress network.

.. expectation:: A service keeps its VIP on a network through an update
    :id: SWARM-OVL-VIP-UPDATE
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A service in vip mode that is attached to a network

    After :api-op:`POST /services/{id}/update` that leaves the service in vip mode and attached to
    that network responds 200, :api-op:`GET /services/{id}` reports the same VIP on that network as
    before.

.. expectation:: A service keeps its VIPs after the leader changes
    :id: SWARM-OVL-VIP-LEADER-CHANGE
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    After the leader changes, :api-op:`GET /services/{id}` reports the same VIPs as before.

.. expectation:: docker service inspect prints one VIP for each attached overlay
    :id: SWARM-OVL-VIP-PER-NETWORK-CLI
    :observed_from: api:mgr
    :depends_on: cli
    :platforms: linux, windows
    :cli_for: SWARM-OVL-VIP-PER-NETWORK

    *Applies to:* A service in vip mode

    :cli:`docker service inspect` prints one VIP in :api-field:`Endpoint.VirtualIPs` for each
    overlay network that the service is attached to.

.. expectation:: The VIP that verbose network inspect reports load-balances to the tasks
    :id: SWARM-OVL-VERBOSE-VIP
    :observed_from: api:mgr, api:wkr (on a node with a local attachment to the network), task
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND

    The VIP that :api-op:`GET /networks/{id}?verbose=true` reports for a service is the address that
    load-balances to its tasks.

.. expectation:: Verbose network inspect lists running tasks and no stopped task
    :id: SWARM-OVL-VERBOSE-TASKS
    :observed_from: api:mgr, api:wkr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A node with a local attachment to an overlay network, and a service on that
    network

    Within the ceiling of the convergence class (:ref:`swarm-lifecycle-convergence`) of a change to
    the service's tasks, the tasks that :api-op:`GET /networks/{id}?verbose=true` reports for the
    service include each of its running tasks, as :ref:`swarm-traffic-service-discovery` defines
    them. They do not include a task whose container has stopped.

.. expectation:: Verbose network inspect on an attached node reports Services
    :id: SWARM-OVL-VERBOSE-SERVICES
    :observed_from: api:mgr, api:wkr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A node with a local attachment to an overlay network

    On that node, :api-op:`GET /networks/{id}?verbose=true` for that network reports
    :api-field:`Services`. :api-field:`Services` gives the VIP of each service and the endpoint IP
    of each task. On the ingress network, it also gives each service's published ports.

.. expectation:: Verbose network inspect on an unattached node reports no services
    :id: SWARM-OVL-VERBOSE-UNATTACHED
    :observed_from: api:mgr, api:wkr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A node that does not have a local attachment to an overlay network

    On that node, :api-op:`GET /networks/{id}?verbose=true` for that network does not report any
    services.

.. expectation:: Peers lists each attached node at its advertise address
    :id: SWARM-OVL-PEERS
    :observed_from: api:mgr, api:wkr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A node with a local attachment to an overlay network

    On that node, :api-op:`GET /networks/{id}` for the network lists in :api-field:`Peers` each node
    that has a local attachment to the network. This includes the node itself. The :api-field:`IP`
    of each entry is the advertise address of that node, whatever its data-path address is.

.. expectation:: IPsInUse rises when a task attaches and falls when it leaves
    :id: SWARM-OVL-STATUS-IN-USE
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    On a manager, at API version 1.52 or later, :api-field:`IPsInUse` in the
    :api-field:`Status.IPAM` of :api-op:`GET /networks/{id}`, with the network's name or a partial
    ID, increases when a task attaches. It decreases when a task leaves. See
    :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: IPsInUse plus DynamicIPsAvailable is the subnet's size during churn
    :id: SWARM-OVL-STATUS-TOTAL
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* An overlay network with an IPv4 subnet that has no :api-field:`IPRange`

    On a manager, at API version 1.52 or later, :api-op:`GET /networks/{id}`, with the network's
    name or a partial ID, reports :api-field:`IPsInUse` and :api-field:`DynamicIPsAvailable` for the
    subnet. Their sum equals the number of addresses in the subnet at every point during churn. That
    number is 2^(32 − prefix length). See :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: Task, VIP and load-balancer addresses stay inside the subnet's IPRange
    :id: SWARM-OVL-IPRANGE
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* An overlay network with a subnet that has an :api-field:`IPRange`

    Every task address, VIP and node load-balancer address is inside that :api-field:`IPRange`.
    :api-op:`GET /tasks`, :api-op:`GET /services/{id}` and :api-op:`GET /networks/{id}` report these
    addresses. The :api-field:`Gateway` that :api-op:`GET /networks/{id}` on a manager reports for
    the subnet is none of them.

.. expectation:: Without a Gateway, the gateway is the lowest IPRange non-network address
    :id: SWARM-OVL-IPRANGE-GATEWAY
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* An overlay network created with a subnet that has an :api-field:`IPRange` and no
    :api-field:`Gateway`

    The :api-field:`Gateway` that :api-op:`GET /networks/{id}` on a manager reports for the subnet
    is the lowest address in :api-field:`IPRange` other than the subnet's network address.

.. expectation:: From API 1.52, Status.IPAM has per-subnet IPsInUse, DynamicIPsAvailable
    :id: SWARM-OVL-STATUS-PER-SUBNET
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    On a manager, at API version 1.52 or later, :api-op:`GET /networks/{id}` reports
    :api-field:`Status.IPAM` with per-subnet :api-field:`IPsInUse` and
    :api-field:`DynamicIPsAvailable`.

.. expectation:: Before API 1.52, GET /networks/{id} does not include Status
    :id: SWARM-OVL-STATUS-API-VERSION
    :observed_from: api:mgr, api:wkr
    :depends_on: none
    :platforms: linux, windows

    At API versions before 1.52, :api-op:`GET /networks/{id}` does not include :api-field:`Status`.

.. expectation:: A worker's Status.IPAM reflects only the worker's own endpoints
    :id: SWARM-OVL-STATUS-WORKER
    :observed_from: api:wkr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A worker with a local attachment to an overlay network

    At API version 1.52 or later, the :api-field:`Status.IPAM` of :api-op:`GET /networks/{id}` on
    that worker reflects only the worker's own endpoints. It does not change when a task attaches on
    another node.

A regression revealed the failed-allocation expectation. In that regression, one unallocated network
made listing and inspection fail for every Swarm-scoped network. `moby/moby#53325
<https://github.com/moby/moby/pull/53325>`__ fixed the regression.

The lower bound of the range of auto-allocated VNIs avoids the 802.1Q VLAN range for Windows.

.. _swarm-networking-the-ingress-network:

The ingress network
===================

A Swarm-scoped network created with :api-field:`Ingress` set is an ingress network. An overlay
network named :network:`ingress` and created with the label :drvopt:`com.docker.swarm.internal` is
also an ingress network, whatever the label's value. Older daemons created the ingress network in
this second form, without :api-field:`Ingress`. :api-op:`GET /networks` reports :api-field:`Ingress`
as true for both forms.

An ingress network exists while :api-op:`GET /networks` on a manager lists a network whose
:api-field:`Ingress` is true. That network is the ingress network. :cli:`swarm init` creates it with
:api-field:`Ingress` set and names it :network:`ingress`. It can be removed, and an ingress network
can then be created with another name. A copy that a node keeps after the network is deleted
(:ref:`swarm-lifecycle-known-divergences-non-goals`) does not count.

.. expectation:: POST /swarm/init creates an ingress network named ingress
    :id: SWARM-INGRESS-DEFAULT
    :observed_from: api:solo, api:mgr
    :depends_on: none
    :platforms: linux, windows
    :within: SWARM-CEIL-ALLOCATION

    *Applies to:* A node that is not in a Swarm

    :api-op:`GET /networks` does not list a network whose :api-field:`Ingress` is true. Within the
    :need:`[[title|lower]] ceiling ([[seconds]] s) <SWARM-CEIL-ALLOCATION>` after :api-op:`POST
    /swarm/init` responds 200 on the node, an ingress network exists. Its name is
    :network:`ingress`.

.. expectation:: A second ingress network responds 409
    :id: SWARM-INGRESS-DUPLICATE-HTTP409
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A Swarm with an ingress network

    :api-op:`POST /networks/create` for an overlay network with :api-field:`Ingress` set and
    :api-field:`Attachable` not set responds 409.

.. expectation:: docker network create --ingress fails when an ingress network exists
    :id: SWARM-INGRESS-DUPLICATE-CLI
    :observed_from: api:mgr
    :depends_on: cli
    :platforms: linux, windows
    :cli_for: SWARM-INGRESS-DUPLICATE-HTTP409

    *Applies to:* A Swarm with an ingress network

    :cli:`docker network create --driver overlay --ingress` exits non-zero and prints the daemon's
    error.

.. expectation:: An ingress network with a node-local driver or no Driver responds 403
    :id: SWARM-INGRESS-NODE-LOCAL-DRIVER-HTTP403
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    :api-op:`POST /networks/create` with :api-field:`Ingress` set and a node-local driver, or no
    :api-field:`Driver`, responds 403.

.. expectation:: docker network create --ingress fails with a node-local driver
    :id: SWARM-INGRESS-NODE-LOCAL-DRIVER-CLI
    :observed_from: api:mgr
    :depends_on: cli
    :platforms: linux, windows
    :cli_for: SWARM-INGRESS-NODE-LOCAL-DRIVER-HTTP403

    :cli:`docker network create --ingress` with a node-local driver, or without
    :cli-flag:`--driver`, exits non-zero and prints the daemon's error.

.. expectation:: An ingress network with a plugin's multi-host driver responds 501
    :id: SWARM-INGRESS-PLUGIN-DRIVER
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A manager on which a managed plugin provides a multi-host driver other than
    :driver:`overlay`

    On that manager, :api-op:`POST /networks/create` with :api-field:`Ingress` set,
    :api-field:`Attachable` not set, and that driver responds 501.

.. expectation:: docker network create --ingress fails with a plugin's multi-host driver
    :id: SWARM-INGRESS-PLUGIN-DRIVER-CLI
    :observed_from: api:mgr
    :depends_on: cli
    :platforms: linux, windows
    :cli_for: SWARM-INGRESS-PLUGIN-DRIVER

    *Applies to:* A manager on which a managed plugin provides a multi-host driver other than
    :driver:`overlay`

    On that manager, :cli:`docker network create --ingress` with that driver exits non-zero, and
    prints the daemon's error.

.. expectation:: overlay is the only multi-host driver supported for the ingress network
    :id: SWARM-INGRESS-PLUGIN-DRIVER-OVERLAY-ONLY
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A manager on which a managed plugin provides a multi-host driver other than
    :driver:`overlay`

    :driver:`overlay` is the only multi-host driver that is supported for the ingress network. On
    that manager, :api-op:`POST /networks/create` with :api-field:`Ingress` set,
    :api-field:`Attachable` not set, and that driver responds 501.

.. expectation:: An ingress network with Attachable set responds 400
    :id: SWARM-INGRESS-ATTACHABLE-HTTP400
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    :api-op:`POST /networks/create` with both :api-field:`Ingress` and :api-field:`Attachable` set
    and a multi-host driver responds 400.

.. expectation:: docker network create --ingress --attachable fails
    :id: SWARM-INGRESS-ATTACHABLE-CLI
    :observed_from: api:mgr
    :depends_on: cli
    :platforms: linux, windows
    :cli_for: SWARM-INGRESS-ATTACHABLE-HTTP400

    :cli:`docker network create --ingress --attachable` with a multi-host driver exits non-zero and
    prints the daemon's error.

.. expectation:: A service that names the ingress network in its networks responds 400
    :id: SWARM-INGRESS-SERVICE-NETWORK-HTTP400
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    :api-op:`POST /services/create` or :api-op:`POST /services/{id}/update` that names the ingress
    network in :api-field:`TaskTemplate.Networks` responds 400. The request does not create or
    change the service.

.. expectation:: docker service create or update naming the ingress network fails
    :id: SWARM-INGRESS-SERVICE-NETWORK-CLI
    :observed_from: api:mgr
    :depends_on: cli
    :platforms: linux, windows
    :cli_for: SWARM-INGRESS-SERVICE-NETWORK-HTTP400

    :cli:`docker service create --network` and :cli:`docker service update --network-add` that name
    the ingress network exit non-zero and print the daemon's error. The commands do not create or
    change the service.

.. expectation:: The ingress network is not deleted while a service publishes through it
    :id: SWARM-INGRESS-DELETE-IN-USE
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A Swarm with a service that publishes a port in ingress mode

    :api-op:`DELETE /networks/{id}` for the ingress network responds with a 4xx or 5xx status. The
    network stays listed.

.. expectation:: Deleting the ingress network in use responds 400
    :id: SWARM-INGRESS-DELETE-IN-USE-HTTP400
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A Swarm with a service that publishes a port in ingress mode

    :api-op:`DELETE /networks/{id}` for the ingress network responds 400, with a message that
    contains :api-error:`ingress network cannot be removed because service`. See
    :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: docker network rm fails for the ingress network in use
    :id: SWARM-INGRESS-DELETE-IN-USE-CLI
    :observed_from: api:mgr
    :depends_on: cli
    :platforms: linux, windows
    :cli_for: SWARM-INGRESS-DELETE-IN-USE

    *Applies to:* A Swarm with a service that publishes a port in ingress mode

    :cli:`docker network rm` for the ingress network, given ``y`` at its confirmation prompt, exits
    non-zero and prints the daemon's error. :cli:`docker network ls` then still lists the network.

.. expectation:: docker network rm for ingress, declined at the prompt, keeps the network
    :id: SWARM-INGRESS-DELETE-DECLINED
    :observed_from: api:mgr
    :depends_on: cli
    :platforms: linux, windows

    *Applies to:* A Swarm with an ingress network

    :cli:`docker network rm` for the ingress network, given any answer other than ``y`` at its
    confirmation prompt, exits zero. :cli:`docker network ls` then still lists the network.

.. expectation:: An ingress network that no service publishes through is deleted
    :id: SWARM-INGRESS-DELETE-UNUSED
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A Swarm with an ingress network created with :api-field:`Ingress` set, where no
    service publishes a port in ingress mode

    :api-op:`DELETE /networks/{id}` for the ingress network responds 204. :api-op:`GET /networks` on
    a manager then does not list a network whose :api-field:`Ingress` is true.

.. expectation:: docker network rm deletes an ingress network that no service uses
    :id: SWARM-INGRESS-DELETE-UNUSED-CLI
    :observed_from: api:mgr
    :depends_on: cli
    :platforms: linux, windows
    :cli_for: SWARM-INGRESS-DELETE-UNUSED

    *Applies to:* A Swarm with an ingress network created with :api-field:`Ingress` set, where no
    service publishes a port in ingress mode

    :cli:`docker network rm` for the ingress network, given ``y`` at its confirmation prompt, exits
    zero. :cli:`docker network ls` then does not list the network.

.. expectation:: An ingress network with a name other than ingress can be created
    :id: SWARM-INGRESS-REPLACEMENT
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows
    :within: SWARM-CEIL-ALLOCATION

    *Applies to:* A Swarm without an ingress network

    :api-op:`POST /networks/create` for an overlay network with :api-field:`Ingress` set,
    :api-field:`Attachable` not set and a name other than :network:`ingress` responds 201. Within
    the :need:`[[title|lower]] ceiling ([[seconds]] s) <SWARM-CEIL-ALLOCATION>`, :api-op:`GET
    /networks` on a manager then lists that network with :api-field:`Ingress` true.

.. expectation:: docker network create --ingress with another name works if none exists
    :id: SWARM-INGRESS-REPLACEMENT-CLI
    :observed_from: api:mgr
    :depends_on: cli
    :platforms: linux, windows
    :cli_for: SWARM-INGRESS-REPLACEMENT
    :within: SWARM-CEIL-ALLOCATION

    *Applies to:* A Swarm without an ingress network

    :cli:`docker network create --driver overlay --ingress` with a name other than
    :network:`ingress` exits zero. Within the :need:`[[title|lower]] ceiling ([[seconds]] s)
    <SWARM-CEIL-ALLOCATION>`, :cli:`docker network ls` then lists the network.

.. expectation:: Without an ingress network, ingress-mode ports are refused
    :id: SWARM-INGRESS-ABSENT-PUBLISH
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A Swarm without an ingress network

    :api-op:`POST /services/create` and :api-op:`POST /services/{id}/update` with a port in ingress
    mode respond with a 4xx or 5xx status. The requests do not create or change the service.

.. expectation:: Without an ingress network, an ingress-mode port responds 400
    :id: SWARM-INGRESS-ABSENT-PUBLISH-HTTP400
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A Swarm without an ingress network

    :api-op:`POST /services/create` and :api-op:`POST /services/{id}/update` with a port in ingress
    mode respond 400, with a message that contains :api-error:`service needs ingress network, but no
    ingress network is present`. See :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: A custom ingress network reports its subnet and gateway
    :id: SWARM-INGRESS-CUSTOM-SUBNET
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A Swarm with an ingress network created with :api-field:`Ingress` set and with a
    custom subnet and gateway

    :api-op:`GET /networks/{id}` on a manager reports that subnet and gateway for the ingress
    network.

.. expectation:: A custom-subnet ingress network serves published ports on every node
    :id: SWARM-INGRESS-CUSTOM-SUBNET-REACHABLE
    :observed_from: client
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND

    *Applies to:* A Swarm with an ingress network created with :api-field:`Ingress` set and with a
    custom subnet and gateway

    A service created in that Swarm is reachable on its ingress-mode published ports on every node.

.. expectation:: Every node lists the ingress network with Ingress true
    :id: SWARM-INGRESS-LIST-EVERY-NODE
    :observed_from: api:mgr, api:wkr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A Swarm with an ingress network created with :api-field:`Ingress` set

    :api-op:`GET /networks` on every node lists that network and reports its :api-field:`Ingress` as
    true.

.. expectation:: ingress with the label com.docker.swarm.internal is an ingress network
    :id: SWARM-INGRESS-LABEL-IS-INGRESS
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A Swarm with an overlay network named :network:`ingress` that was created with the
    label :drvopt:`com.docker.swarm.internal` and without :api-field:`Ingress`

    :api-op:`GET /networks/{id}` on a manager reports :api-field:`Ingress` as true for that network.
    :api-op:`POST /networks/create` for an overlay network with :api-field:`Ingress` set responds
    409.

.. expectation:: A service that names the labeled ingress network responds 400
    :id: SWARM-INGRESS-LABEL-SERVICE-NETWORK-HTTP400
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A Swarm with an overlay network named :network:`ingress` that was created with the
    label :drvopt:`com.docker.swarm.internal` and without :api-field:`Ingress`

    :api-op:`POST /services/create` that names that network in :api-field:`TaskTemplate.Networks`
    responds 400.

.. expectation:: The labeled ingress network is not deleted while a service publishes
    :id: SWARM-INGRESS-LABEL-DELETE-IN-USE
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A Swarm with an overlay network named :network:`ingress` created with the label
    :drvopt:`com.docker.swarm.internal` and without :api-field:`Ingress`, and a service with an
    ingress-mode port

    :api-op:`DELETE /networks/{id}` for that network responds with a 4xx or 5xx status.

.. expectation:: Deleting the labeled ingress network in use responds 400
    :id: SWARM-INGRESS-LABEL-DELETE-IN-USE-HTTP400
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A Swarm with an overlay network named :network:`ingress` created with the label
    :drvopt:`com.docker.swarm.internal` and without :api-field:`Ingress`, and a service with an
    ingress-mode port

    :api-op:`DELETE /networks/{id}` for that network responds 400, with a message that contains
    :api-error:`ingress network cannot be removed because service`. See
    :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: The labeled ingress network serves published ports on task nodes
    :id: SWARM-INGRESS-LABEL-PUBLISH
    :observed_from: client
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND

    *Applies to:* A Swarm with an overlay network named :network:`ingress` that was created with the
    label :drvopt:`com.docker.swarm.internal` and without :api-field:`Ingress`

    A client can reach a service's ingress-mode published port at the address of a node that runs a
    task of the service.

.. expectation:: The com.docker.swarm.internal label marks ingress whatever its value
    :id: SWARM-INGRESS-LABEL-ANY-VALUE
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A Swarm with an overlay network named :network:`ingress` that was created with the
    label :drvopt:`com.docker.swarm.internal`

    That network is an ingress network whatever value the label has, including the empty string.

.. expectation:: A node not attached to the labeled ingress serves no published ports
    :id: SWARM-INGRESS-LABEL-UNATTACHED-NODE
    :observed_from: client
    :depends_on: os-linux, os-windows
    :platforms: linux, windows

    *Applies to:* A Swarm whose only ingress network is an overlay network named :network:`ingress`,
    created with the label :drvopt:`com.docker.swarm.internal` and without :api-field:`Ingress`

    On a node without a local attachment to that network, a client cannot reach an ingress-mode
    published port at the node's address. See :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: A worker not attached to the labeled ingress network does not list it
    :id: SWARM-INGRESS-LABEL-WORKER-UNLISTED
    :observed_from: api:wkr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A Swarm whose only ingress network is an overlay network named :network:`ingress`,
    created with the label :drvopt:`com.docker.swarm.internal` and without :api-field:`Ingress`

    On a worker without a local attachment to that network, :api-op:`GET /networks` does not list
    it. See :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: Standalone containers start on an attachable labeled ingress network
    :id: SWARM-INGRESS-LABEL-ATTACHABLE
    :observed_from: api:active
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A Swarm without an ingress network

    :api-op:`POST /networks/create` for an overlay network named :network:`ingress`, with the label
    :drvopt:`com.docker.swarm.internal`, with :api-field:`Attachable` set and without
    :api-field:`Ingress`, responds 201. On any node whose availability is ``active``, :api-op:`POST
    /containers/{id}/start` for a created or stopped container on that network then responds 204.
    See :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: A labeled ingress beside a custom one is listed but never allocated
    :id: SWARM-INGRESS-LABEL-SECOND
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A Swarm whose ingress network was created with :api-field:`Ingress` set, under a
    name other than :network:`ingress`

    :api-op:`POST /networks/create` for an overlay network named :network:`ingress`, with the label
    :drvopt:`com.docker.swarm.internal` and without :api-field:`Ingress`, responds 201. :api-op:`GET
    /networks` on a manager then lists two networks whose :api-field:`Ingress` is true. The new
    network is never allocated. See :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: The leader logs Cannot allocate ingress network for a labeled second one
    :id: SWARM-INGRESS-LABEL-SECOND-LOG
    :observed_from: log
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A Swarm whose ingress network was created with :api-field:`Ingress` set, under a
    name other than :network:`ingress`

    :api-op:`POST /networks/create` for an overlay network named :network:`ingress`, with the label
    :drvopt:`com.docker.swarm.internal` and without :api-field:`Ingress`, responds 201. The leader
    manager's log then contains :log:`Cannot allocate ingress network`. See
    :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: A deleted labeled ingress stays listed on attached nodes until restart
    :id: SWARM-INGRESS-LABEL-DELETE-STALE
    :observed_from: api:mgr, api:wkr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A Swarm with an overlay network named :network:`ingress` that was created with the
    label :drvopt:`com.docker.swarm.internal` and without :api-field:`Ingress`

    :api-op:`DELETE /networks/{id}` for that network responds 204. Each node that had a local
    attachment to that network then still lists it in :api-op:`GET /networks` until the node's
    daemon restarts. See :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: An attached manager refuses the name ingress with 409 until restart
    :id: SWARM-INGRESS-LABEL-DELETE-RECREATE
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A Swarm with a manager that has a local attachment to an overlay network named
    :network:`ingress` created with the label :drvopt:`com.docker.swarm.internal` and without
    :api-field:`Ingress`

    :api-op:`DELETE /networks/{id}` for that network responds 204. Before the manager's daemon
    restarts, :api-op:`POST /networks/create` on that manager with the name :network:`ingress`
    responds 409. See :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: An attached node rejects tasks on an overlapping ingress until restart
    :id: SWARM-INGRESS-LABEL-DELETE-OVERLAP
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A Swarm with a node that has a local attachment to an overlay network named
    :network:`ingress` created with the label :drvopt:`com.docker.swarm.internal` and without
    :api-field:`Ingress`

    :api-op:`DELETE /networks/{id}` for that network responds 204. Before the node's daemon
    restarts, tasks on that node that attach to a new ingress network with an overlapping subnet are
    rejected. Their error in :api-op:`GET /tasks` contains :api-error:`invalid pool request`. See
    :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. _swarm-networking-known-divergences-non-goals:

*******************************
 Known divergences & non-goals
*******************************

.. expectation:: IPv6 addressing inside the overlay
    :id: SWARM-OVL-V6-ADDRESSING
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    Not supported. With the default IPAM driver, :api-op:`POST /networks/create` accepts an overlay
    network with an IPv6 subnet, but the network is never allocated. An overlay network with
    :api-field:`EnableIPv6` set and without an IPv6 subnet is accepted, and the network's
    attachments get IPv4 addresses only. Each attachment gets exactly one address. The service mesh
    is IPv4-only.

.. expectation:: Outer source address of VXLAN traffic
    :id: SWARM-CLUSTER-VXLAN-SOURCE-NOT-DATA-PATH
    :observed_from: underlay
    :depends_on: none
    :platforms: linux, windows

    Not guaranteed. The outer source address is the address that the node's route to each peer
    selects. It is meant to be the node's data-path address. When that route selects a different
    address, peers drop all of the node's overlay traffic. Neither node's daemon log has an error
    (`moby/moby#53004 <https://github.com/moby/moby/issues/53004>`__).

.. expectation:: Data-path address on Windows
    :id: SWARM-CLUSTER-WINDOWS-DATA-PATH-ADDRESS
    :observed_from: underlay
    :depends_on: none
    :platforms: windows

    Not applied. A Windows node's tunnel endpoint is the address of its network adapter, whatever
    :api-field:`DataPathAddr` or the advertise address says.

.. expectation:: Data-path port on Windows
    :id: SWARM-CLUSTER-WINDOWS-DATA-PATH-PORT
    :observed_from: underlay
    :depends_on: none
    :platforms: windows

    Not applied. Windows nodes do not use the cluster's data-path port. In a cluster with a
    non-default data-path port, Windows nodes cannot exchange overlay traffic with Linux nodes.

.. expectation:: Interface names as DataPathAddr or AdvertiseAddr
    :id: SWARM-CLUSTER-IFACE-NAME-RESOLVED-ONCE
    :observed_from: underlay
    :depends_on: none
    :platforms: linux, windows

    Resolved one time. :api-op:`POST /swarm/init` and :api-op:`POST /swarm/join` replace an
    interface name with the interface's address when the request is made. The node keeps that
    address after the interface's address changes. It also keeps it after the daemon restarts, even
    when no interface on the node has that address
    (:ref:`swarm-networking-cluster-node-configuration`).

.. expectation:: Mixed IPv4 and IPv6 data-path addresses
    :id: SWARM-CLUSTER-DATA-PATH-MIXED-VERSIONS-ACCEPTED
    :observed_from: api:solo
    :depends_on: none
    :platforms: linux, windows

    Not refused. A node can join a Swarm with a data-path address of a different IP version from the
    data-path addresses of the other nodes. Overlay traffic between two nodes whose data-path
    addresses have different IP versions fails. Only the daemon log of each node reports the failure
    to program the other node as a peer (:ref:`swarm-networking-cluster-node-configuration`,
    :need:`SWARM-FAILREP-PEER`).

.. expectation:: Network allocation failures are not reported to the caller
    :id: SWARM-OVL-ALLOCATION-FAILURE-UNREPORTED
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    :api-op:`POST /networks/create` for an overlay network that cannot be allocated responds 201.
    This occurs when a VNI in the network's :drvopt:`vxlanid_list` is in use, or when its address
    pool is exhausted. It also occurs when its requested subnet overlaps the subnet of another Swarm
    network. It also occurs when it has an IPv6 subnet under the default IPAM driver. Networks are
    allocated asynchronously. When a network cannot be allocated, the failure appears in the leader
    manager's log. Allocation is tried again later, for example after another network, service or
    task is removed. Nothing tells the caller about the failure. :api-op:`POST /services/create` for
    a service on such a network also responds 201. The service's tasks never start, and nothing
    tells the caller why. On a manager, :api-op:`GET /networks/{id}` reports the network with an
    empty :api-field:`Driver` and :api-field:`IPAM`, even when the request named them
    (:ref:`swarm-networking-overlay-networks`, :ref:`swarm-lifecycle-scale-and-exhaustion`).

.. expectation:: Network status by full ID on a manager
    :id: SWARM-OVL-STATUS-FULL-ID
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    Node-local. Take a manager that has a local attachment to an overlay network. On that manager,
    :api-op:`GET /networks/{id}` with the network's full ID reports only that node's own endpoints
    in :api-field:`Status.IPAM`. With the network's name or a partial ID, it reports the manager's
    allocation for the whole network (:ref:`swarm-networking-overlay-networks`).

.. expectation:: HTTP 500 for refused requests
    :id: SWARM-CLUSTER-HTTP500-REFUSED
    :observed_from: api
    :depends_on: none
    :platforms: linux

    Each request below is refused on purpose, but the request responds 500. 500 is the status for a
    fault in the daemon. The requests are: (1) :api-op:`POST /swarm/init` with a data-path port
    outside the accepted range. (2) :api-op:`POST /swarm/init` with a :api-field:`SubnetSize`
    greater than 29, or with a :api-field:`SubnetSize` less than the prefix length of one of its :api-field:`DefaultAddrPool` prefixes, or of 0 with one of those prefixes longer than 24 bits. (3) :api-op:`POST /swarm/init` and :api-op:`POST
    /swarm/join` with :daemon-json:`live-restore` true. (4) :api-op:`POST /swarm/init` and
    :api-op:`POST /swarm/join` with :daemon-json:`firewall-backend` set to ``nftables``, with
    :daemon-json:`iptables` true, and without :daemon-json:`features.swarm-nftables`. (5)
    :api-op:`POST /containers/{id}/start` for a created or stopped container on a non-attachable
    Swarm-scoped network, and :api-op:`POST /networks/{id}/connect` to such a network for a running
    container. (6) :api-op:`POST /containers/{id}/start` for a created or stopped container that
    publishes a port that a service publishes in ingress mode. (7) On a node in a Swarm,
    :api-op:`POST /containers/{id}/start` for a created or stopped container whose network was
    deleted after the container was created, or never existed. For some other invalid :api-op:`POST
    /swarm/init` parameters, such as an invalid listen address, the request responds 400
    (:ref:`swarm-networking-cluster-node-configuration`,
    :ref:`swarm-networking-swarm-scoped-networks`, :ref:`swarm-traffic-port-allocation`,
    :ref:`swarm-lifecycle-churn`).

.. expectation:: SubnetSize of 0 at init
    :id: SWARM-CLUSTER-SUBNET-SIZE-ZERO-UNALLOCATED
    :observed_from: api:solo
    :depends_on: none
    :platforms: linux, windows

    Accepted. :api-op:`POST /swarm/init` with :api-field:`DefaultAddrPool` set, with no prefix longer than 24 bits, and a :api-field:`SubnetSize` of 0 responds 200. The leader manager then cannot create its network
    allocator. No Swarm-scoped network is allocated, the ingress network included. Only the leader
    manager's log reports this, in an entry that contains :log:`failed to create allocator`
    (:ref:`swarm-networking-cluster-node-configuration`).

.. expectation:: Error for an overlay network created on a worker
    :id: SWARM-SCOPED-WORKER-OVERLAY-CREATE-ERROR
    :observed_from: api:wkr
    :depends_on: none
    :platforms: linux, windows
    :rationale: RAT-SWARM-SCOPED-WORKER-OVERLAY-CREATE-ERROR

    Misleading. On a worker, the error from :cli:`docker network create --driver overlay` says
    :api-error:`This node is not a swarm manager. Use "docker swarm init" or "docker swarm join" to
    connect this node to swarm and try again.`. This error tells a node that is already in a Swarm
    to join a Swarm (:ref:`swarm-networking-swarm-scoped-networks`).

.. expectation:: Network and IPAM plugins on the leader
    :id: SWARM-SCOPED-PLUGIN-ON-LEADER
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    Required. A Swarm-scoped network whose driver or IPAM driver is a managed plugin can be created
    and allocated only while the leader manager has the plugin. After leadership moves to a manager
    without the plugin, the network still reports its allocation. But the tasks of services on it
    stay in the ``new`` state that :api-op:`GET /tasks` reports. Only the leader manager's log
    reports the failure (:ref:`swarm-networking-swarm-scoped-networks`).

.. expectation:: Plugin names without a tag
    :id: SWARM-SCOPED-PLUGIN-UNTAGGED-NAME
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    Inconsistent. A managed plugin is found by its name without the tag when the request is checked,
    but its driver is registered under the full name. Thus :api-op:`POST /networks/create` with a
    network driver named without the tag responds 500. An IPAM driver named without the tag is
    accepted, but the network is never allocated (:ref:`swarm-networking-swarm-scoped-networks`).

.. expectation:: Config-only networks for Swarm-scoped networks
    :id: SWARM-SCOPED-CONFIG-ONLY-ONE-NODE
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    Checked on one node. :api-op:`POST /networks/create` for a Swarm-scoped network with
    :api-field:`ConfigFrom` responds 201 only when the config-only network exists on the manager
    that handles the request. Each node that runs a task on the network uses its own config-only
    network of that name. Thus a manager that does not run tasks needs the config-only network only
    to allow the request. Task placement does not take into account which nodes have the config-only
    network. A task can be placed on a node that does not have it. That task is then rejected. On
    such a node, the attachment of a standalone container to the network fails after 20 seconds
    (:ref:`swarm-networking-swarm-scoped-networks`).

.. expectation:: Ingress networks without Ingress
    :id: SWARM-INGRESS-LABEL-PARTLY-SUPPORTED
    :observed_from: client
    :depends_on: none
    :platforms: linux, windows

    Partly supported. Older daemons created the ingress network as an overlay network named
    :network:`ingress` with the label :drvopt:`com.docker.swarm.internal`, without
    :api-field:`Ingress`. :api-op:`POST /networks/create` still accepts this form, and the network
    is treated as an ingress network. But a node sets up the routing mesh for it only while the node
    has a local attachment to it. A published port is not reachable at the address of any other
    node. When the network is deleted, the nodes keep their copies until the daemon restarts. A new
    ingress network then fails on those nodes when its subnet overlaps. The checks that
    :api-op:`POST /networks/create` makes for :api-field:`Ingress` do not apply to this form. Such a
    network can be :api-field:`Attachable`. It can also be created while another ingress network
    exists, and it is then never allocated (:ref:`swarm-networking-the-ingress-network`).

.. expectation:: Local and Swarm-scoped networks with the same name
    :id: SWARM-SCOPED-NAME-COLLISION-LOCAL
    :observed_from: api:wkr
    :depends_on: none
    :platforms: linux, windows

    Not prevented on every node. A node can have a network that is not Swarm-scoped and that has the
    name of a Swarm-scoped network. A worker can create such a network, and a worker that has one
    can become a manager. :api-op:`POST /networks/create` refuses the name only on the manager that
    handles the request (:ref:`swarm-networking-swarm-scoped-networks`). On a manager with
    both networks, a request that names the network by name is ambiguous. On any node with both
    networks, the Swarm-scoped network is not usable. Its tasks are rejected there. A standalone
    container that names it by its ID cannot attach to it. One that names it by its name is attached
    to the other network.

.. expectation:: --config-from on overlay networks
    :id: SWARM-SCOPED-CONFIG-FROM-OVERLAY-UNUSABLE
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    Not usable. :api-op:`POST /networks/create` for an overlay network with :api-field:`ConfigFrom`
    responds 201 only if the config-only network exists on the manager that handles the request. It
    responds 404 otherwise. Every attachment to the network then fails. The tasks of a service fail
    with :api-error:`user-specified configurations are not supported if the network depends on a
    configuration network`. :cli:`docker run` exits non-zero after 20 seconds and prints an error
    that contains :api-error:`attaching to network failed, make sure your network options are
    correct and check manager logs: context deadline exceeded`
    (:ref:`swarm-networking-swarm-scoped-networks`).

.. expectation:: Attachment failures for standalone containers time out
    :id: SWARM-SCOPED-ATTACH-FAILURE-TIMEOUT
    :observed_from: api:active
    :depends_on: none
    :platforms: linux

    If the attachment of a standalone container to an attachable network fails, :api-op:`POST
    /containers/{id}/start` waits 20 seconds. It then responds 500 with :api-error:`attaching to
    network failed, make sure your network options are correct and check manager logs: context
    deadline exceeded`. This occurs in seven cases: (1) The network is :drvopt:`encrypted`, and the
    host's kernel does not offer the XFRM netlink interface. (2) The network is an overlay network
    created with :api-field:`ConfigFrom`. (3) The network is deleted after the start has found it,
    but before the attachment task is created. (4) The container's static address is in use or is
    outside the network's subnets. (5) The network is not allocated. (6) The container's node has
    availability ``drain`` or ``pause``. (7) The network is a Swarm-scoped network of a node-local
    driver created with :api-field:`ConfigFrom`, and the container's node does not have the
    config-only network that :api-field:`ConfigFrom` names. :api-op:`POST /networks/{id}/connect`
    for a running container also waits 20 seconds in this case, and then responds 500 with the same
    message. In cases 1, 2 and 7, the reason appears in the attachment task only while the start
    waits. During that time, :api-op:`GET /tasks` with the :api-query:`runtime=attachment` filter
    lists the task as rejected. Then the task is deleted before the request responds. After that,
    the reason is only in the node's daemon log. In case 3, the attachment task is deleted, and no
    manager's daemon log reports the failure. In cases 4 and 5, the attachment task stays in the
    ``new`` state until it is deleted. :api-op:`GET /tasks` reports that state. The reason then
    appears only in the leader manager's log, in an entry that contains :log:`task allocation
    failure`. In case 6, the attachment task stays in the ``pending`` state until it is deleted.
    :api-op:`GET /tasks` reports that state (:ref:`swarm-networking-swarm-scoped-networks`,
    :ref:`swarm-networking-overlay-networks`, :ref:`swarm-lifecycle-encryption`,
    :ref:`swarm-lifecycle-churn`, :ref:`swarm-lifecycle-scale-and-exhaustion`).

.. expectation:: IPv6-only Swarm-scoped networks of node-local drivers
    :id: SWARM-SCOPED-NODE-LOCAL-V6-ONLY
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows
    :rationale: RAT-SWARM-SCOPED-NODE-LOCAL-V6-ONLY

    Refused. :api-op:`POST /networks/create` for any Swarm-scoped network with
    :api-field:`EnableIPv4` false responds 400 with :api-error:`IPv4 cannot be disabled in a Swarm
    scoped network`. This holds whatever the network's driver is. Only :driver:`overlay` needs the
    refusal (:ref:`swarm-networking-swarm-scoped-networks`,
    :ref:`swarm-networking-overlay-networks`).

.. expectation:: Subnets and options on Swarm-scoped networks of node-local drivers
    :id: SWARM-SCOPED-NODE-LOCAL-IPAM-OPTIONS
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    Ignored. :api-op:`POST /networks/create` accepts a valid :api-field:`IPAM.Config` and
    :api-field:`Options` for a Swarm-scoped network of a node-local driver, but they are not kept.
    Each node creates the network from its own default address pools and the driver's defaults. A
    :api-field:`ConfigFrom` that names a config-only network configures such a network on each node
    (:ref:`swarm-networking-swarm-scoped-networks`).

.. expectation:: Auxiliary addresses on overlay networks
    :id: SWARM-OVL-AUXILIARY-ADDRESSES-IGNORED
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    Ignored. :api-op:`POST /networks/create` accepts :api-field:`AuxiliaryAddresses` in the
    :api-field:`IPAM.Config` of an overlay network, but they are not kept. These addresses are not
    reserved, and tasks and VIPs can get them (:ref:`swarm-networking-overlay-networks`).

.. expectation:: mtu on Swarm networks is not checked at create
    :id: SWARM-OVL-MTU-UNCHECKED
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    Accepted. :api-op:`POST /networks/create` with any value of
    :drvopt:`com.docker.network.driver.mtu` responds 201. If the value is not a non-negative
    integer, a Linux node rejects all tasks on the network that are assigned to that node. Their
    errors contain :api-error:`failed to parse` or :api-error:`invalid MTU value`. If the kernel
    cannot create the network's interfaces with the value, every such task fails at start. Windows
    nodes ignore the option (:ref:`swarm-networking-overlay-networks`).

.. expectation:: ifname is not checked before a task starts
    :id: SWARM-SCOPED-IFACE-NAME-OPTION-UNCHECKED
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    Accepted. :api-op:`POST /services/create` and :api-op:`POST /services/{id}/update` accept any
    interface name in the driver options of a network attachment. Every task with an interface name
    that the OS refuses fails at start. Under the default restart policy, such a task is replaced
    indefinitely (:ref:`swarm-networking-swarm-scoped-networks`).

.. expectation:: Services on the host network with other networks
    :id: SWARM-SCOPED-HOST-NETWORK-COMBINED-ACCEPTED
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    Accepted. :api-op:`POST /services/create` for a service attached to the :network:`host` network
    and to an overlay network responds 201. :api-op:`POST /services/create` for a service on
    :network:`host` that publishes a port in ingress mode also responds 201. Every task of such a
    service fails at start. Under the default restart policy, such a task is replaced indefinitely
    (:ref:`swarm-networking-swarm-scoped-networks`).

.. _swarm-networking-deliberately-not-asserted:

***************************
 Deliberately not asserted
***************************

These items are incidental to the implementation. A change to any of them does not break any
expectation.

- The counters of Moby's own rules.
- Precedence over an operator's rule that the operator inserts at the top of the filter table's
  :ipt-chain:`INPUT` chain after an :drvopt:`encrypted` network is programmed. Such a rule can
  accept cleartext VXLAN for the network's VNI (:ref:`swarm-lifecycle-iptables`).
- The content of Moby's own chains and tables, and their names other than :ipt-chain:`DOCKER-USER`,
  except where :ref:`swarm-lifecycle-cross-generation-cleanup` pins them for cleanup and
  :ref:`bridge-networking-rule-content` pins the bridge driver's rules. The content includes
  individual match/target lines and their order in a chain. A backend change replaces exactly these
  rules. A contract asserts their effect on packets, and never the content.
- Which packet-filtering technology is in use, beyond what the
  :ref:`swarm-lifecycle-firewall-anchors-host-integration` expectations about anchors and cleanup
  assert for each backend.
- The existence, name or topology of internal sandboxes (per-network load-balancer sandbox, ingress
  sandbox), network-namespace paths, and generated :link-kind:`veth` / bridge / VXLAN interface
  names. One exception is the name ``ingress-sbox`` in :api-field:`Containers` of :api-op:`GET
  /networks/docker_gwbridge`, which :ref:`network-common-network-api` pins. Another exception is a
  node's load-balancer endpoint on a network. :api-op:`GET /networks/{id}` on that node lists this
  endpoint. :ref:`swarm-networking-overlay-networks` asserts where its address falls, and
  :ref:`swarm-traffic-load-balancing-east-west-vip` asserts that a task observes it as the source
  address in NAT mode. The key and the name of this endpoint are not asserted.
  :ref:`network-common-daemon-restart` asserts that the namespaces of these sandboxes are removed
  after a kill and restart.
- The load-balancer implementation and its scheduler, beyond the distribution property in
  :ref:`swarm-traffic-distribution`.
- Internal refcounting, gossip timers, and reap and rejoin intervals. The surface for these is the
  convergence bounds in :ref:`swarm-lifecycle-convergence`.
- The order in which the daemon allocates subnets from an address pool, addresses within a subnet,
  published ports from the auto-assignment range, and VNIs to networks created without
  :drvopt:`vxlanid_list`.
- The value of :api-field:`LocalLBIndex`. :cli:`docker network inspect --verbose` shows this value,
  but the value is an internal firewall mark.
- The host-port reservation mechanics behind :ref:`swarm-traffic-port-allocation`.
- Whether Windows nodes can use a VNI below 4096.
