###########################
 Swarm networking: traffic
###########################

This document continues :doc:`swarm-networking`. Its conventions, definitions and host requirements
apply here.

.. _swarm-traffic-requirements:

**************
 Requirements
**************

.. req:: A dnsrr service cannot publish ingress-mode ports
    :id: REQ-SWARM-DNSRR-NO-INGRESS-PORTS
    :source_kind: docs
    :source: https://docs.docker.com/engine/swarm/ingress/#without-the-routing-mesh, api/openapi.yaml: definition EndpointSpec (Ports)

    The dnsrr endpoint mode cannot be combined with ports published in ingress mode; such a service
    publishes ports in host mode behind its own load balancer.

.. _swarm-traffic-observable-state-inside-a-container:

*************************************
 Observable state inside a container
*************************************

:ref:`network-common-endpoint-interfaces` covers the name and MAC address of an endpoint's interface
and a static address, for every network driver. :ref:`network-common-default-gateway` covers the
default route. Their expectations hold for overlay endpoints. On Linux nodes, an overlay endpoint
does not give the container a gateway or default route, whatever its :api-field:`GwPriority`.

In this section, the inner MTU is the MTU of a container's interface on an overlay network. ``base``
is the value of the network's :drvopt:`com.docker.network.driver.mtu` option, or 1500 when the
option is 0 or not set. Each expectation about the inner MTU holds for each of a container's
interfaces on overlay networks, whatever other overlay networks the container is attached to.

.. expectation:: A container's overlay IPv4 address is the one the API reports
    :id: SWARM-CTR-ADDRESS
    :observed_from: task, ctr, api
    :depends_on: os-linux, os-windows
    :platforms: linux, windows

    The IPv4 address of a container's interface on an overlay network is the :api-field:`IPAddress`
    that :api-op:`GET /containers/{id}/json` reports for that network. For a task, it is also the
    address that :api-op:`GET /tasks` on a manager reports in
    :api-field:`NetworksAttachments[].Addresses` for the network, without its prefix length. The
    address is in a subnet that :api-op:`GET /networks/{id}` reports for the network.

.. expectation:: A standalone container's configured overlay MAC address is reported
    :id: SWARM-CTR-MAC-SET
    :observed_from: ctr, api
    :depends_on: os-linux
    :platforms: linux

    *Applies to:* Linux nodes with a standalone container whose endpoint settings for an overlay
    network set :api-field:`MacAddress`

    The MAC address of the container's interface on that network is the
    :api-field:`NetworkSettings.Networks.<network>.MacAddress` that :api-op:`GET
    /containers/{id}/json` reports.

.. expectation:: A container's generated overlay MAC address is the one reported
    :id: SWARM-CTR-MAC-GENERATED
    :observed_from: task, ctr, api
    :depends_on: os-linux
    :platforms: linux

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

    *Applies to:* A running standalone container on an attachable Swarm-scoped network

    After :api-op:`POST /networks/{id}/disconnect` for the container responds 200, the container
    does not have the interface that it had on that network.

.. expectation:: A generated overlay MAC address is 02:42 then the IPv4 address
    :id: SWARM-CTR-MAC-GENERATED-FORMAT
    :observed_from: task, ctr, api
    :depends_on: none
    :platforms: linux

    *Applies to:* Linux nodes with a container's endpoint on an overlay network that does not set
    :api-field:`MacAddress`

    The MAC address of the endpoint's interface is ``02:42:`` and then the four octets of its IPv4
    address.

.. expectation:: A don't-fragment packet of the inner MTU crosses nodes unfragmented
    :id: SWARM-CTR-MTU-NO-FRAGMENT
    :observed_from: task → task, underlay
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :needs: SWARM-HOST-XFRM, SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-FIREWALL-ESP

    *Applies to:* An underlay whose MTU is ``base`` or more

    An IP packet the size of the inner MTU, with don't-fragment set, sent from a container's
    interface on an overlay network to a container on another node, arrives. It crosses the underlay
    as one outer packet that is not fragmented. This holds on both cleartext and :drvopt:`encrypted`
    networks. See :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: An inner-MTU packet's outer packet is the underlay MTU, less up to 3
    :id: SWARM-CTR-MTU-OUTER-SIZE
    :observed_from: underlay
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :needs: SWARM-HOST-XFRM, SWARM-HOST-IPTABLES-COMMAND

    *Applies to:* An underlay whose MTU is ``base``

    For an IP packet the size of the inner MTU, sent from a container's interface on an overlay
    network to a container on another node, the outer packet on the underlay is exactly the underlay
    MTU on a cleartext network. On an :drvopt:`encrypted` network, it is at most the underlay MTU
    and more than the underlay MTU minus 4. See :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: On an IPv4 data path, the cleartext inner MTU is base − 50
    :id: SWARM-CTR-MTU-CLEARTEXT
    :observed_from: task
    :depends_on: os-windows
    :platforms: linux, windows

    *Applies to:* A node with an IPv4 data-path address, and a cleartext network that, on a Windows
    node, does not have :drvopt:`com.docker.network.driver.mtu`

    The inner MTU is ``base − 50``. By default, the inner MTU is 1450.

.. expectation:: On an IPv6 data path, the cleartext inner MTU is base − 70
    :id: SWARM-CTR-MTU-CLEARTEXT-V6
    :observed_from: task
    :depends_on: none
    :platforms: linux
    :needs: SWARM-HOST-IPV6

    *Applies to:* Linux nodes with an IPv6 data-path address, and a cleartext network

    The inner MTU is ``base − 70``. By default, the inner MTU is 1430.

.. expectation:: On an IPv4 data path, the encrypted inner MTU is base − 76, 4-aligned
    :id: SWARM-CTR-MTU-ENCRYPTED
    :observed_from: task
    :depends_on: none
    :platforms: linux
    :needs: SWARM-HOST-XFRM, SWARM-HOST-IPTABLES-COMMAND

    *Applies to:* Linux nodes with an IPv4 data-path address, and an :drvopt:`encrypted` network

    The inner MTU is ``base − 50 − 26``, rounded down to a multiple of 4. By default, the inner MTU
    is 1424.

.. expectation:: On an IPv6 data path, the encrypted inner MTU is base − 96, 4-aligned
    :id: SWARM-CTR-MTU-ENCRYPTED-V6
    :observed_from: task
    :depends_on: none
    :platforms: linux
    :needs: SWARM-HOST-XFRM, SWARM-HOST-IPV6, SWARM-HOST-ESP-IPV6, SWARM-HOST-IP6TABLES-COMMAND

    *Applies to:* Linux nodes with an IPv6 data-path address, and an :drvopt:`encrypted` network

    The inner MTU is ``base − 70 − 26``, rounded down to a multiple of 4. By default, the inner MTU
    is 1404.

.. expectation:: A Windows node's cleartext overlay MTU matches the Linux nodes' MTU
    :id: SWARM-CTR-MTU-CLEARTEXT-WINDOWS
    :observed_from: task on two nodes
    :depends_on: os-windows
    :platforms: windows

    *Applies to:* Windows nodes, and a cleartext overlay without
    :drvopt:`com.docker.network.driver.mtu`

    An endpoint's MTU is the MTU that Linux nodes report for the same network.

.. expectation:: A set mtu option gives Linux and Windows nodes different inner MTUs
    :id: SWARM-CTR-MTU-OPTION-MIXED-PLATFORM
    :observed_from: task on two nodes
    :depends_on: os-linux
    :platforms: linux

    *Applies to:* A Linux node and a Windows node with tasks on a cleartext overlay network whose
    :drvopt:`com.docker.network.driver.mtu` is set to a value other than 1500

    The two nodes' inner MTUs on that network differ, because Windows nodes ignore the option. A
    frame larger than the smaller inner MTU, sent by a task on the node with the larger one, does
    not reach the other node. The sender does not get an ICMP error. TCP traffic between the two
    nodes is not affected.

.. expectation:: A container on a non-internal overlay has a default route
    :id: SWARM-CTR-DEFAULT-ROUTE
    :observed_from: task, ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows

    A container on a non-internal overlay has a default route.

.. expectation:: A container on a multi-subnet overlay has a route to each other subnet
    :id: SWARM-CTR-MULTI-SUBNET-ROUTES
    :observed_from: task
    :depends_on: os-linux
    :platforms: linux

    A container on a multi-subnet overlay has a static route to each subnet of the overlay other
    than its own subnet.

The expectation about different inner MTUs on two nodes documents a failure mode that is hard to
diagnose and common in the field. It occurs especially where the overlay runs inside further
encapsulation. In this failure mode, large TCP transfers work, because each side keeps its segments
within the MSS that the other side advertises. But large UDP datagrams and don't-fragment pings
disappear without an error.

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

.. expectation:: A unicast frame reaches its destination on the same overlay subnet
    :id: SWARM-EW-UNICAST
    :observed_from: task, ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows

    A unicast frame that a container sends reaches the destination container on the same overlay
    subnet.

.. expectation:: A broadcast frame reaches the same subnet's containers on its node only
    :id: SWARM-EW-BROADCAST-LOCAL
    :observed_from: task, ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows

    A broadcast frame that a container sends reaches the other containers on the same overlay subnet
    on its own node. It does not reach containers on other nodes.

.. expectation:: A multicast datagram reaches the group's members on its node only
    :id: SWARM-EW-MULTICAST-LOCAL
    :observed_from: task, ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows

    A multicast datagram that a container sends to a group reaches the other members of the group on
    the same overlay subnet on its own node. It does not reach members on other nodes.

.. expectation:: A unicast packet reaches another subnet of the same overlay
    :id: SWARM-EW-CROSS-SUBNET
    :observed_from: task, ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows

    A unicast packet that a container on subnet A sends reaches the destination container on subnet
    B of the same overlay network.

.. expectation:: TCP, UDP and SCTP traffic reaches containers on the same overlay
    :id: SWARM-EW-PROTOCOLS
    :observed_from: task, ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows

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
    :depends_on: os-windows
    :platforms: linux, windows

    A don't-fragment packet larger than the MTU of the sender's interface on the overlay fails at
    the sender with :errno:`EMSGSIZE`.

.. expectation:: Containers with no overlay in common cannot reach each other
    :id: SWARM-EW-NO-SHARED-NETWORK
    :observed_from: task, ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows

    *Applies to:* Two containers that do not have an overlay network in common

    Neither container can reach an address that the other container has on an overlay network.

.. expectation:: Containers cannot reach each other on a daemon-created gateway network
    :id: SWARM-EW-GW-NETWORK
    :observed_from: task, ctr
    :depends_on: os-linux
    :platforms: linux

    Two containers on one node cannot reach each other's addresses on a gateway network
    (:ref:`swarm-traffic-gateway-network`) that the operator did not create.

The expectation about the gateway network is an isolation property. Containers on unrelated overlays
on a node all have addresses on the node's gateway network. If they could reach each other at those
addresses, the overlays would not isolate them.

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
built, and what is forwarded upstream. Its expectations hold inside service tasks. They also hold
for the names and addresses of the endpoints in this section, including endpoints on other nodes.
Each expectation keeps the type and dependencies that it has in container-dns.md. This section
covers the names that Swarm adds, on overlay networks unless an expectation says otherwise. For the
expectations in container-dns.md that depend on a container's DNS options, a service task is created
with those options in the service's DNS config. On Linux nodes, a :api-field:`Links` alias of the
querying container takes precedence over the names in this section
(:ref:`container-dns-names-and-answers`).

In this section, a service's running tasks are the tasks that :api-op:`GET /tasks` reports with
:api-field:`Status.State` and :api-field:`DesiredState` both ``running``. If a task has a
healthcheck, it is a running task only once its container has reported healthy. This excludes a task
whose container has reported unhealthy at any time, even if a later health check passed.
:dns-name:`<service>` is the name of a service, and ``<alias>`` is one of the service's network
aliases.

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

    *Applies to:* A service in vip mode that has a running task

    :dns-name:`<service>` resolves to the service's VIP on one of the overlay networks, other than
    the ingress network, that the querier shares with it.

.. expectation:: A dnsrr service name resolves to its running tasks
    :id: SWARM-SD-DNSRR
    :observed_from: task, ctr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A service in dnsrr mode

    :dns-name:`<service>` resolves to the addresses of the running tasks.

.. expectation:: A dnsrr service reports no VIP
    :id: SWARM-SD-DNSRR-NO-VIP
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A service in ``dnsrr`` endpoint mode

    :api-op:`GET /services/{id}` for the service does not report a VIP in
    :api-field:`Endpoint.VirtualIPs`.

.. expectation:: tasks.<service> resolves to all running tasks
    :id: SWARM-SD-TASKS
    :observed_from: task, ctr
    :depends_on: none
    :platforms: linux, windows

    :dns-name:`tasks.<service>` resolves to the addresses of all running tasks.

.. expectation:: A service alias resolves as the service name does
    :id: SWARM-SD-ALIAS
    :observed_from: task, ctr
    :depends_on: none
    :platforms: linux, windows

    A service network alias resolves as :dns-name:`<service>` does: to the VIP in vip mode while the
    service has a running task, and to the addresses of the running tasks in dnsrr mode.

.. expectation:: tasks.<alias> resolves to the tasks of every service with the alias
    :id: SWARM-SD-TASKS-ALIAS
    :observed_from: task, ctr
    :depends_on: none
    :platforms: linux, windows

    :dns-name:`tasks.<alias>` resolves to the addresses of the running tasks of every service that
    has that alias.

.. expectation:: A running task's container name resolves to its shared overlay address
    :id: SWARM-SD-TASK-NAME
    :observed_from: task, ctr
    :depends_on: none
    :platforms: linux, windows

    A running task's container name resolves to its address on an overlay network that it shares
    with the querier.

.. expectation:: A replicated task's container name is <service>.<slot>.<taskID>
    :id: SWARM-SD-TASK-NAME-REPLICATED
    :observed_from: task, ctr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A task of a replicated service

    The task's container name is :dns-name:`<service>.<slot>.<taskID>`. ``<slot>`` is the task's
    :api-field:`Slot` that :api-op:`GET /tasks` reports.

.. expectation:: A global task's container name is <service>.<nodeID>.<taskID>
    :id: SWARM-SD-TASK-NAME-GLOBAL
    :observed_from: task, ctr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A task of a global service

    The task's container name is :dns-name:`<service>.<nodeID>.<taskID>`.

.. expectation:: A task with a healthcheck resolves only once it reports healthy
    :id: SWARM-SD-HEALTH-PENDING
    :observed_from: task, ctr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A task with a healthcheck whose container has not yet reported healthy

    The task's container name does not resolve to its address on an overlay network.
    :dns-name:`tasks.<service>` and :dns-name:`tasks.<alias>` do not resolve to the task's address.
    In dnsrr mode, :dns-name:`<service>` and the service's aliases do not resolve to the task's
    address either. A reverse lookup of the task's address on an overlay network does not return the
    task's container name.

.. expectation:: A task stops resolving once it stops or reports unhealthy
    :id: SWARM-SD-TASK-STOPPED
    :observed_from: task, ctr
    :depends_on: none
    :platforms: linux, windows

    Once a task's desired state is not ``running``, or its container has stopped or has reported
    unhealthy at any time, its container name stops resolving to its addresses on overlay networks.
    :dns-name:`tasks.<service>` and, in dnsrr mode, :dns-name:`<service>` then stop resolving to the
    task's address, and a reverse lookup of that address stops returning the task's container name.

.. expectation:: A removed service's names stop resolving to its VIP and tasks
    :id: SWARM-SD-SERVICE-REMOVED
    :observed_from: task, ctr
    :depends_on: none
    :platforms: linux, windows

    Once no container of a removed service is running, the service's name and aliases stop resolving
    to the service's VIP and to the addresses of its tasks.

.. expectation:: A vip service with no running task is forwarded upstream
    :id: SWARM-SD-VIP-NO-TASKS
    :observed_from: task, ctr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A service in vip mode that still exists, such as one scaled to 0 replicas

    Once no container of the service is running, the embedded resolver handles a query for
    :dns-name:`<service>` as it handles a name that it does not have a record for.

.. expectation:: A vip service created with no replicas is forwarded upstream
    :id: SWARM-SD-VIP-NO-REPLICAS
    :observed_from: task, ctr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A service in vip mode that was created with no replicas

    The embedded resolver handles a query for :dns-name:`<service>` as it handles a name that it
    does not have a record for.

.. expectation:: A vip service with no healthy container is forwarded upstream
    :id: SWARM-SD-VIP-NONE-HEALTHY
    :observed_from: task, ctr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A service in vip mode with running containers, each with a healthcheck, none of
    which has reported healthy

    The embedded resolver handles a query for :dns-name:`<service>` as it handles a name that it
    does not have a record for.

.. expectation:: The VIP of a service without running tasks gives EHOSTUNREACH
    :id: SWARM-SD-VIP-UNREACHABLE
    :observed_from: task, ctr
    :depends_on: os-linux
    :platforms: linux

    *Applies to:* Linux nodes, and a service in vip mode that still exists, such as one scaled to 0
    replicas

    Once no container of the service is running, a TCP connection to the service's VIP fails at the
    client with :errno:`EHOSTUNREACH`. The client does not get a TCP reset.

.. expectation:: A renamed standalone container resolves by its new name on other nodes
    :id: SWARM-SD-RENAME
    :observed_from: task, ctr
    :depends_on: none
    :platforms: linux, windows
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A standalone container on an attachable overlay, whose hostname is not its name

    Once :api-op:`POST /containers/{id}/rename` for the container responds 204, its new name
    resolves for containers on other nodes within the :need:`[[title|lower]] ceiling ([[seconds]] s)
    <SWARM-CEIL-REASSIGNMENT>`. Within the same ceiling, its old name stops resolving to its address
    for those containers.

.. expectation:: A service alias keeps resolving through a rolling update
    :id: SWARM-SD-ALIAS-ROLLING-UPDATE
    :observed_from: task, ctr
    :depends_on: none
    :platforms: linux, windows
    :within: SWARM-CEIL-ALLOCATION

    *Applies to:* A service whose tasks have a network alias

    A rolling update of the service in which at least one task keeps running throughout starts new
    tasks that also have that alias. Examples are an update in start-first order and an update of
    fewer tasks at a time than are running. The alias does not stop resolving for longer than the
    allocation ceiling: within any interval of that length, the alias resolves.

.. expectation:: A name on Swarm-scoped and local networks resolves to the Swarm address
    :id: SWARM-SD-PREFER-SWARM-SCOPED
    :observed_from: ctr
    :depends_on: os-windows
    :platforms: linux, windows

    *Applies to:* A container attached to a Swarm-scoped network and a local network, such as a
    user-defined bridge network, and records for the same name on each of those networks
    (:ref:`container-dns-names-and-answers`)

    A query from that container for the name gets the address on the Swarm-scoped network, and not
    the address on the local network.

.. expectation:: Services that share only the ingress network do not resolve each other
    :id: SWARM-SD-INGRESS-ONLY
    :observed_from: task
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* Two services that publish ports in ingress mode and do not share another network

    For a task of either service, the other service's name does not resolve. The container names of
    the other service's tasks do not resolve for that task either.

.. expectation:: On a node-local Swarm network, task names resolve as container names do
    :id: SWARM-SD-NODE-LOCAL
    :observed_from: task, ctr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A Swarm-scoped network of a node-local driver

    A task's names resolve for the containers on its node that are attached to the network. They
    resolve as a container's names do on any user-defined network.

.. expectation:: On a node-local Swarm network, service names do not resolve
    :id: SWARM-SD-NODE-LOCAL-NO-SERVICE
    :observed_from: task, ctr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A Swarm-scoped network of a node-local driver

    On that network, :dns-name:`<service>`, :dns-name:`tasks.<service>` and service aliases do not
    resolve.

.. expectation:: On a node-local Swarm network, a task resolves before it reports healthy
    :id: SWARM-SD-NODE-LOCAL-NO-HEALTH-WAIT
    :observed_from: task, ctr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A Swarm-scoped network of a node-local driver, and a task on it with a healthcheck

    The task's names on that network resolve without waiting for its container to report healthy.

.. _swarm-traffic-load-balancing-east-west-vip:

********************************
 Load balancing (east-west VIP)
********************************

The client that sends to a VIP can be a service task or a standalone container on an ``attachable``
overlay. Every expectation in this section holds for either client. The backends behind a VIP are
always service tasks. An expectation observed at the receiving end is observed from ``task``. A VIP
load-balances across a set of tasks when flows to the VIP reach every task in the set.

.. expectation:: Traffic to a service VIP reaches the service's tasks
    :id: SWARM-LB-VIP
    :observed_from: task, ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows

    Traffic to a service VIP reaches that service's tasks.

.. expectation:: The VIP load-balances TCP, UDP and SCTP traffic
    :id: SWARM-LB-PROTOCOLS
    :observed_from: task, ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND

    The VIP load-balances TCP, UDP and SCTP traffic.

.. expectation:: A node without IPVS logs that it cannot get ipvs family information
    :id: SWARM-LB-NO-IPVS
    :observed_from: log
    :depends_on: none
    :platforms: linux
    :pins: SWARM-HOST-IPVS

    *Applies to:* Linux nodes whose kernel does not offer IPVS, and a service in vip mode with a
    running task on a network that is programmed on the node

    The daemon log of that node contains :log:`Could not get ipvs family information from the
    kernel`. See :ref:`Host requirements <swarm-networking-host-requirements>`.

.. expectation:: All packets of a flow to a VIP reach the same task
    :id: SWARM-LB-FLOW-SAME-TASK
    :observed_from: task, ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND

    All packets of a flow reach the same task.

.. expectation:: Traffic to any port of a service VIP reaches the service's tasks
    :id: SWARM-LB-ANY-PORT
    :observed_from: task, ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND

    Traffic to any port on a service VIP reaches the service's tasks, whether or not the service
    publishes that port.

.. expectation:: A node without a task of a service reaches the tasks through the VIP
    :id: SWARM-LB-TASK-FREE-NODE
    :observed_from: task or ctr on a task-free node
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND

    *Applies to:* A node without a task of a service

    Traffic to the service's VIP from a task or container on that node reaches the service's tasks.

.. expectation:: In NAT mode, a task sees a stable per-node source, not the client's
    :id: SWARM-LB-NAT-SOURCE
    :observed_from: task
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND

    *Applies to:* NAT mode (the default)

    The task observes a source address that is stable for each client node. That address is neither
    the client's nor any task's.

.. expectation:: In NAT mode, the source a task sees is the client node's LB address
    :id: SWARM-LB-NAT-SOURCE-LB-ADDRESS
    :observed_from: task
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND

    *Applies to:* NAT mode

    The source address that the task observes is the load-balancer address of the client's node on
    the overlay.

.. expectation:: In NAT mode, a task sees its own address as the destination
    :id: SWARM-LB-NAT-DESTINATION
    :observed_from: task
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND

    *Applies to:* NAT mode

    The destination the task observes is its own address.

.. expectation:: In DSR mode, a task sees the client's real source address
    :id: SWARM-LB-DSR-SOURCE
    :observed_from: task
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND

    *Applies to:* DSR mode (the :drvopt:`dsr` option)

    The task observes the client's real source address.

.. expectation:: In DSR mode, a task sees the VIP as the destination
    :id: SWARM-LB-DSR-DESTINATION
    :observed_from: task
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND

    *Applies to:* DSR mode

    The destination the task observes is the VIP.

.. expectation:: A connection to a task's own address arrives with the sender's address
    :id: SWARM-LB-TASK-ADDRESS
    :observed_from: task
    :depends_on: os-linux, os-windows
    :platforms: linux, windows

    *Applies to:* A task, and a sender task or container that shares an overlay subnet with it

    The sender connects to the task's own address on that subnet. The connection arrives at the task
    with the sender's address on that subnet as its source.

.. expectation:: A task's connections to its own service's VIP reach the service's tasks
    :id: SWARM-LB-SELF-VIP
    :observed_from: task
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND

    Every connection from a task to the VIP of its own service reaches a task of the service.

.. expectation:: In NAT mode, a task's own VIP spreads its connections over all tasks
    :id: SWARM-LB-SELF-VIP-NAT
    :observed_from: task
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND

    *Applies to:* NAT mode

    The VIP of a service load-balances the connections from one of its tasks across all of the
    service's tasks, that task included.

.. expectation:: In DSR mode, a task's connections to its own VIP come back to it
    :id: SWARM-LB-SELF-VIP-DSR
    :observed_from: task
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND

    *Applies to:* DSR mode

    Every connection from a task to the VIP of its own service reaches the task itself. The task
    observes the VIP as the source address of the connection. See
    :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. _swarm-traffic-distribution:

Distribution
============

The distribution property allows round-robin, equal-weight random, and 5-tuple hashing. It excludes
least-connections and other schedulers that depend on backend state. Clause (1), a limit, cannot
exclude them on its own because least-connections also converges to 1/*k* with identical backends.
Clause (2), invariance, tells them apart. There are defensible reasons to switch to a scheduler that
depends on backend state. For that reason, the property is a detector. Such a switch is a deliberate
change. The property does not fix a number of flows or a tolerance on *fᵢ(n)/n*. A difference in
connection holding time is one of the properties in clause (2), and tells least-connections apart
from round-robin.

    Let *H* be the set of running tasks, \|\ *H*\| = *k*, held constant. For *n* client flows let
    *fᵢ(n)* be the number of them that reach task *i*. Then (1) lim *fᵢ(n)/n* = 1/*k* for every *i*
    ∈ *H*, and (2) that limit is invariant to any observable property of the tasks: connection
    holding time, in-flight concurrency, latency, byte volume, or load.

.. expectation:: East-west VIP flows split evenly over running tasks, whatever their load
    :id: SWARM-LBDIST-VIP
    :observed_from: task
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND

    Clauses (1) and (2) of the property above hold for east-west VIP. In DSR mode, they do not hold
    for flows from a task to the VIP of its own service. See
    :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: Ingress flows split evenly over running tasks, whatever their load
    :id: SWARM-LBDIST-INGRESS
    :observed_from: client, task
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND

    Clauses (1) and (2) of the property above hold for ingress.

.. _swarm-traffic-health-and-drain:

Health and drain
================

Every expectation in this subsection holds for flows through the service's VIP and for flows that
enter through an ingress-mode published port. A task shuts down gracefully when the orchestrator
shuts it down, for example after a scale-down, an update that replaces the task, the removal of its
service or a drain of its node. :api-op:`POST /containers/{id}/stop` for the task's container is not
a graceful shutdown in these expectations. A flow starts on a node when its client is on that node,
or when it enters through a published port on that node. In DSR mode, the expectations do not apply
to flows from a task to the VIP of its own service
(:ref:`swarm-lifecycle-known-divergences-non-goals`). In these expectations, a task's process
ignores its stop signal and keeps accepting connections until it is killed at the end of the stop
grace period.

.. expectation:: A task with a healthcheck gets no traffic until it first reports healthy
    :id: SWARM-LBHEALTH-PENDING
    :observed_from: task
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND

    A task with a healthcheck does not receive traffic until it first reports healthy.

.. expectation:: A task without a healthcheck gets traffic once it is running
    :id: SWARM-LBHEALTH-NO-HEALTHCHECK
    :observed_from: task
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND

    A task that does not have a healthcheck receives traffic once it is running, without waiting for
    a health status.

.. expectation:: An unhealthy task gets no new flows within the allocation ceiling
    :id: SWARM-LBHEALTH-UNHEALTHY
    :observed_from: task
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :within: SWARM-CEIL-ALLOCATION

    *Applies to:* A service whose tasks have a healthcheck and a stop grace period longer than the
    :need:`[[title|lower]] ceiling ([[seconds]] s) <SWARM-CEIL-ALLOCATION>`

    Within the allocation ceiling after one of the service's tasks becomes unhealthy, no new flows
    from any node reach that task.

.. expectation:: An established connection works until the stopping task closes it
    :id: SWARM-LBHEALTH-DRAIN-ESTABLISHED
    :observed_from: task
    :depends_on: os-linux, os-windows
    :platforms: linux, windows

    *Applies to:* An established connection to a task

    The task shuts down gracefully. The connection keeps exchanging data in both directions until
    the task closes it, within the task's stop grace period.

.. expectation:: No flow from the task's node reaches it after its kill event
    :id: SWARM-LBHEALTH-DRAIN-LOCAL
    :observed_from: task
    :depends_on: os-linux, os-windows
    :platforms: linux, windows

    *Applies to:* A task with a stop grace period of at least 10 seconds

    When the task shuts down gracefully, no flow that starts on the task's node after the
    container's :api-event:`kill` event reaches the task. This holds at once, without a ceiling,
    until the container's :api-event:`die` event. See
    :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: Within the ceiling, flows from other nodes stop reaching a stopping task
    :id: SWARM-LBHEALTH-DRAIN-REMOTE
    :observed_from: task
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :within: SWARM-CEIL-ALLOCATION

    *Applies to:* A task with a stop grace period longer than the :need:`[[title|lower]] ceiling
    ([[seconds]] s) <SWARM-CEIL-ALLOCATION>`

    The task shuts down gracefully. Within that ceiling after :api-op:`GET /tasks` first reports a
    :api-field:`DesiredState` other than ``running`` for the task, no flow that starts on another
    node reaches the task. See :ref:`swarm-lifecycle-known-divergences-non-goals`.

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

The expectations in this section hold for both modes. :ref:`swarm-traffic-hairpin` covers hairpin
paths. :ref:`swarm-traffic-api-value-mode-ingress` covers what is specific to the routing mesh.
:ref:`swarm-traffic-api-value-mode-host` covers what is specific to :api-value:`mode=host`.

Every expectation in this section and its subsections holds whether or not the :drvopt:`dsr` option
is set on the overlay networks that a service is attached to, other than the ingress network.

.. expectation:: A flow to a published port reaches a task on its target port
    :id: SWARM-PUB-TARGET-PORT
    :observed_from: task
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND

    *Applies to:* A service that publishes a port

    A flow from a client to that port at a node's address reaches a task of the service on its
    target port.

.. expectation:: A client reaches a task through a TCP, UDP or SCTP published port
    :id: SWARM-PUB-PROTOCOLS
    :observed_from: client
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND

    A client reaches a task through a published port, whether the port is TCP, UDP or SCTP. See
    :need:`SWARM-EW-WINDOWS-SCTP`.

.. expectation:: A task's replies reach the client that connected to its published port
    :id: SWARM-PUB-REPLIES
    :observed_from: client
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND

    *Applies to:* A service that publishes a port

    Replies from a task of the service to a client that connected to that port reach the client.

.. expectation:: A datagram to a UDP port with no listener gets ICMP port-unreachable
    :id: SWARM-PUB-UDP-NO-LISTENER
    :observed_from: client
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND

    *Applies to:* A service that publishes a UDP port, with tasks in which nothing listens on the
    target port

    A UDP datagram from a client to that port at a node's address gets an ICMP port-unreachable
    error at the client.

.. expectation:: A client does not reach a task on a port its service does not publish
    :id: SWARM-PUB-UNPUBLISHED-PORT
    :observed_from: client → task
    :depends_on: os-linux, os-windows
    :platforms: linux, windows

    *Applies to:* A task that listens on a port that its service does not publish

    A connection or datagram that a client sends to that port at a node's address does not reach the
    task.

.. expectation:: A spec that lists the same chosen published port twice responds 400
    :id: SWARM-PUB-DUPLICATE-PORT-HTTP400
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    :api-op:`POST /services/create` or :api-op:`POST /services/{id}/update` with a spec that lists
    the same chosen published port twice responds 400, whatever the publish modes and target ports
    of the two entries are. The service is not created or not changed.

.. expectation:: docker service create with one published port twice fails
    :id: SWARM-PUB-DUPLICATE-PORT-CLI
    :observed_from: api:mgr
    :depends_on: cli
    :platforms: linux, windows
    :cli_for: SWARM-PUB-DUPLICATE-PORT-HTTP400

    :cli:`docker service create` with two :cli-flag:`--publish` options that set the same published
    port exits non-zero and prints the daemon's error. The command does not create the service.

.. expectation:: docker service update --publish-add of a used port to a new target fails
    :id: SWARM-PUB-DUPLICATE-PORT-UPDATE-CLI
    :observed_from: api:mgr
    :depends_on: cli
    :platforms: linux, windows

    *Applies to:* A service with a chosen published port to some target port

    :cli:`docker service update --publish-add` that publishes the same published port to another
    target port exits non-zero and prints the daemon's error. The command does not change the
    service.

.. expectation:: --publish-add of a port the service already has changes nothing
    :id: SWARM-PUB-PUBLISH-ADD-EXISTING
    :observed_from: api:mgr
    :depends_on: cli
    :platforms: linux, windows

    *Applies to:* A service with a chosen published port

    :cli:`docker service update --publish-add` with the same published port, target port and publish
    mode exits zero. :cli:`docker service inspect` then prints the same ports as before.

.. expectation:: --publish-add of an auto-assigned port's number reassigns that port
    :id: SWARM-PUB-PUBLISH-ADD-AUTO-NUMBER
    :observed_from: api:mgr
    :depends_on: cli
    :platforms: linux, windows

    *Applies to:* A service with an auto-assigned published port

    :cli:`docker service update --publish-add` that publishes that port's number to another target
    port exits zero. :cli:`docker service inspect` then prints that number for the new entry, and
    another number for the auto-assigned entry. See
    :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: A published port defaults to tcp and to ingress mode
    :id: SWARM-PUB-DEFAULTS
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    :api-op:`POST /services/create` with a published port that does not set :api-field:`Protocol`
    responds 201. :api-op:`GET /services/{id}` then reports ``tcp`` as the port's protocol. It
    reports :network:`ingress` as the publish mode of a port that does not set
    :api-field:`PublishMode`.

.. expectation:: An unknown protocol or publish mode is accepted as tcp or ingress
    :id: SWARM-PUB-UNKNOWN-PROTOCOL
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    :api-op:`POST /services/create` with a published port whose :api-field:`Protocol` is not a
    protocol name, such as ``bogus``, responds 201. :api-op:`GET /services/{id}` then reports
    ``tcp`` as the port's protocol. It reports :network:`ingress` as the publish mode of a port with
    an unknown :api-field:`PublishMode`. See :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: docker service create applies its port, endpoint mode and alias flags
    :id: SWARM-PUB-CREATE-FLAGS
    :observed_from: api:mgr
    :depends_on: cli
    :platforms: linux, windows

    :cli:`docker service create` with :cli-flag:`--publish`, in its short and long forms,
    :cli-flag:`--endpoint-mode` and :cli-flag:`--network` with :cli-flag:`alias=` exits zero when
    the endpoint mode is ``vip`` or every port is published in :api-value:`mode=host`. Then
    :cli:`docker service inspect` prints the ports, publish modes, endpoint mode and aliases that
    the flags name.

.. expectation:: docker service update applies its port, endpoint mode and alias flags
    :id: SWARM-PUB-UPDATE-FLAGS
    :observed_from: api:mgr
    :depends_on: cli
    :platforms: linux, windows

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

    *Applies to:* A removed service whose tasks are all stopped, and no other service or container
    that publishes one of its ports

    The service's published ports do not answer. See
    :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: A service's old published port does not answer after a change
    :id: SWARM-PUB-PORT-CHANGE-OLD
    :observed_from: client, node
    :depends_on: os-linux, os-windows
    :platforms: linux, windows

    *Applies to:* A service whose published port was changed, and whose tasks from before the change
    are all stopped, and no other service or container that publishes the old port

    The old port does not answer. See :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: A client reaches a service's tasks on its updated published port
    :id: SWARM-PUB-PORT-CHANGE
    :observed_from: client, node
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND

    *Applies to:* A service that publishes a port

    :api-op:`POST /services/{id}/update` changes the service's published port. A client reaches a
    task of the service on the new port at the address of a node that runs a task of the service.
    This holds once a task from after the update is running.

.. expectation:: With userland-proxy, a published port is reachable over IPv6 off-node
    :id: SWARM-PUB-V6-USERLAND-PROXY
    :observed_from: client
    :depends_on: os-linux
    :platforms: linux
    :needs: SWARM-HOST-IPV6, SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND

    *Applies to:* A daemon with :daemon-json:`userland-proxy` true

    A published port is reachable over IPv6 at the node's IPv6 addresses, from off-node. This holds
    for TCP and UDP.

.. expectation:: With userland-proxy, a published port is reachable over IPv6 on-node
    :id: SWARM-PUB-V6-USERLAND-PROXY-LOCAL
    :observed_from: node
    :depends_on: os-linux
    :platforms: linux
    :needs: SWARM-HOST-IPV6, SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND

    *Applies to:* A daemon with :daemon-json:`userland-proxy` true

    A published port is reachable over IPv6 from the node itself, at the node's own IPv6 address and
    at ``::1``. This holds for TCP and UDP.

.. expectation:: An accepted IPv6 connection to a published port reaches a task
    :id: SWARM-PUB-V6-DELIVERY
    :observed_from: client, node
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :needs: SWARM-HOST-IPV6, SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND

    When a node accepts an IPv6 TCP connection to a published port, a task of the service receives
    the connection.

.. expectation:: Without userland-proxy, a client cannot reach a task over IPv6
    :id: SWARM-PUB-V6-NO-USERLAND-PROXY
    :observed_from: client, node
    :depends_on: os-linux
    :platforms: linux
    :needs: BR-HOST-BR-NETFILTER-NO-PROXY

    *Applies to:* A daemon with :daemon-json:`userland-proxy` false, and a port published in ingress
    mode, or in :api-value:`mode=host` by a task whose gateway network is :network:`docker_gwbridge`

    A client cannot reach a task of the service through that port over IPv6, at any IPv6 address of
    the node. This holds for TCP and UDP.

These IPv6 expectations do not mean that the service mesh supports IPv6. The service mesh is
IPv4-only (:ref:`swarm-lifecycle-known-divergences-non-goals`). The ingress network and the gateway
network do not give a task an IPv6 address. The proxy accepts traffic on ``[::]`` and forwards it to
the task over IPv4, as it does for any IPv4-only container. Thus the task sees the connection come
from the proxy. For this reason, the real-source expectation in
:ref:`swarm-traffic-api-value-mode-host` does not hold for IPv6 clients.

A bug revealed the expectation about traffic left unserved. In that bug, IPv6 connections to a
published port were accepted and never served (`moby/moby#53091
<https://github.com/moby/moby/issues/53091>`__). The userland proxy is necessary for a published
port to be reachable over IPv6. Without the proxy, the port is not reachable over IPv6, and this
prevents that failure. The expectation about that is a detector because a change that makes IPv6
reachable without the proxy would be legitimate. Such a change also calls for a change to this spec
and to the documentation.

.. _swarm-traffic-hairpin:

Hairpin
=======

The six paths below are independent and can fail separately. The first five hold for both publish
modes. For :api-value:`mode=host`, they hold on the nodes that run a task. In this subsection, the
node's address is one of its non-loopback IPv4 addresses. In this subsection, each task or container
that connects to the node's address must be attached to at least one network that is not
``internal``.

.. expectation:: A published port is reachable from the node via 127.0.0.1
    :id: SWARM-HAIRPIN-LOOPBACK
    :observed_from: node
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND

    A published port is reachable from the node itself via ``127.0.0.1``.

.. expectation:: A published port is reachable from the node via the node's address
    :id: SWARM-HAIRPIN-NODE-ADDRESS
    :observed_from: node
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND

    A published port is reachable from the node itself via the node's address.

.. expectation:: A container on a local bridge reaches a published port at the node
    :id: SWARM-HAIRPIN-BRIDGE-CTR
    :observed_from: ctr
    :depends_on: os-linux
    :platforms: linux
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND

    A published port is reachable from a container on a local bridge network, via the node's
    address.

.. expectation:: A container on the service's overlay reaches the port at its node
    :id: SWARM-HAIRPIN-SERVICE-NETWORK
    :observed_from: task, ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND

    *Applies to:* A service that publishes a port, and a task or container attached to one of the
    service's overlay networks

    The port is reachable via the node's address from the task or container on the same node.

.. expectation:: A container on other overlays reaches the port at its node
    :id: SWARM-HAIRPIN-OTHER-NETWORK
    :observed_from: task, ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND

    *Applies to:* A service that publishes a port, and a task or container attached only to overlay
    networks that the service is not on

    The port is reachable via the node's address from the task or container on the same node.

.. expectation:: A task reaches its own mode=host published port at its node's address
    :id: SWARM-HAIRPIN-HOST-MODE-SELF
    :observed_from: task
    :depends_on: os-linux, os-windows
    :platforms: linux, windows

    *Applies to:* A service that publishes a port in :api-value:`mode=host`

    A task of the service reaches its own published port via its node's address.

.. expectation:: Hairpin reachability holds with userland-proxy true and false
    :id: SWARM-HAIRPIN-USERLAND-PROXY
    :observed_from: node, ctr, task
    :depends_on: os-linux
    :platforms: linux
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND

    Each other expectation of the table in §7.1 holds with :daemon-json:`userland-proxy` true and
    with :daemon-json:`userland-proxy` false.

On Linux nodes, the two overlay expectations and the :api-value:`mode=host` self-hairpin expectation
are the paths whose traffic leaves a bridge and then enters the same bridge again. An overlay
container reaches the node's address through :network:`docker_gwbridge`. The connection then reaches
the ingress load balancer, or the task that serves the port, through :network:`docker_gwbridge`
again. Only those paths are subject to the bridge's inter-container communication setting. A gateway
network that the daemon creates has inter-container communication disabled
(:ref:`swarm-traffic-gateway-network`). A published port is an explicit export to anything that can
route to the node. Thus that setting must not block it.

The userland-proxy expectation exists because the proxy hides these paths. With the proxy, the
connection ends in the host's network namespace and does not enter the bridge again. Without the
proxy, the traffic is DNATed back across the bridge. Access control should not depend on whether the
userland proxy is enabled. Access to a published port from an overlay regressed in 29.8.0 and
29.8.1. Nobody noticed the regression because it broke access only with
:daemon-json:`userland-proxy` false (`moby/moby#53713
<https://github.com/moby/moby/issues/53713>`__).

.. _swarm-traffic-api-value-mode-ingress:

:api-value:`mode=ingress`
=========================

The routing mesh publishes an ingress-mode port on every node, whichever nodes run the service's
tasks. The expectations in this subsection hold with :daemon-json:`userland-proxy` true and with
:daemon-json:`userland-proxy` false, unless an expectation names a value.

.. expectation:: A published port is reachable on every node, with or without a task
    :id: SWARM-INGPUB-EVERY-NODE
    :observed_from: client
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND

    A published port is reachable on every node, including nodes that do not run a task of the
    service.

.. expectation:: An ingress published port is reachable after a firewalld reload
    :id: SWARM-INGPUB-FIREWALLD-RELOAD
    :observed_from: client
    :depends_on: os-linux
    :platforms: linux
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND

    *Applies to:* A node with firewalld running, and a service that publishes a port in ingress mode

    After a firewalld reload on the node, the published port is reachable on the node.

.. expectation:: All packets of an ingress flow reach the same task
    :id: SWARM-INGPUB-FLOW-SAME-TASK
    :observed_from: client
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND

    All packets of a flow reach the same task.

.. expectation:: A task does not see an ingress client's real source address
    :id: SWARM-INGPUB-CLIENT-SOURCE-HIDDEN
    :observed_from: task
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND

    The task does not observe the client's real source address.

.. expectation:: Tasks do not reach each other over the ingress network
    :id: SWARM-INGPUB-NO-EAST-WEST
    :observed_from: task → task
    :depends_on: os-linux, os-windows
    :platforms: linux, windows

    A task is not reachable from another task via its ingress-network address, or via its service's
    VIP on the ingress network, on any port.

.. expectation:: A task's default route is not on the ingress network
    :id: SWARM-INGPUB-NO-DEFAULT-ROUTE
    :observed_from: task
    :depends_on: os-linux, os-windows
    :platforms: linux, windows

    A task's default route is not on the ingress network.

.. expectation:: A task fails with an error when its node cannot add its ingress rules
    :id: SWARM-INGPUB-INGRESS-RULES-FAILED
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :pins: SWARM-HOST-IPTABLES-COMMAND

    *Applies to:* A Linux node that cannot add the ingress rules of a task's network namespace, and
    a service with a task on that node that publishes a port in ingress mode

    The task does not reach the ``running`` state. :api-op:`GET /tasks` reports an error for the
    task.

.. expectation:: The task's error contains failed to add ingress redirect rules
    :id: SWARM-INGPUB-INGRESS-RULES-FAILED-MSG
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :pins: SWARM-HOST-IPTABLES-COMMAND

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
    :platforms: linux, windows

    :api-op:`POST /services/create` or :api-op:`POST /services/{id}/update` for a service in
    ``dnsrr`` endpoint mode that publishes a port in ingress mode responds 400. This holds whether
    or not the port is chosen. The service is not created or not changed.

.. expectation:: docker service create of a dnsrr service with an ingress port fails
    :id: SWARM-INGPUB-DNSRR-REFUSED-CLI
    :observed_from: api:mgr
    :depends_on: cli
    :platforms: linux, windows
    :cli_for: SWARM-INGPUB-DNSRR-REFUSED-HTTP400

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
reports it within the allocation ceiling (:ref:`swarm-lifecycle-convergence`) after the request
responds.

.. expectation:: An auto-assigned port is one no other service publishes in ingress mode
    :id: SWARM-PORTALLOC-AUTO
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    :api-op:`POST /services/create` for a service with an auto-assigned port responds 201.
    :api-op:`GET /services/{id}` then reports a published port for it that no other service
    publishes in ingress mode.

.. expectation:: The auto-assignment range is 30000 to 32767 and is not configurable
    :id: SWARM-PORTALLOC-AUTO-RANGE
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    The auto-assignment range is 30000 to 32767. It is not configurable.

.. expectation:: A chosen ingress-mode port is accepted and reported in Endpoint.Ports
    :id: SWARM-PORTALLOC-CHOSEN
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    :api-op:`POST /services/create` for a service with a chosen ingress-mode port responds 201.
    :api-op:`GET /services/{id}` then reports that port in :api-field:`Endpoint.Ports`.

.. expectation:: Any chosen ingress-mode port from 1 to 65535 is accepted and reported
    :id: SWARM-PORTALLOC-CHOSEN-FULL-RANGE
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    :api-op:`POST /services/create` for a service with a chosen ingress-mode port from 1 to 65535
    responds 201. This includes a port in the auto-assignment range. :api-op:`GET /services/{id}`
    then reports that port in :api-field:`Endpoint.Ports`.

.. expectation:: A chosen ingress-mode port above 65535 is accepted but not reported
    :id: SWARM-PORTALLOC-CHOSEN-OUT-OF-RANGE
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    :api-op:`POST /services/create` for a service with a chosen ingress-mode port above 65535
    responds 201. :api-op:`GET /services/{id}` then does not report the port in
    :api-field:`Endpoint.Ports`. See :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: Auto-assignment skips a port chosen in the auto-assignment range
    :id: SWARM-PORTALLOC-AUTO-SKIPS-CHOSEN
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A Swarm with a service that publishes a chosen ingress-mode port in the
    auto-assignment range, and another service, created after it, with auto-assigned ports

    :api-op:`GET /services/{id}` for the other service does not report that port for any of its
    auto-assigned ports.

.. expectation:: Endpoint.Ports reports every ingress-mode port with its published port
    :id: SWARM-PORTALLOC-REPORTED
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    :api-op:`GET /services/{id}` reports each ingress-mode port of a service in
    :api-field:`Endpoint.Ports`, with its published port. This holds for auto-assigned and chosen
    ports.

.. expectation:: A client reaches a task at any node on the reported published port
    :id: SWARM-PORTALLOC-REPORTED-REACHABLE
    :observed_from: api:mgr, client
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND

    A client reaches a task of a service at any node's address on the published port that
    :api-op:`GET /services/{id}` reports for an ingress-mode port. This holds for auto-assigned and
    chosen ports.

.. expectation:: An update that keeps an auto-assigned port keeps its published port
    :id: SWARM-PORTALLOC-AUTO-KEPT
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A Swarm with a service that has an auto-assigned port

    :api-op:`POST /services/{id}/update` that keeps the number of ports, and keeps the name,
    protocol and target port of that port, responds 200. :api-op:`GET /services/{id}` then reports
    the same published port for it.

.. expectation:: Adding or removing another port reassigns an auto-assigned port
    :id: SWARM-PORTALLOC-AUTO-REASSIGNED
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A Swarm with a service that has an auto-assigned port

    After :api-op:`POST /services/{id}/update` adds or removes another port of the service, in
    either publish mode, the auto-assigned port's published port changes. See
    :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: Another service cannot choose a port already published in ingress mode
    :id: SWARM-PORTALLOC-CONFLICT
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A Swarm with a service that publishes a port in ingress mode

    :api-op:`POST /services/create` or :api-op:`POST /services/{id}/update` for another service that
    chooses the same port in ingress mode responds with a 4xx or 5xx status. The other service is
    not created or not changed. See :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: A conflicting ingress-mode port responds 400, already in use by service
    :id: SWARM-PORTALLOC-CONFLICT-HTTP400
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A Swarm with a service that publishes a port in ingress mode

    :api-op:`POST /services/create` or :api-op:`POST /services/{id}/update` for another service that
    chooses the same port in ingress mode responds 400, with a message that contains :api-error:`is
    already in use by service`. See :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: docker service create or update with a used ingress-mode port fails
    :id: SWARM-PORTALLOC-CONFLICT-CLI
    :observed_from: api:mgr
    :depends_on: cli
    :platforms: linux, windows
    :cli_for: SWARM-PORTALLOC-CONFLICT

    *Applies to:* A Swarm with a service that publishes a port in ingress mode

    :cli:`docker service create` or :cli:`docker service update --publish-add` that publishes the
    same port in ingress mode for another service exits non-zero and prints the daemon's error. The
    command does not create or change the other service.

.. expectation:: A port number published for TCP can be published for UDP too
    :id: SWARM-PORTALLOC-OTHER-PROTOCOL
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A Swarm with a service that publishes a TCP port

    :api-op:`POST /services/create` for another service that chooses the same port number for UDP
    responds 201. :api-op:`GET /services/{id}` then reports each service's port.

.. expectation:: A port published in one mode cannot be chosen in the other mode
    :id: SWARM-PORTALLOC-MODE-CONFLICT
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A Swarm with a service that publishes a port in one publish mode

    :api-op:`POST /services/create` or :api-op:`POST /services/{id}/update` for another service that
    chooses the same port in the other mode responds with a 4xx or 5xx status. This holds for either
    mode of the first service. The other service is not created or not changed. See
    :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: A port conflict across publish modes responds 400, already in use
    :id: SWARM-PORTALLOC-MODE-CONFLICT-HTTP400
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A Swarm with a service that publishes a port in one publish mode

    :api-op:`POST /services/create` or :api-op:`POST /services/{id}/update` for another service that
    chooses the same port in the other mode responds 400, with a message that contains
    :api-error:`is already in use by service`. This holds for either mode of the first service. See
    :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: docker service create or update fails on a port used in the other mode
    :id: SWARM-PORTALLOC-MODE-CONFLICT-CLI
    :observed_from: api:mgr
    :depends_on: cli
    :platforms: linux, windows
    :cli_for: SWARM-PORTALLOC-MODE-CONFLICT

    *Applies to:* A Swarm with a service that publishes a port in one publish mode

    :cli:`docker service create` or :cli:`docker service update --publish-add` that publishes the
    same port in the other mode for another service exits non-zero and prints the daemon's error.
    The command does not create or change the other service.

.. expectation:: A container that publishes a service's ingress port can be created
    :id: SWARM-PORTALLOC-CTR-CREATE
    :observed_from: api:mgr, api:wkr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A Swarm with a service that publishes a port in ingress mode

    On every node, :api-op:`POST /containers/create` for a container that publishes the same port
    responds 201.

.. expectation:: A container that publishes a service's ingress port does not start
    :id: SWARM-PORTALLOC-CTR-START
    :observed_from: api:mgr, api:wkr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A Swarm with a service that publishes a port in ingress mode, and a created or
    stopped container on any of its nodes that publishes the same port

    :api-op:`POST /containers/{id}/start` for the container responds with a 4xx or 5xx status.

.. expectation:: Starting a container on a service's ingress port responds 500
    :id: SWARM-PORTALLOC-CTR-START-HTTP500
    :observed_from: api:mgr, api:wkr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A Swarm with a service that publishes a port in ingress mode, and a created or
    stopped container on any of its nodes that publishes the same port

    :api-op:`POST /containers/{id}/start` for the container responds 500, with a message that
    contains :api-error:`port is already allocated`. See
    :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: docker run -p with a service's ingress port fails on every node
    :id: SWARM-PORTALLOC-CTR-START-CLI
    :observed_from: api:mgr, api:wkr
    :depends_on: cli
    :platforms: linux, windows
    :cli_for: SWARM-PORTALLOC-CTR-START

    *Applies to:* A Swarm with a service that publishes a port in ingress mode

    On every node, :cli:`docker run -d -p` with the same port exits non-zero and prints the daemon's
    error.

.. expectation:: A process or container keeps serving a port a service then publishes
    :id: SWARM-PORTALLOC-PORT-IN-USE
    :observed_from: client
    :depends_on: os-linux
    :platforms: linux

    *Applies to:* A Linux node where a port is in use by a process or container

    The process or container that uses the port keeps serving it after a service is created or
    updated to publish the port in ingress mode.

.. expectation:: A service that publishes a node's in-use port is unreachable there
    :id: SWARM-PORTALLOC-PORT-IN-USE-UNREACHABLE
    :observed_from: client
    :depends_on: os-linux
    :platforms: linux

    *Applies to:* A Linux node where a port is in use by a process or container

    Once a service is created or updated to publish that port in ingress mode, and no task from an
    earlier version of the service's spec remains, a client that connects to the node's address on
    any of the service's published ports does not reach the service's tasks. See
    :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. expectation:: Adding an in-use port leaves old tasks reachable on the earlier ports
    :id: SWARM-PORTALLOC-PORT-IN-USE-OLD-TASKS
    :observed_from: client
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND

    *Applies to:* A node where a port is in use by a process or container, and a service that
    publishes other ports in ingress mode

    :api-op:`POST /services/{id}/update` adds the port in use to the service in ingress mode. Until
    the tasks from before the update are gone, a client reaches them at the node's address on the
    service's earlier ports.

.. _swarm-traffic-api-value-mode-host:

:api-value:`mode=host`
======================

The expectations in this subsection hold with :daemon-json:`userland-proxy` true and with
:daemon-json:`userland-proxy` false, unless an expectation names a value.

.. expectation:: A mode=host port is reachable only on nodes that run a task
    :id: SWARM-HOSTPUB-TASK-NODES-ONLY
    :observed_from: client
    :depends_on: os-linux, os-windows
    :platforms: linux, windows

    A :api-value:`mode=host` published port is reachable only at the addresses of nodes that run a
    task of its service.

.. expectation:: An IPv4 client reaches the task through the port on the task's node
    :id: SWARM-HOSTPUB-REACHABLE
    :observed_from: client
    :depends_on: os-linux, os-windows
    :platforms: linux, windows

    An IPv4 client reaches the task through the published port on the task's node.

.. expectation:: A task sees an IPv4 client's real source address
    :id: SWARM-HOSTPUB-CLIENT-SOURCE
    :observed_from: task
    :depends_on: os-linux, os-windows
    :platforms: linux, windows

    A task observes the real source address of an IPv4 client that connects through the published
    port on the task's node.

.. expectation:: With userland-proxy, a task does not see an IPv6 client's address
    :id: SWARM-HOSTPUB-CLIENT-SOURCE-V6
    :observed_from: task
    :depends_on: none
    :platforms: linux
    :needs: SWARM-HOST-IPV6

    *Applies to:* A daemon with :daemon-json:`userland-proxy` true, and a task whose gateway network
    is :network:`docker_gwbridge`

    The task does not observe an IPv6 client's real source address.

.. expectation:: A connection to a node's mode=host port goes to that node's task
    :id: SWARM-HOSTPUB-LOCAL-TASK
    :observed_from: client
    :depends_on: os-linux, os-windows
    :platforms: linux, windows

    A connection to a published port on a node is delivered to the task on that node that published
    the port.

.. expectation:: A dnsrr service with a mode=host port is created and reports the port
    :id: SWARM-HOSTPUB-DNSRR
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    :api-op:`POST /services/create` for a service in ``dnsrr`` endpoint mode that publishes a port
    in :api-value:`mode=host` responds 201. :api-op:`GET /tasks/{id}` for a running task of the
    service then reports the port in :api-field:`Status.PortStatus`.

.. expectation:: A task reports its assigned mode=host port in Status.PortStatus
    :id: SWARM-HOSTPUB-ASSIGNED-PORT
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A running task of a service with a :api-value:`mode=host` port that does not set
    :api-field:`PublishedPort`

    :api-op:`GET /tasks/{id}` for that task reports its assigned host port in
    :api-field:`Status.PortStatus`.

.. expectation:: The task's container reports the same assigned host port
    :id: SWARM-HOSTPUB-ASSIGNED-PORT-CTR
    :observed_from: api:mgr, api:wkr (on the task's node)
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A running task of a service with a :api-value:`mode=host` port that does not set
    :api-field:`PublishedPort`

    On that task's node, :api-op:`GET /containers/json` reports in the :api-field:`Ports` of the
    task's container the host port that :api-op:`GET /tasks/{id}` reports in
    :api-field:`Status.PortStatus`.

.. expectation:: docker service ps prints a task's assigned host port
    :id: SWARM-HOSTPUB-ASSIGNED-PORT-CLI
    :observed_from: api:mgr
    :depends_on: cli
    :platforms: linux, windows
    :cli_for: SWARM-HOSTPUB-ASSIGNED-PORT

    *Applies to:* A running task of a service with a :api-value:`mode=host` port that does not set
    :api-field:`PublishedPort`

    :cli:`docker service ps` on a manager prints the task's assigned host port.

.. expectation:: docker container ls prints a task's assigned host port
    :id: SWARM-HOSTPUB-ASSIGNED-PORT-CTR-CLI
    :observed_from: api:mgr, api:wkr (on the task's node)
    :depends_on: cli
    :platforms: linux, windows
    :cli_for: SWARM-HOSTPUB-ASSIGNED-PORT-CTR

    *Applies to:* A running task of a service with a :api-value:`mode=host` port that does not set
    :api-field:`PublishedPort`

    :cli:`docker container ls` on the task's node prints the task's assigned host port.

.. expectation:: Two services can publish the same mode=host port
    :id: SWARM-HOSTPUB-SHARED-PORT
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A Swarm with a service that publishes a port in :api-value:`mode=host`

    :api-op:`POST /services/create` for another service that publishes the same port in
    :api-value:`mode=host` responds 201. :api-op:`GET /services` then lists both services.

.. expectation:: Tasks with the same chosen mode=host port never share a node
    :id: SWARM-HOSTPUB-PLACEMENT
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    Tasks that publish the same chosen :api-value:`mode=host` port
    (:ref:`swarm-traffic-port-allocation`) are never placed on the same node, whether they belong to
    one service or several.

.. expectation:: A task stays pending when every node already uses its mode=host port
    :id: SWARM-HOSTPUB-PLACEMENT-PENDING
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A Swarm in which each node runs a task that publishes a chosen
    :api-value:`mode=host` port (:ref:`swarm-traffic-port-allocation`)

    A new task that publishes the same port in :api-value:`mode=host` stays in the ``pending`` state
    that :api-op:`GET /tasks/{id}` reports in :api-field:`Status.State`.

.. expectation:: A mode=host target port above 65535 runs tasks but reports no port
    :id: SWARM-HOSTPUB-TARGET-OUT-OF-RANGE
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    :api-op:`POST /services/create` for a service that publishes a :api-value:`mode=host` port with
    a :api-field:`TargetPort` above 65535 responds 201. The service's tasks run, and :api-op:`GET
    /tasks/{id}` reports no port in :api-field:`Status.PortStatus`. See
    :ref:`swarm-lifecycle-known-divergences-non-goals`.

.. _swarm-traffic-egress-the-gateway-network:

******************************
 Egress & the gateway network
******************************

.. expectation:: A container on a non-internal overlay reaches the physical network
    :id: SWARM-EGRESS-NON-INTERNAL
    :observed_from: task or ctr → underlay
    :depends_on: os-linux, os-windows
    :platforms: linux, windows

    A container on a non-internal overlay has egress to the physical network.

.. expectation:: A container's egress traffic has the node's address as its source
    :id: SWARM-EGRESS-SOURCE-ADDRESS
    :observed_from: underlay
    :depends_on: os-linux, os-windows
    :platforms: linux, windows

    *Applies to:* A container attached only to overlay networks, at least one of them not
    ``internal``

    For the container's egress traffic, the source address that an underlay host observes is the
    node's address, not the container's.

.. expectation:: A container on only internal overlays has no egress
    :id: SWARM-EGRESS-INTERNAL-ONLY
    :observed_from: task, ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows

    A container attached only to ``internal`` overlays does not have egress.

.. expectation:: A container on an internal and a non-internal overlay has egress
    :id: SWARM-EGRESS-MIXED-INTERNAL
    :observed_from: task, ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows

    A container on both an internal and a non-internal overlay has egress. Its default route is not
    on the internal overlay.

.. expectation:: A task of a service with no network is on the default bridge only
    :id: SWARM-EGRESS-NO-NETWORK-DEFAULT-BRIDGE
    :observed_from: api:mgr, api:wkr (on the task's node)
    :depends_on: none
    :platforms: linux

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
    :within: SWARM-CEIL-REJOIN

    Within the :need:`[[title|lower]] ceiling ([[seconds]] s) <SWARM-CEIL-REJOIN>` after
    :api-op:`POST /swarm/init` responds 200 on a node, :api-op:`GET /networks` on the node lists a
    gateway bridge network.

.. expectation:: A gateway bridge network exists within the ceiling after a join
    :id: SWARM-GWNET-AFTER-JOIN
    :observed_from: api:mgr, api:wkr
    :depends_on: none
    :platforms: linux
    :within: SWARM-CEIL-REJOIN

    *Applies to:* A Swarm with an ingress network

    Within the :need:`[[title|lower]] ceiling ([[seconds]] s) <SWARM-CEIL-REJOIN>` after
    :api-op:`POST /swarm/join` responds 200 on a node, :api-op:`GET /networks` on that node lists a
    gateway bridge network.

.. expectation:: Without an ingress network, a joined node has no gateway network yet
    :id: SWARM-GWNET-NO-INGRESS
    :observed_from: api:mgr, api:wkr
    :depends_on: none
    :platforms: linux

    *Applies to:* A node that joined a Swarm without an ingress network, and has not yet started a
    container on a non-internal overlay

    :api-op:`GET /networks` on the node does not list a gateway bridge network.

.. expectation:: Without ingress, the first overlay container adds a gateway network
    :id: SWARM-GWNET-NO-INGRESS-FIRST-CTR
    :observed_from: api:active
    :depends_on: none
    :platforms: linux

    *Applies to:* A node that joined a Swarm without an ingress network

    Once a container on a non-internal overlay has started on the node, :api-op:`GET /networks` on
    the node lists a gateway bridge network.

.. expectation:: A node's gateway bridge network is named docker_gwbridge
    :id: SWARM-GWNET-NAME
    :observed_from: api:mgr, api:wkr
    :depends_on: none
    :platforms: linux

    A node's gateway bridge network has the name :network:`docker_gwbridge`.

.. expectation:: The gateway network's bridge interface is named docker_gwbridge
    :id: SWARM-GWNET-BRIDGE-IFACE-NAME
    :observed_from: node
    :depends_on: none
    :platforms: linux

    The bridge interface of a node's gateway bridge network has the name :network:`docker_gwbridge`.

.. expectation:: An operator-created docker_gwbridge keeps its subnet and options
    :id: SWARM-GWNET-OPERATOR-CREATED
    :observed_from: api:active
    :depends_on: none
    :platforms: linux

    *Applies to:* A node where the operator created :network:`docker_gwbridge` before its first
    container on a non-internal overlay, and before it joined a Swarm with an ingress network

    After containers on non-internal overlays start on that node, :api-op:`GET
    /networks/docker_gwbridge` there reports the operator's subnet and options.

.. expectation:: Containers get addresses in an operator-created docker_gwbridge subnet
    :id: SWARM-GWNET-OPERATOR-CREATED-ADDRESSES
    :observed_from: task, ctr
    :depends_on: os-linux
    :platforms: linux

    *Applies to:* A node where the operator created :network:`docker_gwbridge` before its first
    container on a non-internal overlay, and before it joined a Swarm with an ingress network

    The containers on non-internal overlays that then start on that node have gateway-network
    addresses in the operator's subnet.

.. expectation:: A daemon-created gateway network reports enable_icc as false
    :id: SWARM-GWNET-NO-ICC
    :observed_from: api:mgr, api:wkr
    :depends_on: none
    :platforms: linux

    *Applies to:* A gateway network that the operator did not create

    :api-op:`GET /networks/{id}` for that network reports
    :drvopt:`com.docker.network.bridge.enable_icc` as ``false`` in :api-field:`Options`.

.. expectation:: A daemon-created gateway network gives containers no IPv6 address
    :id: SWARM-GWNET-NO-V6-ADDRESS
    :observed_from: task, ctr
    :depends_on: os-linux
    :platforms: linux

    *Applies to:* A gateway network that the operator did not create

    A container's interface on that network does not have an IPv6 address.

.. expectation:: An underlay host does not reach unpublished ports at a gateway address
    :id: SWARM-GWNET-UNPUBLISHED-PORT
    :observed_from: underlay → task or ctr
    :depends_on: os-linux
    :platforms: linux

    *Applies to:* A container with an address on a node's gateway network, and an underlay host with
    a route to that address via the node's underlay address

    Traffic from that host to that address does not reach the container on a port that the container
    does not publish.

.. _swarm-traffic-known-divergences-non-goals:

*******************************
 Known divergences & non-goals
*******************************

.. expectation:: Encrypted overlay on Windows
    :id: SWARM-CTR-WINDOWS-ENCRYPTION
    :observed_from: underlay
    :depends_on: none
    :platforms: windows

    Not implemented. Traffic between Windows nodes on an :drvopt:`encrypted` overlay is not
    encrypted. Traffic between Windows nodes and Linux nodes on such an overlay does not pass
    because the Linux peers drop cleartext VXLAN for the network's VNI. A Windows node's endpoint
    MTU on an :drvopt:`encrypted` network is the same as on a cleartext network
    (:ref:`swarm-lifecycle-encryption`, :ref:`swarm-lifecycle-encrypted-overlay-networks`).

.. expectation:: mtu option on Windows
    :id: SWARM-CTR-WINDOWS-MTU-OPTION
    :observed_from: task
    :depends_on: none
    :platforms: windows

    Ignored. On a network with :drvopt:`com.docker.network.driver.mtu` set, Windows nodes do not
    report the inner MTU that Linux nodes report. Traffic between Windows nodes and Linux nodes on
    that network can be blackholed (:ref:`swarm-traffic-observable-state-inside-a-container`).

.. expectation:: Egress with both internal and non-internal overlays on Windows
    :id: SWARM-EGRESS-WINDOWS-MIXED-INTERNAL
    :observed_from: task
    :depends_on: none
    :platforms: windows

    Not guaranteed to leave through the non-internal network. A Windows container attached to an
    ``internal`` overlay and to a non-internal overlay has a gateway on each network. Its egress can
    leave through either network (:ref:`swarm-traffic-egress-the-gateway-network`).

.. expectation:: Connections from a task to its own service's VIP in DSR mode
    :id: SWARM-LBHEALTH-DSR-OWN-VIP
    :observed_from: task
    :depends_on: none
    :platforms: linux, windows

    Not load-balanced. On Linux nodes, in DSR mode, each task of a service has the service's VIP as
    a local address. A connection from a task to the VIP of its own service reaches that task. The
    other tasks of the service do not get these connections. The task gets them also while it does
    not yet report healthy, and while it shuts down. Connections from other clients are
    load-balanced (:ref:`swarm-traffic-load-balancing-east-west-vip`).

.. expectation:: VIP on an overlay network with more than one subnet
    :id: SWARM-EW-VIP-MULTIPLE-SUBNETS
    :observed_from: task
    :depends_on: none
    :platforms: linux, windows

    Not load-balanced across subnets. On Linux nodes, in NAT mode and in DSR mode, a backend task
    outside the subnet of the client node's load-balancer endpoint does not answer traffic to the
    VIP. A VIP outside that subnet does not reach any backend. The tasks' own addresses stay
    reachable across subnets (:ref:`swarm-traffic-load-balancing-east-west-vip`).

.. expectation:: dsr option on Windows
    :id: SWARM-LB-WINDOWS-DSR
    :observed_from: task
    :depends_on: none
    :platforms: windows

    Ignored. Windows nodes load-balance VIP traffic in NAT mode, whatever the network's
    :drvopt:`dsr` option says. A task on a Windows node does not observe the client's real source
    address, and does not observe the VIP (:ref:`swarm-traffic-load-balancing-east-west-vip`).

.. expectation:: Isolation between tasks on the ingress network on Windows
    :id: SWARM-INGPUB-WINDOWS-INGRESS-ISOLATION
    :observed_from: task
    :depends_on: none
    :platforms: windows

    Not enforced. Another task can reach a task on a Windows node through the ingress-network
    address of the task on the Windows node (:ref:`swarm-traffic-api-value-mode-ingress`).

.. expectation:: Egress through the ingress link on Windows
    :id: SWARM-INGPUB-WINDOWS-INGRESS-EGRESS
    :observed_from: task
    :depends_on: none
    :platforms: windows

    Not prevented. A task on a Windows node can reach the outside world through its ingress-network
    link (:ref:`swarm-traffic-api-value-mode-ingress`).

.. expectation:: Resolver network preference on Windows
    :id: SWARM-SD-WINDOWS-RESOLVER-PREFERENCE
    :observed_from: ctr
    :depends_on: os-windows
    :platforms: windows

    Not applied. Each network of a Windows container has its own embedded resolver. That resolver
    answers only from the records of its network. The network whose answer the container gets
    depends on which adapter's resolver the Windows DNS client asks
    (:ref:`swarm-traffic-service-discovery`).

.. expectation:: Ingress ports at the node's IPv6 addresses through docker-proxy, with firewalld running
    :id: SWARM-PUB-FIREWALLD-V6-INGRESS
    :observed_from: client
    :depends_on: os-linux
    :platforms: linux

    Refused. With :daemon-json:`userland-proxy` true, an ingress-mode port is also reachable at the
    node's IPv6 addresses through docker-proxy (:ref:`README.md <README-observed-from>`). With
    firewalld running and its default zone in effect for the interface, a connection to the port at
    an IPv6 address of the node does not answer, before and after a firewalld reload. The daemon
    does not open the port in firewalld. The port answers at the node's IPv4 addresses
    (:ref:`swarm-traffic-api-value-mode-ingress`).

.. expectation:: Ingress ports already in use on Linux
    :id: SWARM-PUB-INGRESS-PORT-IN-USE
    :observed_from: client
    :depends_on: os-linux, os-windows
    :platforms: linux, windows

    Not published on that node. On a Linux node where a process or container uses a port when a
    service is created or updated to publish it in ingress mode, the node does not publish any of
    the service's ports in ingress mode. The process or container keeps serving the port. A client
    that connects to the node's address on any of the service's published ports does not reach the
    service's tasks. The daemon log reports the failure, and the node tries again on each of the
    service's later backend events (:ref:`swarm-traffic-port-allocation`,
    :ref:`swarm-lifecycle-failure-reporting`).

.. expectation:: Ingress port reservation on Windows
    :id: SWARM-PORTALLOC-WINDOWS-INGRESS-PORT-RESERVATION
    :observed_from: api:wkr
    :depends_on: none
    :platforms: windows

    Not reserved. On Windows nodes, an ingress port is an HNS load-balancer policy, and its host
    port is not reserved. A :cli:`docker run -p` on a port that a service publishes is not refused.
    On a Windows node, a port is also published in ingress mode when it is already in use
    (:ref:`swarm-traffic-port-allocation`).

.. expectation:: Rootless mode
    :id: SWARM-LB-ROOTLESS
    :observed_from: api:solo
    :depends_on: os-linux, os-windows
    :platforms: linux, windows

    Not supported, and nothing refuses it. :cli:`docker swarm init` and :cli:`docker network create
    --driver overlay` succeed. A service in the ``vip`` endpoint mode on a user-defined overlay
    network reports its tasks as running, but its VIP refuses connections. The daemon cannot program
    IPVS from a user namespace. A service that publishes a port through the routing mesh reports its
    task as running, but the port does not answer. A node cannot join a Swarm whose manager runs
    rootless. The node's connection to the manager is refused.

.. expectation:: Published-port conflicts accepted in a race
    :id: SWARM-PORTALLOC-CONFLICT-RACE
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    Because of a race condition, two or more services with colliding published ports may be accepted
    in error (:ref:`swarm-traffic-port-allocation`).

.. expectation:: Auto-assigned published ports across an update
    :id: SWARM-PORTALLOC-AUTO-UPDATE-REASSIGNED
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows
    :rationale: RAT-SWARM-PORTALLOC-AUTO-UPDATE-REASSIGNED

    Reassigned. An update that adds or removes a published port, in either publish mode, reassigns
    each of the service's other auto-assigned ports. Clients then must discover the ports' numbers
    again (:ref:`swarm-traffic-port-allocation`).

.. expectation:: Auto-assigned ports and mode=host ports
    :id: SWARM-PUB-AUTO-ASSIGNED-PORT-HOST-MODE
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    Not kept apart. An auto-assigned port can be a port that another service publishes in
    :api-value:`mode=host`. :api-op:`POST /services/create` then responds 201. The refusal of a port
    that both modes publish applies only to a chosen port. Take a node where a service's
    auto-assigned port is a port that another service publishes there in :api-value:`mode=host`. On
    that node, one of two things occurs. Either the routing mesh does not serve the service's ports,
    or the :api-value:`mode=host` task fails to start. Which one occurs depends on which of them
    binds first (:ref:`swarm-traffic-port-allocation`).

.. expectation:: Ports above 65535
    :id: SWARM-PORTALLOC-PORT-ABOVE-65535
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    Accepted. :api-op:`POST /services/create` with an ingress-mode :api-field:`PublishedPort` above
    65535 responds 201. The service's tasks stay in the ``new`` state that :api-op:`GET /tasks`
    reports. With a :api-value:`mode=host` :api-field:`PublishedPort` above 65535, the request also
    responds 201, and each of the service's tasks is rejected with :api-error:`invalid port
    specification`. In :api-value:`mode=host`, a :api-field:`TargetPort` above 65535 is also
    accepted. The service's tasks run, but they do not publish the port
    (:ref:`swarm-traffic-port-allocation`, :ref:`swarm-traffic-api-value-mode-host`).

.. expectation:: Recovery from a node-local failure is incomplete
    :id: SWARM-LBDIST-RECOVERY-INCOMPLETE
    :observed_from: task
    :depends_on: none
    :platforms: linux, windows

    On Linux nodes, the next attempt programs only the backend that triggered it. A backend whose
    programming failed does not get traffic through the node's load balancer until its task is
    replaced. This includes every backend that arrived while the failure continued. Take a failure
    that occurs after the node publishes the service's ingress ports, and a next attempt that
    follows. If the service is then deleted or stops publishing those ports, the host ports stay
    published until the daemon restarts (:ref:`swarm-traffic-published-ports`,
    :ref:`swarm-lifecycle-failure-reporting`).

.. expectation:: ESP sequence numbers on busy node pairs
    :id: SWARM-EW-ESP-SEQUENCE-EXHAUSTION
    :observed_from: task
    :depends_on: none
    :platforms: linux, windows

    Exhausted before rekeying. An IPsec security association between two nodes has a 32-bit sequence
    number. The network encryption keys rotate every 12 hours, and the interval starts again when
    the leader manager changes. When a node sends 2³² packets to another node on one security
    association within an interval, the sequence number is exhausted. Traffic between the two nodes
    on :drvopt:`encrypted` networks then fails without an error that the operator can see. At about
    1000 bytes per packet, this limits a node pair to about 100 MB/s on average in each direction.
    See `moby/moby#52005 <https://github.com/moby/moby/issues/52005>`__.

.. expectation:: Unknown Protocol and PublishMode values
    :id: SWARM-PUB-UNKNOWN-PROTOCOL-MODE
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    Accepted. :api-op:`POST /services/create` with a published port whose :api-field:`Protocol` or
    :api-field:`PublishMode` the API reference does not list responds 201. Such a port is stored as
    ``tcp`` in ingress mode (:ref:`swarm-traffic-published-ports`).
