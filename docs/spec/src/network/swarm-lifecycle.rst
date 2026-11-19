#############################
 Swarm networking: lifecycle
#############################

This document continues :doc:`swarm-networking`. Its conventions, definitions and host requirements
apply here.

.. _swarm-lifecycle-rationales:

************
 Rationales
************

.. rationale:: Why a change to SWARM-PORTALLOC-AUTO-UPDATE-REASSIGNED should be deliberate
    :id: RAT-SWARM-PORTALLOC-AUTO-UPDATE-REASSIGNED

    It catches a change that keeps a service's other auto-assigned ports across an update that adds
    or removes a port; clients and load balancers that were given today's numbers see other ports
    after such an update, and a fix would change which ports they must look up again, so the change
    should be deliberate.

.. rationale:: Why a change to SWARM-ENC-CLEARTEXT-VXLAN-UNAUTHENTICATED should be deliberate
    :id: RAT-SWARM-CLEARTEXT-SENDERS-NOT-FILTERED

    It catches a change that makes cleartext overlays filter or authenticate VXLAN by sender; that
    could drop traffic from hosts that deployments route through today, and the documentation
    answers untrusted underlays with encryption, so it should be deliberate.

.. rationale:: Why a change to SWARM-FAILREP-LOG-NO-SERVICE should be deliberate
    :id: RAT-SWARM-FAILREP-LOG-NO-SERVICE

    It catches a change to the content of these failure entries, such as naming the service or
    printing the security association; operators' log searches and alerts match the entries' text,
    so the change should be deliberate.

.. rationale:: Why a change to SWARM-FAILREP-NOT-REPEATED should be deliberate
    :id: RAT-SWARM-FAILREP-NOT-REPEATED

    It catches a change that makes a node report a failure to program cluster-scoped configuration,
    or try again, without a trigger, for example on a timer; that changes the volume and timing of
    the entries that operators' log alerts see, so the change should be deliberate.

.. rationale:: Why a change to SWARM-INGRESS-HTTP400-STATE-REFUSAL should be deliberate
    :id: RAT-SWARM-STATE-REFUSAL-STATUS-MAPPED

    It catches a change of the status of a request that the Swarm's state refuses, such as from 400
    to 409; clients branch on the status to decide whether to try again, so the change should be
    deliberate.

.. rationale:: Why a change to SWARM-CONVERGE-PARTIAL-BLOCK should be deliberate
    :id: RAT-SWARM-PEERS-FROM-GOSSIP

    It catches a change that makes a node detect a data-path-only block and drop or report the peer;
    the documentation makes the open data path a host requirement, so how the daemon reacts to its
    loss is a design choice that should be deliberate.

.. rationale:: Why a change to SWARM-FAILREP-RECOVERY-UNREPORTED should be deliberate
    :id: RAT-SWARM-FAILREP-RECOVERY-UNREPORTED

    It catches a change that makes the daemon log a node's recovery from a failure to program Swarm
    networking at the default log level; that adds entries that operators' log monitoring sees, so
    the change should be deliberate.

.. _swarm-lifecycle-encryption:

************
 Encryption
************

Encrypted overlay networks are implemented on Linux nodes only
(:ref:`swarm-lifecycle-known-divergences-non-goals`). This section holds on Linux nodes, except
where an expectation names Windows. The expectations about the traffic that an underlay host
observes name ``os`` because they observe the data path. The missing-XFRM expectations name ``none``
because the daemon alone decides their verdict once XFRM is missing.

.. expectation:: No cleartext VXLAN for an encrypted network's VNI crosses the underlay
    :id: SWARM-ENC-NO-CLEARTEXT
    :observed_from: underlay
    :depends_on: os-linux
    :platforms: linux
    :needs: SWARM-HOST-XFRM, SWARM-HOST-IPTABLES-COMMAND

    No cleartext VXLAN traffic is observable between nodes on the underlay network for the VNIs of
    :drvopt:`encrypted` overlay networks.

.. expectation:: Underlay traffic for an encrypted network's VNI is IPsec ESP
    :id: SWARM-ENC-ESP
    :observed_from: underlay
    :depends_on: os-linux
    :platforms: linux
    :needs: SWARM-HOST-XFRM, SWARM-HOST-IPTABLES-COMMAND

    Traffic between nodes on the underlay network for the VNIs of :drvopt:`encrypted` overlay
    networks is observable as IPsec ESP.

.. expectation:: ESP is in transport mode and protects the VXLAN datagrams
    :id: SWARM-ENC-ESP-TRANSPORT-MODE
    :observed_from: underlay
    :depends_on: os-linux
    :platforms: linux
    :needs: SWARM-HOST-XFRM, SWARM-HOST-IPTABLES-COMMAND

    ESP is in transport mode and protects the VXLAN datagrams.

.. expectation:: Security associations use rfc4106(gcm(aes)) with a 64-bit ICV
    :id: SWARM-ENC-ALGORITHM
    :observed_from: node
    :depends_on: none
    :platforms: linux
    :needs: SWARM-HOST-XFRM, SWARM-HOST-IPTABLES-COMMAND

    An encrypted network's IPsec security associations use ``rfc4106(gcm(aes))`` with a 64-bit ICV.
    See :ref:`Host requirements <swarm-networking-host-requirements>`.

.. expectation:: Forged cleartext VXLAN from the underlay does not reach encrypted tasks
    :id: SWARM-ENC-FORGED-VXLAN
    :observed_from: underlay → task
    :depends_on: os-linux
    :platforms: linux
    :needs: SWARM-HOST-XFRM, SWARM-HOST-IPTABLES-COMMAND

    A forged cleartext VXLAN datagram for the VNI of an :drvopt:`encrypted` overlay network,
    injected from the underlay, does not reach the network's tasks.

.. expectation:: Cleartext VXLAN for a cleartext network's VNI passes between nodes
    :id: SWARM-ENC-CLEARTEXT-PASSES
    :observed_from: underlay
    :depends_on: os-linux
    :platforms: linux
    :needs: SWARM-HOST-XFRM, SWARM-HOST-IPTABLES-COMMAND

    Cleartext VXLAN for a cleartext network's VNI passes between nodes, whether or not those nodes
    also have :drvopt:`encrypted` networks.

.. expectation:: A cleartext network on a deleted encrypted network's VNI passes VXLAN
    :id: SWARM-ENC-VNI-REUSE
    :observed_from: underlay
    :depends_on: os-linux
    :platforms: linux
    :needs: SWARM-HOST-XFRM, SWARM-HOST-IPTABLES-COMMAND

    *Applies to:* Two nodes on which an encrypted network was programmed

    After that network is deleted, a cleartext network that is allocated the same VNI passes
    cleartext VXLAN between those nodes.

.. expectation:: A VNI freed while the daemon was down passes cleartext traffic
    :id: SWARM-ENC-VNI-REUSE-KILLED-DAEMON
    :observed_from: task
    :depends_on: os-linux
    :platforms: linux
    :needs: SWARM-HOST-XFRM, SWARM-HOST-IPTABLES-COMMAND

    *Applies to:* A node where an encrypted network is programmed

    The node's daemon is killed. Then that network is deleted from the Swarm, and the node's daemon
    is started. After these events, a cleartext network that is allocated the same VNI passes
    traffic to and from the node.

.. expectation:: On a reused encrypted VNI, Windows containers reach Linux containers
    :id: SWARM-ENC-VNI-REUSE-WINDOWS
    :observed_from: task, ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :needs: SWARM-HOST-XFRM, SWARM-HOST-IPTABLES-COMMAND

    *Applies to:* A cleartext network that is allocated the VNI of a deleted encrypted network

    Containers on Windows nodes on that network can exchange traffic with containers on the Linux
    nodes that had containers on the encrypted network.

.. expectation:: Two key rotations leave no delivery gap longer than 1 second
    :id: SWARM-ENC-KEY-ROTATION
    :observed_from: task, node
    :depends_on: os-linux
    :platforms: linux
    :needs: SWARM-HOST-XFRM, SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-FIREWALL-ESP

    Across two consecutive rotations of the network encryption keys, a UDP flow at a fixed rate
    between tasks on different nodes of an :drvopt:`encrypted` network has no gap in delivery longer
    than 1 second. This holds without any action by the operator.

.. expectation:: A node without the encryption keys rejects tasks on encrypted networks
    :id: SWARM-ENC-NO-KEYS
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux

    *Applies to:* A node that does not have the network encryption keys

    On that node, a task attached to an :drvopt:`encrypted` network is rejected.

.. expectation:: A task rejected for missing keys reports encryption keys not present
    :id: SWARM-ENC-NO-KEYS-MSG
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux

    *Applies to:* A node that does not have the network encryption keys

    On that node, the error that :api-op:`GET /tasks` reports for a rejected task on an
    :drvopt:`encrypted` network contains :api-error:`cannot join secure network: encryption keys not
    present`.

.. expectation:: A host without XFRM netlink rejects tasks on encrypted networks
    :id: SWARM-ENC-NO-XFRM
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :pins: SWARM-HOST-XFRM

    *Applies to:* A Linux host whose kernel does not offer the XFRM netlink interface

    On that host, a task attached to an :drvopt:`encrypted` network is rejected.

.. expectation:: A task rejected for missing XFRM reports required modules are missing
    :id: SWARM-ENC-NO-XFRM-MSG
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :pins: SWARM-HOST-XFRM

    *Applies to:* A Linux host whose kernel does not offer the XFRM netlink interface

    On that host, the error that :api-op:`GET /tasks` reports for a rejected task on an
    :drvopt:`encrypted` network contains :api-error:`cannot join secure network: required modules to
    install IPSEC rules are missing on host`.

.. expectation:: Without XFRM netlink, a container on an encrypted network does not start
    :id: SWARM-ENC-NO-XFRM-CTR-START
    :observed_from: api:active
    :depends_on: none
    :platforms: linux
    :pins: SWARM-HOST-XFRM

    *Applies to:* A Linux host whose kernel does not offer the XFRM netlink interface

    On that host, :api-op:`POST /containers/{id}/start` for a created or stopped container on an
    attachable :drvopt:`encrypted` network responds with a 4xx or 5xx status.

.. expectation:: Without XFRM netlink, a container start responds 500 after 20 seconds
    :id: SWARM-ENC-NO-XFRM-CTR-START-HTTP500
    :observed_from: api:active
    :depends_on: none
    :platforms: linux
    :pins: SWARM-HOST-XFRM

    *Applies to:* A Linux host whose kernel does not offer the XFRM netlink interface

    On that host, :api-op:`POST /containers/{id}/start` for a created or stopped container on an
    attachable :drvopt:`encrypted` network responds 500 after 20 seconds, with :api-error:`attaching
    to network failed, make sure your network options are correct and check manager logs: context
    deadline exceeded`. See :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: Without XFRM netlink, docker run on an encrypted network exits non-zero
    :id: SWARM-ENC-NO-XFRM-CTR-START-CLI
    :observed_from: api:active
    :depends_on: cli
    :platforms: linux
    :cli_for: SWARM-ENC-NO-XFRM-CTR-START
    :pins: SWARM-HOST-XFRM

    *Applies to:* A Linux host whose kernel does not offer the XFRM netlink interface

    On that host, :cli:`docker run -d --network` naming an attachable :drvopt:`encrypted` network
    exits non-zero.

.. expectation:: After a start fails without XFRM, the container stays created
    :id: SWARM-ENC-NO-XFRM-CTR-CREATED
    :observed_from: api:active
    :depends_on: none
    :platforms: linux
    :pins: SWARM-HOST-XFRM

    *Applies to:* A Linux host whose kernel does not offer the XFRM netlink interface, and a created
    container without auto-remove (:cli-flag:`--rm`) on an attachable :drvopt:`encrypted` network

    After :api-op:`POST /containers/{id}/start` for the container fails on that host, the container
    stays in the created state.

.. expectation:: After a start fails without XFRM, an auto-remove container is removed
    :id: SWARM-ENC-NO-XFRM-CTR-AUTO-REMOVE
    :observed_from: api:active
    :depends_on: none
    :platforms: linux
    :pins: SWARM-HOST-XFRM

    *Applies to:* A Linux host whose kernel does not offer the XFRM netlink interface, and a created
    container with auto-remove (:cli-flag:`--rm`) on an attachable :drvopt:`encrypted` network

    After :api-op:`POST /containers/{id}/start` for the container fails on that host, the container
    is removed.

.. expectation:: Without rfc4106(gcm(aes)), a task on an encrypted network starts
    :id: SWARM-ENC-NO-ALGORITHM-TASK-STARTS
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :pins: SWARM-HOST-XFRM

    *Applies to:* A Linux host whose kernel offers the XFRM netlink interface but not
    ``rfc4106(gcm(aes))``

    On that host, a task attached to an :drvopt:`encrypted` network starts. See
    :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: Without rfc4106(gcm(aes)), encrypted traffic between nodes fails
    :id: SWARM-ENC-NO-ALGORITHM-NO-TRAFFIC
    :observed_from: task
    :depends_on: none
    :platforms: linux
    :pins: SWARM-HOST-XFRM

    *Applies to:* A Linux host whose kernel offers the XFRM netlink interface but not
    ``rfc4106(gcm(aes))``

    The tasks on that host do not exchange traffic over an :drvopt:`encrypted` network with tasks on
    other nodes. See :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: Without rfc4106(gcm(aes)), each failed SA logs a Failed Adding warning
    :id: SWARM-ENC-NO-ALGORITHM-LOG
    :observed_from: log
    :depends_on: none
    :platforms: linux
    :pins: SWARM-HOST-XFRM

    *Applies to:* A Linux host whose kernel offers the XFRM netlink interface but not
    ``rfc4106(gcm(aes))``

    The daemon log of that host contains a warning-level entry that contains :log:`Failed Adding`
    for each IPsec security association that it fails to add. See
    :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. _swarm-lifecycle-wire-format-interop:

***********************
 Wire format & interop
***********************

Everything here is hard contract. See :ref:`Interop premise <swarm-networking-interop-premise>`.

This section groups the expectations by the kind of overlay network that they apply to. Some
expectations are about what an underlay host sees without the network's keys. Those expectations
apply only to cleartext networks. The reason is that the VXLAN datagram of an encrypted network is
not visible on the wire.

.. _swarm-lifecycle-all-overlay-networks:

All overlay networks
====================

.. expectation:: A container is reachable from other nodes with any IP and MAC assignment
    :id: SWARM-WIRE-ADDRESS-ASSIGNMENT
    :observed_from: task, ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows

    A container is reachable from other nodes whether its IP address and its MAC address are each
    assigned manually or automatically.

.. expectation:: Cross-node expectations hold between workers of the previous and current release
    :id: SWARM-WIRE-MIXED-VERSIONS
    :observed_from: client, task, underlay
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :needs: BR-HOST-NFTABLES, BR-HOST-FORWARDING-V4, BR-HOST-NO-FORWARD-DROP, SWARM-HOST-XFRM, SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-FIREWALL-ESP

    *Applies to:* A Swarm whose managers run the release under test, and whose workers mix nodes of
    the previous release and the release under test

    In that Swarm, the expectations of :ref:`swarm-traffic-east-west-reachability`,
    :ref:`swarm-traffic-service-discovery`, :ref:`swarm-traffic-load-balancing-east-west-vip`,
    :ref:`swarm-traffic-api-value-mode-ingress` and :ref:`swarm-lifecycle-encryption` about traffic
    between nodes hold between nodes of different versions. So do the expectations of
    :ref:`swarm-traffic-service-discovery` about a name whose workload is on another node. Each of
    those expectations keeps its own type and dependencies. They hold on cleartext and
    :drvopt:`encrypted` overlays and on the ingress network.

.. expectation:: Without VXLAN devices, a task fails with error creating vxlan interface
    :id: SWARM-WIRE-NO-VXLAN
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :pins: SWARM-HOST-VXLAN

    *Applies to:* A Linux node whose kernel does not offer VXLAN devices

    On that node, a task attached to an overlay network is rejected. The error that :api-op:`GET
    /tasks` reports for the task contains :api-error:`error creating vxlan interface`. See
    :ref:`Host requirements <swarm-networking-host-requirements>`.

.. _swarm-lifecycle-cleartext-overlay-networks:

Cleartext overlay networks
==========================

:ref:`swarm-networking-cluster-node-configuration` pins the default data-path port.
:ref:`swarm-networking-overlay-networks` pins the range of auto-allocated VNIs.

.. expectation:: Overlay traffic travels in UDP to the configured data-path port
    :id: SWARM-WIRECLEAR-UDP-PORT
    :observed_from: underlay
    :depends_on: os-linux, os-windows
    :platforms: linux, windows

    UDP datagrams carry overlay traffic between hosts, over IPv4 or IPv6, to the configured
    data-path port.

.. expectation:: Each datagram has a VXLAN header with the frame's overlay subnet VNI
    :id: SWARM-WIRECLEAR-VXLAN-VNI
    :observed_from: underlay
    :depends_on: os-linux, os-windows
    :platforms: linux, windows

    A UDP datagram that carries overlay traffic between hosts has a VXLAN header. The header has the
    VNI of the overlay subnet that the frame in the datagram is on.

.. expectation:: The VXLAN payload is an Ethernet frame of container-to-container traffic
    :id: SWARM-WIRECLEAR-ETHERNET-PAYLOAD
    :observed_from: underlay
    :depends_on: os-linux, os-windows
    :platforms: linux, windows

    The VXLAN payload of a UDP datagram that carries overlay traffic between hosts is an Ethernet
    frame that carries container-to-container traffic.

.. _swarm-lifecycle-encrypted-overlay-networks:

Encrypted overlay networks
==========================

Encrypted traffic between hosts is IPsec ESP. :ref:`swarm-lifecycle-encryption` covers the
encryption and the rest of what an encrypted network promises. The expectations below are about the
decrypted payload. The decrypted payload is the same whatever the encryption is. To see the payload,
an underlay host must decrypt it with a node's keys. This subsection holds on Linux nodes, as
:ref:`swarm-lifecycle-encryption` does.

.. expectation:: The decrypted payload is a UDP datagram to the configured data-path port
    :id: SWARM-WIREENC-UDP-PORT
    :observed_from: underlay (decrypted with a node's keys)
    :depends_on: os-linux
    :platforms: linux
    :needs: SWARM-HOST-XFRM, SWARM-HOST-IPTABLES-COMMAND

    The decrypted payload is a UDP datagram to the configured data-path port.

.. expectation:: The decrypted datagram has a VXLAN header with the frame's subnet VNI
    :id: SWARM-WIREENC-VXLAN-VNI
    :observed_from: underlay (decrypted with a node's keys)
    :depends_on: os-linux
    :platforms: linux
    :needs: SWARM-HOST-XFRM, SWARM-HOST-IPTABLES-COMMAND

    The UDP datagram in the decrypted payload has a VXLAN header. The header has the VNI of the
    overlay subnet that the frame in the datagram is on.

.. expectation:: The decrypted VXLAN payload is an Ethernet frame of container traffic
    :id: SWARM-WIREENC-ETHERNET-PAYLOAD
    :observed_from: underlay (decrypted with a node's keys)
    :depends_on: os-linux
    :platforms: linux
    :needs: SWARM-HOST-XFRM, SWARM-HOST-IPTABLES-COMMAND

    The VXLAN payload of the UDP datagram in the decrypted payload is an Ethernet frame that carries
    container-to-container traffic.

.. _swarm-lifecycle-failure-lifecycle:

*********************
 Failure & lifecycle
*********************

.. _swarm-lifecycle-restart-and-reclamation:

Restart and reclamation
=======================

.. expectation:: After a restart, GET /info reports the node active within the ceiling
    :id: SWARM-RESTART-NODE-STATE-ACTIVE
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows
    :within: SWARM-CEIL-REJOIN

    *Applies to:* A node in a Swarm

    After the node's daemon restarts, gracefully or after it is killed, :api-op:`GET /info` on the
    node reports :api-field:`Swarm.LocalNodeState` as ``active`` within the :need:`[[title|lower]]
    ceiling ([[seconds]] s) <SWARM-CEIL-REJOIN>` after the daemon's API responds again.

.. expectation:: After a graceful restart, node tasks are reachable within the ceiling
    :id: SWARM-RESTART-GRACEFUL-TASKS
    :observed_from: task
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :within: SWARM-CEIL-REJOIN

    *Applies to:* A service whose tasks are constrained to the node

    After a graceful daemon restart, the service's tasks on the node are reachable at their
    addresses on their overlay networks within the :need:`[[title|lower]] ceiling ([[seconds]] s)
    <SWARM-CEIL-REJOIN>`.

.. expectation:: After a graceful restart, published ports answer within the ceiling
    :id: SWARM-RESTART-GRACEFUL-PUBLISHED-PORTS
    :observed_from: client
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND
    :within: SWARM-CEIL-REJOIN

    After a graceful daemon restart, the node's published ports answer within the
    :need:`[[title|lower]] ceiling ([[seconds]] s) <SWARM-CEIL-REJOIN>`.

.. expectation:: No event on a third node interrupts flows between other nodes
    :id: SWARM-RESTART-OTHER-NODES
    :observed_from: task, client
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND

    *Applies to:* A flow to a task, from a client on a node to a VIP or from a client outside the
    Swarm to a port that a node publishes, and a third node that is neither that node nor the task's
    node

    No event on the third node interrupts the flow: no TCP flow is reset, and no TCP or UDP flow has
    a gap in delivery longer than 1 second. The events include a graceful restart of the third
    node's daemon, a kill and restart of it, an upgrade of its daemon from the previous release to
    the release under test, a node partition between the third node and the other nodes, the third
    node leaving the Swarm or joining it, and a change of leader manager to or from the third node.
    This holds in the Swarm of :need:`SWARM-WIRE-MIXED-VERSIONS`, whose workers mix releases, when
    the client's node and the task's node run the release under test.

.. expectation:: What a killed daemon left for overlays goes within the rejoin ceiling
    :id: SWARM-RESTART-STALE-OVERLAY
    :observed_from: node
    :depends_on: os-linux
    :platforms: linux
    :within: SWARM-CEIL-REJOIN

    *Applies to:* A Linux node in a Swarm with an ingress network, whose daemon was killed and
    started again, and a namespace, bridge or VXLAN device that the killed daemon left for an
    overlay network or the ingress network

    Within the :need:`[[title|lower]] ceiling ([[seconds]] s) <SWARM-CEIL-REJOIN>` after
    the daemon's API responds again, no such namespace, bridge or VXLAN device remains on the node.
    A namespace remains while a mount of it remains on the node.

.. expectation:: The first overlay programmed or deleted removes all stale namespaces
    :id: SWARM-RESTART-STALE-OVERLAY-REMOVAL
    :observed_from: node
    :depends_on: os-linux
    :platforms: linux
    :exact_form_of: SWARM-RESTART-STALE-OVERLAY
    :within: SWARM-CEIL-REJOIN, SWARM-CEIL-REASSIGNMENT

    *Applies to:* A Linux node whose daemon was killed

    The daemon is started again. The first time after the start that an overlay network or the
    ingress network is programmed on the node, or that the node deletes one, the daemon removes
    every namespace that the killed daemon left for an overlay network or the ingress network, with
    its bridge and VXLAN device. Until then, they stay.

.. expectation:: A start on the overlay network removes a killed daemon's HNS network
    :id: SWARM-RESTART-STALE-OVERLAY-WINDOWS
    :observed_from: node
    :depends_on: os-windows
    :platforms: windows

    *Applies to:* A Windows node whose daemon was killed and started again, and an HNS overlay
    network that the killed daemon left for an overlay network

    Once a task or standalone container on that overlay network starts on the node, the HNS network
    that the killed daemon left does not remain. See
    :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: A start on a network with an overlapping VNI removes a stale HNS network
    :id: SWARM-RESTART-STALE-OVERLAY-WINDOWS-SHARED-VNI
    :observed_from: node
    :depends_on: os-windows
    :platforms: windows

    *Applies to:* A Windows node whose daemon was killed and started again, and an HNS overlay
    network that the killed daemon left for an overlay network

    Once a task or standalone container starts on the node on another overlay network with a VNI
    that overlaps the HNS network's VNIs, the HNS network does not remain. See
    :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: A stale HNS network remains until a task starts on its network or VNI
    :id: SWARM-RESTART-STALE-OVERLAY-REMOVAL-WINDOWS
    :observed_from: node
    :depends_on: os-windows
    :platforms: windows

    *Applies to:* A Windows node whose daemon was killed and started again, and an HNS overlay
    network that the killed daemon left for an overlay network

    The HNS network remains until a task or standalone container starts on the node on that overlay
    network, or on another overlay network with a VNI that overlaps the HNS network's VNIs. After
    that, the HNS network does not remain. See :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: After a kill and restart, a network on a VNI used before passes traffic
    :id: SWARM-RESTART-KILLED-VNI-REUSE
    :observed_from: node, task
    :depends_on: os-linux, os-windows
    :platforms: linux, windows

    *Applies to:* A node that used a VNI before its daemon was killed and restarted

    A task or container on a network that is allocated that VNI starts on the node, and the network
    passes traffic. This holds whether the network is the one that used the VNI before the kill or
    another network.

.. expectation:: After a kill and restart, node tasks are reachable within the ceiling
    :id: SWARM-RESTART-KILLED-TASKS
    :observed_from: task
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :within: SWARM-CEIL-REJOIN

    *Applies to:* A service whose tasks are constrained to the node

    After the daemon is killed and restarted, the service's tasks on the node are reachable at their
    addresses on their overlay networks within the :need:`[[title|lower]] ceiling ([[seconds]] s)
    <SWARM-CEIL-REJOIN>`.

.. expectation:: After a kill and restart, published ports answer within the ceiling
    :id: SWARM-RESTART-KILLED-PUBLISHED-PORTS
    :observed_from: client
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND
    :within: SWARM-CEIL-REJOIN

    After the daemon is killed and restarted, the node's published ports answer within the
    :need:`[[title|lower]] ceiling ([[seconds]] s) <SWARM-CEIL-REJOIN>`.

.. expectation:: After a leave, no Swarm-scoped network is listed or attached on the node
    :id: SWARM-RESTART-LEAVE-NETWORKS
    :observed_from: api:solo
    :depends_on: none
    :platforms: linux, windows

    After :api-op:`POST /swarm/leave` responds 200 on a node, :api-op:`GET /networks` on that node
    does not list any Swarm-scoped network, the ingress network included. The node's standalone
    containers keep running, and none of them is attached to a Swarm-scoped network. See
    :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: On a worker, docker swarm leave removes every Swarm-scoped network
    :id: SWARM-RESTART-LEAVE-NETWORKS-CLI
    :observed_from: api:solo
    :depends_on: cli
    :platforms: linux, windows
    :cli_for: SWARM-RESTART-LEAVE-NETWORKS

    On a worker, :cli:`docker swarm leave` exits zero. :cli:`docker network ls` on that node then
    does not list any Swarm-scoped network, the ingress network included.

.. _swarm-lifecycle-convergence:

Convergence
===========

There are five convergence classes, each with a different dominant term. One bound cannot serve all
of them.

Each class has a ceiling. The ceiling is chosen by judgment to encode "a human would call this
broken". It is never re-baselined. This keeps the ceiling free of flakes. The ceiling still catches
the regression that matters: convergence that goes from seconds to minutes, or to never.

.. ceiling:: Reassignment
    :id: SWARM-CEIL-REASSIGNMENT
    :seconds: 300

    An address moving to another endpoint, a task replaced by one on another node, or a new task,
    service or endpoint.

    Its dominant term is gossip propagation and FDB reprogramming.

    The change is complete when the new endpoint or task is running.

.. ceiling:: Partition heal
    :id: SWARM-CEIL-PARTITION-HEAL
    :seconds: 180

    Its dominant term is reconnection to the failed peers, then full-state syncs.

    The change is complete when the partition is removed.

.. ceiling:: Node leave and rejoin
    :id: SWARM-CEIL-REJOIN
    :seconds: 120

    Its dominant term is full cluster re-formation and network re-programming.

    The change is complete when the rejoined node's task is running. For a rejoined node that runs
    no task, the change is complete when :api-op:`POST /swarm/join` responds.

.. ceiling:: Allocation
    :id: SWARM-CEIL-ALLOCATION
    :seconds: 30

    A Swarm-scoped network that can be allocated, or the endpoint (VIPs and published ports) of a
    service whose networks are allocated. Also the leader's failed attempt to allocate a network or
    a service's endpoint, and what follows from it.

    Its dominant term is the allocator of the leader manager.

    For allocation, the change is complete when :api-op:`POST /networks/create`, :api-op:`POST
    /services/create` or :api-op:`POST /services/{id}/update` responds.

.. ceiling:: Failure detection
    :id: SWARM-CEIL-FAILURE-DETECTION
    :seconds: 120

    The nodes on one side of a partition declare the nodes on the other side failed.

    Its dominant term is gossip probes and suspicion timeouts.

    For failure detection, the change is complete when the partition starts.

Each ceiling starts when the change is complete. For any other effect of :api-op:`POST
/services/{id}/update`, the change is complete when :api-op:`GET /services/{id}` reports
:api-field:`UpdateStatus.State` as ``completed`` for that update. An update of a service's endpoint
mode, networks, published ports or aliases replaces its tasks and sets :api-field:`UpdateStatus`. An
update that does not replace a task is complete when the request responds. For a removal, the change
is complete when the request that removes the object responds. For a restart of a node's daemon, the
change is complete when the daemon's API responds again and :api-op:`GET /info` on the node reports
:api-field:`Swarm.LocalNodeState` as ``active``. How long a node takes to be ``active`` again is
bounded too: an expectation of :ref:`swarm-lifecycle-restart-and-reclamation` times it from when the
daemon's API responds again. For a change in a task's health, or the exit of its container, the
change is complete when the task's node emits the container's :api-event:`health_status` or
:api-event:`die` event. For a change of a task's desired state away from ``running``, the change is
complete when :api-op:`GET /tasks` first reports the new :api-field:`DesiredState`. For a change of
leader, the change is complete when :api-op:`GET /nodes` on a manager reports
:api-field:`ManagerStatus.Leader` as true for the new leader. For an expectation about what holds
once no container of a service is running, the change is complete when the last of those containers
emits its :api-event:`die` event. For an expectation about a removal that reaches a node late, the
ceiling starts when the removal can reach the node.

A node partition divides the nodes of a Swarm into two sides, and blocks all traffic between the
nodes of one side and the nodes of the other, in both directions. The managers that hold a quorum
are on one side. During a node partition, consider each network to which nodes of both sides have a
local attachment. A side has declared the other side's nodes failed once, for each such network, no
node of the side lists a node of the other side in :api-field:`Peers` in :api-op:`GET
/networks/{id}`.

Unless an expectation says otherwise, each end of the traffic in the expectations below can be a
service task or a standalone container on an ``attachable`` overlay. This applies in all four
combinations. Each expectation states the node placements it covers. In the expectations about a
node partition, an end on the side without the managers' quorum is a standalone container, because
the orchestrator replaces the tasks on that side.

.. expectation:: A moved overlay address is reachable at its new endpoint in the ceiling
    :id: SWARM-CONVERGE-ADDRESS-MOVE
    :observed_from: task, ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :within: SWARM-CEIL-REASSIGNMENT

    After overlay address X moves from endpoint S1 to S2, a client that could reach X at S1 reaches
    it at S2. The client does this within the reassignment ceiling, without a manual :program:`ip
    neigh`, :program:`arp -d` or host flush. This holds whether S2 is on S1's node or on another
    node. It also holds whether the client is on S1's node, S2's node or a third node.

.. expectation:: Repeated moves of an address converge, even when learned out of order
    :id: SWARM-CONVERGE-ADDRESS-MOVE-REPEATED
    :observed_from: task, ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :within: SWARM-CEIL-REASSIGNMENT

    Overlay address X moves any number of times between endpoints. Within the reassignment ceiling,
    a client that could reach X at its previous endpoint reaches X at its current endpoint. This
    also holds on a node that learns of the removal of X's previous endpoint only after it learned
    of X's new endpoint. That node can be the node of the new endpoint or any other node.

.. expectation:: A moved address is reachable whether or not its MAC address moves too
    :id: SWARM-CONVERGE-ADDRESS-MOVE-MAC
    :observed_from: task, ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :within: SWARM-CEIL-REASSIGNMENT

    Overlay address X moves between endpoints. Within the reassignment ceiling, a client that could
    reach X at its previous endpoint reaches X at its new endpoint. This holds whether or not the
    MAC address moves with X.

.. expectation:: A client reaches a task's replacement on another node within the ceiling
    :id: SWARM-CONVERGE-TASK-REPLACED
    :observed_from: task, ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND
    :within: SWARM-CEIL-REASSIGNMENT

    When a task is replaced by one on another node, a client that could reach the old task reaches
    the new one. It reaches the new task at the address that :api-op:`GET /tasks` reports for it, at
    the address that :dns-name:`tasks.<service>` resolves to, and, in vip mode, through the
    service's VIP. The client does this within the reassignment ceiling, without manual
    intervention. This holds whether the client is on the old task's node, the new task's node or a
    third node.

.. expectation:: Each side of a partition declares the other failed within the ceiling
    :id: SWARM-CONVERGE-PARTITION-DETECTION
    :observed_from: api:mgr, api:wkr
    :depends_on: none
    :platforms: linux, windows
    :within: SWARM-CEIL-FAILURE-DETECTION

    During a node partition, each side declares the other side's nodes failed within the
    failure-detection ceiling.

.. expectation:: Nodes hold no neighbor or FDB entries for a failed side's endpoints
    :id: SWARM-CONVERGE-PARTITION-NO-ENTRIES
    :observed_from: node
    :depends_on: os-linux
    :platforms: linux

    *Applies to:* Linux nodes on one side of a node partition, which have declared the nodes on the
    other side failed

    Those nodes do not hold overlay neighbor or FDB entries for the other side's endpoints.

.. expectation:: On the quorum side of a partition, VIP flows reach that side's tasks
    :id: SWARM-CONVERGE-PARTITION-QUORUM-VIP
    :observed_from: task, ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND

    *Applies to:* A node partition in which the side with the managers' quorum has declared the
    other side's nodes failed, and a service with running tasks on both sides

    Every flow that starts on a node of that side and goes to the service's VIP reaches a task on
    that side.

.. expectation:: A VXLAN device holds no neighbor or FDB entry for its node's endpoints
    :id: SWARM-CONVERGE-NO-OWN-ENTRIES
    :observed_from: node
    :depends_on: os-linux
    :platforms: linux

    A node's VXLAN device does not hold a neighbor or FDB entry for any of the node's own endpoints.

.. expectation:: Reachability converges within the ceiling after a partition heals
    :id: SWARM-CONVERGE-PARTITION-HEAL
    :observed_from: task, ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :within: SWARM-CEIL-PARTITION-HEAL

    *Applies to:* A node partition in which each side has declared the other side's nodes failed

    After the partition heals, reachability between the two sides converges within the
    partition-heal ceiling.

.. expectation:: Reachability restored after a partition heals outlasts FDB aging
    :id: SWARM-CONVERGE-PARTITION-HEAL-PERSISTS
    :observed_from: task, ctr
    :depends_on: os-linux
    :platforms: linux

    *Applies to:* A node partition between Linux nodes, in which each side has declared the other
    side's nodes failed

    Reachability that is restored when the partition heals persists. A connection opened from either
    side still reaches the other side's containers after they stay silent for longer than the
    kernel's learned-FDB aging time. That aging time is 300 s by default.

.. expectation:: Reachability restored while containers send during the heal persists
    :id: SWARM-CONVERGE-PARTITION-HEAL-PERSISTS-ACTIVE
    :observed_from: task, ctr
    :depends_on: os-linux
    :platforms: linux

    *Applies to:* A node partition between Linux nodes, in which each side has declared the other
    side's nodes failed

    Containers on both sides send traffic as the partition heals. The reachability that is restored
    then persists. A connection opened from either side still reaches the other side's containers
    after they stay silent for longer than the kernel's learned-FDB aging time.

.. expectation:: After a leave and rejoin, new tasks are reachable within the ceiling
    :id: SWARM-CONVERGE-REJOIN-TASKS
    :observed_from: task, ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :within: SWARM-CEIL-REJOIN

    A node leaves the Swarm with :api-op:`POST /swarm/leave` and rejoins with :api-op:`POST
    /swarm/join`. Within the leave-and-rejoin ceiling, the tasks placed on the node after the rejoin
    are reachable at their overlay addresses from that node and from other nodes. Such tasks include
    a global service's tasks.

.. expectation:: After a leave and rejoin, published ports answer within the ceiling
    :id: SWARM-CONVERGE-REJOIN-PUBLISHED-PORTS
    :observed_from: client
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND
    :within: SWARM-CEIL-REJOIN

    After a node leaves the Swarm with :api-op:`POST /swarm/leave` and rejoins with :api-op:`POST
    /swarm/join`, its published ports answer within the leave-and-rejoin ceiling.

.. expectation:: After a rejoin, published ports on any node reach the node's new tasks
    :id: SWARM-CONVERGE-REJOIN-INGRESS-BACKENDS
    :observed_from: client
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND
    :within: SWARM-CEIL-REJOIN

    A node leaves the Swarm with :api-op:`POST /swarm/leave` and rejoins with :api-op:`POST
    /swarm/join`. Within the leave-and-rejoin ceiling, connections to a service's published port on
    any node reach the service's tasks placed on the rejoined node after the rejoin. An example is a
    global service's tasks.

.. expectation:: After a node leaves, new VIP and port flows reach the remaining tasks
    :id: SWARM-CONVERGE-LEAVE-REMAINING-TASKS
    :observed_from: task, ctr, client
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND
    :within: SWARM-CEIL-REJOIN

    *Applies to:* A service with a running task on each of two or more nodes

    One of those nodes leaves the Swarm with :api-op:`POST /swarm/leave`. Within the
    leave-and-rejoin ceiling after the leave, every new flow to the service's VIP from a node still
    in the Swarm reaches a running task on a node still in the Swarm. So does every new flow to the
    service's published port on such a node.

.. expectation:: Other nodes hold no neighbor or FDB entries for a node that left
    :id: SWARM-CONVERGE-LEAVE-NO-ENTRIES
    :observed_from: node
    :depends_on: os-linux
    :platforms: linux
    :within: SWARM-CEIL-REJOIN

    Within the leave-and-rejoin ceiling after :api-op:`POST /swarm/leave` responds 200 on a node,
    the other nodes do not hold overlay neighbor or FDB entries for its endpoints.

In the expectations of this subsection, a node's overlay neighbor and FDB entries are the permanent
entries that the daemon programs. Entries that the kernel learns from traffic are not included. The
persistence contract holds whether or not containers send traffic as the partition heals. A bug
revealed the persistence contract. In that bug, reachability after a heal depended on an entry that
the kernel learned from the peer's traffic. Reachability broke when that entry expired. The fix is
in `moby/moby#53663 <https://github.com/moby/moby/pull/53663>`__. The expectation names ``os``
because how learned and permanent FDB entries interact is behavior of the kernel's VXLAN driver.

.. _swarm-lifecycle-churn:

Churn
=====

This paragraph applies to every expectation below in which a client reaches or resolves a service.
Each such expectation holds whether the client is a service task or a standalone container on an
``attachable`` overlay. It holds for a client on the same node as one of the service's tasks. It
also holds for a client on a node that runs none of the service's tasks. The service's own end is
always its tasks.

.. expectation:: During a rolling update, the VIP's outages never exceed the ceiling
    :id: SWARM-CHURN-ROLLING-UPDATE-VIP
    :observed_from: task, ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND
    :within: SWARM-CEIL-ALLOCATION

    A service has a rolling update in which at least one task keeps running throughout. Examples are
    an update in start-first order and an update of fewer tasks at a time than are running. During
    the update, the VIP does not stop answering for longer than the allocation ceiling: within any
    interval of that length, some new flow to the VIP succeeds. See
    :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: After a rolling update, new flows reach running tasks within the ceiling
    :id: SWARM-CHURN-ROLLING-UPDATE-COMPLETE
    :observed_from: task, ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND
    :within: SWARM-CEIL-REASSIGNMENT

    A service has a rolling update in which at least one task keeps running throughout. Examples are
    an update in start-first order and an update of fewer tasks at a time than are running. Once the
    update completes, every new flow reaches a running task within the reassignment ceiling. See
    :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: After rapid scaling between 1 and 5, VIP flows reach running tasks
    :id: SWARM-CHURN-SCALE-FLAPPING
    :observed_from: task, ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND
    :within: SWARM-CEIL-REASSIGNMENT

    A service's replica count is changed at least 10 times, alternating between 1 and 5. Each update
    is sent as soon as the previous update responds. Once the scaling stops, every new flow to the
    service's VIP reaches a running task within the reassignment ceiling. See
    :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: Networks left by a racing delete and create can each be deleted
    :id: SWARM-CHURN-DELETE-CREATE
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A Swarm-scoped network

    :api-op:`DELETE /networks/{id}` for the network, by its ID, runs concurrently with :api-op:`POST
    /networks/create` for a network with the same name and driver. Afterwards, :api-op:`DELETE
    /networks/{id}` for each network with that name that remains responds 204, and :api-op:`GET
    /networks` then does not list it.

.. expectation:: After a racing delete and create, the name can be created again
    :id: SWARM-CHURN-DELETE-CREATE-RECREATE
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A Swarm-scoped network

    :api-op:`DELETE /networks/{id}` for the network, by its ID, runs concurrently with :api-op:`POST
    /networks/create` for a network with the same name and driver. Once no network with that name
    remains, :api-op:`POST /networks/create` with that name responds 201, and :api-op:`GET
    /networks` then lists the network.

.. expectation:: A delete racing a container start fails, or succeeds and the start fails
    :id: SWARM-CHURN-DELETE-CTR-START
    :observed_from: api:mgr, api:active
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A created or stopped standalone container on a Swarm-scoped attachable network

    :api-op:`DELETE /networks/{id}` on a manager for that network races :api-op:`POST
    /containers/{id}/start` for the container. One of two outcomes occurs. In the first, the delete
    responds with a 4xx or 5xx status, and the container starts attached to the network. In the
    second, the delete responds 204, :api-op:`GET /networks` on a manager then does not list the
    network, and the start fails.

.. expectation:: When a racing delete succeeds, the container start responds 4xx or 5xx
    :id: SWARM-CHURN-DELETE-CTR-START-REFUSED
    :observed_from: api:mgr, api:active
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A created or stopped standalone container on a Swarm-scoped attachable network

    :api-op:`DELETE /networks/{id}` on a manager for that network races :api-op:`POST
    /containers/{id}/start` for the container. When the delete responds 204, the start responds with
    a 4xx or 5xx status.

.. expectation:: When a racing delete succeeds, the container start responds 500
    :id: SWARM-CHURN-DELETE-CTR-START-REFUSED-HTTP500
    :observed_from: api:mgr, api:active
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A created or stopped standalone container on a Swarm-scoped attachable network

    :api-op:`DELETE /networks/{id}` on a manager for that network races :api-op:`POST
    /containers/{id}/start` for the container. When the delete responds 204, the start gives one of
    two responses. The first is 500 at once, with a message that contains :api-error:`network <ref>
    not found`. ``<ref>`` is the network's full ID if the container has been started before.
    Otherwise it is the network name or ID that the container was created with. The second is 500
    after 20 seconds, with :api-error:`attaching to network failed, make sure your network options
    are correct and check manager logs: context deadline exceeded`. See
    :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: A delete racing the last task's shutdown fails or removes the network
    :id: SWARM-CHURN-DELETE-SHUTDOWN
    :observed_from: api:mgr, api:active
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A node with exactly one task or container on a network

    :api-op:`DELETE /networks/{id}` on a manager for that network races the shutdown of that task or
    container. For a task, the shutdown comes from the removal of its service, or from an update
    that removes the network from the service's :api-field:`TaskTemplate.Networks`. For a container,
    it comes from :api-op:`POST /containers/{id}/stop`. One of two outcomes occurs. In the first,
    the delete responds with a 4xx or 5xx status, and the network stays listed. In the second, the
    delete responds 204, and once the shutdown completes, :api-op:`GET /networks` on that node does
    not list the network.

.. expectation:: docker network rm racing docker run fails, or succeeds and run fails
    :id: SWARM-CHURN-DELETE-CTR-START-CLI
    :observed_from: api:mgr, api:active
    :depends_on: cli
    :platforms: linux, windows
    :cli_for: SWARM-CHURN-DELETE-CTR-START

    When :cli:`docker network rm` on a manager races :cli:`docker run -d --network` naming the
    network, one of two outcomes occurs. In the first, :cli:`docker network rm` exits non-zero, and
    the container runs attached to the network. In the second, :cli:`docker network rm` exits zero,
    :cli:`docker network ls` on a manager then does not list the network, and :cli:`docker run -d`
    exits non-zero.

.. expectation:: After a change from vip to dnsrr, the service name resolves to its tasks
    :id: SWARM-CHURN-VIP-TO-DNSRR-RESOLVES
    :observed_from: task, ctr
    :depends_on: none
    :platforms: linux, windows

    After :api-op:`POST /services/{id}/update` changes a service's endpoint mode from ``vip`` to
    ``dnsrr``, :dns-name:`<service>` resolves to the addresses of the running tasks.

.. expectation:: After a change from vip to dnsrr, the old VIP does not reach the service
    :id: SWARM-CHURN-VIP-TO-DNSRR-OLD-VIP
    :observed_from: task, ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows

    After a service's endpoint mode changes from ``vip`` to ``dnsrr``, its old VIP does not reach
    the service.

.. expectation:: After a change from dnsrr to vip, the service name resolves to its VIP
    :id: SWARM-CHURN-DNSRR-TO-VIP-RESOLVES
    :observed_from: task, ctr
    :depends_on: none
    :platforms: linux, windows

    After :api-op:`POST /services/{id}/update` changes a service's endpoint mode from ``dnsrr`` to
    ``vip``, :dns-name:`<service>` resolves to the service VIP.

.. expectation:: After a change from dnsrr to vip, the VIP load-balances across the tasks
    :id: SWARM-CHURN-DNSRR-TO-VIP-BALANCES
    :observed_from: task, ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND

    After a service's endpoint mode changes from ``dnsrr`` to ``vip``, the service's VIP
    load-balances across the service's tasks.

.. expectation:: A service is reachable on a network that an update adds to it
    :id: SWARM-CHURN-NETWORK-ADD
    :observed_from: task, ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND

    *Applies to:* A service, and an overlay network that the service is not attached to

    After :api-op:`POST /services/{id}/update` adds the network to
    :api-field:`TaskTemplate.Networks`, the service is reachable on that network.

.. expectation:: A service is not reachable on a network that an update removes from it
    :id: SWARM-CHURN-NETWORK-REMOVE
    :observed_from: task, ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows

    *Applies to:* A service attached to a Swarm-scoped network

    After :api-op:`POST /services/{id}/update` removes the network from
    :api-field:`TaskTemplate.Networks`, the service is not reachable on that network.

.. expectation:: A service reports no VIP on a network that an update removes
    :id: SWARM-CHURN-NETWORK-REMOVE-VIP
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A service attached to a Swarm-scoped network

    After :api-op:`POST /services/{id}/update` removes the network from
    :api-field:`TaskTemplate.Networks`, :api-op:`GET /services/{id}` does not report a VIP on that
    network.

.. expectation:: docker service update --network-add adds the network to the service
    :id: SWARM-CHURN-NETWORK-ADD-SPEC-CLI
    :observed_from: api:mgr
    :depends_on: cli
    :platforms: linux, windows
    :cli_for: SWARM-CHURN-NETWORK-ADD

    *Applies to:* A service, and a Swarm-scoped network that the service is not attached to

    :cli:`docker service update --network-add` naming that network, for the service, exits zero.
    :cli:`docker service inspect` then prints the network in :api-field:`TaskTemplate.Networks`.

.. expectation:: docker service update --network-rm removes the network from the service
    :id: SWARM-CHURN-NETWORK-REMOVE-SPEC-CLI
    :observed_from: api:mgr
    :depends_on: cli
    :platforms: linux, windows
    :cli_for: SWARM-CHURN-NETWORK-REMOVE

    *Applies to:* A service attached to a Swarm-scoped network

    :cli:`docker service update --network-rm` naming that network, for the service, exits zero.
    :cli:`docker service inspect` then does not print the network in
    :api-field:`TaskTemplate.Networks`.

.. _swarm-lifecycle-scale-and-exhaustion:

Scale and exhaustion
====================

.. expectation:: After create-delete cycles, a deleted network's VNI can be requested
    :id: SWARM-SCALE-VNI-REUSE
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    After at least 10 cycles in which an overlay network is created and deleted, :api-op:`POST
    /networks/create` for an overlay network, with a :drvopt:`vxlanid_list` that has a VNI that a
    deleted network used, responds 201. :api-op:`GET /networks/{id}` on a manager then reports that
    VNI in :api-field:`Options`.

.. expectation:: A removed service's ingress published port can be published again
    :id: SWARM-SCALE-INGRESS-PORT-REUSE
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A Swarm with a service that publishes a port in ingress mode

    After the service is removed, :api-op:`POST /services/create` for another service that publishes
    the same port responds 201. :api-op:`GET /services/{id}` then reports that port.

.. expectation:: Allocated VNIs differ from the VNIs in other networks' vxlanid_list
    :id: SWARM-SCALE-VNI-AUTO-DISTINCT
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    An overlay network is created without :drvopt:`vxlanid_list`. The VNIs allocated to the network
    differ from the VNIs in the :drvopt:`vxlanid_list` of each other overlay network that exists
    when the network is created.

.. expectation:: At least 100 overlay networks pass traffic between two nodes
    :id: SWARM-SCALE-MANY-NETWORKS
    :observed_from: task
    :depends_on: os-linux, os-windows
    :platforms: linux, windows

    At least 100 overlay networks, each with a task on each of two nodes, all pass traffic between
    the nodes.

.. expectation:: Creating an overlay network that cannot be allocated responds 201
    :id: SWARM-SCALE-UNALLOCATABLE-CREATE
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    :api-op:`POST /networks/create` for an overlay network that cannot be allocated responds 201. An
    example is a network whose address pool is exhausted. :api-op:`GET /networks` on a manager then
    lists the network. See :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: docker network create for an unallocatable overlay network exits zero
    :id: SWARM-SCALE-UNALLOCATABLE-CREATE-CLI
    :observed_from: api:mgr
    :depends_on: cli
    :platforms: linux, windows
    :cli_for: SWARM-SCALE-UNALLOCATABLE-CREATE

    :cli:`docker network create` for an overlay network that cannot be allocated exits zero, and
    :cli:`docker network ls` on a manager then lists the network.

.. expectation:: The leader logs an error with Failed allocation for network and its ID
    :id: SWARM-SCALE-UNALLOCATABLE-LOG
    :observed_from: log
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* An overlay network that cannot be allocated

    The daemon log of the leader manager reports the failure at error level, in an entry that
    contains :log:`Failed allocation for network` and the network's ID. On a manager that just
    became leader, the entry can contain :log:`failed allocating network` and the network's ID in
    place of that text.

.. expectation:: An unallocatable network inspects with empty Driver, IPAM and Options
    :id: SWARM-SCALE-UNALLOCATABLE-INSPECT
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* An overlay network that cannot be allocated

    On a manager, :api-op:`GET /networks/{id}` for that network responds 200 with an empty
    :api-field:`Driver`, :api-field:`IPAM` and :api-field:`Options`, and without
    :api-field:`Status`. This is so even when :api-op:`POST /networks/create` named a driver or a
    subnet. The response does not report the failure in any field. See
    :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: A container start on an unallocatable network responds 500 after 20 s
    :id: SWARM-SCALE-UNALLOCATABLE-CTR-START
    :observed_from: api:active
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* An attachable overlay network that cannot be allocated, and a created or stopped
    container on that network, on any node

    :api-op:`POST /containers/{id}/start` for the container responds 500 after 20 seconds with
    :api-error:`attaching to network failed, make sure your network options are correct and check
    manager logs: context deadline exceeded`. See
    :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: docker run on an unallocatable attachable network exits non-zero
    :id: SWARM-SCALE-UNALLOCATABLE-CTR-START-CLI
    :observed_from: api:active
    :depends_on: cli
    :platforms: linux, windows

    *Applies to:* An attachable overlay network that cannot be allocated

    On any node, :cli:`docker run -d --network` naming that network exits non-zero and prints the
    daemon's error.

.. expectation:: A service on an unallocatable network is created but its tasks stay new
    :id: SWARM-SCALE-UNALLOCATABLE-SERVICE
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* An overlay network that cannot be allocated

    :api-op:`POST /services/create` for a service on that network responds 201. The service's tasks
    stay in the ``new`` state, with desired state ``running``, and do not start. See
    :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: The leader logs Failed allocation for service on unallocatable networks
    :id: SWARM-SCALE-UNALLOCATABLE-SERVICE-LOG
    :observed_from: log
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* An overlay network that cannot be allocated

    After :api-op:`POST /services/create` for a service on that network responds 201, the daemon log
    of the leader manager reports the failure at error level. The entry contains :log:`Failed
    allocation for service` and the service's ID. See
    :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: Services publishing one port number in different protocols both answer
    :id: SWARM-SCALE-PORT-PROTOCOLS
    :observed_from: client
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND

    *Applies to:* A Swarm with a service that publishes a port

    After a new service publishes the same port number under a different protocol, both services are
    reachable on their published ports.

.. expectation:: Removing one of two same-port services leaves the other reachable
    :id: SWARM-SCALE-PORT-PROTOCOLS-REMOVAL
    :observed_from: client
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND

    *Applies to:* A Swarm with two services that publish the same port number under different
    protocols

    When one of the services is removed, the other service stays reachable on its published port.
    This holds throughout the removal and after it.

.. _swarm-lifecycle-firewall-backend-transition:

Firewall-backend transition
===========================

A switch of a node in a Swarm to or from the nftables backend is not asserted
(:ref:`swarm-lifecycle-known-divergences-non-goals`).

.. _swarm-lifecycle-failure-reporting:

Failure reporting
=================

When a node cannot program cluster-scoped configuration, no API surface reports it. Cluster-scoped
configuration is what a node programs for the Swarm outside the network namespace of any one task or
container. Examples are encryption parameters for a peer, a load-balancer backend, the firewall
rules that publish an ingress port, and an FDB entry. The affected service continues to look healthy
from ``api:mgr`` while that node's data plane is wrong. The daemon log is the only channel that
reports the failure. This makes the daemon log part of the observable contract. A failure to add a
task's ingress rules in its own network namespace is not such a failure: the task does not start,
and :api-op:`GET /tasks` reports the error (:ref:`swarm-traffic-api-value-mode-ingress`). A failure
to add or remove a VIP alias on a load-balancer endpoint, or to find that endpoint's interface, is a
failure to program the load-balancer service. Under the iptables backend, so is a failure to add the
service's firewall-mark rule.

.. expectation:: A node-local load-balancer programming failure is logged at error level
    :id: SWARM-FAILREP-LB
    :observed_from: log
    :depends_on: none
    :platforms: linux, windows

    The daemon log of a node reports a node-local failure to program a load-balancer service or
    backend at error level. See :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: A Linux load-balancer failure entry contains one of seven messages
    :id: SWARM-FAILREP-LB-MSG-IPT
    :observed_from: log
    :depends_on: none
    :platforms: linux

    A daemon log entry for a node-local failure to program a load-balancer service or backend
    contains :log:`Failed to create an ipvs handle`, :log:`Failed find interface name for endpoint`,
    :log:`Failed add IP alias`, :log:`Failed to remove IP alias`, :log:`Failed to add firewall mark
    rule`, :log:`Failed to create a new service for vip` or :log:`Failed to create real server`.

.. expectation:: A Windows load-balancer failure entry has Failed to add ILB/ELB policy
    :id: SWARM-FAILREP-LB-MSG-WINDOWS
    :observed_from: log
    :depends_on: none
    :platforms: windows

    A daemon log entry for a node-local failure to program a load-balancer service or backend
    contains :log:`Failed to add ILB policy` or :log:`Failed to add ELB policy`.

.. expectation:: A node-local failure to publish an ingress port is logged at error level
    :id: SWARM-FAILREP-INGRESS-PORT
    :observed_from: log
    :depends_on: none
    :platforms: linux

    The daemon log of a node reports a node-local failure to publish an ingress port at error level.

.. expectation:: A failure to publish an ingress port in use is logged with the port
    :id: SWARM-FAILREP-INGRESS-PORT-IN-USE
    :observed_from: log
    :depends_on: none
    :platforms: linux

    *Applies to:* A Linux node where a port is in use by a process or container, and a service that
    publishes that port in ingress mode

    The daemon log of the node reports the node-local failure to publish that port at error level.
    The entry contains the port number.

.. expectation:: The daemon logs that it failed to publish a service's ingress ports
    :id: SWARM-FAILREP-INGRESS-PORT-MSG
    :observed_from: log
    :depends_on: none
    :platforms: linux
    :exact_form_of: SWARM-FAILREP-INGRESS-PORT
    :within: SWARM-CEIL-REASSIGNMENT

    The daemon log entry for a node-local failure to publish an ingress port contains :log:`Failed
    to publish this service's Swarm ingress ports on this node`.

.. expectation:: A failed overlay peer entry is logged as a warning with its addresses
    :id: SWARM-FAILREP-PEER
    :observed_from: log
    :depends_on: none
    :platforms: linux

    The daemon log of a node reports a node-local failure to program an overlay peer's FDB or
    neighbor entry at warning level. The entry contains the network's ID, the address of the peer's
    endpoint, and the data-path address of the peer's node.

.. expectation:: A failed overlay peer entry logs overlay: failed to add peer entry
    :id: SWARM-FAILREP-PEER-MSG
    :observed_from: log
    :depends_on: none
    :platforms: linux

    The daemon log entry for a node-local failure to program an overlay peer's FDB or neighbor entry
    contains :log:`overlay: failed to add peer entry`. Its ``nid`` field is the network's full ID.

.. expectation:: A failure to program an IPsec SA or policy is logged as a warning
    :id: SWARM-FAILREP-IPSEC
    :observed_from: log
    :depends_on: none
    :platforms: linux

    The daemon log of a node reports a node-local failure to program an IPsec security association
    or policy for a peer. The entry is at warning level. For a security association, it contains
    :log:`Failed Adding fSA{` or :log:`Failed Adding rSA{`. For a policy, it contains :log:`Adding
    fSP{`. See :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: Each new task retries the load balancer and logs the failure again
    :id: SWARM-FAILREP-LB-REPEATED
    :observed_from: log
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A node where a node-local failure to program the load-balancer service or a
    backend for a service occurred, and its cause still holds

    A task is then added to the service. The node tries again to program the load-balancer service
    or backend for that task, and the daemon log of the node reports the failure again. The report
    is not suppressed after the first one.

.. expectation:: After a load-balancer failure clears, a new task gets the node's flows
    :id: SWARM-FAILREP-LB-RECOVERY
    :observed_from: task, client
    :depends_on: none
    :platforms: linux, windows
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND

    *Applies to:* A node where a node-local failure to program the load-balancer service or a
    backend for a service occurred

    The cause of the failure clears. A task that is then added to the service receives new flows to
    the service that start on that node. This holds when the failure was to program the
    load-balancer service, and when it was to program a backend. See
    :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: After a load-balancer failure clears, a new task gets no failure logged
    :id: SWARM-FAILREP-LB-RECOVERY-LOG
    :observed_from: log
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A node where a node-local failure to program the load-balancer service or a
    backend for a service occurred

    The cause of the failure clears. A task is then added to the service. The daemon log of that
    node does not report a failure for that task.

.. expectation:: Tasks on a node that cannot program a service are reported running
    :id: SWARM-FAILREP-UNPROGRAMMED-TASKS
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A node that cannot program cluster-scoped configuration for a service

    :api-op:`GET /tasks` reports the node's tasks for the service as ``running``.

.. expectation:: A service reports all tasks running despite a node-local failure
    :id: SWARM-FAILREP-UNPROGRAMMED-SERVICE
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A node that cannot program cluster-scoped configuration for a service

    :api-op:`GET /services?status=true` at API version 1.41 or later reports the service's
    :api-field:`RunningTasks` equal to its :api-field:`DesiredTasks`.

.. expectation:: docker service ps --no-trunc lists tasks with their states and errors
    :id: SWARM-FAILREP-SERVICE-PS
    :observed_from: api:mgr
    :depends_on: cli
    :platforms: linux, windows

    :cli:`docker service ps --no-trunc` for a service lists the service's tasks, with their states
    and errors.

.. expectation:: docker service ls shows all replicas running despite a local failure
    :id: SWARM-FAILREP-UNPROGRAMMED-SERVICE-CLI
    :observed_from: api:mgr
    :depends_on: cli
    :platforms: linux, windows
    :cli_for: SWARM-FAILREP-UNPROGRAMMED-SERVICE

    *Applies to:* A node that cannot program cluster-scoped configuration for a service

    :cli:`docker service ls` prints the service's replicas as all running.

:ref:`swarm-lifecycle-known-divergences-non-goals` lists four gaps in this reporting and recovery as
known divergences:

- An unchanged condition is not reported again.
- Recovery is not reported at all.
- None of the load-balancer entries names the service.
- Recovery is incomplete.

.. _swarm-lifecycle-firewall-anchors-host-integration:

*************************************
 Firewall anchors & host integration
*************************************

The content of Moby's firewall rules is not an interface. A backend change replaces exactly that
content. :ref:`bridge-networking-rule-content` pins the bridge driver's rules with detectors only,
the rules for Swarm ingress ports included. This section covers the overlay driver's own firewall
state. It also covers how Swarm traffic meets the bridge driver's firewall anchors.
:doc:`bridge-networking.md <bridge-networking>` specifies those anchors, :ipt-chain:`DOCKER-USER`
among them. This section holds on Linux nodes.

.. _swarm-lifecycle-nftables:

nftables
========

The overlay driver's nftables table on a node in a Swarm is not asserted
(:ref:`swarm-lifecycle-known-divergences-non-goals`).

.. _swarm-lifecycle-iptables:

iptables
========

The expectations in this subsection hold under the iptables backend.
:ref:`bridge-networking-ipt-chain-docker-user` specifies the :ipt-chain:`DOCKER-USER` chain itself.
Swarm ingress ports do not have a chain of their own. The bridge driver's anchors also cover them
because the bridge driver publishes them like any other published port.

.. expectation:: No daemon rule accepts Swarm published-port traffic before DOCKER-USER
    :id: SWARM-IPT-DOCKER-USER
    :observed_from: client
    :depends_on: os-linux
    :platforms: linux
    :needs: SWARM-HOST-IPTABLES-COMMAND

    Traffic to and from a Swarm published port, in ingress mode and :api-value:`mode=host`, is
    evaluated by an operator's :ipt-chain:`DOCKER-USER` rules. No rule that the daemon programs
    accepts that traffic before the :ipt-chain:`DOCKER-USER` chain.

.. expectation:: An INPUT rule accepting VXLAN lets no cleartext into encrypted networks
    :id: SWARM-IPT-ENCRYPTED-INPUT-ACCEPT
    :observed_from: underlay → task or ctr
    :depends_on: os-linux
    :platforms: linux
    :needs: SWARM-HOST-XFRM, SWARM-HOST-IPTABLES-COMMAND

    *Applies to:* A node where an encrypted network is programmed, and an operator's rule at the end
    of the filter table's :ipt-chain:`INPUT` chain that accepts incoming VXLAN traffic

    Cleartext VXLAN for that network's VNI that arrives at the node does not reach the network's
    tasks or containers.

.. _swarm-lifecycle-cross-generation-cleanup:

Cross-generation cleanup
========================

This subsection covers the upgrade path that a packager exercises. A node that runs release *N−1* is
upgraded to *N*, possibly with a different firewall backend. A generation is the firewall state that
one release creates under one firewall backend. A previous generation is such state that a daemon
left on the node before the current start of the daemon.
:ref:`bridge-networking-cross-generation-cleanup` gives the daemon-wide expectations and the reason
why cleanup depends on names. The expectations here cover state that belongs to Swarm and the
overlay driver.

.. expectation:: The daemon removes the DOCKER-INGRESS chain an old release left at start
    :id: SWARM-CLEAN-INGRESS-CHAIN
    :observed_from: node
    :depends_on: os-linux
    :platforms: linux

    *Applies to:* A node with the state that a daemon of release 28.0.0 or earlier left with the
    iptables backend: a :ipt-chain:`DOCKER-INGRESS` chain in the filter and nat tables, the rule
    :ipt-target:`-m addrtype --dst-type LOCAL -j DOCKER-INGRESS` in the nat table's
    :ipt-chain:`PREROUTING` and :ipt-chain:`OUTPUT` chains, the rule :ipt-target:`-j DOCKER-INGRESS`
    in the filter table's :ipt-chain:`FORWARD` chain, and the rule :ipt-target:`-j
    DOCKER-ISOLATION-STAGE-1` in the filter table's :ipt-chain:`FORWARD` chain. The expectation
    holds for that state whatever created it.

    After the daemon starts, the filter and nat tables do not contain that chain.

.. expectation:: Starting with iptables removes any DOCKER-INGRESS chain
    :id: SWARM-CLEAN-INGRESS-CHAIN-IPT
    :observed_from: node
    :depends_on: os-linux
    :platforms: linux

    *Applies to:* A node with a :ipt-chain:`DOCKER-INGRESS` chain in the filter or nat table

    After the daemon starts with the iptables backend, the filter and nat tables do not contain that
    chain.

.. expectation:: The first start after a switch to nftables removes DOCKER-INGRESS
    :id: SWARM-CLEAN-INGRESS-CHAIN-NFT
    :observed_from: node
    :depends_on: os-linux
    :platforms: linux
    :needs: BR-HOST-NFTABLES, BR-HOST-FORWARDING-V4

    *Applies to:* A node with a :ipt-chain:`DOCKER-INGRESS` chain in the filter or nat table, and a
    jump from the filter table's :ipt-chain:`FORWARD` chain to :ipt-chain:`DOCKER-FORWARD` or
    :ipt-chain:`DOCKER-ISOLATION-STAGE-1`

    After the daemon first starts after a switch to nftables, the filter and nat tables do not
    contain that chain.

.. expectation:: Cleanup searches the filter and nat tables for a fixed chain name
    :id: SWARM-CLEAN-INGRESS-CHAIN-NAME
    :observed_from: node
    :depends_on: none
    :platforms: linux

    The name that cleanup uses for the :ipt-chain:`DOCKER-INGRESS` chain does not change. Cleanup
    searches the filter and nat tables for that chain.

.. _swarm-lifecycle-known-divergences-non-goals:

*******************************
 Known divergences & non-goals
*******************************

Each divergence gives one of two things: a current behavior that differs from what a user might
expect, or a capability that Moby does not provide. A change to the behavior in any divergence is a
behavior change. Make such a change only on purpose.

.. expectation:: VXLAN on cleartext overlays
    :id: SWARM-ENC-CLEARTEXT-VXLAN-UNAUTHENTICATED
    :observed_from: task
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :rationale: RAT-SWARM-CLEARTEXT-SENDERS-NOT-FILTERED

    Not authenticated. A Linux node delivers VXLAN for a cleartext network's VNI from any host that
    can reach the node's data-path port. The frames reach the network's containers. For an encrypted
    network, the node drops VXLAN for the network's VNI that did not arrive under IPsec
    (:ref:`swarm-lifecycle-encryption`).

.. expectation:: IPv6 data path on Windows
    :id: SWARM-CLUSTER-WINDOWS-V6-DATA-PATH
    :observed_from: task
    :depends_on: none
    :platforms: windows

    Not supported. Windows nodes cannot exchange overlay traffic with Linux nodes over an IPv6
    underlay.

.. expectation:: Egress from internal overlays on Windows
    :id: SWARM-EGRESS-WINDOWS-INTERNAL-ONLY
    :observed_from: task
    :depends_on: none
    :platforms: windows

    Not prevented. A Windows container attached only to ``internal`` overlays has egress
    (:ref:`swarm-traffic-egress-the-gateway-network`).

.. expectation:: SCTP on Windows
    :id: SWARM-EW-WINDOWS-SCTP
    :observed_from: task
    :depends_on: os-windows
    :platforms: windows

    Not supported. Windows does not have SCTP. SCTP does not pass to or from containers on Windows
    nodes, directly or through a VIP (:ref:`swarm-traffic-east-west-reachability`,
    :ref:`swarm-traffic-load-balancing-east-west-vip`). A task on a Windows node that publishes an
    SCTP port in :api-value:`mode=host` fails with an error that contains :api-error:`SCTP is
    unsupported on windows`. On a Windows node, a port published as SCTP in
    :api-value:`mode=ingress` is published as TCP. That port then collides with a TCP port of the
    same number (:ref:`swarm-traffic-published-ports`, :need:`SWARM-SCALE-PORT-PROTOCOLS`).

.. expectation:: Orphaned HNS networks on Windows
    :id: SWARM-RESTART-WINDOWS-HNS-ORPHANED
    :observed_from: node
    :depends_on: none
    :platforms: windows

    Not reclaimed. Every HNS overlay network that is on the node when the daemon starts stays in
    place. One of these networks is deleted only when a network with the same ID, or with an
    overlapping VNI, is programmed on the node. Thus the HNS network of a Swarm network that the
    node stopped using while the daemon was not running stays in place
    (:ref:`swarm-lifecycle-restart-and-reclamation`). The HNS endpoints that a killed daemon left on
    these networks also stay in place.

.. expectation:: Flows to a stopping task from other nodes
    :id: SWARM-CHURN-STOPPING-TASK-FLOWS
    :observed_from: task
    :depends_on: none
    :platforms: linux, windows

    Not drained reliably. A task leaves load balancing on its own node at once, and the task is
    stopped 2 s later. Another node may not learn of the withdrawal by that time. That node
    continues to send new flows to the task after the task stops, through the VIP and through
    published ports. Those flows fail until that node learns of the withdrawal
    (:ref:`swarm-traffic-health-and-drain`, :ref:`swarm-lifecycle-churn`).

.. expectation:: Node-local failures are not reported again without a new attempt
    :id: SWARM-FAILREP-NOT-REPEATED
    :observed_from: log
    :depends_on: none
    :platforms: linux, windows
    :rationale: RAT-SWARM-FAILREP-NOT-REPEATED

    A node reports a failure to program cluster-scoped configuration only when something triggers a
    new attempt. An example of a trigger is a change to the service that the configuration is for,
    or to its tasks. Example: a node's data plane is wrong for an hour, and nothing triggers a new
    attempt. In that hour, the node says nothing after its first reports. An operator who did not
    see the first reports cannot discover the condition.

.. expectation:: Recovery from a node-local failure is not reported
    :id: SWARM-FAILREP-RECOVERY-UNREPORTED
    :observed_from: log
    :depends_on: none
    :platforms: linux, windows
    :rationale: RAT-SWARM-FAILREP-RECOVERY-UNREPORTED

    At the default log level, a successful attempt does not log anything. An operator sees the
    failures stop, and sees nothing else. No log entry or report tells the operator that the node's
    data plane is correct again.

.. expectation:: Failure log entries do not name the service
    :id: SWARM-FAILREP-LOG-NO-SERVICE
    :observed_from: log
    :depends_on: none
    :platforms: linux, windows
    :rationale: RAT-SWARM-FAILREP-LOG-NO-SERVICE

    On Linux nodes, a log entry for a load-balancer failure names the network, the VIP or a
    backend's address. Some of these entries contain only internal identifiers. Examples are a
    prefix of a container ID and the path of a network namespace. No entry names the service. An
    entry for a failure to program an IPsec security association prints :log:`%!s(PANIC=String
    method: runtime error: invalid memory address or nil pointer dereference)` in place of the
    association. That entry does not name the peer (:ref:`swarm-lifecycle-failure-reporting`).

.. expectation:: HTTP 400 when the Swarm's state refuses a request
    :id: SWARM-INGRESS-HTTP400-STATE-REFUSAL
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows
    :rationale: RAT-SWARM-STATE-REFUSAL-STATUS-MAPPED

    Misleading. A request that the Swarm's current state refuses responds 400, the status for a
    malformed request. An identical request succeeds once that state changes. The daemon maps the
    gRPC codes ``FailedPrecondition`` and ``InvalidArgument`` to 400. Examples are a service with a
    port in ingress mode in a Swarm without an ingress network, and :api-op:`DELETE /networks/{id}`
    for a network that a service or task uses. Others are :api-op:`DELETE /networks/{id}` for the
    ingress network while a service publishes a port in ingress mode, and a service that chooses a
    port that another service publishes (:ref:`swarm-networking-swarm-scoped-networks`,
    :ref:`swarm-networking-the-ingress-network`, :ref:`swarm-traffic-port-allocation`).

.. expectation:: XFRM algorithm support is not checked
    :id: SWARM-ENC-XFRM-ALGORITHM-UNCHECKED
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux

    Not detected. Before a task or container joins an :drvopt:`encrypted` network, the daemon checks
    only that the kernel offers the XFRM netlink interface. On a host whose kernel does not offer
    ``rfc4106(gcm(aes))``, the task starts, and the daemon logs a warning for each security
    association that it fails to add. The node's IPsec policies are in place, so its traffic on the
    network with other nodes does not pass (:ref:`swarm-lifecycle-encryption`).

.. expectation:: Partial blocks between nodes
    :id: SWARM-CONVERGE-PARTIAL-BLOCK
    :observed_from: api:active
    :depends_on: none
    :platforms: linux, windows
    :rationale: RAT-SWARM-PEERS-FROM-GOSSIP

    Not covered. A firewall that blocks only the data path between two nodes (the data-path port, or
    ESP on an :drvopt:`encrypted` network) while gossip flows is not detected. The nodes stay peers,
    and the overlay traffic between them is dropped. A block in one direction only is not covered
    either.

.. expectation:: Swarm under the nftables backend
    :id: SWARM-NFT-BACKEND
    :observed_from: api
    :depends_on: none
    :platforms: linux

    Not asserted. A node in a Swarm can run the nftables backend. No expectation of these specs is
    asserted on such a node, and a switch of a node in a Swarm to or from the nftables backend is
    not asserted either.

.. expectation:: Node-local attachable networks on Swarm leave
    :id: SWARM-RESTART-LEAVE-LOCAL-ATTACHABLE
    :observed_from: api:solo
    :depends_on: none
    :platforms: linux, windows

    Disconnected. :api-op:`POST /swarm/leave` disconnects every container on the node from each
    network that was created with :api-field:`Attachable`. This includes node-local networks, such
    as user-defined bridge networks. The containers stay disconnected after the leave, and the
    node-local network stays. A warning that contains :log:`is not a dynamic network` appears in the
    daemon log. Only Swarm-scoped networks are meant to be disconnected
    (:ref:`swarm-lifecycle-restart-and-reclamation`).
