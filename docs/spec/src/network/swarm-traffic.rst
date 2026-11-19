###########################
 Swarm networking: traffic
###########################

This document continues :doc:`swarm-networking.md <swarm-networking>`. Its conventions, definitions
and host requirements apply here.

.. _swarm-traffic-requirements:

**************
 Requirements
**************

.. req:: The API reports and changes a container's overlay endpoints as they are
    :id: REQ-SWARM-CTR-IFACE-API
    :source_kind: api
    :source: api/openapi.yaml: definition EndpointSettings (IPAddress/MacAddress), api/openapi.yaml: definition NetworkAttachment, api/openapi.yaml: operation NetworkDisconnect, https://docs.docker.com/reference/cli/docker/network/disconnect/

    The Engine API reports, for each network of a container, the IPv4 address and MAC address of its
    endpoint, and for a task the addresses of its network attachments. Disconnecting a container
    from a network removes it from that network.

.. req:: A dnsrr service cannot publish ingress-mode ports
    :id: REQ-SWARM-DNSRR-NO-INGRESS-PORTS
    :source_kind: docs
    :source: https://docs.docker.com/engine/swarm/ingress/#without-the-routing-mesh, api/openapi.yaml: definition EndpointSpec (Ports)

    The dnsrr endpoint mode cannot be combined with ports published in ingress mode; such a service
    publishes ports in host mode behind its own load balancer.

.. req:: Encrypted overlays carry traffic between nodes only inside IPsec ESP
    :id: REQ-SWARM-ENC-CONFIDENTIAL
    :source_kind: docs
    :source: https://docs.docker.com/engine/network/drivers/overlay/#encrypt-traffic-on-an-overlay-network, https://docs.docker.com/engine/swarm/networking/#encryption, https://docs.docker.com/engine/swarm/swarm-tutorial/#open-protocols-and-ports-between-the-hosts, https://docs.docker.com/engine/release-notes/23.0/#2303, https://docs.docker.com/engine/release-notes/29/#2990

    The encrypted option makes an overlay network encrypt its application data between nodes with
    IPsec at the VXLAN level. Traffic of an encrypted network does not cross between nodes in
    cleartext, and a node whose kernel lacks support for encryption reports an error for a task on
    such a network instead of running it.

.. req:: A service update can change the service's endpoint mode
    :id: REQ-SWARM-ENDPOINT-MODE-UPDATE
    :source_kind: docs
    :source: https://docs.docker.com/engine/swarm/networking/#configure-service-discovery, https://docs.docker.com/reference/cli/docker/service/update/

    The endpoint mode, vip or dnsrr, is set when a service is created or updated. In vip mode the
    service name resolves to a VIP that load-balances; in dnsrr mode it resolves to the tasks'
    addresses.

.. req:: Each node has a docker_gwbridge network that operators can pre-create
    :id: REQ-SWARM-GWBRIDGE
    :source_kind: docs
    :source: https://docs.docker.com/engine/swarm/networking/#overlay-networking, https://docs.docker.com/engine/swarm/networking/#customize-the-docker_gwbridge

    Swarm init and join create a bridge network called docker_gwbridge on the node, which connects
    its overlays to the physical network. An operator who creates docker_gwbridge before joining
    gets a gateway network with their own subnet and options.

.. req:: Only running, healthy tasks get service traffic and DNS records
    :id: REQ-SWARM-HEALTH-AWARE
    :source_kind: docs
    :source: https://docs.docker.com/engine/release-notes/prior-releases/#1130-2017-01-18, https://docs.docker.com/engine/swarm/networking/#container-discovery, https://docs.docker.com/engine/swarm/how-swarm-mode-works/services/#services-tasks-and-containers

    Load balancing and service DNS records are health aware: a task gets traffic and records only
    while it runs and, if it has a healthcheck, once it is healthy. A task that fails its health
    checks or stops leaves them.

.. req:: A port published in host mode keeps the client's source address
    :id: REQ-SWARM-HOSTPUB-CLIENT-SOURCE
    :source_kind: issue
    :source: https://github.com/moby/moby/issues/25526#issuecomment-263046272, https://github.com/moby/moby/issues/25526#issuecomment-263104849

    A port published in host mode keeps the client's source address, as docker run -p does.

.. req:: A mode=host port binds on each task's node, one task per node and port
    :id: REQ-SWARM-HOSTPUB-LOCAL
    :source_kind: docs
    :source: https://docs.docker.com/engine/swarm/ingress/#bypass-the-routing-mesh, https://docs.docker.com/engine/swarm/services/#publish-a-services-ports-directly-on-the-swarm-node, https://docs.docker.com/reference/cli/docker/service/create/#publish, https://docs.docker.com/engine/swarm/how-swarm-mode-works/services/#pending-services

    In host mode, a port is bound only on nodes where the service runs and reaches the task on that
    node; elsewhere nothing of the service listens. A given port on a node can be bound once, so
    tasks with the same chosen host port run on different nodes, and a task that no node can take
    stays pending.

.. req:: A task's assigned mode=host port is reported
    :id: REQ-SWARM-HOSTPUB-PORT-REPORTED
    :source_kind: docs
    :source: https://docs.docker.com/engine/swarm/ingress/#publish-a-port-for-a-service, api/openapi.yaml: definition PortStatus, https://docs.docker.com/engine/release-notes/28/#2810

    Without a published port, host mode binds a random high-numbered port for each task, and the
    user inspects the task to find it; the task's PortStatus reports its host ports. docker ps and
    docker inspect list each host port mapping of a task's container once.

.. req:: A stopping task leaves load balancing before it stops
    :id: REQ-SWARM-LB-GRACEFUL-DRAIN
    :source_kind: docs
    :source: https://docs.docker.com/engine/release-notes/18.03/#18031-ce

    The load balancer gracefully removes a task's endpoint before the task stops, so new flows go to
    other tasks while existing ones finish.

.. req:: VIPs, tasks.<service> and the routing mesh follow the running tasks
    :id: REQ-SWARM-MESH-FOLLOWS-TASKS
    :source_kind: docs
    :source: https://docs.docker.com/engine/swarm/networking/#container-discovery, https://docs.docker.com/engine/swarm/ingress/, https://docs.docker.com/engine/swarm/ingress/#using-the-routing-mesh

    A service's VIP load-balances across its running tasks, and tasks.<service> returns one address
    per running replica. The routing mesh sends requests to an active container, so an external load
    balancer needs no change when the scheduler moves tasks.

.. req:: Containers communicate only across networks they share
    :id: REQ-SWARM-NETWORK-ISOLATION
    :source_kind: docs
    :source: https://docs.docker.com/engine/network/drivers/overlay/, https://docs.docker.com/engine/swarm/networking/#customize-the-docker_gwbridge, https://docs.docker.com/engine/swarm/networking/#key-network-concepts
    :refines: PRIN-LEAST-ACCESS

    Services or containers can communicate only across networks that each of them is connected to by
    the user. The ingress network carries the routing mesh's load-balanced traffic, and
    docker_gwbridge connects the overlay networks to each node's physical network; neither connects
    containers to each other.

.. req:: A task of a service without a network is on the node's default network
    :id: REQ-SWARM-NO-NETWORK-DEFAULT-BRIDGE
    :source_kind: issue
    :source: https://github.com/moby/moby/issues/27082: a maintainer's description of the behavior in a feature design
    :violates: PRIN-LEAST-ACCESS

    When a service names no network and publishes no port in ingress mode, its tasks are attached to
    the node's default network: the default bridge on Linux.

.. req:: Non-internal overlays give egress via the node; internal ones do not
    :id: REQ-SWARM-OVERLAY-EGRESS
    :source_kind: docs
    :source: https://docs.docker.com/reference/cli/docker/network/create/#internal, https://docs.docker.com/engine/swarm/networking/#key-network-concepts, https://docs.docker.com/engine/swarm/networking/#customize-the-docker_gwbridge
    :refines: PRIN-LEAST-ACCESS

    When a container connects to an overlay, Docker also connects it to a bridge network
    (docker_gwbridge) that gives it external connectivity through the node, with masquerading. An
    overlay created as internal gives its containers no access to other networks.

.. req:: An overlay MTU-sized packet crosses nodes without fragmentation
    :id: REQ-SWARM-OVERLAY-MTU-FITS
    :source_kind: issue
    :source: https://github.com/moby/moby/issues/53765, https://github.com/moby/moby/issues/33596

    The MTU of a container's overlay interface leaves room for the encapsulation overhead of the
    data path, so a packet of that size crosses the underlay unfragmented and larger don't-fragment
    packets fail at the sender. This holds on an IPv6 data path too.

.. req:: A published port is the unused port chosen, or one from 30000 to 32767
    :id: REQ-SWARM-PORT-ALLOCATION
    :source_kind: docs
    :source: https://docs.docker.com/engine/swarm/key-concepts/#load-balancing, https://docs.docker.com/engine/swarm/ingress/#publish-a-port-for-a-service, https://docs.docker.com/engine/release-notes/prior-releases/#1123-2016-10-26

    A service can specify any unused port as its published port. If it does not, the swarm manager
    assigns it a port in the 30000-32767 range that no other service uses. Either way the service's
    endpoint reports the port. A service can have several auto-assigned ports.

.. req:: An auto-assigned published port keeps its number while the service keeps the port
    :id: REQ-SWARM-PORT-AUTO-KEPT
    :source_kind: observable
    :source: Clients and external load balancers that are configured with the number that docker service inspect reports for a port that a service publishes without a number and that keep using that number while the service publishes the port

    An auto-assigned published port keeps its number while the service keeps that port.

.. req:: A published port cannot be claimed by two users at once
    :id: REQ-SWARM-PORT-NO-CONFLICT
    :source_kind: docs
    :source: https://docs.docker.com/engine/swarm/ingress/#publish-a-port-for-a-service, https://docs.docker.com/reference/cli/docker/service/create/#publish, https://docs.docker.com/engine/swarm/services/#publish-a-services-ports-directly-on-the-swarm-node

    The routing mesh prevents port conflicts: a published port can be bound only once on a node, and
    another service or container cannot start on a port that is already published there. Existing
    users of a port keep it.

.. req:: One port number can be published for both TCP and UDP
    :id: REQ-SWARM-PORT-PROTOCOLS
    :source_kind: docs
    :source: https://docs.docker.com/engine/swarm/ingress/#publish-a-port-for-tcp-only-or-udp-only, https://docs.docker.com/engine/swarm/networking/#publish-ports-on-an-overlay-network

    A service can publish the same port number for TCP and for UDP through the routing mesh, as two
    separate published ports.

.. req:: Published ports follow service updates and removal
    :id: REQ-SWARM-PUB-FOLLOWS-SPEC
    :source_kind: docs
    :source: https://docs.docker.com/reference/cli/docker/service/update/#publish-add

    docker service update --publish-add adds or updates a published port and --publish-rm removes
    one, so the ports that the swarm serves follow the service's current spec.

.. req:: Published ports are reachable from the node and its containers
    :id: REQ-SWARM-PUB-HAIRPIN
    :source_kind: docs
    :source: https://docs.docker.com/engine/swarm/ingress/#publish-a-port-for-a-service, https://docs.docker.com/engine/release-notes/prior-releases/#1122-2016-10-11, https://docs.docker.com/engine/release-notes/29/#2990, https://github.com/moby/moby/issues/53713

    The routing mesh listens on every address of the node, and addresses that are not externally
    routable are reachable from within the host. Reaching a published port from other services and
    containers on the same host is supported, and must not depend on inter-container communication
    or on the userland proxy setting.

.. req:: With the userland proxy, published ports answer over IPv6
    :id: REQ-SWARM-PUB-IPV6-PROXY
    :source_kind: docs
    :source: https://docs.docker.com/engine/release-notes/29/#2990, https://github.com/moby/moby/issues/53091

    Published Swarm-service ports are accessible at the host's IPv6 addresses when the userland
    proxy is enabled. A task serves each IPv6 connection that such a port accepts.

.. req:: Published ports are tcp, udp or sctp, default tcp and ingress mode
    :id: REQ-SWARM-PUB-PROTOCOLS
    :source_kind: docs
    :source: https://docs.docker.com/reference/cli/docker/service/create/#publish, api/openapi.yaml: definition EndpointPortConfig (Protocol/PublishMode), https://docs.docker.com/engine/swarm/ingress/#publish-a-port-for-tcp-only-or-udp-only, https://docs.docker.com/engine/release-notes/prior-releases/#1123-2016-10-26

    A published port's protocol is tcp, udp or sctp and defaults to tcp; its mode is ingress or host
    and defaults to ingress.

.. req:: A published port delivers client traffic to a task's target port
    :id: REQ-SWARM-PUB-TARGET
    :source_kind: docs
    :source: https://docs.docker.com/engine/swarm/ingress/#publish-a-port-for-a-service, api/openapi.yaml: definition EndpointPortConfig (TargetPort/PublishedPort)

    A published port maps the port where the swarm makes the service available to the target port
    inside the service's containers, and the routing mesh or the node passes the traffic, and the
    task's replies, between the client and a task.

.. req:: A rolling update keeps the service reachable
    :id: REQ-SWARM-ROLLING-UPDATE
    :source_kind: issue
    :source: https://github.com/moby/moby/issues/30321, https://docs.docker.com/engine/release-notes/18.02/#18020-ce, https://docs.docker.com/engine/swarm/#rolling-updates

    A rolling update replaces a service's tasks a few at a time. It is a zero-downtime deployment:
    the service stays reachable while its tasks are replaced.

.. req:: Service network aliases are discoverable names for the service
    :id: REQ-SWARM-SD-ALIASES
    :source_kind: api
    :source: api/openapi.yaml: definition NetworkAttachmentConfig (Aliases), https://docs.docker.com/reference/cli/docker/service/create/#network, https://docs.docker.com/engine/release-notes/29/#2950, https://github.com/moby/moby/issues/44009, https://github.com/moby/moby/pull/52236: the merged fix and its description

    A service's network attachment can carry aliases, which are discoverable alternate names for the
    service on that network. During a rolling update, an alias resolves while a task that has it
    runs, and stops resolving once no task that has it runs.

.. req:: Containers on a shared network resolve each other by container name
    :id: REQ-SWARM-SD-CONTAINER-NAME
    :source_kind: docs
    :source: https://docs.docker.com/engine/network/drivers/overlay/#container-discovery, https://docs.docker.com/engine/network/#user-defined-networks, https://docs.docker.com/engine/release-notes/prior-releases/#1120-2016-07-28

    Containers on an overlay network are discoverable by a DNS lookup of the container name, as on
    any user-defined network. A renamed container resolves by its new name.

.. req:: A name on Swarm-scoped and local networks resolves to the Swarm record
    :id: REQ-SWARM-SD-PREFER-SWARM-SCOPED
    :source_kind: observable
    :source: Users who attach tasks or standalone containers to an overlay network and to a local network and reach peers by name. They rely on getting the overlay address without knowing that the name also has a record on the local network

    For a container attached to a Swarm-scoped network and to a local network, a name that has
    records on both networks resolves to its address on the Swarm-scoped network.

.. req:: Service names resolve to the VIP, or to the running tasks in dnsrr mode
    :id: REQ-SWARM-SD-SERVICE-NAMES
    :source_kind: docs
    :source: https://docs.docker.com/engine/swarm/networking/#configure-service-discovery, https://docs.docker.com/engine/swarm/networking/#container-discovery, https://docs.docker.com/engine/swarm/ingress/#without-the-routing-mesh, api/openapi.yaml: definition EndpointSpec (Mode)

    Each service gets a DNS name on its networks. In vip mode the name resolves to a virtual IP; in
    dnsrr mode there is no VIP and the name resolves to the task addresses. tasks.<service> resolves
    to the addresses of all running tasks.

.. req:: docker service create and update set ports, endpoint mode and aliases
    :id: REQ-SWARM-SERVICE-CLI-ENDPOINT
    :source_kind: docs
    :source: https://docs.docker.com/reference/cli/docker/service/create/#publish, https://docs.docker.com/reference/cli/docker/service/create/#network, https://docs.docker.com/reference/cli/docker/service/create/#options, https://docs.docker.com/reference/cli/docker/service/update/#publish-add, https://docs.docker.com/reference/cli/docker/service/update/#network-add

    docker service create takes --publish (short and long syntax), --endpoint-mode and --network
    with alias=; docker service update takes --publish-add, --endpoint-mode and --network-add with
    alias= to change them.

.. req:: A task's container is named <service>.<slot or node ID>.<task ID>
    :id: REQ-SWARM-TASK-CONTAINER-NAME
    :source_kind: issue
    :source: https://github.com/moby/swarmkit/issues/1554#issuecomment-6420749467: a maintainer's reason for not giving global tasks slots

    A task's container is named after the service, the task's slot and the task's ID, joined by
    dots. The tasks of a global service have no slot, and their containers have the node's ID in its
    place.

.. req:: Only published ports are reachable from outside the service's networks
    :id: REQ-SWARM-UNPUBLISHED-NOT-EXPOSED
    :source_kind: docs
    :source: https://docs.docker.com/engine/swarm/networking/#publish-ports-on-an-overlay-network
    :refines: PRIN-LEAST-ACCESS

    Services on an overlay expose all ports to each other, but a port is accessible from outside
    only when it is published with --publish.

.. req:: DSR keeps the client address and VIP; the default mode does not
    :id: REQ-SWARM-VIP-ADDRESS-MODES
    :source_kind: docs
    :source: https://docs.docker.com/engine/swarm/networking/#use-direct-server-return: what DSR leaves unchanged, https://github.com/moby/libnetwork/pull/2270: the merged change that made direct routing an option beside NAT and whose commit message says that NAT hides the sender's source address

    With the dsr option, east-west load balancing changes only the destination MAC address, so the
    task sees the client's source address and the VIP as destination. Without it, east-west load
    balancing uses NAT, which rewrites both addresses: the task sees neither the client's source
    address nor the VIP.

.. req:: A service VIP balances connections over its tasks from any node
    :id: REQ-SWARM-VIP-LOAD-BALANCING
    :source_kind: docs
    :source: https://docs.docker.com/engine/swarm/networking/#configure-service-discovery, https://docs.docker.com/engine/swarm/key-concepts/#load-balancing

    In vip mode, the service's virtual IP is the front end for clients on its networks, and Docker
    load-balances connections at layers 3 and 4 across the running tasks. Ports used between
    services on the same network need not be published.

.. _swarm-traffic-rationales:

************
 Rationales
************

.. rationale:: How a connection to a VIP without backends fails follows from the load balancer's implementation
    :id: RAT-SWARM-EMPTY-VIP-FAILURE-MODE

    A connection to the VIP of a service without running tasks can fail in three ways: with an
    immediate host-unreachable error, as today; with a TCP reset, which reads as a closed port on a
    host that is up; or with silence, which holds the client until its connect timeout. Nothing
    chooses among them. The outcome follows from how the node's load balancer treats a virtual
    server without backends. A change to the load balancer's implementation, or to the firewall
    backend, can therefore change how every client of a service without tasks fails, and the nodes
    of one Swarm can come to differ, so such a change should be deliberate.

.. rationale:: Carrying broadcast and multicast between nodes multiplies an overlay's traffic on the underlay
    :id: RAT-SWARM-FLOODING-STAYS-ON-NODE

    An overlay network delivers a broadcast frame or a multicast datagram only on the sender's node.
    To deliver it on other nodes, the sender's node has to send a copy to each node that has the
    network. That changes the load that an overlay network puts on the underlay, for every such
    frame. It also changes what containers receive: listeners and group members on other nodes begin
    to get the traffic. Cross-node delivery should arrive as a designed feature, not as a side
    effect of a change to the forwarding entries.

.. rationale:: A task's egress leaves through the gateway network, not through the routing mesh's network
    :id: RAT-SWARM-INGRESS-NO-EGRESS-ROUTE

    The ingress network carries the routing mesh's published-port traffic. A task's traffic to the
    physical network leaves through the node's gateway network, where the node masquerades it
    (:need:`SWARM-EGRESS-NON-INTERNAL`). A default route on the ingress network would move that
    traffic onto the mesh's network. Which network carries a task's egress should change by design.

.. rationale:: The routing mesh replaces the client's source address, and host mode exists to avoid that
    :id: RAT-SWARM-INGRESS-SOURCE-REPLACED

    A connection to a port published in ingress mode can arrive at any node and reach a task on
    another node, and the task sees a source address that routes its reply back through the node
    that received the connection. The :api-value:`mode=host` publish mode was introduced so that a
    service can keep the client's source address (:need:`REQ-SWARM-HOSTPUB-CLIENT-SOURCE`). A change
    that lets an ingress task see the client's address changes the return path of every ingress
    connection and the addresses that operators' logs and source-address filters record for every
    ingress-published service. It should be a decision about the routing mesh.

.. rationale:: The split rule decides how load spreads over a service's tasks
    :id: RAT-SWARM-LB-SCHEDULER

    The property under :ref:`swarm-traffic-distribution` in :doc:`swarm-traffic.md <swarm-traffic>`
    allows round-robin, equal-weight random and 5-tuple hashing. It excludes schedulers that depend
    on the state of the backends, such as least-connections. Current documentation promises load
    balancing over a service's tasks and no particular split. A walk-through in older documentation
    said that the load balancer chose tasks round-robin, and the rewrite of that page dropped the
    sentence. A move to a scheduler that depends on backend state changes the share of flows that
    each task gets when the tasks are loaded unevenly, for every service at once. It should be made
    as a decision.

.. rationale:: A MAC address derived from the IPv4 address is unique on the overlay without the nodes agreeing on it
    :id: RAT-SWARM-MAC-FROM-ADDRESS

    Each node generates the MAC addresses of its own endpoints on an overlay network, and the other
    nodes program those addresses as neighbor and FDB entries. The managers allocate each endpoint's
    IPv4 address, so a MAC address derived from it is unique on the network without any coordination
    between the nodes. It also lets a person who reads a packet capture or a forwarding table name
    the endpoint from the MAC address alone. The bridge driver no longer generates MAC addresses
    this way. A change to the overlay driver's scheme has to keep the addresses unique across nodes
    by other means, and should be deliberate.

.. _swarm-traffic-observable-state-inside-a-container:

*************************************
 Observable state inside a container
*************************************

In :doc:`network-common.md <network-common>`, :ref:`network-common-endpoint-interfaces` covers the
name and MAC address of an endpoint's interface and a static address, for every network driver, and
:ref:`network-common-default-gateway` covers the default route. Their expectations hold for overlay
endpoints. On Linux nodes, an overlay endpoint does not give the container a gateway or default
route, whatever its :api-field:`GwPriority`.

In this section, the inner MTU is the MTU of a container's interface on an overlay network. ``base``
is the value of the network's :drvopt:`com.docker.network.driver.mtu` option, or 1500 when the
option is 0 or not set. Each expectation about the inner MTU holds for each of a container's
interfaces on overlay networks, whatever other overlay networks the container is attached to.

.. expectation:: A container's overlay IPv4 address is the one the API reports
    :id: SWARM-CTR-ADDRESS
    :observed_from: task, ctr, api:mgr, api:wkr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-CTR-IFACE-API
    :within: SWARM-CEIL-ALLOCATION, SWARM-CEIL-REASSIGNMENT

    The IPv4 address of a container's interface on an overlay network is the :api-field:`IPAddress`
    that :api-op:`GET /containers/{id}/json` reports for that network. For a task, it is also the
    address that :api-op:`GET /tasks` on a manager reports in
    :api-field:`NetworksAttachments[].Addresses` for the network, without its prefix length. The
    address is in a subnet that :api-op:`GET /networks/{id}` reports for the network.

.. expectation:: A standalone container's configured overlay MAC address is reported
    :id: SWARM-CTR-MAC-SET
    :observed_from: ctr, api:mgr, api:wkr
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-SWARM-CTR-IFACE-API

    *Applies to:* Linux nodes with a standalone container whose endpoint settings for an overlay
    network set :api-field:`MacAddress`

    The MAC address of the container's interface on that network is the
    :api-field:`NetworkSettings.Networks.<network>.MacAddress` that :api-op:`GET
    /containers/{id}/json` reports.

.. expectation:: A container's generated overlay MAC address is the one reported
    :id: SWARM-CTR-MAC-GENERATED
    :observed_from: task, ctr, api:mgr, api:wkr
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-SWARM-CTR-IFACE-API
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* Linux nodes with a container whose endpoint settings for an overlay network do not
    set :api-field:`MacAddress`

    The MAC address of the container's interface on that network is the
    :api-field:`NetworkSettings.Networks.<network>.MacAddress` that :api-op:`GET
    /containers/{id}/json` reports.

.. expectation:: A disconnected container loses its Swarm-scoped network interface
    :id: SWARM-CTR-DISCONNECT
    :observed_from: ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-CTR-IFACE-API

    *Applies to:* A running standalone container on an attachable Swarm-scoped network

    After :api-op:`POST /networks/{id}/disconnect` for the container responds 200, the container
    does not have the interface that it had on that network.

.. expectation:: A generated overlay MAC address is 02:42 then the IPv4 address
    :id: SWARM-CTR-MAC-GENERATED-FORMAT
    :observed_from: task, ctr, api:mgr, api:wkr
    :depends_on: none
    :platforms: linux
    :within: SWARM-CEIL-REASSIGNMENT
    :rationale: RAT-SWARM-MAC-FROM-ADDRESS

    *Applies to:* Linux nodes with a container's endpoint on an overlay network that does not set
    :api-field:`MacAddress`

    The MAC address of the endpoint's interface is ``02:42:`` and then the four octets of its IPv4
    address.

.. expectation:: Don't-fragment packets of the inner MTU cross nodes unfragmented
    :id: SWARM-CTR-MTU-NO-FRAGMENT
    :observed_from: task → task, underlay
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-OVERLAY-MTU-FITS
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* An underlay whose MTU is ``base`` or more

    UDP traffic in IP packets the size of the inner MTU, with don't-fragment set, sent from a
    container's interface on an overlay network to a container on another node, reaches that
    container. The sending node sends each of those packets onto the underlay as one outer packet
    that is not fragmented. This holds on both cleartext and :drvopt:`encrypted` networks.

.. expectation:: An inner-MTU packet's outer packet is the underlay MTU, less up to 3
    :id: SWARM-CTR-MTU-OUTER-SIZE
    :observed_from: underlay
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-OVERLAY-MTU-FITS
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* An underlay whose MTU is ``base``

    For an IP packet the size of the inner MTU, sent from a container's interface on an overlay
    network to a container on another node, the outer packet on the underlay is exactly the underlay
    MTU on a cleartext network. On an :drvopt:`encrypted` network, it is at most the underlay MTU
    and more than the underlay MTU minus 4.

.. expectation:: On an IPv4 data path, the cleartext inner MTU is base − 50
    :id: SWARM-CTR-MTU-CLEARTEXT
    :observed_from: task
    :depends_on: os-windows
    :platforms: linux, windows
    :exact_form_of: SWARM-CTR-MTU-OUTER-SIZE
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A node with an IPv4 data-path address, and a cleartext network that, on a Windows
    node, does not have :drvopt:`com.docker.network.driver.mtu`

    The inner MTU is ``base − 50``. By default, the inner MTU is 1450.

.. expectation:: On an IPv6 data path, the cleartext inner MTU is base − 70
    :id: SWARM-CTR-MTU-CLEARTEXT-V6
    :observed_from: task
    :depends_on: none
    :platforms: linux
    :exact_form_of: SWARM-CTR-MTU-OUTER-SIZE
    :needs: SWARM-HOST-IPV6
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* Linux nodes with an IPv6 data-path address, and a cleartext network

    The inner MTU is ``base − 70``. By default, the inner MTU is 1430.

.. expectation:: On an IPv4 data path, the encrypted inner MTU is base − 76, 4-aligned
    :id: SWARM-CTR-MTU-ENCRYPTED
    :observed_from: task
    :depends_on: none
    :platforms: linux
    :exact_form_of: SWARM-CTR-MTU-OUTER-SIZE
    :needs: SWARM-HOST-XFRM, SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-IPTABLES-ENC-MATCHES, SWARM-HOST-IPTABLES-LEGACY-MANGLE-KERNEL, SWARM-HOST-IPTABLES-NFT-ROUTE-KERNEL
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* Linux nodes with an IPv4 data-path address, and an :drvopt:`encrypted` network

    The inner MTU is ``base − 50 − 26``, rounded down to a multiple of 4. By default, the inner MTU
    is 1424.

.. expectation:: On an IPv6 data path, the encrypted inner MTU is base − 96, 4-aligned
    :id: SWARM-CTR-MTU-ENCRYPTED-V6
    :observed_from: task
    :depends_on: none
    :platforms: linux
    :exact_form_of: SWARM-CTR-MTU-OUTER-SIZE
    :needs: SWARM-HOST-XFRM, SWARM-HOST-IPV6, SWARM-HOST-ESP-IPV6, SWARM-HOST-IP6TABLES-COMMAND, SWARM-HOST-IP6TABLES-KERNEL, SWARM-HOST-IP6TABLES-LEGACY-KERNEL, SWARM-HOST-IP6TABLES-NFT-KERNEL
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* Linux nodes with an IPv6 data-path address, and an :drvopt:`encrypted` network

    The inner MTU is ``base − 70 − 26``, rounded down to a multiple of 4. By default, the inner MTU
    is 1404.

.. expectation:: Between Windows nodes, an encrypted overlay's traffic is not encrypted
    :id: SWARM-CTR-WINDOWS-ENCRYPTION
    :observed_from: underlay
    :depends_on: none
    :platforms: windows
    :within: SWARM-CEIL-REASSIGNMENT
    :narrows: SWARM-CTR-MTU-NO-FRAGMENT, SWARM-CTR-MTU-OUTER-SIZE, SWARM-EW-UNICAST, SWARM-EW-CROSS-SUBNET, SWARM-EW-PROTOCOLS, SWARM-CLUSTER-DATA-PATH-PORT-DEFAULT
    :concerns: SWARM-ENC-NO-CLEARTEXT, SWARM-CTR-MTU-ENCRYPTED, SWARM-ENC-VNI-REUSE-WINDOWS, SWARM-OVLOPT-ENCRYPTED
    :violates: REQ-SWARM-ENC-CONFIDENTIAL, REQ-SWARM-NETWORK-DRIVER-OPTIONS, REQ-SWARM-OVERLAY-CONNECTIVITY, PRIN-FAILS-CLOSED

    Traffic between Windows nodes on an :drvopt:`encrypted` overlay is not encrypted. Traffic
    between Windows nodes and Linux nodes on such an overlay does not pass because the Linux peers
    drop cleartext VXLAN for the network's VNI. A Windows node's endpoint MTU on an
    :drvopt:`encrypted` network is the same as on a cleartext network.

.. expectation:: A Windows node's cleartext overlay MTU matches the Linux nodes' MTU
    :id: SWARM-CTR-MTU-CLEARTEXT-WINDOWS
    :observed_from: task on two nodes
    :depends_on: os-windows
    :platforms: windows
    :refines: REQ-SWARM-OVERLAY-MTU-FITS
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* Windows nodes, and a cleartext overlay without
    :drvopt:`com.docker.network.driver.mtu`

    An endpoint's MTU is the MTU that Linux nodes report for the same network.

.. expectation:: A set mtu option gives Linux and Windows nodes different inner MTUs
    :id: SWARM-CTR-MTU-OPTION-MIXED-PLATFORM
    :observed_from: task on two nodes
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :pins: SWARM-CTR-WINDOWS-MTU-OPTION
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A Linux node and a Windows node with tasks on a cleartext overlay network whose
    :drvopt:`com.docker.network.driver.mtu` is set to a value other than 0 or 1500

    The two nodes' inner MTUs on that network differ, because Windows nodes ignore the option. A
    frame larger than the smaller inner MTU, sent by a task on the node with the larger one, does
    not reach the task on the other node. The sender does not get an ICMP error. A TCP connection
    between tasks on the two nodes on that network exchanges data larger than the smaller inner MTU
    in both directions.

.. expectation:: A task on a Windows node ignores the network's mtu option
    :id: SWARM-CTR-WINDOWS-MTU-OPTION
    :observed_from: task
    :depends_on: none
    :platforms: windows
    :within: SWARM-CEIL-REASSIGNMENT
    :narrows: SWARM-CTR-MTU-NO-FRAGMENT, SWARM-CTR-MTU-OUTER-SIZE
    :concerns: SWARM-CTR-MTU-OPTION-MIXED-PLATFORM, SWARM-CTR-MTU-CLEARTEXT-WINDOWS, SWARM-OVLOPT-MTU-WINDOWS, SWARM-OVLOPT-MTU
    :violates: PRIN-FAILS-CLOSED, REQ-SWARM-NETWORK-DRIVER-OPTIONS, REQ-SWARM-OVERLAY-MTU-FITS

    On a network with :drvopt:`com.docker.network.driver.mtu` set, a task on a Windows node reports
    the inner MTU that it reports on a network without the option, not the inner MTU that a task on
    a Linux node reports.

.. expectation:: A container on a non-internal overlay has a default route
    :id: SWARM-CTR-DEFAULT-ROUTE
    :observed_from: task, ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :exact_form_of: SWARM-EGRESS-NON-INTERNAL
    :within: SWARM-CEIL-REASSIGNMENT

    A container on a non-internal overlay has a default route.

.. expectation:: A container on a multi-subnet overlay has a route to each other subnet
    :id: SWARM-CTR-MULTI-SUBNET-ROUTES
    :observed_from: task
    :depends_on: os-linux
    :platforms: linux
    :exact_form_of: SWARM-EW-CROSS-SUBNET
    :within: SWARM-CEIL-REASSIGNMENT

    A container on a multi-subnet overlay has a static route to each subnet of the overlay other
    than its own subnet.

.. _swarm-traffic-east-west-reachability:

************************
 East-west reachability
************************

Unless an expectation says otherwise, every expectation below holds whether the sender and receiver
are on the same node or on different nodes. It also holds in all four combinations in which each end
is either a service task or a standalone container on an ``attachable`` overlay. Any one of these
cases can fail on its own. One reason is that the two placements, same node and different nodes,
take different paths through the overlay. The other reason is that tasks and standalone containers
attach to the overlay differently.

.. expectation:: Unicast traffic reaches its destination on the same overlay subnet
    :id: SWARM-EW-UNICAST
    :observed_from: task, ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-OVERLAY-CONNECTIVITY
    :within: SWARM-CEIL-REASSIGNMENT

    Unicast traffic that a container sends reaches the destination container on the same overlay
    subnet.

.. expectation:: Encrypted traffic stops when a node pair exhausts an SA's sequence numbers
    :id: SWARM-EW-ESP-SEQUENCE-EXHAUSTION
    :observed_from: task, node, log
    :depends_on: os-linux
    :platforms: linux
    :needs: SWARM-HOST-XFRM, SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-FIREWALL-ESP, SWARM-HOST-IPTABLES-ENC-MATCHES, SWARM-HOST-IPTABLES-LEGACY-MANGLE-KERNEL, SWARM-HOST-IPTABLES-NFT-ROUTE-KERNEL
    :within: SWARM-CEIL-REASSIGNMENT
    :narrows: SWARM-EW-UNICAST, SWARM-EW-CROSS-SUBNET, SWARM-EW-PROTOCOLS, SWARM-ENC-KEY-ROTATION
    :violates: REQ-SWARM-OVERLAY-CONNECTIVITY

    When a node sends 2³² packets to another node on :drvopt:`encrypted` networks between two of the
    times that it installs new security associations for that node
    (:need:`SWARM-ENC-KEY-ROTATION-INTERVAL`), traffic from that node to the other node on those
    networks then does not pass until the node next installs new security associations for the other
    node, and neither node's daemon log has an entry about it. After a change of leader manager, the
    node installs no security associations for the other node with SPIs that it did not have before
    the change until 12 hours after the change. See `moby/moby#52005
    <https://github.com/moby/moby/issues/52005>`__.

.. expectation:: A broadcast frame reaches the same subnet's containers on its node only
    :id: SWARM-EW-BROADCAST-LOCAL
    :observed_from: task, ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :within: SWARM-CEIL-REASSIGNMENT
    :rationale: RAT-SWARM-FLOODING-STAYS-ON-NODE

    A broadcast frame that a container sends reaches the other containers on the same overlay subnet
    on its own node. It does not reach containers on other nodes.

.. expectation:: A multicast datagram reaches the group's members on its node only
    :id: SWARM-EW-MULTICAST-LOCAL
    :observed_from: task, ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :within: SWARM-CEIL-REASSIGNMENT
    :rationale: RAT-SWARM-FLOODING-STAYS-ON-NODE

    A multicast datagram that a container sends to a group reaches the other members of the group on
    the same overlay subnet on its own node. It does not reach members on other nodes.

.. expectation:: Unicast traffic reaches another subnet of the same overlay
    :id: SWARM-EW-CROSS-SUBNET
    :observed_from: task, ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-OVERLAY-CONNECTIVITY
    :within: SWARM-CEIL-REASSIGNMENT

    Unicast traffic that a container on subnet A sends reaches the destination container on subnet B
    of the same overlay network.

.. expectation:: A VIP reaches only backends in the client node's load-balancer subnet
    :id: SWARM-EW-VIP-MULTIPLE-SUBNETS
    :observed_from: task
    :depends_on: none
    :platforms: linux
    :within: SWARM-CEIL-REASSIGNMENT
    :concerns: SWARM-LB-ANY-PORT, SWARM-EW-CROSS-SUBNET
    :violates: REQ-SWARM-VIP-LOAD-BALANCING, REQ-SWARM-MESH-FOLLOWS-TASKS

    On Linux nodes, in NAT mode and in DSR mode, a backend task outside the subnet of the client
    node's load-balancer endpoint does not answer traffic to the VIP. A VIP outside that subnet does
    not reach any backend. The tasks' own addresses stay reachable across subnets.

.. expectation:: TCP, UDP and SCTP traffic reaches containers on the same overlay
    :id: SWARM-EW-PROTOCOLS
    :observed_from: task, ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-OVERLAY-CONNECTIVITY
    :needs: SWARM-HOST-SCTP
    :within: SWARM-CEIL-REASSIGNMENT

    TCP, UDP and SCTP traffic that a container sends reaches the destination container on the same
    overlay.

.. expectation:: Overlay traffic between Windows and Linux nodes fails over IPv6
    :id: SWARM-EW-WINDOWS-V6-DATA-PATH
    :observed_from: task
    :depends_on: none
    :platforms: windows
    :within: SWARM-CEIL-REJOIN, SWARM-CEIL-REASSIGNMENT
    :concerns: SWARM-WIRECLEAR-UDP-PORT, SWARM-EW-UNICAST, SWARM-EW-CROSS-SUBNET, SWARM-EW-PROTOCOLS, SWARM-CLUSTER-DATA-PATH-PORT-DEFAULT, SWARM-CLUSTER-DATA-PATH-ADDRESS-V6
    :violates: REQ-SWARM-CLUSTER-DATA-PATH-ADDRESS, REQ-SWARM-OVERLAY-CONNECTIVITY

    Overlay traffic between Windows nodes and Linux nodes does not pass over an IPv6 underlay.

.. expectation:: An oversized don't-fragment packet fails at the sender with EMSGSIZE
    :id: SWARM-EW-OVERSIZE
    :observed_from: task, ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-OVERLAY-MTU-FITS
    :within: SWARM-CEIL-REASSIGNMENT

    A don't-fragment packet larger than the MTU of the sender's interface on the overlay fails at
    the sender with :errno:`EMSGSIZE`.

.. expectation:: Containers with no overlay in common cannot reach each other
    :id: SWARM-EW-NO-SHARED-NETWORK
    :observed_from: task, ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-NETWORK-ISOLATION
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* Two containers that do not have an overlay network in common

    Neither container can reach an address that the other container has on an overlay network. This
    holds at every moment at which the two containers do not have an overlay network in common,
    without a ceiling.

.. expectation:: Containers cannot reach each other on a daemon-created gateway network
    :id: SWARM-EW-GW-NETWORK
    :observed_from: task, ctr
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-SWARM-NETWORK-ISOLATION
    :within: SWARM-CEIL-REASSIGNMENT

    Two containers on one node cannot reach each other's addresses on a gateway network that the
    operator did not create (:ref:`swarm-traffic-gateway-network`).

:need:`SWARM-EW-GW-NETWORK` is an isolation property. Containers on unrelated overlays on a node all
have addresses on the node's gateway network. If they could reach each other at those addresses, the
overlays would not isolate them.

.. _swarm-traffic-service-discovery:

*******************
 Service discovery
*******************

Service discovery in a Swarm is DNS. A container on a user-defined network sends its DNS queries to
the embedded DNS resolver, which is a DNS server that the daemon runs for the container. In this
section, a name resolves to an address when the embedded DNS resolver answers a DNS query for the
name's A record with that address. A name stops resolving when the embedded DNS resolver no longer
answers with an address for the name.

An expectation in this section names ``none`` when the embedded DNS resolver answers the query from
its own records. It does this the same way on every platform. Every expectation holds for a query
from a service task and for a query from a standalone container. This holds whatever kind of
workload the resolved name belongs to.

:doc:`container-dns.md <container-dns>` specifies how a container resolves names, whatever its
network's driver. It specifies the :file:`resolv.conf` that the container gets, how answers are
built, and what is forwarded upstream. Its expectations that list ``task`` among their vantage
points hold inside service tasks. They also hold for the names and addresses of the endpoints in
this section, including endpoints on other nodes. Each expectation keeps the category and
dependencies that it has in container-dns.md. This section covers the names that Swarm adds, on
overlay networks unless an expectation says otherwise. For the expectations in container-dns.md that
depend on a container's DNS options, a service task is created with those options in the service's
DNS config. On Linux nodes, a :api-field:`Links` alias of the querying container takes precedence
over the names in this section (:need:`DNS-NAMES-LINK-ALIAS-PRECEDENCE`).

In this section, a service's running tasks are the tasks that :api-op:`GET /tasks` reports with
:api-field:`Status.State` and :api-field:`DesiredState` both ``running``. If a task has a
healthcheck, it is a running task only once its container has reported healthy. This excludes a task
whose container has reported unhealthy at any time, even if a later health check passed.
:dns-name:`<service>` is the name of a service, and ``<alias>`` is one of the service's network
aliases. A service's network aliases are not network aliases of its tasks' endpoints: :api-op:`GET
/containers/{id}/json` for a task's container does not list them in the network's
:api-field:`Aliases` or :api-field:`DNSNames`, and they are not among the endpoint's names that
:doc:`container-dns.md <container-dns>` defines.

Where an expectation says that a name resolves to the addresses of tasks, these are the tasks'
addresses on one of the overlay networks, other than the ingress network, that the querier shares
with the service. The answer holds the addresses on one such network only. In vip mode, a service's
name and aliases have records for its VIP while the service has a running task. In dnsrr mode, they
have records for its running tasks. The :dns-name:`tasks.` names have records for the running tasks.
An expectation about a name holds where, on the querier's networks, no service, task or container
other than the one that the expectation names has records for that name, unless the expectation
names that case.

.. expectation:: A service name resolves to its VIP while a task runs
    :id: SWARM-SD-VIP
    :observed_from: task, ctr
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-SWARM-SD-SERVICE-NAMES, REQ-SWARM-ENDPOINT-MODE-UPDATE
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A service in vip mode that has a running task

    :dns-name:`<service>` resolves to the service's VIP on one of the overlay networks, other than
    the ingress network, that the querier shares with it.

.. expectation:: A dnsrr service name resolves to its running tasks
    :id: SWARM-SD-DNSRR
    :observed_from: task, ctr
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-SWARM-SD-SERVICE-NAMES, REQ-SWARM-ENDPOINT-MODE-UPDATE
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A service in dnsrr mode

    :dns-name:`<service>` resolves to the addresses of the running tasks.

.. expectation:: A dnsrr service reports no VIP
    :id: SWARM-SD-DNSRR-NO-VIP
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-SD-SERVICE-NAMES
    :within: SWARM-CEIL-ALLOCATION

    *Applies to:* A service in ``dnsrr`` endpoint mode

    :api-op:`GET /services/{id}` for the service does not report a VIP in
    :api-field:`Endpoint.VirtualIPs`.

.. expectation:: tasks.<service> resolves to all running tasks
    :id: SWARM-SD-TASKS
    :observed_from: task, ctr
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-SWARM-SD-SERVICE-NAMES
    :within: SWARM-CEIL-REASSIGNMENT

    :dns-name:`tasks.<service>` resolves to the addresses of all running tasks.

.. expectation:: A service alias resolves as the service name does
    :id: SWARM-SD-ALIAS
    :observed_from: task, ctr
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-SWARM-SD-ALIASES
    :within: SWARM-CEIL-REASSIGNMENT

    A service network alias resolves as :dns-name:`<service>` does: to the VIP in vip mode while the
    service has a running task, and to the addresses of the running tasks in dnsrr mode.

.. expectation:: tasks.<alias> resolves to the tasks of every service with the alias
    :id: SWARM-SD-TASKS-ALIAS
    :observed_from: task, ctr
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-SWARM-SD-ALIASES
    :within: SWARM-CEIL-REASSIGNMENT

    :dns-name:`tasks.<alias>` resolves to the addresses of the running tasks of every service that
    has that alias.

.. expectation:: A running task's container name resolves to its shared overlay address
    :id: SWARM-SD-TASK-NAME
    :observed_from: task, ctr
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-SWARM-SD-CONTAINER-NAME
    :within: SWARM-CEIL-REASSIGNMENT

    A running task's container name resolves to its address on an overlay network that it shares
    with the querier.

.. expectation:: A replicated task's container name is <service>.<slot>.<taskID>
    :id: SWARM-SD-TASK-NAME-REPLICATED
    :observed_from: api:mgr, api:wkr (on the task's node)
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-SWARM-TASK-CONTAINER-NAME
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A task of a replicated service

    On the task's node, :api-op:`GET /containers/{id}/json` for the task's container reports
    :api-field:`Name` as ``/<service>.<slot>.<taskID>``. ``<slot>`` is the task's :api-field:`Slot`
    that :api-op:`GET /tasks` reports.

.. expectation:: A global task's container name is <service>.<nodeID>.<taskID>
    :id: SWARM-SD-TASK-NAME-GLOBAL
    :observed_from: api:mgr, api:wkr (on the task's node)
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-SWARM-TASK-CONTAINER-NAME
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A task of a global service

    On the task's node, :api-op:`GET /containers/{id}/json` for the task's container reports
    :api-field:`Name` as ``/<service>.<nodeID>.<taskID>``.

.. expectation:: A task with a healthcheck resolves only once it reports healthy
    :id: SWARM-SD-HEALTH-PENDING
    :observed_from: task, ctr
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-SWARM-HEALTH-AWARE
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A task with a healthcheck whose container has not yet reported healthy

    The task's container name does not resolve to its address on an overlay network.
    :dns-name:`tasks.<service>` and :dns-name:`tasks.<alias>` do not resolve to the task's address.
    In dnsrr mode, :dns-name:`<service>` and the service's aliases do not resolve to the task's
    address either. A reverse lookup of the task's address on an overlay network does not return the
    task's container name. This holds from the :api-event:`start` event of the task's container,
    without a ceiling.

.. expectation:: A task stops resolving once it stops or reports unhealthy
    :id: SWARM-SD-TASK-STOPPED
    :observed_from: task, ctr
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-SWARM-HEALTH-AWARE
    :within: SWARM-CEIL-ALLOCATION, SWARM-CEIL-REASSIGNMENT

    Once a task's desired state is not ``running``, or its container has stopped or has reported
    unhealthy at any time, its container name stops resolving to its addresses on overlay networks.
    :dns-name:`tasks.<service>`, :dns-name:`tasks.<alias>` and, in dnsrr mode, :dns-name:`<service>`
    and the service's aliases then stop resolving to the task's address, and a reverse lookup of
    that address stops returning the task's container name.

.. expectation:: A removed service's names stop resolving to its VIP and tasks
    :id: SWARM-SD-SERVICE-REMOVED
    :observed_from: task, ctr
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-SWARM-SD-SERVICE-NAMES
    :within: SWARM-CEIL-ALLOCATION, SWARM-CEIL-REASSIGNMENT

    Once no container of a removed service is running, the service's name and aliases stop resolving
    to the service's VIP and to the addresses of its tasks.

.. expectation:: A vip service with no running task is forwarded upstream
    :id: SWARM-SD-VIP-NO-TASKS
    :observed_from: task, ctr
    :depends_on: none
    :platforms: linux, windows
    :pins: SWARM-SD-SERVICE-NAME-UPSTREAM
    :within: SWARM-CEIL-ALLOCATION, SWARM-CEIL-REASSIGNMENT

    *Applies to:* A service in vip mode that still exists, such as one scaled to 0 replicas

    Once no container of the service is running, the embedded resolver handles a query for
    :dns-name:`<service>` as it handles a name that it does not have a record for.

.. expectation:: A vip service created with no replicas is forwarded upstream
    :id: SWARM-SD-VIP-NO-REPLICAS
    :observed_from: task, ctr
    :depends_on: none
    :platforms: linux, windows
    :pins: SWARM-SD-SERVICE-NAME-UPSTREAM
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A service in vip mode that was created with no replicas

    The embedded resolver handles a query for :dns-name:`<service>` as it handles a name that it
    does not have a record for.

.. expectation:: A vip service with no healthy container is forwarded upstream
    :id: SWARM-SD-VIP-NONE-HEALTHY
    :observed_from: task, ctr
    :depends_on: none
    :platforms: linux, windows
    :pins: SWARM-SD-SERVICE-NAME-UPSTREAM
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A service in vip mode with running containers, each with a healthcheck, none of
    which has reported healthy

    The embedded resolver handles a query for :dns-name:`<service>` as it handles a name that it
    does not have a record for. For a service none of whose containers has reported healthy since
    the service was created, this holds from when the service is created, without a ceiling.

.. expectation:: A vip service's name without a running or healthy task goes upstream
    :id: SWARM-SD-SERVICE-NAME-UPSTREAM
    :observed_from: task, ctr
    :depends_on: none
    :platforms: linux, windows
    :needs: DNS-HOST-EXTERNAL-ACCESS
    :within: SWARM-CEIL-ALLOCATION, SWARM-CEIL-REASSIGNMENT
    :concerns: SWARM-SD-VIP, SWARM-SD-VIP-NO-TASKS, SWARM-SD-VIP-NO-REPLICAS, SWARM-SD-VIP-NONE-HEALTHY
    :violates: REQ-SWARM-SD-SERVICE-NAMES

    When a service in the ``vip`` endpoint mode has no running container, was created with no
    replicas, or has no container that has reported healthy, the embedded resolver forwards an A
    query for the service's name to the upstream servers, as it does for a name that it does not
    have a record for, and returns their answer (`moby/moby#52049
    <https://github.com/moby/moby/issues/52049>`__).

.. expectation:: The VIP of a service without running tasks gives EHOSTUNREACH
    :id: SWARM-SD-VIP-UNREACHABLE
    :observed_from: task, ctr
    :depends_on: os-linux
    :platforms: linux
    :within: SWARM-CEIL-ALLOCATION, SWARM-CEIL-REASSIGNMENT
    :rationale: RAT-SWARM-EMPTY-VIP-FAILURE-MODE

    *Applies to:* Linux nodes, and a service in vip mode that still exists, such as one scaled to 0
    replicas

    Once no container of the service is running, a TCP connection to the service's VIP fails at the
    client with :errno:`EHOSTUNREACH`. The client does not get a TCP reset.

.. expectation:: A renamed standalone container resolves by its new name on other nodes
    :id: SWARM-SD-RENAME
    :observed_from: task, ctr
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-SWARM-SD-CONTAINER-NAME, REQ-DNS-NAMES-DYNAMIC
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A standalone container on an attachable overlay, whose hostname and network
    aliases differ from its name in more than the case of their letters

    Once :api-op:`POST /containers/{id}/rename` for the container, with a new name that differs from
    its old name in more than the case of its letters, responds 204, its new name resolves for
    containers on other nodes within the :need:`[[title|lower]] ceiling ([[seconds]] s)
    <SWARM-CEIL-REASSIGNMENT>`. Within the same ceiling, its old name stops resolving to its address
    for those containers.

.. expectation:: A service alias keeps resolving through a rolling update
    :id: SWARM-SD-ALIAS-ROLLING-UPDATE
    :observed_from: task, ctr
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-SWARM-SD-ALIASES
    :within: SWARM-CEIL-ALLOCATION, SWARM-CEIL-REASSIGNMENT

    *Applies to:* A service whose tasks have a network alias

    A rolling update of the service that keeps the alias in the service's
    :api-field:`TaskTemplate.Networks` entry for its network, and in which at least one task keeps
    running throughout, starts new tasks that also have that alias. Examples are an update in
    start-first order and an update of fewer tasks at a time than are running. The alias does not
    stop resolving for longer than the :need:`[[title|lower]] ceiling ([[seconds]] s)
    <SWARM-CEIL-ALLOCATION>`: queries for the alias sent more than that length apart, and every
    query for the alias sent between them, do not all fail to resolve.

.. expectation:: An alias that an update drops resolves until no task that has it runs
    :id: SWARM-SD-ALIAS-DROPPED-UPDATE
    :observed_from: task, ctr
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-SWARM-SD-ALIASES
    :within: SWARM-CEIL-ALLOCATION, SWARM-CEIL-REASSIGNMENT

    *Applies to:* A service whose tasks have a network alias that no other service or container on
    that network has

    An update of the service removes the alias from its network attachment. While a task that has
    the alias is one of the service's running tasks, the alias resolves on that network. Once none
    of the service's running tasks has the alias, the alias does not resolve on that network. This
    holds once the :need:`[[title|lower]] ceiling ([[seconds]] s) <SWARM-CEIL-ALLOCATION>` has
    passed from the :api-event:`die` event of the container of the last task that has the alias.

.. expectation:: A name on Swarm-scoped and local networks resolves to the Swarm address
    :id: SWARM-SD-PREFER-SWARM-SCOPED
    :observed_from: ctr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-SD-PREFER-SWARM-SCOPED
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A container attached to a Swarm-scoped network and a local network, such as a
    user-defined bridge network, and records for the same name on each of those networks
    (:ref:`container-dns-names-and-answers` in :doc:`container-dns.md <container-dns>`)

    A query from that container for the name gets the address on the Swarm-scoped network, and not
    the address on the local network.

.. expectation:: On Windows, a local network's record wins over the Swarm-scoped one
    :id: SWARM-SD-WINDOWS-RESOLVER-PREFERENCE
    :observed_from: ctr
    :depends_on: os-windows
    :platforms: windows
    :within: SWARM-CEIL-REASSIGNMENT
    :concerns: SWARM-SD-PREFER-SWARM-SCOPED
    :violates: REQ-SWARM-SD-PREFER-SWARM-SCOPED

    Each network of a Windows container has its own embedded resolver, which answers only from the
    records of its network. For a name that has records on a Swarm-scoped network and on a local
    network, the resolver of the local network's adapter answers with the name's address on the
    local network.

.. expectation:: Services that share only the ingress network do not resolve each other
    :id: SWARM-SD-INGRESS-ONLY
    :observed_from: task
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-SWARM-INGRESS-MESH-ONLY
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* Two services that publish ports in ingress mode and do not share another network

    For a task of either service, the other service's name, and the container names of the other
    service's tasks, do not resolve to an address of the other service or of its tasks.

.. expectation:: On a node-local Swarm network, task names resolve as container names do
    :id: SWARM-SD-NODE-LOCAL
    :observed_from: task, ctr
    :depends_on: none
    :platforms: linux
    :refines: REQ-DNS-NAMES-CTR-NAME
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A Swarm-scoped network of a node-local driver

    A task's names resolve for the containers on its node that are attached to the network. They
    resolve as a container's names do on any user-defined network.

.. expectation:: On a node-local Swarm network, names resolve before the task is healthy
    :id: SWARM-SD-NODE-LOCAL-BEFORE-HEALTHY
    :observed_from: task, ctr
    :depends_on: none
    :platforms: linux
    :refines: REQ-DNS-NAMES-CTR-NAME
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A Swarm-scoped network of a node-local driver, and a task on it with a healthcheck
    whose container has not yet reported healthy

    The task's names resolve to its address on the network for the containers on its node that are
    attached to the network.

.. expectation:: On a node-local Swarm network, service names do not resolve
    :id: SWARM-SD-NODE-LOCAL-NO-SERVICE
    :observed_from: task, ctr
    :depends_on: none
    :platforms: linux
    :within: SWARM-CEIL-REASSIGNMENT
    :rationale: RAT-SWARM-NODE-LOCAL-NO-CLUSTER-RECORDS

    *Applies to:* A Swarm-scoped network of a node-local driver

    On that network, the embedded resolver handles a query for :dns-name:`<service>`,
    :dns-name:`tasks.<service>` or a service alias as it handles a name that it does not have a
    record for.

.. _swarm-traffic-load-balancing-east-west-vip:

********************************
 Load balancing (east-west VIP)
********************************

The client that sends to a VIP can be a service task or a standalone container on an ``attachable``
overlay. Every expectation in this section holds for either client. The backends behind a VIP are
always service tasks. An expectation observed at the receiving end is observed from ``task``. A VIP
load-balances across a set of tasks when flows to the VIP reach every task in the set. In this
section, a service's running tasks are those that :ref:`swarm-traffic-service-discovery` defines.

.. expectation:: The VIP load-balances TCP, UDP and SCTP traffic
    :id: SWARM-LB-PROTOCOLS
    :observed_from: task, ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-VIP-LOAD-BALANCING
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-SCTP, SWARM-HOST-IPVS-SCTP, SWARM-HOST-IPTABLES-LB-EXTENSIONS, SWARM-HOST-IPTABLES-LEGACY-MANGLE-KERNEL
    :within: SWARM-CEIL-REASSIGNMENT

    The VIP load-balances TCP, UDP and SCTP flows across the service's running tasks.

.. expectation:: A node without IPVS logs that it cannot get ipvs family information
    :id: SWARM-LB-NO-IPVS
    :observed_from: log
    :depends_on: none
    :platforms: linux
    :pins: SWARM-HOST-IPVS
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* Linux nodes whose kernel does not offer IPVS, and a service in vip mode with a
    running task on a network that is programmed on the node

    The daemon log of that node contains :log:`Could not get ipvs family information from the
    kernel`.

.. expectation:: All packets of a flow to a VIP reach the same task
    :id: SWARM-LB-FLOW-SAME-TASK
    :observed_from: task, ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-VIP-LOAD-BALANCING
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-IPTABLES-LB-EXTENSIONS, SWARM-HOST-IPTABLES-LEGACY-MANGLE-KERNEL
    :within: SWARM-CEIL-REASSIGNMENT

    All packets of a flow reach the same task.

.. expectation:: Traffic to any port of a service VIP reaches the service's tasks
    :id: SWARM-LB-ANY-PORT
    :observed_from: task, ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-VIP-LOAD-BALANCING, REQ-SWARM-UNPUBLISHED-NOT-EXPOSED
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-IPTABLES-LB-EXTENSIONS, SWARM-HOST-IPTABLES-LEGACY-MANGLE-KERNEL
    :within: SWARM-CEIL-REASSIGNMENT

    Traffic to any port on a service VIP reaches the service's tasks, whether or not the service
    publishes that port.

.. expectation:: In rootless mode with iptables, VIPs and published ports do not answer
    :id: SWARM-LB-ROOTLESS
    :observed_from: api:solo, api:mgr, task, client
    :depends_on: os-linux
    :platforms: linux
    :within: SWARM-CEIL-REASSIGNMENT
    :concerns: SWARM-LB-ANY-PORT, SWARM-INGPUB-EVERY-NODE
    :violates: PRIN-WARN-AHEAD, REQ-SWARM-VIP-LOAD-BALANCING, REQ-SWARM-ROUTING-MESH

    *Applies to:* A daemon that runs in rootless mode with the iptables backend

    :api-op:`POST /swarm/init` responds 200, and :api-op:`POST /networks/create` for an overlay
    network responds 201 without a warning. A service in the ``vip`` endpoint mode on a user-defined
    overlay network reports its tasks as running, but its VIP refuses connections. A service that
    publishes a port through the routing mesh reports its task as running, but the port does not
    answer. :api-op:`POST /swarm/join` on another node, for a Swarm whose manager runs rootless,
    responds with a 4xx or 5xx status, because the node's connection to the manager is refused.

.. expectation:: A node without a task of a service reaches the tasks through the VIP
    :id: SWARM-LB-TASK-FREE-NODE
    :observed_from: task or ctr on a task-free node
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-VIP-LOAD-BALANCING
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-IPTABLES-LB-EXTENSIONS, SWARM-HOST-IPTABLES-LEGACY-MANGLE-KERNEL
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A node without a task of a service

    Traffic to the service's VIP from a task or container on that node reaches the service's tasks.

.. expectation:: In NAT mode, a task sees a stable per-node source, not the client's
    :id: SWARM-LB-NAT-SOURCE
    :observed_from: task
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-VIP-ADDRESS-MODES
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-IPTABLES-LB-EXTENSIONS, SWARM-HOST-IPTABLES-LEGACY-MANGLE-KERNEL
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* NAT mode (the default)

    The task observes a source address that is stable for each client node. That address is neither
    the client's nor any task's.

.. expectation:: In NAT mode, the source a task sees is the client node's LB address
    :id: SWARM-LB-NAT-SOURCE-LB-ADDRESS
    :observed_from: task
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :exact_form_of: SWARM-LB-NAT-SOURCE
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-IPTABLES-LB-EXTENSIONS, SWARM-HOST-IPTABLES-LEGACY-MANGLE-KERNEL
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* NAT mode

    The source address that the task observes is the load-balancer address of the client's node on
    the overlay.

.. expectation:: In NAT mode, a task sees its own address as the destination
    :id: SWARM-LB-NAT-DESTINATION
    :observed_from: task
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-VIP-ADDRESS-MODES
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-IPTABLES-LB-EXTENSIONS, SWARM-HOST-IPTABLES-LEGACY-MANGLE-KERNEL
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* NAT mode

    The destination the task observes is its own address.

.. expectation:: In DSR mode, a task sees the client's real source address
    :id: SWARM-LB-DSR-SOURCE
    :observed_from: task
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-SWARM-VIP-ADDRESS-MODES
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-IPTABLES-LB-EXTENSIONS, SWARM-HOST-IPTABLES-LEGACY-MANGLE-KERNEL
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* DSR mode (the :drvopt:`dsr` option), with the client and the task on Linux nodes

    The task observes the client's real source address.

.. expectation:: In DSR mode, a task sees the VIP as the destination
    :id: SWARM-LB-DSR-DESTINATION
    :observed_from: task
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-SWARM-VIP-ADDRESS-MODES
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-IPTABLES-LB-EXTENSIONS, SWARM-HOST-IPTABLES-LEGACY-MANGLE-KERNEL
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* DSR mode, with the client and the task on Linux nodes

    The destination the task observes is the VIP.

.. expectation:: A connection to a task's own address arrives with the sender's address
    :id: SWARM-LB-TASK-ADDRESS
    :observed_from: task
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-OVERLAY-CONNECTIVITY
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A task, and a sender task or container that shares an overlay subnet with it

    The sender connects to the task's own address on that subnet. The connection arrives at the task
    with the sender's address on that subnet as its source.

.. expectation:: A task's connections to its own service's VIP reach the service's tasks
    :id: SWARM-LB-SELF-VIP
    :observed_from: task
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-VIP-LOAD-BALANCING
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-IPTABLES-LB-EXTENSIONS, SWARM-HOST-IPTABLES-LEGACY-MANGLE-KERNEL
    :within: SWARM-CEIL-REASSIGNMENT

    Every connection from a task to the VIP of its own service reaches a task of the service.

.. expectation:: In NAT mode, a task's own VIP spreads its connections over all tasks
    :id: SWARM-LB-SELF-VIP-NAT
    :observed_from: task
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :exact_form_of: SWARM-LB-SELF-VIP
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-IPTABLES-LB-EXTENSIONS, SWARM-HOST-IPTABLES-LEGACY-MANGLE-KERNEL
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* NAT mode

    The VIP of a service load-balances the connections from one of its tasks across all of the
    service's tasks, that task included.

.. expectation:: In DSR mode, a task's connections to its own VIP come back to it
    :id: SWARM-LB-SELF-VIP-DSR
    :observed_from: task
    :depends_on: os-linux
    :platforms: linux
    :exact_form_of: SWARM-LB-SELF-VIP
    :pins: SWARM-LBHEALTH-DSR-OWN-VIP
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-IPTABLES-LB-EXTENSIONS, SWARM-HOST-IPTABLES-LEGACY-MANGLE-KERNEL
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* DSR mode

    Every connection from a task to the VIP of its own service reaches the task itself. The task
    observes the VIP as the source address of the connection.

.. expectation:: Windows nodes load-balance in NAT mode whatever the dsr option says
    :id: SWARM-LB-WINDOWS-DSR
    :observed_from: task
    :depends_on: none
    :platforms: windows
    :within: SWARM-CEIL-REASSIGNMENT
    :concerns: SWARM-OVLOPT-DSR, SWARM-OVLOPT-DSR-WINDOWS, SWARM-LB-SELF-VIP-DSR, SWARM-LB-DSR-SOURCE, SWARM-LB-DSR-DESTINATION
    :violates: PRIN-FAILS-CLOSED, REQ-SWARM-VIP-ADDRESS-MODES, REQ-SWARM-NETWORK-DRIVER-OPTIONS

    Windows nodes load-balance VIP traffic in NAT mode, whatever the network's :drvopt:`dsr` option
    says. A task on a Windows node does not observe the client's real source address, and does not
    observe the VIP.

.. _swarm-traffic-distribution:

Distribution
============

The distribution property allows round-robin, equal-weight random, and 5-tuple hashing. It excludes
least-connections and other schedulers that depend on backend state. Clause (1), a limit, cannot
exclude them on its own because least-connections also converges to 1/*k* with identical backends.
Clause (2), invariance, tells them apart. The property does not fix a number of flows or a tolerance
on *fᵢ(n)/n*. A difference in connection holding time is one of the properties in clause (2), and
tells least-connections apart from round-robin.

    Let *H* be the set of running tasks, \|\ *H*\| = *k*, held constant. For *n* client flows let
    *fᵢ(n)* be the number of them that reach task *i*. Then (1) lim *fᵢ(n)/n* = 1/*k* for every *i*
    ∈ *H*, and (2) that limit is invariant to any observable property of the tasks: connection
    holding time, in-flight concurrency, latency, byte volume, or load.

.. expectation:: East-west VIP flows split evenly over running tasks, whatever their load
    :id: SWARM-LBDIST-VIP
    :observed_from: task
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-IPTABLES-LB-EXTENSIONS, SWARM-HOST-IPTABLES-LEGACY-MANGLE-KERNEL
    :within: SWARM-CEIL-REASSIGNMENT
    :rationale: RAT-SWARM-LB-SCHEDULER

    Clauses (1) and (2) of the property above hold for east-west VIP.

.. expectation:: Ingress flows split evenly over running tasks, whatever their load
    :id: SWARM-LBDIST-INGRESS
    :observed_from: client, task
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-IPTABLES-LB-EXTENSIONS, SWARM-HOST-IPTABLES-LEGACY-MANGLE-KERNEL, BR-HOST-NO-FORWARD-DROP
    :within: SWARM-CEIL-REASSIGNMENT
    :rationale: RAT-SWARM-LB-SCHEDULER

    Clauses (1) and (2) of the property above hold for ingress.

.. expectation:: After a load-balancer failure, failed backends get no flows until replaced
    :id: SWARM-LBDIST-RECOVERY-INCOMPLETE
    :observed_from: task
    :depends_on: none
    :platforms: linux
    :within: SWARM-CEIL-REASSIGNMENT
    :narrows: SWARM-LBDIST-VIP, SWARM-LBDIST-INGRESS, SWARM-LB-PROTOCOLS, SWARM-LB-TASK-FREE-NODE, SWARM-LB-ANY-PORT
    :concerns: SWARM-FAILREP-LB-RECOVERY
    :violates: REQ-SWARM-MESH-FOLLOWS-TASKS, REQ-SWARM-VIP-LOAD-BALANCING

    *Applies to:* A Linux node with the iptables backend where a node-local failure to program a
    load-balancer backend of a service occurred, and its cause has cleared

    When a task is then added to the service, the node programs only that task's backend. A backend
    whose programming failed does not get traffic through the node's load balancer until its task is
    replaced. This includes every backend that arrived while the failure continued.

.. _swarm-traffic-health-and-drain:

Health and drain
================

Every expectation in this subsection holds for flows through the service's VIP and for flows that
enter through an ingress-mode published port. A task shuts down gracefully when the orchestrator
shuts it down, for example after a scale-down, an update that replaces the task, the removal of its
service or a drain of its node. :api-op:`POST /containers/{id}/stop` for the task's container is not
a graceful shutdown in these expectations. A flow starts on a node when its client is on that node,
or when it enters through a published port on that node. In these expectations, a task's process
ignores its stop signal and keeps accepting connections until it is killed at the end of the stop
grace period.

.. expectation:: A task with a healthcheck gets no traffic until it first reports healthy
    :id: SWARM-LBHEALTH-PENDING
    :observed_from: task
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-HEALTH-AWARE
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-IPTABLES-LB-EXTENSIONS, SWARM-HOST-IPTABLES-LEGACY-MANGLE-KERNEL, BR-HOST-NO-FORWARD-DROP
    :within: SWARM-CEIL-REASSIGNMENT

    A task with a healthcheck does not receive traffic until it first reports healthy. This holds
    from the :api-event:`start` event of the task's container, without a ceiling.

.. expectation:: A task without a healthcheck gets traffic once it is running
    :id: SWARM-LBHEALTH-NO-HEALTHCHECK
    :observed_from: task
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-HEALTH-AWARE
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-IPTABLES-LB-EXTENSIONS, SWARM-HOST-IPTABLES-LEGACY-MANGLE-KERNEL, BR-HOST-NO-FORWARD-DROP
    :within: SWARM-CEIL-REASSIGNMENT

    A task that does not have a healthcheck receives traffic once it is running, without waiting for
    a health status.

.. expectation:: An unhealthy task gets no new flows within the allocation ceiling
    :id: SWARM-LBHEALTH-UNHEALTHY
    :observed_from: task
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-HEALTH-AWARE
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-IPTABLES-LB-EXTENSIONS, SWARM-HOST-IPTABLES-LEGACY-MANGLE-KERNEL, BR-HOST-NO-FORWARD-DROP
    :within: SWARM-CEIL-ALLOCATION, SWARM-CEIL-REASSIGNMENT

    *Applies to:* A service whose tasks have a healthcheck and a stop grace period longer than the
    :need:`[[title|lower]] ceiling ([[seconds]] s) <SWARM-CEIL-ALLOCATION>`

    Within the :need:`[[title|lower]] ceiling ([[seconds]] s) <SWARM-CEIL-ALLOCATION>` after one of
    the service's tasks becomes unhealthy, no new flows from any node reach that task.

.. expectation:: An established connection works until the stopping task closes it
    :id: SWARM-LBHEALTH-DRAIN-ESTABLISHED
    :observed_from: task
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-LB-GRACEFUL-DRAIN
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-IPTABLES-LB-EXTENSIONS, SWARM-HOST-IPTABLES-LEGACY-MANGLE-KERNEL, BR-HOST-NO-FORWARD-DROP
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* An established connection to a task that has been running for longer than the
    :need:`[[title|lower]] ceiling ([[seconds]] s) <SWARM-CEIL-REASSIGNMENT>`

    The task shuts down gracefully. The connection keeps exchanging data in both directions until
    the task closes it, within the task's stop grace period.

.. expectation:: No flow from the task's node reaches it after its first kill event
    :id: SWARM-LBHEALTH-DRAIN-LOCAL
    :observed_from: task
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-LB-GRACEFUL-DRAIN
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-IPTABLES-LB-EXTENSIONS, SWARM-HOST-IPTABLES-LEGACY-MANGLE-KERNEL, BR-HOST-NO-FORWARD-DROP
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A task with a stop grace period of at least 10 seconds, and another task of the
    service that has been running for longer than the :need:`[[title|lower]] ceiling ([[seconds]] s)
    <SWARM-CEIL-REASSIGNMENT>`, and has reported healthy if it has a healthcheck, and whose
    :api-field:`DesiredState` stays ``running`` while the task shuts down

    When the task shuts down gracefully, no flow that starts on the task's node after the
    container's first :api-event:`kill` event, the one for its stop signal, reaches the task. Each
    such flow reaches another running task of the service. This holds at once, without a ceiling,
    until the container's :api-event:`die` event.

.. expectation:: In DSR mode, a task's connections to its own VIP reach only itself
    :id: SWARM-LBHEALTH-DSR-OWN-VIP
    :observed_from: task
    :depends_on: none
    :platforms: linux
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-IPTABLES-LB-EXTENSIONS, SWARM-HOST-IPTABLES-LEGACY-MANGLE-KERNEL
    :within: SWARM-CEIL-REASSIGNMENT
    :narrows: SWARM-LBHEALTH-PENDING, SWARM-LBHEALTH-DRAIN-LOCAL, SWARM-LBHEALTH-UNHEALTHY, SWARM-LBDIST-VIP, SWARM-LB-PROTOCOLS, SWARM-CHURN-DNSRR-TO-VIP-BALANCES
    :concerns: SWARM-LB-SELF-VIP-DSR, SWARM-LB-SELF-VIP
    :violates: REQ-SWARM-VIP-LOAD-BALANCING, REQ-SWARM-HEALTH-AWARE, REQ-SWARM-LB-GRACEFUL-DRAIN

    On Linux nodes, in DSR mode, each task of a service has the service's VIP as a local address. A
    connection from a task to the VIP of its own service reaches that task. The other tasks of the
    service do not get these connections. The task gets them also while it does not yet report
    healthy, and while it shuts down. Connections from other clients are load-balanced.

.. expectation:: Within the ceiling, flows from other nodes stop reaching a stopping task
    :id: SWARM-LBHEALTH-DRAIN-REMOTE
    :observed_from: task
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-LB-GRACEFUL-DRAIN
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-IPTABLES-LB-EXTENSIONS, SWARM-HOST-IPTABLES-LEGACY-MANGLE-KERNEL, BR-HOST-NO-FORWARD-DROP
    :within: SWARM-CEIL-ALLOCATION, SWARM-CEIL-REASSIGNMENT

    *Applies to:* A task with a stop grace period longer than the :need:`[[title|lower]] ceiling
    ([[seconds]] s) <SWARM-CEIL-ALLOCATION>`, and another task of the service that has been running
    for longer than the :need:`[[title|lower]] ceiling ([[seconds]] s) <SWARM-CEIL-REASSIGNMENT>`,
    and has reported healthy if it has a healthcheck, and whose :api-field:`DesiredState` stays
    ``running`` while the task shuts down

    The task shuts down gracefully. Within the :need:`[[title|lower]] ceiling ([[seconds]] s)
    <SWARM-CEIL-ALLOCATION>` after :api-op:`GET /tasks` first reports a :api-field:`DesiredState`
    other than ``running`` for the task, no flow that starts on another node reaches the task. Each
    such flow reaches another running task of the service.

.. _swarm-traffic-published-ports:

*****************
 Published ports
*****************

A published port is a port number and a protocol, as the :api-field:`PublishedPort` and
:api-field:`Protocol` of an entry in a service's :api-field:`EndpointSpec.Ports` give them. An entry
without :api-field:`Protocol` has the protocol ``tcp``. Thus ``53/tcp`` and ``53/udp`` are two
different published ports. A published port is chosen when the request that creates or updates the
service sets its :api-field:`PublishedPort` (:ref:`swarm-traffic-port-allocation`).

A service publishes a port in one of two modes:

- :api-value:`mode=ingress`, the default, publishes the port through the routing mesh on every node.
- :api-value:`mode=host` publishes the port directly on the node that runs each task.

The expectations in this section hold for both modes. For a port that a service publishes in
:api-value:`mode=host`, an expectation in this section that refers to a node's address, or to the
node's addresses, without saying which node, holds at the addresses of the nodes that run a task of
the service. They hold with :daemon-json:`userland-proxy` true and with
:daemon-json:`userland-proxy` false, unless an expectation names a value.
:ref:`swarm-traffic-hairpin` covers hairpin paths. The :ref:`swarm-traffic-api-value-mode-ingress`
subsection covers what is specific to the routing mesh, and the
:ref:`swarm-traffic-api-value-mode-host` subsection covers what is specific to
:api-value:`mode=host`.

Every expectation in this section and its subsections holds whether or not the :drvopt:`dsr` option
is set on the overlay networks that a service is attached to, other than the ingress network.

.. expectation:: A flow to a published port reaches a task on its target port
    :id: SWARM-PUB-TARGET-PORT
    :observed_from: task
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-PUB-TARGET
    :needs: BR-HOST-NO-FORWARD-DROP
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A service that publishes a port

    A flow from a client to that port at a node's address reaches a task of the service on its
    target port.

.. expectation:: A client reaches a task through a TCP, UDP or SCTP published port
    :id: SWARM-PUB-PROTOCOLS
    :observed_from: client
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-PUB-PROTOCOLS
    :needs: SWARM-HOST-SCTP, SWARM-HOST-IPTABLES-SCTP-MATCH, BR-HOST-NO-FORWARD-DROP
    :within: SWARM-CEIL-REASSIGNMENT

    A client reaches a task through a published port, whether the port is TCP, UDP or SCTP.

.. expectation:: A task's replies reach the client that connected to its published port
    :id: SWARM-PUB-REPLIES
    :observed_from: client
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-PUB-TARGET
    :needs: BR-HOST-NO-FORWARD-DROP
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A service that publishes a port

    Replies from a task of the service to a client that connected to that port reach the client.

.. expectation:: UDP traffic to a port with no listener gets ICMP port-unreachable
    :id: SWARM-PUB-UDP-NO-LISTENER
    :observed_from: client
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-SWARM-PUB-TARGET
    :needs: BR-HOST-NO-FORWARD-DROP
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A service that publishes a UDP port, with tasks in which nothing listens on the
    target port

    UDP traffic from a client to that port at a node's IPv4 address gets an ICMP port-unreachable
    error at the client.

.. expectation:: A client does not reach a task on a port its service does not publish
    :id: SWARM-PUB-UNPUBLISHED-PORT
    :observed_from: client → task
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-UNPUBLISHED-NOT-EXPOSED
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A task that listens on a port that its service does not publish

    A connection or datagram that a client sends to that port at a node's address does not reach the
    task.

.. expectation:: A spec that lists the same chosen published port twice is refused
    :id: SWARM-PUB-DUPLICATE-PORT
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-PORT-NO-CONFLICT, REQ-NET-API-ERROR-STATUS

    :api-op:`POST /services/create` or :api-op:`POST /services/{id}/update` with a spec that lists
    the same chosen published port twice responds with a 4xx status, whatever the publish modes and
    target ports of the two entries are. The service is not created or not changed.

.. expectation:: A spec that lists the same chosen published port twice responds 400
    :id: SWARM-PUB-DUPLICATE-PORT-HTTP400
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :exact_form_of: SWARM-PUB-DUPLICATE-PORT

    :api-op:`POST /services/create` or :api-op:`POST /services/{id}/update` with a spec that lists
    the same chosen published port twice responds 400, whatever the publish modes and target ports
    of the two entries are.

.. expectation:: docker service create with one published port twice fails
    :id: SWARM-PUB-DUPLICATE-PORT-CLI
    :observed_from: api:mgr
    :depends_on: cli
    :platforms: linux
    :refines: REQ-SWARM-PORT-NO-CONFLICT
    :cli_for: SWARM-PUB-DUPLICATE-PORT

    :cli:`docker service create` with two :cli-flag:`--publish` options that set the same published
    port exits non-zero and prints the daemon's error. The command does not create the service.

.. expectation:: docker service update --publish-add of a used port to a new target fails
    :id: SWARM-PUB-DUPLICATE-PORT-UPDATE-CLI
    :observed_from: api:mgr
    :depends_on: cli
    :platforms: linux
    :cli_for: SWARM-PUB-DUPLICATE-PORT
    :concerns: SWARM-PUB-DUPLICATE-PORT
    :violates: REQ-SWARM-PUB-FOLLOWS-SPEC

    *Applies to:* A service with a chosen published port to some target port

    :cli:`docker service update --publish-add` that publishes the same published port to another
    target port exits non-zero and prints the daemon's error. The command does not change the
    service.

.. expectation:: --publish-add of a port the service already has changes nothing
    :id: SWARM-PUB-PUBLISH-ADD-EXISTING
    :observed_from: api:mgr
    :depends_on: cli
    :platforms: linux
    :refines: REQ-SWARM-PUB-FOLLOWS-SPEC

    *Applies to:* A service with a chosen published port

    :cli:`docker service update --publish-add` with the same published port, target port and publish
    mode exits zero. :cli:`docker service inspect` then prints the same ports as before.

.. expectation:: --publish-add of an auto-assigned port's number reassigns that port
    :id: SWARM-PUB-PUBLISH-ADD-AUTO-NUMBER
    :observed_from: api:mgr
    :depends_on: cli
    :platforms: linux
    :pins: SWARM-PORTALLOC-AUTO-UPDATE-REASSIGNED
    :within: SWARM-CEIL-ALLOCATION

    *Applies to:* A service with an auto-assigned published port

    :cli:`docker service update --publish-add` that publishes that port's number to another target
    port exits zero. :cli:`docker service inspect` then prints that number for the new entry, and
    another number for the auto-assigned entry.

.. expectation:: A published port defaults to tcp and to ingress mode
    :id: SWARM-PUB-DEFAULTS
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-PUB-PROTOCOLS

    :api-op:`POST /services/create` with a published port that does not set :api-field:`Protocol`
    responds 201. :api-op:`GET /services/{id}` then reports ``tcp`` as the port's protocol. It
    reports :api-value:`PublishMode=ingress` for a port that does not set :api-field:`PublishMode`.

.. expectation:: A published port with an unknown protocol or publish mode is accepted
    :id: SWARM-PUB-UNKNOWN-PROTOCOL
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-NET-API-BACKWARD-COMPATIBLE

    :api-op:`POST /services/create` for a service in the ``vip`` endpoint mode, with a published
    port whose :api-field:`Protocol` is not a protocol name, such as ``bogus``, or whose
    :api-field:`PublishMode` the API reference does not list, responds 201. :api-op:`GET /services`
    then lists the service.

.. expectation:: Unknown Protocol and PublishMode values are stored as tcp and ingress
    :id: SWARM-PUB-UNKNOWN-PROTOCOL-MODE
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :concerns: SWARM-PUB-UNKNOWN-PROTOCOL
    :violates: PRIN-FAILS-CLOSED

    :api-op:`POST /services/create` for a service in the ``vip`` endpoint mode, with a published
    port whose :api-field:`Protocol` or :api-field:`PublishMode` is a non-empty value that is, in
    any letter case, none of the values that the API reference lists for that field, responds 201.
    An unknown :api-field:`Protocol` is stored as ``tcp``, and an unknown :api-field:`PublishMode`
    as ingress mode.

.. expectation:: docker service create applies its port, endpoint mode and alias flags
    :id: SWARM-PUB-CREATE-FLAGS
    :observed_from: api:mgr
    :depends_on: cli
    :platforms: linux
    :refines: REQ-SWARM-SERVICE-CLI-ENDPOINT

    :cli:`docker service create` with :cli-flag:`--publish`, in its short and long forms,
    :cli-flag:`--endpoint-mode` and :cli-flag:`--network` with :cli-flag:`alias=` exits zero when
    the endpoint mode is ``vip`` or every port is published in :api-value:`mode=host`. Then
    :cli:`docker service inspect` prints the ports, publish modes, endpoint mode and aliases that
    the flags name.

.. expectation:: docker service update applies its port, endpoint mode and alias flags
    :id: SWARM-PUB-UPDATE-FLAGS
    :observed_from: api:mgr
    :depends_on: cli
    :platforms: linux
    :refines: REQ-SWARM-SERVICE-CLI-ENDPOINT

    *Applies to:* A service

    :cli:`docker service update` with :cli-flag:`--publish-add`, in its short and long forms,
    :cli-flag:`--endpoint-mode` and :cli-flag:`--network-add` with :cli-flag:`alias=` exits zero
    when the endpoint mode is ``vip`` or every port is published in :api-value:`mode=host`. Then
    :cli:`docker service inspect` prints the ports, publish modes, endpoint mode and aliases that
    the flags name.

.. expectation:: A removed service's published ports do not answer
    :id: SWARM-PUB-SERVICE-REMOVED
    :observed_from: client, node
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-PUB-FOLLOWS-SPEC
    :within: SWARM-CEIL-ALLOCATION, SWARM-CEIL-REASSIGNMENT

    *Applies to:* A service, and no other service or container that publishes one of its ports

    After :api-op:`DELETE /services/{id}` for the service responds 200, once no container of the
    service is running, the service's published ports do not answer.

.. expectation:: A service's old published port does not answer after a change
    :id: SWARM-PUB-PORT-CHANGE-OLD
    :observed_from: client, node
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-PUB-FOLLOWS-SPEC
    :within: SWARM-CEIL-ALLOCATION, SWARM-CEIL-REASSIGNMENT

    *Applies to:* A service that publishes a port, and no other service or container that publishes
    that port

    After :api-op:`POST /services/{id}/update` changes that published port to another port, once no
    container of a task from before the update is running, the old port does not answer. This holds
    once the :need:`[[title|lower]] ceiling ([[seconds]] s) <SWARM-CEIL-ALLOCATION>` has passed from
    the :api-event:`die` event of the last container of a task from before the update, and the
    :need:`[[title|lower]] ceiling ([[seconds]] s) <SWARM-CEIL-REASSIGNMENT>` has passed from when
    the update is complete.

.. expectation:: A port that an update removes from a service does not answer
    :id: SWARM-PUB-PORT-REMOVED
    :observed_from: client, node
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-PUB-FOLLOWS-SPEC
    :within: SWARM-CEIL-ALLOCATION, SWARM-CEIL-REASSIGNMENT

    *Applies to:* A service that publishes two ports, and no other service or container that
    publishes either of them

    After :api-op:`POST /services/{id}/update` removes one of the two ports from the service's
    :api-field:`EndpointSpec.Ports`, once no container of a task from before the update is running,
    the removed port does not answer. This holds once the :need:`[[title|lower]] ceiling
    ([[seconds]] s) <SWARM-CEIL-ALLOCATION>` has passed from the :api-event:`die` event of the last
    container of a task from before the update, and the :need:`[[title|lower]] ceiling ([[seconds]]
    s) <SWARM-CEIL-REASSIGNMENT>` has passed from when the update is complete.

.. expectation:: A client reaches a service's tasks on its updated published port
    :id: SWARM-PUB-PORT-CHANGE
    :observed_from: client, node
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-PUB-FOLLOWS-SPEC
    :needs: BR-HOST-NO-FORWARD-DROP
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A service that publishes a port

    :api-op:`POST /services/{id}/update` changes the service's published port. A client reaches a
    task of the service on the new port at the address of a node that runs a task of the service.
    This holds once a task from after the update is running.

.. expectation:: With userland-proxy, a published port is reachable over IPv6 off-node
    :id: SWARM-PUB-V6-USERLAND-PROXY
    :observed_from: client
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-SWARM-PUB-IPV6-PROXY
    :needs: SWARM-HOST-IPV6
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A daemon with :daemon-json:`userland-proxy` true

    A published port is reachable over IPv6 at the node's IPv6 addresses, from off-node. This holds
    for TCP and UDP.

.. expectation:: With firewalld, a docker-proxy port does not answer at a node's IPv6 address
    :id: SWARM-PUB-FIREWALLD-V6-PROXY
    :observed_from: client
    :depends_on: os-linux
    :platforms: linux
    :needs: BR-HOST-NO-FORWARD-DROP
    :within: SWARM-CEIL-REASSIGNMENT
    :narrows: BR-PUB-HOST-PORT-ASSIGNED, SWARM-PUB-TARGET-PORT, SWARM-PUB-PROTOCOLS, SWARM-PUB-PORT-CHANGE, SWARM-PUB-V6-USERLAND-PROXY, SWARM-HOSTPUB-TASK-NODES-ONLY, SWARM-HOSTPUB-LOCAL-TASK, SWARM-RESTART-GRACEFUL-PUBLISHED-PORTS, SWARM-RESTART-KILLED-PUBLISHED-PORTS, SWARM-CONVERGE-REJOIN-PUBLISHED-PORTS, SWARM-CONVERGE-LEAVE-REMAINING-TASKS, SWARM-SCALE-PORT-PROTOCOLS, SWARM-SCALE-PORT-PROTOCOLS-REMOVAL, BR-FW-FIREWALLD-LATE-START-PORT, BR-PUB-HOST-PORT-ASSIGNED-CLI, SWARM-INGRESS-LABEL-PUBLISH
    :concerns: BR-FW-FIREWALLD-RELOAD-PORT
    :violates: REQ-BR-PUBLISH-DEFAULTS, REQ-SWARM-PUB-IPV6-PROXY

    *Applies to:* A daemon with :daemon-json:`userland-proxy` true and firewalld running, with
    firewalld's default zone, with the configuration that :need:`NET-HOST-FIREWALL-PASSES-TRAFFIC`
    gives firewalld, in effect for the interface that the connection arrives on, and a port
    published without a host address on an endpoint without an IPv6 address, such as a port that a
    Swarm task publishes in :api-value:`mode=host`

    A connection to the port at an IPv6 address of the node does not answer, before and after a
    firewalld reload. The port answers at the node's IPv4 addresses.

.. expectation:: With userland-proxy, a published port is reachable over IPv6 on-node
    :id: SWARM-PUB-V6-USERLAND-PROXY-LOCAL
    :observed_from: node
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-SWARM-PUB-IPV6-PROXY
    :needs: SWARM-HOST-IPV6
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A daemon with :daemon-json:`userland-proxy` true

    A published port is reachable over IPv6 from the node itself, at the node's own IPv6 address and
    at ``::1``. This holds for TCP and UDP.

.. expectation:: An accepted IPv6 connection to a published port reaches a task
    :id: SWARM-PUB-V6-DELIVERY
    :observed_from: client, node
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-PUB-IPV6-PROXY
    :needs: SWARM-HOST-IPV6
    :within: SWARM-CEIL-REASSIGNMENT

    When a node accepts an IPv6 TCP connection to a published port, a task of the service receives
    the connection.

.. expectation:: Without userland-proxy, a client cannot reach a task over IPv6
    :id: SWARM-PUB-V6-NO-USERLAND-PROXY
    :observed_from: client, node
    :depends_on: os-linux
    :platforms: linux
    :pins: SWARM-OVL-V6-ADDRESSING
    :needs: BR-HOST-BR-NETFILTER-NO-PROXY
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A daemon with :daemon-json:`userland-proxy` false, and a port published in ingress
    mode, or in :api-value:`mode=host` by a task whose gateway network is :network:`docker_gwbridge`

    A client cannot reach a task of the service through that port over IPv6, at any IPv6 address of
    the node. This holds for TCP and UDP.

These IPv6 expectations do not mean that the service mesh supports IPv6. The service mesh is
IPv4-only (:need:`SWARM-OVL-V6-ADDRESSING`). The ingress network and the gateway network do not give
a task an IPv6 address. The proxy accepts traffic on ``[::]`` and forwards it to the task over IPv4,
as it does for any IPv4-only container. Thus the task sees the connection come from the proxy. For
this reason, the real-source expectation for :api-value:`mode=host`,
:need:`SWARM-HOSTPUB-CLIENT-SOURCE`, does not hold for IPv6 clients.

For a port published in ingress mode, or in :api-value:`mode=host` by a task whose gateway network
is :network:`docker_gwbridge`, the userland proxy is necessary for the port to be reachable over
IPv6. Without the proxy, such a port is not reachable over IPv6
(:need:`SWARM-PUB-V6-NO-USERLAND-PROXY`).

.. _swarm-traffic-hairpin:

Hairpin
=======

The six paths below are independent and can fail separately. The first five hold for both publish
modes. For :api-value:`mode=host`, they hold on the nodes that run a task. In this subsection, the
node's addresses are its non-loopback IPv4 addresses, and each expectation holds at each of them, as
:ref:`README.md <readme-observed-from>` says for the node's address. In this subsection, each task
or container that connects to the node's address must be attached to at least one network that is
not ``internal``.

.. expectation:: A published port is reachable from the node via 127.0.0.1
    :id: SWARM-HAIRPIN-LOOPBACK
    :observed_from: node
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-SWARM-PUB-HAIRPIN
    :within: SWARM-CEIL-REASSIGNMENT

    A published port is reachable from the node itself via ``127.0.0.1``.

.. expectation:: A published port is reachable from the node via the node's address
    :id: SWARM-HAIRPIN-NODE-ADDRESS
    :observed_from: node
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-SWARM-PUB-HAIRPIN
    :within: SWARM-CEIL-REASSIGNMENT

    A published port is reachable from the node itself via the node's address.

.. expectation:: A container on a local bridge reaches a published port at the node
    :id: SWARM-HAIRPIN-BRIDGE-CTR
    :observed_from: ctr
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-SWARM-PUB-HAIRPIN
    :needs: BR-HOST-NO-FORWARD-DROP
    :within: SWARM-CEIL-REASSIGNMENT

    A published port is reachable from a container on a local bridge network, via the node's
    address.

.. expectation:: A container on the service's overlay reaches the port at its node
    :id: SWARM-HAIRPIN-SERVICE-NETWORK
    :observed_from: task, ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-PUB-HAIRPIN
    :needs: BR-HOST-NO-FORWARD-DROP
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A service that publishes a port, and a task or container attached to one of the
    service's overlay networks

    The port is reachable via the node's address from the task or container on the same node.

.. expectation:: A container on other overlays reaches the port at its node
    :id: SWARM-HAIRPIN-OTHER-NETWORK
    :observed_from: task, ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-PUB-HAIRPIN
    :needs: BR-HOST-NO-FORWARD-DROP
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A service that publishes a port, and a task or container attached only to overlay
    networks that the service is not on

    The port is reachable via the node's address from the task or container on the same node.

.. expectation:: A task reaches its own mode=host published port at its node's address
    :id: SWARM-HAIRPIN-HOST-MODE-SELF
    :observed_from: task
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-PUB-HAIRPIN
    :needs: BR-HOST-NO-FORWARD-DROP
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A service that publishes a port in :api-value:`mode=host`

    A task of the service reaches its own published port via its node's address.

.. expectation:: Hairpin reachability holds with userland-proxy true and false
    :id: SWARM-HAIRPIN-USERLAND-PROXY
    :observed_from: node, ctr, task
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-SWARM-PUB-HAIRPIN
    :needs: BR-HOST-NO-FORWARD-DROP
    :within: SWARM-CEIL-REASSIGNMENT

    Each other expectation of this subsection holds with :daemon-json:`userland-proxy` true and with
    :daemon-json:`userland-proxy` false.

On Linux nodes, the two overlay expectations and the :api-value:`mode=host` self-hairpin expectation
are the paths whose traffic leaves a bridge and then enters the same bridge again. An overlay
container reaches the node's address through :network:`docker_gwbridge`. The connection then reaches
the ingress load balancer, or the task that serves the port, through :network:`docker_gwbridge`
again. Only those paths are subject to the bridge's inter-container communication setting. A gateway
network that the daemon creates has inter-container communication disabled
(:need:`SWARM-GWNET-NO-ICC`).

The proxy can hide these paths (:need:`SWARM-HAIRPIN-USERLAND-PROXY`). For a :api-value:`mode=host`
port, with the proxy, the connection ends in the host's network namespace and does not enter the
bridge again. Without the proxy, the traffic is DNATed back across the bridge. For a
:api-value:`mode=ingress` port, the node DNATs IPv4 traffic that arrives on
:network:`docker_gwbridge` back across the bridge with either :daemon-json:`userland-proxy` value.
With :daemon-json:`userland-proxy` true, the proxy then handles only IPv6 clients and the node's
connections to its loopback addresses.

.. _swarm-traffic-api-value-mode-ingress:

:api-value:`mode=ingress`
=========================

The routing mesh publishes an ingress-mode port on every node, whichever nodes run the service's
tasks.

.. expectation:: A published port is reachable on every node, with or without a task
    :id: SWARM-INGPUB-EVERY-NODE
    :observed_from: client
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-ROUTING-MESH
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-IPTABLES-LB-EXTENSIONS, SWARM-HOST-IPTABLES-LEGACY-MANGLE-KERNEL, BR-HOST-NO-FORWARD-DROP
    :within: SWARM-CEIL-REASSIGNMENT

    A published port is reachable on every node, including nodes that do not run a task of the
    service.

.. expectation:: A service that publishes one port number for TCP and UDP answers on both
    :id: SWARM-PUB-TCP-UDP-ONE-SERVICE
    :observed_from: client
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-PORT-PROTOCOLS
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-IPTABLES-LB-EXTENSIONS, SWARM-HOST-IPTABLES-LEGACY-MANGLE-KERNEL, BR-HOST-NO-FORWARD-DROP
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A service that publishes the same port number in ingress mode for TCP and for UDP

    A client reaches a task of the service at any node's address on that port number with TCP, and
    with UDP.

.. expectation:: An ingress published port is reachable after a firewalld reload
    :id: SWARM-INGPUB-FIREWALLD-RELOAD
    :observed_from: client
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-SWARM-ROUTING-MESH
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-IPTABLES-LB-EXTENSIONS, SWARM-HOST-IPTABLES-LEGACY-MANGLE-KERNEL, BR-HOST-NO-FORWARD-DROP
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A node with firewalld running, and a service that publishes a port in ingress mode

    After a firewalld reload on the node, the published port is reachable on the node.

.. expectation:: With firewalld, an ingress port does not answer at a node's IPv6 address
    :id: SWARM-PUB-FIREWALLD-V6-INGRESS
    :observed_from: client
    :depends_on: os-linux
    :platforms: linux
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-IPTABLES-LB-EXTENSIONS, BR-HOST-NO-FORWARD-DROP, SWARM-HOST-IPTABLES-LEGACY-MANGLE-KERNEL
    :within: SWARM-CEIL-REASSIGNMENT
    :narrows: SWARM-PUB-V6-USERLAND-PROXY, SWARM-INGPUB-EVERY-NODE, SWARM-PUB-TCP-UDP-ONE-SERVICE, SWARM-INGPUB-FIREWALLD-RELOAD, SWARM-PORTALLOC-REPORTED-REACHABLE, SWARM-PORTALLOC-PORT-IN-USE-OLD-TASKS, SWARM-INGRESS-CUSTOM-SUBNET-REACHABLE, SWARM-RESTART-INGRESS-FROM-OTHER-NODES, SWARM-CONVERGE-REJOIN-INGRESS-BACKENDS, SWARM-FAILREP-INGRESS-PORT-RECOVERY
    :violates: REQ-SWARM-PUB-IPV6-PROXY

    *Applies to:* A daemon with :daemon-json:`userland-proxy` true and firewalld running, with
    firewalld's default zone, with the configuration that :need:`NET-HOST-FIREWALL-PASSES-TRAFFIC`
    gives firewalld, in effect for the interface that the connection arrives on, and a port that a
    service publishes in ingress mode

    A connection to the port at an IPv6 address of the node does not answer, before and after a
    firewalld reload. The port answers at the node's IPv4 addresses.

.. expectation:: All packets of an ingress flow reach the same task
    :id: SWARM-INGPUB-FLOW-SAME-TASK
    :observed_from: client
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-ROUTING-MESH
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-IPTABLES-LB-EXTENSIONS, SWARM-HOST-IPTABLES-LEGACY-MANGLE-KERNEL, BR-HOST-NO-FORWARD-DROP
    :within: SWARM-CEIL-REASSIGNMENT

    All packets of a flow reach the same task.

.. expectation:: A task does not see an ingress client's real source address
    :id: SWARM-INGPUB-CLIENT-SOURCE-HIDDEN
    :observed_from: task
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-IPTABLES-LB-EXTENSIONS, SWARM-HOST-IPTABLES-LEGACY-MANGLE-KERNEL, BR-HOST-NO-FORWARD-DROP
    :within: SWARM-CEIL-REASSIGNMENT
    :rationale: RAT-SWARM-INGRESS-SOURCE-REPLACED

    The task does not observe the client's real source address.

.. expectation:: Tasks do not reach each other over the ingress network
    :id: SWARM-INGPUB-NO-EAST-WEST
    :observed_from: task → task
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-NETWORK-ISOLATION
    :within: SWARM-CEIL-REASSIGNMENT

    A task is not reachable from another task via its ingress-network address, or via its service's
    VIP on the ingress network, on any port. This holds from when both tasks run, without a ceiling.

.. expectation:: An ingress task reaches a Windows node's task at its ingress address
    :id: SWARM-INGPUB-WINDOWS-INGRESS-ISOLATION
    :observed_from: task
    :depends_on: none
    :platforms: windows
    :within: SWARM-CEIL-REASSIGNMENT
    :narrows: SWARM-INGPUB-NO-EAST-WEST
    :violates: REQ-SWARM-NETWORK-ISOLATION

    A task attached to the ingress network reaches a task on a Windows node at the ingress-network
    address of the task on the Windows node.

.. expectation:: A task's default route is not on the ingress network
    :id: SWARM-INGPUB-NO-DEFAULT-ROUTE
    :observed_from: task
    :depends_on: os-linux
    :platforms: linux
    :within: SWARM-CEIL-REASSIGNMENT
    :rationale: RAT-SWARM-INGRESS-NO-EGRESS-ROUTE

    A task's default route is not on the ingress network.

.. expectation:: On Windows, a task's ingress link routes traffic outside the Swarm
    :id: SWARM-INGPUB-WINDOWS-INGRESS-EGRESS
    :observed_from: task
    :depends_on: none
    :platforms: windows
    :within: SWARM-CEIL-REASSIGNMENT
    :concerns: SWARM-INGPUB-NO-DEFAULT-ROUTE
    :violates: PRIN-LEAST-ACCESS

    On a Windows node, a task that has an ingress-network link has a route through that link to
    addresses outside the Swarm, and its traffic over that route reaches an address outside the
    Swarm.

.. expectation:: A task fails with an error when its node cannot add its ingress rules
    :id: SWARM-INGPUB-INGRESS-RULES-FAILED
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-TASK-ERRORS
    :pins: SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-IPTABLES-LB-EXTENSIONS

    *Applies to:* A Linux node that cannot add the ingress rules of a task's network namespace, and
    a service with a task on that node that publishes a port in ingress mode

    The task does not reach the ``running`` state. :api-op:`GET /tasks` reports an error for the
    task.

.. expectation:: The task's error says it failed to add ingress redirect rules
    :id: SWARM-INGPUB-INGRESS-RULES-FAILED-MSG
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :exact_form_of: SWARM-INGPUB-INGRESS-RULES-FAILED
    :pins: SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-IPTABLES-LB-EXTENSIONS

    *Applies to:* A Linux node that cannot add the ingress rules of a task's network namespace, and
    a service with a task on that node that publishes a port in ingress mode

    The error that :api-op:`GET /tasks` reports for the task contains :api-error:`failed to add
    ingress redirect rules`.

.. expectation:: A dnsrr service that publishes an ingress-mode port is refused
    :id: SWARM-INGPUB-DNSRR-REFUSED
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-DNSRR-NO-INGRESS-PORTS, REQ-NET-API-ERROR-STATUS

    :api-op:`POST /services/create` or :api-op:`POST /services/{id}/update` with a spec in ``dnsrr``
    endpoint mode that publishes a port in ingress mode responds with a 4xx status. This holds
    whether or not the port is chosen. The service is not created or not changed.

.. expectation:: A dnsrr service that publishes an ingress-mode port responds 400
    :id: SWARM-INGPUB-DNSRR-REFUSED-HTTP400
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :exact_form_of: SWARM-INGPUB-DNSRR-REFUSED

    :api-op:`POST /services/create` or :api-op:`POST /services/{id}/update` with a spec in ``dnsrr``
    endpoint mode that publishes a port in ingress mode responds 400.

.. expectation:: docker service create of a dnsrr service with an ingress port fails
    :id: SWARM-INGPUB-DNSRR-REFUSED-CLI
    :observed_from: api:mgr
    :depends_on: cli
    :platforms: linux
    :refines: REQ-SWARM-DNSRR-NO-INGRESS-PORTS
    :cli_for: SWARM-INGPUB-DNSRR-REFUSED

    :cli:`docker service create` with :cli-flag:`--endpoint-mode dnsrr` and a port published in
    ingress mode exits non-zero and prints the daemon's error. The command does not create the
    service.

.. _swarm-traffic-port-allocation:

Port allocation
---------------

An ingress-mode port is auto-assigned when the request that creates or updates the service does not
set its :api-field:`PublishedPort`. :api-op:`GET /services/{id}` then reports a published port for
it in :api-field:`Endpoint.Ports`. The auto-assignment range is the set of published ports that an
auto-assigned port can get. A port of either publish mode is chosen when the request sets its
:api-field:`PublishedPort`. A service's ports are allocated after the request responds. Where an
expectation in this subsection says that :api-op:`GET /services/{id}` then reports a port, it
reports it within the :need:`[[title|lower]] ceiling ([[seconds]] s) <SWARM-CEIL-ALLOCATION>` after
the request responds.

.. expectation:: An auto-assigned port is one no other service publishes in ingress mode
    :id: SWARM-PORTALLOC-AUTO
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-PORT-ALLOCATION
    :within: SWARM-CEIL-ALLOCATION

    :api-op:`POST /services/create` for a service with an auto-assigned port responds 201.
    :api-op:`GET /services/{id}` then reports a published port for it that no other service
    publishes in ingress mode.

.. expectation:: An auto-assigned port is from 30000 to 32767
    :id: SWARM-PORTALLOC-AUTO-RANGE
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-PORT-ALLOCATION
    :within: SWARM-CEIL-ALLOCATION

    :api-op:`POST /services/create` for a service with an auto-assigned port responds 201.
    :api-op:`GET /services/{id}` then reports a published port from 30000 to 32767 for it.

.. expectation:: A chosen ingress-mode port is accepted and reported in Endpoint.Ports
    :id: SWARM-PORTALLOC-CHOSEN
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-PORT-ALLOCATION
    :within: SWARM-CEIL-ALLOCATION

    :api-op:`POST /services/create` for a service with a chosen ingress-mode port responds 201.
    :api-op:`GET /services/{id}` then reports that port in :api-field:`Endpoint.Ports`.

.. expectation:: Any chosen ingress-mode port from 1 to 65535 is accepted and reported
    :id: SWARM-PORTALLOC-CHOSEN-FULL-RANGE
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :exact_form_of: SWARM-PORTALLOC-CHOSEN
    :within: SWARM-CEIL-ALLOCATION

    :api-op:`POST /services/create` for a service with a chosen ingress-mode port from 1 to 65535
    responds 201. This includes a port in the auto-assignment range. :api-op:`GET /services/{id}`
    then reports that port in :api-field:`Endpoint.Ports`.

.. expectation:: A chosen ingress-mode port above 65535 is accepted but not reported
    :id: SWARM-PORTALLOC-CHOSEN-OUT-OF-RANGE
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :pins: SWARM-PORTALLOC-PORT-ABOVE-65535
    :within: SWARM-CEIL-ALLOCATION

    :api-op:`POST /services/create` for a service with a chosen ingress-mode port above 65535
    responds 201. :api-op:`GET /services/{id}` then does not report the port in
    :api-field:`Endpoint.Ports`.

.. expectation:: A published or target port above 65535 is accepted and never published
    :id: SWARM-PORTALLOC-PORT-ABOVE-65535
    :observed_from: api:mgr, client, node, ctr, task
    :depends_on: none
    :platforms: linux, windows
    :within: SWARM-CEIL-ALLOCATION, SWARM-CEIL-REASSIGNMENT
    :concerns: SWARM-PORTALLOC-CHOSEN-OUT-OF-RANGE, SWARM-HOSTPUB-TARGET-OUT-OF-RANGE
    :violates: PRIN-FAIL-FAST, PRIN-FAILS-CLOSED, REQ-SWARM-TASK-ERRORS

    :api-op:`POST /services/create` with an ingress-mode :api-field:`PublishedPort` above 65535
    responds 201. :api-op:`GET /services/{id}` then does not report the port in
    :api-field:`Endpoint.Ports`, and the service's tasks stay in the ``new`` state that :api-op:`GET
    /tasks` reports. With a :api-value:`mode=host` :api-field:`PublishedPort` above 65535, the
    request also responds 201, and each of the service's tasks is rejected with :api-error:`invalid
    port specification`. In :api-value:`mode=host`, a :api-field:`TargetPort` above 65535 is also
    accepted. The service's tasks run, and :api-op:`GET /tasks/{id}` reports no port in
    :api-field:`Status.PortStatus` for them. When such a port also sets :api-field:`PublishedPort`,
    a connection to that published port at the address of a node that runs a task of the service
    does not reach the task.

.. expectation:: Auto-assignment skips a port chosen in the auto-assignment range
    :id: SWARM-PORTALLOC-AUTO-SKIPS-CHOSEN
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-PORT-ALLOCATION
    :within: SWARM-CEIL-ALLOCATION

    *Applies to:* A Swarm with a service that publishes a chosen ingress-mode port in the
    auto-assignment range, such as 30000 in a Swarm in which no service has had an auto-assigned
    port, and another service, created after it, with auto-assigned ports

    :api-op:`GET /services/{id}` for the other service does not report that port for any of its
    auto-assigned ports.

.. expectation:: Endpoint.Ports reports every ingress-mode port with its published port
    :id: SWARM-PORTALLOC-REPORTED
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-PORT-ALLOCATION
    :within: SWARM-CEIL-ALLOCATION

    :api-op:`GET /services/{id}` reports each ingress-mode port of a service in
    :api-field:`Endpoint.Ports`, with its published port. This holds for auto-assigned and chosen
    ports.

.. expectation:: A client reaches a task at any node on the reported published port
    :id: SWARM-PORTALLOC-REPORTED-REACHABLE
    :observed_from: api:mgr, client
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-PUB-TARGET
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-IPTABLES-LB-EXTENSIONS, SWARM-HOST-IPTABLES-LEGACY-MANGLE-KERNEL, BR-HOST-NO-FORWARD-DROP
    :within: SWARM-CEIL-ALLOCATION, SWARM-CEIL-REASSIGNMENT

    A client reaches a task of a service at any node's address on the published port that
    :api-op:`GET /services/{id}` reports for an ingress-mode port. This holds for auto-assigned and
    chosen ports.

.. expectation:: An update that keeps an auto-assigned port keeps its published port
    :id: SWARM-PORTALLOC-AUTO-KEPT
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-PORT-ALLOCATION, REQ-SWARM-PORT-AUTO-KEPT
    :within: SWARM-CEIL-ALLOCATION

    *Applies to:* A Swarm with a service that has an auto-assigned port

    :api-op:`POST /services/{id}/update` that keeps the number of ports, and keeps that port in
    ingress mode, with the same name, protocol and target port and without
    :api-field:`PublishedPort`, responds 200. :api-op:`GET /services/{id}` then reports the same
    published port for it.

.. expectation:: Adding or removing another port reassigns an auto-assigned port
    :id: SWARM-PORTALLOC-AUTO-REASSIGNED
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :pins: SWARM-PORTALLOC-AUTO-UPDATE-REASSIGNED
    :within: SWARM-CEIL-ALLOCATION

    *Applies to:* A Swarm with a service that has an auto-assigned port

    After :api-op:`POST /services/{id}/update` adds or removes another port of the service, in
    either publish mode, :api-op:`GET /services/{id}` reports a different published port for the
    auto-assigned port.

.. expectation:: Adding or removing a port reassigns a service's auto-assigned ports
    :id: SWARM-PORTALLOC-AUTO-UPDATE-REASSIGNED
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :within: SWARM-CEIL-ALLOCATION
    :concerns: SWARM-PORTALLOC-AUTO-KEPT, SWARM-PORTALLOC-AUTO-REASSIGNED
    :violates: REQ-SWARM-PORT-AUTO-KEPT

    After :api-op:`POST /services/{id}/update` adds or removes a published port of a service, in
    either publish mode, :api-op:`GET /services/{id}` reports a different published port for each of
    the service's other auto-assigned ports.

.. expectation:: Another service cannot choose a port already published in ingress mode
    :id: SWARM-PORTALLOC-CONFLICT
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-PORT-NO-CONFLICT, REQ-NET-API-ERROR-STATUS

    *Applies to:* A Swarm with a service that publishes a port in ingress mode

    :api-op:`POST /services/create` or :api-op:`POST /services/{id}/update` for another service that
    chooses the same port in ingress mode responds with a 4xx status. The other service is not
    created or not changed.

.. expectation:: A conflicting ingress-mode port responds 400, already in use by service
    :id: SWARM-PORTALLOC-CONFLICT-HTTP400
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :exact_form_of: SWARM-PORTALLOC-CONFLICT

    *Applies to:* A Swarm with a service that publishes a port in ingress mode

    :api-op:`POST /services/create` or :api-op:`POST /services/{id}/update` for another service that
    chooses the same port in ingress mode responds 400, with a message that contains :api-error:`is
    already in use by service`.

.. expectation:: docker service create or update with a used ingress-mode port fails
    :id: SWARM-PORTALLOC-CONFLICT-CLI
    :observed_from: api:mgr
    :depends_on: cli
    :platforms: linux
    :refines: REQ-SWARM-PORT-NO-CONFLICT
    :cli_for: SWARM-PORTALLOC-CONFLICT

    *Applies to:* A Swarm with a service that publishes a port in ingress mode

    :cli:`docker service create` or :cli:`docker service update --publish-add` that publishes the
    same port in ingress mode for another service exits non-zero and prints the daemon's error. The
    command does not create or change the other service.

.. expectation:: Another service publishes a TCP port's number for UDP
    :id: SWARM-PORTALLOC-OTHER-PROTOCOL
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-PORT-ALLOCATION
    :within: SWARM-CEIL-ALLOCATION

    *Applies to:* A Swarm with a service that publishes a TCP port

    :api-op:`POST /services/create` for another service that chooses the same port number for UDP
    responds 201. :api-op:`GET /services/{id}` then reports each service's port.

.. expectation:: A port published in one mode cannot be chosen in the other mode
    :id: SWARM-PORTALLOC-MODE-CONFLICT
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-PORT-NO-CONFLICT, REQ-NET-API-ERROR-STATUS

    *Applies to:* A Swarm with a service that publishes a port in one publish mode

    :api-op:`POST /services/create` or :api-op:`POST /services/{id}/update` for another service that
    chooses the same port in the other mode responds with a 4xx status. This holds for either mode
    of the first service. The other service is not created or not changed.

.. expectation:: A port conflict across publish modes responds 400, already in use
    :id: SWARM-PORTALLOC-MODE-CONFLICT-HTTP400
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :exact_form_of: SWARM-PORTALLOC-MODE-CONFLICT

    *Applies to:* A Swarm with a service that publishes a port in one publish mode

    :api-op:`POST /services/create` or :api-op:`POST /services/{id}/update` for another service that
    chooses the same port in the other mode responds 400, with a message that contains
    :api-error:`is already in use by service`. This holds for either mode of the first service.

.. expectation:: docker service create or update fails on a port used in the other mode
    :id: SWARM-PORTALLOC-MODE-CONFLICT-CLI
    :observed_from: api:mgr
    :depends_on: cli
    :platforms: linux
    :refines: REQ-SWARM-PORT-NO-CONFLICT
    :cli_for: SWARM-PORTALLOC-MODE-CONFLICT

    *Applies to:* A Swarm with a service that publishes a port in one publish mode

    :cli:`docker service create` or :cli:`docker service update --publish-add` that publishes the
    same port in the other mode for another service exits non-zero and prints the daemon's error.
    The command does not create or change the other service.

.. expectation:: Two creates with one published port, checked concurrently, both respond 201
    :id: SWARM-PORTALLOC-CONFLICT-RACE
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :narrows: SWARM-PORTALLOC-CONFLICT, SWARM-PORTALLOC-CONFLICT-CLI, SWARM-PORTALLOC-CONFLICT-HTTP400, SWARM-PORTALLOC-MODE-CONFLICT, SWARM-PORTALLOC-MODE-CONFLICT-HTTP400, SWARM-PORTALLOC-MODE-CONFLICT-CLI
    :violates: REQ-SWARM-PORT-NO-CONFLICT

    *Applies to:* A leader manager that has checked a :api-op:`POST /services/create` request, which
    chooses a published port and protocol in either publish mode, for port conflicts, and has not
    yet stored the request's service

    The leader checks a second :api-op:`POST /services/create` request that chooses the same
    published port and protocol, with at least one of the two requests choosing it in ingress mode,
    for port conflicts before it stores the first request's service. Both requests respond 201.
    :api-op:`GET /services` then lists both services, each with that published port in
    :api-field:`Spec.EndpointSpec.Ports`.

.. expectation:: Auto-assignment gives an ingress port that a mode=host task holds
    :id: SWARM-PUB-AUTO-ASSIGNED-PORT-HOST-MODE
    :observed_from: api:mgr, client
    :depends_on: none
    :platforms: linux
    :within: SWARM-CEIL-ALLOCATION, SWARM-CEIL-REASSIGNMENT
    :narrows: SWARM-INGPUB-EVERY-NODE, SWARM-PORTALLOC-REPORTED-REACHABLE, SWARM-PUB-TARGET-PORT, SWARM-PUB-PROTOCOLS, SWARM-PUB-REPLIES, SWARM-PUB-UDP-NO-LISTENER, SWARM-PUB-PORT-CHANGE, SWARM-INGRESS-CUSTOM-SUBNET-REACHABLE, SWARM-INGRESS-LABEL-PUBLISH, SWARM-HAIRPIN-LOOPBACK, SWARM-HAIRPIN-NODE-ADDRESS, SWARM-HAIRPIN-BRIDGE-CTR, SWARM-HAIRPIN-SERVICE-NETWORK, SWARM-HAIRPIN-OTHER-NETWORK, SWARM-PUB-TCP-UDP-ONE-SERVICE, SWARM-INGPUB-FIREWALLD-RELOAD, SWARM-PUB-V6-USERLAND-PROXY, SWARM-PUB-V6-USERLAND-PROXY-LOCAL, SWARM-HAIRPIN-USERLAND-PROXY, SWARM-RESTART-GRACEFUL-PUBLISHED-PORTS, SWARM-RESTART-KILLED-PUBLISHED-PORTS, SWARM-RESTART-INGRESS-FROM-OTHER-NODES, SWARM-CONVERGE-REJOIN-PUBLISHED-PORTS, SWARM-CONVERGE-REJOIN-INGRESS-BACKENDS, SWARM-CONVERGE-LEAVE-REMAINING-TASKS, SWARM-SCALE-PORT-PROTOCOLS, SWARM-SCALE-PORT-PROTOCOLS-REMOVAL
    :concerns: SWARM-PORTALLOC-MODE-CONFLICT, SWARM-PORTALLOC-AUTO
    :violates: REQ-SWARM-PORT-ALLOCATION, REQ-SWARM-PORT-NO-CONFLICT, REQ-SWARM-ROUTING-MESH

    When every port from 30000 to 32767 but one is published by other services, and that one in
    :api-value:`mode=host`, :api-op:`POST /services/create` for a service with an ingress-mode port
    without :api-field:`PublishedPort` responds 201, and :api-op:`GET /services/{id}` reports that
    port as its published port. On a Linux node where the :api-value:`mode=host` task runs, either
    the routing mesh does not serve the service's ports or the :api-value:`mode=host` task fails to
    start, whichever binds the port second.

.. expectation:: Creating a container on a service's ingress port gives no warning
    :id: SWARM-PORTALLOC-CTR-CREATE
    :observed_from: api:mgr, api:wkr
    :depends_on: none
    :platforms: linux
    :concerns: SWARM-PORTALLOC-CTR-START
    :violates: PRIN-WARN-AHEAD

    *Applies to:* A Swarm with a service that publishes a port in ingress mode and has a running
    task

    On every node, :api-op:`POST /containers/create` for a container that publishes the same port
    responds 201 without a warning about the port.

.. expectation:: A container that publishes a service's ingress port does not start
    :id: SWARM-PORTALLOC-CTR-START
    :observed_from: api:mgr, api:wkr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-PORT-NO-CONFLICT
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A Swarm with a service that publishes a port in ingress mode, and a created or
    stopped container on any of its nodes that publishes the same port

    :api-op:`POST /containers/{id}/start` for the container responds with a 4xx or 5xx status. This
    holds once the :need:`[[title|lower]] ceiling ([[seconds]] s) <SWARM-CEIL-REASSIGNMENT>` has
    passed from when a task of the service first runs, or, for a task with a healthcheck, first
    reports healthy.

.. expectation:: Starting a container on a service's ingress port responds 500
    :id: SWARM-PORTALLOC-CTR-START-HTTP500
    :observed_from: api:mgr, api:wkr
    :depends_on: none
    :platforms: linux
    :exact_form_of: SWARM-PORTALLOC-CTR-START
    :pins: SWARM-CLUSTER-HTTP500-REFUSED
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A Swarm with a service that publishes a port in ingress mode, and a created or
    stopped container on any of its nodes that publishes the same port

    :api-op:`POST /containers/{id}/start` for the container responds 500, with a message that
    contains :api-error:`port is already allocated`. This holds once the :need:`[[title|lower]]
    ceiling ([[seconds]] s) <SWARM-CEIL-REASSIGNMENT>` has passed from when a task of the service
    first runs, or, for a task with a healthcheck, first reports healthy.

.. expectation:: docker run -p with a service's ingress port fails on every node
    :id: SWARM-PORTALLOC-CTR-START-CLI
    :observed_from: api:mgr, api:wkr
    :depends_on: cli
    :platforms: linux
    :refines: REQ-SWARM-PORT-NO-CONFLICT
    :cli_for: SWARM-PORTALLOC-CTR-START
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A Swarm with a service that publishes a port in ingress mode

    On every node, :cli:`docker run -d -p` with the same port exits non-zero and prints the daemon's
    error. This holds once the :need:`[[title|lower]] ceiling ([[seconds]] s)
    <SWARM-CEIL-REASSIGNMENT>` has passed from when a task of the service first runs, or, for a task
    with a healthcheck, first reports healthy.

.. expectation:: A container starts on a service's ingress port while no task of it runs
    :id: SWARM-PORTALLOC-CTR-START-NO-TASK
    :observed_from: api:mgr, api:wkr
    :depends_on: none
    :platforms: linux
    :within: SWARM-CEIL-ALLOCATION, SWARM-CEIL-REASSIGNMENT
    :narrows: SWARM-PORTALLOC-CTR-START, SWARM-PORTALLOC-CTR-START-HTTP500, SWARM-PORTALLOC-CTR-START-CLI, SWARM-CLUSTER-HTTP500-REFUSED
    :violates: REQ-SWARM-PORT-NO-CONFLICT

    *Applies to:* A Swarm with a service that publishes a port in ingress mode, and a created or
    stopped container on a Linux node of the Swarm that publishes the same port

    For a service none of whose tasks has run, such as one created with no replicas, :api-op:`POST
    /containers/{id}/start` for the container responds 204, without a ceiling. For a service that
    has had a running task, such as one scaled to 0, the start responds 204 once no container of the
    service is running.

.. expectation:: A process or container keeps serving a port a service then publishes
    :id: SWARM-PORTALLOC-PORT-IN-USE
    :observed_from: client
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-SWARM-PORT-NO-CONFLICT
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A Linux node where a port is in use by a process or container

    The process or container that uses the port keeps serving it after a service is created or
    updated to publish the port in ingress mode.

.. expectation:: On Windows nodes, an ingress port's host port is not reserved
    :id: SWARM-PORTALLOC-WINDOWS-INGRESS-PORT-RESERVATION
    :observed_from: api:wkr, client
    :depends_on: none
    :platforms: windows
    :within: SWARM-CEIL-REASSIGNMENT
    :concerns: SWARM-PORTALLOC-PORT-IN-USE, SWARM-PORTALLOC-CTR-START, SWARM-PORTALLOC-CTR-START-CLI, SWARM-PORTALLOC-CTR-START-HTTP500
    :violates: REQ-SWARM-PORT-NO-CONFLICT

    On a Windows node, :api-op:`POST /containers/{id}/start` for a container that publishes a host
    port that a service publishes in ingress mode responds 204. A port is also published in ingress
    mode on a Windows node where it is already in use.

.. expectation:: A service whose ingress ports a node failed to publish is unreachable there
    :id: SWARM-PORTALLOC-PORT-IN-USE-UNREACHABLE
    :observed_from: client
    :depends_on: os-linux
    :platforms: linux
    :pins: SWARM-PUB-INGRESS-PORT-IN-USE
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A Linux node where a node-local failure to publish one of a service's ingress
    ports occurred, because a process or container on the node uses the port or because the node
    failed to program the firewall rules for the ports, and its cause still holds

    Once no task from an earlier version of the service's spec remains, a client that connects to
    the node's address on any of the service's published ports, in either IP version, does not reach
    the service's tasks.

.. expectation:: Adding an in-use port keeps earlier ports answering while old tasks run
    :id: SWARM-PORTALLOC-PORT-IN-USE-OLD-TASKS
    :observed_from: client
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-ROLLING-UPDATE
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-IPTABLES-LB-EXTENSIONS, SWARM-HOST-IPTABLES-LEGACY-MANGLE-KERNEL, BR-HOST-NO-FORWARD-DROP
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A node where a port is in use by a process or container, and a service that
    publishes other ports in ingress mode

    :api-op:`POST /services/{id}/update` adds the port in use to the service in ingress mode. From
    the response until :api-op:`GET /tasks` first reports a :api-field:`DesiredState` other than
    ``running`` for the last of the tasks from before the update, the service's earlier ports answer
    at the node's address throughout.

.. expectation:: While an ingress publish fails, a listener on the port gets its traffic
    :id: SWARM-PORTALLOC-PUBLISH-FAILED-LISTENER
    :observed_from: client, node
    :depends_on: os-linux
    :platforms: linux
    :refines: PRIN-NO-LEAKS

    *Applies to:* A Linux node where publishing a service's ingress ports failed, and the cause
    still holds

    A process on the node that listens on one of the service's published ports receives the
    connections that clients make to the node's address on that port, in either IP version.

.. expectation:: After a failed ingress publish, removing the service frees the ports
    :id: SWARM-PORTALLOC-PUBLISH-FAILED-PORTS-FREED
    :observed_from: node
    :depends_on: os-linux
    :platforms: linux
    :refines: PRIN-NO-LEAKS
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A Linux node where publishing a service's ingress ports failed

    Within the :need:`[[title|lower]] ceiling ([[seconds]] s) <SWARM-CEIL-REASSIGNMENT>` after
    :api-op:`DELETE /services/{id}` for the service responds 200, a process on the node listens
    without an error on each of the service's published ports that no other process or container on
    the node uses.

.. expectation:: A failure to publish one ingress port leaves the node publishing none
    :id: SWARM-PUB-INGRESS-PORT-IN-USE
    :observed_from: client, log
    :depends_on: os-linux
    :platforms: linux
    :within: SWARM-CEIL-REASSIGNMENT
    :narrows: SWARM-INGPUB-EVERY-NODE, SWARM-PORTALLOC-REPORTED-REACHABLE, SWARM-PUB-TARGET-PORT, SWARM-PUB-PROTOCOLS, SWARM-PUB-REPLIES, SWARM-PUB-UDP-NO-LISTENER, SWARM-PUB-PORT-CHANGE, SWARM-INGRESS-CUSTOM-SUBNET-REACHABLE, SWARM-INGRESS-LABEL-PUBLISH, SWARM-HAIRPIN-LOOPBACK, SWARM-HAIRPIN-NODE-ADDRESS, SWARM-HAIRPIN-BRIDGE-CTR, SWARM-HAIRPIN-SERVICE-NETWORK, SWARM-HAIRPIN-OTHER-NETWORK, SWARM-PUB-TCP-UDP-ONE-SERVICE, SWARM-INGPUB-FIREWALLD-RELOAD, SWARM-PUB-V6-USERLAND-PROXY, SWARM-PUB-V6-USERLAND-PROXY-LOCAL, SWARM-HAIRPIN-USERLAND-PROXY, SWARM-RESTART-GRACEFUL-PUBLISHED-PORTS, SWARM-RESTART-KILLED-PUBLISHED-PORTS, SWARM-RESTART-INGRESS-FROM-OTHER-NODES, SWARM-CONVERGE-REJOIN-PUBLISHED-PORTS, SWARM-CONVERGE-REJOIN-INGRESS-BACKENDS, SWARM-CONVERGE-LEAVE-REMAINING-TASKS, SWARM-SCALE-PORT-PROTOCOLS, SWARM-SCALE-PORT-PROTOCOLS-REMOVAL
    :concerns: SWARM-PORTALLOC-PORT-IN-USE, SWARM-PORTALLOC-PORT-IN-USE-UNREACHABLE, SWARM-FAILREP-INGRESS-PORT-IN-USE, SWARM-PORTALLOC-PUBLISH-FAILED-LISTENER, SWARM-PORTALLOC-PUBLISH-FAILED-PORTS-FREED, SWARM-FAILREP-INGRESS-PORT-RECOVERY
    :violates: REQ-SWARM-ROUTING-MESH, REQ-SWARM-PUB-IPV6-PROXY

    On a Linux node, a node-local failure to publish one of a service's ingress ports leaves the
    node publishing none of them. The failure is a port that a process or container on the node uses
    when the service is created or updated to publish it, or firewall rules for the ports that the
    node fails to program. Once no task from an earlier version of the service's spec remains, a
    client that connects to the node's address on any of the service's published ports, in either IP
    version, does not reach the service's tasks. A process or container that uses one of the ports
    keeps serving it. The daemon log reports the failure.

.. _swarm-traffic-api-value-mode-host:

:api-value:`mode=host`
======================

.. expectation:: A mode=host port is reachable only on nodes that run a task
    :id: SWARM-HOSTPUB-TASK-NODES-ONLY
    :observed_from: client
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-HOSTPUB-LOCAL
    :needs: BR-HOST-NO-FORWARD-DROP
    :within: SWARM-CEIL-REASSIGNMENT

    A :api-value:`mode=host` published port is reachable only at the addresses of nodes that run a
    task of its service.

.. expectation:: A task sees an IPv4 client's real source address
    :id: SWARM-HOSTPUB-CLIENT-SOURCE
    :observed_from: task
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-HOSTPUB-CLIENT-SOURCE
    :needs: BR-HOST-NO-FORWARD-DROP
    :within: SWARM-CEIL-REASSIGNMENT

    A task observes the real source address of an IPv4 client that connects through the published
    port on the task's node.

.. expectation:: With userland-proxy, a task does not see an IPv6 client's address
    :id: SWARM-HOSTPUB-CLIENT-SOURCE-V6
    :observed_from: task
    :depends_on: none
    :platforms: linux
    :pins: SWARM-OVL-V6-ADDRESSING
    :needs: SWARM-HOST-IPV6
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A daemon with :daemon-json:`userland-proxy` true, and a task whose gateway network
    is :network:`docker_gwbridge`

    The task does not observe an IPv6 client's real source address.

.. expectation:: A connection to a node's mode=host port goes to that node's task
    :id: SWARM-HOSTPUB-LOCAL-TASK
    :observed_from: client
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-HOSTPUB-LOCAL
    :needs: BR-HOST-NO-FORWARD-DROP
    :within: SWARM-CEIL-REASSIGNMENT

    A connection to a published port on a node is delivered to the task on that node that published
    the port.

.. expectation:: A dnsrr service with a mode=host port is created and reports the port
    :id: SWARM-HOSTPUB-DNSRR
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-SWARM-DNSRR-NO-INGRESS-PORTS

    :api-op:`POST /services/create` for a service in ``dnsrr`` endpoint mode that publishes a port
    in :api-value:`mode=host` responds 201. :api-op:`GET /tasks/{id}` for a running task of the
    service then reports the port in :api-field:`Status.PortStatus`.

.. expectation:: A task reports its assigned mode=host port in Status.PortStatus
    :id: SWARM-HOSTPUB-ASSIGNED-PORT
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-SWARM-HOSTPUB-PORT-REPORTED

    *Applies to:* A running task of a service with a :api-value:`mode=host` port that does not set
    :api-field:`PublishedPort`

    :api-op:`GET /tasks/{id}` for that task reports its assigned host port in
    :api-field:`Status.PortStatus`.

.. expectation:: The task's container reports the same assigned host port
    :id: SWARM-HOSTPUB-ASSIGNED-PORT-CTR
    :observed_from: api:mgr, api:wkr (on the task's node)
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-SWARM-HOSTPUB-PORT-REPORTED

    *Applies to:* A running task of a service with a :api-value:`mode=host` port that does not set
    :api-field:`PublishedPort`

    On that task's node, :api-op:`GET /containers/json` reports in the :api-field:`Ports` of the
    task's container the host port that :api-op:`GET /tasks/{id}` reports in
    :api-field:`Status.PortStatus`. No two entries of that :api-field:`Ports` are identical. No list
    of bindings in the :api-field:`NetworkSettings.Ports` that :api-op:`GET /containers/{id}/json`
    reports for the container has two identical entries either.

.. expectation:: docker service ps prints a task's assigned host port
    :id: SWARM-HOSTPUB-ASSIGNED-PORT-CLI
    :observed_from: api:mgr
    :depends_on: cli
    :platforms: linux, windows
    :refines: REQ-SWARM-HOSTPUB-PORT-REPORTED
    :cli_for: SWARM-HOSTPUB-ASSIGNED-PORT

    *Applies to:* A running task of a service with a :api-value:`mode=host` port that does not set
    :api-field:`PublishedPort`

    :cli:`docker service ps` on a manager prints the task's assigned host port.

.. expectation:: docker container ls prints a task's assigned host port
    :id: SWARM-HOSTPUB-ASSIGNED-PORT-CTR-CLI
    :observed_from: api:mgr, api:wkr (on the task's node)
    :depends_on: cli
    :platforms: linux, windows
    :refines: REQ-SWARM-HOSTPUB-PORT-REPORTED
    :cli_for: SWARM-HOSTPUB-ASSIGNED-PORT-CTR

    *Applies to:* A running task of a service with a :api-value:`mode=host` port that does not set
    :api-field:`PublishedPort`

    :cli:`docker container ls` on the task's node prints the task's assigned host port.

.. expectation:: Two services publish the same mode=host port
    :id: SWARM-HOSTPUB-SHARED-PORT
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-HOSTPUB-LOCAL

    *Applies to:* A Swarm with a service that publishes a port in :api-value:`mode=host`

    :api-op:`POST /services/create` for another service that publishes the same port in
    :api-value:`mode=host` responds 201. :api-op:`GET /services` then lists both services.

.. expectation:: Tasks with the same chosen mode=host port never share a node
    :id: SWARM-HOSTPUB-PLACEMENT
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-HOSTPUB-LOCAL

    Tasks that publish the same chosen :api-value:`mode=host` port
    (:ref:`swarm-traffic-port-allocation`) are never placed on the same node, whether they belong to
    one service or several. :api-op:`GET /tasks` never reports two such tasks with the same
    :api-field:`NodeID` and a :api-field:`Status.State` from ``assigned`` to ``running``, whatever
    their :api-field:`DesiredState`.

.. expectation:: A task stays pending when every node already uses its mode=host port
    :id: SWARM-HOSTPUB-PLACEMENT-PENDING
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-HOSTPUB-LOCAL
    :within: SWARM-CEIL-ALLOCATION

    *Applies to:* A Swarm in which each node runs a task that publishes a chosen
    :api-value:`mode=host` port (:ref:`swarm-traffic-port-allocation`)

    A new task that publishes the same port in :api-value:`mode=host` stays in the ``pending`` state
    that :api-op:`GET /tasks/{id}` reports in :api-field:`Status.State`.

.. expectation:: A mode=host target port above 65535 runs tasks but reports no port
    :id: SWARM-HOSTPUB-TARGET-OUT-OF-RANGE
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows
    :pins: SWARM-PORTALLOC-PORT-ABOVE-65535
    :within: SWARM-CEIL-ALLOCATION

    :api-op:`POST /services/create` for a service that publishes a :api-value:`mode=host` port with
    a :api-field:`TargetPort` above 65535 responds 201. The service's tasks run, and :api-op:`GET
    /tasks/{id}` reports no port in :api-field:`Status.PortStatus`.

.. _swarm-traffic-egress-the-gateway-network:

******************************
 Egress & the gateway network
******************************

.. expectation:: A container on a non-internal overlay reaches the physical network
    :id: SWARM-EGRESS-NON-INTERNAL
    :observed_from: task or ctr → underlay
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-OVERLAY-EGRESS
    :needs: BR-HOST-NO-FORWARD-DROP
    :within: SWARM-CEIL-REASSIGNMENT

    A container on a non-internal overlay has egress to the physical network.

.. expectation:: A container's egress traffic has the node's address as its source
    :id: SWARM-EGRESS-SOURCE-ADDRESS
    :observed_from: underlay
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-OVERLAY-EGRESS
    :needs: BR-HOST-NO-FORWARD-DROP
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A container attached only to overlay networks, at least one of them not
    ``internal``

    For the container's egress traffic, the source address that an underlay host observes is the
    node's address, not the container's.

.. expectation:: A container on only internal overlays has no egress
    :id: SWARM-EGRESS-INTERNAL-ONLY
    :observed_from: task, ctr
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-SWARM-OVERLAY-EGRESS
    :within: SWARM-CEIL-REASSIGNMENT

    A container attached only to ``internal`` overlays does not have egress.

.. expectation:: A container on an internal and a non-internal overlay has egress
    :id: SWARM-EGRESS-MIXED-INTERNAL
    :observed_from: task, ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-OVERLAY-EGRESS
    :needs: BR-HOST-NO-FORWARD-DROP
    :within: SWARM-CEIL-REASSIGNMENT

    A container on both an internal and a non-internal overlay has egress. Its default route is not
    on the internal overlay.

.. expectation:: A Windows container gets a default route from an internal overlay too
    :id: SWARM-EGRESS-WINDOWS-MIXED-INTERNAL
    :observed_from: task
    :depends_on: none
    :platforms: windows
    :within: SWARM-CEIL-REASSIGNMENT
    :narrows: SWARM-EGRESS-MIXED-INTERNAL
    :violates: REQ-SWARM-OVERLAY-EGRESS

    A Windows container attached to an ``internal`` overlay and to a non-internal overlay has a
    default route through the gateway of each of the two networks.

.. expectation:: A task of a service with no network is on the default bridge only
    :id: SWARM-EGRESS-NO-NETWORK-DEFAULT-BRIDGE
    :observed_from: api:mgr, api:wkr (on the task's node)
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-NO-NETWORK-DEFAULT-BRIDGE
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A Linux node that runs a task of a service whose spec names no network and that
    publishes no port in ingress mode

    On that node, :api-op:`GET /containers/{id}/json` for the task's container reports the default
    bridge network, :network:`bridge`, and no other network in
    :api-field:`NetworkSettings.Networks`.

.. _swarm-traffic-gateway-network:

Gateway network
===============

Each node has a gateway bridge network. Containers on the node's non-internal overlays reach the
physical network through it. This subsection holds on Linux nodes.

.. expectation:: A gateway bridge network exists within the ceiling after swarm init
    :id: SWARM-GWNET-AFTER-INIT
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-GWBRIDGE
    :within: SWARM-CEIL-REJOIN

    Within the :need:`[[title|lower]] ceiling ([[seconds]] s) <SWARM-CEIL-REJOIN>` after
    :api-op:`POST /swarm/init` responds 200 on a node, :api-op:`GET /networks` on the node lists a
    gateway bridge network.

.. expectation:: A gateway bridge network exists within the ceiling after a join
    :id: SWARM-GWNET-AFTER-JOIN
    :observed_from: api:mgr, api:wkr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-GWBRIDGE
    :within: SWARM-CEIL-REJOIN

    *Applies to:* A Swarm with an ingress network

    Within the :need:`[[title|lower]] ceiling ([[seconds]] s) <SWARM-CEIL-REJOIN>` after
    :api-op:`POST /swarm/join` responds 200 on a node, :api-op:`GET /networks` on that node lists a
    gateway bridge network.

.. expectation:: A join without an ingress network does not create a gateway network
    :id: SWARM-GWNET-NO-INGRESS
    :observed_from: api:mgr, api:wkr
    :depends_on: none
    :platforms: linux
    :within: SWARM-CEIL-REJOIN
    :concerns: SWARM-GWNET-AFTER-JOIN, SWARM-GWNET-NO-INGRESS-FIRST-CTR
    :violates: REQ-SWARM-GWBRIDGE

    After :api-op:`POST /swarm/join` responds 200 on a node, for a Swarm without an ingress network,
    :api-op:`GET /networks` on the node does not list a gateway bridge network until a container on
    a non-internal overlay starts on the node.

.. expectation:: Without ingress, the first overlay container adds a gateway network
    :id: SWARM-GWNET-NO-INGRESS-FIRST-CTR
    :observed_from: api:active
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-OVERLAY-EGRESS
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A node that joined a Swarm without an ingress network

    Once a container on a non-internal overlay has started on the node, :api-op:`GET /networks` on
    the node lists a gateway bridge network.

.. expectation:: A node's gateway bridge network is named docker_gwbridge
    :id: SWARM-GWNET-NAME
    :observed_from: api:mgr, api:wkr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-GWBRIDGE

    A node's gateway bridge network has the name :network:`docker_gwbridge`.

.. expectation:: The gateway network's bridge interface is named docker_gwbridge
    :id: SWARM-GWNET-BRIDGE-IFACE-NAME
    :observed_from: node
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-GWBRIDGE

    The bridge interface of a node's gateway bridge network has the name :network:`docker_gwbridge`.

.. expectation:: An operator-created docker_gwbridge keeps its subnet and options
    :id: SWARM-GWNET-OPERATOR-CREATED
    :observed_from: api:active
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-GWBRIDGE
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A node where the operator created :network:`docker_gwbridge` before its first
    container on a non-internal overlay, and before it joined a Swarm with an ingress network, with
    a subnet and with options of the :driver:`bridge` driver that the operator chose, including
    options whose values differ from the ones the daemon sets on the gateway network it creates

    After containers on non-internal overlays start on that node, :api-op:`GET
    /networks/docker_gwbridge` there reports the subnet and each option that the operator gave it.

.. expectation:: Containers get addresses in an operator-created docker_gwbridge subnet
    :id: SWARM-GWNET-OPERATOR-CREATED-ADDRESSES
    :observed_from: task, ctr
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-SWARM-GWBRIDGE
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A node where the operator created :network:`docker_gwbridge` before its first
    container on a non-internal overlay, and before it joined a Swarm with an ingress network

    The containers on non-internal overlays that then start on that node have gateway-network
    addresses in the operator's subnet.

.. expectation:: A daemon-created gateway network reports enable_icc as false
    :id: SWARM-GWNET-NO-ICC
    :observed_from: api:mgr, api:wkr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-NETWORK-ISOLATION

    *Applies to:* A gateway network that the operator did not create

    :api-op:`GET /networks/{id}` for that network reports
    :drvopt:`com.docker.network.bridge.enable_icc` as ``false`` in :api-field:`Options`.

.. expectation:: A daemon-created gateway network gives containers no IPv6 address
    :id: SWARM-GWNET-NO-V6-ADDRESS
    :observed_from: task, ctr
    :depends_on: os-linux
    :platforms: linux
    :refines: PRIN-LEAST-ACCESS
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A gateway network that the operator did not create, and the containers on it,
    among them a container on an overlay network created with :api-field:`EnableIPv6` true

    A container's interface on that network does not have an IPv6 address.

.. expectation:: An underlay host does not reach unpublished ports at a gateway address
    :id: SWARM-GWNET-UNPUBLISHED-PORT
    :observed_from: underlay → task or ctr
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-SWARM-UNPUBLISHED-NOT-EXPOSED
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A container with an address on a node's gateway network, and an underlay host with
    a route to that address via the node's underlay address

    Traffic from that host to that address does not reach the container on a port that the container
    does not publish.
