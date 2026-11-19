#############################
 Swarm networking: lifecycle
#############################

This document continues :doc:`swarm-networking.md <swarm-networking>`. Its conventions, definitions
and host requirements apply here.

.. _swarm-lifecycle-requirements:

**************
 Requirements
**************

.. req:: An overlay address reaches its current endpoint after it moves
    :id: REQ-SWARM-ADDRESS-REUSE
    :source_kind: issue
    :source: https://github.com/moby/moby/issues/50232, https://github.com/moby/moby/issues/52661, https://docs.docker.com/engine/release-notes/17.10/#17100-ce

    When an overlay address moves to another endpoint, for example when a replacement container gets
    the address back, peers reach the new endpoint. Stale neighbor and FDB entries do not misroute
    such an address.

.. req:: Encrypted overlays drop VXLAN that did not arrive under IPsec
    :id: REQ-SWARM-ENC-AUTHENTICATED
    :source_kind: docs
    :source: https://docs.docker.com/engine/swarm/swarm-tutorial/#open-protocols-and-ports-between-the-hosts, https://docs.docker.com/engine/release-notes/17.03/#17030-ce, https://docs.docker.com/engine/release-notes/23.0/#2303, https://docs.docker.com/engine/release-notes/29/#2982

    Docker discards plain VXLAN for an encrypted network.

.. req:: A key rotation does not break encrypted overlay traffic
    :id: REQ-SWARM-ENC-KEY-ROTATION
    :source_kind: docs
    :source: https://docs.docker.com/engine/release-notes/17.03/#17030-ce, https://github.com/moby/moby/commit/4a04857a681d15e9fed49c6e72ef1e8e11be5962

    A rotation of the keys of an encrypted overlay network does not break encrypted traffic between
    nodes, including nodes whose advertise address differs from their local address, as in a
    cross-datacenter Swarm.

.. req:: Swarm mode keeps services available through node failures and membership changes
    :id: REQ-SWARM-FAULT-TOLERANT
    :source_kind: docs
    :source: https://docs.docker.com/engine/swarm/swarm-mode/, https://docs.docker.com/engine/swarm/how-swarm-mode-works/nodes/#manager-nodes, https://docs.docker.com/engine/swarm/admin_guide/#force-the-swarm-to-rebalance

    Swarm mode keeps services running and available. A swarm with several managers recovers from the
    failure of a manager node without downtime. A node that is added to the swarm, or that
    reconnects after a period of unavailability, does not disrupt the running services, and a node
    with running tasks that becomes unavailable disrupts the end user minimally.

.. req:: A node that leaves the Swarm stops running in Swarm mode
    :id: REQ-SWARM-LEAVE
    :source_kind: docs
    :source: https://docs.docker.com/engine/swarm/manage-nodes/#leave-the-swarm, https://docs.docker.com/reference/cli/docker/swarm/leave/

    After docker swarm leave, the node's engine no longer runs in Swarm mode.

.. req:: Swarm leave detaches containers only from Swarm-scoped networks
    :id: REQ-SWARM-LEAVE-SWARM-NETWORKS-ONLY
    :source_kind: issue
    :source: https://github.com/moby/moby/pull/30157: its commit message
    :refines: PRIN-FAITHFUL

    When a node leaves the Swarm, the daemon disconnects its standalone containers from the
    Swarm-scoped networks that they are attached to, and removes those networks from the node.
    Containers keep their attachments to networks that are not Swarm-scoped.

.. req:: A node with the nftables backend takes part in a Swarm of iptables nodes
    :id: REQ-SWARM-MIXED-BACKEND-INTEROP
    :source_kind: issue
    :source: https://github.com/moby/moby/pull/53125: the merged change that lets such a daemon join, https://docs.docker.com/engine/network/firewall-nftables/
    :violates: PRIN-STABLE

    A daemon with the nftables firewall backend joins a Swarm. A Swarm's other nodes can run the
    iptables backend, the default. A node with the nftables backend takes part in the overlay and
    ingress networks of such a Swarm. The page about the nftables backend says that its support is
    experimental: its configuration options, behavior and implementation may all change in future
    releases.

.. req:: A node logs its failure to program a load balancer or an ingress port
    :id: REQ-SWARM-NODE-FAILURE-LOGGED
    :source_kind: issue
    :source: https://github.com/moby/moby/pull/53125, moby/moby daemon/libnetwork/service_linux.go: code comments on retrying the load-balancer and ingress setup at the next backend event

    A node that cannot program a load-balancer service or backend, or publish an ingress port,
    reports it in the daemon log, and again on each retry while the cause holds. Once the cause has
    cleared, a retry reports no failure.

.. req:: Reachability returns after a node fails or leaves and comes back
    :id: REQ-SWARM-REJOIN-RECOVERY
    :source_kind: docs
    :source: https://docs.docker.com/engine/release-notes/29/#2990, https://docs.docker.com/engine/release-notes/29/#2980, https://github.com/moby/moby/issues/35807

    When a node that failed, was cut off or left the Swarm comes back, overlay peers, service
    discovery and published ports reach it again and keep reaching it.

.. req:: A node's data plane recovers after its daemon restarts or is killed
    :id: REQ-SWARM-RESTART-RECOVERY
    :source_kind: docs
    :source: https://docs.docker.com/engine/swarm/admin_guide/#recover-from-losing-the-quorum, https://docs.docker.com/engine/release-notes/29/#2990, https://docs.docker.com/engine/release-notes/17.04/#17040-ce, https://github.com/moby/moby/issues/49555: a report of such a device that maintainers triaged as a bug

    After its daemon restarts, gracefully or after a crash, a node is back in the Swarm and serves
    its tasks and published ports. The other nodes' service discovery and load balancing include the
    node's tasks again. Encryption state that the node held before the restart does not block its
    overlay traffic. Devices that a killed daemon left for its overlay networks do not remain to
    block networks later.

.. req:: A service update adds or removes the service's networks
    :id: REQ-SWARM-SERVICE-NETWORK-UPDATE
    :source_kind: docs
    :source: https://docs.docker.com/engine/swarm/networking/#attach-a-service-to-an-overlay-network, https://docs.docker.com/reference/cli/docker/service/update/#network-add, https://docs.docker.com/engine/release-notes/18.09/#18097

    docker service update --network-add attaches a service to an existing network, and --network-rm
    detaches it. Service containers communicate over the overlay networks they are attached to.

.. req:: At start, the daemon removes the DOCKER-INGRESS chain that 28.0.0 and earlier left
    :id: REQ-SWARM-STALE-FIREWALL-CLEANUP
    :source_kind: docs
    :source: https://docs.docker.com/engine/release-notes/29/#2990

    At start, the daemon removes the empty DOCKER-INGRESS iptables chain, and the FORWARD rule that
    jumps to it, that Docker Engine 28.0.0 and earlier left behind.

.. _swarm-lifecycle-rationales:

************
 Rationales
************

.. rationale:: On a cleartext overlay, limiting who can send VXLAN to a node is the operator's job
    :id: RAT-SWARM-CLEARTEXT-SENDERS-NOT-FILTERED

    VXLAN has no authentication. For a cleartext overlay network, the documentation leaves
    protection to the operator: firewall rules limit access to a node's data-path port, and an
    :drvopt:`encrypted` network is the answer where the network between the nodes is not trusted
    (:need:`REQ-SWARM-ENC-AUTHENTICATED`). A node therefore delivers VXLAN for a cleartext network's
    VNI whatever host sent it. A node that filtered these frames by sender would take over a part of
    the operator's job. It would also tie delivery to what the node knows about its peers at that
    moment: it would drop frames from a peer that it has not learned of yet. Such a filter should
    arrive as a designed feature, and not as a side effect of a change to the node's firewall rules.

.. rationale:: The daemon log is the only report of a node-local failure, and log alerts read its entries
    :id: RAT-SWARM-FAILURE-LOG-ENTRIES

    A node reports a failure to program cluster-scoped configuration in its daemon log and nowhere
    else. :need:`REQ-SWARM-NODE-FAILURE-LOGGED` promises this for a load-balancer service or backend
    and for an ingress port. The node also logs a failure to program an overlay peer, or an IPsec
    security association or policy for a peer, and a failure to unpublish a service's ingress ports
    (:need:`SWARM-FAILREP-PEER`, :need:`SWARM-FAILREP-IPSEC`,
    :need:`SWARM-FAILREP-INGRESS-UNPUBLISH`), which no source promises. Dropping one of those
    entries, or lowering its level, would leave such a failure with no report at all. The
    requirement covers the report of a failure when it occurs and on each new attempt. It does not
    say that a node tries again by itself, and it does not say that a node reports that an attempt
    succeeded. Today a node does neither: its log gets no entry while nothing triggers a new
    attempt, and none at the default log level when an attempt recovers. An operator's log alerts
    are what reads these entries. A node that tried again on a timer, or that logged each recovery
    at the default level, would change the number and the timing of the entries on every node at
    once. A timer would also add work that repeats for as long as a cause holds. How a node tries
    again after a node-local failure, and what it then reports, should change by decision.

.. rationale:: A node knows its peers from gossip, and does not test the data path to them
    :id: RAT-SWARM-PEERS-FROM-GOSSIP

    The :api-field:`Peers` that a node reports for an overlay network are the nodes that gossip
    tells it are on the network. The daemon does not probe the data path between nodes: that the
    firewalls and the physical network pass it is a host requirement
    (:need:`SWARM-HOST-FIREWALL-CLUSTER-PORTS`). A block that stops data-path traffic and lets
    gossip through therefore changes nothing that a node reports. A node that detected such a block,
    and then dropped or flagged the peer, would be a second failure detector beside the one that the
    gossip already has. It would need a rule for a block in one direction only, and for a block on
    the traffic of some networks, such as ESP, and not of others. That should arrive as a designed
    feature.

.. rationale:: Automatic VNIs avoid the VNIs that other networks reserved
    :id: RAT-SWARM-AUTO-VNI-DISTINCT

    The managers take automatic VNIs from the same pool that holds the VNIs that other overlay
    networks' :drvopt:`vxlanid_list` reserved. Two overlay networks with one VNI cannot both be
    programmed on a node, and their VXLAN traffic is the same on the underlay. An allocator change
    that let an automatic VNI equal a reserved one would make one of the two networks fail where
    both have tasks. Which VNIs the allocator may hand out should change by design.

.. rationale:: A wrong neighbor or FDB entry misdirects traffic away from the event that left it
    :id: RAT-SWARM-FORWARDING-ENTRY-FAULTS-LATENT

    The overlay driver's neighbor and FDB entries are permanent, and they decide where a frame for
    an address goes. An entry that should not exist does no harm at first. An entry for an endpoint
    on a node that was declared failed misdirects traffic when the endpoint's address is in use
    elsewhere after the partition heals. An entry for one of the node's own endpoints sends frames
    for a local container into the tunnel. In both cases the failure appears in traffic, at a
    distance from the event that left the entry. A change to which entries a node holds after these
    events should therefore be deliberate.

.. rationale:: A security review records the cipher of encrypted overlay networks
    :id: RAT-SWARM-ENC-CIPHER

    A security review of a deployment records the cipher that its encrypted overlay networks use.
    The overlay driver page named AES-GCM until it was rewritten. :need:`PRIN-SECURITY-NOT-WEAKENED`
    covers a move to a weaker cipher. A move to another cipher at least as strong weakens nothing,
    but it changes what those reviews record. It should be made as a decision and stated in the
    release notes.

.. rationale:: A node's failure to program a service is not the failure of a task
    :id: RAT-SWARM-NODE-FAILURE-NOT-TASK-FAILURE

    When a node cannot program cluster-scoped configuration for a service, the fault lies with the
    node or with a peer, not with any one task. Reporting it in a task's state would blame the task:
    the orchestrator would replace a task that runs, and the replacement on the same node would meet
    the same fault. The node reports the failure in the daemon log only
    (:need:`REQ-SWARM-NODE-FAILURE-LOGGED`), and :api-op:`GET /tasks`, :api-op:`GET /services` and
    :cli:`docker service ls` show the service as healthy. Giving the failure a place in the API is a
    design decision about what it belongs to: the node, the service or the task. Flagging such a
    service in the CLI, with no change to the API, is the same decision.

.. _swarm-lifecycle-encryption:

************
 Encryption
************

Encrypted overlay networks are implemented on Linux nodes only
(:need:`SWARM-CTR-WINDOWS-ENCRYPTION`). This section holds on Linux nodes, except where an
expectation names Windows. The expectations about the traffic that an underlay host observes name
``os-linux`` because they observe the data path. The missing-XFRM expectations name ``none`` because
the daemon alone decides their verdict once XFRM is missing.

.. expectation:: No cleartext VXLAN for an encrypted network's VNI crosses the underlay
    :id: SWARM-ENC-NO-CLEARTEXT
    :observed_from: underlay
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-SWARM-ENC-CONFIDENTIAL
    :needs: SWARM-HOST-XFRM, SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-IPTABLES-ENC-MATCHES, SWARM-HOST-IPTABLES-LEGACY-MANGLE-KERNEL, SWARM-HOST-IPTABLES-NFT-ROUTE-KERNEL
    :within: SWARM-CEIL-REASSIGNMENT

    No cleartext VXLAN traffic is observable between nodes on the underlay network for the VNIs of
    :drvopt:`encrypted` overlay networks. This holds for each such network from when it is created,
    without a ceiling.

.. expectation:: Underlay traffic for an encrypted network's VNI is IPsec ESP
    :id: SWARM-ENC-ESP
    :observed_from: underlay
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-SWARM-ENC-CONFIDENTIAL
    :needs: SWARM-HOST-XFRM, SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-IPTABLES-ENC-MATCHES, SWARM-HOST-IPTABLES-LEGACY-MANGLE-KERNEL, SWARM-HOST-IPTABLES-NFT-ROUTE-KERNEL
    :within: SWARM-CEIL-REASSIGNMENT

    Traffic between nodes on the underlay network for the VNIs of :drvopt:`encrypted` overlay
    networks is observable as IPsec ESP. This holds for each such network from when it is created,
    without a ceiling.

.. expectation:: ESP is in transport mode and protects the VXLAN datagrams
    :id: SWARM-ENC-ESP-TRANSPORT-MODE
    :observed_from: underlay (decrypted with a node's keys)
    :depends_on: os-linux
    :platforms: linux
    :exact_form_of: SWARM-ENC-ESP
    :needs: SWARM-HOST-XFRM, SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-IPTABLES-ENC-MATCHES, SWARM-HOST-IPTABLES-LEGACY-MANGLE-KERNEL, SWARM-HOST-IPTABLES-NFT-ROUTE-KERNEL
    :within: SWARM-CEIL-REASSIGNMENT

    ESP is in transport mode and protects the VXLAN datagrams. This holds for the VNI of each
    :drvopt:`encrypted` network from when the network is created, without a ceiling.

.. expectation:: SAs use keys of at least 128 bits and ICVs of at least 64 bits
    :id: SWARM-ENC-KEY-STRENGTH
    :observed_from: node
    :depends_on: none
    :platforms: linux
    :refines: PRIN-SECURITY-NOT-WEAKENED
    :needs: SWARM-HOST-XFRM, SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-IPTABLES-ENC-MATCHES, SWARM-HOST-IPTABLES-LEGACY-MANGLE-KERNEL, SWARM-HOST-IPTABLES-NFT-ROUTE-KERNEL
    :within: SWARM-CEIL-REASSIGNMENT

    An :drvopt:`encrypted` network's IPsec security associations encrypt with a key of at least 128
    bits, not counting a salt that the key material carries, and authenticate with an integrity
    check value of at least 64 bits.

.. expectation:: Security associations use AES-GCM
    :id: SWARM-ENC-AES-GCM
    :observed_from: node
    :depends_on: none
    :platforms: linux
    :needs: SWARM-HOST-XFRM, SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-IPTABLES-ENC-MATCHES, SWARM-HOST-IPTABLES-LEGACY-MANGLE-KERNEL, SWARM-HOST-IPTABLES-NFT-ROUTE-KERNEL
    :within: SWARM-CEIL-REASSIGNMENT
    :rationale: RAT-SWARM-ENC-CIPHER

    An :drvopt:`encrypted` network's IPsec security associations use AES-GCM.

.. expectation:: Security associations use rfc4106(gcm(aes)) with a 64-bit ICV
    :id: SWARM-ENC-ALGORITHM
    :observed_from: node
    :depends_on: none
    :platforms: linux
    :exact_form_of: SWARM-ENC-AES-GCM
    :pins: SWARM-HOST-XFRM
    :needs: SWARM-HOST-XFRM, SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-IPTABLES-ENC-MATCHES, SWARM-HOST-IPTABLES-LEGACY-MANGLE-KERNEL, SWARM-HOST-IPTABLES-NFT-ROUTE-KERNEL
    :within: SWARM-CEIL-REASSIGNMENT

    An encrypted network's IPsec security associations use ``rfc4106(gcm(aes))`` with a 64-bit ICV.

.. expectation:: Forged cleartext VXLAN from the underlay does not reach encrypted tasks
    :id: SWARM-ENC-FORGED-VXLAN
    :observed_from: underlay → task
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-SWARM-ENC-AUTHENTICATED
    :needs: SWARM-HOST-XFRM, SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-IPTABLES-ENC-MATCHES, SWARM-HOST-IPTABLES-LEGACY-MANGLE-KERNEL, SWARM-HOST-IPTABLES-NFT-ROUTE-KERNEL
    :within: SWARM-CEIL-REASSIGNMENT

    A forged cleartext VXLAN datagram for the VNI of an :drvopt:`encrypted` overlay network,
    injected from the underlay, does not reach the network's tasks. This holds from when the network
    is created, without a ceiling.

.. expectation:: After a firewalld reload, no encrypted VNI's VXLAN crosses in cleartext
    :id: SWARM-ENC-NO-CLEARTEXT-FIREWALLD
    :observed_from: underlay
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-SWARM-ENC-CONFIDENTIAL
    :needs: SWARM-HOST-XFRM, SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-IPTABLES-ENC-MATCHES, SWARM-HOST-IPTABLES-LEGACY-MANGLE-KERNEL, SWARM-HOST-IPTABLES-NFT-ROUTE-KERNEL
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A node with firewalld running, and an :drvopt:`encrypted` overlay network with a
    task on that node and a task on another node

    After a firewalld reload on the node, while the task on the node sends traffic to the task on
    the other node over the network, no cleartext VXLAN traffic for the network's VNI from the node
    is observable on the underlay network.

.. expectation:: After a firewalld reload, forged VXLAN does not reach encrypted tasks
    :id: SWARM-ENC-FORGED-VXLAN-FIREWALLD
    :observed_from: underlay → task
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-SWARM-ENC-AUTHENTICATED
    :needs: SWARM-HOST-XFRM, SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-IPTABLES-ENC-MATCHES, SWARM-HOST-IPTABLES-LEGACY-MANGLE-KERNEL, SWARM-HOST-IPTABLES-NFT-ROUTE-KERNEL
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A node with firewalld running, and an :drvopt:`encrypted` overlay network with a
    task on that node

    After a firewalld reload on the node, a forged cleartext VXLAN datagram for the network's VNI,
    injected from the underlay to the node, with an inner frame addressed to the task's MAC and IP
    addresses on the network, does not reach the task.

.. expectation:: Cleartext VXLAN for a cleartext network's VNI passes between nodes
    :id: SWARM-ENC-CLEARTEXT-PASSES
    :observed_from: underlay
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-SWARM-OVERLAY-CONNECTIVITY
    :within: SWARM-CEIL-REASSIGNMENT

    Cleartext VXLAN for a cleartext network's VNI passes between nodes, whether or not those nodes
    also have :drvopt:`encrypted` networks.

.. expectation:: A Linux node delivers a cleartext network's VXLAN from any sender
    :id: SWARM-ENC-CLEARTEXT-VXLAN-UNAUTHENTICATED
    :observed_from: underlay → task
    :depends_on: os-linux
    :platforms: linux
    :within: SWARM-CEIL-REASSIGNMENT
    :rationale: RAT-SWARM-CLEARTEXT-SENDERS-NOT-FILTERED

    A Linux node delivers VXLAN for a cleartext network's VNI from any host that can reach the
    node's data-path port. The frames reach the network's containers.

.. expectation:: A cleartext network on a deleted encrypted network's VNI passes VXLAN
    :id: SWARM-ENC-VNI-REUSE
    :observed_from: underlay
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-SWARM-OVERLAY-CONNECTIVITY
    :needs: SWARM-HOST-XFRM, SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-IPTABLES-ENC-MATCHES, SWARM-HOST-IPTABLES-LEGACY-MANGLE-KERNEL, SWARM-HOST-IPTABLES-NFT-ROUTE-KERNEL
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* Two nodes on which an encrypted network was programmed

    After that network is deleted, a cleartext network that is allocated the same VNI passes
    cleartext VXLAN between those nodes.

.. expectation:: A VNI freed while the daemon was down passes cleartext traffic
    :id: SWARM-ENC-VNI-REUSE-KILLED-DAEMON
    :observed_from: task
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-SWARM-OVERLAY-CONNECTIVITY, REQ-SWARM-RESTART-RECOVERY
    :needs: SWARM-HOST-XFRM, SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-IPTABLES-ENC-MATCHES, SWARM-HOST-IPTABLES-LEGACY-MANGLE-KERNEL, SWARM-HOST-IPTABLES-NFT-ROUTE-KERNEL
    :within: SWARM-CEIL-REJOIN, SWARM-CEIL-REASSIGNMENT

    *Applies to:* A node where an encrypted network is programmed

    The node's daemon is killed. Then that network is deleted from the Swarm, and the node's daemon
    is started. After these events, a cleartext network that is allocated the same VNI passes
    traffic to and from the node.

.. expectation:: On a reused encrypted VNI, Windows containers reach Linux containers
    :id: SWARM-ENC-VNI-REUSE-WINDOWS
    :observed_from: task, ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-OVERLAY-CONNECTIVITY
    :needs: SWARM-HOST-XFRM, SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-IPTABLES-ENC-MATCHES, SWARM-HOST-IPTABLES-LEGACY-MANGLE-KERNEL, SWARM-HOST-IPTABLES-NFT-ROUTE-KERNEL
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A cleartext network that is allocated the VNI of a deleted encrypted network

    Containers on Windows nodes on that network exchange traffic with containers on the Linux nodes
    that had containers on the encrypted network.

.. expectation:: Key rotations leave no delivery gap longer than 1 second
    :id: SWARM-ENC-KEY-ROTATION
    :observed_from: task
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-SWARM-ENC-KEY-ROTATION, REQ-SWARM-OVERLAY-CONNECTIVITY
    :needs: SWARM-HOST-XFRM, SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-FIREWALL-ESP, SWARM-HOST-IPTABLES-ENC-MATCHES, SWARM-HOST-IPTABLES-LEGACY-MANGLE-KERNEL, SWARM-HOST-IPTABLES-NFT-ROUTE-KERNEL
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* An :drvopt:`encrypted` overlay network with tasks on two nodes, in a Swarm whose
    leader manager does not change, including two nodes that a one-to-one NAT between them
    separates, whose advertise and data-path addresses are the addresses that the NAT maps to
    addresses of their own interfaces, and whose routes to each other select those interface
    addresses as the source addresses

    Throughout a period of 24 hours plus the :need:`[[title|lower]] ceiling ([[seconds]] s)
    <SWARM-CEIL-REASSIGNMENT>`, a UDP flow between tasks on the two nodes has no gap in delivery
    longer than 1 second.

.. expectation:: Each node moves to new SAs at least once every 12 hours plus the ceiling
    :id: SWARM-ENC-KEY-ROTATION-INTERVAL
    :observed_from: node, underlay
    :depends_on: os-linux
    :platforms: linux
    :refines: PRIN-SECURITY-NOT-WEAKENED
    :needs: SWARM-HOST-XFRM, SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-IPTABLES-ENC-MATCHES, SWARM-HOST-IPTABLES-LEGACY-MANGLE-KERNEL, SWARM-HOST-IPTABLES-NFT-ROUTE-KERNEL
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* An :drvopt:`encrypted` overlay network with tasks on two nodes, in a Swarm whose
    leader manager does not change

    In every period of 12 hours plus the :need:`[[title|lower]] ceiling ([[seconds]] s)
    <SWARM-CEIL-REASSIGNMENT>`, each node installs, for the other node on the network, security
    associations with SPIs that it did not have at the start of the period. In every such period,
    the ESP packets that each node sends to the other node, while tasks on the two nodes exchange
    traffic on the network, also come to carry an SPI that its ESP packets to that node did not
    carry at the start of the period.

.. expectation:: Without the encryption keys, a task on an encrypted network does not run
    :id: SWARM-ENC-NO-KEYS
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-ENC-CONFIDENTIAL, REQ-SWARM-TASK-ERRORS

    *Applies to:* A node that does not have the network encryption keys

    On that node, a task attached to an :drvopt:`encrypted` network does not reach the ``running``
    state, and :api-op:`GET /tasks` reports an error for the task.

.. expectation:: A task on a node without the keys says encryption keys are not present
    :id: SWARM-ENC-NO-KEYS-MSG
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :exact_form_of: SWARM-ENC-NO-KEYS

    *Applies to:* A node that does not have the network encryption keys

    On that node, the error that :api-op:`GET /tasks` reports for such a task on an
    :drvopt:`encrypted` network contains :api-error:`cannot join secure network: encryption keys not
    present`.

.. expectation:: Without XFRM netlink, a task on an encrypted network does not run
    :id: SWARM-ENC-NO-XFRM
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-ENC-CONFIDENTIAL, REQ-SWARM-TASK-ERRORS
    :pins: SWARM-HOST-XFRM

    *Applies to:* A Linux host whose kernel does not offer the XFRM netlink interface

    On that host, a task attached to an :drvopt:`encrypted` network does not reach the ``running``
    state, and :api-op:`GET /tasks` reports an error for the task.

.. expectation:: A task on a host without XFRM says required modules are missing
    :id: SWARM-ENC-NO-XFRM-MSG
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :exact_form_of: SWARM-ENC-NO-XFRM
    :pins: SWARM-HOST-XFRM

    *Applies to:* A Linux host whose kernel does not offer the XFRM netlink interface

    On that host, the error that :api-op:`GET /tasks` reports for such a task on an
    :drvopt:`encrypted` network contains :api-error:`cannot join secure network: required modules to
    install IPSEC rules are missing on host`.

.. expectation:: Without rfc4106(gcm(aes)), encrypted tasks run but pass no traffic
    :id: SWARM-ENC-XFRM-ALGORITHM-UNCHECKED
    :observed_from: api:mgr, api:active, task, log
    :depends_on: none
    :platforms: linux
    :within: SWARM-CEIL-REASSIGNMENT
    :concerns: SWARM-ENC-NO-XFRM, SWARM-ENC-ALGORITHM
    :violates: REQ-SWARM-ENC-CONFIDENTIAL, REQ-SWARM-ATTACH-FAILURE-START, REQ-SWARM-OVERLAY-CONNECTIVITY

    On a host whose kernel offers the XFRM netlink interface and does not offer
    ``rfc4106(gcm(aes))``, :api-op:`GET /tasks` reports the :api-field:`Status.State` of a task on
    an :drvopt:`encrypted` network on that host as ``running``, and :api-op:`POST
    /containers/{id}/start` for a created container on an attachable :drvopt:`encrypted` network
    responds 204. The daemon logs a warning for each security association that it fails to add. The
    node's traffic on the network with other nodes does not pass.

.. expectation:: Without XFRM netlink, a container on an encrypted network does not start
    :id: SWARM-ENC-NO-XFRM-CTR-START
    :observed_from: api:active
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-ENC-CONFIDENTIAL, REQ-SWARM-ATTACH-FAILURE-START
    :pins: SWARM-HOST-XFRM

    *Applies to:* A Linux host whose kernel does not offer the XFRM netlink interface

    On that host, :api-op:`POST /containers/{id}/start` for a created or stopped container on an
    attachable :drvopt:`encrypted` network responds with a 4xx or 5xx status.

.. expectation:: Without XFRM netlink, a container start responds 500 after 20 seconds
    :id: SWARM-ENC-NO-XFRM-CTR-START-HTTP500
    :observed_from: api:active
    :depends_on: none
    :platforms: linux
    :exact_form_of: SWARM-ENC-NO-XFRM-CTR-START
    :pins: SWARM-SCOPED-ATTACH-FAILURE-TIMEOUT, SWARM-HOST-XFRM
    :concerns: SWARM-ENC-NO-XFRM-CTR-START
    :violates: PRIN-ERRORS-ACTIONABLE

    *Applies to:* A Linux host whose kernel does not offer the XFRM netlink interface

    On that host, :api-op:`POST /containers/{id}/start` for a created or stopped container on an
    attachable :drvopt:`encrypted` network responds 500 after 20 seconds, with :api-error:`attaching
    to network failed, make sure your network options are correct and check manager logs: context
    deadline exceeded`.

.. expectation:: Without XFRM netlink, docker run on an encrypted network exits non-zero
    :id: SWARM-ENC-NO-XFRM-CTR-START-CLI
    :observed_from: api:active
    :depends_on: cli
    :platforms: linux
    :refines: REQ-SWARM-ENC-CONFIDENTIAL, REQ-SWARM-ATTACH-FAILURE-START
    :cli_for: SWARM-ENC-NO-XFRM-CTR-START
    :pins: SWARM-HOST-XFRM

    *Applies to:* A Linux host whose kernel does not offer the XFRM netlink interface

    On that host, :cli:`docker run -d --network` naming an attachable :drvopt:`encrypted` network
    exits with status 125.

.. expectation:: After a start fails without XFRM, the container stays created
    :id: SWARM-ENC-NO-XFRM-CTR-CREATED
    :observed_from: api:active
    :depends_on: none
    :platforms: linux
    :refines: REQ-NET-START-FAILED-CONTAINER-KEPT
    :pins: SWARM-HOST-XFRM

    *Applies to:* A Linux host whose kernel does not offer the XFRM netlink interface, and a created
    container without :api-field:`HostConfig.AutoRemove` on an attachable :drvopt:`encrypted`
    network

    After :api-op:`POST /containers/{id}/start` for the container responds with a 4xx or 5xx status
    on that host, :api-op:`GET /containers/{id}/json` reports :api-field:`State.Status` as
    ``created``.

.. expectation:: After a start fails without XFRM, an auto-remove container is removed
    :id: SWARM-ENC-NO-XFRM-CTR-AUTO-REMOVE
    :observed_from: api:active
    :depends_on: none
    :platforms: linux
    :refines: REQ-NET-START-FAILED-AUTO-REMOVED, PRIN-NO-LEAKS
    :pins: SWARM-HOST-XFRM

    *Applies to:* A Linux host whose kernel does not offer the XFRM netlink interface, and a created
    container with :api-field:`HostConfig.AutoRemove` on an attachable :drvopt:`encrypted` network

    After :api-op:`POST /containers/{id}/start` for the container responds with a 4xx or 5xx status
    on that host, :api-op:`GET /containers/{id}/json` for the container responds 404.

.. expectation:: Without rfc4106(gcm(aes)), a task on an encrypted network runs
    :id: SWARM-ENC-NO-ALGORITHM-TASK-STARTS
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :pins: SWARM-ENC-XFRM-ALGORITHM-UNCHECKED, SWARM-HOST-XFRM

    *Applies to:* A Linux host whose kernel offers the XFRM netlink interface but not
    ``rfc4106(gcm(aes))``

    On that host, :api-op:`GET /tasks` reports the :api-field:`Status.State` of a task attached to
    an :drvopt:`encrypted` network as ``running``.

.. expectation:: Without rfc4106(gcm(aes)), encrypted traffic between nodes fails
    :id: SWARM-ENC-NO-ALGORITHM-NO-TRAFFIC
    :observed_from: task
    :depends_on: none
    :platforms: linux
    :pins: SWARM-ENC-XFRM-ALGORITHM-UNCHECKED, SWARM-HOST-XFRM
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A Linux host whose kernel offers the XFRM netlink interface but not
    ``rfc4106(gcm(aes))``

    The tasks on that host do not exchange traffic over an :drvopt:`encrypted` network with tasks on
    other nodes.

.. expectation:: Without rfc4106(gcm(aes)), the daemon logs that adding each SA failed
    :id: SWARM-ENC-NO-ALGORITHM-LOG
    :observed_from: log
    :depends_on: none
    :platforms: linux
    :pins: SWARM-ENC-XFRM-ALGORITHM-UNCHECKED, SWARM-HOST-XFRM
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A Linux host whose kernel offers the XFRM netlink interface but not
    ``rfc4106(gcm(aes))``

    The daemon log of that host contains a warning-level entry that contains :log:`Failed Adding`
    for each IPsec security association that it fails to add. The entry prints
    :log:`%!s(PANIC=String method: runtime error: invalid memory address or nil pointer
    dereference)` in place of the security association, and gives the kernel's error. It does not
    name the algorithm that the kernel lacks.

.. _swarm-lifecycle-wire-format-interop:

***********************
 Wire format & interop
***********************

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
    :refines: REQ-SWARM-OVERLAY-CONNECTIVITY
    :within: SWARM-CEIL-REASSIGNMENT

    A container is reachable from other nodes whether its IP address and its MAC address are each
    assigned manually or automatically.

.. expectation:: Cross-node expectations hold between workers of different releases
    :id: SWARM-WIRE-MIXED-VERSIONS
    :observed_from: client, task, underlay
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-SWARM-MIXED-VERSION-INTEROP

    *Applies to:* A Swarm whose managers run the release under test, and whose workers mix nodes of
    the previous release and the release under test, with every node on the iptables backend, where
    the nodes of the previous release all run one of the two releases that :doc:`../README.md
    <../README>` calls *N−1*

    In that Swarm, the expectations of the following sections about traffic between nodes hold
    between nodes of different versions: in :doc:`swarm-traffic.md <swarm-traffic>`,
    :ref:`swarm-traffic-east-west-reachability`, :ref:`swarm-traffic-load-balancing-east-west-vip`
    with its subsections, and the :ref:`swarm-traffic-api-value-mode-ingress` subsection of
    :ref:`swarm-traffic-published-ports`; in this document, :ref:`swarm-lifecycle-encryption`. They
    also include the expectations of :ref:`swarm-traffic-service-discovery` about a name whose
    workload is on another node. Each of those expectations keeps its own category and dependencies.
    An expectation is about traffic between nodes when its statement is about traffic that one node
    of the Swarm sends to another, or about the encryption keys and security associations that a
    node keeps for such traffic. An expectation about traffic that another host on the underlay
    sends to a node is not among them. They hold on cleartext and :drvopt:`encrypted` overlays and
    on the ingress network. Other expectations, those of :ref:`swarm-lifecycle-failure-lifecycle`
    included, are asserted only between nodes of the release under test. No expectation is asserted
    between two nodes of the previous release. An expectation that involves more than two nodes
    holds for every assignment of the two releases to its nodes in which none of the traffic between
    nodes that it states passes between two nodes of the previous release.

.. expectation:: Cross-node expectations hold between nodes with different firewall backends
    :id: SWARM-WIRE-MIXED-BACKENDS
    :observed_from: client, task, underlay
    :depends_on: os-linux
    :platforms: linux
    :provisional_on: REQ-SWARM-MIXED-BACKEND-INTEROP

    *Applies to:* A Swarm that mixes Linux nodes with the iptables firewall backend and Linux nodes
    with the nftables firewall backend

    In that Swarm, every expectation of :doc:`swarm-networking.md <swarm-networking>`,
    :doc:`swarm-traffic.md <swarm-traffic>` and this document about traffic between nodes holds
    between nodes with different backends, when the nodes run the same release. In a Swarm that
    :need:`SWARM-WIRE-MIXED-VERSIONS` describes, but with either backend on each node, the
    expectations that it names also hold between nodes with different backends. An expectation is
    about traffic between nodes when its statement is about traffic that one node of the Swarm sends
    to another, or about the encryption keys and security associations that a node keeps for such
    traffic. An expectation about traffic that another host on the underlay sends to a node is not
    among them. An expectation that involves more than two nodes holds whatever backend each of its
    nodes runs. These include the expectations of the following sections that are about traffic
    between nodes: in :doc:`swarm-traffic.md <swarm-traffic>`,
    :ref:`swarm-traffic-east-west-reachability`, :ref:`swarm-traffic-load-balancing-east-west-vip`
    with its subsections, and the :ref:`swarm-traffic-api-value-mode-ingress` subsection of
    :ref:`swarm-traffic-published-ports`; in this document, :ref:`swarm-lifecycle-encryption` and
    :ref:`swarm-lifecycle-failure-lifecycle`. They also include the expectations of
    :ref:`swarm-traffic-service-discovery` about a name whose workload is on another node. Each of
    those expectations keeps its own category and dependencies. They hold on cleartext and
    :drvopt:`encrypted` overlays and on the ingress network.

.. expectation:: Without VXLAN devices, a task on an overlay network does not run
    :id: SWARM-WIRE-NO-VXLAN
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-TASK-ERRORS
    :pins: SWARM-HOST-VXLAN

    *Applies to:* A Linux node whose kernel does not offer VXLAN devices

    On that node, a task attached to an overlay network does not reach the ``running`` state, and
    :api-op:`GET /tasks` reports an error for the task.

.. expectation:: A task on a node without VXLAN says creating a vxlan interface failed
    :id: SWARM-WIRE-NO-VXLAN-MSG
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :exact_form_of: SWARM-WIRE-NO-VXLAN
    :pins: SWARM-HOST-VXLAN

    *Applies to:* A Linux node whose kernel does not offer VXLAN devices

    On that node, the error that :api-op:`GET /tasks` reports for such a task on an overlay network
    contains :api-error:`error creating vxlan interface`.

.. _swarm-lifecycle-cleartext-overlay-networks:

Cleartext overlay networks
==========================

.. expectation:: Overlay traffic travels in UDP to the configured data-path port
    :id: SWARM-WIRECLEAR-UDP-PORT
    :observed_from: underlay
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-DATA-PATH-VXLAN
    :within: SWARM-CEIL-REASSIGNMENT

    UDP datagrams carry the traffic of overlay networks between hosts, the ingress network included,
    over IPv4 or IPv6, to the configured data-path port.

.. expectation:: Each datagram has a VXLAN header with the frame's overlay subnet VNI
    :id: SWARM-WIRECLEAR-VXLAN-VNI
    :observed_from: underlay
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-DATA-PATH-VXLAN
    :within: SWARM-CEIL-REASSIGNMENT

    A UDP datagram that carries the traffic of an overlay network between hosts, the ingress network
    included, has a VXLAN header. The header has the VNI of the overlay subnet that the frame in the
    datagram is on.

.. expectation:: The VXLAN payload is an Ethernet frame of container-to-container traffic
    :id: SWARM-WIRECLEAR-ETHERNET-PAYLOAD
    :observed_from: underlay
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-DATA-PATH-VXLAN
    :within: SWARM-CEIL-REASSIGNMENT

    The VXLAN payload of a UDP datagram that carries overlay traffic between hosts is an Ethernet
    frame that carries container-to-container traffic.

.. _swarm-lifecycle-encrypted-overlay-networks:

Encrypted overlay networks
==========================

Encrypted traffic between hosts is IPsec ESP. The expectations below are about the decrypted
payload. The decrypted payload is the same whatever the encryption is. To see the payload, an
underlay host must decrypt it with a node's keys. This subsection holds on Linux nodes.

.. expectation:: The decrypted payload is a UDP datagram to the configured data-path port
    :id: SWARM-WIREENC-UDP-PORT
    :observed_from: underlay (decrypted with a node's keys)
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-SWARM-DATA-PATH-VXLAN, REQ-SWARM-ENC-CONFIDENTIAL
    :needs: SWARM-HOST-XFRM, SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-IPTABLES-ENC-MATCHES, SWARM-HOST-IPTABLES-LEGACY-MANGLE-KERNEL, SWARM-HOST-IPTABLES-NFT-ROUTE-KERNEL
    :within: SWARM-CEIL-REASSIGNMENT

    The decrypted payload is a UDP datagram to the configured data-path port.

.. expectation:: The decrypted datagram has a VXLAN header with the frame's subnet VNI
    :id: SWARM-WIREENC-VXLAN-VNI
    :observed_from: underlay (decrypted with a node's keys)
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-SWARM-DATA-PATH-VXLAN, REQ-SWARM-ENC-CONFIDENTIAL
    :needs: SWARM-HOST-XFRM, SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-IPTABLES-ENC-MATCHES, SWARM-HOST-IPTABLES-LEGACY-MANGLE-KERNEL, SWARM-HOST-IPTABLES-NFT-ROUTE-KERNEL
    :within: SWARM-CEIL-REASSIGNMENT

    The UDP datagram in the decrypted payload has a VXLAN header. The header has the VNI of the
    overlay subnet that the frame in the datagram is on.

.. expectation:: The decrypted VXLAN payload is an Ethernet frame of container traffic
    :id: SWARM-WIREENC-ETHERNET-PAYLOAD
    :observed_from: underlay (decrypted with a node's keys)
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-SWARM-DATA-PATH-VXLAN, REQ-SWARM-ENC-CONFIDENTIAL
    :needs: SWARM-HOST-XFRM, SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-IPTABLES-ENC-MATCHES, SWARM-HOST-IPTABLES-LEGACY-MANGLE-KERNEL, SWARM-HOST-IPTABLES-NFT-ROUTE-KERNEL
    :within: SWARM-CEIL-REASSIGNMENT

    The VXLAN payload of the UDP datagram in the decrypted payload is an Ethernet frame that carries
    container-to-container traffic.

.. _swarm-lifecycle-failure-lifecycle:

*********************
 Failure & lifecycle
*********************

In this section and its subsections, a service's running tasks are those that
:ref:`swarm-traffic-service-discovery` in :doc:`swarm-traffic.md <swarm-traffic>` defines, and a VIP
load-balances across a set of tasks as :ref:`swarm-traffic-load-balancing-east-west-vip` in that
document defines it. Where an expectation in this section times a ceiling from when :api-op:`GET
/tasks` first reports a task's state as ``running``, the ceiling for a task with a healthcheck is
timed from when its container first reports healthy.

.. _swarm-lifecycle-restart-and-reclamation:

Restart and reclamation
=======================

In this subsection, a namespace, bridge, VXLAN device or HNS network that a killed daemon left is
one that existed on the node when the daemon was killed. One that the restarted daemon creates is
another object, even for the same network and under the same name or path: a namespace is told apart
by the inode of its namespace file, a bridge or VXLAN device by the namespace that holds it, and an
HNS network by its ID.

.. expectation:: After a restart, GET /info reports the node active within the ceiling
    :id: SWARM-RESTART-NODE-STATE-ACTIVE
    :observed_from: api:mgr, api:wkr
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-SWARM-RESTART-RECOVERY
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
    :refines: REQ-SWARM-RESTART-RECOVERY
    :within: SWARM-CEIL-REJOIN, SWARM-CEIL-REASSIGNMENT

    *Applies to:* A node, and a service whose tasks are constrained to it

    After a graceful restart of the node's daemon, each of the service's tasks on the node is
    reachable at its addresses on its overlay networks once the :need:`[[title|lower]] ceiling
    ([[seconds]] s) <SWARM-CEIL-REJOIN>` has passed from when the restart is complete, and the
    :need:`[[title|lower]] ceiling ([[seconds]] s) <SWARM-CEIL-REASSIGNMENT>` has passed from when
    :api-op:`GET /tasks` first reports that task's state as ``running``.

.. expectation:: After a graceful restart, published ports answer within the ceiling
    :id: SWARM-RESTART-GRACEFUL-PUBLISHED-PORTS
    :observed_from: client
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-RESTART-RECOVERY
    :needs: BR-HOST-NO-FORWARD-DROP
    :within: SWARM-CEIL-REJOIN, SWARM-CEIL-REASSIGNMENT

    After a graceful daemon restart, each of the node's published ports answers once the
    :need:`[[title|lower]] ceiling ([[seconds]] s) <SWARM-CEIL-REJOIN>` has passed from when the
    restart is complete, and the :need:`[[title|lower]] ceiling ([[seconds]] s)
    <SWARM-CEIL-REASSIGNMENT>` has passed from when :api-op:`GET /tasks` first reports the last of
    the tasks that the port is published for as ``running``.

.. expectation:: No event on a third node interrupts flows between other nodes
    :id: SWARM-RESTART-OTHER-NODES
    :observed_from: task, client
    :depends_on: os-linux, os-windows, upgrade
    :platforms: linux, windows
    :refines: REQ-SWARM-MIXED-VERSION-INTEROP, REQ-SWARM-RESTART-RECOVERY, REQ-SWARM-FAULT-TOLERANT
    :needs: BR-HOST-NO-FORWARD-DROP
    :within: SWARM-CEIL-REASSIGNMENT, SWARM-CEIL-REJOIN, SWARM-CEIL-FAILURE-DETECTION, SWARM-CEIL-PARTITION-HEAL, SWARM-CEIL-ALLOCATION

    *Applies to:* A flow to a task, from a client on a node to a VIP or from a client outside the
    Swarm to a port that a node publishes, and a third node that is neither that node nor the task's
    node

    No event on the third node interrupts the flow: no TCP flow is reset, and no TCP or UDP flow has
    a gap in delivery longer than 1 second. The events include a graceful restart of the third
    node's daemon, a kill and restart of it, an upgrade of its daemon from the previous release to
    the release under test, a restart of it with the iptables backend in place of nftables when it
    is a Linux node, a node partition between the third node and the other nodes in which the
    managers' quorum is on the other nodes' side, the third node leaving the Swarm or joining it,
    and a change of leader manager to or from the third node. For each event, this holds from when
    the event starts until the ceiling of its class has passed: the :need:`[[title|lower]] ceiling
    ([[seconds]] s) <SWARM-CEIL-REJOIN>` from when a restart of the third node's daemon is complete,
    whether graceful, after a kill, after an upgrade or with the other firewall backend, or from
    when its request to leave or join the Swarm responds; the :need:`[[title|lower]] ceiling
    ([[seconds]] s) <SWARM-CEIL-FAILURE-DETECTION>` from when the partition starts; the
    :need:`[[title|lower]] ceiling ([[seconds]] s) <SWARM-CEIL-PARTITION-HEAL>` from when the
    partition is removed, which is an event of its own; and the :need:`[[title|lower]] ceiling
    ([[seconds]] s) <SWARM-CEIL-ALLOCATION>` from the change of leader. This holds in the Swarm of
    :need:`SWARM-WIRE-MIXED-VERSIONS`, whose workers mix releases, when the client's node and the
    task's node run the release under test.

.. expectation:: What a killed daemon left for overlays goes within the rejoin ceiling
    :id: SWARM-RESTART-STALE-OVERLAY
    :observed_from: node
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-SWARM-RESTART-RECOVERY, PRIN-NO-LEAKS
    :within: SWARM-CEIL-REJOIN

    *Applies to:* A Linux node in a Swarm with an ingress network, whose daemon was killed, and a
    namespace, bridge or VXLAN device that the killed daemon left for an overlay network or the
    ingress network

    The node's daemon is started again. Within the :need:`[[title|lower]] ceiling ([[seconds]] s)
    <SWARM-CEIL-REJOIN>` after the restart is complete, no such namespace, bridge or VXLAN device
    remains on the node. A namespace remains while a mount of it remains in the mount namespace that
    the daemon runs in, and a bridge or VXLAN device remains while the namespace that holds it
    remains.

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
    :refines: REQ-SWARM-RESTART-RECOVERY
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A Windows node whose daemon was killed and started again, and an HNS overlay
    network that the killed daemon left for an overlay network

    Once a task or standalone container on that overlay network starts on the node, the HNS network
    that the killed daemon left does not remain.

.. expectation:: A start on a network with an overlapping VNI removes a stale HNS network
    :id: SWARM-RESTART-STALE-OVERLAY-WINDOWS-SHARED-VNI
    :observed_from: node
    :depends_on: os-windows
    :platforms: windows
    :refines: REQ-SWARM-RESTART-RECOVERY
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A Windows node whose daemon was killed and started again, and an HNS overlay
    network that the killed daemon left for an overlay network

    Once a task or standalone container starts on the node on another overlay network with a VNI
    that overlaps the HNS network's VNIs, the HNS network does not remain.

.. expectation:: A stale HNS network remains until a task starts on its network or VNI
    :id: SWARM-RESTART-STALE-OVERLAY-REMOVAL-WINDOWS
    :observed_from: node
    :depends_on: os-windows
    :platforms: windows
    :exact_form_of: SWARM-RESTART-STALE-OVERLAY-WINDOWS
    :pins: SWARM-RESTART-WINDOWS-HNS-ORPHANED
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A Windows node whose daemon was killed and started again, and an HNS overlay
    network that the killed daemon left for an overlay network

    The HNS network remains until a task or standalone container starts on the node on that overlay
    network, or on another overlay network with a VNI that overlaps the HNS network's VNIs. After
    that, the HNS network does not remain.

.. expectation:: A Windows daemon keeps the HNS networks that it finds at start
    :id: SWARM-RESTART-WINDOWS-HNS-ORPHANED
    :observed_from: node
    :depends_on: none
    :platforms: windows
    :within: SWARM-CEIL-REJOIN
    :concerns: SWARM-RESTART-STALE-OVERLAY-WINDOWS, SWARM-RESTART-STALE-OVERLAY-WINDOWS-SHARED-VNI
    :violates: PRIN-NO-LEAKS

    Every HNS overlay network that is on the node when the daemon starts stays in place. One of
    these networks is deleted only when a network with the same ID, or with an overlapping VNI, is
    programmed on the node. Thus the HNS network of a Swarm network that the node stopped using
    while the daemon was not running stays in place. The HNS endpoints that a killed daemon left on
    these networks also stay in place.

.. expectation:: After a kill and restart, a network on a VNI used before passes traffic
    :id: SWARM-RESTART-KILLED-VNI-REUSE
    :observed_from: node, task
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-RESTART-RECOVERY, REQ-SWARM-OVERLAY-CONNECTIVITY
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A node that used a VNI before its daemon was killed and restarted

    A task or container on a network that is allocated that VNI starts on the node, and the network
    passes traffic. This holds whether the network is the one that used the VNI before the kill or
    another network.

.. expectation:: After a kill and restart, node tasks are reachable within the ceiling
    :id: SWARM-RESTART-KILLED-TASKS
    :observed_from: task
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-RESTART-RECOVERY
    :within: SWARM-CEIL-REJOIN, SWARM-CEIL-REASSIGNMENT

    *Applies to:* A node, and a service whose tasks are constrained to it

    After the node's daemon is killed and restarted, each of the service's tasks on the node is
    reachable at its addresses on its overlay networks once the :need:`[[title|lower]] ceiling
    ([[seconds]] s) <SWARM-CEIL-REJOIN>` has passed from when the restart is complete, and the
    :need:`[[title|lower]] ceiling ([[seconds]] s) <SWARM-CEIL-REASSIGNMENT>` has passed from when
    :api-op:`GET /tasks` first reports that task's state as ``running``.

.. expectation:: After a kill and restart, published ports answer within the ceiling
    :id: SWARM-RESTART-KILLED-PUBLISHED-PORTS
    :observed_from: client
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-RESTART-RECOVERY
    :needs: BR-HOST-NO-FORWARD-DROP
    :within: SWARM-CEIL-REJOIN, SWARM-CEIL-REASSIGNMENT

    After the daemon is killed and restarted, each of the node's published ports answers once the
    :need:`[[title|lower]] ceiling ([[seconds]] s) <SWARM-CEIL-REJOIN>` has passed from when the
    restart is complete, and the :need:`[[title|lower]] ceiling ([[seconds]] s)
    <SWARM-CEIL-REASSIGNMENT>` has passed from when :api-op:`GET /tasks` first reports the last of
    the tasks that the port is published for as ``running``.

.. expectation:: After a restart, VIPs and tasks.<service> on other nodes reach its tasks
    :id: SWARM-RESTART-TASKS-FROM-OTHER-NODES
    :observed_from: task, ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-RESTART-RECOVERY, REQ-SWARM-VIP-LOAD-BALANCING, REQ-SWARM-SD-SERVICE-NAMES, REQ-SWARM-MESH-FOLLOWS-TASKS
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-NFTABLES-NAT, SWARM-HOST-NFT-COMMAND, SWARM-HOST-IPTABLES-LB-EXTENSIONS, SWARM-HOST-IPTABLES-LEGACY-MANGLE-KERNEL, DNS-HOST-NFTABLES-NAT, DNS-HOST-IPTABLES-COMMAND, DNS-HOST-NFT-COMMAND, DNS-HOST-IPTABLES-KERNEL, DNS-HOST-IPTABLES-LEGACY-KERNEL, DNS-HOST-IPTABLES-NFT-KERNEL
    :within: SWARM-CEIL-REJOIN, SWARM-CEIL-REASSIGNMENT

    *Applies to:* A node, a service in vip mode on an overlay network whose tasks are constrained to
    that node, and a task or container on that network on another node

    After the node's daemon restarts, gracefully or after it is killed, :dns-name:`tasks.<service>`
    resolves in that task or container to the addresses on that network of the service's running
    tasks, and its connections to the service's VIP reach them. Both hold once the
    :need:`[[title|lower]] ceiling ([[seconds]] s) <SWARM-CEIL-REJOIN>` has passed from when the
    restart is complete, and the :need:`[[title|lower]] ceiling ([[seconds]] s)
    <SWARM-CEIL-REASSIGNMENT>` has passed from when :api-op:`GET /tasks` first reports each of those
    tasks' state as ``running``.

.. expectation:: After a restart, published ports on other nodes reach its tasks
    :id: SWARM-RESTART-INGRESS-FROM-OTHER-NODES
    :observed_from: client
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-RESTART-RECOVERY, REQ-SWARM-ROUTING-MESH, REQ-SWARM-MESH-FOLLOWS-TASKS
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-NFTABLES-NAT, SWARM-HOST-NFT-COMMAND, SWARM-HOST-IPTABLES-LB-EXTENSIONS, SWARM-HOST-IPTABLES-LEGACY-MANGLE-KERNEL, BR-HOST-NO-FORWARD-DROP
    :within: SWARM-CEIL-REJOIN, SWARM-CEIL-REASSIGNMENT

    *Applies to:* A node, and a service whose tasks are constrained to that node and that publishes
    a port in ingress mode

    After the node's daemon restarts, gracefully or after it is killed, connections to the service's
    published port at the address of another node reach the service's running tasks. This holds once
    the :need:`[[title|lower]] ceiling ([[seconds]] s) <SWARM-CEIL-REJOIN>` has passed from when the
    restart is complete, and the :need:`[[title|lower]] ceiling ([[seconds]] s)
    <SWARM-CEIL-REASSIGNMENT>` has passed from when :api-op:`GET /tasks` first reports each of those
    tasks' state as ``running``.

.. expectation:: After a leave, GET /info reports the node inactive
    :id: SWARM-RESTART-LEAVE-INACTIVE
    :observed_from: api:solo
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-SWARM-LEAVE
    :within: SWARM-CEIL-REJOIN

    After :api-op:`POST /swarm/leave` responds 200 on a node, :api-op:`GET /info` on the node
    reports :api-field:`Swarm.LocalNodeState` as ``inactive``.

.. expectation:: After a leave, no Swarm-scoped network is listed or attached on the node
    :id: SWARM-RESTART-LEAVE-NETWORKS
    :observed_from: api:solo
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-SWARM-LEAVE-SWARM-NETWORKS-ONLY
    :within: SWARM-CEIL-REJOIN

    After :api-op:`POST /swarm/leave` responds 200 on a node, :api-op:`GET /networks` on that node
    does not list any Swarm-scoped network, the ingress network included. The node's standalone
    containers keep running, and none of them is attached to a Swarm-scoped network.

.. expectation:: On a worker, docker swarm leave removes every Swarm-scoped network
    :id: SWARM-RESTART-LEAVE-NETWORKS-CLI
    :observed_from: api:solo
    :depends_on: cli
    :platforms: linux, windows
    :refines: REQ-SWARM-LEAVE-SWARM-NETWORKS-ONLY
    :cli_for: SWARM-RESTART-LEAVE-NETWORKS
    :within: SWARM-CEIL-REJOIN

    On a worker, :cli:`docker swarm leave` exits zero. :cli:`docker network ls` on that node then
    does not list any Swarm-scoped network, the ingress network included.

.. expectation:: After a leave, a container keeps its networks that are not Swarm-scoped
    :id: SWARM-RESTART-LEAVE-LOCAL-NETWORKS-KEPT
    :observed_from: api:solo
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-SWARM-LEAVE-SWARM-NETWORKS-ONLY
    :within: SWARM-CEIL-REJOIN

    *Applies to:* A node in a Swarm with a running standalone container that is attached to a
    network that is not Swarm-scoped

    After :api-op:`POST /swarm/leave` responds 200 on the node, :api-op:`GET /containers/{id}/json`
    for the container still lists that network in :api-field:`NetworkSettings.Networks`, with a
    non-empty :api-field:`EndpointID`.

.. expectation:: Swarm leave disconnects containers from local attachable networks
    :id: SWARM-RESTART-LEAVE-LOCAL-ATTACHABLE
    :observed_from: api:solo, log
    :depends_on: none
    :platforms: linux, windows
    :within: SWARM-CEIL-REJOIN
    :narrows: SWARM-RESTART-LEAVE-LOCAL-NETWORKS-KEPT
    :concerns: SWARM-RESTART-LEAVE-NETWORKS
    :violates: REQ-SWARM-LEAVE-SWARM-NETWORKS-ONLY

    :api-op:`POST /swarm/leave` disconnects every container on the node from each network that was
    created with :api-field:`Attachable`. This includes node-local networks, such as user-defined
    bridge networks. The containers stay disconnected after the leave, and the node-local network
    stays. A warning that contains :log:`is not a dynamic network` appears in the daemon log.

.. _swarm-lifecycle-convergence:

Convergence
===========

There are five convergence classes, each with a different dominant term. One bound cannot serve all
of them.

Each class has a ceiling. The ceiling is chosen by judgment to encode "a human would call this
broken". It is not lowered to follow measured convergence times. It bounds convergence that goes
from seconds to minutes, or that never completes.

.. ceiling:: Reassignment
    :id: SWARM-CEIL-REASSIGNMENT
    :seconds: 300

    An address moving to another endpoint, a task replaced by one on another node, or a new task,
    service or endpoint. :ref:`README.md <readme-timing>` lists the other changes in this class.

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

    A node's :api-op:`POST /swarm/init`, :api-op:`POST /swarm/join` or :api-op:`POST /swarm/leave`,
    on that node and on the other nodes, and a restart of a node's daemon.

    Its dominant term is full cluster re-formation and network re-programming.

    For an init, a join or a leave, the change is complete when the request responds.

.. ceiling:: Allocation
    :id: SWARM-CEIL-ALLOCATION
    :seconds: 30

    A Swarm-scoped network that can be allocated, or the endpoint (VIPs and published ports) of a
    service whose networks are allocated. Also the leader's failed attempt to allocate a network or
    a service's endpoint, and what follows from it. :ref:`README.md <readme-timing>` lists the other
    changes in this class.

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
bounded too: :need:`SWARM-RESTART-NODE-STATE-ACTIVE`, or :need:`SWARM-BACKEND-NODE-STATE-ACTIVE` for
a restart with the nftables backend in place of iptables, times it from when the daemon's API
responds again. For a change in a task's health, or the exit of its container, the change is
complete when the task's node emits the container's :api-event:`health_status` or :api-event:`die`
event. For a change of a task's desired state away from ``running``, the change is complete when
:api-op:`GET /tasks` first reports the new :api-field:`DesiredState`. For a change of leader, the
change is complete when :api-op:`GET /nodes` on a manager reports :api-field:`ManagerStatus.Leader`
as true for the new leader. For an expectation about what holds once no container of a service is
running, the change is complete when the last of those containers emits its :api-event:`die` event.

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
    :refines: REQ-SWARM-ADDRESS-REUSE
    :within: SWARM-CEIL-REASSIGNMENT

    After overlay address X moves from endpoint S1 to S2, a client that could reach X at S1 reaches
    it at S2. The client does this within the :need:`[[title|lower]] ceiling ([[seconds]] s)
    <SWARM-CEIL-REASSIGNMENT>`, without a manual :program:`ip neigh`, :program:`arp -d` or host
    flush. This holds whether S2 is on S1's node or on another node. It also holds whether the
    client is on S1's node, S2's node or a third node.

.. expectation:: Repeated moves of an address converge, even when learned out of order
    :id: SWARM-CONVERGE-ADDRESS-MOVE-REPEATED
    :observed_from: task, ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-ADDRESS-REUSE
    :within: SWARM-CEIL-REASSIGNMENT

    Overlay address X moves any number of times between endpoints. Within the :need:`[[title|lower]]
    ceiling ([[seconds]] s) <SWARM-CEIL-REASSIGNMENT>`, a client that could reach X at its previous
    endpoint reaches X at its current endpoint. This also holds on a node that learns of the removal
    of X's previous endpoint only after it learned of X's new endpoint. This holds on the node of
    the new endpoint and on every other node.

.. expectation:: A moved address is reachable whether or not its MAC address moves too
    :id: SWARM-CONVERGE-ADDRESS-MOVE-MAC
    :observed_from: task, ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-ADDRESS-REUSE
    :within: SWARM-CEIL-REASSIGNMENT

    Overlay address X moves between endpoints. Within the :need:`[[title|lower]] ceiling
    ([[seconds]] s) <SWARM-CEIL-REASSIGNMENT>`, a client that could reach X at its previous endpoint
    reaches X at its new endpoint. This holds whether or not the MAC address moves with X.

.. expectation:: A client reaches a task's replacement on another node within the ceiling
    :id: SWARM-CONVERGE-TASK-REPLACED
    :observed_from: task, ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-MESH-FOLLOWS-TASKS
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-NFTABLES-NAT, SWARM-HOST-NFT-COMMAND, SWARM-HOST-IPTABLES-LB-EXTENSIONS, SWARM-HOST-IPTABLES-LEGACY-MANGLE-KERNEL, DNS-HOST-NFTABLES-NAT, DNS-HOST-IPTABLES-COMMAND, DNS-HOST-NFT-COMMAND, DNS-HOST-IPTABLES-KERNEL, DNS-HOST-IPTABLES-LEGACY-KERNEL, DNS-HOST-IPTABLES-NFT-KERNEL
    :within: SWARM-CEIL-REASSIGNMENT

    When a task is replaced by one on another node, a client that could reach the old task reaches
    the new one. It reaches the new task at the address that :api-op:`GET /tasks` reports for it, at
    the address that :dns-name:`tasks.<service>` resolves to, and, in vip mode, through the
    service's VIP. The client does this within the :need:`[[title|lower]] ceiling ([[seconds]] s)
    <SWARM-CEIL-REASSIGNMENT>`, without manual intervention. This holds whether the client is on the
    old task's node, the new task's node or a third node.

.. expectation:: Nodes hold no neighbor or FDB entries for a failed side's endpoints
    :id: SWARM-CONVERGE-PARTITION-NO-ENTRIES
    :observed_from: node
    :depends_on: os-linux
    :platforms: linux
    :within: SWARM-CEIL-FAILURE-DETECTION
    :rationale: RAT-SWARM-FORWARDING-ENTRY-FAULTS-LATENT

    *Applies to:* Linux nodes on one side of a node partition

    Those nodes do not hold overlay neighbor or FDB entries for the other side's endpoints.

.. expectation:: On the quorum side of a partition, VIP flows reach that side's tasks
    :id: SWARM-CONVERGE-PARTITION-QUORUM-VIP
    :observed_from: task, ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-MESH-FOLLOWS-TASKS, REQ-SWARM-FAULT-TOLERANT
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-NFTABLES-NAT, SWARM-HOST-NFT-COMMAND, SWARM-HOST-IPTABLES-LB-EXTENSIONS, SWARM-HOST-IPTABLES-LEGACY-MANGLE-KERNEL
    :within: SWARM-CEIL-FAILURE-DETECTION, SWARM-CEIL-REASSIGNMENT

    *Applies to:* A node partition, and a service with running tasks on both sides

    Within the :need:`[[title|lower]] ceiling ([[seconds]] s) <SWARM-CEIL-FAILURE-DETECTION>`, every
    flow that starts on a node of the side with the managers' quorum and goes to the service's VIP
    reaches a task on that side.

.. expectation:: A VXLAN device holds no neighbor or FDB entry for its node's endpoints
    :id: SWARM-CONVERGE-NO-OWN-ENTRIES
    :observed_from: node
    :depends_on: os-linux
    :platforms: linux
    :within: SWARM-CEIL-REASSIGNMENT
    :rationale: RAT-SWARM-FORWARDING-ENTRY-FAULTS-LATENT

    A node's VXLAN device does not hold a neighbor or FDB entry for any of the node's own endpoints.

.. expectation:: After a partition heals, each side reaches the other's overlay addresses
    :id: SWARM-CONVERGE-PARTITION-HEAL
    :observed_from: task, ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-REJOIN-RECOVERY
    :within: SWARM-CEIL-PARTITION-HEAL, SWARM-CEIL-REASSIGNMENT

    *Applies to:* A node partition in which each side has declared the other side's nodes failed

    After the partition is removed, each task or container on one side reaches each task or
    container on the other side at its address on every overlay network that both are attached to.
    This holds for each such end that is running when the partition is removed, including a task
    that the orchestrator started during the partition, once the :need:`[[title|lower]] ceiling
    ([[seconds]] s) <SWARM-CEIL-PARTITION-HEAL>` has passed from when the partition is removed, and
    the :need:`[[title|lower]] ceiling ([[seconds]] s) <SWARM-CEIL-REASSIGNMENT>` has passed from
    when the last of the two ends runs.

.. expectation:: Nodes whose data path is blocked keep listing each other in Peers
    :id: SWARM-CONVERGE-PARTIAL-BLOCK
    :observed_from: api:active
    :depends_on: none
    :platforms: linux, windows
    :within: SWARM-CEIL-FAILURE-DETECTION
    :rationale: RAT-SWARM-PEERS-FROM-GOSSIP

    While a firewall between two nodes drops their data-path traffic (UDP to the data-path port, or
    ESP on an :drvopt:`encrypted` network), in one direction or both, and lets their gossip through,
    each node keeps listing the other in the :api-field:`Peers` that :api-op:`GET /networks/{id}`
    reports for each overlay network that both nodes have a local attachment to.

.. expectation:: Reachability restored after a partition heals outlasts FDB aging
    :id: SWARM-CONVERGE-PARTITION-HEAL-PERSISTS
    :observed_from: task, ctr
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-SWARM-REJOIN-RECOVERY
    :within: SWARM-CEIL-PARTITION-HEAL, SWARM-CEIL-REASSIGNMENT

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
    :refines: REQ-SWARM-REJOIN-RECOVERY
    :within: SWARM-CEIL-PARTITION-HEAL, SWARM-CEIL-REASSIGNMENT

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
    :refines: REQ-SWARM-REJOIN-RECOVERY
    :within: SWARM-CEIL-REJOIN, SWARM-CEIL-REASSIGNMENT

    A node leaves the Swarm with :api-op:`POST /swarm/leave` and rejoins with :api-op:`POST
    /swarm/join`. A task placed on the node after the rejoin is reachable at its overlay addresses
    from that node and from other nodes. It is reachable once the :need:`[[title|lower]] ceiling
    ([[seconds]] s) <SWARM-CEIL-REJOIN>` has passed from the response to :api-op:`POST /swarm/join`,
    and the :need:`[[title|lower]] ceiling ([[seconds]] s) <SWARM-CEIL-REASSIGNMENT>` has passed
    from when :api-op:`GET /tasks` first reports the task's state as ``running``. Such tasks include
    a global service's tasks.

.. expectation:: After a leave and rejoin, published ports answer within the ceiling
    :id: SWARM-CONVERGE-REJOIN-PUBLISHED-PORTS
    :observed_from: client
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-REJOIN-RECOVERY
    :needs: BR-HOST-NO-FORWARD-DROP
    :within: SWARM-CEIL-REJOIN, SWARM-CEIL-REASSIGNMENT

    After a node leaves the Swarm with :api-op:`POST /swarm/leave` and rejoins with :api-op:`POST
    /swarm/join`, each of its published ports answers once the :need:`[[title|lower]] ceiling
    ([[seconds]] s) <SWARM-CEIL-REJOIN>` has passed from the response to :api-op:`POST /swarm/join`,
    and the :need:`[[title|lower]] ceiling ([[seconds]] s) <SWARM-CEIL-REASSIGNMENT>` has passed
    from when :api-op:`GET /tasks` first reports the last of the tasks that the port is published
    for as ``running``.

.. expectation:: After a rejoin, published ports on any node reach the node's new tasks
    :id: SWARM-CONVERGE-REJOIN-INGRESS-BACKENDS
    :observed_from: client
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-REJOIN-RECOVERY, REQ-SWARM-MESH-FOLLOWS-TASKS
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-NFTABLES-NAT, SWARM-HOST-NFT-COMMAND, SWARM-HOST-IPTABLES-LB-EXTENSIONS, SWARM-HOST-IPTABLES-LEGACY-MANGLE-KERNEL, BR-HOST-NO-FORWARD-DROP
    :within: SWARM-CEIL-REJOIN, SWARM-CEIL-REASSIGNMENT

    A node leaves the Swarm with :api-op:`POST /swarm/leave` and rejoins with :api-op:`POST
    /swarm/join`. Connections to a service's published port on any node reach the service's running
    tasks placed on the rejoined node after the rejoin. They reach such a task once the
    :need:`[[title|lower]] ceiling ([[seconds]] s) <SWARM-CEIL-REJOIN>` has passed from the response
    to :api-op:`POST /swarm/join`, and the :need:`[[title|lower]] ceiling ([[seconds]] s)
    <SWARM-CEIL-REASSIGNMENT>` has passed from when :api-op:`GET /tasks` first reports the task's
    state as ``running``. An example is a global service's tasks.

.. expectation:: After a leave and rejoin, other nodes' VIPs and names reach its new tasks
    :id: SWARM-CONVERGE-REJOIN-TASKS-FROM-OTHER-NODES
    :observed_from: task, ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-REJOIN-RECOVERY, REQ-SWARM-VIP-LOAD-BALANCING, REQ-SWARM-SD-SERVICE-NAMES, REQ-SWARM-MESH-FOLLOWS-TASKS
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-NFTABLES-NAT, SWARM-HOST-NFT-COMMAND, SWARM-HOST-IPTABLES-LB-EXTENSIONS, SWARM-HOST-IPTABLES-LEGACY-MANGLE-KERNEL, DNS-HOST-NFTABLES-NAT, DNS-HOST-IPTABLES-COMMAND, DNS-HOST-NFT-COMMAND, DNS-HOST-IPTABLES-KERNEL, DNS-HOST-IPTABLES-LEGACY-KERNEL, DNS-HOST-IPTABLES-NFT-KERNEL
    :within: SWARM-CEIL-REJOIN, SWARM-CEIL-REASSIGNMENT

    *Applies to:* A service in vip mode on an overlay network, and a task or container on that
    network on a node other than the one that leaves and rejoins

    A node leaves the Swarm with :api-op:`POST /swarm/leave` and rejoins with :api-op:`POST
    /swarm/join`. :dns-name:`tasks.<service>` resolves in that task or container to the addresses on
    that network of the service's running tasks, including those placed on the rejoined node after
    the rejoin, and its connections to the service's VIP reach them. Both hold once the
    :need:`[[title|lower]] ceiling ([[seconds]] s) <SWARM-CEIL-REJOIN>` has passed from the response
    to :api-op:`POST /swarm/join`, and the :need:`[[title|lower]] ceiling ([[seconds]] s)
    <SWARM-CEIL-REASSIGNMENT>` has passed from when :api-op:`GET /tasks` first reports each of those
    tasks' state as ``running``.

.. expectation:: After a node leaves, new VIP and port flows reach the remaining tasks
    :id: SWARM-CONVERGE-LEAVE-REMAINING-TASKS
    :observed_from: task, ctr, client
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-MESH-FOLLOWS-TASKS, REQ-SWARM-FAULT-TOLERANT
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-NFTABLES-NAT, SWARM-HOST-NFT-COMMAND, SWARM-HOST-IPTABLES-LB-EXTENSIONS, SWARM-HOST-IPTABLES-LEGACY-MANGLE-KERNEL, BR-HOST-NO-FORWARD-DROP
    :within: SWARM-CEIL-REJOIN, SWARM-CEIL-REASSIGNMENT

    *Applies to:* A service with a running task on each of two or more nodes

    One of those nodes leaves the Swarm with :api-op:`POST /swarm/leave`. Within the
    :need:`[[title|lower]] ceiling ([[seconds]] s) <SWARM-CEIL-REJOIN>` after the leave, every new
    flow to the service's VIP from a node still in the Swarm reaches a running task on a node still
    in the Swarm. So does every new flow to the service's published port on such a node.

.. expectation:: Other nodes hold no neighbor or FDB entries for a node that left
    :id: SWARM-CONVERGE-LEAVE-NO-ENTRIES
    :observed_from: node
    :depends_on: os-linux
    :platforms: linux
    :refines: PRIN-NO-LEAKS
    :within: SWARM-CEIL-REJOIN

    Within the :need:`[[title|lower]] ceiling ([[seconds]] s) <SWARM-CEIL-REJOIN>` after
    :api-op:`POST /swarm/leave` responds 200 on a node, the other nodes do not hold overlay neighbor
    or FDB entries for its endpoints.

In the expectations of this subsection, a node's overlay neighbor and FDB entries are the permanent
entries that the daemon programs. Entries that the kernel learns from traffic are not included.
:need:`SWARM-CONVERGE-PARTITION-HEAL-PERSISTS` holds whether or not containers send traffic as the
partition heals. :need:`SWARM-CONVERGE-PARTITION-HEAL-PERSISTS-ACTIVE` names the case in which they
do. The expectations name ``os-linux`` because how learned and permanent FDB entries interact is
behavior of the kernel's VXLAN driver.

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
    :refines: REQ-SWARM-ROLLING-UPDATE
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-NFTABLES-NAT, SWARM-HOST-NFT-COMMAND, SWARM-HOST-IPTABLES-LB-EXTENSIONS, SWARM-HOST-IPTABLES-LEGACY-MANGLE-KERNEL
    :within: SWARM-CEIL-ALLOCATION, SWARM-CEIL-REASSIGNMENT

    A service has a rolling update in which at least one task keeps running throughout, and that
    changes neither the service's endpoint mode nor its :api-field:`TaskTemplate.Networks`. Examples
    are an update in start-first order and an update of fewer tasks at a time than are running.
    During the update, the VIP does not stop answering for longer than the :need:`[[title|lower]]
    ceiling ([[seconds]] s) <SWARM-CEIL-ALLOCATION>`: new flows to the VIP started more than that
    length apart, and every new flow to the VIP started between them, do not all fail to answer.

.. expectation:: After a rolling update, new VIP flows reach running tasks in the ceiling
    :id: SWARM-CHURN-ROLLING-UPDATE-COMPLETE
    :observed_from: task, ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-ROLLING-UPDATE, REQ-SWARM-MESH-FOLLOWS-TASKS
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-NFTABLES-NAT, SWARM-HOST-NFT-COMMAND, SWARM-HOST-IPTABLES-LB-EXTENSIONS, SWARM-HOST-IPTABLES-LEGACY-MANGLE-KERNEL
    :within: SWARM-CEIL-REASSIGNMENT

    A service has a rolling update in which at least one task keeps running throughout, and that
    changes neither the service's endpoint mode nor its :api-field:`TaskTemplate.Networks`. Examples
    are an update in start-first order and an update of fewer tasks at a time than are running. Once
    the update completes, every new flow to the service's VIP reaches a running task within the
    :need:`[[title|lower]] ceiling ([[seconds]] s) <SWARM-CEIL-REASSIGNMENT>`.

.. expectation:: After rapid scaling between 1 and 5, VIP flows reach running tasks
    :id: SWARM-CHURN-SCALE-FLAPPING
    :observed_from: task, ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-MESH-FOLLOWS-TASKS
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-NFTABLES-NAT, SWARM-HOST-NFT-COMMAND, SWARM-HOST-IPTABLES-LB-EXTENSIONS, SWARM-HOST-IPTABLES-LEGACY-MANGLE-KERNEL
    :within: SWARM-CEIL-REASSIGNMENT

    A service's replica count is changed at least 10 times, alternating between 1 and 5. Each update
    is sent as soon as the previous update responds. Once the scaling stops, every new flow to the
    service's VIP reaches a running task within the :need:`[[title|lower]] ceiling ([[seconds]] s)
    <SWARM-CEIL-REASSIGNMENT>`.

.. expectation:: Other nodes send new flows to a task after it stops
    :id: SWARM-CHURN-STOPPING-TASK-FLOWS
    :observed_from: task, client
    :depends_on: none
    :platforms: linux, windows
    :within: SWARM-CEIL-REASSIGNMENT
    :narrows: SWARM-PORTALLOC-PORT-IN-USE-OLD-TASKS
    :concerns: SWARM-LBHEALTH-DRAIN-REMOTE, SWARM-CHURN-ROLLING-UPDATE-VIP, SWARM-CHURN-ROLLING-UPDATE-COMPLETE, SWARM-CHURN-SCALE-FLAPPING, SWARM-PUB-SERVICE-REMOVED, SWARM-PUB-PORT-CHANGE-OLD
    :violates: REQ-SWARM-ROLLING-UPDATE, REQ-SWARM-MESH-FOLLOWS-TASKS, REQ-SWARM-LB-GRACEFUL-DRAIN

    A task leaves load balancing on its own node at once, and 2 s later the daemon sends its
    container the stop signal (the container's first :api-event:`kill` event), whether or not the
    other nodes have withdrawn the task by then. A node that has not withdrawn the task when its
    container exits (its :api-event:`die` event) sends new flows to it after that, through the VIP
    and through published ports, and those flows fail.

.. expectation:: Each network left by a racing delete and create is deleted on request
    :id: SWARM-CHURN-DELETE-CREATE
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-NETWORK-DELETE

    *Applies to:* A Swarm-scoped network

    :api-op:`DELETE /networks/{id}` for the network, by its ID, runs concurrently with :api-op:`POST
    /networks/create` for a network with the same name and driver. Afterwards, :api-op:`DELETE
    /networks/{id}` for each network with that name that remains responds 204, and :api-op:`GET
    /networks` then does not list it.

.. expectation:: After a racing delete and create, the name is created again
    :id: SWARM-CHURN-DELETE-CREATE-RECREATE
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-NETWORK-CREATE-ON-MANAGER

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
    :refines: REQ-SWARM-NETWORK-DELETE, REQ-SWARM-ATTACH-FAILURE-START
    :within: SWARM-CEIL-REASSIGNMENT

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
    :refines: REQ-SWARM-ATTACH-FAILURE-START, REQ-SWARM-NETWORK-DELETE

    *Applies to:* A created or stopped standalone container on a Swarm-scoped attachable network

    :api-op:`DELETE /networks/{id}` on a manager for that network races :api-op:`POST
    /containers/{id}/start` for the container. When the delete responds 204, the start responds with
    a 4xx or 5xx status.

.. expectation:: When a racing delete succeeds, the container start responds 500
    :id: SWARM-CHURN-DELETE-CTR-START-REFUSED-HTTP500
    :observed_from: api:mgr, api:active
    :depends_on: none
    :platforms: linux, windows
    :exact_form_of: SWARM-CHURN-DELETE-CTR-START-REFUSED
    :pins: SWARM-CLUSTER-HTTP500-REFUSED, SWARM-SCOPED-ATTACH-FAILURE-TIMEOUT
    :concerns: SWARM-CHURN-DELETE-CTR-START-REFUSED
    :violates: PRIN-ERRORS-ACTIONABLE, REQ-NET-API-ERROR-STATUS

    *Applies to:* A created or stopped standalone container on a Swarm-scoped attachable network

    :api-op:`DELETE /networks/{id}` on a manager for that network races :api-op:`POST
    /containers/{id}/start` for the container. When the delete responds 204, the start gives one of
    two responses. The first is 500 at once, with a message that contains :api-error:`network <ref>
    not found`. ``<ref>`` is the network's full ID if the container has been started before.
    Otherwise it is the network name or ID that the container was created with. The second is 500
    after 20 seconds, with :api-error:`attaching to network failed, make sure your network options
    are correct and check manager logs: context deadline exceeded`.

.. expectation:: A delete racing the last task's shutdown fails or removes the network
    :id: SWARM-CHURN-DELETE-SHUTDOWN
    :observed_from: api:mgr, api:active
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-SWARM-NETWORK-DELETE
    :within: SWARM-CEIL-REASSIGNMENT

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
    :refines: REQ-SWARM-NETWORK-DELETE, REQ-SWARM-ATTACH-FAILURE-START
    :cli_for: SWARM-CHURN-DELETE-CTR-START
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A Swarm-scoped attachable network

    When :cli:`docker network rm` on a manager races :cli:`docker run -d --network` naming that
    network, one of two outcomes occurs. In the first, :cli:`docker network rm` exits non-zero, and
    the container runs attached to the network. In the second, :cli:`docker network rm` exits zero,
    :cli:`docker network ls` on a manager then does not list the network, and :cli:`docker run -d`
    exits with status 125.

.. expectation:: After a change from vip to dnsrr, the service name resolves to its tasks
    :id: SWARM-CHURN-VIP-TO-DNSRR-RESOLVES
    :observed_from: task, ctr
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-SWARM-ENDPOINT-MODE-UPDATE
    :needs: DNS-HOST-NFTABLES-NAT, DNS-HOST-IPTABLES-COMMAND, DNS-HOST-NFT-COMMAND, DNS-HOST-IPTABLES-KERNEL, DNS-HOST-IPTABLES-LEGACY-KERNEL, DNS-HOST-IPTABLES-NFT-KERNEL
    :within: SWARM-CEIL-REASSIGNMENT

    After :api-op:`POST /services/{id}/update` changes a service's endpoint mode from ``vip`` to
    ``dnsrr``, :dns-name:`<service>` resolves to the addresses of the running tasks.

.. expectation:: After a change from vip to dnsrr, the old VIP does not reach the service
    :id: SWARM-CHURN-VIP-TO-DNSRR-OLD-VIP
    :observed_from: task, ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-ENDPOINT-MODE-UPDATE
    :within: SWARM-CEIL-REASSIGNMENT

    After a service's endpoint mode changes from ``vip`` to ``dnsrr``, its old VIP does not reach
    the service.

.. expectation:: After a change from dnsrr to vip, the service name resolves to its VIP
    :id: SWARM-CHURN-DNSRR-TO-VIP-RESOLVES
    :observed_from: task, ctr
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-SWARM-ENDPOINT-MODE-UPDATE
    :needs: DNS-HOST-NFTABLES-NAT, DNS-HOST-IPTABLES-COMMAND, DNS-HOST-NFT-COMMAND, DNS-HOST-IPTABLES-KERNEL, DNS-HOST-IPTABLES-LEGACY-KERNEL, DNS-HOST-IPTABLES-NFT-KERNEL
    :within: SWARM-CEIL-REASSIGNMENT

    After :api-op:`POST /services/{id}/update` changes a service's endpoint mode from ``dnsrr`` to
    ``vip``, :dns-name:`<service>` resolves to the service VIP.

.. expectation:: After a change from dnsrr to vip, the VIP load-balances across the tasks
    :id: SWARM-CHURN-DNSRR-TO-VIP-BALANCES
    :observed_from: task, ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-ENDPOINT-MODE-UPDATE
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-NFTABLES-NAT, SWARM-HOST-NFT-COMMAND, SWARM-HOST-IPTABLES-LB-EXTENSIONS, SWARM-HOST-IPTABLES-LEGACY-MANGLE-KERNEL
    :within: SWARM-CEIL-REASSIGNMENT

    After a service's endpoint mode changes from ``dnsrr`` to ``vip``, the service's VIP
    load-balances across the service's running tasks.

.. expectation:: A service is reachable on a network that an update adds to it
    :id: SWARM-CHURN-NETWORK-ADD
    :observed_from: task, ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-SERVICE-NETWORK-UPDATE
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-NFTABLES-NAT, SWARM-HOST-NFT-COMMAND, SWARM-HOST-IPTABLES-LB-EXTENSIONS, SWARM-HOST-IPTABLES-LEGACY-MANGLE-KERNEL, DNS-HOST-NFTABLES-NAT, DNS-HOST-IPTABLES-COMMAND, DNS-HOST-NFT-COMMAND, DNS-HOST-IPTABLES-KERNEL, DNS-HOST-IPTABLES-LEGACY-KERNEL, DNS-HOST-IPTABLES-NFT-KERNEL
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A service, an overlay network that the service is not attached to, and a client on
    that network that shares no other network with the service

    After :api-op:`POST /services/{id}/update` adds the network to
    :api-field:`TaskTemplate.Networks`, :dns-name:`tasks.<service>` resolves in the client to the
    addresses on that network of the service's running tasks, and the client reaches each of those
    tasks at that address. For a service in the ``vip`` endpoint mode, :dns-name:`<service>`
    resolves in the client to the service's VIP on that network, and the client's connections to
    that VIP reach the service's running tasks.

.. expectation:: A service is not reachable on a network that an update removes from it
    :id: SWARM-CHURN-NETWORK-REMOVE
    :observed_from: task, ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-SERVICE-NETWORK-UPDATE
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A service attached to a Swarm-scoped network, and a client on that network that
    shares no other network with the service

    After :api-op:`POST /services/{id}/update` removes the network from
    :api-field:`TaskTemplate.Networks`, neither :dns-name:`<service>` nor
    :dns-name:`tasks.<service>` resolves in the client to an address of any of the service's tasks
    or to the VIP that the service had on that network, and the client's connections to that VIP
    reach none of the service's tasks.

.. expectation:: A service reports no VIP on a network that an update removes
    :id: SWARM-CHURN-NETWORK-REMOVE-VIP
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-SERVICE-NETWORK-UPDATE
    :within: SWARM-CEIL-ALLOCATION

    *Applies to:* A service attached to a Swarm-scoped network

    After :api-op:`POST /services/{id}/update` removes the network from
    :api-field:`TaskTemplate.Networks`, :api-op:`GET /services/{id}` does not report a VIP on that
    network.

.. expectation:: A service update that adds a network stores it
    :id: SWARM-CHURN-NETWORK-ADD-SPEC
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-SERVICE-NETWORK-UPDATE

    *Applies to:* A service, and a Swarm-scoped network that the service is not attached to

    :api-op:`POST /services/{id}/update` that adds the network to :api-field:`TaskTemplate.Networks`
    responds 200. :api-op:`GET /services/{id}` then reports the network in
    :api-field:`TaskTemplate.Networks`.

.. expectation:: docker service update --network-add adds the network to the service
    :id: SWARM-CHURN-NETWORK-ADD-SPEC-CLI
    :observed_from: api:mgr
    :depends_on: cli
    :platforms: linux
    :refines: REQ-SWARM-SERVICE-NETWORK-UPDATE
    :cli_for: SWARM-CHURN-NETWORK-ADD-SPEC

    *Applies to:* A service, and a Swarm-scoped network that the service is not attached to

    :cli:`docker service update --network-add` naming that network, for the service, exits zero.
    :cli:`docker service inspect` then prints the network in :api-field:`TaskTemplate.Networks`.

.. expectation:: A service update that removes a network stores the removal
    :id: SWARM-CHURN-NETWORK-REMOVE-SPEC
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-SERVICE-NETWORK-UPDATE

    *Applies to:* A service attached to a Swarm-scoped network

    :api-op:`POST /services/{id}/update` that removes the network from
    :api-field:`TaskTemplate.Networks` responds 200. :api-op:`GET /services/{id}` then does not
    report the network in :api-field:`TaskTemplate.Networks`.

.. expectation:: docker service update --network-rm removes the network from the service
    :id: SWARM-CHURN-NETWORK-REMOVE-SPEC-CLI
    :observed_from: api:mgr
    :depends_on: cli
    :platforms: linux
    :refines: REQ-SWARM-SERVICE-NETWORK-UPDATE
    :cli_for: SWARM-CHURN-NETWORK-REMOVE-SPEC

    *Applies to:* A service attached to a Swarm-scoped network

    :cli:`docker service update --network-rm` naming that network, for the service, exits zero.
    :cli:`docker service inspect` then does not print the network in
    :api-field:`TaskTemplate.Networks`.

.. _swarm-lifecycle-scale-and-exhaustion:

Scale and exhaustion
====================

.. expectation:: After create-delete cycles, a deleted network's VNI is accepted again
    :id: SWARM-SCALE-VNI-REUSE
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-OVERLAY-VNI-LIST
    :within: SWARM-CEIL-ALLOCATION, SWARM-CEIL-REASSIGNMENT

    After at least 10 cycles in which an overlay network is created and deleted, :api-op:`POST
    /networks/create` for an overlay network, with a :drvopt:`vxlanid_list` that has a VNI that a
    deleted network used, responds 201. :api-op:`GET /networks/{id}` on a manager then reports that
    VNI in :api-field:`Options`.

.. expectation:: A removed service's ingress port is accepted for a new service
    :id: SWARM-SCALE-INGRESS-PORT-REUSE
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-PORT-ALLOCATION
    :within: SWARM-CEIL-ALLOCATION, SWARM-CEIL-REASSIGNMENT

    *Applies to:* A Swarm with a service that publishes a port in ingress mode

    After the service is removed, :api-op:`POST /services/create` for another service that publishes
    the same port responds 201. :api-op:`GET /services/{id}` then reports that port.

.. expectation:: Allocated VNIs differ from the VNIs in other networks' vxlanid_list
    :id: SWARM-SCALE-VNI-AUTO-DISTINCT
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :within: SWARM-CEIL-ALLOCATION
    :rationale: RAT-SWARM-AUTO-VNI-DISTINCT

    An overlay network is created without :drvopt:`vxlanid_list`. The VNIs allocated to the network
    differ from the VNIs in the :drvopt:`vxlanid_list` of each other overlay network that exists
    when the network is created.

.. expectation:: At least 100 overlay networks pass traffic between two nodes
    :id: SWARM-SCALE-MANY-NETWORKS
    :observed_from: task
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-OVERLAY-CONNECTIVITY
    :within: SWARM-CEIL-REASSIGNMENT

    At least 100 overlay networks, each with a task on each of two nodes, all pass traffic between
    the nodes.

.. expectation:: The leader logs an error that it failed to allocate the network
    :id: SWARM-SCALE-UNALLOCATABLE-LOG
    :observed_from: log
    :depends_on: none
    :platforms: linux
    :pins: SWARM-OVL-ALLOCATION-FAILURE-UNREPORTED
    :within: SWARM-CEIL-ALLOCATION

    *Applies to:* An overlay network that cannot be allocated

    The daemon log of the leader manager reports the failure at error level, in an entry that
    contains :log:`Failed allocation for network` and the network's ID. For a network that a manager
    finds unallocated when it becomes leader, the entry contains :log:`failed allocating network`
    and the network's ID in place of that text.

.. expectation:: An unallocatable network inspects with empty Driver, IPAM and Options
    :id: SWARM-SCALE-UNALLOCATABLE-INSPECT
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :pins: SWARM-OVL-ALLOCATION-FAILURE-UNREPORTED
    :within: SWARM-CEIL-ALLOCATION

    *Applies to:* An overlay network that cannot be allocated

    On a manager, :api-op:`GET /networks/{id}` for that network responds 200 with an empty
    :api-field:`Driver`, :api-field:`IPAM` and :api-field:`Options`, and without
    :api-field:`Status`. This is so even when :api-op:`POST /networks/create` named a driver or a
    subnet. The response does not report the failure in any field.

.. expectation:: A container start on an unallocatable network fails
    :id: SWARM-SCALE-UNALLOCATABLE-CTR-START
    :observed_from: api:active
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-SWARM-ATTACH-FAILURE-START

    *Applies to:* An attachable overlay network that cannot be allocated, and a created or stopped
    container on that network, on any node

    :api-op:`POST /containers/{id}/start` for the container responds with a 4xx or 5xx status.

.. expectation:: A container start on an unallocatable network responds 500 after 20 s
    :id: SWARM-SCALE-UNALLOCATABLE-CTR-START-HTTP500
    :observed_from: api:active
    :depends_on: none
    :platforms: linux, windows
    :exact_form_of: SWARM-SCALE-UNALLOCATABLE-CTR-START
    :pins: SWARM-SCOPED-ATTACH-FAILURE-TIMEOUT
    :concerns: SWARM-SCALE-UNALLOCATABLE-CTR-START
    :violates: PRIN-ERRORS-ACTIONABLE, REQ-NET-API-ERROR-STATUS

    *Applies to:* An attachable overlay network that cannot be allocated, and a created or stopped
    container on that network, on any node

    :api-op:`POST /containers/{id}/start` for the container responds 500 after 20 seconds with
    :api-error:`attaching to network failed, make sure your network options are correct and check
    manager logs: context deadline exceeded`.

.. expectation:: docker run on an unallocatable attachable network exits non-zero
    :id: SWARM-SCALE-UNALLOCATABLE-CTR-START-CLI
    :observed_from: api:active
    :depends_on: cli
    :platforms: linux, windows
    :refines: REQ-SWARM-ATTACH-FAILURE-START
    :cli_for: SWARM-SCALE-UNALLOCATABLE-CTR-START

    *Applies to:* An attachable overlay network that cannot be allocated

    On any node, :cli:`docker run -d --network` naming that network exits with status 125 and prints
    the daemon's error.

.. expectation:: An overlay network that cannot be allocated is created without a warning
    :id: SWARM-SCALE-UNALLOCATABLE-CREATE
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :pins: SWARM-OVL-ALLOCATION-FAILURE-UNREPORTED
    :concerns: SWARM-SCALE-UNALLOCATABLE-CTR-START
    :violates: PRIN-WARN-AHEAD

    :api-op:`POST /networks/create` for an overlay network that cannot be allocated responds 201
    without a warning that the network cannot be allocated. An example is a network whose address
    pool is exhausted. :api-op:`GET /networks` on a manager then lists the network.

.. expectation:: docker network create for an unallocatable overlay network exits zero
    :id: SWARM-SCALE-UNALLOCATABLE-CREATE-CLI
    :observed_from: api:mgr
    :depends_on: cli
    :platforms: linux
    :cli_for: SWARM-SCALE-UNALLOCATABLE-CREATE
    :pins: SWARM-OVL-ALLOCATION-FAILURE-UNREPORTED
    :concerns: SWARM-SCALE-UNALLOCATABLE-CTR-START-CLI
    :violates: PRIN-WARN-AHEAD

    :cli:`docker network create` for an overlay network that cannot be allocated exits zero and
    prints no warning, and :cli:`docker network ls` on a manager then lists the network.

.. expectation:: On an unallocatable network, a service gets no warning; tasks stay new
    :id: SWARM-SCALE-UNALLOCATABLE-SERVICE
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :pins: SWARM-OVL-ALLOCATION-FAILURE-UNREPORTED
    :within: SWARM-CEIL-ALLOCATION
    :concerns: SWARM-SCALE-UNALLOCATABLE-SERVICE-LOG
    :violates: PRIN-WARN-AHEAD, REQ-SWARM-TASK-ERRORS

    *Applies to:* An overlay network that cannot be allocated

    :api-op:`POST /services/create` for a service on that network responds 201 without a warning
    about the network. The service's tasks stay in the ``new`` state, with desired state
    ``running``, and do not start.

.. expectation:: The leader logs that it failed to allocate a vip service on the network
    :id: SWARM-SCALE-UNALLOCATABLE-SERVICE-LOG
    :observed_from: log
    :depends_on: none
    :platforms: linux
    :pins: SWARM-OVL-ALLOCATION-FAILURE-UNREPORTED
    :within: SWARM-CEIL-ALLOCATION

    *Applies to:* An overlay network that cannot be allocated

    After :api-op:`POST /services/create` for a service in vip mode on that network responds 201,
    the daemon log of the leader manager reports the failure at error level. The entry contains
    :log:`Failed allocation for service` and the service's ID.

.. expectation:: Services publishing one port number in different protocols both answer
    :id: SWARM-SCALE-PORT-PROTOCOLS
    :observed_from: client
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-ROUTING-MESH
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-NFTABLES-NAT, SWARM-HOST-NFT-COMMAND, SWARM-HOST-SCTP, SWARM-HOST-IPVS-SCTP, SWARM-HOST-IPTABLES-SCTP-MATCH, SWARM-HOST-IPTABLES-LB-EXTENSIONS, SWARM-HOST-IPTABLES-LEGACY-MANGLE-KERNEL, BR-HOST-NO-FORWARD-DROP
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A Swarm with a service that publishes a port in ingress mode

    After a new service publishes the same port number in ingress mode under a different protocol,
    both services are reachable on their published ports.

.. expectation:: Removing one of two same-port services leaves the other reachable
    :id: SWARM-SCALE-PORT-PROTOCOLS-REMOVAL
    :observed_from: client
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-ROUTING-MESH
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-NFTABLES-NAT, SWARM-HOST-NFT-COMMAND, SWARM-HOST-SCTP, SWARM-HOST-IPVS-SCTP, SWARM-HOST-IPTABLES-SCTP-MATCH, SWARM-HOST-IPTABLES-LB-EXTENSIONS, SWARM-HOST-IPTABLES-LEGACY-MANGLE-KERNEL, BR-HOST-NO-FORWARD-DROP
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A Swarm with two services that publish the same port number in ingress mode under
    different protocols

    When one of the services is removed, the other service stays reachable on its published port.
    This holds throughout the removal and after it.

.. expectation:: SCTP does not pass to or from containers on Windows nodes
    :id: SWARM-SCALE-WINDOWS-SCTP
    :observed_from: task, client, api:mgr
    :depends_on: os-windows
    :platforms: windows
    :within: SWARM-CEIL-REASSIGNMENT
    :narrows: SWARM-EW-PROTOCOLS, SWARM-LB-PROTOCOLS, SWARM-PUB-PROTOCOLS, SWARM-SCALE-PORT-PROTOCOLS, SWARM-SCALE-PORT-PROTOCOLS-REMOVAL
    :violates: REQ-SWARM-OVERLAY-CONNECTIVITY, REQ-SWARM-PUB-PROTOCOLS, REQ-SWARM-VIP-LOAD-BALANCING, REQ-SWARM-ROUTING-MESH, PRIN-FAILS-CLOSED

    SCTP does not pass to or from containers on Windows nodes, directly or through a VIP. A task on
    a Windows node that publishes an SCTP port in :api-value:`mode=host` fails with an error that
    contains :api-error:`SCTP is unsupported on windows`. On a Windows node, a port published as
    SCTP in :api-value:`mode=ingress` is published as TCP.

.. _swarm-lifecycle-firewall-backend-transition:

Firewall-backend transition
===========================

The firewall backend is a Linux daemon setting. This subsection holds on Linux nodes. A rolling
backend switch changes one node at a time, from either backend to the other. For each node, the
steps are to drain the node, switch its backend once none of the node's tasks is running, and set
the node's availability to ``active`` again. A node is being switched from the request that drains
it until its availability is ``active`` again and the :need:`[[title|lower]] ceiling ([[seconds]] s)
<SWARM-CEIL-REJOIN>` has passed since its restart was complete, as
:ref:`swarm-lifecycle-convergence` defines it. A restart in this section is graceful, with or
without a drain first, unless an expectation says that it follows a kill. When the daemon starts
again with the iptables backend in place of nftables, :need:`SWARM-RESTART-NODE-STATE-ACTIVE`,
:need:`SWARM-RESTART-OTHER-NODES`, :need:`SWARM-RESTART-TASKS-FROM-OTHER-NODES` and
:need:`SWARM-RESTART-INGRESS-FROM-OTHER-NODES` hold, and after a kill, so does
:need:`SWARM-RESTART-KILLED-VNI-REUSE`. After a kill, in either direction,
:need:`SWARM-RESTART-STALE-OVERLAY` and :need:`SWARM-RESTART-STALE-OVERLAY-REMOVAL` hold too. For a
restart with the other backend, in either direction, the expectations below take the place of the
expectations of :ref:`swarm-lifecycle-restart-and-reclamation` that the previous two sentences do
not name for that direction. In the expectations about a rolling backend switch, each service has a
running task on a node that is not being switched, and a task's process ignores its stop signal and
keeps accepting connections until it is killed at the end of a stop grace period longer than the
:need:`[[title|lower]] ceiling ([[seconds]] s) <SWARM-CEIL-ALLOCATION>`.

.. expectation:: After a switch to nftables, the node is active within the ceiling
    :id: SWARM-BACKEND-NODE-STATE-ACTIVE
    :observed_from: api:mgr, api:wkr
    :depends_on: none
    :platforms: linux
    :provisional_on: REQ-BR-NFT-MIGRATION, REQ-SWARM-MIXED-BACKEND-INTEROP
    :within: SWARM-CEIL-REJOIN

    *Applies to:* A node in a Swarm

    After the node's daemon restarts with the nftables backend in place of iptables, gracefully or
    after a kill, :api-op:`GET /info` on the node reports :api-field:`Swarm.LocalNodeState` as
    ``active`` within the :need:`[[title|lower]] ceiling ([[seconds]] s) <SWARM-CEIL-REJOIN>` after
    the daemon's API responds again.

.. expectation:: After a switch to nftables, node tasks are reachable within the ceiling
    :id: SWARM-BACKEND-TASKS
    :observed_from: task
    :depends_on: os-linux
    :platforms: linux
    :provisional_on: REQ-BR-NFT-MIGRATION, REQ-SWARM-MIXED-BACKEND-INTEROP
    :within: SWARM-CEIL-REJOIN, SWARM-CEIL-REASSIGNMENT

    *Applies to:* A service whose tasks are constrained to a node

    After the node's daemon restarts with the nftables backend in place of iptables, gracefully or
    after a kill, without the node being drained first, each of the service's tasks on the node is
    reachable over its overlay networks once the :need:`[[title|lower]] ceiling ([[seconds]] s)
    <SWARM-CEIL-REJOIN>` has passed from when the restart is complete, and the
    :need:`[[title|lower]] ceiling ([[seconds]] s) <SWARM-CEIL-REASSIGNMENT>` has passed from when
    :api-op:`GET /tasks` first reports that task's state as ``running``.

.. expectation:: After a switch to iptables, node tasks are reachable within the ceiling
    :id: SWARM-BACKEND-TASKS-IPT
    :observed_from: task
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-SWARM-RESTART-RECOVERY
    :within: SWARM-CEIL-REJOIN, SWARM-CEIL-REASSIGNMENT

    *Applies to:* A service whose tasks are constrained to a node

    After the node's daemon restarts with the iptables backend in place of nftables, gracefully or
    after a kill, without the node being drained first, each of the service's tasks on the node is
    reachable over its overlay networks once the :need:`[[title|lower]] ceiling ([[seconds]] s)
    <SWARM-CEIL-REJOIN>` has passed from when the restart is complete, and the
    :need:`[[title|lower]] ceiling ([[seconds]] s) <SWARM-CEIL-REASSIGNMENT>` has passed from when
    :api-op:`GET /tasks` first reports that task's state as ``running``.

.. expectation:: After a switch to iptables, published ports answer within the ceiling
    :id: SWARM-BACKEND-PUBLISHED-PORTS-IPT
    :observed_from: client
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-SWARM-RESTART-RECOVERY
    :within: SWARM-CEIL-REJOIN, SWARM-CEIL-REASSIGNMENT

    After a node's daemon restarts with the iptables backend in place of nftables, gracefully or
    after a kill, without the node being drained first, each of the node's published ports answers
    once the :need:`[[title|lower]] ceiling ([[seconds]] s) <SWARM-CEIL-REJOIN>` has passed from
    when the restart is complete, and the :need:`[[title|lower]] ceiling ([[seconds]] s)
    <SWARM-CEIL-REASSIGNMENT>` has passed from when :api-op:`GET /tasks` first reports the last of
    the tasks that the port is published for as ``running``.

.. expectation:: After a switch to nftables, published ports answer within the ceiling
    :id: SWARM-BACKEND-PUBLISHED-PORTS-NFT
    :observed_from: client
    :depends_on: os-linux
    :platforms: linux
    :provisional_on: REQ-BR-NFT-MIGRATION, REQ-SWARM-MIXED-BACKEND-INTEROP
    :needs: BR-HOST-NO-FORWARD-DROP
    :within: SWARM-CEIL-REJOIN, SWARM-CEIL-REASSIGNMENT

    *Applies to:* A node whose filter table's :ipt-chain:`FORWARD` chain has policy ACCEPT

    After the node's daemon restarts with the nftables backend in place of iptables, gracefully or
    after a kill, without the node being drained first, each of the node's published ports answers
    once the :need:`[[title|lower]] ceiling ([[seconds]] s) <SWARM-CEIL-REJOIN>` has passed from
    when the restart is complete, and the :need:`[[title|lower]] ceiling ([[seconds]] s)
    <SWARM-CEIL-REASSIGNMENT>` has passed from when :api-op:`GET /tasks` first reports the last of
    the tasks that the port is published for as ``running``.

.. expectation:: After a switch to nftables, other nodes' VIPs and names reach its tasks
    :id: SWARM-BACKEND-TASKS-FROM-OTHER-NODES
    :observed_from: task, ctr
    :depends_on: os-linux
    :platforms: linux
    :provisional_on: REQ-BR-NFT-MIGRATION, REQ-SWARM-MIXED-BACKEND-INTEROP
    :needs: SWARM-HOST-IPVS, SWARM-HOST-NFTABLES-NAT, SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-NFT-COMMAND, SWARM-HOST-IPTABLES-LB-EXTENSIONS, SWARM-HOST-IPTABLES-LEGACY-MANGLE-KERNEL, DNS-HOST-NFTABLES-NAT, DNS-HOST-IPTABLES-COMMAND, DNS-HOST-NFT-COMMAND, DNS-HOST-IPTABLES-KERNEL, DNS-HOST-IPTABLES-LEGACY-KERNEL, DNS-HOST-IPTABLES-NFT-KERNEL
    :within: SWARM-CEIL-REJOIN, SWARM-CEIL-REASSIGNMENT

    *Applies to:* A node, a service in vip mode on an overlay network whose tasks are constrained to
    that node, and a task or container on that network on another node

    After the node's daemon restarts with the nftables backend in place of iptables, gracefully or
    after a kill, without the node being drained first, :dns-name:`tasks.<service>` resolves in that
    task or container to the addresses on that network of the service's running tasks, and its
    connections to the service's VIP reach them. Both hold once the :need:`[[title|lower]] ceiling
    ([[seconds]] s) <SWARM-CEIL-REJOIN>` has passed from when the restart is complete, and the
    :need:`[[title|lower]] ceiling ([[seconds]] s) <SWARM-CEIL-REASSIGNMENT>` has passed from when
    :api-op:`GET /tasks` first reports each of those tasks' state as ``running``.

.. expectation:: After a switch to nftables, published ports elsewhere reach its tasks
    :id: SWARM-BACKEND-INGRESS-FROM-OTHER-NODES
    :observed_from: client
    :depends_on: os-linux
    :platforms: linux
    :provisional_on: REQ-BR-NFT-MIGRATION, REQ-SWARM-MIXED-BACKEND-INTEROP
    :needs: BR-HOST-NO-FORWARD-DROP, SWARM-HOST-IPVS, SWARM-HOST-NFTABLES-NAT, SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-NFT-COMMAND, SWARM-HOST-IPTABLES-LB-EXTENSIONS, SWARM-HOST-IPTABLES-LEGACY-MANGLE-KERNEL
    :within: SWARM-CEIL-REJOIN, SWARM-CEIL-REASSIGNMENT

    *Applies to:* A node, and a service whose tasks are constrained to that node and that publishes
    a port in ingress mode

    After the node's daemon restarts with the nftables backend in place of iptables, gracefully or
    after a kill, without the node being drained first, connections to the service's published port
    at the address of another node reach the service's running tasks. This holds once the
    :need:`[[title|lower]] ceiling ([[seconds]] s) <SWARM-CEIL-REJOIN>` has passed from when the
    restart is complete, and the :need:`[[title|lower]] ceiling ([[seconds]] s)
    <SWARM-CEIL-REASSIGNMENT>` has passed from when :api-op:`GET /tasks` first reports each of those
    tasks' state as ``running``.

.. expectation:: After a kill and switch to nftables, a reused VNI passes traffic
    :id: SWARM-BACKEND-KILLED-VNI-REUSE
    :observed_from: node, task
    :depends_on: os-linux
    :platforms: linux
    :provisional_on: REQ-BR-NFT-MIGRATION, REQ-SWARM-MIXED-BACKEND-INTEROP
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A node that used a VNI before its daemon was killed and started again with the
    nftables backend in place of iptables

    A task or container on a network that is allocated that VNI starts on the node, and the network
    passes traffic. This holds whether the network is the one that used the VNI before the kill or
    another network.

.. expectation:: A rolling backend switch does not interrupt flows among unswitched nodes
    :id: SWARM-BACKEND-ROLLING-FLOWS
    :observed_from: client, task
    :depends_on: os-linux
    :platforms: linux
    :provisional_on: REQ-BR-NFT-MIGRATION, REQ-SWARM-MIXED-BACKEND-INTEROP
    :needs: BR-HOST-NO-FORWARD-DROP, SWARM-HOST-IPVS, SWARM-HOST-NFTABLES-NAT, SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-NFT-COMMAND, SWARM-HOST-IPTABLES-LB-EXTENSIONS, SWARM-HOST-IPTABLES-LEGACY-MANGLE-KERNEL
    :within: SWARM-CEIL-REASSIGNMENT, SWARM-CEIL-REJOIN

    *Applies to:* A Swarm whose nodes' filter tables' :ipt-chain:`FORWARD` chains have policy ACCEPT
    once they run the nftables backend

    Throughout a rolling backend switch, a flow is not interrupted when two conditions are true. The
    flow starts at a client on a node that is not being switched and is sent to a VIP, or enters
    through a published port on such a node. The flow goes to a task on a node that is not being
    switched. The expectation covers the flow from its start until the node where it starts or
    enters, or the task's node, starts being switched. Not interrupted means that a TCP flow is not
    reset, and that no TCP or UDP flow has a gap in delivery longer than 1 second. The
    :need:`[[title|lower]] ceiling ([[seconds]] s) <SWARM-CEIL-REJOIN>` is a length of time in the
    definition of a node that is being switched, and does not delay this expectation. The
    expectation covers a flow that starts once the :need:`[[title|lower]] ceiling ([[seconds]] s)
    <SWARM-CEIL-REASSIGNMENT>` has passed from when the last of the flow's client and task runs.

.. expectation:: In a rolling backend switch, ingress ports answer on unswitched nodes
    :id: SWARM-BACKEND-ROLLING-PUBLISHED-PORTS
    :observed_from: client
    :depends_on: os-linux
    :platforms: linux
    :provisional_on: REQ-BR-NFT-MIGRATION, REQ-SWARM-MIXED-BACKEND-INTEROP
    :needs: BR-HOST-NO-FORWARD-DROP, SWARM-HOST-IPVS, SWARM-HOST-NFTABLES-NAT, SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-NFT-COMMAND, SWARM-HOST-IPTABLES-LB-EXTENSIONS, SWARM-HOST-IPTABLES-LEGACY-MANGLE-KERNEL
    :within: SWARM-CEIL-ALLOCATION, SWARM-CEIL-REASSIGNMENT, SWARM-CEIL-REJOIN

    *Applies to:* A Swarm whose nodes' filter tables' :ipt-chain:`FORWARD` chains have policy ACCEPT
    once they run the nftables backend

    Throughout a rolling backend switch, every port that a service publishes in ingress mode answers
    on the nodes that are not being switched. The :need:`[[title|lower]] ceiling ([[seconds]] s)
    <SWARM-CEIL-REJOIN>` and the :need:`[[title|lower]] ceiling ([[seconds]] s)
    <SWARM-CEIL-ALLOCATION>` are lengths of time in the introduction of this subsection, and do not
    delay this expectation. A new connection is covered when, at the time it is made, every task of
    the service that :api-op:`GET /tasks` reports as ``running`` was first reported as ``running``
    at least the :need:`[[title|lower]] ceiling ([[seconds]] s) <SWARM-CEIL-REASSIGNMENT>` earlier.

.. expectation:: A third node's switch to nftables does not interrupt other nodes' flows
    :id: SWARM-BACKEND-OTHER-NODES-FLOWS
    :observed_from: task, client
    :depends_on: os-linux
    :platforms: linux
    :provisional_on: REQ-BR-NFT-MIGRATION, REQ-SWARM-MIXED-BACKEND-INTEROP
    :needs: BR-HOST-NO-FORWARD-DROP
    :within: SWARM-CEIL-REASSIGNMENT, SWARM-CEIL-REJOIN

    *Applies to:* A flow to a task, from a client on a node to a VIP or from a client outside the
    Swarm to a port that a node publishes, and a third node that is neither that node nor the task's
    node

    A restart of the third node's daemon with the nftables backend in place of iptables, gracefully
    or after a kill, without the node being drained first, does not interrupt the flow: no TCP flow
    is reset, and no TCP or UDP flow has a gap in delivery longer than 1 second. This holds from
    when the restart starts until the :need:`[[title|lower]] ceiling ([[seconds]] s)
    <SWARM-CEIL-REJOIN>` has passed from when the restart is complete.

.. expectation:: After a switch to nftables, a DROP FORWARD policy drops container traffic
    :id: SWARM-BACKEND-NFT-FORWARD-POLICY
    :observed_from: client, task, ctr
    :depends_on: os-linux
    :platforms: linux
    :provisional_on: REQ-BR-NFT-MIGRATION
    :within: SWARM-CEIL-REJOIN, SWARM-CEIL-REASSIGNMENT

    *Applies to:* A node whose filter table's :ipt-chain:`FORWARD` chain has policy DROP

    After the node's daemon restarts with the nftables backend in place of iptables, the node drops
    traffic that it routes to and from containers, such as traffic to published ports and the egress
    of containers, unless an operator's rule accepts it.

.. _swarm-lifecycle-failure-reporting:

Failure reporting
=================

When a node cannot program cluster-scoped configuration, no API surface reports it. Cluster-scoped
configuration is what a node programs for the Swarm outside the network namespace of any one task or
container. Examples are encryption parameters for a peer, a load-balancer backend, the firewall
rules that publish an ingress port, and an FDB entry. The affected service continues to look healthy
from ``api:mgr`` while that node's data plane is wrong. The daemon log is the only channel that
reports the failure. A failure to add a task's ingress rules in its own network namespace is not
such a failure: the task does not start, and :api-op:`GET /tasks` reports the error
(:need:`SWARM-INGPUB-INGRESS-RULES-FAILED`). A failure to add or remove a VIP alias on a
load-balancer endpoint, or to find that endpoint's interface, is a failure to program the
load-balancer service. Under the iptables backend, so is a failure to add the service's
firewall-mark rule. Under the nftables backend, a failure to apply the nat table when a backend is
added is a failure to program that backend, and a failure to apply a :drvopt:`dsr` network's
direct-routing table is a failure to program the load-balancer service.

.. expectation:: A node-local load-balancer programming failure is logged at error level
    :id: SWARM-FAILREP-LB
    :observed_from: log
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-SWARM-NODE-FAILURE-LOGGED
    :within: SWARM-CEIL-REASSIGNMENT

    The daemon log of a node reports a node-local failure to program a load-balancer service or
    backend at error level.

.. expectation:: With iptables, a load-balancer failure entry has one of twelve messages
    :id: SWARM-FAILREP-LB-MSG-IPT
    :observed_from: log
    :depends_on: none
    :platforms: linux
    :exact_form_of: SWARM-FAILREP-LB
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* Linux nodes with the iptables backend

    A daemon log entry for a node-local failure to program a load-balancer service or backend
    contains :log:`Failed to create an ipvs handle`, :log:`Failed find interface name for endpoint`,
    :log:`Failed add IP alias`, :log:`Failed to remove IP alias`, :log:`Failed to add firewall mark
    rule`, :log:`Failed to create a new service for vip`, :log:`Failed to create real server`,
    :log:`Failed to delete real server`, :log:`Failed to set LB weight of real server`, :log:`Failed
    to delete service for vip`, :log:`Failed to delete firewall mark rule` or :log:`addLBBackend`.
    No entry names the service. The entries about an ipvs handle, a firewall mark rule, a service
    and a real server identify the load-balancer sandbox only by prefixes of its ID and of its
    container ID. The entry about an ipvs handle adds the path of its network namespace.

.. expectation:: With nftables, a load-balancer failure entry has one of six messages
    :id: SWARM-FAILREP-LB-MSG-NFT
    :observed_from: log
    :depends_on: none
    :platforms: linux
    :exact_form_of: SWARM-FAILREP-LB
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* Linux nodes with the nftables backend

    A daemon log entry for a node-local failure to program a load-balancer service or backend
    contains :log:`Failed to apply changes to nftables nat table`, :log:`Failed add IP alias`,
    :log:`Failed remove IP alias`, :log:`Failed find interface name for endpoint`,
    :log:`addLBBackend` or :log:`Failed to apply nftables dsr table`. No entry names the service.
    The entry that contains :log:`Failed to apply changes to nftables nat table` names no network
    either. The entry that contains :log:`Failed to apply nftables dsr table` has the network's full
    ID in its ``network`` field.

.. expectation:: Without the netdev egress hook, a dsr network logs a fallback to NAT
    :id: SWARM-FAILREP-DSR-NO-EGRESS-HOOK
    :observed_from: log
    :depends_on: none
    :platforms: linux
    :pins: SWARM-HOST-NFT-NETDEV-EGRESS-COMMAND, SWARM-HOST-NFTABLES-NETDEV
    :needs: SWARM-HOST-NFTABLES-NAT, SWARM-HOST-NFT-COMMAND
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* Linux nodes whose kernel does not offer the nftables netdev egress hook, or whose
    :program:`nft` does not accept netdev tables with that hook, with the nftables backend, and a
    service in vip mode on an overlay network with the :drvopt:`dsr` option

    The daemon log of the node contains an error-level entry that contains :log:`Failed to apply
    nftables dsr table; this network's overlay VIP traffic will be load-balanced via NAT rather than
    direct server return`. Its ``network`` field is the network's full ID.

.. expectation:: A Windows load-balancer failure entry has one of seven messages
    :id: SWARM-FAILREP-LB-MSG-WINDOWS
    :observed_from: log
    :depends_on: none
    :platforms: windows
    :exact_form_of: SWARM-FAILREP-LB
    :within: SWARM-CEIL-REASSIGNMENT

    A daemon log entry for a node-local failure to program a load-balancer service or backend
    contains :log:`Failed to add ILB policy`, :log:`Failed to add ELB policy`, :log:`Failed to find
    load balancer IP for network`, :log:`Failed to find HNS ID for endpoint`, :log:`Failed to remove
    HNS ILB policylist`, :log:`Failed to remove HNS ELB policylist` or :log:`Failed to find policies
    for service`.

.. expectation:: A node-local failure to publish an ingress port is logged at error level
    :id: SWARM-FAILREP-INGRESS-PORT
    :observed_from: log
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-NODE-FAILURE-LOGGED
    :within: SWARM-CEIL-REASSIGNMENT

    The daemon log of a node reports a node-local failure to publish an ingress port at error level.

.. expectation:: A failure to publish an ingress port in use is logged with the port
    :id: SWARM-FAILREP-INGRESS-PORT-IN-USE
    :observed_from: log
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-NODE-FAILURE-LOGGED, PRIN-ERRORS-ACTIONABLE
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A Linux node where a port is in use by a process or container, and a service that
    publishes that port in ingress mode

    The daemon log of the node reports a node-local failure to publish an ingress port at error
    level, as for any such failure. The entry contains the port number.

.. expectation:: The daemon logs that it failed to publish a service's ingress ports
    :id: SWARM-FAILREP-INGRESS-PORT-MSG
    :observed_from: log
    :depends_on: none
    :platforms: linux
    :exact_form_of: SWARM-FAILREP-INGRESS-PORT
    :within: SWARM-CEIL-REASSIGNMENT

    The daemon log entry for a node-local failure to publish an ingress port contains :log:`Failed
    to publish this service's Swarm ingress ports on this node`.

.. expectation:: A failure to unpublish a service's ingress ports is logged as an error
    :id: SWARM-FAILREP-INGRESS-UNPUBLISH
    :observed_from: log
    :depends_on: none
    :platforms: linux
    :within: SWARM-CEIL-REASSIGNMENT
    :rationale: RAT-SWARM-FAILURE-LOG-ENTRIES

    *Applies to:* A Linux node that publishes a service's ingress ports

    When the node removes the service's last backend from the ingress network's load balancer, for
    example after the service is removed, and it fails to unpublish the service's ingress ports, the
    daemon log of the node contains an error-level entry that contains :log:`Failed to unpublish
    this service's Swarm ingress ports on this node`.

.. expectation:: A failure to program an overlay peer is logged as a warning
    :id: SWARM-FAILREP-PEER
    :observed_from: log
    :depends_on: none
    :platforms: linux, windows
    :within: SWARM-CEIL-REASSIGNMENT
    :rationale: RAT-SWARM-FAILURE-LOG-ENTRIES

    The daemon log of a node reports a node-local failure to program an overlay peer at warning
    level. On a Linux node, that is the peer's FDB or neighbor entry. On a Windows node, it is the
    peer's HNS remote endpoint.

.. expectation:: The daemon logs that it failed to add or delete an overlay peer entry
    :id: SWARM-FAILREP-PEER-MSG
    :observed_from: log
    :depends_on: none
    :platforms: linux, windows
    :exact_form_of: SWARM-FAILREP-PEER
    :within: SWARM-CEIL-REASSIGNMENT

    The daemon log entry for a node-local failure to program an overlay peer contains :log:`overlay:
    failed to add peer entry` or :log:`overlay: failed to delete peer entry`. The entry contains the
    network's ID, the address of the peer's endpoint, and the data-path address of the peer's node.
    Its ``nid`` field is the network's full ID.

.. expectation:: A failure to program an IPsec SA or policy is logged as a warning
    :id: SWARM-FAILREP-IPSEC
    :observed_from: log
    :depends_on: none
    :platforms: linux
    :within: SWARM-CEIL-REASSIGNMENT
    :rationale: RAT-SWARM-FAILURE-LOG-ENTRIES

    The daemon log of a node reports a node-local failure to program an IPsec security association
    or policy for a peer at warning level.

.. expectation:: A failed IPsec SA or policy logs a warning that names fSA, rSA or fSP
    :id: SWARM-FAILREP-IPSEC-MSG
    :observed_from: log
    :depends_on: none
    :platforms: linux
    :exact_form_of: SWARM-FAILREP-IPSEC
    :within: SWARM-CEIL-REASSIGNMENT

    The daemon log entry for a node-local failure to program an IPsec security association for a
    peer contains :log:`Failed Adding fSA{`, :log:`Failed Adding rSA{`, :log:`Failed Removing fSA{`
    or :log:`Failed Removing rSA{`. The entry for a policy contains :log:`Adding fSP{`,
    :log:`Removing fSP{` or :log:`Failed to update fSP{`. The entry for a security association
    prints :log:`%!s(PANIC=String method: runtime error: invalid memory address or nil pointer
    dereference)` in place of the association, and does not name the peer.

.. expectation:: Each new task retries the load balancer and logs the failure again
    :id: SWARM-FAILREP-LB-REPEATED
    :observed_from: log
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-SWARM-NODE-FAILURE-LOGGED
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A node where a node-local failure to program the load-balancer service or a
    backend for a service occurred, and its cause still holds

    A task is then added to the service. The node tries again to program the load-balancer service
    or backend for that task, and the daemon log of the node reports the failure again. The report
    is not suppressed after the first one.

.. expectation:: Each new task retries the ingress ports and logs the failure again
    :id: SWARM-FAILREP-INGRESS-PORT-REPEATED
    :observed_from: log
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-NODE-FAILURE-LOGGED
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A Linux node where a node-local failure to publish a service's ingress ports
    occurred, and its cause still holds

    A task is then added to the service. The node tries again to publish the service's ingress
    ports, and the daemon log of the node reports the failure again. The report is not suppressed
    after the first one.

.. expectation:: Node-local failures are not reported again without a new attempt
    :id: SWARM-FAILREP-NOT-REPEATED
    :observed_from: log
    :depends_on: none
    :platforms: linux, windows
    :within: SWARM-CEIL-REASSIGNMENT
    :rationale: RAT-SWARM-FAILURE-LOG-ENTRIES

    A node reports a node-local failure to program cluster-scoped configuration when the failure
    occurs. While the cause holds and nothing changes in the Swarm, the node does not try again, and
    its daemon log gets no further entry about the failure. Nothing changes in the Swarm while no
    service, task, network or node is created, updated or removed, no node's daemon restarts, no
    node is partitioned from the others, and the encryption keys do not rotate.

.. expectation:: After a load-balancer failure clears, a new task gets the node's flows
    :id: SWARM-FAILREP-LB-RECOVERY
    :observed_from: task, client
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-MESH-FOLLOWS-TASKS
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-NFTABLES-NAT, SWARM-HOST-NFT-COMMAND, SWARM-HOST-IPTABLES-LB-EXTENSIONS, SWARM-HOST-IPTABLES-LEGACY-MANGLE-KERNEL
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A node where a node-local failure to program the load-balancer service or a
    backend for a service occurred

    The cause of the failure clears. A task that is then added to the service receives new flows to
    the service that start on that node. This holds when the failure was to program the
    load-balancer service, and when it was to program a backend.

.. expectation:: After a load-balancer failure clears, a new task gets no failure logged
    :id: SWARM-FAILREP-LB-RECOVERY-LOG
    :observed_from: log
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-SWARM-NODE-FAILURE-LOGGED
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A node where a node-local failure to program the load-balancer service or a
    backend for a service occurred

    The cause of the failure clears. A task is then added to the service. After the task is added,
    no new entry in the daemon log of that node reports a failure to program the load-balancer
    service or a backend for that task.

.. expectation:: After an ingress publish failure clears, a new task brings the ports up
    :id: SWARM-FAILREP-INGRESS-PORT-RECOVERY
    :observed_from: client
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-SWARM-ROUTING-MESH
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-NFTABLES-NAT, SWARM-HOST-NFT-COMMAND, SWARM-HOST-IPTABLES-LB-EXTENSIONS, SWARM-HOST-IPTABLES-LEGACY-MANGLE-KERNEL, BR-HOST-NO-FORWARD-DROP
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A Linux node where publishing a service's ingress ports failed

    The cause of the failure clears. A task is then added to the service. Within the
    :need:`[[title|lower]] ceiling ([[seconds]] s) <SWARM-CEIL-REASSIGNMENT>` after that task runs,
    clients reach the service's tasks at the node's address on each of the service's published
    ports.

.. expectation:: After an ingress publish failure clears, a new task gets no failure logged
    :id: SWARM-FAILREP-INGRESS-PORT-RECOVERY-LOG
    :observed_from: log
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-NODE-FAILURE-LOGGED
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A Linux node where publishing a service's ingress ports failed

    The cause of the failure clears. A task is then added to the service. After the task is added,
    no new entry in the daemon log of that node reports a failure to publish the service's ingress
    ports.

.. expectation:: Recovery from a node-local failure is not reported
    :id: SWARM-FAILREP-RECOVERY-UNREPORTED
    :observed_from: log
    :depends_on: none
    :platforms: linux, windows
    :within: SWARM-CEIL-REASSIGNMENT
    :rationale: RAT-SWARM-FAILURE-LOG-ENTRIES

    At the default log level, a successful attempt that follows a failure adds no entry to the
    node's daemon log about it: after the attempt, no entry that contains the ID or name of the
    service, or the address of the peer endpoint or node, that the failed configuration was for
    reports that the attempt succeeded or that the failure cleared.

.. expectation:: Tasks on a node that cannot program a service are reported running
    :id: SWARM-FAILREP-UNPROGRAMMED-TASKS
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows
    :rationale: RAT-SWARM-NODE-FAILURE-NOT-TASK-FAILURE

    *Applies to:* A node that cannot program cluster-scoped configuration for a service

    :api-op:`GET /tasks` reports the node's tasks for the service as ``running``.

.. expectation:: A service reports all tasks running despite a node-local failure
    :id: SWARM-FAILREP-UNPROGRAMMED-SERVICE
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows
    :rationale: RAT-SWARM-NODE-FAILURE-NOT-TASK-FAILURE

    *Applies to:* A node that cannot program cluster-scoped configuration for a service

    :api-op:`GET /services?status=true` at API version 1.41 or later reports the service's
    :api-field:`RunningTasks` equal to its :api-field:`DesiredTasks`.

.. expectation:: docker service ps --no-trunc lists tasks with their states and errors
    :id: SWARM-FAILREP-SERVICE-PS
    :observed_from: api:mgr
    :depends_on: cli
    :platforms: linux
    :refines: REQ-SWARM-TASK-ERRORS

    :cli:`docker service ps --no-trunc` for a service lists the service's tasks, with their states
    and errors.

.. expectation:: docker service ls shows all replicas running despite a local failure
    :id: SWARM-FAILREP-UNPROGRAMMED-SERVICE-CLI
    :observed_from: api:mgr
    :depends_on: cli
    :platforms: linux, windows
    :cli_for: SWARM-FAILREP-UNPROGRAMMED-SERVICE
    :rationale: RAT-SWARM-NODE-FAILURE-NOT-TASK-FAILURE

    *Applies to:* A node that cannot program cluster-scoped configuration for a service

    :cli:`docker service ls` prints the service's replicas as all running.

.. _swarm-lifecycle-firewall-anchors-host-integration:

*************************************
 Firewall anchors & host integration
*************************************

The content of Moby's firewall rules is not an interface. A backend change replaces exactly that
content. :ref:`bridge-networking-rule-content` in :doc:`bridge-networking.md <bridge-networking>`
states the bridge driver's rules in detectors only, the rules for Swarm ingress ports included. This
section covers the overlay driver's own firewall state. It also covers how Swarm traffic meets the
bridge driver's firewall anchors. :doc:`bridge-networking.md <bridge-networking>` specifies those
anchors, :ipt-chain:`DOCKER-USER` among them. This section holds on Linux nodes. The
:ref:`swarm-lifecycle-nftables` subsection states the content of the overlay driver's nftables table
in detectors only.

.. _swarm-lifecycle-nftables:

nftables
========

The first expectation below states the hooks and priorities of the base chains in the nftables table
that the driver uses for encrypted networks. The names of that table and its chains are not anchors.
An operator's rules can reach them only if the rules are added to Moby's own table. That is not a
supported configuration. The last expectation below asserts the content of that table, so that a
change to it is visible in review. A deliberate change to this content is not a breakage.

.. expectation:: The overlay nftables table's family follows the data-path address
    :id: SWARM-NFT-OVERLAY-TABLE
    :observed_from: node
    :depends_on: none
    :platforms: linux
    :pins: SWARM-HOST-NFTABLES-ROUTE-CHAINS
    :needs: SWARM-HOST-NFTABLES-ROUTE-CHAINS

    *Applies to:* A node with the nftables backend where an overlay network, the ingress network
    included, is programmed

    The overlay driver's nftables table is in the ``ip`` family when the node's data-path address is
    IPv4, and in the ``ip6`` family when it is IPv6. Its base chains are on the input hook at raw
    priority (−300) and on the output hook at mangle priority (−150).

.. expectation:: The overlay driver's nftables table matches the reference
    :id: SWARM-NFT-REFERENCE
    :observed_from: node
    :depends_on: none
    :platforms: linux
    :pins: SWARM-HOST-NFTABLES-ROUTE-CHAINS
    :needs: SWARM-HOST-XFRM, SWARM-HOST-NFTABLES-ROUTE-CHAINS
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A node with the nftables backend and an IPv4 data-path address

    For each configuration that the repository keeps a reference nftables ruleset for in
    :repo-path:`daemon/libnetwork/drivers/overlay/testdata/TestOverlayEncNftTable`, the overlay
    driver's nftables table matches that reference. The reference gives the matches and statements
    of each rule, the order of the rules in each chain, and the table, set and chain names. The
    table matches it whatever the values of its counters, with the VNIs of the encrypted overlay
    networks that are programmed on the node in place of the elements of the reference's set. The
    configurations are an encrypted overlay network while it is programmed on the node, and after it
    stops being programmed.

.. _swarm-lifecycle-iptables:

iptables
========

The expectations in this subsection hold under the iptables backend. The
:ref:`bridge-networking-ipt-chain-docker-user` section of :doc:`bridge-networking.md
<bridge-networking>` specifies the :ipt-chain:`DOCKER-USER` chain itself. Swarm ingress ports do not
have a chain of their own. The bridge driver's anchors also cover them because the bridge driver
publishes them like any other published port.

.. expectation:: No daemon rule accepts Swarm published-port traffic before DOCKER-USER
    :id: SWARM-IPT-DOCKER-USER
    :observed_from: client
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-BR-DOCKER-USER
    :needs: SWARM-HOST-IPTABLES-COMMAND
    :within: SWARM-CEIL-REASSIGNMENT

    Traffic to and from a Swarm published port, in ingress mode and :api-value:`mode=host`, that the
    node forwards, not relaying it through :program:`docker-proxy`, is evaluated by an operator's
    :ipt-chain:`DOCKER-USER` rules. No rule that the daemon programs accepts that traffic before the
    :ipt-chain:`DOCKER-USER` chain.

.. expectation:: An INPUT rule accepting VXLAN lets no cleartext into encrypted networks
    :id: SWARM-IPT-ENCRYPTED-INPUT-ACCEPT
    :observed_from: underlay → task or ctr
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-SWARM-ENC-AUTHENTICATED
    :needs: SWARM-HOST-XFRM, SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-IPTABLES-ENC-MATCHES, SWARM-HOST-IPTABLES-LEGACY-MANGLE-KERNEL, SWARM-HOST-IPTABLES-NFT-ROUTE-KERNEL
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A node where an encrypted network is programmed, and an operator's rule at the end
    of the filter table's :ipt-chain:`INPUT` chain that accepts incoming VXLAN traffic

    Cleartext VXLAN for that network's VNI that arrives at the node does not reach the network's
    tasks or containers. This holds whenever the network is programmed on the node and the
    operator's rule is in place, without a ceiling.

.. _swarm-lifecycle-cross-generation-cleanup:

Cross-generation cleanup
========================

This subsection covers the upgrade path that a packager exercises. A node that runs release *N−1* is
upgraded to *N*, possibly with a different firewall backend. A generation is the firewall state that
one release creates under one firewall backend. A previous generation is such state that a daemon
left on the node before the current start of the daemon.
:ref:`bridge-networking-cross-generation-cleanup` in :doc:`bridge-networking.md <bridge-networking>`
gives the daemon-wide expectations and the reason why cleanup depends on names. The expectations
here cover state that belongs to Swarm and the overlay driver.

.. expectation:: The daemon removes the DOCKER-INGRESS chain an old release left at start
    :id: SWARM-CLEAN-INGRESS-CHAIN
    :observed_from: node
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-SWARM-STALE-FIREWALL-CLEANUP, PRIN-NO-LEAKS
    :needs: SWARM-HOST-IPTABLES-CLEANUP-COMMAND

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
    :exact_form_of: SWARM-CLEAN-INGRESS-CHAIN

    *Applies to:* A node with a :ipt-chain:`DOCKER-INGRESS` chain in the filter or nat table

    After the daemon starts with the iptables backend, the filter and nat tables do not contain that
    chain.

.. expectation:: The first start after a switch to nftables removes DOCKER-INGRESS
    :id: SWARM-CLEAN-INGRESS-CHAIN-NFT
    :observed_from: node
    :depends_on: os-linux
    :platforms: linux
    :exact_form_of: SWARM-CLEAN-INGRESS-CHAIN
    :needs: SWARM-HOST-IPTABLES-CLEANUP-COMMAND

    *Applies to:* A node with a :ipt-chain:`DOCKER-INGRESS` chain in the filter or nat table, and a
    jump from the filter table's :ipt-chain:`FORWARD` chain to :ipt-chain:`DOCKER-FORWARD` or
    :ipt-chain:`DOCKER-ISOLATION-STAGE-1`

    After the daemon first starts after a switch to nftables, the filter and nat tables do not
    contain that chain.

.. expectation:: A programmed VNI has no encryption state that the other backend left
    :id: SWARM-CLEAN-ENCRYPTION-OTHER-BACKEND
    :observed_from: node
    :depends_on: os-linux, upgrade
    :platforms: linux
    :refines: PRIN-NO-LEAKS
    :provisional_on: REQ-BR-NFT-MIGRATION
    :needs: SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-IPTABLES-CLEANUP-COMMAND, SWARM-HOST-NFT-CLEANUP-COMMAND
    :within: SWARM-CEIL-REJOIN, SWARM-CEIL-REASSIGNMENT

    *Applies to:* Firewall state for overlay encryption that a previous generation left for a VNI
    under the other firewall backend, such as after the previous generation's daemon was killed, on
    a node in a Swarm with an ingress network

    The node switches firewall backend from iptables to nftables across a package upgrade from *N−1*
    to *N*, or in either direction across a same-binary restart. After a switch to nftables, that
    state is the rules for the VNI in filter :ipt-chain:`INPUT` or mangle :ipt-chain:`OUTPUT`. After
    a switch to iptables, it is the :nft-table:`docker-overlay` table in the family of the node's
    data-path address. While a network with that VNI is programmed on the node, the node does not
    have that state.

.. expectation:: Old iptables encryption rules for a VNI remain until it is programmed
    :id: SWARM-CLEAN-ENCRYPTION-OTHER-BACKEND-NFT
    :observed_from: node
    :depends_on: os-linux
    :platforms: linux
    :exact_form_of: SWARM-CLEAN-ENCRYPTION-OTHER-BACKEND
    :pins: SWARM-CLEAN-ENCRYPTION-LEFT-IN-PLACE
    :needs: SWARM-HOST-IPTABLES-CLEANUP-COMMAND
    :within: SWARM-CEIL-REJOIN, SWARM-CEIL-REASSIGNMENT

    *Applies to:* A node with encryption rules for a VNI in filter :ipt-chain:`INPUT` and mangle
    :ipt-chain:`OUTPUT` that a previous generation left

    After a switch from iptables to nftables, the rules remain until a network with that VNI is
    programmed on the node. They do not remain after that.

.. expectation:: Old docker-overlay tables stay until an overlay is programmed or deleted
    :id: SWARM-CLEAN-ENCRYPTION-OTHER-BACKEND-IPT
    :observed_from: node
    :depends_on: os-linux
    :platforms: linux
    :exact_form_of: SWARM-CLEAN-ENCRYPTION-OTHER-BACKEND
    :pins: SWARM-CLEAN-ENCRYPTION-LEFT-IN-PLACE
    :needs: SWARM-HOST-NFT-CLEANUP-COMMAND, SWARM-HOST-IPTABLES-COMMAND
    :within: SWARM-CEIL-REJOIN, SWARM-CEIL-REASSIGNMENT

    *Applies to:* A node in a Swarm with an ingress network, with a :nft-table:`docker-overlay`
    table that a previous generation left under the nftables backend, in the family of the node's
    data-path address

    After the node switches firewall backend from nftables to iptables, the table remains until the
    first time after the daemon's start that an overlay network or the ingress network is programmed
    on the node, or that the node deletes one.

.. expectation:: A backend switch leaves old overlay encryption rules in place
    :id: SWARM-CLEAN-ENCRYPTION-LEFT-IN-PLACE
    :observed_from: node, log
    :depends_on: none
    :platforms: linux
    :needs: SWARM-HOST-IPTABLES-CLEANUP-COMMAND, SWARM-HOST-NFT-CLEANUP-COMMAND, SWARM-HOST-IPTABLES-COMMAND
    :within: SWARM-CEIL-REJOIN, SWARM-CEIL-REASSIGNMENT
    :narrows: SWARM-CLEAN-ENCRYPTION-OTHER-BACKEND, SWARM-CLEAN-ENCRYPTION-OTHER-BACKEND-IPT
    :violates: PRIN-NO-LEAKS

    After a switch from nftables to iptables, the :nft-table:`docker-overlay` table is deleted one
    time. This occurs the first time after the start that an overlay network or the ingress network
    is programmed on the node, or that the node deletes one. If that time comes before the daemon
    has initialized the node's part of the Swarm's networking on that start, which it does after the
    daemon log entry that contains :log:`Initializing Libnetwork Agent`, the table is not deleted.
    An error that contains :log:`Deleting overlay encryption nftables rules` then appears in the
    daemon log. The table stays in place until the next restart of the daemon. The node programs the
    ingress network only after that initialization. After a switch from iptables to nftables, a
    previous generation's encryption rules for a VNI stay in place. They stay until a network with
    that VNI is programmed on the node again. Cleanup deletes the :nft-table:`docker-overlay` table
    only in the family of the node's data-path address. A table in the other family, which a
    previous generation left when the node had a data-path address of that family, stays in place.
