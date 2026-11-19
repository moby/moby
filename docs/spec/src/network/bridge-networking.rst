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
gateway mode for that address's family is not ``routed``, and without
:drvopt:`com.docker.network.bridge.host_binding_ipv4`, unless the expectation names them. An
expectation about IPv4 traffic through the network's gateway, such as egress, a published port or a
default route, holds for a network without :drvopt:`com.docker.network.bridge.inhibit_ipv4`, unless
the expectation names it. :doc:`swarm-networking` covers what is specific to Swarm-scoped bridge
networks (:ref:`swarm-networking-swarm-scoped-networks`) and to :network:`docker_gwbridge`
(:ref:`swarm-traffic-gateway-network`).

This spec is a stub. It holds expectations that belong to the bridge driver. They come from the work
on the :doc:`swarm-networking` and from reviews of the specs' coverage. This spec is not yet a
complete account of the bridge driver. :doc:`../README.md <../README>` gives the model that this
spec follows: the items of a spec, their categories and what their verdicts depend on.
:doc:`README.md <README>` gives the conventions of the networking specs, such as vantage points,
daemon settings, timing and convergence ceilings, and host requirements. :doc:`network-common.md
<network-common>` covers what every network driver must do, such as naming an endpoint's interface.

.. _bridge-networking-rationales:

************
 Rationales
************

.. rationale:: Why a change to BR-OPT-DAEMON-MTU-ERROR should be deliberate
    :id: RAT-BR-OPT-DAEMON-MTU-ERROR

    It catches a change to the error that a daemon-wide mtu the kernel refuses gives at start;
    operators and tools match it, so it should be deliberate.

.. rationale:: Why a change to BR-FW-FIREWALLD-RELOAD-NFT should be deliberate
    :id: RAT-BR-RELOAD-NFT-WARNINGS

    It catches a change to the warnings logged on a firewalld reload, start or stop under the
    nftables backend; operators' log alerts see them, so a change should be deliberate.

.. rationale:: Why a change to BR-OPT-MTU-OUT-OF-RANGE should be deliberate
    :id: RAT-BR-OPT-MTU-OUT-OF-RANGE

    It catches a change to the error message that network create gives for an MTU that the kernel
    refuses, such as a message that names the option; clients and scripts match the message, so the
    change should be deliberate.

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
    :kernel_options: NF_TABLES, NF_TABLES_IPV4, NF_TABLES_IPV6, NFT_NAT, NFT_MASQ, NFT_CT, NFT_FIB_IPV4, NFT_FIB_IPV6, NFT_CHAIN_NAT_IPV4, NFT_CHAIN_NAT_IPV6, NFT_MASQ_IPV4, NFT_MASQ_IPV6
    :when: nftables-backend

    *Facility:* nftables with the NAT, masquerade, connection-tracking and FIB expressions

    *Needed when:* The nftables backend

    *Kernel options:* :kconfig:`NF_TABLES`, :kconfig:`NF_TABLES_IPV4`, :kconfig:`NF_TABLES_IPV6`,
    :kconfig:`NFT_NAT`, :kconfig:`NFT_MASQ`, :kconfig:`NFT_CT`, :kconfig:`NFT_FIB_IPV4`,
    :kconfig:`NFT_FIB_IPV6`. :kconfig:`NFT_CHAIN_NAT_IPV4`, :kconfig:`NFT_CHAIN_NAT_IPV6`,
    :kconfig:`NFT_MASQ_IPV4` and :kconfig:`NFT_MASQ_IPV6` on kernels that have them.

.. needextend:: type == "expectation" and docname in ['network/bridge-networking'] and ("linux" in platforms) and "BR-HOST-NFTABLES" not in pins
    :+needs: BR-HOST-NFTABLES

.. hostreq:: Netfilter hooks for bridged traffic
    :id: BR-HOST-BR-NETFILTER
    :kind: os
    :platforms: linux
    :kernel_options: BRIDGE_NETFILTER
    :when: iptables-backend, userland-proxy

    *Facility:* Netfilter hooks for bridged traffic (``br_netfilter``)

    *Needed when:* The daemon option :daemon-json:`iptables` true, and either a network that does
    not allow inter-container communication or the daemon option :daemon-json:`userland-proxy` false

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
    ``addrtype``, ``conntrack`` and ``sctp`` matches

    *Needed when:* The iptables backend, with the daemon option :daemon-json:`iptables` true

.. needextend:: type == "expectation" and docname in ['network/bridge-networking'] and ("linux" in platforms) and "BR-HOST-IPTABLES-COMMAND" not in pins
    :+needs: BR-HOST-IPTABLES-COMMAND

.. hostreq:: ip6tables with the same tables, targets and matches
    :id: BR-HOST-IP6TABLES-COMMAND
    :kind: program
    :platforms: linux
    :when: iptables-backend, ip6tables

    *Facility:* :program:`ip6tables`, accepting the same tables, targets and matches

    *Needed when:* The iptables backend, with the daemon option :daemon-json:`ip6tables` true

.. hostreq:: The nft command
    :id: BR-HOST-NFT-COMMAND
    :kind: program
    :platforms: linux
    :when: nftables-backend

    *Facility:* :program:`nft`

    *Needed when:* The nftables backend

.. needextend:: type == "expectation" and docname in ['network/bridge-networking'] and ("linux" in platforms) and "BR-HOST-NFT-COMMAND" not in pins
    :+needs: BR-HOST-NFT-COMMAND

The daemon does not depend on which implementation of :program:`iptables` and :program:`ip6tables`
the host has. The kernel options that the programs need depend on that implementation:

.. hostreq:: Kernel options for iptables
    :id: BR-HOST-IPTABLES-KERNEL
    :kind: os
    :platforms: linux
    :kernel_options: NETFILTER, NETFILTER_ADVANCED, NETFILTER_XTABLES, NF_CONNTRACK, NF_NAT, NETFILTER_XT_NAT, NETFILTER_XT_TARGET_MASQUERADE, NETFILTER_XT_MATCH_ADDRTYPE, NETFILTER_XT_MATCH_CONNTRACK, NETFILTER_XT_MATCH_SCTP, IP_NF_NAT, IP_NF_TARGET_MASQUERADE
    :when: iptables-backend, iptables

    The kernel options that :program:`iptables` needs, whichever implementation provides it. *Kernel
    options:* :kconfig:`NETFILTER`, :kconfig:`NETFILTER_ADVANCED`, :kconfig:`NETFILTER_XTABLES`,
    :kconfig:`NF_CONNTRACK`, :kconfig:`NF_NAT`, :kconfig:`NETFILTER_XT_NAT`,
    :kconfig:`NETFILTER_XT_TARGET_MASQUERADE`, :kconfig:`NETFILTER_XT_MATCH_ADDRTYPE`,
    :kconfig:`NETFILTER_XT_MATCH_CONNTRACK`, :kconfig:`NETFILTER_XT_MATCH_SCTP`. Before Linux 5.2,
    :kconfig:`IP_NF_NAT` and :kconfig:`IP_NF_TARGET_MASQUERADE` in place of
    :kconfig:`NETFILTER_XT_TARGET_MASQUERADE`.

.. needextend:: type == "expectation" and docname in ['network/bridge-networking'] and ("linux" in platforms) and "BR-HOST-IPTABLES-KERNEL" not in pins
    :+needs: BR-HOST-IPTABLES-KERNEL

.. hostreq:: Kernel options for iptables from iptables-legacy
    :id: BR-HOST-IPTABLES-LEGACY-KERNEL
    :kind: os
    :platforms: linux
    :kernel_options: IP_NF_IPTABLES, IP_NF_FILTER, IP_NF_NAT, IP_NF_RAW, IP_NF_IPTABLES_LEGACY, NETFILTER_XTABLES_LEGACY
    :when: iptables-backend, iptables, iptables-legacy

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
    :kernel_options: IPV6, IP6_NF_NAT, IP6_NF_TARGET_MASQUERADE, NETFILTER_XT_TARGET_MASQUERADE, NETFILTER, NETFILTER_ADVANCED, NETFILTER_XTABLES, NF_CONNTRACK, NF_NAT, NETFILTER_XT_NAT, NETFILTER_XT_MATCH_ADDRTYPE, NETFILTER_XT_MATCH_CONNTRACK, NETFILTER_XT_MATCH_SCTP, IP_NF_NAT, IP_NF_TARGET_MASQUERADE
    :when: iptables-backend, ip6tables

    The kernel options that :program:`ip6tables` needs, whichever implementation provides it.
    *Kernel options:* :kconfig:`IPV6`, and the options that iptables needs, whichever implementation
    provides it. Before Linux 5.2, :kconfig:`IP6_NF_NAT` and :kconfig:`IP6_NF_TARGET_MASQUERADE` in
    place of :kconfig:`NETFILTER_XT_TARGET_MASQUERADE`.

.. hostreq:: Kernel options for ip6tables from iptables-legacy
    :id: BR-HOST-IP6TABLES-LEGACY-KERNEL
    :kind: os
    :platforms: linux
    :kernel_options: IP6_NF_IPTABLES, IP6_NF_FILTER, IP6_NF_NAT, IP6_NF_RAW, IP6_NF_IPTABLES_LEGACY, NETFILTER_XTABLES_LEGACY
    :when: iptables-backend, ip6tables, iptables-legacy

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

    The kernel options that :program:`ip6tables` needs when iptables-nft provides it. *Kernel
    options:* :kconfig:`NF_TABLES`, :kconfig:`NF_TABLES_IPV6`, :kconfig:`NFT_COMPAT`,
    :kconfig:`NFT_NAT`. Before Linux 5.1, :kconfig:`NFT_CHAIN_NAT_IPV6` in place of
    :kconfig:`NFT_NAT`.

.. _bridge-networking-host-configuration:

Host configuration
==================

.. hostreq:: IPv4 forwarding enabled (net.ipv4.ip_forward)
    :id: BR-HOST-FORWARDING-V4
    :kind: config
    :platforms: linux
    :when: nftables-backend, ip-forward

    *Facility:* :sysctl:`net.ipv4.ip_forward` is 1

    *Needed when:* The nftables backend, with the daemon option :daemon-json:`ip-forward` true

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

    *Needed when:* The nftables backend

.. _bridge-networking-firewall-anchors-host-integration:

*************************************
 Firewall anchors & host integration
*************************************

The content of Moby's firewall rules is not an interface. A backend change replaces exactly this
content. :ref:`bridge-networking-rule-content` pins it with detectors only. Two things are
interfaces. They are the anchors of this section:

1. The :ipt-chain:`DOCKER-USER` iptables chain. Operators attach rules to it. The documentation
   names it as the place to put policy that survives daemon restarts. The daemon never deletes or
   modifies its contents.
2. The hooks and priorities of the bridge driver's nftables base chains. They decide the evaluation
   order relative to everything else on the host, such as firewalld, kube-proxy or an operator's own
   ruleset. A changed priority silently changes who sees a packet first.

No other name is an anchor. Moby's other iptables chains are not anchors, and no name in nftables is
an anchor. An operator's rules can reach Moby's nftables chains only if they are programmed into
Moby's own tables. That is not a supported configuration.

IPv4 is an enabled IP version while the daemon's :program:`iptables` option is true. IPv6 is an
enabled IP version while the daemon's :program:`ip6tables` option is true. This applies under either
firewall backend, and whether or not :daemon-json:`ipv6` is set.

.. _bridge-networking-base-chain-priorities:

Base-chain priorities
=====================

These expectations cover the host's network namespace only. The specs do not assert the rules that
the daemon programs in a container's network namespace
(:ref:`container-dns-deliberately-not-asserted`).

.. expectation:: Each docker-bridges table has exactly five base chains at set priorities
    :id: BR-PRIO-BASE-CHAINS
    :observed_from: node
    :depends_on: none
    :platforms: linux
    :needs: BR-HOST-NFTABLES, BR-HOST-FORWARDING-V4

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

    For each enabled IP version, :ipt-chain:`DOCKER-USER` exists in the filter table and
    :ipt-chain:`FORWARD` jumps to it.

.. expectation:: An operator's DROP rule in DOCKER-USER drops traffic to a published port
    :id: BR-USER-DROP-RULE
    :observed_from: client
    :depends_on: os-linux
    :platforms: linux

    A DROP rule that an operator places in :ipt-chain:`DOCKER-USER` drops matching traffic to a
    published port. This is also true for a rule that an operator places after a firewalld reload.

.. expectation:: An operator's DOCKER-USER rules stay unchanged across a daemon start
    :id: BR-USER-RULES-KEPT
    :observed_from: node, client
    :depends_on: none
    :platforms: linux

    :ipt-chain:`DOCKER-USER` rules that an operator added before a start of the daemon are still
    there, unchanged, after the start. This includes a restart with :program:`iptables` false.

.. expectation:: After a firewalld reload, DOCKER-USER exists and FORWARD jumps to it
    :id: BR-USER-CHAIN-JUMP-FIREWALLD
    :observed_from: node
    :depends_on: os-linux
    :platforms: linux

    After a firewalld reload, for each enabled IP version, :ipt-chain:`DOCKER-USER` exists and
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
with :daemon-json:`live-restore` true.

A daemon of one release does not show a rename of a previous release's names. The detector that pins
the names that cleanup needs fails on such a rename.

.. expectation:: No table or chain of the other backend remains after the daemon starts
    :id: BR-CLEAN-OTHER-BACKEND
    :observed_from: node
    :depends_on: os-linux, upgrade
    :platforms: linux
    :needs: BR-HOST-NFTABLES, BR-HOST-FORWARDING-V4

    After the daemon has started, the host netns does not contain any table or chain that a previous
    generation left there with the other backend. The exceptions are :ipt-chain:`DOCKER-USER` and
    the :ipt-chain:`FORWARD` jump to it, which hold the operator's rules and stay in place. The
    tables and built-in chains that an :program:`iptables` command creates by itself, such as the
    filter table and its :ipt-chain:`FORWARD` chain, are also exceptions. The overlay driver's
    :nft-table:`docker-overlay` table is also an exception. It is not asserted
    (:ref:`swarm-lifecycle-nftables`). This holds for a switch from iptables to nftables and for a
    switch from nftables to iptables. It holds across a package upgrade from *N−1* to *N* and across
    a same-binary restart.

.. expectation:: Neither the old DOCKER-ISOLATION chain nor its FORWARD rule remains
    :id: BR-CLEAN-ISOLATION-CHAIN
    :observed_from: node
    :depends_on: os-linux
    :platforms: linux

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
    :needs: BR-HOST-NFTABLES, BR-HOST-FORWARDING-V4

    *Applies to:* A rule that the previous generation left in a built-in chain for a network,
    endpoint or port binding

    After a switch from iptables to nftables, the rule does not remain after the daemon has restored
    that network, endpoint or port binding. This holds across a package upgrade from the previous
    release and across a same-binary restart. See
    :ref:`bridge-networking-known-divergences-non-goals`.

.. expectation:: No docker-bridges table remains after a start with iptables
    :id: BR-CLEAN-OTHER-BACKEND-IPT
    :observed_from: node
    :depends_on: os-linux
    :platforms: linux

    When the daemon starts with the iptables backend, no :nft-table:`docker-bridges` table remains
    after the start.

.. expectation:: No Docker iptables chain or jump remains after a start with nftables
    :id: BR-CLEAN-OTHER-BACKEND-NFT
    :observed_from: node
    :depends_on: os-linux
    :platforms: linux
    :needs: BR-HOST-NFTABLES, BR-HOST-FORWARDING-V4

    *Applies to:* A jump from the filter table's :ipt-chain:`FORWARD` chain to
    :ipt-chain:`DOCKER-FORWARD` or to :ipt-chain:`DOCKER-ISOLATION-STAGE-1`

    After the daemon starts with the nftables backend, some chains do not remain. These chains are
    :ipt-chain:`DOCKER` (in the filter and nat tables), :ipt-chain:`DOCKER-FORWARD`,
    :ipt-chain:`DOCKER-BRIDGE`, :ipt-chain:`DOCKER-CT`, :ipt-chain:`DOCKER-INTERNAL`,
    :ipt-chain:`DOCKER-ISOLATION-STAGE-1`, :ipt-chain:`DOCKER-ISOLATION-STAGE-2` and
    :ipt-chain:`DOCKER-ISOLATION`. The jumps to these chains from :ipt-chain:`FORWARD` and from the
    nat table's :ipt-chain:`PREROUTING` and :ipt-chain:`OUTPUT` do not remain either.

.. expectation:: Old built-in rules remain until the daemon restores what they are for
    :id: BR-CLEAN-BUILTIN-RULES-UNTIL-RESTORED
    :observed_from: node
    :depends_on: os-linux
    :platforms: linux
    :needs: BR-HOST-NFTABLES, BR-HOST-FORWARDING-V4

    *Applies to:* The rules of the previous generation in built-in chains for a network, endpoint or
    port binding

    After a switch from iptables to nftables, these rules remain until that network, endpoint or
    port binding is restored. They do not remain after that.

.. expectation:: The table and chain names that cleanup needs are unchanged
    :id: BR-CLEAN-NAMES-UNCHANGED
    :observed_from: node
    :depends_on: none
    :platforms: linux

    The names that cleanup needs are unchanged. They are the nftables table
    :nft-table:`docker-bridges`, and the iptables chains :ipt-chain:`DOCKER` (in the filter and nat
    tables), :ipt-chain:`DOCKER-FORWARD`, :ipt-chain:`DOCKER-BRIDGE`, :ipt-chain:`DOCKER-CT`,
    :ipt-chain:`DOCKER-INTERNAL`, :ipt-chain:`DOCKER-ISOLATION-STAGE-1`,
    :ipt-chain:`DOCKER-ISOLATION-STAGE-2` and :ipt-chain:`DOCKER-ISOLATION`.

.. expectation:: A start with nftables does not modify the FORWARD chain's policy
    :id: BR-CLEAN-FORWARD-POLICY-KEPT
    :observed_from: node
    :depends_on: none
    :platforms: linux
    :pins: BR-HOST-NO-FORWARD-DROP
    :needs: BR-HOST-NFTABLES, BR-HOST-FORWARDING-V4

    The policy of the iptables filter table's :ipt-chain:`FORWARD` chain is not modified when the
    daemon starts with the nftables backend. This includes a policy of DROP.

.. expectation:: A FORWARD policy of DROP logs a warning on the first start with nftables
    :id: BR-CLEAN-FORWARD-DROP-WARNING
    :observed_from: log
    :depends_on: none
    :platforms: linux
    :pins: BR-HOST-NO-FORWARD-DROP
    :needs: BR-HOST-NFTABLES, BR-HOST-FORWARDING-V4

    *Applies to:* The filter table's :ipt-chain:`FORWARD` chain with policy DROP

    On the first start after a switch from iptables to nftables, the daemon logs a warning that
    contains :log:`Network traffic for published ports may be dropped, iptables chain FORWARD has
    policy DROP.`

.. expectation:: After a switch to nftables, a DOCKER-USER DROP rule still drops traffic
    :id: BR-CLEAN-USER-DROP-RULE
    :observed_from: client
    :depends_on: os-linux
    :platforms: linux
    :pins: BR-HOST-NO-FORWARD-DROP
    :needs: BR-HOST-NFTABLES, BR-HOST-FORWARDING-V4

    After a switch from iptables to nftables, a DROP rule that an operator placed in
    :ipt-chain:`DOCKER-USER` still drops the forwarded traffic that it matches. This includes
    traffic to published ports.

.. _bridge-networking-firewall-backend-and-firewalld:

Firewall backend and firewalld
==============================

.. expectation:: The daemon does not start with an unknown firewall-backend
    :id: BR-FW-BACKEND-INVALID
    :observed_from: log
    :depends_on: none
    :platforms: linux

    *Applies to:* A daemon configured with a :daemon-json:`firewall-backend` other than
    :program:`iptables` or ``nftables``

    The daemon does not start. The daemon log contains an error.

.. expectation:: An unknown firewall-backend is logged as invalid firewall-backend
    :id: BR-FW-BACKEND-INVALID-MSG
    :observed_from: log
    :depends_on: none
    :platforms: linux

    *Applies to:* A daemon configured with a :daemon-json:`firewall-backend` other than
    :program:`iptables` or ``nftables``

    The daemon does not start. The daemon log contains an error that contains :log:`invalid
    firewall-backend`.

.. expectation:: A published port answers after a firewalld reload
    :id: BR-FW-FIREWALLD-RELOAD-PORT
    :observed_from: client
    :depends_on: os-linux
    :platforms: linux

    *Applies to:* A node with firewalld running

    A published port answers after a firewalld reload.

.. expectation:: After a firewalld reload, GET /info reports a ReloadedAt time
    :id: BR-FW-FIREWALLD-RELOADED-AT
    :observed_from: api
    :depends_on: os-linux
    :platforms: linux

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

    *Applies to:* A node with firewalld running, and a running container on a bridge network that is
    not ``internal``, whose :drvopt:`com.docker.network.bridge.gateway_mode_ipv4` is not ``routed``,
    and whose :drvopt:`com.docker.network.bridge.enable_ip_masquerade` is not ``false``

    After a firewalld reload, a connection from the container to an IPv4 address of an underlay host
    succeeds.

.. expectation:: An IPv4 bridge network turns on ip_forward and sets FORWARD to DROP
    :id: BR-FW-FORWARDING-ENABLED
    :observed_from: node
    :depends_on: os-linux
    :platforms: linux
    :pins: BR-HOST-FORWARDING-V4

    *Applies to:* The iptables backend, with :daemon-json:`ip-forward` and :daemon-json:`iptables`
    true and :daemon-json:`ip-forward-no-drop` false, and a host where :sysctl:`net.ipv4.ip_forward`
    is 0

    After the daemon has created a bridge network with IPv4, :sysctl:`net.ipv4.ip_forward` is 1. The
    filter table's :ipt-chain:`FORWARD` chain also has policy DROP.

.. expectation:: An IPv6 bridge network turns on IPv6 forwarding and sets FORWARD to DROP
    :id: BR-FW-FORWARDING-ENABLED-V6
    :observed_from: node
    :depends_on: os-linux
    :platforms: linux
    :pins: BR-HOST-FORWARDING-V6

    *Applies to:* The iptables backend, with :daemon-json:`ip-forward` and :daemon-json:`ip6tables`
    true and :daemon-json:`ip-forward-no-drop` false, and a host where
    :sysctl:`net.ipv6.conf.all.forwarding` or :sysctl:`net.ipv6.conf.default.forwarding` is 0

    After the daemon has created a bridge network with IPv6, both are 1. The ip6tables filter
    table's :ipt-chain:`FORWARD` chain also has policy DROP.

.. expectation:: The FORWARD chain has policy DROP after a firewalld reload
    :id: BR-FW-FORWARD-DROP-FIREWALLD
    :observed_from: node
    :depends_on: os-linux
    :platforms: linux
    :pins: BR-HOST-FORWARDING-V4

    *Applies to:* The iptables backend, with :daemon-json:`ip-forward` and :daemon-json:`iptables`
    true and :daemon-json:`ip-forward-no-drop` false, firewalld running, and a bridge network with
    IPv4 that the daemon created on a host where :sysctl:`net.ipv4.ip_forward` was 0

    After a firewalld reload, the filter table's :ipt-chain:`FORWARD` chain has policy DROP.

.. expectation:: The ip6tables FORWARD chain has policy DROP after a firewalld reload
    :id: BR-FW-FORWARD-DROP-FIREWALLD-V6
    :observed_from: node
    :depends_on: os-linux
    :platforms: linux
    :pins: BR-HOST-FORWARDING-V6

    *Applies to:* The iptables backend, with :daemon-json:`ip-forward` and :daemon-json:`ip6tables`
    true and :daemon-json:`ip-forward-no-drop` false, firewalld running, and a bridge network with
    IPv6 that the daemon created on a host where :sysctl:`net.ipv6.conf.all.forwarding` or
    :sysctl:`net.ipv6.conf.default.forwarding` was 0

    After a firewalld reload, the ip6tables filter table's :ipt-chain:`FORWARD` chain has policy
    DROP.

.. expectation:: With nftables, the daemon does not start while ip_forward is 0
    :id: BR-FW-NFT-NO-FORWARDING
    :observed_from: log
    :depends_on: none
    :platforms: linux
    :pins: BR-HOST-FORWARDING-V4
    :needs: BR-HOST-NFTABLES

    *Applies to:* A daemon with the nftables backend, :daemon-json:`ip-forward` true and the default
    bridge enabled, on a host where :sysctl:`net.ipv4.ip_forward` is 0, and without containers that
    :daemon-json:`live-restore` keeps running

    The daemon does not start. It prints an error that contains :log:`IPv4 forwarding is disabled`.

.. expectation:: With nftables, an IPv6 network responds 500 while forwarding is off
    :id: BR-FW-NFT-NO-FORWARDING-V6
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :pins: BR-HOST-FORWARDING-V6
    :needs: BR-HOST-NFTABLES, BR-HOST-FORWARDING-V4

    *Applies to:* A daemon with the nftables backend and :daemon-json:`ip-forward` true, on a host
    where :sysctl:`net.ipv6.conf.all.forwarding` or :sysctl:`net.ipv6.conf.default.forwarding` is 0

    :api-op:`POST /networks/create` for a bridge network with :api-field:`EnableIPv6` set responds
    500, with a message that contains :api-error:`IPv6 global forwarding is disabled`. See
    :ref:`Host requirements <bridge-networking-host-requirements>`.

.. expectation:: Without the iptables command, the daemon does not start
    :id: BR-FW-IPTABLES-MISSING
    :observed_from: log
    :depends_on: none
    :platforms: linux
    :pins: BR-HOST-IPTABLES-COMMAND, DNS-HOST-IPTABLES-COMMAND, SWARM-HOST-IPTABLES-COMMAND

    *Applies to:* A daemon with the iptables backend and :daemon-json:`iptables` true, on a host
    without the :program:`iptables` command

    The daemon does not start. The daemon log contains :log:`iptables not found`. See :ref:`Host
    requirements <bridge-networking-host-requirements>`.

.. expectation:: With no ip6tables command, the daemon starts but cannot set up chains
    :id: BR-FW-IP6TABLES-MISSING
    :observed_from: log
    :depends_on: none
    :platforms: linux
    :pins: BR-HOST-IP6TABLES-COMMAND

    *Applies to:* A daemon with the iptables backend and :daemon-json:`ip6tables` true, on a host
    without the :program:`ip6tables` command

    The daemon starts. The daemon log contains :log:`ip6tables is enabled, but cannot set up
    ip6tables chains`. See :ref:`Host requirements <bridge-networking-host-requirements>`.

.. expectation:: Without the ip6tables command, an IPv6 bridge network is refused
    :id: BR-FW-IP6TABLES-MISSING-NETWORK
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :pins: BR-HOST-IP6TABLES-COMMAND

    *Applies to:* A daemon with the iptables backend and :daemon-json:`ip6tables` true, on a host
    without the :program:`ip6tables` command

    :api-op:`POST /networks/create` for a bridge network with :api-field:`EnableIPv6` set responds
    with a 4xx or 5xx status. The network is not created.

.. expectation:: Without the nft command, the daemon does not start with nftables
    :id: BR-FW-NFT-MISSING
    :observed_from: log
    :depends_on: none
    :platforms: linux
    :pins: BR-HOST-NFT-COMMAND, DNS-HOST-NFT-COMMAND

    *Applies to:* A daemon with the nftables backend, on a host without the :program:`nft` command

    The daemon does not start. The daemon log contains :log:`failed to find nft tool`. See
    :ref:`Host requirements <bridge-networking-host-requirements>`.

.. _bridge-networking-rule-content:

Rule content
============

The expectations in this subsection pin the content of the bridge driver's rules, so that a change
to it is visible in review. A deliberate change to this content is not a breakage.

.. expectation:: The bridge driver's IPv4 iptables rules match the reference rulesets
    :id: BR-RULES-REFERENCE
    :observed_from: node
    :depends_on: none
    :platforms: linux

    For each configuration that the repository keeps a reference iptables ruleset for in
    :repo-path:`integration/network/bridge/iptablesdoc`, the bridge driver's IPv4 iptables rules
    match that reference. The reference gives the matches and target of each rule, the order of the
    rules in each chain, and the names of the chains. The configurations include a Swarm service
    that publishes a port.

.. expectation:: The bridge driver's IPv4 nftables ruleset matches the reference
    :id: BR-RULES-REFERENCE-NFT
    :observed_from: node
    :depends_on: none
    :platforms: linux
    :needs: BR-HOST-NFTABLES, BR-HOST-FORWARDING-V4

    For each configuration that the repository keeps a reference nftables ruleset for in
    :repo-path:`integration/network/bridge/nftablesdoc`, the bridge driver's IPv4 nftables ruleset
    matches that reference. The reference gives the matches and verdict of each rule, the order of
    the rules in each chain, and the table and chain names. The configurations include a Swarm
    service that publishes a port.

.. _bridge-networking-network-and-endpoint-options:

******************************
 Network and endpoint options
******************************

.. expectation:: A network MTU that is not an integer in range responds 400
    :id: BR-OPT-MTU-INVALID
    :observed_from: api
    :depends_on: none
    :platforms: linux

    :api-op:`POST /networks/create` for a bridge network whose
    :drvopt:`com.docker.network.driver.mtu` is not an integer from 0 to 2⁶³ − 1 responds 400. The
    message includes the value.

.. expectation:: docker network create with an invalid MTU exits non-zero
    :id: BR-OPT-MTU-INVALID-CLI
    :observed_from: api
    :depends_on: cli
    :platforms: linux
    :cli_for: BR-OPT-MTU-INVALID

    :cli:`docker network create --driver bridge` with a :drvopt:`com.docker.network.driver.mtu`
    option that is not an integer from 0 to 2⁶³ − 1 exits non-zero. It prints an error that includes
    the value.

.. expectation:: A network with an MTU that the kernel refuses is not created
    :id: BR-OPT-MTU-REFUSED
    :observed_from: api
    :depends_on: none
    :platforms: linux

    :api-op:`POST /networks/create` for a bridge network with an MTU that the kernel refuses
    responds with a 4xx or 5xx status. :api-op:`GET /networks` then does not list the network.

.. expectation:: A network MTU that the kernel refuses responds 500
    :id: BR-OPT-MTU-REFUSED-HTTP500
    :observed_from: api
    :depends_on: none
    :platforms: linux

    :api-op:`POST /networks/create` for a bridge network with an MTU that the kernel refuses
    responds 500. See :ref:`bridge-networking-known-divergences-non-goals`.

.. expectation:: docker network create exits non-zero for an MTU the kernel refuses
    :id: BR-OPT-MTU-REFUSED-CLI
    :observed_from: api
    :depends_on: cli
    :platforms: linux
    :cli_for: BR-OPT-MTU-REFUSED

    :cli:`docker network create --driver bridge` with an MTU that the kernel refuses exits non-zero.
    :cli:`docker network ls` then does not list the network.

.. expectation:: A network's positive MTU applies to its containers' interfaces
    :id: BR-OPT-MTU-APPLIED
    :observed_from: ctr
    :depends_on: os-linux
    :platforms: linux

    *Applies to:* A bridge network whose :drvopt:`com.docker.network.driver.mtu` is positive

    The interfaces of the network's containers have that MTU. See
    :ref:`bridge-networking-known-divergences-non-goals`.

.. expectation:: A network MTU of 0 leaves the MTU that the kernel set
    :id: BR-OPT-MTU-ZERO
    :observed_from: node, ctr
    :depends_on: none
    :platforms: linux

    A bridge network's :drvopt:`com.docker.network.driver.mtu` of 0 leaves the MTU of the bridge and
    of its containers' interfaces as the kernel set it.

.. expectation:: The daemon does not start with a negative mtu
    :id: BR-OPT-DAEMON-MTU-NEGATIVE
    :observed_from: log
    :depends_on: none
    :platforms: linux

    *Applies to:* A daemon configured with a negative :daemon-json:`mtu`

    The daemon does not start. The daemon log contains an error.

.. expectation:: A negative daemon mtu is logged as invalid default MTU
    :id: BR-OPT-DAEMON-MTU-NEGATIVE-MSG
    :observed_from: log
    :depends_on: none
    :platforms: linux

    *Applies to:* A daemon configured with a negative :daemon-json:`mtu`

    The daemon does not start. The daemon log contains an error that contains :log:`invalid default
    MTU`.

.. expectation:: The daemon does not start with an mtu the kernel cannot apply
    :id: BR-OPT-DAEMON-MTU-REFUSED
    :observed_from: log
    :depends_on: none
    :platforms: linux

    *Applies to:* A daemon configured with an :daemon-json:`mtu` that the kernel cannot apply to the
    default bridge, and without containers that :daemon-json:`live-restore` keeps running

    The daemon does not start. See :ref:`bridge-networking-known-divergences-non-goals`.

.. expectation:: With live-restored containers, a refused mtu leaves the bridge MTU as is
    :id: BR-OPT-DAEMON-MTU-REFUSED-LIVE-RESTORE
    :observed_from: log, node
    :depends_on: none
    :platforms: linux

    *Applies to:* A daemon configured with an :daemon-json:`mtu` that the kernel cannot apply to the
    default bridge, with containers that :daemon-json:`live-restore` keeps running

    The daemon starts. The default bridge keeps its MTU.

.. expectation:: A positive daemon mtu applies to containers on the default bridge
    :id: BR-OPT-DAEMON-MTU-APPLIED
    :observed_from: ctr
    :depends_on: os-linux
    :platforms: linux

    *Applies to:* A daemon configured with a positive :daemon-json:`mtu` that the kernel can apply
    to the default bridge

    The interfaces of containers on the default bridge have that MTU. See
    :ref:`bridge-networking-known-divergences-non-goals`.

.. expectation:: With --mtu 0, the default bridge keeps the MTU that the kernel gave it
    :id: BR-OPT-DAEMON-MTU-ZERO
    :observed_from: node, ctr
    :depends_on: none
    :platforms: linux

    *Applies to:* A daemon started with :dockerd-flag:`--mtu 0`

    The default bridge and its containers' interfaces keep the MTU that the kernel gave them. See
    :ref:`bridge-networking-known-divergences-non-goals`.

.. expectation:: A daemon mtu of 2³² or more sets its remainder modulo 2³² as the MTU
    :id: BR-OPT-DAEMON-MTU-MODULO
    :observed_from: log, node
    :depends_on: none
    :platforms: linux

    *Applies to:* A daemon configured with an :daemon-json:`mtu` from 2³² to 2⁶³ − 1 whose remainder
    modulo 2³² the kernel accepts

    The daemon starts. The default bridge gets that remainder as its MTU. For example, 4294968796
    gives an MTU of 1500. See :ref:`bridge-networking-known-divergences-non-goals`.

.. _bridge-networking-data-path:

***********
 Data path
***********

.. _bridge-networking-isolation:

Isolation
=========

The expectations in this subsection hold with :daemon-json:`userland-proxy` true and with
:daemon-json:`userland-proxy` false, unless an expectation names a value.

.. expectation:: Containers on separate networks are isolated
    :id: BR-ISO-SEPARATE-NETWORKS
    :observed_from: ctr → ctr
    :depends_on: os-linux
    :platforms: linux

    *Applies to:* A daemon with :daemon-json:`allow-direct-routing` false, and a target network
    whose :drvopt:`com.docker.network.bridge.gateway_mode_ipv4` is ``nat`` or not set

    A container on one bridge network cannot reach a container on another bridge network at that
    container's IPv4 address, on any port. It reaches the published ports of that container at the
    node's address.

.. expectation:: A network without ICC isolates its containers
    :id: BR-ISO-NO-ICC
    :observed_from: ctr → ctr
    :depends_on: os-linux
    :platforms: linux
    :needs: BR-HOST-BR-NETFILTER

    *Applies to:* A bridge network with :drvopt:`com.docker.network.bridge.enable_icc` set to
    ``false``

    A container cannot reach another container on the network at that container's address, on any
    port. This does not apply to two containers that a legacy link joins (see the next expectation).
    It reaches the published ports of that container at the node's address.

.. expectation:: Without ICC, a legacy link reaches only exposed ports over IPv4
    :id: BR-ISO-NO-ICC-LEGACY-LINK
    :observed_from: ctr → ctr
    :depends_on: os-linux
    :platforms: linux
    :needs: BR-HOST-BR-NETFILTER

    *Applies to:* A daemon with :daemon-json:`icc` false, and a container on the default bridge
    network whose :api-field:`HostConfig.Links` names another container on that network

    The container reaches the linked container at the linked container's IPv4 address, on each port
    that the linked container exposes. It does not reach the linked container on any other port, or
    at the linked container's IPv6 address.

.. expectation:: The node does not forward traffic for an internal network
    :id: BR-ISO-INTERNAL-NO-FORWARD
    :observed_from: ctr, underlay
    :depends_on: os-linux
    :platforms: linux

    The node does not forward traffic between a container on an ``internal`` bridge network and an
    address outside the network's subnets, in either direction. This holds even when the container
    has a default route through the bridge, such as one that a container with :cap:`NET_ADMIN` adds
    itself. Traffic that the node delivers to its own addresses is not forwarded, and this
    expectation does not cover it.

.. expectation:: The node reaches containers on an internal network
    :id: BR-ISO-INTERNAL-HOST-TO-CTR
    :observed_from: node
    :depends_on: os-linux
    :platforms: linux

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

    *Applies to:* An ``internal`` bridge network with
    :drvopt:`com.docker.network.bridge.gateway_mode_ipv4` set to ``isolated``

    The bridge does not have an IPv4 address. A process in the node's host network namespace cannot
    reach the network's containers at their IPv4 addresses.

.. expectation:: Containers on an isolated network cannot reach the node
    :id: BR-ISO-ISOLATED-CTR-TO-HOST
    :observed_from: ctr
    :depends_on: os-linux
    :platforms: linux

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

    *Applies to:* A daemon with :daemon-json:`allow-direct-routing` false, a container on a bridge
    network whose :drvopt:`com.docker.network.bridge.gateway_mode_ipv4` is ``nat`` or not set, and
    an underlay host with a route to the container's IPv4 address via the node's underlay address

    Traffic from that host to the container's IPv4 address arrives on an interface that is not in
    the network's :drvopt:`com.docker.network.bridge.trusted_host_interfaces`. The traffic does not
    reach the container on any port.

.. expectation:: With allow-direct-routing, direct routing reaches published ports only
    :id: BR-ISO-DIRECT-ROUTING-ON
    :observed_from: underlay → ctr
    :depends_on: os-linux
    :platforms: linux

    *Applies to:* A daemon with :daemon-json:`allow-direct-routing` true, a container on a bridge
    network whose :drvopt:`com.docker.network.bridge.gateway_mode_ipv4` is ``nat`` or not set, and
    an underlay host with a route to the container's IPv4 address via the node's underlay address

    Traffic from that host to the container's IPv4 address reaches the container's published ports
    only.

.. expectation:: A trusted interface reaches published ports only
    :id: BR-ISO-TRUSTED-IFACE
    :observed_from: underlay → ctr
    :depends_on: os-linux
    :platforms: linux

    *Applies to:* A container on a bridge network whose
    :drvopt:`com.docker.network.bridge.gateway_mode_ipv4` is ``nat`` or not set, and an underlay
    host with a route to the container's IPv4 address via the node's underlay address

    When traffic from that host to the container's IPv4 address arrives on an interface in the
    network's :drvopt:`com.docker.network.bridge.trusted_host_interfaces`, it reaches the
    container's published ports only.

.. expectation:: A colon-separated list names several trusted interfaces
    :id: BR-ISO-TRUSTED-IFACE-LIST
    :observed_from: underlay → ctr
    :depends_on: none
    :platforms: linux

    *Applies to:* A bridge network whose :drvopt:`com.docker.network.bridge.trusted_host_interfaces`
    names several interfaces separated by colons, such as ``eth1:eth2``, and whose
    :drvopt:`com.docker.network.bridge.gateway_mode_ipv4` is ``nat`` or not set, and an underlay
    host with a route to the IPv4 address of a container on the network via the node's underlay
    address

    When traffic from that host to the container's IPv4 address arrives on any of the named
    interfaces, it reaches the container's published ports.

.. expectation:: The daemon enables bridge-nf-call-iptables
    :id: BR-ISO-BRIDGE-NF-CALL
    :observed_from: node
    :depends_on: none
    :platforms: linux
    :needs: BR-HOST-BR-NETFILTER

    *Applies to:* A daemon with :daemon-json:`iptables` true, on a host where
    :sysctl:`net.ipv4.ip_forward` is 1, and a bridge network with
    :drvopt:`com.docker.network.bridge.enable_icc` set to ``false``, or any bridge network when
    :daemon-json:`userland-proxy` is false

    The sysctl :sysctl:`net.bridge.bridge-nf-call-iptables` is 1. See :ref:`Host requirements
    <bridge-networking-host-requirements>`.

.. expectation:: A container does not start without veth support
    :id: BR-ISO-NO-VETH
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :pins: BR-HOST-NETNS-VETH-BRIDGE

    *Applies to:* A Linux host whose kernel does not offer veth pairs, and a created or stopped
    container on a bridge network

    :api-op:`POST /containers/{id}/start` for the container responds 500, with a message that
    contains :api-error:`pair interfaces`. See :ref:`Host requirements
    <bridge-networking-host-requirements>`.

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

    :api-op:`POST /containers/{id}/start` for a created or stopped container that is on a bridge
    network and publishes a port without a host port responds 204. :api-op:`GET
    /containers/{id}/json` then reports the assigned host port in
    :api-field:`NetworkSettings.Ports`. A client reaches the container on that host port at the
    node's address.

.. expectation:: A container that publishes an SCTP port does not start without SCTP
    :id: BR-PUB-NO-SCTP
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :pins: BR-HOST-SCTP, SWARM-HOST-SCTP

    *Applies to:* A Linux host whose kernel does not offer SCTP, and a created or stopped container
    on a bridge network that publishes an SCTP port

    :api-op:`POST /containers/{id}/start` for the container responds with a 4xx or 5xx status, with
    a message that contains :api-error:`failed to create socket for` and ``/sctp``. See :ref:`Host
    requirements <bridge-networking-host-requirements>`.

.. expectation:: A container's unpublished port is not reachable at the node's address
    :id: BR-PUB-UNPUBLISHED-PORT
    :observed_from: client, ctr
    :depends_on: os-linux
    :platforms: linux

    *Applies to:* A container that is on a bridge network and listens on a port that it does not
    publish

    A client that connects to the node's address, on any port, does not reach that listener.

.. expectation:: A port published on 127.0.0.1 is reachable from the node
    :id: BR-PUB-LOOPBACK-NODE
    :observed_from: node
    :depends_on: os-linux
    :platforms: linux

    A port published on ``127.0.0.1`` is reachable from the node at ``127.0.0.1``.

.. expectation:: A port published on 127.0.0.1 is not reachable from a client
    :id: BR-PUB-LOOPBACK-CLIENT
    :observed_from: client
    :depends_on: os-linux
    :platforms: linux

    A port published on ``127.0.0.1`` is not reachable from a client at any address of the node.
    This includes a packet to ``127.0.0.1`` that a client on the node's link sends in an Ethernet
    frame addressed to the node's MAC address.

.. expectation:: A port published on a node address is reachable at that address only
    :id: BR-PUB-HOST-ADDRESS
    :observed_from: node, client
    :depends_on: os-linux
    :platforms: linux

    A port published on an address of the node is reachable at that address, from the node and from
    a client. It is not reachable at the node's other addresses, ``127.0.0.1`` included.

.. expectation:: A port published with no host address answers at node IPv6 addresses
    :id: BR-PUB-V6-REACHABLE
    :observed_from: client
    :depends_on: os-linux
    :platforms: linux

    *Applies to:* A bridge network with IPv6 enabled, and a daemon with :daemon-json:`ip6tables`
    true

    A port published without a host address is reachable at the node's IPv6 addresses.

.. expectation:: Over IPv6, the container sees the client's address as the source
    :id: BR-PUB-V6-CLIENT-SOURCE
    :observed_from: client → ctr
    :depends_on: os-linux
    :platforms: linux

    *Applies to:* A bridge network with IPv6 enabled, and a daemon with :daemon-json:`ip6tables`
    true

    A client connects to a port published without a host address, at one of the node's IPv6
    addresses. The container observes the IPv6 address of the client as the source address.

.. expectation:: Without the userland proxy, published ports do not answer at link-local
    :id: BR-PUB-V6-LINK-LOCAL
    :observed_from: client
    :depends_on: os-linux
    :platforms: linux
    :needs: BR-HOST-BR-NETFILTER

    *Applies to:* A bridge network with IPv6 enabled, and a daemon with :daemon-json:`ip6tables`
    true and :daemon-json:`userland-proxy` false

    A port published without a host address is not reachable from a client at an IPv6 link-local
    address of the node.

.. expectation:: Without the userland proxy, the bridge's route_localnet is 1
    :id: BR-PUB-ROUTE-LOCALNET
    :observed_from: node
    :depends_on: none
    :platforms: linux
    :needs: BR-HOST-BR-NETFILTER

    *Applies to:* A daemon with :daemon-json:`userland-proxy` false, and a bridge network

    The sysctl :sysctl:`net.ipv4.conf.<bridge>.route_localnet` is 1, where ``<bridge>`` is the
    network's bridge interface. See :ref:`Host requirements <bridge-networking-host-requirements>`.

.. _bridge-networking-egress:

Egress
======

.. expectation:: IPv4 egress from containers has the node's address as its source
    :id: BR-EGRESS-MASQUERADE
    :observed_from: underlay
    :depends_on: os-linux
    :platforms: linux

    *Applies to:* A bridge network without the options
    :drvopt:`com.docker.network.bridge.enable_ip_masquerade`,
    :drvopt:`com.docker.network.bridge.gateway_mode_ipv4` and :drvopt:`com.docker.network.host_ipv4`

    An underlay host observes an address of the node as the source address of IPv4 egress from the
    network's containers.

.. expectation:: Without masquerade, IPv4 egress has the container's address as source
    :id: BR-EGRESS-NO-MASQUERADE
    :observed_from: underlay
    :depends_on: os-linux
    :platforms: linux

    *Applies to:* A bridge network with :drvopt:`com.docker.network.bridge.enable_ip_masquerade` set
    to ``false``, and without :drvopt:`com.docker.network.bridge.gateway_mode_ipv4`

    An underlay host observes the container's address as the source address of IPv4 egress from a
    container on the network.

.. expectation:: An unmasqueraded egress reply on an untrusted interface is not delivered
    :id: BR-EGRESS-REPLY-DIRECT-ROUTING-OFF
    :observed_from: underlay → ctr
    :depends_on: os-linux
    :platforms: linux

    *Applies to:* A daemon with :daemon-json:`allow-direct-routing` false, and a bridge network with
    :drvopt:`com.docker.network.bridge.enable_ip_masquerade` set to ``false`` and without
    :drvopt:`com.docker.network.bridge.gateway_mode_ipv4`

    An underlay host replies to IPv4 egress from a container on the network. The reply arrives on an
    interface that is not in the network's
    :drvopt:`com.docker.network.bridge.trusted_host_interfaces`. The reply does not reach the
    container.

.. expectation:: With allow-direct-routing, a reply to unmasqueraded egress is delivered
    :id: BR-EGRESS-REPLY-DIRECT-ROUTING-ON
    :observed_from: underlay → ctr
    :depends_on: os-linux
    :platforms: linux

    *Applies to:* A daemon with :daemon-json:`allow-direct-routing` true, and a bridge network with
    :drvopt:`com.docker.network.bridge.enable_ip_masquerade` set to ``false`` and without
    :drvopt:`com.docker.network.bridge.gateway_mode_ipv4`

    An underlay host replies to IPv4 egress from a container on the network. The reply reaches the
    container.

.. expectation:: A reply to unmasqueraded egress on a trusted interface is delivered
    :id: BR-EGRESS-REPLY-TRUSTED-IFACE
    :observed_from: underlay → ctr
    :depends_on: os-linux
    :platforms: linux

    *Applies to:* A bridge network with :drvopt:`com.docker.network.bridge.enable_ip_masquerade` set
    to ``false`` and without :drvopt:`com.docker.network.bridge.gateway_mode_ipv4`

    An underlay host replies to IPv4 egress from a container on the network. The reply arrives on an
    interface in the network's :drvopt:`com.docker.network.bridge.trusted_host_interfaces`. The
    reply reaches the container.

.. expectation:: With host_ipv4 set, IPv4 egress has that address as its source
    :id: BR-EGRESS-HOST-IPV4
    :observed_from: underlay
    :depends_on: os-linux
    :platforms: linux

    *Applies to:* A bridge network with :drvopt:`com.docker.network.host_ipv4` set to an address of
    the node, :drvopt:`com.docker.network.bridge.enable_ip_masquerade` not ``false``, and
    :drvopt:`com.docker.network.bridge.gateway_mode_ipv4` not ``routed``

    An underlay host observes that address as the source address of IPv4 egress from the network's
    containers.

.. _bridge-networking-gateway-modes:

Gateway modes
=============

.. expectation:: In routed mode, IPv4 egress has the container's address as source
    :id: BR-GWMODE-ROUTED-EGRESS
    :observed_from: underlay
    :depends_on: os-linux
    :platforms: linux

    *Applies to:* A bridge network with :drvopt:`com.docker.network.bridge.gateway_mode_ipv4` set to
    ``routed``

    An underlay host observes the container's address as the source address of IPv4 egress from a
    container on the network.

.. expectation:: In routed mode, a client with a route reaches published ports only
    :id: BR-GWMODE-ROUTED-INBOUND
    :observed_from: client
    :depends_on: os-linux
    :platforms: linux

    *Applies to:* A container on a bridge network with
    :drvopt:`com.docker.network.bridge.gateway_mode_ipv4` set to ``routed``, and a client with a
    route to the container's IPv4 address via the node's address

    Traffic from the client to the container's IPv4 address reaches the container's published ports,
    and not its other ports.

.. expectation:: In routed mode, published ports are not reachable at the node's address
    :id: BR-GWMODE-ROUTED-NODE-ADDRESS
    :observed_from: client
    :depends_on: os-linux
    :platforms: linux

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

    *Applies to:* A container on a bridge network with
    :drvopt:`com.docker.network.bridge.gateway_mode_ipv4` set to ``nat-unprotected``, and a client
    with a route to the container's IPv4 address via the node's address

    Traffic from the client to the container's IPv4 address reaches every port of the container.

.. expectation:: Isolated gateway mode on a network that is not internal is refused
    :id: BR-GWMODE-ISOLATED-NOT-INTERNAL
    :observed_from: api
    :depends_on: none
    :platforms: linux

    :api-op:`POST /networks/create` for a bridge network with :api-field:`Internal` not true and
    with :drvopt:`com.docker.network.bridge.gateway_mode_ipv4` or
    :drvopt:`com.docker.network.bridge.gateway_mode_ipv6` set to ``isolated`` responds with a 4xx or
    5xx status. :api-op:`GET /networks` then does not list the network.

.. expectation:: Isolated gateway mode on a network that is not internal responds 500
    :id: BR-GWMODE-ISOLATED-NOT-INTERNAL-HTTP500
    :observed_from: api
    :depends_on: none
    :platforms: linux

    :api-op:`POST /networks/create` for a bridge network with :api-field:`Internal` not true and
    with :drvopt:`com.docker.network.bridge.gateway_mode_ipv4` or
    :drvopt:`com.docker.network.bridge.gateway_mode_ipv6` set to ``isolated`` responds 500. The
    message contains :api-error:`gateway mode 'isolated' can only be used for an internal network`.
    See :ref:`bridge-networking-known-divergences-non-goals`.

.. expectation:: docker network create without --internal refuses isolated mode
    :id: BR-GWMODE-ISOLATED-NOT-INTERNAL-CLI
    :observed_from: api
    :depends_on: cli
    :platforms: linux
    :cli_for: BR-GWMODE-ISOLATED-NOT-INTERNAL

    :cli:`docker network create --driver bridge --opt
    com.docker.network.bridge.gateway_mode_ipv4=isolated` without :cli-flag:`--internal` exits
    non-zero and prints the daemon's error. :cli:`docker network ls` then does not list the network.

.. expectation:: An unknown gateway mode is refused
    :id: BR-GWMODE-UNKNOWN
    :observed_from: api
    :depends_on: none
    :platforms: linux

    :api-op:`POST /networks/create` for a bridge network with
    :drvopt:`com.docker.network.bridge.gateway_mode_ipv4` or
    :drvopt:`com.docker.network.bridge.gateway_mode_ipv6` set to a value other than ``nat``,
    ``nat-unprotected``, ``routed`` and ``isolated`` responds with a 4xx or 5xx status. :api-op:`GET
    /networks` then does not list the network.

.. expectation:: An unknown gateway mode responds 400
    :id: BR-GWMODE-UNKNOWN-MSG
    :observed_from: api
    :depends_on: none
    :platforms: linux

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
    :cli_for: BR-GWMODE-UNKNOWN

    :cli:`docker network create --driver bridge` with the option
    :drvopt:`com.docker.network.bridge.gateway_mode_ipv4` set to a value other than ``nat``,
    ``nat-unprotected``, ``routed`` and ``isolated`` exits non-zero and prints the daemon's error.
    :cli:`docker network ls` then does not list the network.

.. _bridge-networking-default-gateway:

Default gateway
===============

:ref:`network-common-default-gateway` covers which endpoint gives a container its default route, for
every network driver. The expectations in this subsection hold with :daemon-json:`userland-proxy`
true and with :daemon-json:`userland-proxy` false, unless an expectation names a value.

.. expectation:: A published port reaches the container on its default route's network
    :id: BR-GW-DEFAULT-ROUTE-NETWORK
    :observed_from: client, ctr
    :depends_on: os-linux
    :platforms: linux

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

    *Applies to:* A Linux host booted with ``ipv6.disable=1``, and a daemon with its default
    settings

    The daemon starts. :api-op:`GET /info` responds 200.

.. expectation:: On a host without IPv6, a container on a bridge network starts
    :id: BR-NOV6-CTR-START
    :observed_from: api
    :depends_on: none
    :platforms: linux

    *Applies to:* A Linux host booted with ``ipv6.disable=1``, and a created container on a bridge
    network without IPv6

    :api-op:`POST /containers/{id}/start` for the container responds 204.

.. expectation:: On a host without IPv6, a published port is reachable over IPv4
    :id: BR-NOV6-PUBLISHED-PORT
    :observed_from: client
    :depends_on: os-linux
    :platforms: linux

    *Applies to:* A Linux host booted with ``ipv6.disable=1``, and a running container on a bridge
    network without IPv6 that publishes a port without a host address

    A client reaches the container on that port at the node's IPv4 address.

.. expectation:: On a host without IPv6, an IPv6 bridge network is refused
    :id: BR-NOV6-V6-NETWORK-REFUSED
    :observed_from: api
    :depends_on: none
    :platforms: linux

    *Applies to:* A Linux host booted with ``ipv6.disable=1``

    :api-op:`POST /networks/create` for a bridge network with :api-field:`EnableIPv6` set responds
    with a 4xx or 5xx status. :api-op:`GET /networks` then does not list the network.

.. _bridge-networking-known-divergences-non-goals:

*******************************
 Known divergences & non-goals
*******************************

.. expectation:: Rules left in built-in iptables chains after a switch to nftables
    :id: BR-CLEAN-BUILTIN-CHAIN-RULES
    :observed_from: node
    :depends_on: none
    :platforms: linux

    The daemon deletes them only for the networks, endpoints and port bindings that it restores.
    Rules for anything that it does not restore stay in the built-in chains
    (:ref:`bridge-networking-cross-generation-cleanup`).

.. expectation:: Firewalld reload under the nftables backend
    :id: BR-FW-FIREWALLD-RELOAD-NFT
    :observed_from: log
    :depends_on: none
    :platforms: linux
    :rationale: RAT-BR-RELOAD-NFT-WARNINGS

    Logged as failures. A firewalld reload does not change the daemon's nftables tables. On each
    reload, the daemon still adds its rules for each bridge network again, and nftables refuses each
    rule that exists. The daemon log then contains these warning-level entries for each reload: (1)
    one entry per bridge network that contains :log:`ReapplyNetworkLevelRules is not implemented for
    nftables`; (2) one entry per endpoint that contains :log:`Failed to reconfigure Endpoint` and
    :log:`rule exists`, for an endpoint on a network that is not ``internal`` and whose gateway mode
    for IPv4 or IPv6 is ``nat``, the default, on a daemon with :daemon-json:`allow-direct-routing`
    false; (3) one entry per endpoint that publishes a port that contains :log:`Failed to
    reconfigure NAT` and :log:`rule exists`. The daemon's tables do not change.

.. expectation:: Published ports at the node's IPv6 addresses through docker-proxy, with firewalld running
    :id: SWARM-PUB-FIREWALLD-V6-PROXY
    :observed_from: client
    :depends_on: os-linux
    :platforms: linux

    Refused. A port published on an endpoint without an IPv6 address can be reachable at the node's
    IPv6 addresses through docker-proxy (:ref:`README.md <README-observed-from>`). With firewalld
    running and its default zone in effect for the interface, a connection to such a port at an IPv6
    address of the node does not answer, before and after a firewalld reload, with either firewall
    backend. The daemon does not open the port in firewalld. The port answers at the node's IPv4
    addresses (:ref:`bridge-networking-port-publishing`).

.. expectation:: Out-of-range bridge MTU error
    :id: BR-OPT-MTU-OUT-OF-RANGE
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :rationale: RAT-BR-OPT-MTU-OUT-OF-RANGE

    The error does not name the option. :api-op:`POST /networks/create` with an MTU that the kernel
    does not accept responds 500 with :api-error:`invalid argument`. A request can have IPv6 enabled
    and an MTU that the kernel accepts for IPv4 but not for IPv6. That request responds 500 with a
    message that contains :api-error:`Cannot read IPv6 setup for bridge` and :api-error:`no such
    file or directory` (:ref:`bridge-networking-network-and-endpoint-options`).

.. expectation:: Bridge MTU from 2³² to 2⁶³ − 1
    :id: BR-OPT-MTU-MODULO
    :observed_from: ctr
    :depends_on: none
    :platforms: linux

    The value is applied modulo 2³². This applies to :drvopt:`com.docker.network.driver.mtu` and to
    the daemon-wide :daemon-json:`mtu`. A daemon-wide :daemon-json:`mtu` whose remainder the kernel
    accepts does not stop the daemon. :api-op:`POST /networks/create` accepts a value whose
    remainder the kernel accepts. For example, 4294968796 is applied as 1500. :api-op:`GET
    /networks/{id}` then reports the requested value in :api-field:`Options`. :api-op:`POST
    /networks/create` refuses a value whose remainder the kernel refuses, as the *Out-of-range
    bridge MTU error* entry describes (:ref:`bridge-networking-network-and-endpoint-options`).

.. expectation:: Daemon-wide mtu error
    :id: BR-OPT-DAEMON-MTU-ERROR
    :observed_from: log
    :depends_on: none
    :platforms: linux
    :rationale: RAT-BR-OPT-DAEMON-MTU-ERROR

    A value that the kernel cannot apply stops the daemon with an error that ends in :log:`error
    creating default "bridge" network: invalid argument`. This error does not name the setting
    (:ref:`bridge-networking-network-and-endpoint-options`).

.. expectation:: Daemon-wide mtu of 0 in daemon.json
    :id: BR-OPT-DAEMON-MTU-ZERO-REPLACED
    :observed_from: node
    :depends_on: none
    :platforms: linux

    Replaced by the default. A :daemon-json:`"mtu"` of 0 in daemon.json is replaced by 1500 when the
    file is merged with the flags. The daemon then sets 1500 on the default bridge and its
    containers' interfaces. Only :dockerd-flag:`--mtu 0` leaves the MTU that the kernel gave them
    (:ref:`bridge-networking-network-and-endpoint-options`).

.. expectation:: HTTP 500 for refused requests
    :id: BR-FW-HTTP500-REFUSED
    :observed_from: api
    :depends_on: none
    :platforms: linux

    The daemon refuses each request below on purpose, but the request responds 500. 500 is the
    status for a fault in the daemon. The requests are: (1) :api-op:`POST /networks/create` for a
    bridge network with an MTU that the kernel refuses. (2) :api-op:`POST /networks/create` for a
    bridge network with :drvopt:`com.docker.network.bridge.gateway_mode_ipv4` or
    :drvopt:`com.docker.network.bridge.gateway_mode_ipv6` set to ``isolated``, when
    :api-field:`Internal` is not true (:ref:`bridge-networking-network-and-endpoint-options`,
    :ref:`bridge-networking-gateway-modes`).

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
  firewalld's behavior. After a reload, the daemon recreates the chain and the jump to it. It does
  not restore the chain's rules.
