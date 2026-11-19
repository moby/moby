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
is programmed on a node while the node has a local attachment to it. A task of a service on one of
the Swarm's predefined networks, which :doc:`network-common.md <network-common>` defines, is on the
node's own network of that name: its container is attached to that network. No node has a local
attachment to the Swarm's predefined networks. A node creates its load-balancer endpoint on a
network of the :driver:`overlay` driver, the ingress network included, with the network, and deletes
it with the network. :api-op:`GET /networks/{id}` on the node lists that endpoint, with its address,
in :api-field:`Containers`, as the entry that is not one of the node's containers. Except on an
ingress network, in either form that :ref:`swarm-networking-the-ingress-network` describes, the node
deletes the network once no task or standalone container of the node is on it, so the load-balancer
endpoint does not outlast them. A node keeps its local attachment to an ingress network after its
last task on that network is gone. Unless an expectation or its section names other values, an
expectation holds in a Swarm with the cluster settings that :api-op:`POST /swarm/init` sets by
default. An expectation holds whatever data-path, advertise and listen addresses each node gives in
:api-op:`POST /swarm/init` or :api-op:`POST /swarm/join`, when the data-path addresses of the
Swarm's nodes are all of one IP version, and when each node's route for its VXLAN traffic to the
other nodes selects its data-path address as the source address
(:need:`SWARM-CLUSTER-VXLAN-SOURCE-NOT-DATA-PATH`), unless the expectation names that case. An
expectation about traffic between nodes holds for a Linux node and a Windows node only when their
data-path addresses are IPv4 (:need:`SWARM-EW-WINDOWS-V6-DATA-PATH`).

An expectation that involves the ingress network holds for an ingress network that was created with
:api-field:`Ingress` set, that is not ``internal`` and that does not have the option :drvopt:`dsr`.
Only an expectation that names an ingress network without :api-field:`Ingress` covers such a network
(:ref:`swarm-networking-the-ingress-network`). An expectation about a request, or a command that
makes one, that gives a service a published port in ingress mode, whether the request sets that mode
or the daemon stores the port in ingress mode because :api-field:`PublishMode` is unset or is a
value that the API reference does not list, holds for a request whose service spec is in the ``vip``
endpoint mode, in a Swarm where an ingress network exists. Only an expectation that names a spec in
the ``dnsrr`` endpoint mode or a Swarm without an ingress network covers those cases
(:need:`SWARM-INGPUB-DNSRR-REFUSED`, :need:`SWARM-INGRESS-ABSENT-PUBLISH`). An expectation that
involves a node's gateway network holds for one that the daemon created, or that the operator
created with the options of one that the daemon creates and no others but a subnet and a gateway
(:ref:`swarm-traffic-gateway-network` in :doc:`swarm-traffic.md <swarm-traffic>`), unless the
expectation names other options. The daemon creates its gateway network with
:drvopt:`com.docker.network.bridge.name` set to :network:`docker_gwbridge`,
:drvopt:`com.docker.network.bridge.enable_icc` set to ``false`` and
:drvopt:`com.docker.network.bridge.enable_ip_masquerade` set to ``true``. On an overlay network with
more than one subnet, an expectation about traffic to a VIP holds for the VIP and a backend task
only when both are in the subnet of the load-balancer endpoint of the client's node on that network
(:need:`SWARM-EW-VIP-MULTIPLE-SUBNETS`). On Linux nodes, an expectation about a
:api-value:`mode=host` published port holds for a service that is attached to at least one network
that is not ``internal``, and for a service whose spec names no network and that publishes no port
in ingress mode, since its tasks run on the default bridge network
(:need:`SWARM-EGRESS-NO-NETWORK-DEFAULT-BRIDGE`). An expectation about reaching a
:api-value:`mode=host` published port at an address of the node holds on a network whose gateway
mode for that address's family is not ``routed``, with
:drvopt:`com.docker.network.bridge.host_binding_ipv4` unset or ``0.0.0.0`` and without
:drvopt:`com.docker.network.bridge.inhibit_ipv4`, unless the expectation names them
(:doc:`bridge-networking.md <bridge-networking>`).

.. _swarm-networking-requirements:

**************
 Requirements
**************

.. req:: A container whose network attachment fails does not start
    :id: REQ-SWARM-ATTACH-FAILURE-START
    :source_kind: docs
    :source: api/openapi.yaml: operation ContainerStart, https://docs.docker.com/engine/containers/run/#exit-status, https://docs.docker.com/engine/release-notes/23.0/#2303

    When the daemon cannot attach a container to its network, the start request fails with an error
    response, and docker run exits non-zero (125 for a daemon error).

.. req:: An overlay network without a subnet gets one from the default address pools
    :id: REQ-SWARM-CLUSTER-ADDRESS-POOLS
    :source_kind: docs
    :source: https://docs.docker.com/engine/swarm/swarm-mode/#configuring-default-address-pools, https://docs.docker.com/engine/swarm/networking/#using-custom-default-address-pools, https://docs.docker.com/reference/cli/docker/swarm/init/#default-addr-pool, https://docs.docker.com/reference/api/engine/version/v1.54/#tag/System/operation/SystemInfo

    An overlay network created without a subnet gets a subnet allocated from the default address
    pools set at swarm init, 10.0.0.0/8 by default, with the configured mask length, /24 by default.
    The subnet size is at most 29.

.. req:: Swarm control and management plane traffic is always encrypted
    :id: REQ-SWARM-CLUSTER-CONTROL-ENCRYPTED
    :source_kind: docs
    :source: https://docs.docker.com/engine/swarm/networking/#swarm-and-types-of-traffic, https://docs.docker.com/engine/swarm/networking/#encryption

    Management and control plane traffic between Swarm nodes is always encrypted, whether or not
    application data on an overlay network is encrypted.

.. req:: DataPathAddr sets the address peers send a node's overlay traffic to
    :id: REQ-SWARM-CLUSTER-DATA-PATH-ADDRESS
    :source_kind: docs
    :source: https://docs.docker.com/reference/cli/docker/swarm/init/#data-path-addr, https://docs.docker.com/engine/swarm/networking/#use-a-separate-interface-for-control-and-data-traffic, https://docs.docker.com/reference/api/engine/version/v1.54/#tag/Swarm/operation/SwarmInit, https://docs.docker.com/engine/release-notes/25.0/#2500, https://docs.docker.com/reference/api/engine/version/v1.54/#tag/Swarm/operation/SwarmJoin, https://docs.docker.com/reference/cli/docker/swarm/init/#advertise-addr

    The data-path address given at swarm init or join, an IPv4 or IPv6 address or an interface, is
    the address other nodes use to reach the node's containers, so that data traffic can be
    separated from management traffic. Without it, the advertise address is used. When the data-path
    address or the advertise address is given as an interface, traffic among a service's containers
    is sent over the data-path interface, and traffic about joining, leaving and managing the Swarm
    over the advertise interface. Overlay networks are supported over an IPv6 transport.

.. req:: The data-path port is set at init, 1024 to 49151, default 4789
    :id: REQ-SWARM-CLUSTER-DATA-PATH-PORT
    :source_kind: docs
    :source: https://docs.docker.com/reference/cli/docker/swarm/init/#data-path-port, https://docs.docker.com/reference/api/engine/version/v1.54/#tag/Swarm/operation/SwarmInit, https://docs.docker.com/engine/network/drivers/overlay/#requirements

    The UDP port for overlay data traffic is set at swarm init and applies to every node of the
    swarm. It must be in the range 1024 to 49151; when it is unset or 0, the default port 4789 is
    used, and docker info reports the configured port.

.. req:: Nodes discover each other's networks on TCP and UDP port 7946
    :id: REQ-SWARM-CLUSTER-GOSSIP-PORTS
    :source_kind: docs
    :source: https://docs.docker.com/engine/network/drivers/overlay/#requirements, https://docs.docker.com/engine/swarm/networking/#firewall-considerations, https://docs.docker.com/reference/cli/docker/swarm/init/#advertise-addr, https://docs.docker.com/reference/cli/docker/swarm/init/#listen-addr

    Nodes exchange container network discovery traffic on TCP and UDP port 7946. A node is reached
    at the address it advertises to the other members for overlay networking.

.. req:: Overlay traffic between nodes is VXLAN to the data-path port
    :id: REQ-SWARM-DATA-PATH-VXLAN
    :source_kind: docs
    :source: https://docs.docker.com/engine/network/drivers/overlay/#requirements, https://docs.docker.com/engine/swarm/networking/#firewall-considerations, https://docs.docker.com/engine/swarm/swarm-tutorial/#open-protocols-and-ports-between-the-hosts, https://docs.docker.com/engine/release-notes/24.0/#2403

    Overlay traffic between nodes, the ingress network included, is VXLAN over UDP to the Swarm
    data-path port: 4789 by default, configurable at swarm init. Encrypted overlays use the
    configured data-path port too.

.. req:: Swarm init or join creates the ingress network on each node
    :id: REQ-SWARM-INGRESS-CREATED-AT-INIT
    :source_kind: docs
    :source: https://docs.docker.com/engine/swarm/networking/#overlay-networking, https://docs.docker.com/engine/swarm/networking/#key-network-concepts, https://docs.docker.com/engine/network/drivers/overlay/#use-the-default-overlay-network

    When a swarm is initialized or a node joins one, an overlay network called ingress is created on
    that node to carry routing-mesh traffic; every node of the swarm lists it.

.. req:: An overlay named ingress with the internal label is the ingress network
    :id: REQ-SWARM-INGRESS-LEGACY-FORM
    :source_kind: issue
    :source: https://github.com/moby/swarmkit/pull/2028: the maintainers' request that the API definition of the flag say so, swarmkit api/specs.proto: NetworkSpec.Ingress

    Ingress networks created before the Ingress flag existed have the name ingress and the label
    com.docker.swarm.internal instead of the flag. Such a network is the ingress network.

.. req:: The ingress network carries the routing mesh's published-port traffic
    :id: REQ-SWARM-INGRESS-MESH-ONLY
    :source_kind: docs
    :source: https://docs.docker.com/engine/swarm/networking/#key-network-concepts, https://docs.docker.com/engine/release-notes/prior-releases/#1122-2016-10-11

    The ingress network exists to carry published-port traffic of the routing mesh. Service
    discovery is disabled on it and services cannot be placed on it as a regular network.

.. req:: A service cannot name the ingress network as one of its networks
    :id: REQ-SWARM-INGRESS-NOT-FOR-SERVICES
    :source_kind: api
    :source: https://docs.docker.com/reference/api/engine/version/v1.54/#tag/Service/operation/ServiceCreate, https://docs.docker.com/engine/swarm/networking/#customize-ingress

    Service create and update refuse a network that is not eligible for services; the ingress
    network is reached only through published ports.

.. req:: The ingress network is an overlay network and cannot be attachable
    :id: REQ-SWARM-INGRESS-OVERLAY-ONLY
    :source_kind: docs
    :source: https://docs.docker.com/reference/cli/docker/network/create/#ingress, https://docs.docker.com/reference/api/engine/version-history/#v129-api-changes, api/openapi.yaml: operation NetworkCreate

    Creating the ingress network is supported only with the overlay driver. Any overlay option is
    available for it except --attachable.

.. req:: An ingress network with a driver other than overlay is refused with 501
    :id: REQ-SWARM-INGRESS-DRIVER-NOT-IMPLEMENTED
    :source_kind: issue
    :source: https://github.com/moby/swarmkit/pull/2028: the merged change that lets the ingress network be replaced

    Creating an ingress network with a driver other than overlay is refused as not implemented, with
    status 501.

.. req:: docker network rm asks before it removes the ingress network
    :id: REQ-SWARM-INGRESS-REMOVE-CONFIRM
    :source_kind: docs
    :source: https://docs.docker.com/engine/swarm/networking/#customize-ingress

    docker network rm for the ingress network prints a warning and asks the user to confirm, with no
    as the default answer ([y/N]).

.. req:: The ingress network can be removed and recreated with custom options
    :id: REQ-SWARM-INGRESS-REPLACEABLE
    :source_kind: docs
    :source: https://docs.docker.com/engine/swarm/networking/#customize-ingress, https://docs.docker.com/reference/cli/docker/network/create/#ingress, https://github.com/moby/moby/pull/31714

    Once no service publishes ports through it, the ingress network can be removed and recreated
    with --ingress, under any name and with a custom subnet, gateway and options. While no ingress
    network exists, services cannot publish ports through the routing mesh.

.. req:: A swarm has at most one ingress network
    :id: REQ-SWARM-INGRESS-SINGLE
    :source_kind: docs
    :source: https://docs.docker.com/engine/swarm/networking/#customize-ingress, https://docs.docker.com/reference/cli/docker/network/create/#ingress

    Only one ingress network can exist at a time; an attempt to create a second one fails.

.. req:: Nodes of different engine releases share overlay and ingress networks
    :id: REQ-SWARM-MIXED-VERSION-INTEROP
    :source_kind: docs
    :source: https://docs.docker.com/engine/release-notes/29/#2921, https://docs.docker.com/engine/release-notes/18.02/#18020-ce, https://docs.docker.com/engine/swarm/networking/#customize-ingress

    A Swarm is upgraded node by node, so nodes of different releases share overlay and ingress
    networks during the rollout. During the rollout, traffic between nodes of different releases
    passes on the ingress network and on overlay networks. On an encrypted overlay network it passes
    too, except between a node of 29.2.1 or later and a node of 29.0.0 to 29.2.0, 28.2.2, 25.0.14 or
    25.0.13.

.. req:: Standalone containers attach only to attachable Swarm networks
    :id: REQ-SWARM-NETWORK-ATTACHABLE
    :source_kind: docs
    :source: https://docs.docker.com/reference/cli/docker/network/create/, https://docs.docker.com/engine/network/drivers/overlay/#attach-a-container-to-an-overlay-network, https://docs.docker.com/reference/api/engine/version/v1.54/#tag/Network/operation/NetworkConnect

    By default a Swarm-scoped network accepts only Swarm services. Created with Attachable, it also
    accepts standalone containers through docker run and docker network connect, on any node of the
    swarm.

.. req:: A container leaving a Swarm network releases its attachment
    :id: REQ-SWARM-NETWORK-ATTACHMENT-RELEASED
    :source_kind: docs
    :source: https://docs.docker.com/engine/network/drivers/overlay/#use-an-overlay-network-for-standalone-containers, https://docs.docker.com/reference/cli/docker/network/rm/

    When a standalone container on an attachable Swarm network stops or is disconnected, the network
    is released on that node, and once no container uses the network it can be removed.

.. req:: A Swarm network of a local driver takes each node's config-only network
    :id: REQ-SWARM-NETWORK-CONFIG-FROM
    :source_kind: docs
    :source: https://docs.docker.com/reference/cli/docker/network/create/#swarm-networks-with-local-scope-drivers, https://docs.docker.com/reference/api/engine/version/v1.54/#tag/Network/operation/NetworkCreate

    A network of a node-local driver can be promoted to scope swarm and used by services.
    Node-specific configuration, such as the subnet, comes from a config-only network of the name
    that --config-from gives, which is created on each node.

.. req:: Swarm-scoped networks are created on a manager
    :id: REQ-SWARM-NETWORK-CREATE-ON-MANAGER
    :source_kind: docs
    :source: https://docs.docker.com/engine/swarm/services/#connect-the-service-to-an-overlay-network, https://docs.docker.com/reference/cli/docker/network/create/, https://docs.docker.com/reference/cli/docker/network/create/#swarm-networks-with-local-scope-drivers, https://docs.docker.com/reference/api/engine/version/v1.54/#tag/Network/operation/NetworkCreate

    Once swarm mode is enabled, an overlay network, or a network of another driver with scope swarm,
    is created on a manager node with docker network create; the network then exists for the whole
    swarm.

.. req:: A network with no containers can be removed, also with only dead tasks
    :id: REQ-SWARM-NETWORK-DELETE
    :source_kind: docs
    :source: https://docs.docker.com/reference/cli/docker/network/rm/, https://docs.docker.com/engine/release-notes/17.03/#17031-ce

    A network can be removed once no container is connected to it, also when only dead tasks
    reference it.

.. req:: A Swarm network that a service uses is not removed
    :id: REQ-SWARM-NETWORK-DELETE-IN-USE
    :source_kind: docs
    :source: https://docs.docker.com/engine/swarm/networking/#customize-ingress, https://docs.docker.com/reference/cli/docker/network/create/#ingress, api/openapi.yaml: operation NetworkDelete

    A Swarm network can be removed only once the services that use it are removed. DELETE
    /networks/{id} for a Swarm-scoped network that a service or a task uses responds 400.

.. req:: Overlay options select encryption, direct server return and the MTU
    :id: REQ-SWARM-NETWORK-DRIVER-OPTIONS
    :source_kind: docs
    :source: https://docs.docker.com/engine/swarm/networking/#encryption, https://docs.docker.com/engine/swarm/networking/#use-direct-server-return, https://docs.docker.com/engine/swarm/networking/#customize-ingress, https://docs.docker.com/engine/network/drivers/overlay/#encrypt-traffic-on-an-overlay-network

    When an overlay network is created, the encrypted option enables IPsec encryption of its
    application data, the dsr option makes east-west load balancing use direct server return, and
    com.docker.network.driver.mtu sets its MTU.

.. req:: Network inspect reports per-subnet IPAM usage from API 1.52 only
    :id: REQ-SWARM-NETWORK-IPAM-STATUS
    :source_kind: api
    :source: https://docs.docker.com/reference/api/engine/version-history/#v152-api-changes, https://docs.docker.com/reference/api/engine/version/v1.54/#tag/Network/operation/NetworkInspect

    From API 1.52, GET /networks/{id} includes a Status field with, for each subnet of the network,
    the number of addresses in use or reserved and the number available for dynamic allocation. At
    earlier API versions, the response does not include Status.

.. req:: Managed network plugins work in Swarm mode, legacy plugins do not
    :id: REQ-SWARM-NETWORK-PLUGIN-DRIVERS
    :source_kind: docs
    :source: https://docs.docker.com/engine/extend/plugins_network/#network-plugins-and-swarm-mode, moby/moby docs/extend/plugins_services.md: section Network plugins

    Network plugins of the v2 plugin system work in swarm mode when installed on the nodes, and
    plugin availability determines where networks are created and tasks are scheduled. Legacy
    plugins do not work in swarm mode.

.. req:: Every manager lists a Swarm network with scope swarm
    :id: REQ-SWARM-NETWORK-SCOPE-VISIBLE
    :source_kind: docs
    :source: https://docs.docker.com/engine/swarm/services/#connect-the-service-to-an-overlay-network, https://docs.docker.com/engine/swarm/networking/#create-an-overlay-network, https://docs.docker.com/reference/api/engine/version/v1.54/#tag/Network/operation/NetworkInspect, https://docs.docker.com/reference/api/engine/version-history/#v129-api-changes

    After an overlay network is created in swarm mode, all manager nodes have access to it, and
    network ls and network inspect report its driver and the scope swarm. The scope parameter of the
    network list and inspect requests selects networks by scope.

.. req:: A vip-mode service has a VIP on each overlay network it is attached to
    :id: REQ-SWARM-NETWORK-SERVICE-VIP
    :source_kind: docs
    :source: https://docs.docker.com/engine/swarm/networking/#configure-service-discovery, https://docs.docker.com/reference/api/engine/version/v1.54/#tag/Service/operation/ServiceInspect

    A service in the default vip endpoint mode is assigned a virtual IP, the front end that clients
    use to reach it, on each overlay network it is attached to; the service's Endpoint reports these
    VirtualIPs with their network IDs.

.. req:: A vip-mode service keeps its VIP on a network while it stays attached
    :id: REQ-SWARM-NETWORK-SERVICE-VIP-KEPT
    :source_kind: observable
    :source: Clients that resolve a service's name once and keep connecting to that address (such as a reverse proxy that resolves its upstream at startup) and clients that cache the embedded DNS server's answer for its TTL of 600 seconds. They keep using a service's VIP across updates of the service and changes of leader

    A service in vip endpoint mode keeps its VIP on a network while it stays in vip mode and
    attached to that network.

.. req:: A static address that is not available makes the container fail to start
    :id: REQ-SWARM-NETWORK-STATIC-ADDRESS
    :source_kind: docs
    :source: https://docs.docker.com/reference/cli/docker/network/connect/#ip, https://docs.docker.com/reference/cli/docker/network/connect/#network-implications-of-stopping-pausing-or-restarting-containers

    A container's requested IP address on a network is applied when it starts; if the address is no
    longer available, the container fails to start. An address from outside the network's IP range
    is not given to another container while its holder is off the network.

.. req:: An overlay network uses the subnet, gateway and IP range it is given
    :id: REQ-SWARM-NETWORK-SUBNET-CONFIG
    :source_kind: docs
    :source: https://docs.docker.com/engine/swarm/networking/#configure-the-subnet-and-gateway, https://docs.docker.com/reference/cli/docker/network/create/

    An overlay network can be created with its own subnets, gateways and IP ranges in place of an
    allocated subnet. Container addresses, and the other IP addresses that the network allocates,
    then come from the IP range.

.. req:: A network that is not allocated does not break requests about others
    :id: REQ-SWARM-NETWORK-UNALLOCATED-ISOLATED
    :source_kind: issue
    :source: https://github.com/moby/moby/pull/53325

    A Swarm network whose allocation is pending or failed must not make network list or inspect
    requests fail for other, healthy networks.

.. req:: Verbose network inspect shows each service's VIP, ports and tasks
    :id: REQ-SWARM-NETWORK-VERBOSE-INSPECT
    :source_kind: docs
    :source: https://docs.docker.com/reference/cli/docker/network/inspect/#options, https://docs.docker.com/reference/api/engine/version-history/#v128-api-changes, https://docs.docker.com/reference/api/engine/version/v1.54/#tag/Network/operation/NetworkInspect

    For swarm overlay networks, verbose network inspect shows service details: each service's VIP
    and port mappings and the addresses of its tasks, local and non-local.

.. req:: A Swarm network is extended to a worker that uses it
    :id: REQ-SWARM-NETWORK-WORKER-ON-DEMAND
    :source_kind: docs
    :source: https://docs.docker.com/engine/swarm/services/#connect-the-service-to-an-overlay-network, https://docs.docker.com/engine/network/drivers/overlay/#use-an-overlay-network-for-standalone-containers

    A Swarm network is extended to a worker when a service task or an attached container on that
    worker needs it.

.. req:: Containers on a shared overlay reach each other across nodes
    :id: REQ-SWARM-OVERLAY-CONNECTIVITY
    :source_kind: docs
    :source: https://docs.docker.com/engine/swarm/networking/#attach-a-service-to-an-overlay-network, https://docs.docker.com/engine/network/drivers/overlay/, https://docs.docker.com/engine/network/drivers/overlay/#container-discovery, https://docs.docker.com/engine/release-notes/18.03/#18030-ce

    Services and containers attached to the same overlay network communicate with each other across
    it, whichever nodes they run on; Docker routes each packet to the destination container
    transparently. Port mappings on overlays cover TCP, UDP and SCTP.

.. req:: Overlay inspect lists the network's peer nodes at their advertise addresses
    :id: REQ-SWARM-OVERLAY-PEERS
    :source_kind: api
    :source: api/openapi.yaml: definition Network (Peers), api/openapi.yaml: definition PeerInfo, https://docs.docker.com/reference/cli/docker/swarm/join/#advertise-addr

    On a node with a local attachment to an overlay network, GET /networks/{id} lists in Peers the
    nodes that have a local attachment to the network. The IP of each entry is the address that the
    node advertises to the other members of the Swarm.

.. req:: Overlay inspect reports the VNIs that the network uses
    :id: REQ-SWARM-OVERLAY-VNI-REPORTED
    :source_kind: observable
    :source: Operators who read the VNIs of an overlay network in network inspect to match VXLAN traffic to networks in firewall rules and packet captures and to create a network again with the same VNIs. Network inspect is the only place that shows a VNI that the managers allocated, https://docs.docker.com/engine/swarm/networking/#create-an-overlay-network: example output that shows the option, api/openapi.yaml: definition Network (Options)

    GET /networks/{id} on a manager reports the VNIs of an overlay network in its vxlanid_list
    option.

.. req:: An overlay network's vxlanid_list option sets its VNIs
    :id: REQ-SWARM-OVERLAY-VNI-LIST
    :source_kind: observable
    :source: Operators who pass com.docker.network.driver.overlay.vxlanid_list to docker network create or in a stack file's driver_opts to choose the VNIs that an overlay network's VXLAN traffic carries on the underlay. Network inspect shows the option on every overlay network

    An overlay network created with the driver option com.docker.network.driver.overlay.vxlanid_list
    uses the VNIs in that list, one per subnet in order, and reports them in its options. Subnets
    beyond the list get automatic VNIs. A VNI that a deleted network used can be requested again.

.. req:: An ingress-mode port is reachable on every node, task or not
    :id: REQ-SWARM-ROUTING-MESH
    :source_kind: docs
    :source: https://docs.docker.com/engine/swarm/ingress/, https://docs.docker.com/engine/swarm/services/#publish-a-services-ports-using-the-routing-mesh, api/openapi.yaml: definition EndpointPortConfig (PublishMode), https://docs.docker.com/engine/release-notes/29/#2900

    In ingress mode, every node accepts connections on the published port, whether or not it runs a
    task, and routes each connection to an active task. Services stay reachable on published ports
    after a firewalld reload.

.. req:: A task that cannot run fails or is rejected, and reports its error
    :id: REQ-SWARM-TASK-ERRORS
    :source_kind: docs
    :source: https://docs.docker.com/engine/swarm/how-swarm-mode-works/swarm-task-states/, https://docs.docker.com/reference/cli/docker/service/ps/, api/openapi.yaml: definition TaskStatus (Err)

    A task advances through states and ends as failed or rejected when it cannot run; a worker can
    reject a task, which is then in the REJECTED state. The error appears in the task's status, and
    docker service ps lists a service's tasks with their states, and with --no-trunc their full
    error messages.

.. _swarm-networking-rationales:

************
 Rationales
************

.. rationale:: Windows nodes cannot use a VNI in the range of 802.1Q VLAN IDs
    :id: RAT-SWARM-AUTO-VNI-ABOVE-VLAN-IDS

    Windows does not support VNIs in the range of 802.1Q VLAN IDs, 0 to 4095. The managers choose a
    network's VNIs before they know which nodes will attach to it, so they take automatic VNIs from
    4096 up. An automatic VNI below 4096 would give an overlay network that Windows nodes cannot
    use. A change to the range from which the managers take automatic VNIs should therefore be
    deliberate.

.. rationale:: The managers do not track which nodes have a config-only network
    :id: RAT-SWARM-CONFIG-ONLY-PER-NODE

    A config-only network is node-local: each node has its own, and the managers' store does not
    record which nodes have one of a given name. The manager that handles a create checks only its
    own config-only networks, and the scheduler places a service's tasks without regard to them;
    the operator creates them on each node (:need:`REQ-SWARM-NETWORK-CONFIG-FROM`). Tracking them
    would make the managers depend on node-local state and would change where the tasks of every
    such service can run. That change should be made by design, with the placement and the check
    at create in view.

.. rationale:: Live-restore on a node in a Swarm changes what a daemon restart does to its tasks
    :id: RAT-SWARM-LIVE-RESTORE-REFUSED

    The daemon refuses to combine :daemon-json:`live-restore` with Swarm mode when a node
    initializes or joins a Swarm, and when a daemon in a Swarm starts. A reload of the configuration
    that turns :daemon-json:`live-restore` on for a node in a Swarm is accepted, and the daemon then
    does not start with it. :need:`SWARM-CLUSTER-LIVE-RESTORE-REFUSED` and
    :need:`SWARM-CLUSTER-LIVE-RESTORE-NO-START` notice a change that accepts the combination, and
    :need:`SWARM-CLUSTER-LIVE-RESTORE-RELOAD` notices a change to what the reload does, which is
    part of the same decision. The refusal came with the change that made the daemon restore
    container networking at start. Older documentation said that the two are not compatible; the
    current page says only that live restore does not pertain to Swarm services. Accepting the
    combination would change what a daemon restart does to the task containers on the node: they
    would keep running across the restart instead of stopping with the daemon, so such a change
    should be deliberate.

.. rationale:: A node knows of other nodes' tasks only on networks that share records between nodes
    :id: RAT-SWARM-NODE-LOCAL-NO-CLUSTER-RECORDS

    Service names, :dns-name:`tasks.<service>` and service aliases resolve from records that the
    nodes of an overlay network share with each other. A Swarm-scoped network of a node-local driver
    shares no records between nodes: each node knows only the containers that it runs itself
    (:need:`SWARM-SD-NODE-LOCAL`). A service name that started to resolve there would need a
    definition of what it resolves to, and it would hide the same name on the upstream servers,
    which answer for it today. Which names exist on a network should change by design. For the same
    reason, a service in vip mode on such a network gets no VIP: an entry in its
    :api-field:`Endpoint.VirtualIPs` names the network without an address
    (:need:`SWARM-OVL-VIP-NODE-LOCAL`). A VIP there would need an address that every node's copy of
    the network has, while each node takes its subnet from its own config-only network or address
    pools, so giving it one should be a decision about what that address means on each node.

.. rationale:: Some fields of a Swarm-scoped network describe the node that answers, not the cluster
    :id: RAT-SWARM-NODE-REPORTS-OWN-STATE

    A node fills some fields of a Swarm-scoped network from its own state and not from the managers'
    store: :api-field:`Containers`, :api-field:`Peers` and the services of a verbose inspect from
    its own attachment to the network, and, while it has a local attachment to a network created with :api-field:`ConfigFrom`, the :api-field:`IPAM` of that network from its own config-only network. A node without that state reports the
    field empty or leaves it out. The same request therefore gets different answers on different
    nodes, and each answer describes the node that gave it. Filling one of these fields from
    cluster-wide data changes what the field means on every node. Such a change should be made on
    purpose, and with the other fields in view.

.. rationale:: The Swarm's bridge and host networks stand for each node's own network
    :id: RAT-SWARM-PREDEFINED-NETWORKS

    A service that names :network:`bridge` or :network:`host` stores the ID of one of the Swarm's
    predefined networks, and :api-op:`GET /services/{id}` and :api-op:`GET /tasks` report that
    ID. The task's container uses the node's own network of that name. The network list leaves the
    Swarm's predefined networks out, so that it shows each of :network:`bridge` and
    :network:`host` once, as the network that containers use. A lookup by ID finds them, so that
    the IDs in services and tasks resolve to a network. A lookup by the name :network:`bridge` or
    :network:`host` without :api-query:`scope` ``swarm`` finds the node's own network, which is
    what the name means on every node. Listing them would show two networks of each name on every
    manager to the tooling that reads the list. A lookup by name that matched them would make the
    names :network:`bridge` and :network:`host` ambiguous on every manager, and a lookup by ID that
    did not find them would leave those IDs without a network. Each of these changes should be made
    on purpose.

.. rationale:: A worker is given only the cluster state that its own workloads use
    :id: RAT-SWARM-WORKER-HOLDS-WHAT-IT-USES

    The managers hold the cluster object and every Swarm-scoped network. A worker has no access to
    their store. It is sent a network when a task or an attached container on it uses that network
    (:need:`REQ-SWARM-NETWORK-WORKER-ON-DEMAND`), and it is not sent the cluster object. What a
    worker's API reports follows from what the worker holds. A worker that starts to report
    :api-field:`Swarm.Cluster`, or a network that nothing on it uses, has been given more of the
    cluster's state than before. How much of that state reaches the workers is a design decision
    about the cluster, and should not change as a side effect.

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

.. needextend:: type == "expectation" and (docname in ['network/swarm-networking', 'network/swarm-traffic', 'network/swarm-lifecycle'] or "In a Swarm" in sections) and ("linux" in platforms) and "SWARM-HOST-VXLAN" not in pins
    :+needs: SWARM-HOST-VXLAN

.. hostreq:: IPv6
    :id: SWARM-HOST-IPV6
    :kind: os
    :platforms: linux
    :kernel_options: IPV6
    :tags: ipv6

    *Needed when:* A node with an IPv6 data-path address, or traffic to an IPv6 address of a node

.. hostreq:: ESP over IPv6
    :id: SWARM-HOST-ESP-IPV6
    :kind: os
    :platforms: linux
    :kernel_options: INET6_ESP, INET6_XFRM_MODE_TRANSPORT
    :tags: encrypted, ipv6

    *Needed when:* An :drvopt:`encrypted` overlay network, on a node with an IPv6 data-path address

    *Kernel options:* :kconfig:`INET6_ESP`. :kconfig:`INET6_XFRM_MODE_TRANSPORT` on kernels that
    have it.

.. hostreq:: Network namespaces, veth pairs and Linux bridges
    :id: SWARM-HOST-NETNS-VETH-BRIDGE
    :kind: os
    :platforms: linux
    :kernel_options: NET_NS, VETH, BRIDGE

    *Needed when:* Always

.. needextend:: type == "expectation" and (docname in ['network/swarm-networking', 'network/swarm-traffic', 'network/swarm-lifecycle'] or "In a Swarm" in sections) and ("linux" in platforms) and "SWARM-HOST-NETNS-VETH-BRIDGE" not in pins
    :+needs: SWARM-HOST-NETNS-VETH-BRIDGE

.. hostreq:: Netfilter hooks for bridged traffic
    :id: SWARM-HOST-BR-NETFILTER
    :kind: os
    :platforms: linux
    :kernel_options: BRIDGE_NETFILTER
    :when: iptables

    *Facility:* Netfilter hooks for bridged traffic (``br_netfilter``)

    *Needed when:* The daemon option :daemon-json:`iptables` true

.. needextend:: type == "expectation" and (docname in ['network/swarm-networking', 'network/swarm-traffic', 'network/swarm-lifecycle'] or "In a Swarm" in sections) and ("linux" in platforms) and "SWARM-HOST-BR-NETFILTER" not in pins
    :+needs: SWARM-HOST-BR-NETFILTER

.. hostreq:: IPVS with firewall marks, round-robin and connection tracking
    :id: SWARM-HOST-IPVS
    :kind: os
    :platforms: linux
    :kernel_options: IP_VS, IP_VS_RR, IP_VS_NFCT, IP_VS_PROTO_TCP, IP_VS_PROTO_UDP
    :when: iptables-backend

    *Facility:* IPVS with firewall-mark virtual services, the round-robin scheduler and connection
    tracking

    *Needed when:* The iptables backend, and traffic to a VIP or to a port that a service publishes
    in ingress mode

.. hostreq:: IPVS for SCTP
    :id: SWARM-HOST-IPVS-SCTP
    :kind: os
    :platforms: linux
    :kernel_options: IP_VS_PROTO_SCTP
    :when: iptables-backend
    :tags: sctp

    *Facility:* IPVS load balancing of SCTP

    *Needed when:* The iptables backend, and SCTP traffic to a VIP or to a port that a service
    publishes in ingress mode

    *Kernel options:* :kconfig:`IP_VS_PROTO_SCTP`

.. hostreq:: nftables with route chains
    :id: SWARM-HOST-NFTABLES-ROUTE-CHAINS
    :kind: os
    :platforms: linux
    :kernel_options: NF_TABLES, NF_TABLES_IPV4, NF_TABLES_IPV6, NFT_CHAIN_ROUTE_IPV4, NFT_CHAIN_ROUTE_IPV6
    :when: nftables-backend

    *Needed when:* The nftables backend

    *Kernel options:* :kconfig:`NF_TABLES`, :kconfig:`NF_TABLES_IPV4`, :kconfig:`NF_TABLES_IPV6`.
    Before Linux 5.2, :kconfig:`NFT_CHAIN_ROUTE_IPV4` and :kconfig:`NFT_CHAIN_ROUTE_IPV6`.

.. needextend:: type == "expectation" and (docname in ['network/swarm-networking', 'network/swarm-traffic', 'network/swarm-lifecycle'] or "In a Swarm" in sections) and ("linux" in platforms) and "SWARM-HOST-NFTABLES-ROUTE-CHAINS" not in pins
    :+needs: SWARM-HOST-NFTABLES-ROUTE-CHAINS

.. hostreq:: nftables with NAT, reject, numgen and concatenated interval maps
    :id: SWARM-HOST-NFTABLES-NAT
    :kind: os
    :platforms: linux
    :kernel_options: NF_TABLES, NF_TABLES_IPV4, NFT_NAT, NFT_MASQ, NFT_REDIR, NFT_REJECT, NFT_REJECT_IPV4, NFT_CT, NFT_NUMGEN, NFT_MASQ_IPV4, NFT_REDIR_IPV4
    :when: nftables-backend

    *Facility:* nftables with the NAT, masquerade, redirect, reject, connection-tracking and
    random-number expressions, and maps keyed on concatenations with intervals

    *Needed when:* The nftables backend, and traffic to a VIP or to a port that a service publishes
    in ingress mode

    *Kernel options:* :kconfig:`NF_TABLES`, :kconfig:`NF_TABLES_IPV4`, :kconfig:`NFT_NAT`,
    :kconfig:`NFT_MASQ`, :kconfig:`NFT_REDIR`, :kconfig:`NFT_REJECT`, :kconfig:`NFT_REJECT_IPV4`,
    :kconfig:`NFT_CT`, :kconfig:`NFT_NUMGEN`. :kconfig:`NFT_MASQ_IPV4` and :kconfig:`NFT_REDIR_IPV4`
    on kernels that have them. Maps keyed on concatenations with intervals are part of
    :kconfig:`NF_TABLES` from Linux 5.6.

.. hostreq:: nftables netdev tables with ingress and egress hooks
    :id: SWARM-HOST-NFTABLES-NETDEV
    :kind: os
    :platforms: linux
    :kernel_options: NF_TABLES_NETDEV, NETFILTER_INGRESS, NETFILTER_EGRESS, NFT_FWD_NETDEV, NFT_CT
    :when: nftables-backend
    :tags: dsr

    *Facility:* nftables netdev tables with the ingress and egress hooks and the forward expression

    *Needed when:* The nftables backend, and an overlay network with the :drvopt:`dsr` option, on
    the node of a client that sends traffic to a VIP on that network

    *Kernel options:* :kconfig:`NF_TABLES_NETDEV`, :kconfig:`NETFILTER_INGRESS`,
    :kconfig:`NETFILTER_EGRESS`, :kconfig:`NFT_FWD_NETDEV`, :kconfig:`NFT_CT`.
    :kconfig:`NETFILTER_EGRESS` from Linux 5.16.

.. hostreq:: SCTP
    :id: SWARM-HOST-SCTP
    :kind: os
    :platforms: linux
    :kernel_options: IP_SCTP, NF_CT_PROTO_SCTP
    :tags: sctp

    *Facility:* SCTP

    *Needed when:* A container that sends or receives SCTP on an overlay network, or a service that
    publishes an SCTP port

    *Kernel options:* :kconfig:`IP_SCTP`. :kconfig:`NF_CT_PROTO_SCTP` on kernels that have it.

.. _swarm-networking-programs:

Programs
========

.. hostreq:: iptables with the tables, targets and matches it needs
    :id: SWARM-HOST-IPTABLES-COMMAND
    :kind: program
    :platforms: linux
    :when: iptables-backend

    *Facility:* :program:`iptables`, accepting the filter, mangle and nat tables, the
    :ipt-target:`MARK`, :ipt-target:`DNAT`, :ipt-target:`SNAT` and :ipt-target:`REDIRECT` targets,
    and the ``ipvs``, ``conntrack``, ``mark``, ``bpf``, ``policy`` and ``owner`` matches

    *Needed when:* The iptables backend, and traffic to a VIP or to a port that a service publishes
    in ingress mode, or an :drvopt:`encrypted` overlay network

.. hostreq:: nft accepting concatenated interval maps and numgen
    :id: SWARM-HOST-NFT-COMMAND
    :kind: program
    :platforms: linux
    :when: nftables-backend

    *Facility:* :program:`nft`, accepting sets and maps keyed on concatenations with intervals, and
    the ``numgen`` expression

    *Needed when:* The nftables backend, on a daemon built without the ``libnftables`` build tag,
    and traffic to a VIP or to a port that a service publishes in ingress mode. A daemon built with
    it uses the libnftables library and runs no :program:`nft` command.

.. hostreq:: nft accepting netdev tables with the egress hook
    :id: SWARM-HOST-NFT-NETDEV-EGRESS-COMMAND
    :kind: program
    :platforms: linux
    :when: nftables-backend
    :tags: dsr

    *Facility:* :program:`nft`, accepting netdev tables with the egress hook

    *Needed when:* The nftables backend, and an overlay network with the :drvopt:`dsr` option, on
    the node of a client that sends traffic to a VIP on that network, on a daemon built without the
    ``libnftables`` build tag

.. hostreq:: iptables under the nftables backend
    :id: SWARM-HOST-IPTABLES-CLEANUP-COMMAND
    :kind: program
    :platforms: linux
    :when: nftables-backend

    *Facility:* :program:`iptables`, and :program:`ip6tables` on a node with an IPv6 data-path
    address, accepting the filter, nat and mangle tables

    *Needed when:* The nftables backend, on a node with chains or rules that a daemon left in those
    tables under the iptables backend

.. hostreq:: nft under the iptables backend
    :id: SWARM-HOST-NFT-CLEANUP-COMMAND
    :kind: program
    :platforms: linux
    :when: iptables-backend

    *Facility:* :program:`nft`

    *Needed when:* The iptables backend, on a node with a :nft-table:`docker-overlay` table that a
    daemon left under the nftables backend, on a daemon built without the ``libnftables`` build tag.
    A daemon built with it uses the libnftables library and runs no :program:`nft` command.

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
    :kernel_options: NETFILTER, NETFILTER_ADVANCED, NETFILTER_XTABLES, NF_CONNTRACK, NF_NAT, NETFILTER_XT_NAT, NETFILTER_XT_TARGET_MASQUERADE, NETFILTER_XT_MATCH_ADDRTYPE, NETFILTER_XT_MATCH_CONNTRACK, IP_NF_NAT, IP_NF_TARGET_MASQUERADE
    :when: iptables-backend

    *Needed when:* The iptables backend

    The kernel options that :program:`iptables` needs, whichever implementation provides it. *Kernel
    options:* :kconfig:`NETFILTER`, :kconfig:`NETFILTER_ADVANCED`, :kconfig:`NETFILTER_XTABLES`,
    :kconfig:`NF_CONNTRACK`, :kconfig:`NF_NAT`, :kconfig:`NETFILTER_XT_NAT`,
    :kconfig:`NETFILTER_XT_TARGET_MASQUERADE`, :kconfig:`NETFILTER_XT_MATCH_ADDRTYPE`,
    :kconfig:`NETFILTER_XT_MATCH_CONNTRACK`. Before Linux 5.2, :kconfig:`IP_NF_NAT` and
    :kconfig:`IP_NF_TARGET_MASQUERADE` in place of :kconfig:`NETFILTER_XT_TARGET_MASQUERADE`.

.. needextend:: type == "expectation" and (docname in ['network/swarm-networking', 'network/swarm-traffic', 'network/swarm-lifecycle'] or "In a Swarm" in sections) and ("linux" in platforms) and "SWARM-HOST-IPTABLES-KERNEL" not in pins
    :+needs: SWARM-HOST-IPTABLES-KERNEL

.. hostreq:: Kernel options for iptables from iptables-legacy
    :id: SWARM-HOST-IPTABLES-LEGACY-KERNEL
    :kind: os
    :platforms: linux
    :kernel_options: IP_NF_IPTABLES, IP_NF_FILTER, IP_NF_NAT, IP_NF_RAW, IP_NF_IPTABLES_LEGACY, NETFILTER_XTABLES_LEGACY
    :when: iptables-backend, iptables-legacy

    *Needed when:* The iptables backend with iptables-legacy providing :program:`iptables`

    The kernel options that :program:`iptables` needs when iptables-legacy provides it. *Kernel
    options:* :kconfig:`IP_NF_IPTABLES`, :kconfig:`IP_NF_FILTER`, :kconfig:`IP_NF_NAT`,
    :kconfig:`IP_NF_RAW`. :kconfig:`IP_NF_IPTABLES_LEGACY` from Linux 6.9, and
    :kconfig:`NETFILTER_XTABLES_LEGACY` from Linux 6.17.

.. needextend:: type == "expectation" and (docname in ['network/swarm-networking', 'network/swarm-traffic', 'network/swarm-lifecycle'] or "In a Swarm" in sections) and ("linux" in platforms) and "SWARM-HOST-IPTABLES-LEGACY-KERNEL" not in pins
    :+needs: SWARM-HOST-IPTABLES-LEGACY-KERNEL

.. hostreq:: The mangle table from iptables-legacy
    :id: SWARM-HOST-IPTABLES-LEGACY-MANGLE-KERNEL
    :kind: os
    :platforms: linux
    :kernel_options: IP_NF_MANGLE
    :when: iptables-backend, iptables-legacy

    *Facility:* The mangle table of :program:`iptables` when iptables-legacy provides it

    *Needed when:* The iptables backend with iptables-legacy, and traffic to a VIP or to a port that
    a service publishes in ingress mode, or an :drvopt:`encrypted` overlay network

    *Kernel options:* :kconfig:`IP_NF_MANGLE`

.. hostreq:: Kernel options for iptables from iptables-nft
    :id: SWARM-HOST-IPTABLES-NFT-KERNEL
    :kind: os
    :platforms: linux
    :kernel_options: NF_TABLES, NF_TABLES_IPV4, NFT_COMPAT, NFT_NAT, NFT_CHAIN_NAT_IPV4
    :when: iptables-backend, iptables-nft

    *Needed when:* The iptables backend with iptables-nft providing :program:`iptables`

    The kernel options that :program:`iptables` needs when iptables-nft provides it. *Kernel
    options:* :kconfig:`NF_TABLES`, :kconfig:`NF_TABLES_IPV4`, :kconfig:`NFT_COMPAT`,
    :kconfig:`NFT_NAT`. Before Linux 5.1, :kconfig:`NFT_CHAIN_NAT_IPV4` in place of
    :kconfig:`NFT_NAT`.

.. needextend:: type == "expectation" and (docname in ['network/swarm-networking', 'network/swarm-traffic', 'network/swarm-lifecycle'] or "In a Swarm" in sections) and ("linux" in platforms) and "SWARM-HOST-IPTABLES-NFT-KERNEL" not in pins
    :+needs: SWARM-HOST-IPTABLES-NFT-KERNEL

.. hostreq:: The mangle OUTPUT chain from iptables-nft
    :id: SWARM-HOST-IPTABLES-NFT-ROUTE-KERNEL
    :kind: os
    :platforms: linux
    :kernel_options: NFT_CHAIN_ROUTE_IPV4
    :when: iptables-backend, iptables-nft
    :tags: encrypted

    *Facility:* The :ipt-chain:`OUTPUT` chain of the mangle table of :program:`iptables` when
    iptables-nft provides it

    *Needed when:* The iptables backend with iptables-nft, and an :drvopt:`encrypted` overlay
    network

    *Kernel options:* Before Linux 5.2, :kconfig:`NFT_CHAIN_ROUTE_IPV4`

.. hostreq:: The iptables sctp match
    :id: SWARM-HOST-IPTABLES-SCTP-MATCH
    :kind: os
    :platforms: linux
    :kernel_options: NETFILTER_XT_MATCH_SCTP
    :when: iptables-backend
    :tags: sctp

    *Facility:* The ``sctp`` match of :program:`iptables`

    *Needed when:* The iptables backend, and a service that publishes an SCTP port

    *Kernel options:* :kconfig:`NETFILTER_XT_MATCH_SCTP`

.. hostreq:: The iptables bpf, policy, owner and mark extensions
    :id: SWARM-HOST-IPTABLES-ENC-MATCHES
    :kind: os
    :platforms: linux
    :kernel_options: NETFILTER_XT_MATCH_BPF, NETFILTER_XT_MATCH_POLICY, NETFILTER_XT_MATCH_OWNER, NETFILTER_XT_MARK
    :when: iptables-backend
    :tags: encrypted

    *Facility:* The ``bpf``, ``policy``, ``owner`` and ``mark`` matches and the :ipt-target:`MARK`
    target of :program:`iptables`

    *Needed when:* The iptables backend, and an :drvopt:`encrypted` overlay network

    *Kernel options:* :kconfig:`NETFILTER_XT_MATCH_BPF`, :kconfig:`NETFILTER_XT_MATCH_POLICY`,
    :kconfig:`NETFILTER_XT_MATCH_OWNER`, :kconfig:`NETFILTER_XT_MARK`

.. hostreq:: The iptables ipvs match and MARK and REDIRECT targets
    :id: SWARM-HOST-IPTABLES-LB-EXTENSIONS
    :kind: os
    :platforms: linux
    :kernel_options: NETFILTER_XT_MATCH_IPVS, NETFILTER_XT_MARK, NETFILTER_XT_TARGET_REDIRECT
    :when: iptables-backend

    *Facility:* The ``ipvs`` match and the :ipt-target:`MARK` and :ipt-target:`REDIRECT` targets of
    :program:`iptables`

    *Needed when:* The iptables backend, and traffic to a VIP or to a port that a service publishes
    in ingress mode

    *Kernel options:* :kconfig:`NETFILTER_XT_MATCH_IPVS`, :kconfig:`NETFILTER_XT_MARK`,
    :kconfig:`NETFILTER_XT_TARGET_REDIRECT`

.. hostreq:: Kernel options for ip6tables
    :id: SWARM-HOST-IP6TABLES-KERNEL
    :kind: os
    :platforms: linux
    :kernel_options: NETFILTER, NETFILTER_ADVANCED, NETFILTER_XTABLES, NETFILTER_XT_MARK, NETFILTER_XT_MATCH_BPF, NETFILTER_XT_MATCH_POLICY, NETFILTER_XT_MATCH_OWNER
    :when: iptables-backend
    :tags: encrypted, ipv6

    *Needed when:* The iptables backend, and an :drvopt:`encrypted` overlay network on a node with
    an IPv6 data-path address

    The kernel options that :program:`ip6tables` needs, whichever implementation provides it.
    *Kernel options:* :kconfig:`NETFILTER`, :kconfig:`NETFILTER_ADVANCED`,
    :kconfig:`NETFILTER_XTABLES`, :kconfig:`NETFILTER_XT_MARK`, :kconfig:`NETFILTER_XT_MATCH_BPF`,
    :kconfig:`NETFILTER_XT_MATCH_POLICY`, :kconfig:`NETFILTER_XT_MATCH_OWNER`

.. hostreq:: Kernel options for ip6tables from iptables-legacy
    :id: SWARM-HOST-IP6TABLES-LEGACY-KERNEL
    :kind: os
    :platforms: linux
    :kernel_options: IP6_NF_IPTABLES, IP6_NF_FILTER, IP6_NF_MANGLE, IP6_NF_IPTABLES_LEGACY, NETFILTER_XTABLES_LEGACY
    :when: iptables-backend, iptables-legacy
    :tags: encrypted, ipv6

    *Needed when:* The iptables backend with iptables-legacy, and an :drvopt:`encrypted` overlay
    network on a node with an IPv6 data-path address

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
    :tags: encrypted, ipv6

    *Needed when:* The iptables backend with iptables-nft, and an :drvopt:`encrypted` overlay
    network on a node with an IPv6 data-path address

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

.. needextend:: type == "expectation" and (docname in ['network/swarm-networking', 'network/swarm-traffic', 'network/swarm-lifecycle'] or "In a Swarm" in sections) and ("windows" in platforms) and "SWARM-HOST-HNS-OVERLAY" not in pins
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

    *Facility:* Each node's firewall and the physical network between nodes pass TCP to the Swarm
    management port of each manager, 2377 unless its :api-field:`ListenAddr` names another, TCP and
    UDP on port 7946, and UDP on the data-path port, 4789 unless :api-op:`POST /swarm/init` set
    another. On a Windows node, the data-path port is always 4789.

    *Needed when:* A node in a Swarm

.. needextend:: type == "expectation" and (docname in ['network/swarm-networking', 'network/swarm-traffic', 'network/swarm-lifecycle'] or "In a Swarm" in sections) and ("linux" in platforms or "windows" in platforms) and "SWARM-HOST-FIREWALL-CLUSTER-PORTS" not in pins
    :+needs: SWARM-HOST-FIREWALL-CLUSTER-PORTS

.. hostreq:: Firewalls pass IP protocol 50 (ESP)
    :id: SWARM-HOST-FIREWALL-ESP
    :kind: config
    :platforms: linux
    :tags: encrypted

    *Facility:* Each node's firewall and the physical network between nodes pass IP protocol 50
    (ESP)

    *Needed when:* An :drvopt:`encrypted` overlay network

.. _swarm-networking-interop-premise:

*****************
 Interop premise
*****************

Users upgrade a Swarm node by node, so nodes of different releases share overlay and ingress
networks during a rollout (:need:`REQ-SWARM-MIXED-VERSION-INTEROP`).

.. _swarm-networking-cluster-node-configuration:

******************************
 Cluster & node configuration
******************************

These expectations cover cluster configuration. The accepted range of the data-path port is 1024 to
49151. A data-path port of 0 is also accepted, and selects the default port 4789. In these
expectations, a data-path port outside the accepted range is a port other than 0 that is less than
1024 or greater than 49151. The reference for :cli:`docker swarm init --data-path-port` documents
this range and the default. :api-field:`AdvertiseAddr`, :api-field:`DataPathAddr` and
:api-field:`ListenAddr` in :api-op:`POST /swarm/init` or :api-op:`POST /swarm/join` can name a
network interface in place of an IP address. An expectation about such a field holds for an
interface that has at most one IPv4 address and at most one IPv6 address, link-local addresses
included. The address of such an interface is its IPv4 address when it has one, and its IPv6 address
otherwise. The host part of a listen address that names an interface is that address. With the
default :api-field:`ListenAddr`, ``0.0.0.0:2377``, the host part is ``0.0.0.0``, and a node that
listens on it listens on every IPv4 and IPv6 address of the node.

.. expectation:: DataPathAddr sets the address peers send the node's VXLAN traffic to
    :id: SWARM-CLUSTER-DATA-PATH-ADDRESS
    :observed_from: underlay (outer destination address)
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-SWARM-CLUSTER-DATA-PATH-ADDRESS
    :within: SWARM-CEIL-REJOIN, SWARM-CEIL-REASSIGNMENT

    :api-field:`DataPathAddr` in :api-op:`POST /swarm/init` or :api-op:`POST /swarm/join` sets the
    address peers send the node's VXLAN traffic to.

.. expectation:: Without DataPathAddr, peers send VXLAN traffic to the advertise address
    :id: SWARM-CLUSTER-DATA-PATH-ADDRESS-DEFAULT
    :observed_from: underlay (outer destination address)
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-CLUSTER-DATA-PATH-ADDRESS
    :within: SWARM-CEIL-REJOIN, SWARM-CEIL-REASSIGNMENT

    Without :api-field:`DataPathAddr`, peers send the node's VXLAN traffic to its advertise address.

.. expectation:: Peers keep sending to the address a DataPathAddr interface first had
    :id: SWARM-CLUSTER-DATA-PATH-ADDRESS-IFACE
    :observed_from: underlay (outer destination address)
    :depends_on: none
    :platforms: linux
    :pins: SWARM-CLUSTER-IFACE-NAME-RESOLVED-ONCE
    :within: SWARM-CEIL-REJOIN, SWARM-CEIL-REASSIGNMENT

    :api-field:`DataPathAddr` in :api-op:`POST /swarm/init` or :api-op:`POST /swarm/join` names a
    network interface. Peers then send the node's VXLAN traffic to the address that the interface
    had when the request was made. They continue to send it to that address after the interface's
    address changes, and after the daemon restarts.

.. expectation:: A Windows node's tunnel endpoint is its HNS management address, not DataPathAddr
    :id: SWARM-CLUSTER-WINDOWS-DATA-PATH-ADDRESS
    :observed_from: underlay
    :depends_on: none
    :platforms: windows
    :within: SWARM-CEIL-REJOIN, SWARM-CEIL-REASSIGNMENT
    :narrows: SWARM-CLUSTER-DATA-PATH-ADDRESS-DEFAULT
    :concerns: SWARM-CLUSTER-DATA-PATH-ADDRESS-IFACE, SWARM-CLUSTER-DATA-PATH-ADDRESS
    :violates: REQ-SWARM-CLUSTER-DATA-PATH-ADDRESS

    *Applies to:* A Windows node with a task on an overlay network, whose :api-field:`DataPathAddr`,
    or advertise address when it has none, is not the management address (``ManagementIP``) that HNS
    reports for the node's HNS network of that overlay

    The node's tunnel endpoint for the network is that management address, whatever
    :api-field:`DataPathAddr` or the advertise address says: the other nodes send the node's VXLAN
    traffic for the network to it.

.. expectation:: With both given as interfaces, a manager is reached at the advertise one and VXLAN at the data-path one
    :id: SWARM-CLUSTER-ADVERTISE-ADDRESS-IFACE
    :observed_from: underlay (destination address)
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-SWARM-CLUSTER-DATA-PATH-ADDRESS
    :within: SWARM-CEIL-REJOIN, SWARM-CEIL-REASSIGNMENT

    *Applies to:* A Linux node whose :api-op:`POST /swarm/init` or :api-op:`POST /swarm/join` named
    one interface in :api-field:`AdvertiseAddr` and another in :api-field:`DataPathAddr`

    When the node is a manager, the other nodes connect to it for Swarm management, on TCP port 2377
    or the port that :api-field:`AdvertiseAddr` or :api-field:`ListenAddr` names, at the address of
    the :api-field:`AdvertiseAddr` interface. The other nodes send the node's VXLAN traffic to the
    address of the :api-field:`DataPathAddr` interface.

.. expectation:: A DataPathAddr interface is resolved to an address once, at init or join
    :id: SWARM-CLUSTER-IFACE-NAME-RESOLVED-ONCE
    :observed_from: underlay (outer destination address)
    :depends_on: none
    :platforms: linux
    :within: SWARM-CEIL-REJOIN, SWARM-CEIL-REASSIGNMENT
    :narrows: SWARM-CLUSTER-DATA-PATH-ADDRESS, SWARM-CLUSTER-ADVERTISE-ADDRESS-IFACE
    :concerns: SWARM-CLUSTER-DATA-PATH-ADDRESS-IFACE
    :violates: REQ-SWARM-CLUSTER-DATA-PATH-ADDRESS

    :api-op:`POST /swarm/init` and :api-op:`POST /swarm/join` with :api-field:`DataPathAddr` naming
    a network interface replace the name with the interface's address when the request is made.
    Peers keep sending the node's VXLAN traffic to that address after the interface's address
    changes, and after the daemon restarts, even when no interface on the node has that address.

.. expectation:: VXLAN traffic's outer source address is the one the route selects
    :id: SWARM-CLUSTER-VXLAN-SOURCE-ADDRESS
    :observed_from: underlay (outer source address)
    :depends_on: os-linux
    :platforms: linux
    :pins: SWARM-CLUSTER-VXLAN-SOURCE-NOT-DATA-PATH
    :within: SWARM-CEIL-REASSIGNMENT

    A node's VXLAN traffic to a peer has the outer source address that the node's route to that peer
    selects. This holds whether or not that address is the node's data-path address.

.. expectation:: VXLAN leaves from the routed source address, not the data-path address
    :id: SWARM-CLUSTER-VXLAN-SOURCE-NOT-DATA-PATH
    :observed_from: underlay, task, log
    :depends_on: os-linux
    :platforms: linux
    :within: SWARM-CEIL-REASSIGNMENT
    :concerns: SWARM-CLUSTER-VXLAN-SOURCE-ADDRESS, SWARM-CLUSTER-DATA-PATH-ADDRESS
    :violates: REQ-SWARM-OVERLAY-CONNECTIVITY

    The outer source address of a node's VXLAN traffic to a peer is the address that the node's
    route to that peer selects, whether or not it is the node's data-path address. When it is
    another address, and no one-to-one NAT between the nodes maps it to the node's data-path
    address, overlay traffic from that node does not reach the tasks on the peer, and neither node's
    daemon log has an error (`moby/moby#53004 <https://github.com/moby/moby/issues/53004>`__).

.. expectation:: DataPathPort in POST /swarm/init sets the cluster's VXLAN UDP port
    :id: SWARM-CLUSTER-DATA-PATH-PORT
    :observed_from: underlay
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-SWARM-CLUSTER-DATA-PATH-PORT
    :within: SWARM-CEIL-REJOIN, SWARM-CEIL-REASSIGNMENT

    :api-field:`DataPathPort` in :api-op:`POST /swarm/init` sets the VXLAN UDP port of every Linux
    node of the cluster.

.. expectation:: Linux and Windows nodes exchange overlay traffic on the default port
    :id: SWARM-CLUSTER-DATA-PATH-PORT-DEFAULT
    :observed_from: task
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-CLUSTER-DATA-PATH-PORT
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A Swarm with the default data-path port

    Linux and Windows nodes in that Swarm exchange overlay traffic.

.. expectation:: Windows nodes ignore a non-default data-path port
    :id: SWARM-CLUSTER-WINDOWS-DATA-PATH-PORT
    :observed_from: underlay, task
    :depends_on: none
    :platforms: windows
    :within: SWARM-CEIL-REJOIN, SWARM-CEIL-REASSIGNMENT
    :narrows: SWARM-WIRECLEAR-UDP-PORT
    :concerns: SWARM-CLUSTER-DATA-PATH-PORT-DEFAULT, SWARM-CLUSTER-DATA-PATH-PORT
    :violates: REQ-SWARM-CLUSTER-DATA-PATH-PORT, REQ-SWARM-DATA-PATH-VXLAN, REQ-SWARM-OVERLAY-CONNECTIVITY

    In a cluster with a non-default data-path port, Windows nodes do not use that port, and overlay
    traffic between a task on a Windows node and a task on a Linux node does not reach its
    destination.

.. expectation:: docker swarm init refuses a data-path port outside the accepted range
    :id: SWARM-CLUSTER-DATA-PATH-PORT-INVALID-CLI
    :observed_from: api:solo
    :depends_on: cli
    :platforms: linux
    :refines: REQ-SWARM-CLUSTER-DATA-PATH-PORT
    :cli_for: SWARM-CLUSTER-DATA-PATH-PORT-INVALID

    *Applies to:* A node that is not yet in a Swarm

    :cli:`docker swarm init` with a data-path port outside the accepted range exits non-zero. The
    node is then still not in a Swarm.

.. expectation:: POST /swarm/init refuses a data-path port outside the accepted range
    :id: SWARM-CLUSTER-DATA-PATH-PORT-INVALID
    :observed_from: api:solo
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-CLUSTER-DATA-PATH-PORT

    *Applies to:* A node that is not yet in a Swarm

    :api-op:`POST /swarm/init` with a data-path port outside the accepted range responds with a 4xx
    or 5xx status. The node is then still not in a Swarm.

.. expectation:: A data-path port outside the accepted range responds 500
    :id: SWARM-CLUSTER-DATA-PATH-PORT-INVALID-HTTP500
    :observed_from: api:solo
    :depends_on: none
    :platforms: linux
    :exact_form_of: SWARM-CLUSTER-DATA-PATH-PORT-INVALID
    :pins: SWARM-CLUSTER-HTTP500-REFUSED

    *Applies to:* A node that is not yet in a Swarm

    :api-op:`POST /swarm/init` with a data-path port outside the accepted range responds 500.

.. expectation:: A data-path port of 0 in POST /swarm/init selects 4789
    :id: SWARM-CLUSTER-DATA-PATH-PORT-ZERO
    :observed_from: api:solo, api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-CLUSTER-DATA-PATH-PORT
    :within: SWARM-CEIL-REJOIN

    *Applies to:* A node that is not yet in a Swarm

    :api-op:`POST /swarm/init` with a data-path port of 0 responds 200. :api-op:`GET /info` on the
    node then reports 4789 as the data-path port in :api-field:`Swarm.Cluster`.

.. expectation:: An overlay without a subnet gets one of SubnetSize from DefaultAddrPool
    :id: SWARM-CLUSTER-ADDRESS-POOL-ALLOCATION
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-CLUSTER-ADDRESS-POOLS
    :within: SWARM-CEIL-ALLOCATION

    *Applies to:* An overlay network, the ingress network included, that uses the default IPAM
    driver and is created without a subnet

    Once the network is allocated, as :ref:`swarm-networking-network-lifecycle-api-surface` defines
    it, :api-op:`GET /networks/{id}` on a manager reports a subnet for it. That subnet is inside one
    of the :api-field:`DefaultAddrPool` prefixes that :api-op:`GET /info` on a manager reports in
    :api-field:`Swarm.Cluster`. The prefix length of that subnet is the :api-field:`SubnetSize` that
    :api-op:`GET /info` reports.

.. expectation:: docker swarm init refuses a mask length over 29 or below a pool's prefix
    :id: SWARM-CLUSTER-SUBNET-SIZE-INVALID-CLI
    :observed_from: api:solo
    :depends_on: cli
    :platforms: linux
    :refines: REQ-SWARM-CLUSTER-ADDRESS-POOLS
    :cli_for: SWARM-CLUSTER-SUBNET-SIZE-INVALID

    *Applies to:* A node that is not yet in a Swarm

    :cli:`docker swarm init` with :cli-flag:`--default-addr-pool` exits non-zero when
    :cli-flag:`--default-addr-pool-mask-length` is greater than 29. It also exits non-zero when that
    mask length is less than the prefix length of one of the pools, or is 0 and one of the pools has a prefix longer than 24 bits. In both cases, the
    node is then still not in a Swarm.

.. expectation:: POST /swarm/init refuses a SubnetSize over 29 or below a pool's prefix
    :id: SWARM-CLUSTER-SUBNET-SIZE-INVALID
    :observed_from: api:solo
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-CLUSTER-ADDRESS-POOLS

    *Applies to:* A node that is not yet in a Swarm

    :api-op:`POST /swarm/init` with :api-field:`DefaultAddrPool` set responds with a 4xx or 5xx
    status when the request's :api-field:`SubnetSize` is greater than 29. It also responds with a
    4xx or 5xx status when the request's :api-field:`SubnetSize` is less than the prefix length of one of the pools, or is 0 and one of the pools has a prefix longer than 24 bits. In both cases, the node is then still not in a Swarm.

.. expectation:: A SubnetSize over 29 or below a pool's prefix responds 500
    :id: SWARM-CLUSTER-SUBNET-SIZE-INVALID-HTTP500
    :observed_from: api:solo
    :depends_on: none
    :platforms: linux
    :exact_form_of: SWARM-CLUSTER-SUBNET-SIZE-INVALID
    :pins: SWARM-CLUSTER-HTTP500-REFUSED

    *Applies to:* A node that is not yet in a Swarm

    :api-op:`POST /swarm/init` with :api-field:`DefaultAddrPool` set responds 500 when the request's
    :api-field:`SubnetSize` is greater than 29. It also responds 500 when the request's
    :api-field:`SubnetSize` is less than the prefix length of one of the pools, or is 0 and one of the pools has a prefix longer than 24 bits.

.. expectation:: Some refused Swarm, start and connect requests respond 500
    :id: SWARM-CLUSTER-HTTP500-REFUSED
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows
    :concerns: SWARM-CLUSTER-DATA-PATH-PORT-INVALID-HTTP500, SWARM-CLUSTER-SUBNET-SIZE-INVALID-HTTP500, SWARM-SCOPED-NOT-ATTACHABLE-START-HTTP500, SWARM-SCOPED-NOT-ATTACHABLE-CONNECT-HTTP500, SWARM-PORTALLOC-CTR-START-HTTP500, SWARM-CHURN-DELETE-CTR-START-REFUSED-HTTP500, NET-API-CONNECT-MISSING-SWARM-START-FAILS
    :violates: REQ-NET-API-ERROR-STATUS

    Each request below is refused, and responds 500: (1) :api-op:`POST /swarm/init` with a data-path
    port outside the accepted range. (2) :api-op:`POST /swarm/init` with
    :api-field:`DefaultAddrPool` set and a :api-field:`SubnetSize` greater than 29, or with a :api-field:`SubnetSize` less than the prefix length of one of its :api-field:`DefaultAddrPool` prefixes, or of 0 with one of those prefixes longer than 24 bits. (3) :api-op:`POST /containers/{id}/start` for a created
    or stopped container on a non-attachable Swarm-scoped network, and :api-op:`POST
    /networks/{id}/connect` to such a network for a running container. (4) On a Linux node,
    :api-op:`POST /containers/{id}/start` for a created or stopped container that publishes a port
    that a service publishes in ingress mode. (5) On a node in a Swarm, :api-op:`POST
    /containers/{id}/start` for a created or stopped container whose network was deleted after the
    container was created, or never existed.

.. expectation:: Without a pool, POST /swarm/init accepts and drops SubnetSize
    :id: SWARM-CLUSTER-SUBNET-SIZE-WITHOUT-POOL-DROPPED
    :observed_from: api:solo, api:mgr
    :depends_on: none
    :platforms: linux
    :within: SWARM-CEIL-REJOIN, SWARM-CEIL-ALLOCATION
    :concerns: SWARM-CLUSTER-SUBNET-SIZE-INVALID, SWARM-CLUSTER-INFO-MANAGER-DEFAULT-POOL, SWARM-CLUSTER-INFO-MANAGER-DEFAULT-POOL-CLI
    :violates: REQ-SWARM-CLUSTER-ADDRESS-POOLS

    *Applies to:* A node that is not yet in a Swarm

    :api-op:`POST /swarm/init` without :api-field:`DefaultAddrPool` and with a
    :api-field:`SubnetSize` other than 0 responds 200, also when the size is greater than 29.
    :api-op:`GET /info` on the node then reports a :api-field:`SubnetSize` of 24 in
    :api-field:`Swarm.Cluster`. An overlay network created later without a subnet gets a subnet with
    a prefix length of 24.

.. expectation:: POST /swarm/init accepts a SubnetSize of 0, and GET /info reports 0
    :id: SWARM-CLUSTER-SUBNET-SIZE-ZERO
    :observed_from: api:solo, api:mgr
    :depends_on: none
    :platforms: linux
    :pins: SWARM-CLUSTER-SUBNET-SIZE-ZERO-UNALLOCATED
    :within: SWARM-CEIL-REJOIN

    *Applies to:* A node that is not yet in a Swarm

    :api-op:`POST /swarm/init` with :api-field:`DefaultAddrPool` set, with no prefix longer than 24 bits, and a :api-field:`SubnetSize` of 0 responds 200. :api-op:`GET /info` on the node then reports a :api-field:`SubnetSize` of 0
    in :api-field:`Swarm.Cluster`.

.. expectation:: A SubnetSize of 0 with a pool is accepted and nothing is allocated
    :id: SWARM-CLUSTER-SUBNET-SIZE-ZERO-UNALLOCATED
    :observed_from: api:solo, api:mgr, log
    :depends_on: none
    :platforms: linux
    :within: SWARM-CEIL-REJOIN
    :concerns: SWARM-CLUSTER-SUBNET-SIZE-ZERO, SWARM-CLUSTER-SUBNET-SIZE-INVALID, SWARM-CLUSTER-ADDRESS-POOL-ALLOCATION
    :violates: REQ-SWARM-CLUSTER-ADDRESS-POOLS, PRIN-ERRORS-ACTIONABLE

    :api-op:`POST /swarm/init` with :api-field:`DefaultAddrPool` set, with no prefix longer than 24 bits, and a :api-field:`SubnetSize` of 0 responds 200. No Swarm-scoped network is allocated, the ingress network included. Only the
    leader manager's log reports this, in an entry that contains :log:`failed to create allocator`.
    The entry's error says that the network at index 0 :log:`is not in canonical form`, and does not
    name :api-field:`SubnetSize`.

.. expectation:: A manager's GET /info reports the data-path port, pool and subnet size
    :id: SWARM-CLUSTER-INFO-MANAGER
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-CLUSTER-ADDRESS-POOLS, REQ-SWARM-CLUSTER-DATA-PATH-PORT
    :within: SWARM-CEIL-REJOIN

    On a manager, :api-op:`GET /info` includes :api-field:`Swarm.Cluster`.
    :api-field:`Swarm.Cluster` includes the data-path port, :api-field:`DefaultAddrPool` and
    :api-field:`SubnetSize`. When :api-op:`POST /swarm/init` gave a pool, the pool is the one that
    the request gave, and so is the size when the request gave a :api-field:`SubnetSize` other than
    0.

.. expectation:: Without a pool or size at init, GET /info reports 10.0.0.0/8 and 24
    :id: SWARM-CLUSTER-INFO-MANAGER-DEFAULT-POOL
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-CLUSTER-ADDRESS-POOLS
    :within: SWARM-CEIL-REJOIN

    On a manager, when :api-op:`POST /swarm/init` gave neither a pool nor a :api-field:`SubnetSize`,
    :api-field:`Swarm.Cluster` in :api-op:`GET /info` reports the pool ``10.0.0.0/8`` and the size
    24.

.. expectation:: A manager's docker info prints the data-path port, pool and subnet size
    :id: SWARM-CLUSTER-INFO-MANAGER-CLI
    :observed_from: api:mgr
    :depends_on: cli
    :platforms: linux
    :refines: REQ-SWARM-CLUSTER-ADDRESS-POOLS, REQ-SWARM-CLUSTER-DATA-PATH-PORT
    :cli_for: SWARM-CLUSTER-INFO-MANAGER
    :within: SWARM-CEIL-REJOIN

    On a manager, :cli:`docker info` prints the data-path port, the default address pool and the
    subnet size. When :cli:`docker swarm init` gave :cli-flag:`--default-addr-pool`, the pool is the
    one that the command gave, and so is the size unless :cli-flag:`--default-addr-pool-mask-length`
    was 0.

.. expectation:: Without a pool or size at init, docker info prints 10.0.0.0/8 and 24
    :id: SWARM-CLUSTER-INFO-MANAGER-DEFAULT-POOL-CLI
    :observed_from: api:mgr
    :depends_on: cli
    :platforms: linux
    :refines: REQ-SWARM-CLUSTER-ADDRESS-POOLS
    :cli_for: SWARM-CLUSTER-INFO-MANAGER-DEFAULT-POOL
    :within: SWARM-CEIL-REJOIN

    On a manager, when :cli:`docker swarm init` gave neither :cli-flag:`--default-addr-pool` nor
    :cli-flag:`--default-addr-pool-mask-length`, :cli:`docker info` prints the pool ``10.0.0.0/8``
    and the size 24.

.. expectation:: A worker's GET /info does not include Swarm.Cluster
    :id: SWARM-CLUSTER-INFO-WORKER
    :observed_from: api:wkr
    :depends_on: none
    :platforms: linux, windows
    :within: SWARM-CEIL-REJOIN
    :rationale: RAT-SWARM-WORKER-HOLDS-WHAT-IT-USES

    On a worker, :api-op:`GET /info` does not include :api-field:`Swarm.Cluster`.

.. expectation:: A worker's docker info prints no data-path port or address pool
    :id: SWARM-CLUSTER-INFO-WORKER-CLI
    :observed_from: api:wkr
    :depends_on: cli
    :platforms: linux, windows
    :cli_for: SWARM-CLUSTER-INFO-WORKER
    :within: SWARM-CEIL-REJOIN
    :rationale: RAT-SWARM-WORKER-HOLDS-WHAT-IT-USES

    On a worker, :cli:`docker info` does not print the data-path port or a default address pool.

.. expectation:: Peers send a node's gossip traffic to its advertise address
    :id: SWARM-CLUSTER-GOSSIP-ADVERTISE-ADDRESS
    :observed_from: underlay
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-SWARM-CLUSTER-GOSSIP-PORTS
    :within: SWARM-CEIL-REJOIN

    Peers send a node's gossip traffic, on TCP and UDP port 7946, to the node's advertise address.

.. expectation:: An AdvertiseAddr interface is resolved to an address once, at init or join
    :id: SWARM-CLUSTER-ADVERTISE-IFACE-RESOLVED-ONCE
    :observed_from: underlay
    :depends_on: none
    :platforms: linux, windows
    :within: SWARM-CEIL-REJOIN
    :narrows: SWARM-CLUSTER-ADVERTISE-ADDRESS-IFACE
    :concerns: SWARM-CLUSTER-GOSSIP-ADVERTISE-ADDRESS
    :violates: REQ-SWARM-CLUSTER-DATA-PATH-ADDRESS

    :api-op:`POST /swarm/init` and :api-op:`POST /swarm/join` with :api-field:`AdvertiseAddr` naming
    a network interface replace the name with the interface's address when the request is made. When
    the node is a manager, the other nodes keep connecting to it for Swarm management at that
    address after the interface's address changes, and after the daemon restarts, even when no
    interface on the node has that address. Peers also keep sending the node's gossip traffic to
    that address.

.. expectation:: A node listens for gossip on port 7946 at its listen address's host
    :id: SWARM-CLUSTER-GOSSIP-LISTEN-ADDRESS
    :observed_from: node
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-SWARM-CLUSTER-GOSSIP-PORTS
    :within: SWARM-CEIL-REJOIN

    A node listens for gossip on TCP and UDP port 7946, on the host part of its listen address.

.. expectation:: Gossip traffic shows no network IDs, service names or task addresses
    :id: SWARM-CLUSTER-GOSSIP-NO-CLEARTEXT
    :observed_from: underlay
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-SWARM-CLUSTER-CONTROL-ENCRYPTED
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* An overlay network, and a service on it whose tasks run on two nodes

    A capture of the gossip traffic between the two nodes, on TCP and UDP port 7946, taken from
    before the service is created until each task resolves the name of the task on the other node,
    does not contain the network's ID, the service's name or the tasks' addresses on the network.
    Each is searched for in the text form that the API reports, an address without its prefix
    length. Every UDP datagram and every message on a TCP connection in the capture is encrypted:
    without the Swarm's gossip keys, none of them decodes as one of memberlist's unencrypted
    messages. This holds whether or not the network is :drvopt:`encrypted`.

.. expectation:: Management traffic to a manager carries only TLS
    :id: SWARM-CLUSTER-MANAGEMENT-NO-CLEARTEXT
    :observed_from: underlay
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-SWARM-CLUSTER-CONTROL-ENCRYPTED

    *Applies to:* A Swarm with one manager, and a node that is not in a Swarm

    :api-op:`POST /swarm/join` on the node, with the Swarm's worker join token and the manager's
    address in :api-field:`RemoteAddrs`, responds 200, and :api-op:`POST /swarm/leave` on the node
    then responds 200. A capture of the TCP traffic between the node and the manager, on the port
    that :api-field:`RemoteAddrs` names, taken from before the join is sent until the leave
    responds, carries only TLS records on every connection. It does not contain the node's name as
    :api-op:`GET /info` on the node reports it in :api-field:`Name`.

.. expectation:: An IPv6 DataPathAddr produces IPv6 outer packets for overlay traffic
    :id: SWARM-CLUSTER-DATA-PATH-ADDRESS-V6
    :observed_from: underlay
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-SWARM-CLUSTER-DATA-PATH-ADDRESS
    :needs: SWARM-HOST-IPV6
    :within: SWARM-CEIL-REJOIN, SWARM-CEIL-REASSIGNMENT

    An IPv6 :api-field:`DataPathAddr` in :api-op:`POST /swarm/init` or :api-op:`POST /swarm/join`
    produces IPv6 outer packets for overlay traffic.

.. expectation:: A join with a data-path address of the other IP version succeeds
    :id: SWARM-CLUSTER-DATA-PATH-ADDRESS-MIXED-VERSION
    :observed_from: api:solo
    :depends_on: none
    :platforms: linux, windows
    :pins: SWARM-CLUSTER-DATA-PATH-MIXED-VERSIONS-ACCEPTED
    :needs: SWARM-HOST-IPV6
    :within: SWARM-CEIL-REJOIN

    *Applies to:* A Swarm whose nodes have data-path addresses of one IP version

    :api-op:`POST /swarm/join` to that Swarm with a data-path address of the other IP version
    responds 200. The node is then in the Swarm.

.. expectation:: A join with a data-path address of the other IP version is accepted
    :id: SWARM-CLUSTER-DATA-PATH-MIXED-VERSIONS-ACCEPTED
    :observed_from: api:solo, task, log
    :depends_on: none
    :platforms: linux, windows
    :needs: SWARM-HOST-IPV6
    :within: SWARM-CEIL-REJOIN, SWARM-CEIL-REASSIGNMENT
    :concerns: SWARM-CLUSTER-DATA-PATH-ADDRESS-MIXED-VERSION, SWARM-CLUSTER-DATA-PATH-ADDRESS-V6
    :violates: REQ-SWARM-OVERLAY-CONNECTIVITY

    :api-op:`POST /swarm/join` with a :api-field:`DataPathAddr` of a different IP version from the
    data-path addresses of the Swarm's other nodes responds 200. Overlay traffic between two nodes
    whose data-path addresses have different IP versions does not reach its destination. Only the
    daemon log of each of the two nodes reports the failure to program the other node as a peer
    (:need:`SWARM-FAILREP-PEER`).

.. expectation:: docker swarm init and join fail on a node with live-restore
    :id: SWARM-CLUSTER-LIVE-RESTORE-REFUSED-CLI
    :observed_from: api:solo
    :depends_on: cli
    :platforms: linux
    :cli_for: SWARM-CLUSTER-LIVE-RESTORE-REFUSED
    :rationale: RAT-SWARM-LIVE-RESTORE-REFUSED

    *Applies to:* A Linux node that is not yet in a Swarm, with :daemon-json:`live-restore` true

    :cli:`docker swarm init` and :cli:`docker swarm join` exit non-zero. The node is then still not
    in a Swarm.

.. expectation:: POST /swarm/init and /swarm/join fail on a node with live-restore
    :id: SWARM-CLUSTER-LIVE-RESTORE-REFUSED
    :observed_from: api:solo
    :depends_on: none
    :platforms: linux
    :rationale: RAT-SWARM-LIVE-RESTORE-REFUSED

    *Applies to:* A Linux node that is not yet in a Swarm, with :daemon-json:`live-restore` true

    :api-op:`POST /swarm/init` and :api-op:`POST /swarm/join` respond with a 4xx or 5xx status. The
    node is then still not in a Swarm.

.. expectation:: POST /swarm/init and /swarm/join respond 500 with live-restore
    :id: SWARM-CLUSTER-LIVE-RESTORE-REFUSED-HTTP500
    :observed_from: api:solo
    :depends_on: none
    :platforms: linux
    :exact_form_of: SWARM-CLUSTER-LIVE-RESTORE-REFUSED

    *Applies to:* A Linux node that is not yet in a Swarm, with :daemon-json:`live-restore` true

    :api-op:`POST /swarm/init` and :api-op:`POST /swarm/join` respond 500.

.. expectation:: A daemon in a Swarm with live-restore does not start
    :id: SWARM-CLUSTER-LIVE-RESTORE-NO-START
    :observed_from: log, node
    :depends_on: none
    :platforms: linux
    :rationale: RAT-SWARM-LIVE-RESTORE-REFUSED

    *Applies to:* A Linux node that is already in a Swarm, with :daemon-json:`live-restore` true

    The daemon does not start. The daemon log contains an error.

.. expectation:: A Swarm node logs that live-restore is incompatible with swarm mode
    :id: SWARM-CLUSTER-LIVE-RESTORE-NO-START-MSG
    :observed_from: log
    :depends_on: none
    :platforms: linux
    :exact_form_of: SWARM-CLUSTER-LIVE-RESTORE-NO-START

    *Applies to:* A Linux node that is already in a Swarm, with :daemon-json:`live-restore` true

    The daemon log contains an error that contains :log:`incompatible with swarm mode`.

.. expectation:: A reload that turns on live-restore on a node in a Swarm is accepted
    :id: SWARM-CLUSTER-LIVE-RESTORE-RELOAD
    :observed_from: api:mgr, api:wkr
    :depends_on: none
    :platforms: linux
    :rationale: RAT-SWARM-LIVE-RESTORE-REFUSED

    *Applies to:* A Linux node in a Swarm, with :daemon-json:`live-restore` false

    The daemon's :file:`daemon.json` is changed to set :daemon-json:`live-restore` to true, and the
    daemon receives :signal:`SIGHUP`. :api-op:`GET /info` then reports
    :api-field:`LiveRestoreEnabled` as true and :api-field:`Swarm.LocalNodeState` as ``active``.

.. _swarm-networking-network-lifecycle-api-surface:

*********************************
 Network lifecycle & API surface
*********************************

A Swarm-scoped network is a network that the Swarm managers keep. Its driver is a multi-host driver
or a node-local driver. A multi-host driver is :driver:`overlay`, or a driver of a managed plugin
that reports the scope ``global`` in its capabilities. A node-local driver is :driver:`bridge`,
:driver:`host`, :driver:`ipvlan`, :driver:`macvlan`, or a driver of a managed plugin that reports
the scope ``local``. A network of a node-local driver is Swarm-scoped when it is created with
:api-field:`Scope` set to ``swarm`` (:cli-flag:`--scope swarm`). A manager accepts as the node-local
driver of a Swarm-scoped network only :driver:`bridge`, :driver:`host`, :driver:`ipvlan`,
:driver:`macvlan` or a driver that a managed plugin provides. Windows nodes have none of these
drivers, so the expectations about a Swarm-scoped network of a node-local driver hold on Linux
nodes.

A Swarm-scoped network is allocated after the network is created, and asynchronously. In this spec,
a network is allocated once :api-op:`GET /networks/{id}` on a manager reports the network's
:api-field:`Driver`.

.. _swarm-networking-overlay-driver-options:

Overlay driver options
======================

An expectation in this section that refers to other expectations or to another section states that
:api-op:`POST /networks/create` for an overlay network with the option responds 201, and that the
network then shows the behavior that the option selects, in the observable that the expectation
names. Its *Observed from* and *Depends on* fields cover that observable. The expectations or the
section that it refers to specify the behavior in full, and what it depends on.

.. expectation:: The mtu option sets the base MTU before encapsulation overhead
    :id: SWARM-OVLOPT-MTU
    :key: com.docker.network.driver.mtu
    :observed_from: api:mgr, task
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-NETWORK-DRIVER-OPTIONS
    :within: SWARM-CEIL-REASSIGNMENT

    The option sets the base MTU from which encapsulation overhead is subtracted.
    :api-op:`POST /networks/create` for an overlay network with the option responds 201, and a task
    on the network observes the inner MTU that these expectations compute from the option's value as
    the base: :need:`SWARM-CTR-MTU-CLEARTEXT`, :need:`SWARM-CTR-MTU-CLEARTEXT-V6`,
    :need:`SWARM-CTR-MTU-ENCRYPTED` and :need:`SWARM-CTR-MTU-ENCRYPTED-V6`.

.. expectation:: Windows nodes run tasks on a network with the mtu option set
    :id: SWARM-OVLOPT-MTU-WINDOWS
    :key: com.docker.network.driver.mtu
    :observed_from: api:mgr
    :depends_on: none
    :platforms: windows
    :refines: REQ-NET-API-BACKWARD-COMPATIBLE

    A task on a network with this option set runs on a Windows node.

.. expectation:: The vxlanid_list option sets one VNI per IPAM subnet
    :id: SWARM-OVLOPT-VNI-LIST
    :key: com.docker.network.driver.overlay.vxlanid_list
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-OVERLAY-VNI-LIST, REQ-SWARM-OVERLAY-VNI-REPORTED
    :within: SWARM-CEIL-ALLOCATION

    The option sets the network's VNIs, one per IPAM subnet. :api-op:`POST /networks/create` for an
    overlay network whose option lists as many VNIs as the network has subnets responds 201. Once
    the network is allocated, :api-op:`GET /networks/{id}` on a manager reports those VNIs in
    :drvopt:`vxlanid_list`, in the order of the network's subnets. See :need:`SWARM-OVL-VNI-LIST`.

.. expectation:: The encrypted option selects an encrypted overlay
    :id: SWARM-OVLOPT-ENCRYPTED
    :key: encrypted
    :observed_from: api:mgr, underlay
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-SWARM-NETWORK-DRIVER-OPTIONS
    :needs: SWARM-HOST-XFRM, SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-IPTABLES-ENC-MATCHES, SWARM-HOST-IPTABLES-LEGACY-MANGLE-KERNEL, SWARM-HOST-IPTABLES-NFT-ROUTE-KERNEL
    :within: SWARM-CEIL-REASSIGNMENT

    The option selects an encrypted overlay, which :ref:`swarm-lifecycle-encryption` in
    :doc:`swarm-lifecycle.md <swarm-lifecycle>` specifies. :api-op:`POST /networks/create` for an
    overlay network with the option responds 201, and the traffic between nodes for the network's
    VNI is IPsec ESP, as :need:`SWARM-ENC-ESP` states.

.. expectation:: The dsr option selects direct server return for east-west VIP traffic
    :id: SWARM-OVLOPT-DSR
    :key: dsr
    :observed_from: api:mgr, task
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-SWARM-NETWORK-DRIVER-OPTIONS
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-NFTABLES-NAT, SWARM-HOST-NFT-COMMAND, SWARM-HOST-NFTABLES-NETDEV, SWARM-HOST-NFT-NETDEV-EGRESS-COMMAND, SWARM-HOST-IPTABLES-LB-EXTENSIONS, SWARM-HOST-IPTABLES-LEGACY-MANGLE-KERNEL
    :within: SWARM-CEIL-REASSIGNMENT

    The option selects direct server return for east-west VIP traffic on Linux nodes.
    :api-op:`POST /networks/create` for an overlay network with the option responds 201, and a task
    that receives VIP traffic on the network observes the client's real source address, as
    :need:`SWARM-LB-DSR-SOURCE` states. See also :need:`SWARM-LB-DSR-DESTINATION`.

.. expectation:: Windows nodes run tasks on a network with the dsr option set
    :id: SWARM-OVLOPT-DSR-WINDOWS
    :key: dsr
    :observed_from: api:mgr
    :depends_on: none
    :platforms: windows
    :refines: REQ-NET-API-BACKWARD-COMPATIBLE

    A task on a network with this option set runs on a Windows node.

.. expectation:: On Linux, encrypted and dsr take effect whatever their value
    :id: SWARM-OVLOPT-FLAG-ANY-VALUE
    :key: encrypted, dsr
    :observed_from: api:mgr, underlay, task
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-SWARM-NETWORK-DRIVER-OPTIONS
    :needs: SWARM-HOST-XFRM, SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-NFTABLES-NAT, SWARM-HOST-NFT-COMMAND, SWARM-HOST-NFTABLES-NETDEV, SWARM-HOST-NFT-NETDEV-EGRESS-COMMAND, SWARM-HOST-IPTABLES-ENC-MATCHES, SWARM-HOST-IPTABLES-LB-EXTENSIONS, SWARM-HOST-IPTABLES-LEGACY-MANGLE-KERNEL, SWARM-HOST-IPTABLES-NFT-ROUTE-KERNEL
    :within: SWARM-CEIL-REASSIGNMENT

    On Linux nodes, each key takes effect when it is present in the network's options, whatever its
    value. This includes the empty string and ``false`` as values: with either value, a network
    created with :drvopt:`encrypted` shows the observable that :need:`SWARM-OVLOPT-ENCRYPTED` names,
    and one created with :drvopt:`dsr` shows the observable that :need:`SWARM-OVLOPT-DSR` names.

.. _swarm-networking-swarm-scoped-networks:

Swarm-scoped networks
=====================

The expectations in this subsection hold for any Swarm-scoped network, whatever its driver. An
expectation here about a network that is not Swarm-scoped and that has the name of a Swarm-scoped
network does not cover the Swarm's predefined networks, whose names are those of the node's own
:network:`bridge` and :network:`host` networks.

.. expectation:: A manager creates a network with Scope swarm or a multi-host driver
    :id: SWARM-SCOPED-CREATE
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-NETWORK-CREATE-ON-MANAGER, REQ-SWARM-NETWORK-PLUGIN-DRIVERS, REQ-SWARM-NETWORK-CONFIG-FROM

    *Applies to:* A manager, and the :driver:`bridge`, :driver:`ipvlan`, :driver:`macvlan` or
    :driver:`overlay` driver, or a driver that a managed plugin provides

    On that manager, :api-op:`POST /networks/create` that sets no fields other than
    :api-field:`Name`, :api-field:`Driver` and :api-field:`Scope`, with a name that no Swarm-scoped
    network has and no network on that manager has, responds 201 when it sets :api-field:`Scope` to
    ``swarm``, or when it names a multi-host driver, such as :driver:`overlay`, and does not set
    :api-field:`Scope` to ``local``. :api-op:`GET /networks` on a manager then lists the network.

.. expectation:: docker network create --scope=swarm or --driver overlay makes a network
    :id: SWARM-SCOPED-CREATE-CLI
    :observed_from: api:mgr
    :depends_on: cli
    :platforms: linux
    :refines: REQ-SWARM-NETWORK-CREATE-ON-MANAGER
    :cli_for: SWARM-SCOPED-CREATE

    *Applies to:* A manager

    On that manager, :cli:`docker network create --scope=swarm` with a name that no Swarm-scoped
    network has, and no network on that manager has, exits zero. :cli:`docker network create
    --driver overlay` with such a name, in place of the first command, also exits zero. :cli:`docker
    network ls` on a manager then lists the network.

.. expectation:: A manager refuses a multi-host driver with Scope local
    :id: SWARM-SCOPED-CREATE-LOCAL-SCOPE
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: PRIN-FAILS-CLOSED, REQ-NET-API-ERROR-STATUS

    *Applies to:* A manager, and a name that no network has

    On that manager, :api-op:`POST /networks/create` with that name, a multi-host driver, such as
    :driver:`overlay`, and :api-field:`Scope` set to ``local`` responds with a 4xx status.
    :api-op:`GET /networks` on a manager then lists no network with that name.

.. expectation:: A multi-host driver with Scope local responds 403
    :id: SWARM-SCOPED-CREATE-LOCAL-SCOPE-HTTP403
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :exact_form_of: SWARM-SCOPED-CREATE-LOCAL-SCOPE

    *Applies to:* A manager

    On that manager, :api-op:`POST /networks/create` with a multi-host driver, such as
    :driver:`overlay`, and :api-field:`Scope` set to ``local`` responds 403, with a message that
    contains :api-error:`cannot downgrade network scope for`.

.. expectation:: A Swarm-scoped network reports its Driver within the allocation ceiling
    :id: SWARM-SCOPED-CREATE-ALLOCATED
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-NETWORK-CREATE-ON-MANAGER
    :within: SWARM-CEIL-ALLOCATION

    Within the :need:`[[title|lower]] ceiling ([[seconds]] s) <SWARM-CEIL-ALLOCATION>` after
    :api-op:`POST /networks/create` for a Swarm-scoped network responds 201, :api-op:`GET
    /networks/{id}` on a manager reports the network's :api-field:`Driver`.

.. expectation:: Creating a Swarm-scoped network on a worker is refused
    :id: SWARM-SCOPED-CREATE-WORKER
    :observed_from: api:wkr
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-SWARM-NETWORK-CREATE-ON-MANAGER, REQ-NET-API-ERROR-STATUS

    On a worker, :api-op:`POST /networks/create` with :api-field:`Scope` set to ``swarm`` responds
    with a 4xx status. It also responds with a 4xx status with a multi-host driver, such as
    :driver:`overlay`, whatever :api-field:`Scope` the request gives.

.. expectation:: Creating a Swarm-scoped network on a worker responds 403
    :id: SWARM-SCOPED-CREATE-WORKER-HTTP403
    :observed_from: api:wkr
    :depends_on: none
    :platforms: linux, windows
    :exact_form_of: SWARM-SCOPED-CREATE-WORKER

    On a worker, :api-op:`POST /networks/create` with :api-field:`Scope` set to ``swarm`` responds 403. It also
    responds 403 with a multi-host driver, such as :driver:`overlay`, whatever :api-field:`Scope` the request
    gives.

.. expectation:: docker network create of a Swarm-scoped network fails on a worker
    :id: SWARM-SCOPED-CREATE-WORKER-CLI
    :observed_from: api:wkr
    :depends_on: cli
    :platforms: linux, windows
    :refines: REQ-SWARM-NETWORK-CREATE-ON-MANAGER
    :cli_for: SWARM-SCOPED-CREATE-WORKER

    On a worker, :cli:`docker network create --scope=swarm` exits non-zero and prints the daemon's
    error. :cli:`docker network create --driver overlay` also exits non-zero and prints the daemon's
    error.

.. expectation:: An overlay create on a worker says to join a Swarm
    :id: SWARM-SCOPED-WORKER-OVERLAY-CREATE-ERROR
    :observed_from: api:wkr
    :depends_on: none
    :platforms: linux, windows
    :concerns: SWARM-SCOPED-CREATE-WORKER
    :violates: PRIN-ERRORS-ACTIONABLE

    On a worker, :api-op:`POST /networks/create` with :api-field:`Driver` set to :driver:`overlay`
    responds with an error whose message is :api-error:`This node is not a swarm manager. Use
    "docker swarm init" or "docker swarm join" to connect this node to swarm and try again.` The
    message tells a node that is already in a Swarm to join a Swarm.

.. expectation:: A network that a service references is not deleted
    :id: SWARM-SCOPED-DELETE-IN-USE-SERVICE
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-NETWORK-DELETE-IN-USE, REQ-NET-API-ERROR-STATUS

    *Applies to:* A Swarm with a Swarm-scoped network other than the ingress network, and a service
    that references the network

    :api-op:`DELETE /networks/{id}` for the network responds 400. The network stays listed.

.. expectation:: The refusal to delete a service's network says it is in use by a service
    :id: SWARM-SCOPED-DELETE-IN-USE-SERVICE-MSG
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :exact_form_of: SWARM-SCOPED-DELETE-IN-USE-SERVICE

    *Applies to:* A Swarm with a Swarm-scoped network other than the ingress network, and a service
    that references the network

    :api-op:`DELETE /networks/{id}` for the network responds 400, with a message that contains
    :api-error:`is in use by service`.

.. expectation:: A network with an attached task or container is not deleted
    :id: SWARM-SCOPED-DELETE-IN-USE-TASK
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-NET-DELETE-IN-USE, REQ-NET-API-ERROR-STATUS, REQ-SWARM-NETWORK-DELETE-IN-USE

    *Applies to:* A Swarm with a Swarm-scoped network other than the ingress network, and a running
    standalone container attached to it, or a task whose :api-field:`Spec.Networks` in
    :api-op:`GET /tasks` names the network and whose :api-field:`DesiredState` and
    :api-field:`Status.State` are each ``running`` or a state before ``running``

    :api-op:`DELETE /networks/{id}` for the network responds 400. The network stays listed.

.. expectation:: The refusal to delete a network a task uses says it is in use by a task
    :id: SWARM-SCOPED-DELETE-IN-USE-TASK-MSG
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :exact_form_of: SWARM-SCOPED-DELETE-IN-USE-TASK

    *Applies to:* A Swarm with a Swarm-scoped network other than the ingress network that no service
    references, and a running standalone container attached to it, or a task whose
    :api-field:`Spec.Networks` in :api-op:`GET /tasks` names the network and whose
    :api-field:`DesiredState` and :api-field:`Status.State` are each ``running`` or a state before
    ``running``

    :api-op:`DELETE /networks/{id}` for the network responds 400, with a message that contains
    :api-error:`is in use by task`.

.. expectation:: docker network rm fails for a network that a service references
    :id: SWARM-SCOPED-DELETE-IN-USE-SERVICE-CLI
    :observed_from: api:mgr
    :depends_on: cli
    :platforms: linux
    :refines: REQ-SWARM-NETWORK-DELETE-IN-USE
    :cli_for: SWARM-SCOPED-DELETE-IN-USE-SERVICE

    *Applies to:* A Swarm with a Swarm-scoped network other than the ingress network, and a service
    that references the network

    :cli:`docker network rm` for the network exits non-zero and prints the daemon's error.
    :cli:`docker network ls` on a manager then still lists the network.

.. expectation:: docker network rm fails for a network with an attached task or container
    :id: SWARM-SCOPED-DELETE-IN-USE-TASK-CLI
    :observed_from: api:mgr
    :depends_on: cli
    :platforms: linux
    :refines: REQ-NET-DELETE-IN-USE, REQ-SWARM-NETWORK-DELETE-IN-USE
    :cli_for: SWARM-SCOPED-DELETE-IN-USE-TASK

    *Applies to:* A Swarm with a Swarm-scoped network other than the ingress network, and a running
    standalone container attached to it, or a task whose :api-field:`Spec.Networks` in
    :api-op:`GET /tasks` names the network and whose :api-field:`DesiredState` and
    :api-field:`Status.State` are each ``running`` or a state before ``running``

    :cli:`docker network rm` for the network exits non-zero and prints the daemon's error.
    :cli:`docker network ls` on a manager then still lists the network.

.. expectation:: A network that only stopped tasks reference is deleted
    :id: SWARM-SCOPED-DELETE-DEAD-TASKS
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-NETWORK-DELETE

    *Applies to:* A Swarm with a Swarm-scoped network other than the ingress network that no service
    references and no standalone container is attached to, and tasks that :api-op:`GET /tasks` lists
    with the network in :api-field:`Spec.Networks`, each with :api-field:`Status.State`
    ``complete``, ``shutdown``, ``failed``, ``rejected`` or ``orphaned``, such as the tasks of a
    service from before an update that removed the network from its
    :api-field:`TaskTemplate.Networks`

    :api-op:`DELETE /networks/{id}` for the network responds 204, and :api-op:`GET /networks` on a
    manager then does not list it.

.. expectation:: Deleting the Swarm's bridge or host network by its ID responds 404
    :id: SWARM-SCOPED-PREDEFINED-DELETE-NOT-FOUND
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :narrows: SWARM-SCOPED-DELETE-IN-USE-SERVICE, SWARM-SCOPED-DELETE-IN-USE-SERVICE-MSG, SWARM-SCOPED-DELETE-IN-USE-SERVICE-CLI, SWARM-SCOPED-DELETE-IN-USE-TASK, SWARM-SCOPED-DELETE-IN-USE-TASK-MSG, SWARM-SCOPED-DELETE-IN-USE-TASK-CLI
    :concerns: NET-API-DELETE-PREDEFINED, NET-API-DELETE-PREDEFINED-CLI
    :violates: REQ-NET-LOOKUP, REQ-SWARM-NETWORK-DELETE-IN-USE

    On a manager, :api-op:`DELETE /networks/{id}` with the full ID of one of the Swarm's
    predefined networks, or with a prefix of that ID that the ID of no other network starts with
    and that is not a network name, responds 404, with a message that contains :api-error:`not
    found`. This holds whether or not a service refers to the network. :api-op:`GET
    /networks/{id}` with that ID then still responds 200 with the network.

.. expectation:: Creating a network with a Swarm-scoped network's name responds 409
    :id: SWARM-SCOPED-NAME-CONFLICT
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-NET-NAME-UNIQUE, REQ-NET-API-ERROR-STATUS

    *Applies to:* A Swarm with a Swarm-scoped network

    :api-op:`POST /networks/create` with the name of that network responds 409. :api-op:`GET
    /networks` on a manager then lists only one network with that name.

.. expectation:: docker network create with a Swarm-scoped network's name fails
    :id: SWARM-SCOPED-NAME-CONFLICT-CLI
    :observed_from: api:mgr
    :depends_on: cli
    :platforms: linux
    :refines: REQ-NET-NAME-UNIQUE
    :cli_for: SWARM-SCOPED-NAME-CONFLICT

    *Applies to:* A Swarm with a Swarm-scoped network

    :cli:`docker network create` with the name of that network exits non-zero and prints the
    daemon's error. :cli:`docker network ls` on a manager then lists only one network with that
    name.

.. expectation:: A manager lists a local and a Swarm-scoped network of the same name
    :id: SWARM-SCOPED-NAME-COLLISION-LIST
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-NETWORK-SCOPE-VISIBLE

    *Applies to:* A manager with a network that is not Swarm-scoped and that has the name of a
    Swarm-scoped network

    On that manager, :api-op:`GET /networks` lists both networks.

.. expectation:: A name shared with a local network fails to inspect or delete either
    :id: SWARM-SCOPED-NAME-COLLISION-AMBIGUOUS
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: PRIN-AMBIGUITY-NOT-DESTRUCTIVE, REQ-NET-API-ERROR-STATUS, REQ-NET-LOOKUP

    *Applies to:* A manager with a network that is not Swarm-scoped and that has the name of a
    Swarm-scoped network

    On that manager, :api-op:`GET /networks/{id}` and :api-op:`DELETE /networks/{id}` with that name
    respond with a 4xx status. :api-op:`DELETE /networks/{id}` does not delete either
    network.

.. expectation:: A shared name responds 400 with a message that it is ambiguous
    :id: SWARM-SCOPED-NAME-COLLISION-AMBIGUOUS-HTTP400
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :exact_form_of: SWARM-SCOPED-NAME-COLLISION-AMBIGUOUS
    :pins: SWARM-SCOPED-NAME-COLLISION-LOCAL

    *Applies to:* A manager with a network that is not Swarm-scoped and that has the name of a
    Swarm-scoped network

    On that manager, :api-op:`GET /networks/{id}` and :api-op:`DELETE /networks/{id}` with that name
    respond 400, with a message that contains :api-error:`is ambiguous`.

.. expectation:: docker network inspect and rm fail for a name a local network shares
    :id: SWARM-SCOPED-NAME-COLLISION-AMBIGUOUS-CLI
    :observed_from: api:mgr
    :depends_on: cli
    :platforms: linux
    :refines: PRIN-AMBIGUITY-NOT-DESTRUCTIVE, REQ-NET-LOOKUP
    :cli_for: SWARM-SCOPED-NAME-COLLISION-AMBIGUOUS

    *Applies to:* A manager with a network that is not Swarm-scoped and that has the name of a
    Swarm-scoped network

    On that manager, :cli:`docker network inspect` and :cli:`docker network rm` with that name exit
    non-zero and print the daemon's error.

.. expectation:: The scope parameter selects which network a shared name returns
    :id: SWARM-SCOPED-NAME-COLLISION-SCOPE-FILTER
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-NETWORK-SCOPE-VISIBLE

    *Applies to:* A manager with a network that is not Swarm-scoped and that has the name of a
    Swarm-scoped network

    On that manager, :api-op:`GET /networks/{id}` with that name and with :api-query:`scope` set to
    ``local`` returns the network that is not Swarm-scoped. With :api-query:`scope` set to
    ``swarm``, it returns the Swarm-scoped network.

.. expectation:: A node with a same-named local network rejects the Swarm network's tasks
    :id: SWARM-SCOPED-NAME-COLLISION-TASKS
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows
    :pins: SWARM-SCOPED-NAME-COLLISION-LOCAL

    *Applies to:* A node with a network that is not Swarm-scoped and that has the name of a
    Swarm-scoped network

    On that node, the tasks of a service on the Swarm-scoped network are rejected.

.. expectation:: A container naming the Swarm network by ID fails to start with 500
    :id: SWARM-SCOPED-NAME-COLLISION-START-BY-ID
    :observed_from: api:active
    :depends_on: none
    :platforms: linux, windows
    :pins: SWARM-SCOPED-NAME-COLLISION-LOCAL
    :narrows: SWARM-SCOPED-ATTACHABLE-START, SWARM-SCOPED-ATTACHABLE-START-CLI
    :concerns: SWARM-SCOPED-NAME-COLLISION-START-BY-NAME
    :violates: PRIN-ERRORS-ACTIONABLE

    *Applies to:* A node with a network that is not Swarm-scoped and that has the name of an
    attachable Swarm-scoped network, with a created or stopped standalone container whose endpoint
    settings for the Swarm-scoped network carry its ID

    On that node, :api-op:`POST /containers/{id}/start` for the container responds 500, with a
    message that contains :api-error:`could not find network <ID> after successful attachment`. The
    message does not name the network that has the same name.

.. expectation:: A container naming the shared name starts on the local network
    :id: SWARM-SCOPED-NAME-COLLISION-START-BY-NAME
    :observed_from: api:active
    :depends_on: none
    :platforms: linux, windows
    :pins: SWARM-SCOPED-NAME-COLLISION-LOCAL

    *Applies to:* A node with a network that is not Swarm-scoped and that has the name of a
    Swarm-scoped network, with a created standalone container whose endpoint settings name that
    network by its name

    On that node, :api-op:`POST /containers/{id}/start` for the container responds 204. :api-op:`GET
    /containers/{id}/json` then reports the ID of the network that is not Swarm-scoped as the
    :api-field:`NetworkID` for that name.

.. expectation:: A worker creates a local network with a Swarm-scoped network's name
    :id: SWARM-SCOPED-NAME-COLLISION-LOCAL
    :observed_from: api:mgr, api:wkr
    :depends_on: none
    :platforms: linux, windows
    :narrows: SWARM-SCOPED-NAME-CONFLICT, SWARM-SCOPED-ATTACHABLE-START, SWARM-SCOPED-ATTACHABLE-START-CLI, SWARM-SCOPED-ATTACHABLE-CONNECT, SWARM-SCOPED-ATTACHABLE-CONNECT-CLI
    :concerns: SWARM-SCOPED-NAME-COLLISION-TASKS, SWARM-SCOPED-NAME-COLLISION-START-BY-ID, SWARM-SCOPED-NAME-COLLISION-START-BY-NAME, SWARM-SCOPED-NAME-COLLISION-AMBIGUOUS, NET-API-INSPECT-ID-NAME-PREFIX, NET-API-INSPECT-ID-NAME-PREFIX-CLI
    :violates: REQ-NET-NAME-UNIQUE, REQ-SWARM-NETWORK-WORKER-ON-DEMAND

    :api-op:`POST /networks/create` on a worker, for a network that is not Swarm-scoped and that
    has the name of a Swarm-scoped network other than one of the Swarm's predefined networks,
    responds 201. The same request responds 409 on a manager. :api-op:`POST /nodes/{id}/update`
    that sets the :api-field:`Role` of a worker that has such a network to ``manager`` responds
    200, and :api-op:`GET /nodes/{id}` then reports ``manager`` as the node's
    :api-field:`Spec.Role`. On a manager with both networks, :api-op:`GET /networks/{id}` and
    :api-op:`DELETE /networks/{id}` with the shared name respond with a 4xx or 5xx status. On any
    node with both networks, the node rejects the Swarm-scoped network's tasks,
    :api-op:`POST /containers/{id}/start` for a standalone container that names the Swarm-scoped
    network by its ID fails, and one that names it by its name is attached to the other network.
    :api-op:`POST /networks/{id}/connect` for a running container there fails when ``{id}`` is the
    Swarm-scoped network's ID, and attaches the container to the other network when ``{id}`` is the
    shared name.

.. expectation:: GET /networks/{id} reports Scope swarm and the network's create fields
    :id: SWARM-SCOPED-INSPECT
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-NET-INSPECT-CONFIGURATION, REQ-SWARM-NETWORK-SCOPE-VISIBLE
    :within: SWARM-CEIL-ALLOCATION

    Once a Swarm-scoped network is allocated, :api-op:`GET /networks/{id}` for that
    network reports :api-field:`Scope` as ``swarm``, and reports :api-field:`Created`. It also
    reports the :api-field:`Driver`, :api-field:`Ingress`, :api-field:`Attachable`,
    :api-field:`Internal`, :api-field:`EnableIPv6`, :api-field:`ConfigFrom` and :api-field:`Labels`
    given at create. For an overlay network, it also reports the :api-field:`IPAM` of the
    allocation.

.. expectation:: docker network inspect prints a Swarm-scoped network's Scope and Driver
    :id: SWARM-SCOPED-INSPECT-CLI
    :observed_from: api:mgr
    :depends_on: cli
    :platforms: linux
    :refines: REQ-SWARM-NETWORK-SCOPE-VISIBLE
    :cli_for: SWARM-SCOPED-INSPECT

    *Applies to:* An allocated Swarm-scoped network

    On a manager, :cli:`docker network inspect` for the network prints ``swarm`` as its
    :api-field:`Scope`, and prints its :api-field:`Driver`.

.. expectation:: A worker lists a Swarm-scoped network while it has an attachment
    :id: SWARM-SCOPED-LIST-WORKER
    :observed_from: api:wkr
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-SWARM-NETWORK-WORKER-ON-DEMAND

    *Applies to:* A worker with a local attachment to a Swarm-scoped network

    On that worker, :api-op:`GET /networks` lists the network.

.. expectation:: A worker does not list a Swarm-scoped network it has no attachment to
    :id: SWARM-SCOPED-LIST-WORKER-UNATTACHED
    :observed_from: api:wkr
    :depends_on: none
    :platforms: linux, windows
    :rationale: RAT-SWARM-WORKER-HOLDS-WHAT-IT-USES

    *Applies to:* A worker that does not have a local attachment to a Swarm-scoped network other
    than the ingress network

    On that worker, :api-op:`GET /networks` does not list the network.

.. expectation:: dangling=true lists every Swarm-scoped network, and false attached ones
    :id: SWARM-SCOPED-LIST-DANGLING
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :pins: NET-API-FILTER-DANGLING-SWARM-SCOPED

    *Applies to:* A manager

    :api-op:`GET /networks` with the filter :api-query:`dangling=true` lists every Swarm-scoped
    network other than the Swarm's predefined networks, including the ingress network and networks
    that services use. :api-op:`GET /networks` with the filter :api-query:`dangling=false` lists the
    Swarm-scoped networks that have a local attachment on that manager. Those networks are then in
    both responses.

.. expectation:: An unattached worker's GET /networks/{id} responds 404
    :id: SWARM-SCOPED-INSPECT-WORKER-UNATTACHED
    :observed_from: api:wkr
    :depends_on: none
    :platforms: linux, windows
    :rationale: RAT-SWARM-WORKER-HOLDS-WHAT-IT-USES

    *Applies to:* A worker that does not have a local attachment to a Swarm-scoped network other
    than the ingress network

    On that worker, :api-op:`GET /networks/{id}` for that network responds 404.

.. expectation:: An unattached manager reports empty Containers and no Peers
    :id: SWARM-SCOPED-INSPECT-MANAGER-UNATTACHED
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :rationale: RAT-SWARM-NODE-REPORTS-OWN-STATE

    *Applies to:* A manager that does not have a local attachment to an overlay network, and another
    node with a running task or standalone container on that network

    On that manager, :api-op:`GET /networks/{id}` for that network reports an empty
    :api-field:`Containers`. It does not include :api-field:`Peers`.

.. expectation:: A manager lists each Swarm-scoped network with Scope swarm
    :id: SWARM-SCOPED-LIST-MANAGER
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-NETWORK-SCOPE-VISIBLE

    :api-op:`GET /networks` on a manager lists each Swarm-scoped network other than the Swarm's
    predefined networks, and reports its :api-field:`Scope` as ``swarm``. This holds whether or not
    the manager has a local attachment to the network.

.. expectation:: docker network ls on a manager lists each Swarm-scoped network
    :id: SWARM-SCOPED-LIST-MANAGER-CLI
    :observed_from: api:mgr
    :depends_on: cli
    :platforms: linux
    :refines: REQ-SWARM-NETWORK-SCOPE-VISIBLE
    :cli_for: SWARM-SCOPED-LIST-MANAGER

    :cli:`docker network ls` on a manager lists each Swarm-scoped network other than the Swarm's
    predefined networks.

.. expectation:: The scope filter lists only the networks of that scope
    :id: SWARM-SCOPED-LIST-SCOPE-FILTER
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-NETWORK-SCOPE-VISIBLE

    *Applies to:* A manager, a Swarm-scoped network other than one of the Swarm's predefined
    networks, and a network whose :api-field:`Scope` is ``local``

    On that manager, :api-op:`GET /networks` with the filter :api-query:`scope=swarm` lists the
    Swarm-scoped network and not the other one. With the filter :api-query:`scope=local`, it lists
    the other one and not the Swarm-scoped network.

.. expectation:: On a manager, GET /networks never lists the Swarm's bridge or host network
    :id: SWARM-SCOPED-PREDEFINED-NOT-LISTED
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :rationale: RAT-SWARM-PREDEFINED-NETWORKS

    On a manager, :api-op:`GET /networks` does not list the Swarm's predefined networks, with or
    without filters. This includes the filters :api-query:`scope=swarm`, :api-query:`type=builtin`,
    :api-query:`type=custom` and :api-query:`dangling=true`, and an :api-query:`id` filter with
    the full ID of one of them.

.. expectation:: The Swarm's bridge and host networks are found by ID, prefix or scope=swarm
    :id: SWARM-SCOPED-PREDEFINED-INSPECT
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :rationale: RAT-SWARM-PREDEFINED-NETWORKS

    On a manager, :api-op:`GET /networks/{id}` responds 200 with one of the Swarm's predefined
    networks when ``{id}`` is its full ID, or a prefix of its ID that the ID of no other network
    starts with and that is not a network name. It does the same when ``{id}`` is :network:`bridge`
    or :network:`host` and :api-query:`scope` is ``swarm``. The response reports the
    :api-field:`Name` :network:`bridge` or :network:`host`, a :api-field:`Driver` of the same
    name, :api-field:`Scope` ``swarm``, an empty :api-field:`IPAM.Config`, and
    ``com.docker.swarm.predefined`` set to ``true`` in :api-field:`Labels`.

.. expectation:: A container on an attachable network starts on any active node
    :id: SWARM-SCOPED-ATTACHABLE-START
    :observed_from: api:active
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-SWARM-NETWORK-ATTACHABLE

    On any node whose availability is ``active``, :api-op:`POST /containers/{id}/start` for a
    created or stopped container on an attachable Swarm-scoped network, with no static address on
    that network or with static addresses that are inside the network's subnets and not in use on
    the network, responds 204. The container is then running and has an address on the network. For
    a network created with :api-field:`ConfigFrom`, this holds for a network of a node-local driver,
    on a node that has the config-only network that :api-field:`ConfigFrom` names. For a network
    whose driver is a managed plugin, this holds on a node that has that plugin enabled.

.. expectation:: docker run on an attachable network succeeds on any active node
    :id: SWARM-SCOPED-ATTACHABLE-START-CLI
    :observed_from: api:active
    :depends_on: cli
    :platforms: linux, windows
    :refines: REQ-SWARM-NETWORK-ATTACHABLE
    :cli_for: SWARM-SCOPED-ATTACHABLE-START

    On any node whose availability is ``active``, :cli:`docker run -d --network` naming an
    attachable Swarm-scoped network exits zero. The container then runs and is attached to the
    network. For a network created with :api-field:`ConfigFrom`, this holds for a network of a
    node-local driver, on a node that has the config-only network that :api-field:`ConfigFrom`
    names. For a network whose driver is a managed plugin, this holds on a node that has that plugin
    enabled.

.. expectation:: A container naming an attachable network by ID prefix fails with 500
    :id: SWARM-SCOPED-ATTACHABLE-START-ID-PREFIX
    :observed_from: api:active
    :depends_on: none
    :platforms: linux, windows
    :narrows: SWARM-SCOPED-ATTACHABLE-START, SWARM-SCOPED-ATTACHABLE-START-CLI
    :violates: REQ-NET-LOOKUP, REQ-NET-API-ERROR-STATUS

    *Applies to:* An attachable Swarm-scoped network, a prefix of its ID, shorter than the ID, that
    the ID of no other network starts with and that is not a network name, and a created or stopped
    standalone container that names the network by that prefix

    :api-op:`POST /containers/{id}/start` for the container responds 500, with a message that
    contains :api-error:`network <prefix> not found`, where ``<prefix>`` is that prefix.

.. expectation:: A running container connects to an attachable network on any active node
    :id: SWARM-SCOPED-ATTACHABLE-CONNECT
    :observed_from: api:active
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-SWARM-NETWORK-ATTACHABLE

    On any node whose availability is ``active``, :api-op:`POST /networks/{id}/connect` to an
    attachable Swarm-scoped network, for a running standalone container that is not attached to the
    network, with no address in :api-field:`EndpointConfig.IPAMConfig` or with addresses there that
    are inside the network's subnets and not in use on the network, responds 200. The container then
    has an address on the network. For a network created with :api-field:`ConfigFrom`, this holds
    for a network of a node-local driver, on a node that has the config-only network that
    :api-field:`ConfigFrom` names. For a network whose driver is a managed plugin, this holds on a
    node that has that plugin enabled.

.. expectation:: docker network connect to an attachable network succeeds on active nodes
    :id: SWARM-SCOPED-ATTACHABLE-CONNECT-CLI
    :observed_from: api:active
    :depends_on: cli
    :platforms: linux, windows
    :refines: REQ-SWARM-NETWORK-ATTACHABLE
    :cli_for: SWARM-SCOPED-ATTACHABLE-CONNECT

    On any node whose availability is ``active``, :cli:`docker network connect` naming an attachable
    Swarm-scoped network and a running standalone container that is not attached to it, without
    :cli-flag:`--ip` and :cli-flag:`--ip6` or with addresses in them that are inside the network's
    subnets and not in use on the network, exits zero. The container then has an address on the
    network. For a network created with :api-field:`ConfigFrom`, this holds for a network of a
    node-local driver, on a node that has the config-only network that :api-field:`ConfigFrom`
    names. For a network whose driver is a managed plugin, this holds on a node that has that plugin
    enabled.

.. expectation:: Connecting by an attachable network's ID prefix fails with 500
    :id: SWARM-SCOPED-ATTACHABLE-CONNECT-ID-PREFIX
    :observed_from: api:active
    :depends_on: none
    :platforms: linux, windows
    :narrows: SWARM-SCOPED-ATTACHABLE-CONNECT, SWARM-SCOPED-ATTACHABLE-CONNECT-CLI
    :violates: REQ-NET-LOOKUP, REQ-NET-API-ERROR-STATUS

    *Applies to:* An attachable Swarm-scoped network, a prefix of its ID, shorter than the ID, that
    the ID of no other network starts with and that is not a network name, and a running standalone
    container that is not attached to the network

    :api-op:`POST /networks/{id}/connect` with that prefix as ``{id}``, for the container, responds
    500, with a message that contains :api-error:`network <prefix> not found`, where ``<prefix>`` is
    that prefix. :api-op:`GET /containers/{id}/json` then does not list the network in
    :api-field:`NetworkSettings.Networks`.

.. expectation:: Connecting an already attached container again responds 409
    :id: SWARM-SCOPED-CONNECT-ALREADY-ATTACHED-HTTP409
    :observed_from: api:active
    :depends_on: none
    :platforms: linux, windows
    :exact_form_of: NET-API-CONNECT-ALREADY-ATTACHED

    *Applies to:* A running standalone container on any node whose availability is ``active`` that
    is already attached to an attachable Swarm-scoped network

    :api-op:`POST /networks/{id}/connect` to that network, for that container, responds 409. The
    container's addresses on that network do not change.

.. expectation:: A container's attachment task disappears once it disconnects or stops
    :id: SWARM-SCOPED-ATTACHMENT-TASK-REMOVED
    :observed_from: api:mgr, api:active
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-SWARM-NETWORK-ATTACHMENT-RELEASED, PRIN-NO-LEAKS
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A standalone container on an attachable Swarm-scoped network

    The container disconnects from the network, or stops. The container's attachment task then
    disappears from :api-op:`GET /tasks` on a manager with the :api-query:`runtime=attachment`
    filter.

.. expectation:: Once its last standalone container leaves, a network's delete responds 204
    :id: SWARM-SCOPED-ATTACHABLE-LAST-DETACH-DELETE
    :observed_from: api:mgr, api:active
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-SWARM-NETWORK-ATTACHMENT-RELEASED, PRIN-NO-LEAKS, REQ-SWARM-NETWORK-DELETE
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A standalone container on an attachable Swarm-scoped network other than an ingress
    network, that no service references and to which nothing else is attached

    The container disconnects from the network, or stops. :api-op:`DELETE /networks/{id}` for the
    network on a manager then responds 204.

.. expectation:: Starting on an attachable network fails on a drained or paused node
    :id: SWARM-SCOPED-DRAINED-START
    :observed_from: api:mgr, api:wkr
    :depends_on: none
    :platforms: linux, windows
    :pins: SWARM-SCOPED-DRAINED-STANDALONE-ATTACH

    *Applies to:* A created or stopped container on an attachable Swarm-scoped network, on a node
    whose availability is ``drain`` or ``pause``

    :api-op:`POST /containers/{id}/start` for the container responds with a 4xx or 5xx status.

.. expectation:: Starting on a drained or paused node responds 500 after 20 seconds
    :id: SWARM-SCOPED-DRAINED-START-HTTP500
    :observed_from: api:mgr, api:wkr
    :depends_on: none
    :platforms: linux, windows
    :exact_form_of: SWARM-SCOPED-DRAINED-START
    :pins: SWARM-SCOPED-ATTACH-FAILURE-TIMEOUT
    :concerns: SWARM-SCOPED-DRAINED-START
    :violates: PRIN-ERRORS-ACTIONABLE

    *Applies to:* A created or stopped container on an attachable Swarm-scoped network, on a node
    whose availability is ``drain`` or ``pause``

    :api-op:`POST /containers/{id}/start` for the container responds 500 after 20 seconds, with a
    message that contains :api-error:`attaching to network failed, make sure your network options
    are correct and check manager logs: context deadline exceeded`.

.. expectation:: Connecting to an attachable network fails on a drained or paused node
    :id: SWARM-SCOPED-DRAINED-CONNECT
    :observed_from: api:mgr, api:wkr
    :depends_on: none
    :platforms: linux, windows
    :pins: SWARM-SCOPED-DRAINED-STANDALONE-ATTACH

    *Applies to:* A running standalone container on a node whose availability is ``drain`` or
    ``pause``, and an attachable Swarm-scoped network that the container is not attached to

    :api-op:`POST /networks/{id}/connect` to that network, for the container, responds with a 4xx or
    5xx status. :api-op:`GET /containers/{id}/json` then does not list the network in
    :api-field:`NetworkSettings.Networks`.

.. expectation:: Connecting on a drained or paused node responds 500 after 20 seconds
    :id: SWARM-SCOPED-DRAINED-CONNECT-HTTP500
    :observed_from: api:mgr, api:wkr
    :depends_on: none
    :platforms: linux, windows
    :exact_form_of: SWARM-SCOPED-DRAINED-CONNECT
    :pins: SWARM-SCOPED-ATTACH-FAILURE-TIMEOUT
    :concerns: SWARM-SCOPED-DRAINED-CONNECT
    :violates: PRIN-ERRORS-ACTIONABLE

    *Applies to:* A running standalone container on a node whose availability is ``drain`` or
    ``pause``, and an attachable Swarm-scoped network that the container is not attached to

    :api-op:`POST /networks/{id}/connect` to that network, for the container, responds 500 after 20
    seconds, with a message that contains :api-error:`attaching to network failed, make sure your
    network options are correct and check manager logs: context deadline exceeded`.

.. expectation:: docker run on an attachable network fails on a drained or paused node
    :id: SWARM-SCOPED-DRAINED-START-CLI
    :observed_from: api:mgr, api:wkr
    :depends_on: cli
    :platforms: linux, windows
    :cli_for: SWARM-SCOPED-DRAINED-START
    :pins: SWARM-SCOPED-DRAINED-STANDALONE-ATTACH

    *Applies to:* A node whose availability is ``drain`` or ``pause``, and an attachable
    Swarm-scoped network

    On that node, :cli:`docker run -d --network` naming that network exits non-zero and prints the
    daemon's error.

.. expectation:: A standalone container cannot attach on a drained or paused node
    :id: SWARM-SCOPED-DRAINED-STANDALONE-ATTACH
    :observed_from: api:mgr, api:wkr
    :depends_on: none
    :platforms: linux, windows
    :concerns: SWARM-SCOPED-ATTACHABLE-START, SWARM-SCOPED-ATTACHABLE-CONNECT, SWARM-SCOPED-DRAINED-START, SWARM-SCOPED-DRAINED-CONNECT, SWARM-SCOPED-DRAINED-START-CLI
    :violates: REQ-SWARM-NETWORK-ATTACHABLE

    On a node whose availability is ``drain`` or ``pause``, :api-op:`POST /containers/{id}/start`
    for a container on an attachable Swarm-scoped network, and :api-op:`POST /networks/{id}/connect`
    to such a network for a running container, respond with a 4xx or 5xx status.

.. expectation:: A container on a non-attachable network is created
    :id: SWARM-SCOPED-NOT-ATTACHABLE-CREATE
    :observed_from: api:mgr, api:wkr
    :depends_on: none
    :platforms: linux, windows
    :pins: SWARM-SCOPED-NOT-ATTACHABLE-UNCHECKED

    :api-op:`POST /containers/create` for a container on a non-attachable Swarm-scoped network
    responds 201. :api-op:`GET /containers/{id}/json` then reports the container in the ``created``
    state.

.. expectation:: A container on a non-attachable network fails to start
    :id: SWARM-SCOPED-NOT-ATTACHABLE-START
    :observed_from: api:mgr, api:wkr
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-SWARM-NETWORK-ATTACHABLE, REQ-SWARM-ATTACH-FAILURE-START

    :api-op:`POST /containers/{id}/start` for a created or stopped container on a non-attachable
    Swarm-scoped network, the ingress network included, responds with a 4xx or 5xx status.

.. expectation:: Starting a container on a non-attachable network responds 500
    :id: SWARM-SCOPED-NOT-ATTACHABLE-START-HTTP500
    :observed_from: api:mgr, api:wkr
    :depends_on: none
    :platforms: linux, windows
    :exact_form_of: SWARM-SCOPED-NOT-ATTACHABLE-START
    :pins: SWARM-CLUSTER-HTTP500-REFUSED

    :api-op:`POST /containers/{id}/start` for a created or stopped container on a non-attachable
    Swarm-scoped network, the ingress network included, responds 500, with a message that contains
    :api-error:`not manually attachable`.

.. expectation:: docker run on a non-attachable network fails
    :id: SWARM-SCOPED-NOT-ATTACHABLE-START-CLI
    :observed_from: api:mgr, api:wkr
    :depends_on: cli
    :platforms: linux, windows
    :refines: REQ-SWARM-NETWORK-ATTACHABLE, REQ-SWARM-ATTACH-FAILURE-START
    :cli_for: SWARM-SCOPED-NOT-ATTACHABLE-START

    :cli:`docker run -d --network` naming a non-attachable Swarm-scoped network, the ingress network
    included, exits with status 125 and prints the daemon's error.

.. expectation:: Non-attachable networks are not checked at container create
    :id: SWARM-SCOPED-NOT-ATTACHABLE-UNCHECKED
    :observed_from: api:active
    :depends_on: none
    :platforms: linux, windows
    :concerns: SWARM-SCOPED-NOT-ATTACHABLE-CREATE, SWARM-SCOPED-NOT-ATTACHABLE-START
    :violates: PRIN-FAIL-FAST

    :api-op:`POST /containers/create` for a container on a non-attachable
    Swarm-scoped network responds 201. Only :api-op:`POST /containers/{id}/start` refuses the
    network.

.. expectation:: On a non-attachable network, a container without AutoRemove stays created after a failed start
    :id: SWARM-SCOPED-NOT-ATTACHABLE-STAYS-CREATED
    :observed_from: api:mgr, api:wkr
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-NET-START-FAILED-CONTAINER-KEPT

    *Applies to:* A created container without :api-field:`HostConfig.AutoRemove` on a non-attachable
    Swarm-scoped network

    :api-op:`POST /containers/{id}/start` for the container responds with a 4xx or 5xx status.
    :api-op:`GET /containers/{id}/json` then reports :api-field:`State.Status` as ``created``.

.. expectation:: On a non-attachable network, an AutoRemove container is removed after a failed start
    :id: SWARM-SCOPED-NOT-ATTACHABLE-AUTO-REMOVE
    :observed_from: api:mgr, api:wkr
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-NET-START-FAILED-AUTO-REMOVED, PRIN-NO-LEAKS

    *Applies to:* A created container with :api-field:`HostConfig.AutoRemove` on a non-attachable
    Swarm-scoped network

    :api-op:`POST /containers/{id}/start` for the container responds with a 4xx or 5xx status.
    :api-op:`GET /containers/{id}/json` for the container then responds 404.

.. expectation:: Connecting a container to a non-attachable network fails
    :id: SWARM-SCOPED-NOT-ATTACHABLE-CONNECT
    :observed_from: api:mgr, api:wkr
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-SWARM-NETWORK-ATTACHABLE

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
    :exact_form_of: SWARM-SCOPED-NOT-ATTACHABLE-CONNECT
    :pins: SWARM-CLUSTER-HTTP500-REFUSED

    *Applies to:* A running standalone container, and a non-attachable Swarm-scoped network, the
    ingress network included

    :api-op:`POST /networks/{id}/connect` to that network, for the container, responds 500, with a
    message that contains :api-error:`not manually attachable`.

.. expectation:: docker network connect to a non-attachable network fails
    :id: SWARM-SCOPED-NOT-ATTACHABLE-CONNECT-CLI
    :observed_from: api:mgr, api:wkr
    :depends_on: cli
    :platforms: linux, windows
    :refines: REQ-SWARM-NETWORK-ATTACHABLE
    :cli_for: SWARM-SCOPED-NOT-ATTACHABLE-CONNECT

    *Applies to:* A running standalone container, and a non-attachable Swarm-scoped network, the
    ingress network included

    :cli:`docker network connect` naming that network and the container exits non-zero and prints
    the daemon's error.

.. expectation:: Tasks on a ConfigFrom network run where the config-only network exists
    :id: SWARM-SCOPED-CONFIG-FROM-TASKS
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-NETWORK-CONFIG-FROM

    *Applies to:* A Swarm-scoped network of a node-local driver, such as :driver:`bridge`, with
    :api-field:`ConfigFrom` naming a config-only network that exists on each node that runs a task

    :api-op:`GET /tasks` reports the tasks of a service on the network as ``running``.

.. expectation:: A task's address is in its node's config-only network's subnet
    :id: SWARM-SCOPED-CONFIG-FROM-SUBNET
    :observed_from: task
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-SWARM-NETWORK-CONFIG-FROM, REQ-NET-CONFIG-FROM
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A Swarm-scoped network of a node-local driver, such as :driver:`bridge`, with
    :api-field:`ConfigFrom` naming a config-only network that exists on each node that runs a task

    Each task of a service on the network has an address on the network in the subnet of its node's
    config-only network.

.. expectation:: A node without the config-only network rejects the network's tasks
    :id: SWARM-SCOPED-CONFIG-FROM-MISSING-TASKS
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-NETWORK-CONFIG-FROM, REQ-SWARM-TASK-ERRORS, PRIN-ERRORS-ACTIONABLE

    *Applies to:* A Swarm-scoped network of a node-local driver, created with
    :api-field:`ConfigFrom` naming a config-only network, and a node that does not have that
    config-only network

    On that node, the tasks of a service on the network are rejected. Their error in :api-op:`GET
    /tasks` contains the name of the config-only network.

.. expectation:: A rejected task's error says the configuration network does not exist
    :id: SWARM-SCOPED-CONFIG-FROM-MISSING-TASKS-MSG
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :exact_form_of: SWARM-SCOPED-CONFIG-FROM-MISSING-TASKS

    *Applies to:* A Swarm-scoped network of a node-local driver, created with
    :api-field:`ConfigFrom` naming a config-only network, and a node that does not have that
    config-only network

    On that node, the error of a rejected task in :api-op:`GET /tasks` contains
    :api-error:`configuration network "<name>" does not exist`.

.. expectation:: A node without the config-only network fails a container start
    :id: SWARM-SCOPED-CONFIG-FROM-MISSING-START
    :observed_from: api:active
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-NETWORK-CONFIG-FROM, REQ-SWARM-ATTACH-FAILURE-START

    *Applies to:* A Swarm-scoped network of a node-local driver, created with
    :api-field:`ConfigFrom` naming a config-only network, and a node whose availability is
    ``active`` that does not have that config-only network, with a created or stopped container on
    the network

    On that node, :api-op:`POST /containers/{id}/start` for the container responds with a 4xx or 5xx
    status.

.. expectation:: Without the config-only network, start responds 500 after 20 seconds
    :id: SWARM-SCOPED-CONFIG-FROM-MISSING-START-HTTP500
    :observed_from: api:active
    :depends_on: none
    :platforms: linux
    :exact_form_of: SWARM-SCOPED-CONFIG-FROM-MISSING-START
    :pins: SWARM-SCOPED-ATTACH-FAILURE-TIMEOUT
    :concerns: SWARM-SCOPED-CONFIG-FROM-MISSING-START
    :violates: PRIN-ERRORS-ACTIONABLE

    *Applies to:* An attachable Swarm-scoped network of a node-local driver, created with
    :api-field:`ConfigFrom` naming a config-only network, and a node whose availability is
    ``active`` that does not have that config-only network, with a created or stopped container on
    the network

    On that node, :api-op:`POST /containers/{id}/start` for the container responds 500 after 20
    seconds, with a message that contains :api-error:`attaching to network failed, make sure your
    network options are correct and check manager logs: context deadline exceeded`.

.. expectation:: A node without the config-only network fails a connect
    :id: SWARM-SCOPED-CONFIG-FROM-MISSING-CONNECT
    :observed_from: api:active
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-NETWORK-CONFIG-FROM, PRIN-FAILS-CLOSED

    *Applies to:* A Swarm-scoped network of a node-local driver, created with
    :api-field:`ConfigFrom` naming a config-only network, and a node whose availability is
    ``active`` that does not have that config-only network, with a running standalone container that
    is not on the network

    On that node, :api-op:`POST /networks/{id}/connect` to the network, for the container, responds
    with a 4xx or 5xx status. :api-op:`GET /containers/{id}/json` then does not list the network in
    :api-field:`NetworkSettings.Networks`.

.. expectation:: Without the config-only network, connect responds 500 after 20 seconds
    :id: SWARM-SCOPED-CONFIG-FROM-MISSING-CONNECT-HTTP500
    :observed_from: api:active
    :depends_on: none
    :platforms: linux
    :exact_form_of: SWARM-SCOPED-CONFIG-FROM-MISSING-CONNECT
    :pins: SWARM-SCOPED-ATTACH-FAILURE-TIMEOUT
    :concerns: SWARM-SCOPED-CONFIG-FROM-MISSING-CONNECT
    :violates: PRIN-ERRORS-ACTIONABLE

    *Applies to:* An attachable Swarm-scoped network of a node-local driver, created with
    :api-field:`ConfigFrom` naming a config-only network, and a node whose availability is
    ``active`` that does not have that config-only network, with a running standalone container that
    is not on the network

    On that node, :api-op:`POST /networks/{id}/connect` to the network, for the container, responds
    500 after 20 seconds, with a message that contains :api-error:`attaching to network failed, make
    sure your network options are correct and check manager logs: context deadline exceeded`.

.. expectation:: Attachment failures for standalone containers time out
    :id: SWARM-SCOPED-ATTACH-FAILURE-TIMEOUT
    :observed_from: api:mgr, api:wkr, log
    :depends_on: none
    :platforms: linux, windows
    :concerns: SWARM-SCOPED-DRAINED-START, SWARM-SCOPED-DRAINED-CONNECT, SWARM-OVL-STATIC-ADDRESS-IN-USE, SWARM-SCOPED-CONFIG-FROM-MISSING-START, SWARM-SCOPED-CONFIG-FROM-MISSING-CONNECT, SWARM-ENC-NO-XFRM-CTR-START, SWARM-CHURN-DELETE-CTR-START, SWARM-SCALE-UNALLOCATABLE-CTR-START, SWARM-OVL-STATIC-ADDRESS-OUTSIDE-ATTACHED
    :violates: REQ-NET-API-ERROR-STATUS, PRIN-ERRORS-ACTIONABLE

    If the attachment of a standalone container to an attachable network fails,
    :api-op:`POST /containers/{id}/start` waits 20 seconds. It then responds 500 with
    :api-error:`attaching to network failed, make sure your network options are correct and check
    manager logs: context deadline exceeded`. This occurs in nine cases: (1) On a Linux node, the
    network is :drvopt:`encrypted`, and the host's kernel does not offer the XFRM netlink interface.
    (2) The network is an overlay network created with :api-field:`ConfigFrom`. (3) The network is
    deleted after the request has found it, but before the attachment task is created. (4) The
    container's static address is in use or is outside the network's subnets. (5) The network is not
    allocated. (6) The container's node has availability ``drain`` or ``pause``. (7) The network is
    a Swarm-scoped network of a node-local driver created with :api-field:`ConfigFrom`, and the
    container's node does not have the config-only network that :api-field:`ConfigFrom` names. (8)
    The network is a Swarm-scoped network of the :driver:`host` driver. (9) The network's driver is
    a managed plugin that the container's node does not have enabled.
    :api-op:`POST /networks/{id}/connect` for a running container also waits 20 seconds in each of
    these cases, and then responds 500 with the same message. In cases 1, 2, 7 and 8, while the
    request waits, :api-op:`GET /tasks` with the :api-query:`runtime=attachment` filter lists the
    attachment task as rejected, and its :api-field:`Status.Err` contains the error that a service's
    task gets for the same cause: :api-error:`cannot join secure network: required modules to
    install IPSEC rules are missing on host` in case 1, :api-error:`user-specified configurations
    are not supported if the network depends on a configuration network` in case 2,
    :api-error:`configuration network "<name>" does not exist` in case 7, and :api-error:`only one
    instance of "host" network is allowed` in case 8. The task is deleted before the request
    responds. After that, that error is only in the node's daemon log, in an entry at the error
    level that contains it. In case 3, the attachment task is deleted, and no manager's daemon log
    reports the failure. In cases 4 and 5, the attachment task stays in the ``new`` state until it
    is deleted. :api-op:`GET /tasks` reports that state. The reason then appears only in the leader
    manager's log, in an entry that contains :log:`task allocation failure`. In cases 6 and 9, the
    attachment task stays in the ``pending`` state until it is deleted. :api-op:`GET /tasks` reports
    that state.

.. expectation:: A Swarm-scoped host network is created, and every attachment fails
    :id: SWARM-SCOPED-HOST-DRIVER-UNUSABLE
    :observed_from: api:mgr, api:active
    :depends_on: none
    :platforms: linux
    :concerns: SWARM-SCOPED-ATTACH-FAILURE-TIMEOUT
    :violates: PRIN-FAIL-FAST

    :api-op:`POST /networks/create` on a manager for a Swarm-scoped network whose
    :api-field:`Driver` is :driver:`host` responds 201. Every attachment to the network then fails,
    on every node. The tasks of a service on the network are rejected, and their error in
    :api-op:`GET /tasks` contains :api-error:`only one instance of "host" network is allowed`. When
    the network is attachable, :api-op:`POST /containers/{id}/start` for a standalone container on
    the network, and :api-op:`POST /networks/{id}/connect` to the network for a running container,
    fail as :need:`SWARM-SCOPED-ATTACH-FAILURE-TIMEOUT` states.

.. expectation:: After an attachment times out, the next request skips the attachment
    :id: SWARM-SCOPED-ATTACH-RETRY-SKIPS-ATTACHMENT
    :observed_from: api:mgr, api:wkr
    :depends_on: none
    :platforms: linux, windows
    :narrows: SWARM-SCOPED-ATTACHABLE-START, SWARM-SCOPED-ATTACHABLE-CONNECT, SWARM-SCOPED-ATTACHABLE-CONNECT-CLI, SWARM-SCOPED-ATTACH-FAILURE-TIMEOUT, SWARM-SCOPED-DRAINED-START-HTTP500, SWARM-SCOPED-DRAINED-CONNECT, SWARM-SCOPED-DRAINED-CONNECT-HTTP500, SWARM-SCOPED-DRAINED-STANDALONE-ATTACH, SWARM-SCOPED-CONFIG-FROM-MISSING-START-HTTP500, SWARM-SCOPED-CONFIG-FROM-MISSING-CONNECT-HTTP500, SWARM-SCOPED-CONFIG-FROM-OVERLAY-UNUSABLE, SWARM-OVL-STATIC-ADDRESS-IN-USE-HTTP500, SWARM-ENC-NO-XFRM-CTR-START-HTTP500, SWARM-SCALE-UNALLOCATABLE-CTR-START-HTTP500, SWARM-CHURN-DELETE-CTR-START-REFUSED-HTTP500
    :violates: REQ-SWARM-NETWORK-ATTACHABLE, PRIN-REPEATABLE

    *Applies to:* A standalone container, an attachable Swarm-scoped network and a node, where the
    container's last :api-op:`POST /containers/{id}/start` on the node, or its last :api-op:`POST
    /networks/{id}/connect` to the network on the node, responded 500 after 20 seconds with
    :api-error:`attaching to network failed, make sure your network options are correct and check
    manager logs: context deadline exceeded`

    The next such request on that node, a start of the container or a connect of the container that
    names the network as the timed-out connect did, does not wait for an attachment where the node
    has no local attachment to the network, whatever the node's availability. It responds 404 at
    once, with a message that contains :api-error:`network <ref> not found`. For a start, ``<ref>``
    is the network's full ID if the container has been started before, and otherwise the network
    name or ID that the container was created with. For a connect, it is the ``{id}`` of the
    request. Where the node has a local attachment to the network, the next such connect responds
    200 at once. :api-op:`GET /containers/{id}/json` then lists the network in
    :api-field:`NetworkSettings.Networks`, and :api-op:`GET /tasks` on a manager with the
    :api-query:`runtime=attachment` filter lists no attachment task for the container. On a node
    whose availability is still ``drain`` or ``pause``, that connect responds 200 where the same
    request in the same state responded 500.

.. expectation:: An attached node reports its config-only network's IPAM configuration
    :id: SWARM-SCOPED-CONFIG-FROM-INSPECT-ATTACHED
    :observed_from: api:mgr, api:wkr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-NETWORK-CONFIG-FROM, REQ-NET-CONFIG-FROM

    *Applies to:* A Swarm-scoped network of a node-local driver, created with
    :api-field:`ConfigFrom` naming a config-only network, and a node with a local attachment to the
    network

    On that node, :api-op:`GET /networks/{id}` for the network reports the :api-field:`IPAM`
    configuration of that node's config-only network.

.. expectation:: An unattached manager reports an empty IPAM for a ConfigFrom network
    :id: SWARM-SCOPED-CONFIG-FROM-INSPECT-UNATTACHED
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :rationale: RAT-SWARM-NODE-REPORTS-OWN-STATE

    *Applies to:* A Swarm-scoped network of a node-local driver, created with
    :api-field:`ConfigFrom` naming a config-only network, and a manager that has that config-only
    network and does not have a local attachment to the network

    On that manager, :api-op:`GET /networks/{id}` for the network reports an empty
    :api-field:`IPAM`.

.. expectation:: Deleting a config-only network in use on the node is refused
    :id: SWARM-SCOPED-CONFIG-ONLY-DELETE-REFUSED
    :observed_from: api:mgr, api:wkr
    :depends_on: none
    :platforms: linux
    :refines: REQ-NET-API-ERROR-STATUS, REQ-NET-CONFIG-ONLY-DELETE-IN-USE

    *Applies to:* A Swarm-scoped network of a node-local driver, created with
    :api-field:`ConfigFrom` naming a config-only network, and a node with a local attachment to the
    network

    On that node, :api-op:`DELETE /networks/{id}` for the config-only network responds with a 4xx
    status. :api-op:`GET /networks` on that node then still lists the config-only network.

.. expectation:: Deleting a config-only network in use on the node responds 403
    :id: SWARM-SCOPED-CONFIG-ONLY-DELETE-REFUSED-HTTP403
    :observed_from: api:mgr, api:wkr
    :depends_on: none
    :platforms: linux
    :exact_form_of: SWARM-SCOPED-CONFIG-ONLY-DELETE-REFUSED

    *Applies to:* A Swarm-scoped network of a node-local driver, created with
    :api-field:`ConfigFrom` naming a config-only network, and a node with a local attachment to the
    network

    On that node, :api-op:`DELETE /networks/{id}` for the config-only network responds 403.
    :api-op:`GET /networks` on that node then still lists the config-only network.

.. expectation:: docker network rm fails for a config-only network in use on the node
    :id: SWARM-SCOPED-CONFIG-ONLY-DELETE-REFUSED-CLI
    :observed_from: api:mgr, api:wkr
    :depends_on: cli
    :platforms: linux
    :refines: REQ-NET-CONFIG-ONLY-DELETE-IN-USE
    :cli_for: SWARM-SCOPED-CONFIG-ONLY-DELETE-REFUSED

    *Applies to:* A Swarm-scoped network of a node-local driver, created with
    :api-field:`ConfigFrom` naming a config-only network, and a node with a local attachment to the
    network

    On that node, :cli:`docker network rm` for the config-only network exits non-zero and prints the
    daemon's error.

.. expectation:: A config-only network that the node does not use is deleted
    :id: SWARM-SCOPED-CONFIG-ONLY-DELETE-UNUSED
    :observed_from: api:mgr, api:wkr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-NETWORK-DELETE, REQ-NET-CONFIG-ONLY-DELETE-IN-USE, PRIN-NODE-LOCAL

    *Applies to:* A Swarm-scoped network of a node-local driver, created with
    :api-field:`ConfigFrom` naming a config-only network, and a node that has the config-only
    network, on which no network that is not Swarm-scoped names that config-only network in
    :api-field:`ConfigFrom`, and that has no local attachment to a Swarm-scoped network that names
    it in :api-field:`ConfigFrom`

    On that node, :api-op:`DELETE /networks/{id}` for the config-only network responds 204.

.. expectation:: The enable_ipv4 option false on a node-local Swarm network responds 400
    :id: SWARM-SCOPED-NODE-LOCAL-V4-DISABLED
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :pins: SWARM-SCOPED-NODE-LOCAL-V6-ONLY

    :api-op:`POST /networks/create` for a Swarm-scoped network of a node-local driver, without
    :api-field:`EnableIPv4` and with the :drvopt:`com.docker.network.enable_ipv4` option set to
    ``false``, responds 400 with :api-error:`IPv4 cannot be disabled in a Swarm scoped network`, at
    any API version.

.. expectation:: docker network create --scope=swarm fails with IPv4 disabled
    :id: SWARM-SCOPED-NODE-LOCAL-V4-DISABLED-CLI
    :observed_from: api:mgr
    :depends_on: cli
    :platforms: linux
    :cli_for: SWARM-SCOPED-NODE-LOCAL-V6-ONLY
    :concerns: SWARM-SCOPED-NODE-LOCAL-V6-ONLY
    :violates: REQ-NET-IPV6-ONLY

    :cli:`docker network create --scope=swarm` with a node-local driver and :cli-flag:`--ipv4=false`
    exits non-zero and prints the daemon's error.

.. expectation:: A node-local Swarm-scoped network with EnableIPv4 false responds 400
    :id: SWARM-SCOPED-NODE-LOCAL-V6-ONLY
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :concerns: SWARM-SCOPED-NODE-LOCAL-V4-DISABLED, SWARM-SCOPED-NODE-LOCAL-V4-DISABLED-CLI, SWARM-OVL-V4-DISABLED
    :violates: REQ-NET-IPV6-ONLY

    :api-op:`POST /networks/create` for a Swarm-scoped network of a node-local driver with
    :api-field:`EnableIPv4` false, at API version 1.48 or later, responds 400 with :api-error:`IPv4
    cannot be disabled in a Swarm scoped network`, as it does for an overlay network. It responds
    the same way to a request with :api-field:`ConfigFrom` that names a config-only network with
    :api-field:`EnableIPv4` false.

.. expectation:: Create accepts IPAM.Config and Options for a node-local network
    :id: SWARM-SCOPED-NODE-LOCAL-IPAM-CREATE
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-NET-API-BACKWARD-COMPATIBLE

    :api-op:`POST /networks/create` for a Swarm-scoped network of a node-local driver, without
    :api-field:`ConfigFrom`, with a subnet and gateway in :api-field:`IPAM.Config` and with the
    driver's own options in :api-field:`Options`, responds 201.

.. expectation:: An unattached manager reports empty IPAM and no Options for a node-local network
    :id: SWARM-SCOPED-NODE-LOCAL-IPAM-MANAGER
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :pins: SWARM-SCOPED-NODE-LOCAL-IPAM-OPTIONS
    :concerns: SWARM-SCOPED-NODE-LOCAL-IPAM-CREATE
    :violates: REQ-NET-INSPECT-CONFIGURATION

    *Applies to:* A manager

    :api-op:`POST /networks/create` for a Swarm-scoped network of a node-local driver without
    :api-field:`ConfigFrom`, with a subnet and gateway in :api-field:`IPAM.Config`, and with
    :api-field:`Options` that hold only the driver's own options, at least one of them, such as
    :drvopt:`com.docker.network.bridge.*`, responds 201. :api-op:`GET /networks/{id}` on that
    manager then reports an empty :api-field:`IPAM` and no :api-field:`Options`.

.. expectation:: A task node reports a subnet of its own and no create-time Options
    :id: SWARM-SCOPED-NODE-LOCAL-IPAM-TASK-NODE
    :observed_from: api:mgr, api:wkr
    :depends_on: none
    :platforms: linux
    :pins: SWARM-SCOPED-NODE-LOCAL-IPAM-OPTIONS
    :concerns: SWARM-SCOPED-NODE-LOCAL-IPAM-CREATE
    :violates: REQ-NET-INSPECT-CONFIGURATION

    *Applies to:* A Swarm-scoped network of a node-local driver that was created without
    :api-field:`ConfigFrom`, with a subnet outside the node's default address pools and a gateway in
    :api-field:`IPAM.Config`, and with one of the driver's own options in :api-field:`Options` at a
    value other than its default

    On a node that runs a task on that network, :api-op:`GET /networks/{id}` does not report the
    subnet or any of the :api-field:`Options` given at create. It reports a subnet from the node's
    default address pools.

.. expectation:: IPAM.Config and Options of a node-local Swarm-scoped network are dropped
    :id: SWARM-SCOPED-NODE-LOCAL-IPAM-OPTIONS
    :observed_from: api:mgr, api:wkr
    :depends_on: none
    :platforms: linux
    :within: SWARM-CEIL-REASSIGNMENT
    :concerns: SWARM-SCOPED-NODE-LOCAL-IPAM-MANAGER, SWARM-SCOPED-NODE-LOCAL-IPAM-TASK-NODE, SWARM-SCOPED-NODE-LOCAL-IPAM-CREATE
    :violates: PRIN-FAILS-CLOSED

    :api-op:`POST /networks/create` accepts a valid :api-field:`IPAM.Config` and
    :api-field:`Options` for a Swarm-scoped network of a node-local driver created without
    :api-field:`ConfigFrom`, but they are not kept. Each node creates the network with its own
    defaults: a Linux node takes the subnet from its default address pools and the options from the
    driver's defaults.

.. expectation:: A manager accepts ConfigFrom naming a config-only network only it has
    :id: SWARM-SCOPED-CONFIG-FROM-MANAGER-ONLY
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :pins: SWARM-SCOPED-CONFIG-ONLY-ONE-NODE

    *Applies to:* A manager with a config-only network that no other node has

    On that manager, :api-op:`POST /networks/create` for a Swarm-scoped network of a node-local
    driver, with :api-field:`ConfigFrom` naming the config-only network, responds 201.

.. expectation:: A manager without the config-only network responds 404 to ConfigFrom
    :id: SWARM-SCOPED-CONFIG-FROM-CREATE-MISSING
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :pins: SWARM-SCOPED-CONFIG-ONLY-ONE-NODE

    *Applies to:* A manager that does not have a given config-only network, and another manager that
    has it

    On the first manager, :api-op:`POST /networks/create` for a Swarm-scoped network of any driver,
    with :api-field:`ConfigFrom` naming that config-only network, responds 404.

.. expectation:: An overlay network created with ConfigFrom responds 201
    :id: SWARM-SCOPED-CONFIG-FROM-OVERLAY
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :pins: SWARM-SCOPED-CONFIG-FROM-OVERLAY-UNUSABLE

    *Applies to:* A manager with a config-only network

    On that manager, :api-op:`POST /networks/create` for an overlay network, with
    :api-field:`ConfigFrom` naming the config-only network, responds 201.

.. expectation:: An overlay with ConfigFrom is created, and every attachment fails
    :id: SWARM-SCOPED-CONFIG-FROM-OVERLAY-UNUSABLE
    :observed_from: api:mgr, api:active
    :depends_on: none
    :platforms: linux, windows
    :concerns: SWARM-SCOPED-CONFIG-FROM-OVERLAY, SWARM-SCOPED-CONFIG-FROM-TASKS
    :violates: REQ-NET-CONFIG-FROM, PRIN-ERRORS-ACTIONABLE, REQ-NET-API-ERROR-STATUS

    :api-op:`POST /networks/create` for an overlay network with :api-field:`ConfigFrom` responds 201
    only if the config-only network exists on the manager that handles the request. It responds 404
    otherwise. Every attachment to the network then fails. The tasks of a service on the network are
    rejected, and their error in :api-op:`GET /tasks` contains :api-error:`user-specified
    configurations are not supported if the network depends on a configuration network`. When the
    network is attachable, :api-op:`POST /containers/{id}/start` for a standalone container on the
    network responds 500 after 20 seconds, with a message that contains :api-error:`attaching to
    network failed, make sure your network options are correct and check manager logs: context
    deadline exceeded`.

.. expectation:: docker network create --config-from fails without the config-only one
    :id: SWARM-SCOPED-CONFIG-FROM-CREATE-MISSING-CLI
    :observed_from: api:mgr
    :depends_on: cli
    :platforms: linux
    :cli_for: SWARM-SCOPED-CONFIG-FROM-CREATE-MISSING
    :pins: SWARM-SCOPED-CONFIG-ONLY-ONE-NODE

    *Applies to:* A manager that does not have a given config-only network

    On that manager, :cli:`docker network create --scope=swarm --config-from` naming that network
    exits non-zero and prints the daemon's error. So does :cli:`docker network create --driver
    overlay --config-from` naming that network.

.. expectation:: ConfigFrom is checked only on the manager that handles the create
    :id: SWARM-SCOPED-CONFIG-ONLY-ONE-NODE
    :observed_from: api:mgr, api:active
    :depends_on: none
    :platforms: linux, windows
    :rationale: RAT-SWARM-CONFIG-ONLY-PER-NODE

    :api-op:`POST /networks/create` for a Swarm-scoped network with :api-field:`ConfigFrom` responds
    201 only when the config-only network exists on the manager that handles the request, whether or
    not that manager runs tasks on the network. Each node that runs a task on the network uses its
    own config-only network of that name. Task placement does not exclude nodes that do not have the
    config-only network: when the only nodes that can take a service's task do not have it, the task
    is placed on one of them and rejected. On such a node, when the network is attachable,
    :api-op:`POST /containers/{id}/start` for a created or stopped standalone container on the
    network responds with a 4xx or 5xx status.

.. expectation:: A manager without the driver plugin refuses the network
    :id: SWARM-SCOPED-PLUGIN-CREATE-MISSING
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-NETWORK-PLUGIN-DRIVERS, REQ-NET-API-ERROR-STATUS

    *Applies to:* A manager that does not have a given managed plugin

    On that manager, :api-op:`POST /networks/create` for a Swarm-scoped network whose driver is that
    plugin responds with a 4xx status.

.. expectation:: A manager without the driver plugin responds 404
    :id: SWARM-SCOPED-PLUGIN-CREATE-MISSING-HTTP404
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :exact_form_of: SWARM-SCOPED-PLUGIN-CREATE-MISSING

    *Applies to:* A manager that does not have a given managed plugin

    On that manager, :api-op:`POST /networks/create` for a Swarm-scoped network whose driver is that
    plugin responds 404.

.. expectation:: A manager without the driver plugin says the plugin is not found
    :id: SWARM-SCOPED-PLUGIN-CREATE-MISSING-MSG
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :exact_form_of: SWARM-SCOPED-PLUGIN-CREATE-MISSING

    *Applies to:* A manager that does not have a given managed plugin

    On that manager, :api-op:`POST /networks/create` for a Swarm-scoped network whose driver is that
    plugin responds 404, with a message that contains :api-error:`plugin "<name>" not found`.

.. expectation:: A Swarm-scoped network on a legacy v1 driver plugin is refused
    :id: SWARM-SCOPED-PLUGIN-LEGACY
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-NETWORK-PLUGIN-DRIVERS, REQ-NET-API-ERROR-STATUS

    *Applies to:* A manager with a legacy network plugin, a v1 plugin that the daemon finds in its
    plugin directories

    On that manager, :api-op:`POST /networks/create` for a Swarm-scoped network whose driver is that
    plugin responds with a 4xx status.

.. expectation:: A Swarm-scoped network on a legacy v1 driver plugin responds 400
    :id: SWARM-SCOPED-PLUGIN-LEGACY-HTTP400
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :exact_form_of: SWARM-SCOPED-PLUGIN-LEGACY

    *Applies to:* A manager with a legacy network plugin, a v1 plugin that the daemon finds in its
    plugin directories

    On that manager, :api-op:`POST /networks/create` for a Swarm-scoped network whose driver is that
    plugin responds 400.

.. expectation:: A legacy plugin's refusal says it is not supported in swarm mode
    :id: SWARM-SCOPED-PLUGIN-LEGACY-MSG
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :exact_form_of: SWARM-SCOPED-PLUGIN-LEGACY

    *Applies to:* A manager with a legacy network plugin, a v1 plugin that the daemon finds in its
    plugin directories

    On that manager, :api-op:`POST /networks/create` for a Swarm-scoped network whose driver is that
    plugin responds 400, with a message that contains :api-error:`is not supported in swarm mode`.

.. expectation:: Tasks on a plugin network run only on nodes that have the plugin
    :id: SWARM-SCOPED-PLUGIN-PLACEMENT
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-NETWORK-PLUGIN-DRIVERS

    *Applies to:* A Swarm whose leader manager has a given managed network plugin, a Swarm-scoped
    network whose driver is that plugin, a service on that network, a node whose availability is
    ``active`` that has the plugin installed and enabled, and nodes whose availability is ``active``
    that do not have it installed, or have it disabled

    :api-op:`GET /tasks` reports the service's tasks as ``running``, each on a node that has the
    plugin enabled. It reports no task of the service as running on a node that does not.

.. expectation:: Without the plugin on the leader, another manager's create responds 400
    :id: SWARM-SCOPED-PLUGIN-LEADER-MISSING
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :pins: SWARM-SCOPED-PLUGIN-ON-LEADER
    :concerns: SWARM-SCOPED-PLUGIN-IPAM-ON-LEADER
    :violates: REQ-SWARM-NETWORK-PLUGIN-DRIVERS, PRIN-ERRORS-ACTIONABLE

    *Applies to:* A Swarm whose leader manager does not have a given managed plugin, and another
    manager that has it

    On that other manager, :api-op:`POST /networks/create` for a Swarm-scoped network whose driver
    or IPAM driver is that plugin responds 400, with a message that contains :api-error:`error
    during lookup of plugin <name>`. The message does not name the leader manager or say that it
    lacks the plugin.

.. expectation:: An overlay with the leader's IPAM plugin is allocated from any manager
    :id: SWARM-SCOPED-PLUGIN-IPAM-ON-LEADER
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :pins: SWARM-SCOPED-PLUGIN-ON-LEADER
    :within: SWARM-CEIL-ALLOCATION

    *Applies to:* A Swarm whose leader manager has a given managed plugin

    :api-op:`POST /networks/create` for an overlay network whose IPAM driver is that plugin responds 201.
    This holds whether or not the manager that handles the request has the plugin. The network is
    then allocated.

.. expectation:: An IPAM plugin named without its tag responds 201 but is never allocated
    :id: SWARM-SCOPED-PLUGIN-UNTAGGED-IPAM
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :pins: SWARM-SCOPED-PLUGIN-UNTAGGED-NAME
    :within: SWARM-CEIL-ALLOCATION
    :concerns: SWARM-SCOPED-PLUGIN-IPAM-ON-LEADER
    :violates: REQ-SWARM-NETWORK-PLUGIN-DRIVERS

    *Applies to:* A Swarm whose leader manager has a managed plugin that provides an IPAM driver

    :api-op:`POST /networks/create` for an overlay network whose IPAM driver is the plugin's name
    without its tag responds 201. This holds whether or not the manager that handles the request has
    the plugin. The network is never allocated.

.. expectation:: A new leader without the plugin still reports the allocation
    :id: SWARM-SCOPED-PLUGIN-LEADER-CHANGE-ALLOCATION
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :pins: SWARM-SCOPED-PLUGIN-ON-LEADER
    :within: SWARM-CEIL-ALLOCATION

    *Applies to:* An allocated network, and a manager that does not have the driver or IPAM plugin
    of that network

    Leadership moves to that manager. :api-op:`GET /networks/{id}` still reports the network's
    allocation.

.. expectation:: Under a new leader without the plugin, tasks stay in state new
    :id: SWARM-SCOPED-PLUGIN-LEADER-CHANGE-TASKS
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :pins: SWARM-SCOPED-PLUGIN-ON-LEADER
    :within: SWARM-CEIL-ALLOCATION

    *Applies to:* An allocated network, and a manager that does not have the driver or IPAM plugin
    of that network

    Leadership moves to that manager. The tasks that a service on the network gets after the change,
    from :api-op:`POST /services/create` or from an update or scale-up of an existing service, stay
    in the ``new`` state.

.. expectation:: A new leader without the plugin logs failed allocating network
    :id: SWARM-SCOPED-PLUGIN-LEADER-CHANGE-LOG
    :observed_from: log
    :depends_on: none
    :platforms: linux
    :pins: SWARM-SCOPED-PLUGIN-ON-LEADER
    :within: SWARM-CEIL-ALLOCATION

    *Applies to:* An allocated network, and a manager that does not have the driver or IPAM plugin
    of that network

    Leadership moves to that manager. The daemon log of the leader manager contains :log:`failed
    allocating network` and the network's ID.

.. expectation:: A plugin network is allocated only while the leader has the plugin
    :id: SWARM-SCOPED-PLUGIN-ON-LEADER
    :observed_from: api:mgr, log
    :depends_on: none
    :platforms: linux
    :within: SWARM-CEIL-ALLOCATION
    :narrows: SWARM-SCOPED-CREATE
    :concerns: SWARM-SCOPED-PLUGIN-LEADER-MISSING, SWARM-SCOPED-PLUGIN-IPAM-ON-LEADER, SWARM-SCOPED-PLUGIN-LEADER-CHANGE-ALLOCATION, SWARM-SCOPED-PLUGIN-LEADER-CHANGE-TASKS, SWARM-SCOPED-PLUGIN-LEADER-CHANGE-LOG
    :violates: REQ-SWARM-NETWORK-PLUGIN-DRIVERS, REQ-SWARM-TASK-ERRORS

    A Swarm-scoped network whose driver or IPAM driver is a managed plugin is created and allocated
    only while the leader manager has the plugin. After leadership moves to a manager without the
    plugin, the network still reports its allocation. But the tasks that services on it get after
    the change stay in the ``new`` state that :api-op:`GET /tasks` reports. Only the leader
    manager's log reports the failure.

.. expectation:: A driver plugin named without its tag responds 500
    :id: SWARM-SCOPED-PLUGIN-UNTAGGED-DRIVER
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :pins: SWARM-SCOPED-PLUGIN-UNTAGGED-NAME

    *Applies to:* A manager with a managed plugin that provides a network driver

    On that manager, :api-op:`POST /networks/create` for a Swarm-scoped network whose
    :api-field:`Driver` is the plugin's name without its tag, such as ``name`` for ``name:latest``,
    responds 500, with a message that contains :api-error:`could not resolve driver`.

.. expectation:: A plugin named without its tag responds 500 or is never allocated
    :id: SWARM-SCOPED-PLUGIN-UNTAGGED-NAME
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :within: SWARM-CEIL-ALLOCATION
    :narrows: SWARM-SCOPED-CREATE
    :concerns: SWARM-SCOPED-PLUGIN-UNTAGGED-DRIVER, SWARM-SCOPED-PLUGIN-UNTAGGED-IPAM
    :violates: REQ-SWARM-NETWORK-PLUGIN-DRIVERS, REQ-NET-API-ERROR-STATUS

    :api-op:`POST /networks/create` for a Swarm-scoped network whose network driver is a managed
    plugin's name without its tag responds 500. With an IPAM driver that is a managed plugin's name
    without its tag, the request responds 201, and the network is never allocated.

.. expectation:: A task whose ifname the OS refuses fails with that name in its error
    :id: SWARM-SCOPED-IFNAME-REFUSED
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-TASK-ERRORS, PRIN-ERRORS-ACTIONABLE

    *Applies to:* Linux nodes, and a service whose network attachment requests a
    :drvopt:`com.docker.network.endpoint.ifname` that the OS refuses

    Each task of the service fails. Its error in :api-op:`GET /tasks` contains the requested name.

.. expectation:: A service that requests any ifname is created
    :id: SWARM-SCOPED-IFNAME-ANY-VALUE
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :pins: SWARM-SCOPED-IFACE-NAME-OPTION-UNCHECKED

    For any value of :drvopt:`com.docker.network.endpoint.ifname`, :api-op:`POST /services/create`
    for a service whose network attachment requests that option responds 201, and :api-op:`GET
    /services` then lists the service.

.. expectation:: A task with a refused ifname says renaming the interface failed
    :id: SWARM-SCOPED-IFNAME-REFUSED-MSG
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :exact_form_of: SWARM-SCOPED-IFNAME-REFUSED
    :pins: SWARM-SCOPED-IFACE-NAME-OPTION-UNCHECKED

    *Applies to:* Linux nodes, and a service whose network attachment requests a
    :drvopt:`com.docker.network.endpoint.ifname` that the OS refuses

    The error of each task of the service in :api-op:`GET /tasks` contains :api-error:`error
    renaming interface`.

.. expectation:: ifname is not checked before a task starts
    :id: SWARM-SCOPED-IFACE-NAME-OPTION-UNCHECKED
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :concerns: SWARM-SCOPED-IFNAME-ANY-VALUE, SWARM-SCOPED-IFNAME-REFUSED
    :violates: PRIN-FAIL-FAST

    :api-op:`POST /services/create` and :api-op:`POST /services/{id}/update` accept any
    interface name in the driver options of a network attachment. Every task with an interface name
    that the OS refuses fails at start. Under the default restart policy, such a task is replaced
    indefinitely.

.. expectation:: A manager creates a service on host with an overlay or ingress port
    :id: SWARM-SCOPED-HOST-NETWORK-COMBINED
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :pins: SWARM-SCOPED-HOST-NETWORK-COMBINED-ACCEPTED

    :api-op:`POST /services/create` for a service attached to the :network:`host` network and to an
    overlay network responds 201. So does a request for a service on :network:`host` that publishes
    a port in ingress mode.

.. expectation:: A service on host with an overlay or ingress port is created, tasks fail
    :id: SWARM-SCOPED-HOST-NETWORK-COMBINED-ACCEPTED
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :concerns: SWARM-SCOPED-HOST-NETWORK-COMBINED
    :violates: PRIN-FAIL-FAST

    :api-op:`POST /services/create` for a service attached to the :network:`host` network
    and to an overlay network responds 201. :api-op:`POST /services/create` for a service on
    :network:`host` that publishes a port in ingress mode also responds 201. Every task of such a
    service fails at start. Under the default restart policy, such a task is replaced indefinitely.

.. _swarm-networking-overlay-networks:

Overlay networks
================

The expectations in this subsection depend on the addresses that are allocated for a network: its
subnets, task addresses and VIPs. These addresses are allocated only for an overlay network. Where
an expectation says that :api-op:`GET /services/{id}` then reports a VIP, it reports it within the
:need:`[[title|lower]] ceiling ([[seconds]] s) <SWARM-CEIL-ALLOCATION>` after the request responds.

.. expectation:: An overlay network with IPv4 disabled responds 400
    :id: SWARM-OVL-V4-DISABLED
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :concerns: SWARM-OVL-V4-DISABLED-CLI
    :violates: REQ-NET-IPV6-ONLY

    On a manager, :api-op:`POST /networks/create` for an overlay network with
    :api-field:`EnableIPv4` false, at API version 1.48 or later, responds 400.

.. expectation:: docker network create --driver overlay fails with IPv4 disabled
    :id: SWARM-OVL-V4-DISABLED-CLI
    :observed_from: api:mgr
    :depends_on: cli
    :platforms: linux
    :cli_for: SWARM-OVL-V4-DISABLED
    :concerns: SWARM-OVL-V4-DISABLED
    :violates: REQ-NET-IPV6-ONLY

    On a manager, :cli:`docker network create --driver overlay --ipv4=false` exits non-zero and
    prints the daemon's error.

.. expectation:: An overlay network with the enable_ipv4 option false responds 400
    :id: SWARM-OVL-V4-DISABLED-OPTION
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :pins: SWARM-OVL-V4-DISABLED

    On a manager, :api-op:`POST /networks/create` for an overlay network without
    :api-field:`EnableIPv4`, with the :drvopt:`com.docker.network.enable_ipv4` option set to
    ``false``, responds 400, at any API version.

.. expectation:: A subnet's VXLAN traffic carries the VNI at its position in vxlanid_list
    :id: SWARM-OVL-VNI-LIST
    :observed_from: underlay
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-OVERLAY-VNI-LIST
    :within: SWARM-CEIL-ALLOCATION, SWARM-CEIL-REASSIGNMENT

    A user-specified :drvopt:`vxlanid_list` is honored: for each *n* at which the network has a
    subnet and the list has a VNI, the VXLAN traffic of the network's *n*-th subnet, in the order of
    the network's subnets, carries the *n*-th VNI of the list. On a Windows node, this holds for
    VNIs of 4096 or more.

.. expectation:: A short vxlanid_list gets auto-allocated VNIs for the remaining subnets
    :id: SWARM-OVL-VNI-LIST-SHORT
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-OVERLAY-VNI-LIST, REQ-SWARM-OVERLAY-VNI-REPORTED
    :within: SWARM-CEIL-ALLOCATION

    :api-op:`POST /networks/create` for an overlay network whose :drvopt:`vxlanid_list` has fewer
    VNIs than the network has subnets responds 201. Once the network is allocated, :api-op:`GET
    /networks/{id}` on a manager reports the same number of VNIs in :drvopt:`vxlanid_list` as the
    overlay has subnets. These are the requested VNIs, then an auto-allocated VNI for each remaining
    subnet.

.. expectation:: A long vxlanid_list gives the network its first VNIs
    :id: SWARM-OVL-VNI-LIST-LONG
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-OVERLAY-VNI-LIST, REQ-SWARM-OVERLAY-VNI-REPORTED
    :within: SWARM-CEIL-ALLOCATION

    :api-op:`POST /networks/create` for an overlay network whose :drvopt:`vxlanid_list` has more
    VNIs than the network has subnets responds 201. Once the network is allocated, :api-op:`GET
    /networks/{id}` on a manager reports the same number of VNIs in :drvopt:`vxlanid_list` as the
    overlay has subnets. These are the first VNIs of the requested list.

.. expectation:: The surplus VNIs of a long vxlanid_list are dropped without an error
    :id: SWARM-OVL-VNI-LIST-LONG-SURPLUS-DROPPED
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :within: SWARM-CEIL-ALLOCATION
    :concerns: SWARM-OVL-VNI-LIST-LONG
    :violates: PRIN-FAILS-CLOSED

    :api-op:`POST /networks/create` for an overlay network whose :drvopt:`vxlanid_list` has more
    VNIs than the network has subnets responds 201, and the VNIs beyond the number of subnets are
    dropped. :api-op:`GET /networks/{id}` does not report them. They are not reserved:
    :api-op:`POST /networks/create` for another overlay network whose :drvopt:`vxlanid_list` has
    one of them responds 201, and that network is then allocated.

.. expectation:: An overlay network with another overlay's VNI is created without a warning
    :id: SWARM-OVL-VNI-DUPLICATE
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :pins: SWARM-OVL-ALLOCATION-FAILURE-UNREPORTED
    :within: SWARM-CEIL-ALLOCATION
    :concerns: SWARM-SCALE-UNALLOCATABLE-CTR-START, SWARM-OVL-UNALLOCATED-LISTED
    :violates: PRIN-WARN-AHEAD, REQ-SWARM-TASK-ERRORS

    *Applies to:* An allocated overlay network

    :api-op:`POST /networks/create` for another overlay network, with a VNI in its
    :drvopt:`vxlanid_list` that :api-op:`GET /networks/{id}` reports in the first network's
    :drvopt:`vxlanid_list`, responds 201 without a warning about the VNI. :api-op:`GET /networks` on
    a manager then lists the new network. The new network is not allocated while the first network
    exists, and the tasks of a service on the new network stay in the ``new`` state that
    :api-op:`GET /tasks` reports. Within the :need:`[[title|lower]] ceiling ([[seconds]] s)
    <SWARM-CEIL-ALLOCATION>` after :api-op:`DELETE /networks/{id}` for the first network responds,
    the new network is allocated.

.. expectation:: docker network create with another overlay's VNI succeeds
    :id: SWARM-OVL-VNI-DUPLICATE-CLI
    :observed_from: api:mgr
    :depends_on: cli
    :platforms: linux
    :cli_for: SWARM-OVL-VNI-DUPLICATE
    :pins: SWARM-OVL-ALLOCATION-FAILURE-UNREPORTED
    :concerns: SWARM-SCALE-UNALLOCATABLE-CTR-START-CLI, SWARM-OVL-UNALLOCATED-LISTED
    :violates: PRIN-WARN-AHEAD, REQ-SWARM-TASK-ERRORS

    *Applies to:* An allocated overlay network

    :cli:`docker network create --driver overlay --opt
    com.docker.network.driver.overlay.vxlanid_list=<VNI>`, with a VNI that :cli:`docker network
    inspect` prints in the first network's :drvopt:`vxlanid_list`, exits zero and prints no warning.
    :cli:`docker network ls` on a manager then lists the new network.

.. expectation:: An overlay network that is not allocated is listed and returned
    :id: SWARM-OVL-UNALLOCATED-LISTED
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-NETWORK-CREATE-ON-MANAGER

    *Applies to:* An overlay network that is not allocated

    :api-op:`GET /networks` lists the network. :api-op:`GET /networks/{id}` returns the network.

.. expectation:: An unallocated overlay does not hide other Swarm-scoped networks
    :id: SWARM-OVL-UNALLOCATED-OTHERS-LISTED
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-NETWORK-UNALLOCATED-ISOLATED

    An overlay network that is not allocated does not prevent :api-op:`GET /networks` from listing
    any other Swarm-scoped network. It also does not prevent :api-op:`GET /networks/{id}` from
    returning any other Swarm-scoped network.

.. expectation:: Network allocation failures are not reported to the caller
    :id: SWARM-OVL-ALLOCATION-FAILURE-UNREPORTED
    :observed_from: api:mgr, log
    :depends_on: none
    :platforms: linux
    :within: SWARM-CEIL-ALLOCATION
    :concerns: SWARM-SCOPED-CREATE-ALLOCATED, SWARM-OVL-UNALLOCATED-LISTED, SWARM-OVL-UNALLOCATED-OTHERS-LISTED, SWARM-OVL-VNI-DUPLICATE, SWARM-SCALE-UNALLOCATABLE-CTR-START
    :violates: REQ-NET-INSPECT-CONFIGURATION, REQ-SWARM-TASK-ERRORS, PRIN-WARN-AHEAD, REQ-SWARM-NETWORK-SUBNET-CONFIG

    :api-op:`POST /networks/create` for an overlay network that cannot be allocated responds 201.
    This occurs when a VNI in the network's :drvopt:`vxlanid_list` is in use, or when its address
    pool is exhausted. It also occurs when its requested subnet overlaps the subnet of another Swarm
    network. It also occurs when it has an IPv6 subnet under the default IPAM driver. Networks are
    allocated asynchronously. When a network cannot be allocated, the leader manager's log gets an
    entry at the error level that contains :log:`Failed allocation for network`, or :log:`failed allocating network` for a network that a manager finds unallocated when it becomes leader, and the network's ID. Once the network that holds what it needs is removed, the network is allocated within the
    :need:`[[title|lower]] ceiling ([[seconds]] s) <SWARM-CEIL-ALLOCATION>` after the request that
    removes it responds. What it needs is the VNI in its :drvopt:`vxlanid_list`, a subnet of its
    exhausted address pool, or the subnet that its requested subnet overlaps. A network with an IPv6
    subnet under the default IPAM driver is never allocated. The response to the create has an empty
    :api-field:`Warning`. :api-op:`POST /services/create` for a service on such a network also
    responds 201, without warnings. While the network is not allocated, :api-op:`GET /tasks` reports
    each of the service's tasks in the ``new`` state, without an error. On a manager,
    :api-op:`GET /networks/{id}` reports the network with an empty :api-field:`Driver` and
    :api-field:`IPAM`, even when the request named them.

.. expectation:: Auto-allocated VNIs come from [4096, 2²⁴)
    :id: SWARM-OVL-VNI-AUTO-RANGE
    :observed_from: underlay
    :depends_on: none
    :platforms: linux
    :within: SWARM-CEIL-ALLOCATION, SWARM-CEIL-REASSIGNMENT
    :rationale: RAT-SWARM-AUTO-VNI-ABOVE-VLAN-IDS

    Auto-allocated VNIs come from [4096, 2²⁴).

.. expectation:: GET /networks/{id} reports the effective vxlanid_list in Options
    :id: SWARM-OVL-VNI-LIST-REPORTED
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-OVERLAY-VNI-REPORTED
    :within: SWARM-CEIL-ALLOCATION

    :api-op:`GET /networks/{id}` reports the effective :drvopt:`vxlanid_list` in
    :api-field:`Options`.

.. expectation:: An overlay network accepts and reports any mtu option value
    :id: SWARM-OVL-MTU-ANY-VALUE
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :pins: SWARM-OVL-MTU-UNCHECKED
    :within: SWARM-CEIL-ALLOCATION

    :api-op:`POST /networks/create` for an overlay network responds 201 whatever value
    :drvopt:`com.docker.network.driver.mtu` has. :api-op:`GET /networks/{id}` on a manager then
    reports that value in :api-field:`Options`.

.. expectation:: docker network create accepts and inspect prints any overlay mtu value
    :id: SWARM-OVL-MTU-ANY-VALUE-CLI
    :observed_from: api:mgr
    :depends_on: cli
    :platforms: linux
    :cli_for: SWARM-OVL-MTU-ANY-VALUE
    :pins: SWARM-OVL-MTU-UNCHECKED
    :within: SWARM-CEIL-ALLOCATION

    :cli:`docker network create --driver overlay` exits zero whatever value :cli-flag:`--opt` gives
    :drvopt:`com.docker.network.driver.mtu`.
    :cli:`docker network inspect` on a manager then prints that value in the network's options.

.. expectation:: mtu on overlay networks is not checked at create
    :id: SWARM-OVL-MTU-UNCHECKED
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :concerns: SWARM-OVL-MTU-ANY-VALUE, SWARM-OVL-MTU-ANY-VALUE-CLI, SWARM-OVLOPT-MTU
    :violates: PRIN-FAIL-FAST, PRIN-ERRORS-ACTIONABLE

    :api-op:`POST /networks/create` for an overlay network with any value of
    :drvopt:`com.docker.network.driver.mtu` responds 201. If the value is not a non-negative
    integer, a Linux node rejects all tasks on the network that are assigned to that node. Their
    errors contain :api-error:`failed to parse` or :api-error:`invalid MTU value`. The
    :api-error:`failed to parse` error gives the value and does not name the option. If the kernel
    cannot create the network's interfaces with the value, a Linux node also rejects every such
    task.

.. expectation:: An overlay network with AuxiliaryAddresses is created
    :id: SWARM-OVL-AUXILIARY-ADDRESSES-CREATE
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-NET-API-BACKWARD-COMPATIBLE

    :api-op:`POST /networks/create` for an overlay network with :api-field:`AuxiliaryAddresses` in
    :api-field:`IPAM.Config` responds 201.

.. expectation:: A task or VIP gets an overlay's AuxiliaryAddresses when no other is free
    :id: SWARM-OVL-AUXILIARY-ADDRESSES
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :pins: SWARM-OVL-AUXILIARY-ADDRESSES-IGNORED
    :within: SWARM-CEIL-ALLOCATION
    :concerns: SWARM-OVL-AUXILIARY-ADDRESSES-CREATE
    :violates: REQ-NET-INSPECT-CONFIGURATION

    *Applies to:* An overlay network created with :api-field:`AuxiliaryAddresses` in
    :api-field:`IPAM.Config`

    :api-op:`GET /networks/{id}` on a manager does not report these addresses. When every other
    address of the subnet is in use, a task or VIP that is then allocated on the network gets one of
    these addresses.

.. expectation:: AuxiliaryAddresses of an overlay are accepted and not reserved
    :id: SWARM-OVL-AUXILIARY-ADDRESSES-IGNORED
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :within: SWARM-CEIL-ALLOCATION
    :concerns: SWARM-OVL-AUXILIARY-ADDRESSES, SWARM-OVL-AUXILIARY-ADDRESSES-CREATE
    :violates: PRIN-FAILS-CLOSED

    :api-op:`POST /networks/create` accepts :api-field:`AuxiliaryAddresses` in the
    :api-field:`IPAM.Config` of an overlay network, but they are not kept. These addresses are not
    reserved: when every other address of the subnet is in use, a task or VIP that is then allocated
    on the network gets one of them.

.. expectation:: A container does not start with a static address another one uses
    :id: SWARM-OVL-STATIC-ADDRESS-IN-USE
    :observed_from: api:active
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-SWARM-NETWORK-STATIC-ADDRESS, REQ-SWARM-ATTACH-FAILURE-START

    *Applies to:* A created or stopped container with a static IPv4 address on an attachable
    overlay, and another container on the overlay that uses that address

    :api-op:`POST /containers/{id}/start` responds with a 4xx or 5xx status.

.. expectation:: A static address in use responds 500 after 20 seconds
    :id: SWARM-OVL-STATIC-ADDRESS-IN-USE-HTTP500
    :observed_from: api:active
    :depends_on: none
    :platforms: linux, windows
    :exact_form_of: SWARM-OVL-STATIC-ADDRESS-IN-USE
    :pins: SWARM-SCOPED-ATTACH-FAILURE-TIMEOUT
    :concerns: SWARM-OVL-STATIC-ADDRESS-IN-USE
    :violates: PRIN-ERRORS-ACTIONABLE, REQ-NET-API-ERROR-STATUS

    *Applies to:* A created or stopped container with a static IPv4 address on an attachable
    overlay, and another container on the overlay that uses that address

    :api-op:`POST /containers/{id}/start` responds 500 after 20 seconds. The response's message
    contains :api-error:`attaching to network failed, make sure your network options are correct and
    check manager logs: context deadline exceeded`.

.. expectation:: docker run --ip with an address in use on the overlay fails
    :id: SWARM-OVL-STATIC-ADDRESS-IN-USE-CLI
    :observed_from: api:active
    :depends_on: cli
    :platforms: linux, windows
    :refines: REQ-SWARM-NETWORK-STATIC-ADDRESS, REQ-SWARM-ATTACH-FAILURE-START
    :cli_for: SWARM-OVL-STATIC-ADDRESS-IN-USE

    *Applies to:* A running container on an attachable overlay

    :cli:`docker run -d --network --ip` with that overlay and the container's IPv4 address on it
    exits with status 125 and prints the daemon's error.

.. expectation:: An attached node refuses an off-subnet static address at create
    :id: SWARM-OVL-STATIC-ADDRESS-OUTSIDE-ATTACHED
    :observed_from: api:active
    :depends_on: none
    :platforms: linux, windows
    :refines: PRIN-FAIL-FAST, PRIN-FAILS-CLOSED, REQ-NET-API-ERROR-STATUS

    *Applies to:* An attachable overlay, and a node with a local attachment to it

    On that node, :api-op:`POST /containers/create` for a container on the overlay, with a static
    IPv4 address outside the overlay's subnets, responds with a 4xx status.

.. expectation:: An attached node refuses an off-subnet static address at create with 400
    :id: SWARM-OVL-STATIC-ADDRESS-OUTSIDE-ATTACHED-HTTP400
    :observed_from: api:active
    :depends_on: none
    :platforms: linux, windows
    :exact_form_of: SWARM-OVL-STATIC-ADDRESS-OUTSIDE-ATTACHED

    *Applies to:* An attachable overlay, and a node with a local attachment to it

    On that node, :api-op:`POST /containers/create` for a container on the overlay, with a static
    IPv4 address outside the overlay's subnets, responds 400.

.. expectation:: Off-subnet refusal at create says no configured subnet contains it
    :id: SWARM-OVL-STATIC-ADDRESS-OUTSIDE-ATTACHED-MSG
    :observed_from: api:active
    :depends_on: none
    :platforms: linux, windows
    :exact_form_of: SWARM-OVL-STATIC-ADDRESS-OUTSIDE-ATTACHED

    *Applies to:* An attachable overlay, and a node with a local attachment to it

    On that node, :api-op:`POST /containers/create` for a container on the overlay, with a static
    IPv4 address outside the overlay's subnets, responds 400. The response's message contains
    :api-error:`no configured subnet contains IP address`.

.. expectation:: An unattached node creates with an off-subnet address and start fails
    :id: SWARM-OVL-STATIC-ADDRESS-OUTSIDE-UNATTACHED
    :observed_from: api:active
    :depends_on: none
    :platforms: linux, windows
    :exact_form_of: SWARM-OVL-STATIC-ADDRESS-OUTSIDE
    :pins: SWARM-SCOPED-ATTACH-FAILURE-TIMEOUT
    :concerns: SWARM-OVL-STATIC-ADDRESS-OUTSIDE
    :violates: PRIN-ERRORS-ACTIONABLE, PRIN-FAIL-FAST, REQ-NET-API-ERROR-STATUS

    *Applies to:* An attachable overlay, and a node without a local attachment to it

    On that node, :api-op:`POST /containers/create` for a container on the overlay, with a static
    IPv4 address outside the overlay's subnets, responds 201. :api-op:`POST /containers/{id}/start`
    for that container then responds 500 after 20 seconds, with a message that contains
    :api-error:`attaching to network failed, make sure your network options are correct and check
    manager logs: context deadline exceeded`.

.. expectation:: A static address outside the overlay's subnets fails at create or start
    :id: SWARM-OVL-STATIC-ADDRESS-OUTSIDE
    :observed_from: api:active
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-SWARM-NETWORK-STATIC-ADDRESS, REQ-SWARM-ATTACH-FAILURE-START, PRIN-FAILS-CLOSED

    *Applies to:* An attachable overlay

    On any node, :api-op:`POST /containers/create` for a container on the overlay, with a static
    IPv4 address outside the overlay's subnets, responds with a 4xx or 5xx status. Or it responds
    201, and :api-op:`POST /containers/{id}/start` for that container then responds with a 4xx or
    5xx status.

.. expectation:: docker run --ip with an address outside the overlay's subnets fails
    :id: SWARM-OVL-STATIC-ADDRESS-OUTSIDE-CLI
    :observed_from: api:active
    :depends_on: cli
    :platforms: linux, windows
    :refines: REQ-SWARM-NETWORK-STATIC-ADDRESS, REQ-SWARM-ATTACH-FAILURE-START, PRIN-FAILS-CLOSED
    :cli_for: SWARM-OVL-STATIC-ADDRESS-OUTSIDE

    *Applies to:* An attachable overlay

    :cli:`docker run -d --network --ip` with that overlay and an IPv4 address outside its subnets
    exits with status 125 and prints the daemon's error.

.. expectation:: A container's address is free for a static address once it leaves
    :id: SWARM-OVL-STATIC-ADDRESS-RELEASED
    :observed_from: api:mgr, api:active
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-SWARM-NETWORK-STATIC-ADDRESS, REQ-SWARM-NETWORK-ATTACHMENT-RELEASED, PRIN-NO-LEAKS
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A standalone container on an attachable overlay

    After the container disconnects from the overlay or stops, its attachment task disappears from
    :api-op:`GET /tasks` on a manager. On any node whose availability is ``active``, :api-op:`POST
    /containers/{id}/start` for another container on the overlay, with the first container's address
    as its static IPv4 address, then responds 204.

.. expectation:: A vip service has one VIP on each attached overlay network
    :id: SWARM-OVL-VIP-PER-NETWORK
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-NETWORK-SERVICE-VIP
    :within: SWARM-CEIL-ALLOCATION

    *Applies to:* A service in vip mode

    :api-op:`GET /services/{id}` reports one VIP in :api-field:`Endpoint.VirtualIPs` for each
    overlay network that the service is attached to. Each of these VIPs is in one of that network's
    subnets.

.. expectation:: A vip service with an ingress-mode port has one VIP on ingress
    :id: SWARM-OVL-VIP-INGRESS
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-NETWORK-SERVICE-VIP
    :within: SWARM-CEIL-ALLOCATION

    *Applies to:* A service in vip mode that publishes an ingress-mode port

    :api-op:`GET /services/{id}` for the service includes one VIP on the ingress network.

.. expectation:: A service keeps its VIP on a network through an update
    :id: SWARM-OVL-VIP-UPDATE
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-NETWORK-SERVICE-VIP, REQ-SWARM-NETWORK-SERVICE-VIP-KEPT
    :within: SWARM-CEIL-ALLOCATION

    *Applies to:* A service in vip mode that is attached to a network

    After :api-op:`POST /services/{id}/update` that leaves the service in vip mode and attached to
    that network responds 200, :api-op:`GET /services/{id}` reports the same VIP on that network as
    before.

.. expectation:: A service keeps its VIPs after the leader changes
    :id: SWARM-OVL-VIP-LEADER-CHANGE
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-NETWORK-SERVICE-VIP, REQ-SWARM-NETWORK-SERVICE-VIP-KEPT
    :within: SWARM-CEIL-ALLOCATION

    After the leader changes, :api-op:`GET /services/{id}` reports the same VIPs as before.

.. expectation:: docker service inspect prints one VIP for each attached overlay
    :id: SWARM-OVL-VIP-PER-NETWORK-CLI
    :observed_from: api:mgr
    :depends_on: cli
    :platforms: linux
    :refines: REQ-SWARM-NETWORK-SERVICE-VIP
    :cli_for: SWARM-OVL-VIP-PER-NETWORK
    :within: SWARM-CEIL-ALLOCATION

    *Applies to:* A service in vip mode

    :cli:`docker service inspect` prints one VIP in :api-field:`Endpoint.VirtualIPs` for each
    overlay network that the service is attached to.

.. expectation:: On a node-local Swarm network, a vip service's VirtualIPs entry has no address
    :id: SWARM-OVL-VIP-NODE-LOCAL
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :within: SWARM-CEIL-ALLOCATION
    :rationale: RAT-SWARM-NODE-LOCAL-NO-CLUSTER-RECORDS

    *Applies to:* A service in vip mode attached to a Swarm-scoped network of a node-local driver

    :api-op:`GET /services/{id}` reports an entry in :api-field:`Endpoint.VirtualIPs` with that
    network's ID and without :api-field:`Addr`.

.. expectation:: An overlay with an IPv6 subnet is accepted and never allocated
    :id: SWARM-OVL-V6-ADDRESSING
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :within: SWARM-CEIL-ALLOCATION
    :concerns: SWARM-CTR-ADDRESS, SWARM-OVL-VIP-PER-NETWORK
    :violates: PRIN-FAILS-CLOSED, REQ-SWARM-NETWORK-SUBNET-CONFIG

    With the default IPAM driver, :api-op:`POST /networks/create` accepts an overlay network with an
    IPv6 subnet, but the network is never allocated. An overlay network with :api-field:`EnableIPv6`
    set and without an IPv6 subnet is accepted, and the network's attachments get IPv4 addresses
    only. Each attachment gets exactly one address.

.. expectation:: The VIP that verbose network inspect reports load-balances to the tasks
    :id: SWARM-OVL-VERBOSE-VIP
    :observed_from: api:mgr, api:wkr (on a node with a local attachment to the network), task
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-NETWORK-VERBOSE-INSPECT
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-NFTABLES-NAT, SWARM-HOST-NFT-COMMAND, SWARM-HOST-IPTABLES-LB-EXTENSIONS, SWARM-HOST-IPTABLES-LEGACY-MANGLE-KERNEL
    :within: SWARM-CEIL-REASSIGNMENT

    The VIP that :api-op:`GET /networks/{id}?verbose=true` reports for a service is the address that
    load-balances to its tasks.

.. expectation:: Verbose network inspect lists running tasks and no stopped task
    :id: SWARM-OVL-VERBOSE-TASKS
    :observed_from: api:mgr, api:wkr
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-SWARM-NETWORK-VERBOSE-INSPECT
    :within: SWARM-CEIL-ALLOCATION, SWARM-CEIL-REASSIGNMENT

    *Applies to:* A node with a local attachment to an overlay network, and a service on that
    network

    Within the :need:`[[title|lower]] ceiling ([[seconds]] s) <SWARM-CEIL-REASSIGNMENT>` after a
    task of the service is added, and within the :need:`[[title|lower]] ceiling ([[seconds]] s)
    <SWARM-CEIL-ALLOCATION>` after a task's container exits or its desired state changes from
    ``running``, the tasks that :api-op:`GET /networks/{id}?verbose=true` reports for the service
    include each of its running tasks, as :ref:`swarm-traffic-service-discovery` in
    :doc:`swarm-traffic.md <swarm-traffic>` defines them. They do not include a task whose container
    has stopped.

.. expectation:: Verbose network inspect on an attached node reports Services
    :id: SWARM-OVL-VERBOSE-SERVICES
    :observed_from: api:mgr, api:wkr
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-SWARM-NETWORK-VERBOSE-INSPECT
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A node with a local attachment to an overlay network

    On that node, :api-op:`GET /networks/{id}?verbose=true` for that network reports
    :api-field:`Services`. :api-field:`Services` gives the VIP of each service and the endpoint IP
    of each task. On the ingress network, it also gives each service's published ports.

.. expectation:: docker network inspect --verbose prints Services on an attached node
    :id: SWARM-OVL-VERBOSE-SERVICES-CLI
    :observed_from: api:mgr, api:wkr
    :depends_on: cli
    :platforms: linux, windows
    :refines: REQ-SWARM-NETWORK-VERBOSE-INSPECT
    :cli_for: SWARM-OVL-VERBOSE-SERVICES
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A node with a local attachment to an overlay network

    On that node, :cli:`docker network inspect --verbose` for that network prints
    :api-field:`Services`.

.. expectation:: Verbose network inspect on an unattached manager reports no services
    :id: SWARM-OVL-VERBOSE-UNATTACHED
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :rationale: RAT-SWARM-NODE-REPORTS-OWN-STATE

    *Applies to:* A manager that does not have a local attachment to an overlay network, and a
    service on that network whose tasks run on other nodes

    On that manager, :api-op:`GET /networks/{id}?verbose=true` for that network responds 200 and
    does not report any services.

.. expectation:: Peers lists each attached node at its advertise address
    :id: SWARM-OVL-PEERS
    :observed_from: api:mgr, api:wkr
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-SWARM-OVERLAY-PEERS
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A node with a local attachment to an overlay network

    On that node, :api-op:`GET /networks/{id}` for the network lists in :api-field:`Peers` each node
    that has a local attachment to the network. This includes the node itself. The :api-field:`IP`
    of each entry is the advertise address of that node, whatever its data-path address is.

.. expectation:: IPsInUse rises when a task attaches and falls when it leaves
    :id: SWARM-OVL-STATUS-IN-USE
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-NETWORK-IPAM-STATUS
    :within: SWARM-CEIL-ALLOCATION

    On a manager, at API version 1.52 or later, :api-field:`IPsInUse` in the
    :api-field:`Status.IPAM` of :api-op:`GET /networks/{id}`, with the network's name or a partial
    ID, increases when a task attaches. It decreases when a task leaves.

.. expectation:: A worker's Status.IPAM counts only the worker's own endpoints
    :id: SWARM-OVL-STATUS-WORKER
    :observed_from: api:wkr
    :depends_on: none
    :platforms: linux, windows
    :within: SWARM-CEIL-REASSIGNMENT
    :concerns: SWARM-OVL-STATUS-IN-USE
    :violates: REQ-SWARM-NETWORK-IPAM-STATUS

    *Applies to:* A worker with a local attachment to an overlay network

    At API version 1.52 or later, the :api-field:`Status.IPAM` of :api-op:`GET /networks/{id}` on
    that worker reflects only the worker's own endpoints. It does not change when a task attaches on
    another node.

.. expectation:: IPsInUse plus DynamicIPsAvailable is the subnet's size during churn
    :id: SWARM-OVL-STATUS-TOTAL
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-NETWORK-IPAM-STATUS

    *Applies to:* An overlay network with an IPv4 subnet that has no :api-field:`IPRange`

    On a manager, at API version 1.52 or later, :api-op:`GET /networks/{id}`, with the network's
    name or a partial ID, reports :api-field:`IPsInUse` and :api-field:`DynamicIPsAvailable` for the
    subnet. Their sum equals the number of addresses in the subnet at every point during churn. That
    number is 2^(32 − prefix length).

.. expectation:: By full ID, a manager reports only its own endpoints in Status.IPAM
    :id: SWARM-OVL-STATUS-FULL-ID
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :within: SWARM-CEIL-ALLOCATION, SWARM-CEIL-REASSIGNMENT
    :concerns: SWARM-OVL-STATUS-IN-USE, SWARM-OVL-STATUS-TOTAL
    :violates: REQ-SWARM-NETWORK-IPAM-STATUS

    *Applies to:* A manager with a local attachment to an overlay network

    On that manager, at API version 1.52 or later, :api-op:`GET /networks/{id}` with the network's
    full ID reports only that node's own endpoints in :api-field:`Status.IPAM`. With the network's
    name or a partial ID, it reports the manager's allocation for the whole network.

.. expectation:: Addresses that are not static stay inside the subnet's IPRange
    :id: SWARM-OVL-IPRANGE
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-NETWORK-SUBNET-CONFIG, REQ-SWARM-NETWORK-STATIC-ADDRESS
    :within: SWARM-CEIL-ALLOCATION

    *Applies to:* An overlay network with a subnet that has an :api-field:`IPRange`

    Every task address, VIP and node load-balancer address in that subnet is inside that
    :api-field:`IPRange`. So is the address in that subnet of each standalone container on the
    network that does not request a static address. :api-op:`GET /tasks`,
    :api-op:`GET /services/{id}` and :api-op:`GET /networks/{id}` report these addresses. When the
    request that created the network gave no :api-field:`Gateway` for the subnet, the
    :api-field:`Gateway` that :api-op:`GET /networks/{id}` on a manager reports for the subnet is
    inside that :api-field:`IPRange` too. The :api-field:`Gateway` is none of the addresses above.

.. expectation:: Without a Gateway, the gateway is the lowest IPRange non-network address
    :id: SWARM-OVL-IPRANGE-GATEWAY
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :exact_form_of: SWARM-OVL-IPRANGE
    :within: SWARM-CEIL-ALLOCATION

    *Applies to:* An overlay network created with a subnet that has an :api-field:`IPRange` and no
    :api-field:`Gateway`

    The :api-field:`Gateway` that :api-op:`GET /networks/{id}` on a manager reports for the subnet
    is the lowest address in :api-field:`IPRange` other than the subnet's network address.

.. expectation:: From API 1.52, Status.IPAM has per-subnet IPsInUse, DynamicIPsAvailable
    :id: SWARM-OVL-STATUS-PER-SUBNET
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-NETWORK-IPAM-STATUS
    :within: SWARM-CEIL-ALLOCATION

    On a manager, at API version 1.52 or later, :api-op:`GET /networks/{id}` reports
    :api-field:`Status.IPAM` with per-subnet :api-field:`IPsInUse` and
    :api-field:`DynamicIPsAvailable`.

.. expectation:: Before API 1.52, GET /networks/{id} does not include Status
    :id: SWARM-OVL-STATUS-API-VERSION
    :observed_from: api:mgr, api:wkr
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-SWARM-NETWORK-IPAM-STATUS, REQ-NET-API-BACKWARD-COMPATIBLE

    At API versions before 1.52, :api-op:`GET /networks/{id}` does not include :api-field:`Status`.

.. expectation:: On a host without veth support, a task on an overlay network does not run
    :id: SWARM-OVL-NO-VETH
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-TASK-ERRORS
    :pins: SWARM-HOST-NETNS-VETH-BRIDGE

    *Applies to:* A Linux node whose kernel does not offer veth pairs

    On that node, a task attached to an overlay network does not reach the ``running`` state, and
    :api-op:`GET /tasks` reports an error for the task.

.. expectation:: A task that cannot get a veth pair says creating the veth pair failed
    :id: SWARM-OVL-NO-VETH-MSG
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :exact_form_of: SWARM-OVL-NO-VETH
    :pins: SWARM-HOST-NETNS-VETH-BRIDGE

    *Applies to:* A Linux node whose kernel does not offer veth pairs

    On that node, the error that :api-op:`GET /tasks` reports for a task on an overlay network
    contains :api-error:`error creating veth pair`.

.. _swarm-networking-the-ingress-network:

The ingress network
===================

A Swarm-scoped network created with :api-field:`Ingress` set is an ingress network. A Swarm-scoped
network named :network:`ingress` and created with the label ``com.docker.swarm.internal`` in
:api-field:`Labels` is also an ingress network, whatever its driver and the label's value. Older
daemons created the ingress network in this second form, without :api-field:`Ingress`. :api-op:`GET
/networks` reports :api-field:`Ingress` as true for both forms.

An ingress network exists while :api-op:`GET /networks` on a manager lists a network whose
:api-field:`Ingress` is true. That network is the ingress network. :cli:`docker swarm init` creates
it with :api-field:`Ingress` set and names it :network:`ingress`. It can be removed, and an ingress
network can then be created with another name. A copy that a node keeps after the network is deleted
(:need:`SWARM-INGRESS-LABEL-DELETE-STALE`) does not count. Nor does a network created in the second
form while another ingress network exists, which is never allocated
(:need:`SWARM-INGRESS-LABEL-SECOND`).

The managers give every node of the Swarm an address on the ingress network for the node's
load-balancer endpoint. A node that has such an address on an ingress network created with
:api-field:`Ingress` set creates the network and its load-balancer endpoint on it, whether or not a
task runs on the node, and so has a local attachment to the network. When the network is deleted,
the node removes it and has no local attachment to it. A node does not create an ingress network of
the second form for its address on it. It has a local attachment to such a network only once a task
on the node of a service that publishes a port in ingress mode, or a standalone container on the
node, attaches to the network.

.. expectation:: POST /swarm/init creates an ingress network named ingress
    :id: SWARM-INGRESS-DEFAULT
    :observed_from: api:solo, api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-INGRESS-CREATED-AT-INIT
    :within: SWARM-CEIL-REJOIN

    *Applies to:* A node that is not in a Swarm, on which :api-op:`GET /networks` lists no network
    whose :api-field:`Ingress` is true

    Within the :need:`[[title|lower]] ceiling ([[seconds]] s) <SWARM-CEIL-REJOIN>` after
    :api-op:`POST /swarm/init` responds 200 on the node, an ingress network exists. Its name is
    :network:`ingress`.

.. expectation:: A second ingress network is refused
    :id: SWARM-INGRESS-DUPLICATE
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-INGRESS-SINGLE, REQ-NET-API-ERROR-STATUS

    *Applies to:* A Swarm with an ingress network

    :api-op:`POST /networks/create` for an overlay network with :api-field:`Ingress` set and
    :api-field:`Attachable` not set responds with a 4xx status.

.. expectation:: A second ingress network responds 409
    :id: SWARM-INGRESS-DUPLICATE-HTTP409
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :exact_form_of: SWARM-INGRESS-DUPLICATE

    *Applies to:* A Swarm with an ingress network

    :api-op:`POST /networks/create` for an overlay network with :api-field:`Ingress` set and
    :api-field:`Attachable` not set responds 409.

.. expectation:: docker network create --ingress fails when an ingress network exists
    :id: SWARM-INGRESS-DUPLICATE-CLI
    :observed_from: api:mgr
    :depends_on: cli
    :platforms: linux
    :refines: REQ-SWARM-INGRESS-SINGLE
    :cli_for: SWARM-INGRESS-DUPLICATE

    *Applies to:* A Swarm with an ingress network

    :cli:`docker network create --driver overlay --ingress` exits non-zero and prints the daemon's
    error.

.. expectation:: An ingress network with a node-local driver or no Driver is refused
    :id: SWARM-INGRESS-NODE-LOCAL-DRIVER
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-INGRESS-OVERLAY-ONLY, REQ-NET-API-ERROR-STATUS

    :api-op:`POST /networks/create` with :api-field:`Ingress` set and a node-local driver, or no
    :api-field:`Driver`, responds with a 4xx status.

.. expectation:: An ingress network with a node-local driver or no Driver responds 403
    :id: SWARM-INGRESS-NODE-LOCAL-DRIVER-HTTP403
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :exact_form_of: SWARM-INGRESS-NODE-LOCAL-DRIVER

    :api-op:`POST /networks/create` with :api-field:`Ingress` set and a node-local driver, or no
    :api-field:`Driver`, responds 403.

.. expectation:: docker network create --ingress fails with a node-local driver
    :id: SWARM-INGRESS-NODE-LOCAL-DRIVER-CLI
    :observed_from: api:mgr
    :depends_on: cli
    :platforms: linux
    :refines: REQ-SWARM-INGRESS-OVERLAY-ONLY
    :cli_for: SWARM-INGRESS-NODE-LOCAL-DRIVER

    :cli:`docker network create --ingress` with a node-local driver, or without
    :cli-flag:`--driver`, exits non-zero and prints the daemon's error.

.. expectation:: A node-local labeled ingress is created and its tasks are rejected
    :id: SWARM-INGRESS-LABEL-NODE-LOCAL-DRIVER
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :concerns: SWARM-INGRESS-NODE-LOCAL-DRIVER
    :violates: REQ-SWARM-INGRESS-OVERLAY-ONLY

    *Applies to:* A Swarm without an ingress network

    On a manager, :api-op:`POST /networks/create` for a Swarm-scoped network of a node-local
    driver, named :network:`ingress`, with the label ``com.docker.swarm.internal`` and without
    :api-field:`Ingress`, responds 201. :api-op:`GET /networks` on a manager then lists it with
    :api-field:`Ingress` true. The tasks of a service that publishes a port in ingress mode are
    then rejected, and their error in :api-op:`GET /tasks` contains :api-error:`Ingress network can
    only be global scope network`.

.. expectation:: An ingress network with a plugin's multi-host driver responds 501
    :id: SWARM-INGRESS-PLUGIN-DRIVER
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-INGRESS-OVERLAY-ONLY, REQ-SWARM-INGRESS-DRIVER-NOT-IMPLEMENTED, REQ-NET-API-ERROR-STATUS

    *Applies to:* A manager on which a managed plugin provides a multi-host driver other than
    :driver:`overlay`

    On that manager, :api-op:`POST /networks/create` with :api-field:`Ingress` set,
    :api-field:`Attachable` not set, and that driver responds 501.

.. expectation:: docker network create --ingress fails with a plugin's multi-host driver
    :id: SWARM-INGRESS-PLUGIN-DRIVER-CLI
    :observed_from: api:mgr
    :depends_on: cli
    :platforms: linux
    :refines: REQ-SWARM-INGRESS-OVERLAY-ONLY
    :cli_for: SWARM-INGRESS-PLUGIN-DRIVER

    *Applies to:* A manager on which a managed plugin provides a multi-host driver other than
    :driver:`overlay`

    On that manager, :cli:`docker network create --ingress` with that driver exits non-zero, and
    prints the daemon's error.

.. expectation:: An ingress network with Attachable set is refused
    :id: SWARM-INGRESS-ATTACHABLE
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-INGRESS-OVERLAY-ONLY, REQ-NET-API-ERROR-STATUS

    :api-op:`POST /networks/create` with both :api-field:`Ingress` and :api-field:`Attachable` set
    and a multi-host driver responds with a 4xx status.

.. expectation:: An ingress network with Attachable set responds 400
    :id: SWARM-INGRESS-ATTACHABLE-HTTP400
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :exact_form_of: SWARM-INGRESS-ATTACHABLE

    :api-op:`POST /networks/create` with both :api-field:`Ingress` and :api-field:`Attachable` set
    and a multi-host driver responds 400.

.. expectation:: docker network create --ingress --attachable fails
    :id: SWARM-INGRESS-ATTACHABLE-CLI
    :observed_from: api:mgr
    :depends_on: cli
    :platforms: linux
    :refines: REQ-SWARM-INGRESS-OVERLAY-ONLY
    :cli_for: SWARM-INGRESS-ATTACHABLE

    :cli:`docker network create --ingress --attachable` with a multi-host driver exits non-zero and
    prints the daemon's error.

.. expectation:: A service that names the ingress network in its networks is refused
    :id: SWARM-INGRESS-SERVICE-NETWORK
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-INGRESS-NOT-FOR-SERVICES, REQ-NET-API-ERROR-STATUS, REQ-SWARM-INGRESS-MESH-ONLY

    :api-op:`POST /services/create` or :api-op:`POST /services/{id}/update` that names the ingress
    network in :api-field:`TaskTemplate.Networks` responds with a 4xx status. The request does not
    create or change the service.

.. expectation:: A service that names the ingress network in its networks responds 400
    :id: SWARM-INGRESS-SERVICE-NETWORK-HTTP400
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :exact_form_of: SWARM-INGRESS-SERVICE-NETWORK

    :api-op:`POST /services/create` or :api-op:`POST /services/{id}/update` that names the ingress
    network in :api-field:`TaskTemplate.Networks` responds 400.

.. expectation:: docker service create or update naming the ingress network fails
    :id: SWARM-INGRESS-SERVICE-NETWORK-CLI
    :observed_from: api:mgr
    :depends_on: cli
    :platforms: linux
    :refines: REQ-SWARM-INGRESS-NOT-FOR-SERVICES, REQ-SWARM-INGRESS-MESH-ONLY
    :cli_for: SWARM-INGRESS-SERVICE-NETWORK

    :cli:`docker service create --network` and :cli:`docker service update --network-add` that name
    the ingress network exit non-zero and print the daemon's error. The commands do not create or
    change the service.

.. expectation:: The ingress network is not deleted while a service publishes through it
    :id: SWARM-INGRESS-DELETE-IN-USE
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-INGRESS-REPLACEABLE, REQ-NET-API-ERROR-STATUS, REQ-SWARM-NETWORK-DELETE-IN-USE

    *Applies to:* A Swarm with a service that publishes a port in ingress mode

    :api-op:`DELETE /networks/{id}` for the ingress network responds 400. The network stays listed.

.. expectation:: The refusal to delete the ingress network says a service depends on it
    :id: SWARM-INGRESS-DELETE-IN-USE-MSG
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :exact_form_of: SWARM-INGRESS-DELETE-IN-USE

    *Applies to:* A Swarm with a service that publishes a port in ingress mode

    :api-op:`DELETE /networks/{id}` for the ingress network responds 400, with a message that
    contains :api-error:`ingress network cannot be removed because service`.

.. expectation:: docker network rm fails for the ingress network in use
    :id: SWARM-INGRESS-DELETE-IN-USE-CLI
    :observed_from: api:mgr
    :depends_on: cli
    :platforms: linux
    :refines: REQ-SWARM-INGRESS-REPLACEABLE, REQ-SWARM-NETWORK-DELETE-IN-USE
    :cli_for: SWARM-INGRESS-DELETE-IN-USE

    *Applies to:* A Swarm with a service that publishes a port in ingress mode

    :cli:`docker network rm` for the ingress network, given ``y`` at its confirmation prompt, exits
    non-zero and prints the daemon's error. :cli:`docker network ls` then still lists the network.

.. expectation:: docker network rm for ingress warns, asks [y/N], and keeps the network when declined
    :id: SWARM-INGRESS-DELETE-DECLINED
    :observed_from: api:mgr
    :depends_on: cli
    :platforms: linux
    :refines: REQ-SWARM-INGRESS-REMOVE-CONFIRM

    *Applies to:* A Swarm with an ingress network

    :cli:`docker network rm` for the ingress network prints a warning that contains
    ``Before removing the routing-mesh network``, followed by a prompt that ends with ``[y/N]``,
    before it removes the network. Given an answer other than ``y`` or ``Y`` at that prompt, it does
    not remove the network: :cli:`docker network ls` then still lists it.

.. expectation:: docker network rm for ingress exits zero when declined at the prompt
    :id: SWARM-INGRESS-DELETE-DECLINED-EXIT-ZERO
    :observed_from: api:mgr
    :depends_on: cli
    :platforms: linux
    :exact_form_of: SWARM-INGRESS-DELETE-DECLINED

    *Applies to:* A Swarm with an ingress network

    :cli:`docker network rm` for the ingress network, given an answer other than ``y`` or ``Y`` at
    its confirmation prompt, exits zero. :cli:`docker network ls` then still lists the network.

.. expectation:: An ingress network that no service publishes through is deleted
    :id: SWARM-INGRESS-DELETE-UNUSED
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-INGRESS-REPLACEABLE
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A Swarm with an ingress network created with :api-field:`Ingress` set, where no
    service publishes a port in ingress mode

    :api-op:`DELETE /networks/{id}` for the ingress network responds 204. :api-op:`GET /networks` on
    a manager then does not list a network whose :api-field:`Ingress` is true.

.. expectation:: docker network rm deletes an ingress network that no service uses
    :id: SWARM-INGRESS-DELETE-UNUSED-CLI
    :observed_from: api:mgr
    :depends_on: cli
    :platforms: linux
    :refines: REQ-SWARM-INGRESS-REPLACEABLE
    :cli_for: SWARM-INGRESS-DELETE-UNUSED
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A Swarm with an ingress network created with :api-field:`Ingress` set, where no
    service publishes a port in ingress mode

    :cli:`docker network rm` for the ingress network, given ``y`` at its confirmation prompt, exits
    zero. :cli:`docker network ls` then does not list the network.

.. expectation:: An ingress network with a name other than ingress is created
    :id: SWARM-INGRESS-REPLACEMENT
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-INGRESS-REPLACEABLE

    *Applies to:* A Swarm without an ingress network

    :api-op:`POST /networks/create` for an overlay network with :api-field:`Ingress` set,
    :api-field:`Attachable` not set and a name other than :network:`ingress` responds 201.
    :api-op:`GET /networks` on a manager then lists that network with :api-field:`Ingress` true.

.. expectation:: docker network create --ingress with another name works if none exists
    :id: SWARM-INGRESS-REPLACEMENT-CLI
    :observed_from: api:mgr
    :depends_on: cli
    :platforms: linux
    :refines: REQ-SWARM-INGRESS-REPLACEABLE
    :cli_for: SWARM-INGRESS-REPLACEMENT

    *Applies to:* A Swarm without an ingress network

    :cli:`docker network create --driver overlay --ingress` with a name other than
    :network:`ingress` exits zero. :cli:`docker network ls` then lists the network.

.. expectation:: Without an ingress network, ingress-mode ports are refused
    :id: SWARM-INGRESS-ABSENT-PUBLISH
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-INGRESS-REPLACEABLE, REQ-NET-API-ERROR-STATUS

    *Applies to:* A Swarm without an ingress network

    :api-op:`POST /services/create` and :api-op:`POST /services/{id}/update` with a port in ingress
    mode respond with a 4xx status. The requests do not create or change the service.

.. expectation:: Without an ingress network, an ingress-mode port responds 400
    :id: SWARM-INGRESS-ABSENT-PUBLISH-HTTP400
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :exact_form_of: SWARM-INGRESS-ABSENT-PUBLISH

    *Applies to:* A Swarm without an ingress network

    :api-op:`POST /services/create` and :api-op:`POST /services/{id}/update` with a port in ingress
    mode respond 400, with a message that contains :api-error:`service needs ingress network, but no
    ingress network is present`.

.. expectation:: A custom ingress network reports its subnet and gateway
    :id: SWARM-INGRESS-CUSTOM-SUBNET
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-INGRESS-REPLACEABLE, REQ-SWARM-NETWORK-SUBNET-CONFIG
    :within: SWARM-CEIL-ALLOCATION

    *Applies to:* A Swarm with an ingress network created with :api-field:`Ingress` set and with a
    custom subnet and gateway

    :api-op:`GET /networks/{id}` on a manager reports that subnet and gateway for the ingress
    network.

.. expectation:: A custom-subnet ingress network serves published ports on every node
    :id: SWARM-INGRESS-CUSTOM-SUBNET-REACHABLE
    :observed_from: client
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-INGRESS-REPLACEABLE, REQ-SWARM-ROUTING-MESH
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-NFTABLES-NAT, SWARM-HOST-NFT-COMMAND, SWARM-HOST-IPTABLES-LB-EXTENSIONS, SWARM-HOST-IPTABLES-LEGACY-MANGLE-KERNEL, BR-HOST-NO-FORWARD-DROP
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A Swarm with an ingress network created with :api-field:`Ingress` set and with a
    custom subnet and gateway

    A service created in that Swarm is reachable on its ingress-mode published ports on every node.

.. expectation:: The mtu option of a custom ingress network sets its inner MTU
    :id: SWARM-INGRESS-CUSTOM-MTU
    :observed_from: task
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-INGRESS-REPLACEABLE, REQ-SWARM-INGRESS-OVERLAY-ONLY, REQ-SWARM-NETWORK-DRIVER-OPTIONS
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* Linux nodes, and a Swarm with an ingress network created with :api-field:`Ingress`
    set and with :drvopt:`com.docker.network.driver.mtu` in its options

    The MTU of a task's interface on the ingress network is the inner MTU that
    :need:`SWARM-CTR-MTU-CLEARTEXT` and :need:`SWARM-CTR-MTU-CLEARTEXT-V6` give for a cleartext
    overlay network with that option.

.. expectation:: Every node lists the ingress network with Ingress true
    :id: SWARM-INGRESS-LIST-EVERY-NODE
    :observed_from: api:mgr, api:wkr
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-SWARM-INGRESS-CREATED-AT-INIT, REQ-SWARM-INGRESS-REPLACEABLE
    :within: SWARM-CEIL-REJOIN, SWARM-CEIL-ALLOCATION

    *Applies to:* A Swarm with an ingress network created with :api-field:`Ingress` set

    :api-op:`GET /networks` on every node lists that network and reports its :api-field:`Ingress` as
    true.

.. expectation:: The labeled network is the ingress network, and a second one is refused
    :id: SWARM-INGRESS-LABEL-IS-INGRESS
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-INGRESS-LEGACY-FORM, REQ-SWARM-INGRESS-SINGLE, REQ-NET-API-ERROR-STATUS

    *Applies to:* A Swarm with an overlay network named :network:`ingress` that was created with the
    label ``com.docker.swarm.internal`` and without :api-field:`Ingress`

    :api-op:`GET /networks/{id}` on a manager reports :api-field:`Ingress` as true for that network.
    :api-op:`POST /networks/create` for an overlay network with :api-field:`Ingress` set responds
    with a 4xx status.

.. expectation:: Beside the labeled ingress network, a second ingress responds 409
    :id: SWARM-INGRESS-LABEL-IS-INGRESS-HTTP409
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :exact_form_of: SWARM-INGRESS-LABEL-IS-INGRESS

    *Applies to:* A Swarm with an overlay network named :network:`ingress` that was created with the
    label ``com.docker.swarm.internal`` and without :api-field:`Ingress`

    :api-op:`POST /networks/create` for an overlay network with :api-field:`Ingress` set responds
    409.

.. expectation:: A service that names the labeled ingress network is refused
    :id: SWARM-INGRESS-LABEL-SERVICE-NETWORK
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-INGRESS-LEGACY-FORM, REQ-SWARM-INGRESS-NOT-FOR-SERVICES, REQ-NET-API-ERROR-STATUS, REQ-SWARM-INGRESS-MESH-ONLY

    *Applies to:* A Swarm with an overlay network named :network:`ingress` that was created with the
    label ``com.docker.swarm.internal`` and without :api-field:`Ingress`

    :api-op:`POST /services/create` or :api-op:`POST /services/{id}/update` that names that network
    in :api-field:`TaskTemplate.Networks` responds with a 4xx status. The request does not create or
    change the service.

.. expectation:: A service that names the labeled ingress network responds 400
    :id: SWARM-INGRESS-LABEL-SERVICE-NETWORK-HTTP400
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :exact_form_of: SWARM-INGRESS-LABEL-SERVICE-NETWORK

    *Applies to:* A Swarm with an overlay network named :network:`ingress` that was created with the
    label ``com.docker.swarm.internal`` and without :api-field:`Ingress`

    :api-op:`POST /services/create` or :api-op:`POST /services/{id}/update` that names that network
    in :api-field:`TaskTemplate.Networks` responds 400.

.. expectation:: The labeled ingress network is not deleted while a service publishes
    :id: SWARM-INGRESS-LABEL-DELETE-IN-USE
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-INGRESS-LEGACY-FORM, REQ-SWARM-INGRESS-REPLACEABLE, REQ-NET-API-ERROR-STATUS, REQ-SWARM-NETWORK-DELETE-IN-USE

    *Applies to:* A Swarm with an overlay network named :network:`ingress` created with the label
    ``com.docker.swarm.internal`` and without :api-field:`Ingress`, and a service with an
    ingress-mode port

    :api-op:`DELETE /networks/{id}` for that network responds 400.

.. expectation:: The refusal to delete the labeled ingress says a service depends on it
    :id: SWARM-INGRESS-LABEL-DELETE-IN-USE-MSG
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :exact_form_of: SWARM-INGRESS-LABEL-DELETE-IN-USE

    *Applies to:* A Swarm with an overlay network named :network:`ingress` created with the label
    ``com.docker.swarm.internal`` and without :api-field:`Ingress`, and a service with an
    ingress-mode port

    :api-op:`DELETE /networks/{id}` for that network responds 400, with a message that contains
    :api-error:`ingress network cannot be removed because service`.

.. expectation:: The labeled ingress network serves published ports on task nodes
    :id: SWARM-INGRESS-LABEL-PUBLISH
    :observed_from: client
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-SWARM-INGRESS-LEGACY-FORM, REQ-SWARM-ROUTING-MESH
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-NFTABLES-NAT, SWARM-HOST-NFT-COMMAND, SWARM-HOST-IPTABLES-LB-EXTENSIONS, SWARM-HOST-IPTABLES-LEGACY-MANGLE-KERNEL, BR-HOST-NO-FORWARD-DROP
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A Swarm with an overlay network named :network:`ingress` that was created with the
    label ``com.docker.swarm.internal`` and without :api-field:`Ingress`

    A client reaches a service's ingress-mode published port at the address of a node that runs a
    task of the service.

.. expectation:: The com.docker.swarm.internal label marks ingress whatever its value
    :id: SWARM-INGRESS-LABEL-ANY-VALUE
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-SWARM-INGRESS-LEGACY-FORM

    *Applies to:* A Swarm with an overlay network named :network:`ingress` that was created with the
    label ``com.docker.swarm.internal``

    :api-op:`GET /networks/{id}` on a manager reports :api-field:`Ingress` as true for that network,
    whatever value the label has, the empty string included.

.. expectation:: A node not attached to the labeled ingress serves no published ports
    :id: SWARM-INGRESS-LABEL-UNATTACHED-NODE
    :observed_from: client
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :pins: SWARM-INGRESS-LABEL-PARTLY-SUPPORTED
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A Swarm whose only ingress network is an overlay network named :network:`ingress`,
    created with the label ``com.docker.swarm.internal`` and without :api-field:`Ingress`

    On a node without a local attachment to that network, a client cannot reach an ingress-mode
    published port at the node's address.

.. expectation:: A worker not attached to the labeled ingress network does not list it
    :id: SWARM-INGRESS-LABEL-WORKER-UNLISTED
    :observed_from: api:wkr
    :depends_on: none
    :platforms: linux, windows
    :pins: SWARM-INGRESS-LABEL-PARTLY-SUPPORTED

    *Applies to:* A Swarm whose only ingress network is an overlay network named :network:`ingress`,
    created with the label ``com.docker.swarm.internal`` and without :api-field:`Ingress`

    On a worker without a local attachment to that network, :api-op:`GET /networks` does not list
    it.

.. expectation:: Standalone containers start on an attachable labeled ingress network
    :id: SWARM-INGRESS-LABEL-ATTACHABLE
    :observed_from: api:active
    :depends_on: none
    :platforms: linux, windows
    :pins: SWARM-INGRESS-LABEL-PARTLY-SUPPORTED
    :within: SWARM-CEIL-ALLOCATION

    *Applies to:* A Swarm without an ingress network

    :api-op:`POST /networks/create` for an overlay network named :network:`ingress`, with the label
    ``com.docker.swarm.internal``, with :api-field:`Attachable` set and without
    :api-field:`Ingress`, responds 201. On any node whose availability is ``active``, :api-op:`POST
    /containers/{id}/start` for a created or stopped container on that network then responds 204.

.. expectation:: A labeled ingress beside a custom one is listed but never allocated
    :id: SWARM-INGRESS-LABEL-SECOND
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :pins: SWARM-INGRESS-LABEL-PARTLY-SUPPORTED
    :within: SWARM-CEIL-ALLOCATION
    :concerns: SWARM-INGRESS-LABEL-SECOND-LOG
    :violates: REQ-SWARM-INGRESS-SINGLE

    *Applies to:* A Swarm whose ingress network was created with :api-field:`Ingress` set, under a
    name other than :network:`ingress`

    :api-op:`POST /networks/create` for an overlay network named :network:`ingress`, with the label
    ``com.docker.swarm.internal`` and without :api-field:`Ingress`, responds 201 without a warning.
    :api-op:`GET /networks` on a manager then lists two networks whose :api-field:`Ingress` is true.
    The new network is never allocated.

.. expectation:: The leader logs Cannot allocate ingress network for a labeled second one
    :id: SWARM-INGRESS-LABEL-SECOND-LOG
    :observed_from: log
    :depends_on: none
    :platforms: linux
    :pins: SWARM-INGRESS-LABEL-PARTLY-SUPPORTED
    :within: SWARM-CEIL-ALLOCATION

    *Applies to:* A Swarm whose ingress network was created with :api-field:`Ingress` set, under a
    name other than :network:`ingress`

    :api-op:`POST /networks/create` for an overlay network named :network:`ingress`, with the label
    ``com.docker.swarm.internal`` and without :api-field:`Ingress`, responds 201. The leader
    manager's log then contains :log:`Cannot allocate ingress network`.

.. expectation:: A deleted labeled ingress stays listed on attached nodes until restart
    :id: SWARM-INGRESS-LABEL-DELETE-STALE
    :observed_from: api:mgr, api:wkr
    :depends_on: none
    :platforms: linux, windows
    :pins: SWARM-INGRESS-LABEL-PARTLY-SUPPORTED
    :within: SWARM-CEIL-REJOIN, SWARM-CEIL-REASSIGNMENT

    *Applies to:* A Swarm with an overlay network named :network:`ingress` that was created with the
    label ``com.docker.swarm.internal`` and without :api-field:`Ingress`, where no service publishes
    a port in ingress mode

    :api-op:`DELETE /networks/{id}` for that network responds 204. Each node that had a local
    attachment to that network then still lists it in :api-op:`GET /networks` until the node's
    daemon restarts. Once the node's daemon has restarted, :api-op:`GET /networks` on the node no
    longer lists it.

.. expectation:: An attached node rejects tasks on an overlapping ingress until restart
    :id: SWARM-INGRESS-LABEL-DELETE-OVERLAP
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows
    :pins: SWARM-INGRESS-LABEL-PARTLY-SUPPORTED
    :concerns: SWARM-INGRESS-LABEL-DELETE-STALE
    :violates: PRIN-ERRORS-ACTIONABLE

    *Applies to:* A Swarm with a node that has a local attachment to an overlay network named
    :network:`ingress` created with the label ``com.docker.swarm.internal`` and without
    :api-field:`Ingress`, where no service publishes a port in ingress mode

    :api-op:`DELETE /networks/{id}` for that network responds 204. Before the node's daemon
    restarts, tasks on that node that attach to a new ingress network with an overlapping subnet are
    rejected. Their error in :api-op:`GET /tasks` contains :api-error:`invalid pool request`. The
    error says :api-error:`Pool overlaps with other one on this address space`. It names neither the
    subnet nor the deleted network whose copy holds it.

.. expectation:: An attached manager refuses the name ingress with 409 until restart
    :id: SWARM-INGRESS-LABEL-DELETE-RECREATE
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :pins: SWARM-INGRESS-LABEL-PARTLY-SUPPORTED
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A Swarm with a manager that has a local attachment to an overlay network named
    :network:`ingress` created with the label ``com.docker.swarm.internal`` and without
    :api-field:`Ingress`, where no service publishes a port in ingress mode

    :api-op:`DELETE /networks/{id}` for that network responds 204. Before the manager's daemon
    restarts, :api-op:`POST /networks/create` on that manager with the name :network:`ingress`
    responds 409.

.. expectation:: A labeled ingress without Ingress is created and only partly served
    :id: SWARM-INGRESS-LABEL-PARTLY-SUPPORTED
    :observed_from: api:mgr, api:wkr, client
    :depends_on: none
    :platforms: linux, windows
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-NFTABLES-NAT, SWARM-HOST-NFT-COMMAND, SWARM-HOST-IPTABLES-LB-EXTENSIONS, BR-HOST-NO-FORWARD-DROP, SWARM-HOST-IPTABLES-LEGACY-MANGLE-KERNEL
    :within: SWARM-CEIL-REJOIN, SWARM-CEIL-ALLOCATION, SWARM-CEIL-REASSIGNMENT
    :concerns: SWARM-INGRESS-LABEL-IS-INGRESS, SWARM-INGRESS-LABEL-PUBLISH, SWARM-INGRESS-LABEL-UNATTACHED-NODE, SWARM-INGRESS-LABEL-WORKER-UNLISTED, SWARM-INGRESS-LABEL-ATTACHABLE, SWARM-INGRESS-LABEL-SECOND, SWARM-INGRESS-LABEL-DELETE-STALE, SWARM-INGRESS-LABEL-DELETE-RECREATE, SWARM-INGRESS-LABEL-DELETE-OVERLAP
    :violates: REQ-SWARM-ROUTING-MESH, REQ-SWARM-INGRESS-SINGLE, REQ-SWARM-INGRESS-OVERLAY-ONLY, PRIN-NO-LEAKS

    :api-op:`POST /networks/create` for an overlay network named :network:`ingress` with the label
    ``com.docker.swarm.internal`` and without :api-field:`Ingress` responds 201, also with
    :api-field:`Attachable` true, and also while an ingress network with another name exists, in which case the
    network is never allocated. A port that a service publishes in ingress mode through such a
    network answers at the address of a node only while that node has a local attachment to the
    network. After :api-op:`DELETE /networks/{id}` for the network, each node that had a local
    attachment to it keeps its copy until its daemon restarts, and until then the tasks on that node
    that attach to a new ingress network whose subnet overlaps it are rejected.

.. _swarm-networking-deliberately-not-asserted:

***************************
 Deliberately not asserted
***************************

These items are incidental to the implementation. A change to any of them does not break any
expectation.

- The counters of Moby's own rules.
- Precedence over an operator's rule that the operator inserts at the top of the filter table's
  :ipt-chain:`INPUT` chain after an :drvopt:`encrypted` network is programmed. Such a rule can
  accept cleartext VXLAN for the network's VNI (:need:`SWARM-IPT-ENCRYPTED-INPUT-ACCEPT` covers a
  rule at the end of the chain).
- The content of Moby's own chains and tables, and their names other than :ipt-chain:`DOCKER-USER`,
  except where :ref:`swarm-lifecycle-cross-generation-cleanup` in :doc:`swarm-lifecycle.md
  <swarm-lifecycle>` pins them for cleanup and :ref:`bridge-networking-rule-content` in
  :doc:`bridge-networking.md <bridge-networking>` pins the bridge driver's rules.
  :need:`SWARM-NFT-REFERENCE` also asserts the content of the overlay driver's nftables table. The
  content includes individual match/target lines and their order in a chain. A backend change
  replaces exactly these rules. A contract asserts their effect on packets, and never the content.
  This item does not include the overlay driver's base-chain hooks and priorities.
  :need:`SWARM-NFT-OVERLAY-TABLE` asserts them.
- Which packet-filtering technology is in use, beyond what the expectations of
  :ref:`swarm-lifecycle-firewall-anchors-host-integration` in :doc:`swarm-lifecycle.md
  <swarm-lifecycle>` about anchors, cleanup and the overlay driver's nftables table assert for each
  backend.
- The existence, name or topology of internal sandboxes (per-network load-balancer sandbox, ingress
  sandbox), network-namespace paths, and generated :link-kind:`veth` / bridge / VXLAN interface
  names. One exception is the name ``ingress-sbox`` in :api-field:`Containers` of :api-op:`GET
  /networks/docker_gwbridge`, which :need:`NET-API-GWBRIDGE-INGRESS-SBOX` asserts. Another exception
  is a node's load-balancer endpoint on a network. :api-op:`GET /networks/{id}` on that node lists
  this endpoint. :need:`SWARM-OVL-IPRANGE` asserts where its address falls, and
  :need:`SWARM-LB-NAT-SOURCE-LB-ADDRESS` asserts that a task observes it as the source address in
  NAT mode. The key and the name of this endpoint are not asserted.
  :need:`NET-RESTART-NAMESPACES-REMOVED` asserts that the namespaces of these sandboxes are removed
  after a kill and restart.
- The load-balancer implementation and its scheduler, beyond the distribution property in
  :ref:`swarm-traffic-distribution` in :doc:`swarm-traffic.md <swarm-traffic>`.
- Internal refcounting, gossip timers, and reap and rejoin intervals. The surface for these is the
  convergence bounds in :ref:`swarm-lifecycle-convergence` in :doc:`swarm-lifecycle.md
  <swarm-lifecycle>`.
- The order in which the daemon allocates subnets from an address pool, addresses within a subnet,
  published ports from the auto-assignment range, and VNIs to networks created without
  :drvopt:`vxlanid_list`. Of the addresses within a subnet, :need:`SWARM-OVL-IPRANGE-GATEWAY`
  asserts which address is the gateway of a subnet with an :api-field:`IPRange`.
- The value of :api-field:`LocalLBIndex`. :cli:`docker network inspect --verbose` shows this value,
  but the value is an internal firewall mark.
- The host-port reservation mechanics behind :ref:`swarm-traffic-port-allocation` in
  :doc:`swarm-traffic.md <swarm-traffic>`.
- Whether Windows nodes can use a VNI below 4096.
