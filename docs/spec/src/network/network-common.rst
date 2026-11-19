################################
 Common networking surface area
################################

Scope: the behavior that every network driver must have, and the Engine API requests for networks
and endpoints that do not depend on the network's driver. This spec describes them as observable
from outside the daemon.

This spec holds expectations that every network driver shares. It is not yet a complete account.
:doc:`../README.md <../README>` gives the model that this spec follows: the items of a spec, their
categories and what their verdicts depend on. :doc:`README.md <README>` gives the conventions of the
networking specs, such as vantage points, daemon settings, timing and convergence ceilings, and host
requirements. :doc:`swarm-networking.md <swarm-networking>` and :doc:`bridge-networking.md
<bridge-networking>` cover what is specific to their drivers. :doc:`ipvlan-networking.md
<ipvlan-networking>` covers the ipvlan driver. The macvlan driver has no spec of its own, and its
expectations are here. :doc:`container-dns.md <container-dns>` covers name resolution.

In this spec, a predefined network is a network that :api-op:`GET /networks` with the filter
:api-query:`type=builtin` lists. On a manager, the Swarm also has two Swarm-scoped networks named
:network:`bridge` and :network:`host`, whose :api-field:`Labels` include
``com.docker.swarm.predefined``: the Swarm's predefined networks. A service that names
:network:`bridge` or :network:`host` refers to one of them, and its tasks use the node's own network
of that name. The filter :api-query:`type=builtin` does not list them, so they are not predefined
networks in the sense of this spec. :api-op:`GET /networks` does not list them at all
(:need:`SWARM-SCOPED-PREDEFINED-NOT-LISTED`). A lookup by name without :api-query:`scope` ``swarm``
does not match them (:need:`NET-API-INSPECT-NAME-SWARM-PREDEFINED`), so an expectation that says
that a name matches more than one network does not count them. The expectations here also hold for
Swarm-scoped networks and on nodes in a Swarm, unless an expectation's own text says otherwise.
:ref:`swarm-networking-network-lifecycle-api-surface` in :doc:`swarm-networking.md
<swarm-networking>` covers the requests that are specific to Swarm-scoped networks.

.. _network-common-requirements:

**************
 Requirements
**************

.. req:: Code written against an API version keeps working on later daemons
    :id: REQ-NET-API-BACKWARD-COMPATIBLE
    :source_kind: docs
    :source: https://docs.docker.com/reference/api/engine/#versioned-api-and-sdk, https://docs.docker.com/reference/api/engine/#minimum-api-version
    :refines: PRIN-STABLE

    Code that uses the Engine API keeps working against later daemons without changes. Requests and
    responses are adjusted to the API version that the client uses.

.. req:: A refused request responds with a 4xx status
    :id: REQ-NET-API-ERROR-STATUS
    :source_kind: docs
    :source: api/openapi.yaml: operation NetworkConnect, api/openapi.yaml: operation NetworkDisconnect, api/openapi.yaml: operation ContainerStart, api/openapi.yaml: operation SwarmInit, https://docs.docker.com/reference/api/engine/version-history/#v131-api-changes, https://docs.docker.com/engine/release-notes/prior-releases/#1120-2016-07-28, https://docs.docker.com/engine/release-notes/18.09/#18091, https://docs.docker.com/engine/release-notes/19.03/#19033, https://docs.docker.com/engine/release-notes/25.0/#2500, https://docs.docker.com/engine/release-notes/25.0/#2501, https://docs.docker.com/engine/release-notes/28/#2800, api/openapi.yaml: operation NetworkCreate

    A request that the daemon refuses, because of its parameters or the state of the objects that it
    names, responds with a 4xx status. The exception is a refusal that another requirement promises
    status 501 for, such as an ingress network with a driver other than overlay.

.. req:: A ConfigFrom network takes its configuration from a config-only network
    :id: REQ-NET-CONFIG-FROM
    :source_kind: api
    :source: https://docs.docker.com/reference/api/engine/version/v1.54/#tag/Network/operation/NetworkCreate, https://docs.docker.com/reference/cli/docker/network/create/#swarm-networks-with-local-scope-drivers

    ConfigFrom names the source of a network's configuration, which must be an existing config-only
    network. The network then uses that network's configuration, such as its subnets and gateway.

.. req:: A config-only network holds configuration and cannot run containers
    :id: REQ-NET-CONFIG-ONLY
    :source_kind: api
    :source: https://docs.docker.com/reference/api/engine/version/v1.54/#tag/Network/operation/NetworkCreate, https://docs.docker.com/reference/cli/docker/network/create/#swarm-networks-with-local-scope-drivers

    A network created with ConfigOnly is a placeholder whose configuration other networks use. It
    cannot be used directly to run containers or services.

.. req:: A config-only network that another network uses cannot be removed
    :id: REQ-NET-CONFIG-ONLY-DELETE-IN-USE
    :source_kind: issue
    :source: https://github.com/moby/libnetwork/pull/1742: the commit that adds configuration networks

    A config-only network can be removed once no other network uses it, and not before.

.. req:: A failed connect or start leaves the container's networks as they were
    :id: REQ-NET-CONNECT-FAILED-NO-TRACE
    :source_kind: issue
    :source: https://github.com/moby/moby/pull/52480: a merged fix and its stated intent

    A failed connect drops the NetworkSettings.Networks entry that it added. A start that fails at
    network setup keeps the container's networks from container create, and the next start fails too
    while the cause of the failure holds. The container list then reports no endpoint from the
    failed attempt.

.. req:: A network cannot be connected again to a running container
    :id: REQ-NET-CONNECT-ONCE
    :source_kind: api
    :source: https://docs.docker.com/reference/api/engine/version/v1.54/#tag/Network/operation/NetworkConnect

    POST /networks/{id}/connect cannot re-attach a network to a running container that is already
    attached to it.

.. req:: A repeat connect of a stopped container succeeds
    :id: REQ-NET-CONNECT-REPEAT-STOPPED
    :source_kind: observable
    :source: Scripts and unit files that connect a stopped container to its networks before each start (such as a systemd ExecStartPre that runs docker network connect on every boot). The API reference refuses a repeat connect only for a running container

    POST /networks/{id}/connect for a created or stopped container that already has the network
    succeeds, and the container keeps the network.

.. req:: Container create accepts several networks from API 1.44
    :id: REQ-NET-CREATE-MULTIPLE-NETWORKS
    :source_kind: api
    :source: https://docs.docker.com/reference/api/engine/version-history/#v144-api-changes

    From API version 1.44, POST /containers/create accepts more than one network in
    NetworkingConfig.EndpointsConfig; earlier API versions accept one.

.. req:: A daemon.json key sets the same option as the dockerd flag of its name
    :id: REQ-NET-DAEMON-JSON-KEYS
    :source_kind: docs
    :source: https://docs.docker.com/reference/cli/dockerd/#daemon-configuration-file

    A daemon.json key that is the name of a dockerd flag, or the plural of the name of a flag that
    allows several entries, sets the same option as that flag, and a value means the same in the
    file as on the command line. Keys that are not named after a flag are not covered.

.. req:: A container run without a network is connected to the default bridge
    :id: REQ-NET-DEFAULT-BRIDGE-ATTACHMENT
    :source_kind: docs
    :source: https://docs.docker.com/engine/network/, https://docs.docker.com/engine/network/drivers/bridge/#differences-between-user-defined-bridges-and-the-default-bridge, https://docs.docker.com/engine/network/drivers/bridge/#connect-a-container-to-the-default-bridge-network
    :violates: PRIN-LEAST-ACCESS

    On Linux, a container that is run without a network is connected to the default bridge network.
    Containers on the default bridge can communicate with each other by IP address, and have access
    to network services outside the host.

.. req:: A network with connected containers cannot be removed
    :id: REQ-NET-DELETE-IN-USE
    :source_kind: docs
    :source: https://docs.docker.com/reference/cli/docker/network/rm/, https://docs.docker.com/reference/cli/docker/network/ls/#filter

    To remove a network, its containers must first be disconnected from it; removing a network that
    has containers attached fails.

.. req:: A container can be disconnected, and connected or disconnected while stopped
    :id: REQ-NET-DISCONNECT
    :source_kind: docs
    :source: https://docs.docker.com/reference/cli/docker/network/disconnect/, https://docs.docker.com/engine/release-notes/prior-releases/#1100-2016-02-04

    docker network disconnect removes a container from a network. Connect and disconnect also work
    on stopped containers.

.. req:: A disconnect from a network the container is not on fails
    :id: REQ-NET-DISCONNECT-NOT-ATTACHED
    :source_kind: issue
    :source: https://github.com/moby/moby/issues/44151: a report that maintainers triaged and that expects the error

    A request to disconnect a container from a network that it is not attached to fails with an
    error.

.. req:: An endpoint MacAddress sets the container's MAC address on the network
    :id: REQ-NET-ENDPOINT-MAC-ADDRESS
    :source_kind: api
    :source: https://docs.docker.com/reference/api/engine/version/v1.54/#tag/Network/operation/NetworkConnect, https://docs.docker.com/reference/api/engine/version-history/#v144-api-changes, https://docs.docker.com/reference/api/engine/version-history/#v154-api-changes, https://docs.docker.com/reference/cli/docker/container/run/#network

    EndpointSettings.MacAddress sets the endpoint's MAC address on container create from API 1.44,
    and on network connect from API 1.54. At earlier API versions, each of these requests ignores
    the field.

.. req:: The endpoint with the highest gw-priority gives the default gateway
    :id: REQ-NET-GW-PRIORITY
    :source_kind: docs
    :source: https://docs.docker.com/engine/network/#connecting-to-multiple-networks, https://docs.docker.com/reference/api/engine/version-history/#v148-api-changes, api/openapi.yaml: definition EndpointSettings (GwPriority)

    The network whose endpoint has the highest gw-priority (default 0) provides the container's
    default gateway, on create and on connect. Container create and network connect accept
    GwPriority from API 1.48. Among endpoints with the same priority, one with both an IPv4 and an
    IPv6 gateway comes before one with a gateway for a single address family, and remaining ties go
    to the network name that sorts first. Each address family takes its default gateway from the
    first endpoint in that order that has one.

.. req:: ExtraHosts entries are added to the container's hosts file
    :id: REQ-NET-EXTRA-HOSTS
    :source_kind: api
    :source: api/openapi.yaml: definition HostConfig (ExtraHosts), https://docs.docker.com/reference/cli/docker/container/run/#add-host

    HostConfig.ExtraHosts lists hostname to IP address mappings, in the form hostname:IP, to add to
    the container's /etc/hosts file. docker run --add-host adds such an entry.

.. req:: host-gateway resolves to the host-gateway addresses
    :id: REQ-NET-HOST-GATEWAY
    :source_kind: docs
    :source: https://docs.docker.com/reference/cli/dockerd/#configure-host-gateway-ip, https://docs.docker.com/reference/cli/docker/container/run/#add-host, https://docs.docker.com/engine/release-notes/28/#2800

    An --add-host value of host-gateway resolves to addresses of the host: by default the default
    bridge's IPv4 address and its IPv6 address if it has one, or the addresses set with
    host-gateway-ips.

.. req:: A dotted hostname gives full and short names in /etc/hosts
    :id: REQ-NET-HOSTS-FQDN
    :source_kind: issue
    :source: https://github.com/moby/moby/issues/29100: a report of the missing short name that maintainers agreed was a bug and fixed

    When a container's hostname is a fully qualified name, its /etc/hosts line has both the full
    name and the unqualified name.

.. req:: A container's /etc/hosts maps its addresses to its hostname
    :id: REQ-NET-HOSTS-OWN-NAME
    :source_kind: docs
    :source: https://docs.docker.com/engine/network/#custom-hosts

    A container's /etc/hosts has lines that define the container's own hostname, besides localhost
    and other common entries.

.. req:: A service's Hosts entries map their names and aliases in its containers
    :id: REQ-NET-HOSTS-SERVICE-ENTRIES
    :source_kind: api
    :source: api/openapi.yaml: definition TaskSpec (ContainerSpec.Hosts)

    ContainerSpec.Hosts lists hostname to IP address mappings to add to the hosts file of a
    service's containers. Each entry has the form of a line of hosts(5): an IP address, a canonical
    hostname, and optional aliases.

.. req:: An endpoint's ifname option names its interface in the container
    :id: REQ-NET-IFACE-NAME
    :source_kind: docs
    :source: https://docs.docker.com/engine/release-notes/28/#2800

    The endpoint option com.docker.network.endpoint.ifname sets the name of the container's
    interface, for every built-in driver on Linux.

.. req:: Network inspect reports the network's configuration as created
    :id: REQ-NET-INSPECT-CONFIGURATION
    :source_kind: api
    :source: api/openapi.yaml: definition Network (Created/Scope/Driver/EnableIPv6/IPAM/Internal/Attachable/Ingress/ConfigFrom/Labels), api/openapi.yaml: operation NetworkInspect

    GET /networks/{id} reports a network's scope, when it was created, its IPAM configuration, the
    name of the driver used to create it, and the EnableIPv6, Internal, Attachable, Ingress,
    ConfigFrom and Labels that it was created with.

.. req:: An interface without a requested name is named eth<N>
    :id: REQ-NET-IFACE-NAME-DEFAULT
    :source_kind: observable
    :source: Images and scripts that name eth0 inside the container in ip or tc or iptables commands or in the configuration of keepalived or Zeek or in an application's --interface flag. The bridge driver documentation gives com.docker.network.container_iface_prefix the default eth (https://docs.docker.com/engine/network/drivers/bridge/#options)

    On Linux, the interface of an endpoint on a network of a built-in driver, without a requested
    name and without com.docker.network.container_iface_prefix, is named eth<N>. The interface of a
    container's only endpoint is named eth0.

.. req:: Containers cannot reach other networks through an internal network
    :id: REQ-NET-INTERNAL-NO-EXTERNAL-ACCESS
    :source_kind: docs
    :source: https://docs.docker.com/reference/cli/docker/network/create/#internal
    :refines: PRIN-LEAST-ACCESS

    docker network create --internal restricts external access to the network. A container cannot
    reach other networks through an internal network.

.. req:: A network can be created without IPv4, as an IPv6-only network
    :id: REQ-NET-IPV6-ONLY
    :source_kind: docs
    :source: https://docs.docker.com/engine/network/, https://docs.docker.com/engine/network/drivers/bridge/#ipv6-only-bridge-networks

    IPv4 address allocation can be disabled when a network is created (--ipv4=false, EnableIPv4
    false), giving an IPv6-only network.

.. req:: A container's requested link-local addresses must be link-local
    :id: REQ-NET-LINK-LOCAL-ADDRESS
    :source_kind: docs
    :source: https://docs.docker.com/reference/cli/docker/container/run/#network, https://docs.docker.com/reference/cli/docker/network/connect/, api/openapi.yaml: definition EndpointIPAMConfig (LinkLocalIPs), https://docs.docker.com/engine/release-notes/29/#2991, https://github.com/moby/moby/pull/53923

    --link-local-ip on docker run, docker create and docker network connect, which sets LinkLocalIPs
    in the endpoint's IPAMConfig, gives the container IPv4 or IPv6 link-local addresses on the
    network. Container create and network connect refuse an address that is not in 169.254.0.0/16 or
    fe80::/10, or that has a zone, before they change the container.

.. req:: An invalid network list filter responds 400
    :id: REQ-NET-LIST-FILTER-INVALID
    :source_kind: issue
    :source: https://github.com/moby/moby/issues/53676: a report of a 500 for such a filter, https://github.com/moby/moby/pull/53678: the merged fix that makes it respond 400

    A GET /networks filter that is not well formed, such as a type value other than builtin or
    custom, is a client error, and the request responds 400.

.. req:: GET /networks filters by dangling and type with the documented values
    :id: REQ-NET-LIST-FILTERS
    :source_kind: api
    :source: https://docs.docker.com/reference/api/engine/version/v1.54/#tag/Network/operation/NetworkList, https://docs.docker.com/reference/cli/docker/network/ls/#filter, https://docs.docker.com/reference/api/engine/version-history/#v140-api-changes, api/openapi.yaml: operation NetworkList, https://github.com/moby/moby/pull/31551: the merged change whose commit message says that dangling leaves out the builtin networks

    The network list accepts the filters dangling and type. dangling true or 1 lists the networks
    that are not predefined and that no container uses; false or 0 lists the predefined networks and
    the networks that containers use. type builtin lists the predefined networks and custom the
    user-defined ones. Several values of type are combined with OR.

.. req:: With live-restore, containers keep running while the daemon is down
    :id: REQ-NET-LIVE-RESTORE
    :source_kind: docs
    :source: https://docs.docker.com/engine/daemon/live-restore/

    With live-restore enabled, containers remain running when the daemon terminates or restarts, so
    that daemon crashes, outages and upgrades do not interrupt them.

.. req:: A network is found by its ID, its name or a short ID
    :id: REQ-NET-LOOKUP
    :source_kind: api
    :source: https://docs.docker.com/reference/api/engine/version/v1.54/#tag/Network/operation/NetworkInspect, https://docs.docker.com/reference/cli/docker/network/rm/#remove-multiple-networks: an example that removes a network by its short ID, api/openapi.yaml: operation NetworkDelete, moby/moby daemon/network.go: code comment on FindNetwork that lists the order: full ID then full name then partial ID

    Network requests take a network's full ID, its name or a prefix of its ID, such as a short ID. A
    full ID is matched first, then a name, then a prefix, and the first of these that matches a
    network decides. GET /networks/{id} and DELETE /networks/{id} respond 404 when no network
    matches, and DELETE /networks/{id} responds 400 when the name or ID prefix matches more than one
    network at that rank.

.. req:: A macvlan network's parent can be a VLAN sub-interface that Docker creates
    :id: REQ-NET-MACVLAN-VLAN-PARENT
    :source_kind: docs
    :source: https://docs.docker.com/engine/network/drivers/macvlan/#8021q-trunk-bridge-mode

    A macvlan network's parent can be named <interface>.<VLAN ID>. Docker interprets the name as a
    sub-interface of <interface> and creates the sub-interface automatically.

.. req:: A macvlan network without a parent interface is internal
    :id: REQ-NET-MACVLAN-NO-PARENT-INTERNAL
    :source_kind: issue
    :source: moby/moby daemon/libnetwork/drivers/macvlan/macvlan_network.go: code comment on a network without a parent interface and the debug log message that the driver writes when it creates the dummy parent ("Empty -o parent= flags limit communications to other containers inside of network")

    A macvlan network created without a parent interface is internal: its containers communicate
    only with other containers on the network, and reach no address outside it.

.. req:: Creating a network with an existing network's name is refused
    :id: REQ-NET-NAME-UNIQUE
    :source_kind: api
    :source: https://docs.docker.com/reference/api/engine/version-history/#v144-api-changes, https://github.com/moby/moby/pull/35030

    Network names must be unique. A create with a duplicate name fails at every API version, and the
    refusal responds 409.

.. req:: Predefined networks cannot be created by name, removed or pruned
    :id: REQ-NET-PREDEFINED-RESERVED
    :source_kind: api
    :source: https://docs.docker.com/reference/api/engine/version/v1.54/#tag/Network/operation/NetworkCreate, https://docs.docker.com/reference/api/engine/version/v1.54/#tag/Network/operation/NetworkDelete, https://docs.docker.com/reference/cli/docker/network/create/, https://docs.docker.com/reference/cli/docker/network/prune/, api/openapi.yaml: operation NetworkCreate

    Creating a network named after a predefined network is forbidden (403), and so is removing a
    predefined network. On a Swarm manager, the names of the Swarm's predefined networks, bridge and
    host on Linux, conflict with those networks instead (409). The default bridge network cannot be
    removed, and bridge, host and none are never pruned.

.. req:: Network prune removes unused networks and names them
    :id: REQ-NET-PRUNE-UNUSED
    :source_kind: docs
    :source: https://docs.docker.com/reference/cli/docker/network/prune/, https://docs.docker.com/engine/manage-resources/pruning/#prune-networks, https://docs.docker.com/reference/api/engine/version/v1.54/#tag/Network/operation/NetworkPrune

    docker network prune and POST /networks/prune remove the networks that no container references,
    never the system networks, and report the names of the networks deleted.

.. req:: Network names that the container network mode claims are reserved
    :id: REQ-NET-RESERVED-NAMES
    :source_kind: issue
    :source: https://github.com/moby/moby/pull/51973: the merged change that reserves these names

    The network name container, and every name that starts with container:, are reserved, because
    the container network mode claims these names. Creating a network with such a name is refused
    with 403, on Linux and Windows alike.

.. req:: An auto-remove container is removed when its start fails
    :id: REQ-NET-START-FAILED-AUTO-REMOVED
    :source_kind: issue
    :source: https://github.com/moby/moby/pull/20848: the merged move of auto-remove into the daemon whose stated intent includes this case

    A container created with auto-remove (--rm) is removed when its start fails.

.. req:: A container whose start fails remains and can be started again
    :id: REQ-NET-START-FAILED-CONTAINER-KEPT
    :source_kind: observable
    :source: Users and tools that read State.Error with docker inspect after a failed start and that start the same container again once they fix the cause (docker start retries/docker compose up run again)

    When a start request fails for a container without auto-remove, the container remains, and its
    State.Status is still created or exited, as before the request. It can be inspected, and started
    again.

.. req:: A container gets its requested static address on a user-defined network
    :id: REQ-NET-STATIC-ADDRESS
    :source_kind: docs
    :source: https://docs.docker.com/reference/cli/docker/container/run/#network, https://docs.docker.com/reference/cli/docker/network/connect/#ip, https://docs.docker.com/engine/release-notes/29/#2913: a container can be created with a specific IP address on a network that was not configured with a subnet

    --ip and --ip6 on docker run or docker network connect choose the container's address on a
    user-defined network, also on one created without a subnet; the address is reapplied when the
    container restarts.

.. req:: Network create refuses a bad parameter with an error response
    :id: REQ-NET-NETWORK-CREATE-ERRORS
    :source_kind: api
    :source: api/openapi.yaml: operation NetworkCreate

    POST /networks/create documents status 400 for a bad parameter and 500 for a server error; a
    refused request does not create the network.

.. _network-common-rationales:

************
 Rationales
************

.. rationale:: The ingress sandbox's endpoint on docker_gwbridge shows a node's part of the routing mesh
    :id: RAT-NET-INGRESS-SBOX-ON-GWBRIDGE

    :need:`NET-API-GWBRIDGE-INGRESS-SBOX` notices a change to when a node gives its ingress sandbox
    an endpoint on :network:`docker_gwbridge`, or to how :api-op:`GET /networks/docker_gwbridge`
    reports it. Operators debugging the routing mesh look for ``ingress-sbox`` there to tell whether
    a node has set up its part of the mesh, and the endpoint is what keeps
    :network:`docker_gwbridge` from :api-op:`POST /networks/prune`
    (:need:`NET-API-PRUNE-GWBRIDGE-SWARM`). Renaming the endpoint, moving it, or setting it up later
    than the init or join would mislead that debugging and change what a prune leaves, so such a
    change should be deliberate.

.. rationale:: Driver properties decide a container's interfaces and its MAC requests
    :id: RAT-NET-DRIVER-PROPERTY-SETS

    Expectations in several specs are scoped by a property of the network's driver, such as "a
    driver that adds an interface to the container" or "a driver that accepts an endpoint MAC
    address", and do not list the drivers. :need:`NET-DRVPROP-ADDS-INTERFACE` and
    :need:`NET-DRVPROP-ENDPOINT-MAC` say which built-in drivers have each property. A driver that
    started or stopped adding an interface would add or remove an interface in every container on
    its networks, so the operator's tooling and debugging inside the container would see a different
    set of interfaces. A driver that started or stopped accepting an endpoint MAC address would
    change which requests that set :api-field:`MacAddress` for an endpoint on its networks succeed.
    Either change should be deliberate.

.. rationale:: A filter that matches nothing is an empty result, not a refusal
    :id: RAT-NET-FILTER-NO-MATCH-NOT-REFUSED

    :need:`NET-API-FILTER-INVALID` refuses a filter that is not well formed. A well-formed filter
    value that matches no network gives an empty list instead, as the Engine API's other list
    requests do for a filter that matches nothing. A change that refused such a value would make an
    empty selection look like a malformed request, and set the network list apart from its sibling
    requests. It should be made by decision.

.. rationale:: A network named default could not be told from the default network mode
    :id: RAT-NET-NAME-DEFAULT-REFUSED

    In a container's :api-field:`HostConfig.NetworkMode`, ``default`` selects the daemon's default
    network, so a network with that name could not be named there. No source reserves the name, as
    :need:`REQ-NET-RESERVED-NAMES` does for ``container``. A change that lets such a network be
    created should be made together with a decision about what ``default`` then selects.

.. _network-common-host-requirements:

*******************
 Host requirements
*******************

The kernel facilities and the host configuration are requirements of each Linux node. The
conventions for this section are in :ref:`README.md <README-host-requirements>`. The specs of the
drivers list what each driver needs, in the host requirements of :ref:`bridge-networking.md
<bridge-networking-host-requirements>`, :ref:`swarm-networking.md
<swarm-networking-host-requirements>` and :ref:`ipvlan-networking.md
<ipvlan-networking-host-requirements>`.

.. _network-common-kernel-facilities:

Kernel facilities
=================

.. hostreq:: Network namespaces
    :id: NET-HOST-NETNS
    :kind: os
    :platforms: linux
    :kernel_options: NET_NS

    *Needed when:* Always

.. needextend:: type == "expectation" and docname in ['network/network-common', 'network/bridge-networking', 'network/ipvlan-networking', 'network/container-dns', 'network/swarm-networking', 'network/swarm-traffic', 'network/swarm-lifecycle'] and ("linux" in platforms) and "NET-HOST-NETNS" not in pins
    :+needs: NET-HOST-NETNS

.. hostreq:: macvlan links
    :id: NET-HOST-MACVLAN
    :kind: os
    :platforms: linux
    :kernel_options: MACVLAN
    :tags: macvlan

    *Needed when:* A container on a :driver:`macvlan` network

.. hostreq:: Dummy links
    :id: NET-HOST-DUMMY
    :kind: os
    :platforms: linux
    :kernel_options: DUMMY
    :tags: macvlan, no-parent

    *Needed when:* A :driver:`macvlan` network without a :drvopt:`parent`

    Not pinned. The error for a missing dummy link does not name the facility.

.. hostreq:: VLAN links
    :id: NET-HOST-VLAN
    :kind: os
    :platforms: linux
    :kernel_options: VLAN_8021Q
    :tags: macvlan, vlan-parent

    *Needed when:* A :driver:`macvlan` network with a :drvopt:`parent` of the form
    ``<interface>.<VLAN ID>``

.. _network-common-host-configuration:

Host configuration
==================

.. hostreq:: The host's firewall passes the traffic that an expectation involves
    :id: NET-HOST-FIREWALL-PASSES-TRAFFIC
    :kind: config
    :platforms: linux

    *Facility:* The host's firewall passes the traffic that an expectation involves, to and from the
    node, its containers and its published ports. The rules and the firewalld configuration that the
    daemon programs are not part of the host's firewall.

    *Needed when:* Always. In an expectation whose state or statement names firewalld, the host's
    firewall is firewalld, running with the configuration that the node's operating system installs,
    so that firewalld's default zone applies to the node's interfaces. Its only additions open the
    traffic that a host configuration requirement of the expectation's spec names, such as the Swarm
    ports (:need:`SWARM-HOST-FIREWALL-CLUSTER-PORTS`), with configuration that a firewalld reload
    keeps. Where the expectation's spec has no such requirement, as :doc:`bridge-networking.md
    <bridge-networking>` and :doc:`container-dns.md <container-dns>` do not, firewalld passes the
    expectation's traffic only where its installed configuration or the configuration that the
    daemon programs passes it. An expectation that names neither firewalld nor its absence holds
    whether or not firewalld runs. On a node where firewalld runs, such an expectation holds with
    the configuration that the node's baseline gives firewalld. For a daemon installed from a
    package, that is the configuration that the operating system installs and the setup that the
    product's installation documentation prescribes, so traffic that firewalld drops there fails the
    expectation unless a defect that narrows it states the case. For any other daemon, it is a
    configuration that passes the expectation's traffic.

.. needextend:: type == "expectation" and docname in ['network/network-common', 'network/bridge-networking', 'network/ipvlan-networking', 'network/container-dns', 'network/swarm-networking', 'network/swarm-traffic', 'network/swarm-lifecycle'] and ("linux" in platforms) and "NET-HOST-FIREWALL-PASSES-TRAFFIC" not in pins
    :+needs: NET-HOST-FIREWALL-PASSES-TRAFFIC

.. _network-common-network-api:

*************
 Network API
*************

.. expectation:: Creating a network with an existing network's name responds 409
    :id: NET-API-CREATE-DUPLICATE-NAME
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-NET-NAME-UNIQUE, REQ-NET-API-ERROR-STATUS

    *Applies to:* An existing network that is not Swarm-scoped and not predefined

    :api-op:`POST /networks/create` with the network's name responds 409. :api-op:`GET /networks`
    then lists only one network with that name.

.. expectation:: docker network create with an existing network's name fails
    :id: NET-API-CREATE-DUPLICATE-NAME-CLI
    :observed_from: api
    :depends_on: cli
    :platforms: linux, windows
    :refines: REQ-NET-NAME-UNIQUE
    :cli_for: NET-API-CREATE-DUPLICATE-NAME

    *Applies to:* An existing network that is not Swarm-scoped and not predefined

    :cli:`docker network create` with the network's name exits non-zero and prints the daemon's
    error. :cli:`docker network ls` then lists only one network with that name.

.. expectation:: Outside a manager, a predefined or reserved network name responds 403
    :id: NET-API-CREATE-PREDEFINED-NAME
    :observed_from: api:solo, api:wkr
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-NET-PREDEFINED-RESERVED, REQ-NET-API-ERROR-STATUS, REQ-NET-RESERVED-NAMES

    *Applies to:* A node that is not a Swarm manager

    :api-op:`POST /networks/create` with the name of a predefined network or ``container`` responds
    403. :api-op:`POST /networks/create` with a name that starts with ``container:`` also responds
    403. :api-op:`GET /networks` then does not list a new network with that name.

.. expectation:: A network named default is refused with 403
    :id: NET-API-CREATE-DEFAULT-NAME
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows
    :rationale: RAT-NET-NAME-DEFAULT-REFUSED

    :api-op:`POST /networks/create` with the name :network:`default` responds 403, on a node that is
    not in a Swarm and on a Swarm node. :api-op:`GET /networks` then does not list a network with
    that name.

.. expectation:: docker network create with a predefined network's name fails
    :id: NET-API-CREATE-PREDEFINED-NAME-CLI
    :observed_from: api
    :depends_on: cli
    :platforms: linux, windows
    :refines: REQ-NET-PREDEFINED-RESERVED
    :cli_for: NET-API-CREATE-PREDEFINED-NAME

    :cli:`docker network create` with the name of a predefined network exits non-zero and prints the
    daemon's error.

.. expectation:: Deleting a predefined network responds 403
    :id: NET-API-DELETE-PREDEFINED
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-NET-PREDEFINED-RESERVED, REQ-NET-API-ERROR-STATUS

    :api-op:`DELETE /networks/{id}` for a predefined network responds 403. :api-op:`GET /networks`
    then still lists the network.

.. expectation:: docker network rm of a predefined network fails
    :id: NET-API-DELETE-PREDEFINED-CLI
    :observed_from: api
    :depends_on: cli
    :platforms: linux, windows
    :refines: REQ-NET-PREDEFINED-RESERVED
    :cli_for: NET-API-DELETE-PREDEFINED

    :cli:`docker network rm` for a predefined network exits non-zero and prints the daemon's error.
    :cli:`docker network ls` then still lists the network.

.. expectation:: Deleting a network with a running container is refused
    :id: NET-API-DELETE-IN-USE
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-NET-DELETE-IN-USE, REQ-NET-API-ERROR-STATUS

    *Applies to:* A network that is not Swarm-scoped, with a running container attached to it

    :api-op:`DELETE /networks/{id}` for the network responds with a 4xx status. :api-op:`GET
    /networks` then still lists the network.

.. expectation:: Deleting a network with a running container responds 403
    :id: NET-API-DELETE-IN-USE-HTTP403
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows
    :exact_form_of: NET-API-DELETE-IN-USE

    *Applies to:* A network that is not Swarm-scoped, with a running container attached to it

    :api-op:`DELETE /networks/{id}` for the network responds 403. :api-op:`GET /networks` then still
    lists the network.

.. expectation:: docker network rm of a network with a running container fails
    :id: NET-API-DELETE-IN-USE-CLI
    :observed_from: api
    :depends_on: cli
    :platforms: linux, windows
    :refines: REQ-NET-DELETE-IN-USE
    :cli_for: NET-API-DELETE-IN-USE

    *Applies to:* A network that is not Swarm-scoped, with a running container attached to it

    :cli:`docker network rm` for the network exits non-zero and prints the daemon's error.
    :cli:`docker network ls` then still lists the network.

.. expectation:: Connecting a running container to a network it is attached to fails
    :id: NET-API-CONNECT-ALREADY-ATTACHED
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-NET-CONNECT-ONCE, REQ-NET-API-ERROR-STATUS

    *Applies to:* A running container attached to a network

    :api-op:`POST /networks/{id}/connect` to that network, for the container, responds with a 4xx
    status, also when its endpoint settings request another address in :api-field:`IPAMConfig`. The
    container's addresses on that network do not change.

.. expectation:: A repeat connect to a network that is not Swarm-scoped responds 403
    :id: NET-API-CONNECT-ALREADY-ATTACHED-HTTP403
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows
    :exact_form_of: NET-API-CONNECT-ALREADY-ATTACHED

    *Applies to:* A running container attached to a network that is not Swarm-scoped

    :api-op:`POST /networks/{id}/connect` to that network, for the container, responds 403.

.. expectation:: docker network connect to a network already attached fails
    :id: NET-API-CONNECT-ALREADY-ATTACHED-CLI
    :observed_from: api
    :depends_on: cli
    :platforms: linux, windows
    :refines: REQ-NET-CONNECT-ONCE
    :cli_for: NET-API-CONNECT-ALREADY-ATTACHED

    *Applies to:* A running container attached to a network

    :cli:`docker network connect` naming that network and the container exits non-zero and prints
    the daemon's error.

.. expectation:: A repeat connect of a created or stopped container responds 200
    :id: NET-API-CONNECT-ALREADY-ATTACHED-STOPPED
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-NET-CONNECT-REPEAT-STOPPED

    *Applies to:* A created or stopped container whose :api-field:`NetworkSettings.Networks` lists a
    network that is not Swarm-scoped

    :api-op:`POST /networks/{id}/connect` to that network, for the container, without endpoint
    settings or with endpoint settings that the daemon accepts for that network, responds 200.
    :api-field:`NetworkSettings.Networks` of the container then still lists the network.

.. expectation:: A repeat docker network connect of a created or stopped container succeeds
    :id: NET-API-CONNECT-ALREADY-ATTACHED-STOPPED-CLI
    :observed_from: api
    :depends_on: cli
    :platforms: linux, windows
    :refines: REQ-NET-CONNECT-REPEAT-STOPPED
    :cli_for: NET-API-CONNECT-ALREADY-ATTACHED-STOPPED

    *Applies to:* A created or stopped container whose :api-field:`NetworkSettings.Networks` lists a
    network that is not Swarm-scoped

    :cli:`docker network connect` naming that network and the container, without options or with
    options that the daemon accepts for that network, exits zero. :cli:`docker inspect` for the
    container then still lists the network in :api-field:`NetworkSettings.Networks`.

.. expectation:: A repeat connect of a stopped container drops the new endpoint settings
    :id: NET-API-CONNECT-ALREADY-ATTACHED-STOPPED-SETTINGS
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows
    :concerns: NET-API-CONNECT-ALREADY-ATTACHED-STOPPED
    :violates: PRIN-FAILS-CLOSED

    *Applies to:* A created or stopped container whose :api-field:`NetworkSettings.Networks` lists a
    network that is not Swarm-scoped

    :api-op:`POST /networks/{id}/connect` to that network, for the container, with endpoint settings
    that the daemon accepts for that network and that differ from the stored ones, such as other
    aliases, responds 200. The container's endpoint settings for the network do not change.

.. expectation:: A created or stopped container connects to a network
    :id: NET-API-CONNECT-STOPPED
    :observed_from: api, ctr
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-NET-DISCONNECT

    *Applies to:* A created or stopped container attached to a bridge network, and a user-defined
    bridge network that it is not attached to

    :api-op:`POST /networks/{id}/connect` to that network, for the container, responds 200.
    :api-op:`GET /containers/{id}/json` then lists the network in
    :api-field:`NetworkSettings.Networks`. After :api-op:`POST /containers/{id}/start` for the
    container responds 204, the container has an interface on the network.

.. expectation:: docker network connect of a created or stopped container succeeds
    :id: NET-API-CONNECT-STOPPED-CLI
    :observed_from: api
    :depends_on: cli
    :platforms: linux
    :refines: REQ-NET-DISCONNECT
    :cli_for: NET-API-CONNECT-STOPPED

    *Applies to:* A created or stopped container attached to a bridge network, and a user-defined
    bridge network that it is not attached to

    :cli:`docker network connect` naming that network and the container exits zero. :cli:`docker
    inspect` for the container then lists the network in :api-field:`NetworkSettings.Networks`.

.. expectation:: A created or stopped container's connect by a shared ID prefix responds 200
    :id: NET-API-CONNECT-STOPPED-AMBIGUOUS-PREFIX
    :observed_from: api:solo
    :depends_on: none
    :platforms: linux, windows
    :concerns: NET-API-CONNECT-STOPPED, NET-API-INSPECT-AMBIGUOUS-PREFIX
    :violates: PRIN-AMBIGUITY-NOT-DESTRUCTIVE

    *Applies to:* A node that is not in a Swarm, two or more networks whose IDs start with a prefix
    that is not a network name, and a created or stopped container

    :api-op:`POST /networks/{id}/connect` for the container, with that prefix as ``{id}``, responds
    200. :api-op:`GET /containers/{id}/json` for the container then lists the prefix in
    :api-field:`NetworkSettings.Networks`.

.. expectation:: A connect to a missing network gives no warning, and the start responds 404
    :id: NET-API-CONNECT-MISSING-START-FAILS
    :observed_from: api:solo
    :depends_on: none
    :platforms: linux, windows
    :concerns: NET-API-CONNECT-MISSING-CREATED-LATER
    :violates: PRIN-WARN-AHEAD

    *Applies to:* A node that is not in a Swarm, with a created or stopped container

    :api-op:`POST /networks/{id}/connect` for the container, with an ``{id}`` that is not the name
    or an ID prefix of any network, responds 200 without a warning. :api-op:`POST
    /containers/{id}/start` for the container then responds 404 while no network has that name, with
    a message that contains :api-error:`network <name> not found`.

.. expectation:: A container connected to a missing network uses it once it is created
    :id: NET-API-CONNECT-MISSING-CREATED-LATER
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows
    :pins: NET-API-CONNECT-MISSING-START-FAILS

    *Applies to:* A created or stopped container

    :api-op:`POST /networks/{id}/connect` for the container, with an ``{id}`` that is not the name
    or an ID prefix of any network, responds 200. A network with that name that is not config-only
    is then created. :api-op:`POST /containers/{id}/start` for the container then responds 204, and
    the container has an address on that network.

.. expectation:: A static address outside the network's subnets is refused
    :id: NET-API-STATIC-ADDRESS-OUTSIDE-SUBNET
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows
    :refines: PRIN-FAILS-CLOSED, PRIN-FAIL-FAST, REQ-NET-API-ERROR-STATUS

    *Applies to:* A network that is not Swarm-scoped and that, on a Windows node, was created with a
    subnet in :api-field:`IPAM.Config`, and a container that is not attached to it

    :api-op:`POST /containers/create` with a static address in :api-field:`IPAMConfig` that is
    outside every subnet of that network responds with a 4xx status, and does not create a
    container. :api-op:`POST /networks/{id}/connect` to that network, for the container, with that
    address, responds with a 4xx status, and does not attach the container.

.. expectation:: A static address outside the network's subnets responds 400
    :id: NET-API-STATIC-ADDRESS-OUTSIDE-SUBNET-HTTP400
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows
    :exact_form_of: NET-API-STATIC-ADDRESS-OUTSIDE-SUBNET

    *Applies to:* A network that is not Swarm-scoped and that, on a Windows node, was created with a
    subnet in :api-field:`IPAM.Config`, and a container that is not attached to it

    :api-op:`POST /containers/create` with a static address in :api-field:`IPAMConfig` that is
    outside every subnet of that network responds 400, and does not create a container.
    :api-op:`POST /networks/{id}/connect` to that network, for the container, with that address,
    responds 400, and does not attach the container.

.. expectation:: A static address on the default bridge is refused
    :id: NET-API-STATIC-ADDRESS-DEFAULT-BRIDGE
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :refines: PRIN-FAILS-CLOSED, PRIN-FAIL-FAST, REQ-NET-API-ERROR-STATUS

    *Applies to:* Linux nodes with a container that is not attached to the network :network:`bridge`

    :api-op:`POST /containers/create` with a static address in :api-field:`IPAMConfig` for the
    network :network:`bridge` responds with a 4xx status, and does not create a container.
    :api-op:`POST /networks/{id}/connect` to the network :network:`bridge`, for the container, with
    a static address in :api-field:`IPAMConfig`, responds with a 4xx status, and does not attach the
    container.

.. expectation:: A static address on the default bridge responds 400
    :id: NET-API-STATIC-ADDRESS-DEFAULT-BRIDGE-HTTP400
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :exact_form_of: NET-API-STATIC-ADDRESS-DEFAULT-BRIDGE

    *Applies to:* Linux nodes with a container that is not attached to the network :network:`bridge`

    :api-op:`POST /containers/create` with a static address in :api-field:`IPAMConfig` for the
    network :network:`bridge` responds 400, and does not create a container. :api-op:`POST
    /networks/{id}/connect` to the network :network:`bridge`, for the container, with a static
    address in :api-field:`IPAMConfig`, responds 400, and does not attach the container.

.. expectation:: A create naming the default bridge by its ID accepts a static address
    :id: NET-API-STATIC-ADDRESS-DEFAULT-BRIDGE-ID
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :narrows: NET-API-STATIC-ADDRESS-DEFAULT-BRIDGE, NET-API-STATIC-ADDRESS-DEFAULT-BRIDGE-HTTP400
    :violates: PRIN-FAIL-FAST

    :api-op:`POST /containers/create` whose :api-field:`NetworkingConfig.EndpointsConfig` names the
    network :network:`bridge` by its ID, or by a prefix of its ID, with a static address in
    :api-field:`IPAMConfig` that a subnet of the network contains, responds 201 and creates the
    container. :api-op:`POST /containers/{id}/start` for the container then responds with a 4xx
    status and does not start it.

.. expectation:: docker run and docker network connect with --ip on the default bridge fail
    :id: NET-API-STATIC-ADDRESS-DEFAULT-BRIDGE-CLI
    :observed_from: api
    :depends_on: cli
    :platforms: linux
    :refines: PRIN-FAILS-CLOSED
    :cli_for: NET-API-STATIC-ADDRESS-DEFAULT-BRIDGE

    :cli:`docker run` and :cli:`docker network connect` with :cli-flag:`--ip` for the network
    :network:`bridge` exit non-zero and print the daemon's error.

.. expectation:: docker run and docker network connect with a refused --ip fail
    :id: NET-API-STATIC-ADDRESS-OUTSIDE-SUBNET-CLI
    :observed_from: api
    :depends_on: cli
    :platforms: linux, windows
    :refines: PRIN-FAILS-CLOSED
    :cli_for: NET-API-STATIC-ADDRESS-OUTSIDE-SUBNET

    :cli:`docker run` and :cli:`docker network connect` with :cli-flag:`--ip` outside every subnet
    of a network that is not Swarm-scoped exit non-zero and print the daemon's error. On Windows
    nodes, they do so for a network that was created with a subnet.

.. expectation:: A LinkLocalIPs entry that is not link-local or has a zone is refused
    :id: NET-API-LINK-LOCAL-ADDRESS-INVALID
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-NET-LINK-LOCAL-ADDRESS, REQ-NET-API-ERROR-STATUS, PRIN-FAIL-FAST

    *Applies to:* A network that is not Swarm-scoped, and a container that is not attached to it

    :api-op:`POST /containers/create` with an address that is not in ``169.254.0.0/16`` or
    ``fe80::/10``, or an address with a zone, such as ``fe80::1%eth0``, in
    :api-field:`IPAMConfig.LinkLocalIPs` for that network responds with a 4xx status, and does not
    create a container. :api-op:`POST /networks/{id}/connect` to that network, for the container,
    with such an address, responds with a 4xx status, and does not attach the container.

.. expectation:: An invalid LinkLocalIPs entry responds 400 and says the IP is invalid
    :id: NET-API-LINK-LOCAL-ADDRESS-INVALID-HTTP400
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows
    :exact_form_of: NET-API-LINK-LOCAL-ADDRESS-INVALID

    *Applies to:* A network that is not Swarm-scoped, and a container that is not attached to it

    :api-op:`POST /containers/create` with an address that is not in ``169.254.0.0/16`` or
    ``fe80::/10``, or an address with a zone, such as ``fe80::1%eth0``, in
    :api-field:`IPAMConfig.LinkLocalIPs` for that network responds 400, with a message that contains
    :api-error:`invalid link-local IP address`. So does :api-op:`POST /networks/{id}/connect` to
    that network, for the container, with such an address.

.. expectation:: A --link-local-ip that is not link-local fails docker run and connect
    :id: NET-API-LINK-LOCAL-ADDRESS-INVALID-CLI
    :observed_from: api
    :depends_on: cli
    :platforms: linux, windows
    :refines: REQ-NET-LINK-LOCAL-ADDRESS
    :cli_for: NET-API-LINK-LOCAL-ADDRESS-INVALID

    :cli:`docker run` and :cli:`docker network connect` with :cli-flag:`--link-local-ip` set to an
    address that is not in ``169.254.0.0/16`` or ``fe80::/10``, for a network that is not
    Swarm-scoped, exit non-zero and print the daemon's error.

.. expectation:: At API version 1.43 or earlier, create on several networks is refused
    :id: NET-API-CTR-CREATE-MULTIPLE-NETWORKS-REFUSED
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-NET-CREATE-MULTIPLE-NETWORKS, REQ-NET-API-ERROR-STATUS

    At API version 1.43 or earlier, :api-op:`POST /containers/create` with more than one network in
    :api-field:`NetworkingConfig.EndpointsConfig` responds with a 4xx status, and does not create a
    container.

.. expectation:: At API version 1.43 or earlier, create on several networks responds 400
    :id: NET-API-CTR-CREATE-MULTIPLE-NETWORKS-REFUSED-HTTP400
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows
    :exact_form_of: NET-API-CTR-CREATE-MULTIPLE-NETWORKS-REFUSED

    At API version 1.43 or earlier, :api-op:`POST /containers/create` with more than one network in
    :api-field:`NetworkingConfig.EndpointsConfig` responds 400, and does not create a container.

.. expectation:: At API version 1.44 or later, a container is created on several networks
    :id: NET-API-CTR-CREATE-MULTIPLE-NETWORKS
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-NET-CREATE-MULTIPLE-NETWORKS

    *Applies to:* Two networks that are not Swarm-scoped, not predefined and not config-only

    At API version 1.44 or later, :api-op:`POST /containers/create` with both networks in
    :api-field:`NetworkingConfig.EndpointsConfig` responds 201. :api-op:`GET /containers/{id}/json`
    then lists both networks in :api-field:`NetworkSettings.Networks`. After :api-op:`POST
    /containers/{id}/start` for the container responds 204, :api-op:`GET /containers/{id}/json`
    reports a non-empty :api-field:`EndpointID` for each of the two networks.

.. expectation:: A container created without a network is attached to the default bridge
    :id: NET-API-CTR-CREATE-DEFAULT-BRIDGE
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :refines: REQ-NET-DEFAULT-BRIDGE-ATTACHMENT

    *Applies to:* A Linux node whose daemon has the default bridge enabled

    :api-op:`POST /containers/create` without :api-field:`HostConfig.NetworkMode` and without
    :api-field:`NetworkingConfig.EndpointsConfig` responds 201. :api-op:`GET /containers/{id}/json`
    then reports the default bridge network, :network:`bridge`, and no other network in
    :api-field:`NetworkSettings.Networks`.

.. expectation:: docker run with two --network flags attaches the container to both
    :id: NET-API-CTR-CREATE-MULTIPLE-NETWORKS-CLI
    :observed_from: api
    :depends_on: cli
    :platforms: linux, windows
    :refines: REQ-NET-CREATE-MULTIPLE-NETWORKS
    :cli_for: NET-API-CTR-CREATE-MULTIPLE-NETWORKS

    *Applies to:* Two networks that are not Swarm-scoped, not predefined and not config-only

    :cli:`docker run -d` with a :cli-flag:`--network` flag for each of the two networks exits zero.
    :cli:`docker inspect` for the container then lists both networks in
    :api-field:`NetworkSettings.Networks`.

.. expectation:: Disconnecting a container from a network it is not attached to fails
    :id: NET-API-DISCONNECT-NOT-ATTACHED
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-NET-DISCONNECT-NOT-ATTACHED

    *Applies to:* A container, and a network that it is not attached to

    :api-op:`POST /networks/{id}/disconnect` for the container from that network responds with a 4xx
    or 5xx status.

.. expectation:: Disconnecting from a network the container is not on responds 500
    :id: NET-API-DISCONNECT-NOT-ATTACHED-HTTP500
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows
    :exact_form_of: NET-API-DISCONNECT-NOT-ATTACHED
    :pins: NET-IFACE-HTTP500-REFUSED

    *Applies to:* A container, and a network that it is not attached to

    :api-op:`POST /networks/{id}/disconnect` for the container from that network responds 500, with
    a message that contains :api-error:`is not connected to`.

.. expectation:: docker network disconnect from a network not attached fails
    :id: NET-API-DISCONNECT-NOT-ATTACHED-CLI
    :observed_from: api
    :depends_on: cli
    :platforms: linux, windows
    :refines: REQ-NET-DISCONNECT-NOT-ATTACHED
    :cli_for: NET-API-DISCONNECT-NOT-ATTACHED

    *Applies to:* A container, and a network that it is not attached to

    :cli:`docker network disconnect` naming that network and the container exits non-zero and prints
    the daemon's error.

.. expectation:: A running container disconnects from a network and keeps running
    :id: NET-API-DISCONNECT-RUNNING
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-NET-DISCONNECT

    *Applies to:* A running container with an endpoint on a network other than :network:`host`, from
    its start or from a later connect

    :api-op:`POST /networks/{id}/disconnect` for the container responds 200. :api-op:`GET
    /containers/{id}/json` then does not list the network in :api-field:`NetworkSettings.Networks`,
    and the container still runs.

.. expectation:: A disconnected container no longer has its interface on the network
    :id: NET-API-DISCONNECT-RUNNING-IFACE
    :observed_from: ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-NET-DISCONNECT

    *Applies to:* A running container with an endpoint on a network other than :network:`host`, from
    its start or from a later connect

    After :api-op:`POST /networks/{id}/disconnect` for the container responds 200, the container
    does not have its interface on that network.

.. expectation:: A disconnect leaves an interface whose name the kernel numbered
    :id: NET-API-DISCONNECT-IFACE-LEFT
    :observed_from: api, ctr
    :depends_on: none
    :platforms: linux
    :narrows: NET-API-DISCONNECT-RUNNING-IFACE, SWARM-CTR-DISCONNECT
    :violates: PRIN-NO-LEAKS, REQ-NET-DISCONNECT

    For an endpoint whose requested interface name contains ``%d``, :api-op:`POST
    /networks/{id}/disconnect` for the container responds 200, but the interface stays in the
    container. The interface stays up and keeps its address.

.. expectation:: docker network disconnect of a running container succeeds
    :id: NET-API-DISCONNECT-RUNNING-CLI
    :observed_from: api
    :depends_on: cli
    :platforms: linux, windows
    :refines: REQ-NET-DISCONNECT
    :cli_for: NET-API-DISCONNECT-RUNNING

    *Applies to:* A running container with an endpoint on a network other than :network:`host`, from
    its start or from a later connect

    :cli:`docker network disconnect` for the container exits zero. :cli:`docker inspect` for the
    container then does not list the network in :api-field:`NetworkSettings.Networks`.

.. expectation:: A created or stopped container disconnects from a listed network
    :id: NET-API-DISCONNECT-STOPPED
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-NET-DISCONNECT

    *Applies to:* A created or stopped container whose :api-field:`NetworkSettings.Networks` lists a
    network

    :api-op:`POST /networks/{id}/disconnect` for the container from that network responds 200.
    :api-op:`GET /containers/{id}/json` then does not list the network in
    :api-field:`NetworkSettings.Networks`.

.. expectation:: docker network disconnect of a created or stopped container succeeds
    :id: NET-API-DISCONNECT-STOPPED-CLI
    :observed_from: api
    :depends_on: cli
    :platforms: linux, windows
    :refines: REQ-NET-DISCONNECT
    :cli_for: NET-API-DISCONNECT-STOPPED

    *Applies to:* A created or stopped container whose :api-field:`NetworkSettings.Networks` lists a
    network

    :cli:`docker network disconnect` naming that network and the container exits zero. :cli:`docker
    inspect` for the container then does not list the network in
    :api-field:`NetworkSettings.Networks`.

.. expectation:: A network is found by its full ID, its name or a unique ID prefix
    :id: NET-API-INSPECT-ID-NAME-PREFIX
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-NET-LOOKUP

    :api-op:`GET /networks/{id}` responds 200 with the network when ``{id}`` is the network's full
    ID, or its name when no other network has that name. It does the same when ``{id}`` is a prefix
    of the network's ID that the ID of no other network starts with, and that is not the name of
    another network.

.. expectation:: docker network inspect finds a network by ID, name or unique prefix
    :id: NET-API-INSPECT-ID-NAME-PREFIX-CLI
    :observed_from: api
    :depends_on: cli
    :platforms: linux, windows
    :refines: REQ-NET-LOOKUP
    :cli_for: NET-API-INSPECT-ID-NAME-PREFIX

    :cli:`docker network inspect` prints the network when it is given the network's full ID, or its
    name when no other network has that name. It does the same when it is given a prefix of the
    network's ID that the ID of no other network starts with, and that is not the name of another
    network.

.. expectation:: Inspecting a network that matches no ID, name or prefix responds 404
    :id: NET-API-INSPECT-NOT-FOUND
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-NET-LOOKUP, REQ-NET-API-ERROR-STATUS

    :api-op:`GET /networks/{id}` responds 404 when ``{id}`` is not the full ID, the name or a prefix
    of the ID of any network.

.. expectation:: Deleting a network that matches no ID, name or prefix responds 404
    :id: NET-API-DELETE-NOT-FOUND
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-NET-LOOKUP, REQ-NET-API-ERROR-STATUS

    :api-op:`DELETE /networks/{id}` responds 404 when ``{id}`` is not the full ID, the name or a
    prefix of the ID of any network.

.. expectation:: Inspecting by an ID prefix that several networks share fails
    :id: NET-API-INSPECT-AMBIGUOUS-PREFIX
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-NET-API-ERROR-STATUS, PRIN-AMBIGUITY-NOT-DESTRUCTIVE

    *Applies to:* Two or more networks whose IDs start with a prefix that is not a network name

    :api-op:`GET /networks/{id}` with that prefix as ``{id}`` responds with a 4xx status.

.. expectation:: An ID prefix that several networks share responds 400
    :id: NET-API-INSPECT-AMBIGUOUS-PREFIX-HTTP400
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows
    :exact_form_of: NET-API-INSPECT-AMBIGUOUS-PREFIX

    *Applies to:* Two or more networks whose IDs start with a prefix that is not a network name

    :api-op:`GET /networks/{id}` with that prefix as ``{id}`` responds 400, with a message that
    contains :api-error:`is ambiguous`.

.. expectation:: GET /networks/{id} reports Scope local and the network's create fields
    :id: NET-API-INSPECT-CONFIGURATION
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-NET-INSPECT-CONFIGURATION

    *Applies to:* A network of a built-in node-local driver that is not Swarm-scoped, created
    without :api-field:`ConfigFrom`

    :api-op:`GET /networks/{id}` for the network reports :api-field:`Scope` as ``local``, and
    reports :api-field:`Created`. It also reports the :api-field:`Driver`, :api-field:`Internal`,
    :api-field:`Attachable`, :api-field:`EnableIPv6` and :api-field:`Labels` given at create, and in
    :api-field:`IPAM.Config` the subnets given at create.

.. expectation:: Deleting by an ID prefix that several networks share responds 400
    :id: NET-API-DELETE-AMBIGUOUS-PREFIX
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows
    :refines: PRIN-AMBIGUITY-NOT-DESTRUCTIVE, REQ-NET-LOOKUP, REQ-NET-API-ERROR-STATUS

    *Applies to:* Two or more networks that are not Swarm-scoped, whose IDs start with a prefix that
    is not a network name

    :api-op:`DELETE /networks/{id}` with that prefix as ``{id}`` responds 400. :api-op:`GET
    /networks` then still lists each of the networks.

.. expectation:: docker network rm by an ID prefix that several networks share fails
    :id: NET-API-DELETE-AMBIGUOUS-PREFIX-CLI
    :observed_from: api
    :depends_on: cli
    :platforms: linux, windows
    :refines: PRIN-AMBIGUITY-NOT-DESTRUCTIVE, REQ-NET-LOOKUP
    :cli_for: NET-API-DELETE-AMBIGUOUS-PREFIX

    *Applies to:* Two or more networks that are not Swarm-scoped, whose IDs start with a prefix that
    is not a network name

    :cli:`docker network rm` with that prefix exits non-zero and prints the daemon's error.
    :cli:`docker network ls` then still lists each of the networks.

.. expectation:: The filter type=custom lists the networks that type=builtin does not
    :id: NET-API-FILTER-TYPE-CUSTOM
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-NET-LIST-FILTERS

    :api-op:`GET /networks` with the filter :api-query:`type=custom` lists the networks, other than
    the Swarm's predefined networks, that the filter :api-query:`type=builtin` does not list. It
    does not list a network that the filter :api-query:`type=builtin` lists.

.. expectation:: type=builtin lists host and none, and bridge while it exists
    :id: NET-API-FILTER-TYPE-BUILTIN-PREDEFINED
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :refines: REQ-NET-LIST-FILTERS

    :api-op:`GET /networks` with the filter :api-query:`type=builtin` lists :network:`host` and
    :network:`none`. It also lists :network:`bridge` while the default bridge network exists.

.. expectation:: The filter type=builtin lists no network but host, none and bridge
    :id: NET-API-FILTER-TYPE-BUILTIN
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :exact_form_of: NET-API-FILTER-TYPE-BUILTIN-PREDEFINED

    :api-op:`GET /networks` with the filter :api-query:`type=builtin` lists :network:`host` and
    :network:`none`, :network:`bridge` while the default bridge network exists, and no other
    network.

.. expectation:: On Windows, type=builtin lists none, and nat while it exists
    :id: NET-API-FILTER-TYPE-BUILTIN-PREDEFINED-WINDOWS
    :observed_from: api
    :depends_on: none
    :platforms: windows
    :refines: REQ-NET-LIST-FILTERS

    :api-op:`GET /networks` with the filter :api-query:`type=builtin` lists :network:`none`. It also
    lists :network:`nat` while a network named :network:`nat` exists.

.. expectation:: On Windows, the filter type=builtin lists no network but none and nat
    :id: NET-API-FILTER-TYPE-BUILTIN-WINDOWS
    :observed_from: api
    :depends_on: none
    :platforms: windows
    :exact_form_of: NET-API-FILTER-TYPE-BUILTIN-PREDEFINED-WINDOWS

    :api-op:`GET /networks` with the filter :api-query:`type=builtin` lists :network:`none`,
    :network:`nat` while a network named :network:`nat` exists, and no other network.

.. expectation:: An unknown filter term or a bad type or dangling value responds 400
    :id: NET-API-FILTER-INVALID
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-NET-LIST-FILTER-INVALID, REQ-NET-API-ERROR-STATUS

    :api-op:`GET /networks` with a filter term that it does not support responds 400. It also
    responds 400 for a value of the :api-query:`type` or :api-query:`dangling` term that the term
    does not accept, and for more than one value for :api-query:`dangling`.

.. expectation:: A well-formed filter that matches no network lists none
    :id: NET-API-FILTER-NO-MATCH
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows
    :rationale: RAT-NET-FILTER-NO-MATCH-NOT-REFUSED

    :api-op:`GET /networks` with a value of a supported filter term other than :api-query:`type` and
    :api-query:`dangling` that matches no network responds 200 and lists no network.

.. expectation:: docker network ls with an unsupported filter term or value fails
    :id: NET-API-FILTER-INVALID-CLI
    :observed_from: api
    :depends_on: cli
    :platforms: linux, windows
    :refines: REQ-NET-LIST-FILTER-INVALID
    :cli_for: NET-API-FILTER-INVALID

    :cli:`docker network ls --filter` exits non-zero and prints the daemon's error for a filter term
    that the daemon does not support. It does the same for a value of the :api-query:`type` or
    :api-query:`dangling` term that the term does not accept.

.. expectation:: The type and dangling filter terms each accept a fixed set of values
    :id: NET-API-FILTER-TYPE-VALUES
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-NET-LIST-FILTERS

    :api-op:`GET /networks` with :api-query:`type=builtin` or :api-query:`type=custom` responds 200.
    With :api-query:`dangling` set to one of ``true``, ``1``, ``false`` or ``0``, it responds 200.

.. expectation:: With both type values, the filter lists every network
    :id: NET-API-FILTER-TYPE-BOTH
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-NET-LIST-FILTERS

    :api-op:`GET /networks` with both :api-query:`type=builtin` and :api-query:`type=custom` lists
    every network other than the Swarm's predefined networks.

.. expectation:: The filter dangling=true lists a network with no running container
    :id: NET-API-FILTER-DANGLING-TRUE
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-NET-LIST-FILTERS

    *Applies to:* A network that is not Swarm-scoped and not predefined, with no running container
    attached to it, other than :network:`docker_gwbridge` on a Linux node in a Swarm that has an
    ingress network created with :api-field:`Ingress` set

    :api-op:`GET /networks` with the filter :api-query:`dangling=true` lists the network, and with
    the filter :api-query:`dangling=false` does not list it.

.. expectation:: The filter dangling=false lists a predefined or in-use network
    :id: NET-API-FILTER-DANGLING-FALSE
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-NET-LIST-FILTERS

    *Applies to:* A network that is not Swarm-scoped, and that is predefined or has a running
    container attached to it

    :api-op:`GET /networks` with the filter :api-query:`dangling=false` lists the network, and with
    the filter :api-query:`dangling=true` does not list it.

.. expectation:: Network prune deletes unused networks and lists their names
    :id: NET-API-PRUNE-UNUSED
    :observed_from: api:solo
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-NET-PRUNE-UNUSED, REQ-NET-PREDEFINED-RESERVED

    *Applies to:* A node that is not in a Swarm

    :api-op:`POST /networks/prune` responds 200. It deletes each network that is not Swarm-scoped,
    not predefined, not config-only and not adopted from the host, and that no container lists in
    :api-field:`NetworkSettings.Networks`. It does not delete a predefined network, or a network
    that a container lists in :api-field:`NetworkSettings.Networks`. On Windows nodes, it does not
    delete a network that the daemon adopted from the host: one with
    :drvopt:`com.docker.network.windowsshim.hnsowned` in its :api-field:`Options`, or one named
    ``Default Switch`` or ``WSL (Hyper-V firewall)``. :api-field:`NetworksDeleted` in the response
    lists the names of the networks that it deletes.

.. expectation:: docker network prune --force deletes unused networks and names them
    :id: NET-API-PRUNE-UNUSED-CLI
    :observed_from: api:solo
    :depends_on: cli
    :platforms: linux, windows
    :refines: REQ-NET-PRUNE-UNUSED
    :cli_for: NET-API-PRUNE-UNUSED

    *Applies to:* A node that is not in a Swarm

    :cli:`docker network prune --force` exits zero, and prints the names of the networks that it
    deletes. :cli:`docker network ls` then does not list those networks.

.. expectation:: Network prune deletes a network that only stopped containers list
    :id: NET-API-PRUNE-STOPPED-CTR-NETWORK
    :observed_from: api:solo
    :depends_on: none
    :platforms: linux, windows
    :narrows: NET-API-PRUNE-UNUSED
    :violates: REQ-NET-PRUNE-UNUSED

    *Applies to:* A node that is not in a Swarm, and a network that is not Swarm-scoped, not
    predefined and not config-only, that a created or stopped container lists in
    :api-field:`NetworkSettings.Networks`, and that no running container is attached to

    :api-op:`POST /networks/prune` deletes the network, and :api-field:`NetworksDeleted` lists its
    name. :api-op:`POST /containers/{id}/start` for the container then responds 404 while no network
    has that name.

.. expectation:: Network prune keeps a config-only network that nothing uses
    :id: NET-API-PRUNE-CONFIG-ONLY-KEPT
    :observed_from: api:solo
    :depends_on: none
    :platforms: linux, windows
    :concerns: NET-API-PRUNE-UNUSED
    :violates: REQ-NET-PRUNE-UNUSED

    *Applies to:* A node that is not in a Swarm, and a config-only network that no container lists
    in :api-field:`NetworkSettings.Networks` and that no network's :api-field:`ConfigFrom` names

    :api-op:`POST /networks/prune` responds 200, and does not delete the network.
    :api-field:`NetworksDeleted` does not list its name.

.. expectation:: Outside a Swarm, network prune deletes an unused docker_gwbridge
    :id: NET-API-PRUNE-GWBRIDGE-NO-SWARM
    :observed_from: api:solo
    :depends_on: none
    :platforms: linux
    :refines: REQ-NET-PRUNE-UNUSED

    *Applies to:* A node that is not in a Swarm, and a :network:`docker_gwbridge` network that no
    container lists in :api-field:`NetworkSettings.Networks`

    :api-op:`POST /networks/prune` deletes :network:`docker_gwbridge`. This includes a
    :network:`docker_gwbridge` that an operator creates before the node joins a Swarm, to configure
    the gateway network.

.. expectation:: A network created with ConfigOnly reports ConfigOnly true
    :id: NET-API-CONFIG-ONLY-CREATE
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-NET-CONFIG-ONLY

    :api-op:`POST /networks/create` with :api-field:`ConfigOnly` true, without
    :api-field:`ConfigFrom` and :api-field:`Scope`, and with :api-field:`Ingress`,
    :api-field:`Internal` and :api-field:`Attachable` false or unset, responds 201 when its
    :api-field:`Driver` is not :driver:`overlay` or the node is a manager. :api-op:`GET
    /networks/{id}` then reports :api-field:`ConfigOnly` true.

.. expectation:: A ConfigFrom network reports the config-only network's IPAM
    :id: NET-API-CONFIG-FROM-IPAM
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-NET-CONFIG-FROM

    *Applies to:* A network that is not Swarm-scoped, created with :api-field:`ConfigFrom` naming a
    config-only network

    :api-op:`GET /networks/{id}` for the network reports the config-only network's :api-field:`IPAM`
    configuration.

.. expectation:: A ConfigFrom network's containers get addresses in the config subnets
    :id: NET-API-CONFIG-FROM-ADDRESSES
    :observed_from: ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-NET-CONFIG-FROM

    *Applies to:* A network created with :api-field:`ConfigFrom` naming a config-only network

    The interface of a running container on the network has an address in a subnet of the
    config-only network's :api-field:`IPAM` configuration, for each IP version that the config-only
    network enables and that its configuration has a subnet for.

.. expectation:: Creating a network with a missing ConfigFrom network fails
    :id: NET-API-CONFIG-FROM-MISSING
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-NET-CONFIG-FROM, REQ-NET-API-ERROR-STATUS, REQ-NET-NETWORK-CREATE-ERRORS

    :api-op:`POST /networks/create` with :api-field:`ConfigFrom` naming a network that does not
    exist responds with a 4xx status. :api-op:`GET /networks` then does not list a network with the
    request's :api-field:`Name`.

.. expectation:: A missing ConfigFrom network responds 404
    :id: NET-API-CONFIG-FROM-MISSING-HTTP404
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows
    :exact_form_of: NET-API-CONFIG-FROM-MISSING

    :api-op:`POST /networks/create` with :api-field:`ConfigFrom` naming a network that does not
    exist responds 404, with a message that contains :api-error:`configuration network "<name>" does
    not exist`.

.. expectation:: docker network create --config-from with a missing network fails
    :id: NET-API-CONFIG-FROM-MISSING-CLI
    :observed_from: api
    :depends_on: cli
    :platforms: linux, windows
    :refines: REQ-NET-CONFIG-FROM
    :cli_for: NET-API-CONFIG-FROM-MISSING

    :cli:`docker network create --config-from` naming a network that does not exist exits non-zero
    and prints the daemon's error.

.. expectation:: Creating a network whose ConfigFrom is not config-only fails
    :id: NET-API-CONFIG-FROM-WRONG-KIND
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-NET-CONFIG-FROM, REQ-NET-API-ERROR-STATUS, REQ-NET-NETWORK-CREATE-ERRORS

    *Applies to:* A network that is not config-only

    :api-op:`POST /networks/create` with :api-field:`ConfigFrom` naming that network responds with a
    4xx status. :api-op:`GET /networks` then does not list a network with the request's
    :api-field:`Name`.

.. expectation:: A ConfigFrom that is not config-only is reported as missing
    :id: NET-API-CONFIG-FROM-WRONG-KIND-MISREPORTED
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows
    :concerns: NET-API-CONFIG-FROM-WRONG-KIND
    :violates: PRIN-ERRORS-ACTIONABLE

    *Applies to:* A network that is not config-only

    :api-op:`POST /networks/create` with :api-field:`ConfigFrom` naming that network responds 404,
    with a message that contains :api-error:`configuration network "<name>" does not exist`.

.. expectation:: Deleting a config-only network that a network uses is refused
    :id: NET-API-CONFIG-ONLY-DELETE-USED
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-NET-CONFIG-ONLY-DELETE-IN-USE, REQ-NET-API-ERROR-STATUS

    *Applies to:* A config-only network, and a network that is not Swarm-scoped whose
    :api-field:`ConfigFrom` names it

    :api-op:`DELETE /networks/{id}` for the config-only network responds with a 4xx status.
    :api-op:`GET /networks` then still lists the config-only network.

.. expectation:: Deleting a config-only network that a network uses responds 403
    :id: NET-API-CONFIG-ONLY-DELETE-USED-HTTP403
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows
    :exact_form_of: NET-API-CONFIG-ONLY-DELETE-USED

    *Applies to:* A config-only network, and a network that is not Swarm-scoped whose
    :api-field:`ConfigFrom` names it

    :api-op:`DELETE /networks/{id}` for the config-only network responds 403. :api-op:`GET
    /networks` then still lists the config-only network.

.. expectation:: docker network rm of a config-only network in use fails
    :id: NET-API-CONFIG-ONLY-DELETE-USED-CLI
    :observed_from: api
    :depends_on: cli
    :platforms: linux, windows
    :refines: REQ-NET-CONFIG-ONLY-DELETE-IN-USE
    :cli_for: NET-API-CONFIG-ONLY-DELETE-USED

    *Applies to:* A config-only network, and a network that is not Swarm-scoped whose
    :api-field:`ConfigFrom` names it

    :cli:`docker network rm` for the config-only network exits non-zero and prints the daemon's
    error.

.. expectation:: A container on a config-only network is created, and only its start is refused
    :id: NET-API-CONFIG-ONLY-CTR-CREATE
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows
    :concerns: NET-API-CONFIG-ONLY-START
    :violates: PRIN-FAIL-FAST

    *Applies to:* A config-only network

    :api-op:`POST /containers/create` for a container on that network responds 201. :api-op:`POST
    /containers/{id}/start` for the container then responds with an error.

.. expectation:: A stopped container connects to a config-only network, and only its start is refused
    :id: NET-API-CONFIG-ONLY-CONNECT-STOPPED
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows
    :concerns: NET-API-CONFIG-ONLY-START
    :violates: PRIN-FAIL-FAST

    *Applies to:* A config-only network, and a created or stopped container that is not attached to
    it

    :api-op:`POST /networks/{id}/connect` to that network, for the container, responds 200.
    :api-op:`POST /containers/{id}/start` for the container then responds with an error.

.. expectation:: A container on a config-only network does not start
    :id: NET-API-CONFIG-ONLY-START
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-NET-CONFIG-ONLY, REQ-NET-API-ERROR-STATUS, REQ-NET-START-FAILED-CONTAINER-KEPT

    *Applies to:* A created container without :api-field:`HostConfig.AutoRemove` on a config-only
    network

    :api-op:`POST /containers/{id}/start` for the container responds with a 4xx status. :api-op:`GET
    /containers/{id}/json` then reports :api-field:`State.Status` as ``created``.

.. expectation:: Starting a container on a config-only network responds 403
    :id: NET-API-CONFIG-ONLY-START-HTTP403
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows
    :exact_form_of: NET-API-CONFIG-ONLY-START

    *Applies to:* A created container on a config-only network

    :api-op:`POST /containers/{id}/start` for the container responds 403, with a message that
    contains :api-error:`cannot create endpoint on configuration-only network`.

.. expectation:: docker run -d on a config-only network fails
    :id: NET-API-CONFIG-ONLY-START-CLI
    :observed_from: api
    :depends_on: cli
    :platforms: linux, windows
    :refines: REQ-NET-CONFIG-ONLY
    :cli_for: NET-API-CONFIG-ONLY-START

    *Applies to:* A config-only network

    :cli:`docker run -d --network` naming that network exits non-zero and prints the daemon's error.

.. expectation:: Connecting a running container to a config-only network fails
    :id: NET-API-CONFIG-ONLY-CONNECT-RUNNING
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-NET-CONFIG-ONLY, REQ-NET-API-ERROR-STATUS

    *Applies to:* A running container, and a config-only network

    :api-op:`POST /networks/{id}/connect` to that network, for the container, responds with a 4xx
    status. :api-op:`GET /networks/{id}` then does not list the container.

.. expectation:: A running container's connect to a config-only network responds 403
    :id: NET-API-CONFIG-ONLY-CONNECT-RUNNING-HTTP403
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows
    :exact_form_of: NET-API-CONFIG-ONLY-CONNECT-RUNNING

    *Applies to:* A running container, and a config-only network

    :api-op:`POST /networks/{id}/connect` to that network, for the container, responds 403, with a
    message that contains :api-error:`cannot create endpoint on configuration-only network`.

.. expectation:: docker network connect of a running container to config-only fails
    :id: NET-API-CONFIG-ONLY-CONNECT-RUNNING-CLI
    :observed_from: api
    :depends_on: cli
    :platforms: linux, windows
    :refines: REQ-NET-CONFIG-ONLY
    :cli_for: NET-API-CONFIG-ONLY-CONNECT-RUNNING

    *Applies to:* A running container, and a config-only network

    :cli:`docker network connect` naming that network and the container exits non-zero and prints
    the daemon's error.

.. expectation:: A failed connect leaves the network out of the container's settings
    :id: NET-API-CONNECT-FAILED-NOT-LISTED
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-NET-CONNECT-FAILED-NO-TRACE, PRIN-NO-LEAKS

    *Applies to:* A running container, and a network that it is not attached to

    After :api-op:`POST /networks/{id}/connect` to that network, for the container, fails, whatever
    the reason, :api-op:`GET /containers/{id}/json` does not list the network in
    :api-field:`NetworkSettings.Networks`, and :api-op:`GET /networks/{id}` does not list the
    container. On Linux nodes, this includes a connect, at API version 1.54 or later, to an
    :driver:`ipvlan` network with :api-field:`MacAddress` set in the endpoint settings, which fails
    when the daemon creates the endpoint.

.. expectation:: A failed connect adds no interface to the container
    :id: NET-API-CONNECT-FAILED-NO-IFACE
    :observed_from: ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: PRIN-NO-LEAKS

    *Applies to:* A running container, and a network that it is not attached to

    After :api-op:`POST /networks/{id}/connect` to that network, for the container, fails, whatever
    the reason, the container has the same interfaces as before the request. On Linux nodes, this
    includes a connect whose :drvopt:`com.docker.network.endpoint.sysctls` sets a setting of the new
    interface that the kernel does not have, which fails after the daemon has added the interface to
    the container.

.. expectation:: A connect refused at the interface name leaves the interface in the container
    :id: NET-API-FAILED-CONNECT-IFACE-LEFT
    :observed_from: ctr, node
    :depends_on: none
    :platforms: linux
    :narrows: NET-API-CONNECT-FAILED-NO-IFACE
    :concerns: NET-API-CONNECT-FAILED-NOT-LISTED, NET-IFACE-NAME-REFUSED-CONNECT, NET-IFACE-NAME-CONFLICT-CONNECT
    :violates: PRIN-NO-LEAKS

    After :api-op:`POST /networks/{id}/connect` for a running container fails, with a
    :drvopt:`com.docker.network.endpoint.ifname` that the OS refuses or that names :netdev:`lo` or
    one of the container's interfaces, the interface stays in the container under the name that the
    driver generated for it. The interface is down and does not have addresses. It stays until the
    container's network namespace is destroyed. For a bridge network, the interface is a
    :link-kind:`veth`, and its peer stays attached to the network's bridge on the node until then.

.. expectation:: A container whose host port another one holds is created without a warning
    :id: NET-API-CTR-CREATE-HOST-PORT-HELD
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :concerns: NET-API-START-FAILED-NETWORKS-KEPT
    :violates: PRIN-WARN-AHEAD

    *Applies to:* A running container that publishes a host port

    :api-op:`POST /containers/create` for a container on a bridge network that publishes the same
    host port responds 201 without a warning about the host port.

.. expectation:: A start that fails at network setup keeps the container's networks
    :id: NET-API-START-FAILED-NETWORKS-KEPT
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :refines: REQ-NET-CONNECT-FAILED-NO-TRACE, PRIN-FAILS-CLOSED, REQ-NET-START-FAILED-CONTAINER-KEPT

    *Applies to:* A running container that publishes a host port, and a created container without
    :api-field:`HostConfig.AutoRemove` on a bridge network that publishes the same host port

    :api-op:`POST /containers/{id}/start` for the created container responds with a 4xx or 5xx
    status. :api-op:`GET /containers/{id}/json` then reports :api-field:`State.Status` as
    ``created``, and lists in :api-field:`NetworkSettings.Networks` each network that it listed
    before the start. A second :api-op:`POST /containers/{id}/start` for the container also responds
    with a 4xx or 5xx status.

.. expectation:: A failed start leaves an exited container exited
    :id: NET-API-START-FAILED-EXITED-KEPT
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :refines: REQ-NET-START-FAILED-CONTAINER-KEPT

    *Applies to:* A running container that publishes a host port, and an exited container without
    :api-field:`HostConfig.AutoRemove` on a bridge network that publishes the same host port

    :api-op:`POST /containers/{id}/start` for the exited container responds with a 4xx or 5xx
    status. :api-op:`GET /containers/{id}/json` then reports :api-field:`State.Status` as
    ``exited``.

.. expectation:: A container whose start failed starts once the cause is gone
    :id: NET-API-START-FAILED-STARTED-AGAIN
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :refines: REQ-NET-START-FAILED-CONTAINER-KEPT

    *Applies to:* A created container without :api-field:`HostConfig.AutoRemove` on a bridge network
    that publishes a host port, whose start failed while a running container published the same host
    port, and that container has stopped since

    :api-op:`POST /containers/{id}/start` for the created container responds 204. :api-op:`GET
    /containers/{id}/json` then reports :api-field:`State.Status` as ``running``.

.. expectation:: After a failed start, the container list reports no endpoint for it
    :id: NET-API-START-FAILED-ENDPOINT-NOT-LISTED
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :refines: REQ-NET-CONNECT-FAILED-NO-TRACE, PRIN-NO-LEAKS

    *Applies to:* A running container that publishes a host port, and a created container without
    :api-field:`HostConfig.AutoRemove` on a bridge network that publishes the same host port

    After :api-op:`POST /containers/{id}/start` for the created container fails, :api-op:`GET
    /containers/json` with :api-query:`all` set reports an empty :api-field:`EndpointID` and an
    empty :api-field:`IPAddress` for each of the container's networks.

.. _network-common-network-api-in-a-swarm:

In a Swarm
==========

.. expectation:: On a manager, the names bridge and host respond 409, the rest 403
    :id: NET-API-CREATE-PREDEFINED-NAME-MANAGER
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-NET-PREDEFINED-RESERVED, REQ-NET-NAME-UNIQUE, REQ-NET-RESERVED-NAMES, REQ-NET-API-ERROR-STATUS

    *Applies to:* A Linux manager

    :api-op:`POST /networks/create` with the name :network:`bridge` or :network:`host` responds 409.
    It responds 403 when the name is ``none`` or ``container``, or starts with ``container:``. This
    holds for the default driver and for :driver:`overlay`.

.. expectation:: In a Swarm, a connect to a missing network gives no warning, and start gets 500
    :id: NET-API-CONNECT-MISSING-SWARM-START-FAILS
    :observed_from: api:mgr, api:wkr
    :depends_on: none
    :platforms: linux, windows
    :pins: SWARM-CLUSTER-HTTP500-REFUSED
    :concerns: NET-API-CONNECT-MISSING-CREATED-LATER
    :violates: PRIN-WARN-AHEAD, REQ-NET-API-ERROR-STATUS

    *Applies to:* A node in a Swarm, and a created or stopped container

    :api-op:`POST /networks/{id}/connect` for the container, with an ``{id}`` that is not the name
    or an ID prefix of any network, responds 200 without a warning. :api-op:`POST
    /containers/{id}/start` for the container then responds 500, with a message that contains
    :api-error:`network <name> not found`. ``<name>`` is the ``{id}`` that the connect gave.

.. expectation:: On a manager, a name finds the Swarm-scoped network whose ID it prefixes
    :id: NET-API-NAME-SWARM-ID-PREFIX
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :narrows: NET-API-INSPECT-ID-NAME-PREFIX, NET-API-INSPECT-ID-NAME-PREFIX-CLI
    :violates: REQ-NET-LOOKUP

    *Applies to:* A manager, a network that is not Swarm-scoped, and exactly one Swarm-scoped
    network whose ID starts with that network's name, where no Swarm-scoped network has that name

    On that manager, :api-op:`GET /networks/{id}` with that name as ``{id}`` responds 200 with the
    Swarm-scoped network whose ID starts with it.

.. expectation:: On a manager, the name bridge or host returns the node's own network
    :id: NET-API-INSPECT-NAME-SWARM-PREDEFINED
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :rationale: RAT-SWARM-PREDEFINED-NETWORKS

    On a manager, :api-op:`GET /networks/{id}` with ``{id}`` :network:`bridge` or :network:`host`,
    and without :api-query:`scope`, responds 200 with the node's own network of that name.

.. expectation:: On a manager, an ID prefix shared with one Swarm-scoped network returns it
    :id: NET-API-INSPECT-AMBIGUOUS-PREFIX-SWARM-SCOPED
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :narrows: NET-API-INSPECT-AMBIGUOUS-PREFIX, NET-API-INSPECT-AMBIGUOUS-PREFIX-HTTP400
    :violates: PRIN-AMBIGUITY-NOT-DESTRUCTIVE

    *Applies to:* A manager, one Swarm-scoped network, and one or more networks that are not
    Swarm-scoped, whose IDs start with a prefix that is not a network name and that the ID of no
    other Swarm-scoped network starts with

    On that manager, :api-op:`GET /networks/{id}` with that prefix as ``{id}`` responds 200 with the
    Swarm-scoped network.

.. expectation:: A prefix shared with the Swarm's bridge or host returns the other network
    :id: NET-API-INSPECT-AMBIGUOUS-PREFIX-SWARM-PREDEFINED
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :narrows: NET-API-INSPECT-AMBIGUOUS-PREFIX, NET-API-INSPECT-AMBIGUOUS-PREFIX-HTTP400
    :violates: PRIN-AMBIGUITY-NOT-DESTRUCTIVE

    *Applies to:* A manager, one of the Swarm's predefined networks, and exactly one other
    Swarm-scoped network, whose IDs start with a prefix that is not a network name and that the ID
    of no network that is not Swarm-scoped starts with

    On that manager, :api-op:`GET /networks/{id}` with that prefix as ``{id}`` responds 200 with the
    other Swarm-scoped network.

.. expectation:: A prefix shared with the Swarm's bridge or host deletes the other network
    :id: NET-API-DELETE-AMBIGUOUS-PREFIX-SWARM-PREDEFINED
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :concerns: NET-API-DELETE-AMBIGUOUS-PREFIX, NET-API-DELETE-AMBIGUOUS-PREFIX-CLI
    :violates: REQ-NET-LOOKUP

    *Applies to:* A manager, one of the Swarm's predefined networks, and exactly one other network
    that no container or service uses, whose IDs start with a prefix that is not a network name

    On that manager, :api-op:`DELETE /networks/{id}` with that prefix as ``{id}`` responds 204, and
    deletes the other network.

.. expectation:: On a manager, dangling=true lists every Swarm-scoped network
    :id: NET-API-FILTER-DANGLING-SWARM-SCOPED
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :concerns: SWARM-SCOPED-LIST-DANGLING, NET-API-FILTER-DANGLING-TRUE, NET-API-FILTER-DANGLING-FALSE
    :violates: REQ-NET-LIST-FILTERS

    On a manager, :api-op:`GET /networks` with the filter :api-query:`dangling=true` lists every
    Swarm-scoped network other than the Swarm's predefined networks, in use or not. A Swarm-scoped
    network with a local attachment is listed for :api-query:`dangling=false` too.

.. expectation:: Network prune does not interrupt the routing mesh or the egress of tasks
    :id: NET-API-PRUNE-SWARM-MESH-EGRESS
    :observed_from: client, task → underlay
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-NET-PRUNE-UNUSED, REQ-SWARM-ROUTING-MESH
    :needs: SWARM-HOST-IPVS, SWARM-HOST-IPTABLES-COMMAND, SWARM-HOST-IPTABLES-LB-EXTENSIONS, SWARM-HOST-IPTABLES-LEGACY-MANGLE-KERNEL, BR-HOST-NO-FORWARD-DROP

    *Applies to:* A Linux node in a Swarm that has an ingress network created with
    :api-field:`Ingress` set, a service that publishes a port in ingress mode, and a task on the
    node that is attached to an overlay network that is not ``internal``, where the published port
    answers at the node's address and the task's traffic to the physical network reaches its
    destination

    :api-op:`POST /networks/prune` is sent to the node. The published port answers at the node's
    address throughout the prune and after it. Traffic that the task sends to the physical network
    reaches its destination throughout the prune and after it.

.. expectation:: In a Swarm with an ingress network, prune keeps docker_gwbridge
    :id: NET-API-PRUNE-GWBRIDGE-SWARM
    :observed_from: api:mgr, api:wkr
    :depends_on: none
    :platforms: linux
    :exact_form_of: NET-API-PRUNE-SWARM-MESH-EGRESS
    :rationale: RAT-NET-INGRESS-SBOX-ON-GWBRIDGE

    *Applies to:* A Linux node in a Swarm whose :api-op:`GET /networks/docker_gwbridge` lists
    ``ingress-sbox`` in :api-field:`Containers`

    :api-op:`POST /networks/prune` does not delete :network:`docker_gwbridge`, also when
    ``ingress-sbox`` is the only entry in its :api-field:`Containers`.

.. expectation:: After init or join with an ingress network, docker_gwbridge lists ingress-sbox
    :id: NET-API-GWBRIDGE-INGRESS-SBOX
    :observed_from: api:mgr, api:wkr
    :depends_on: none
    :platforms: linux
    :within: SWARM-CEIL-REJOIN
    :rationale: RAT-NET-INGRESS-SBOX-ON-GWBRIDGE

    *Applies to:* A Linux node that is not in a Swarm

    Within the :need:`[[title|lower]] ceiling ([[seconds]] s) <SWARM-CEIL-REJOIN>` after
    :api-op:`POST /swarm/init` responds 200 on the node, or after :api-op:`POST /swarm/join`
    responds 200 on the node for a Swarm that has an ingress network created with
    :api-field:`Ingress` set, :api-op:`GET /networks/docker_gwbridge` lists ``ingress-sbox`` in
    :api-field:`Containers`.

.. expectation:: A service on a config-only network is refused
    :id: NET-API-CONFIG-ONLY-SERVICE
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :refines: REQ-NET-CONFIG-ONLY, REQ-NET-API-ERROR-STATUS

    *Applies to:* A manager with a config-only network

    :api-op:`POST /services/create` with that network in :api-field:`TaskTemplate.Networks` responds
    with a 4xx status. :api-op:`GET /services` then does not list a service with the request's
    :api-field:`Name`.

.. expectation:: A service on a config-only network responds 403
    :id: NET-API-CONFIG-ONLY-SERVICE-HTTP403
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux
    :exact_form_of: NET-API-CONFIG-ONLY-SERVICE

    *Applies to:* A manager with a config-only network

    :api-op:`POST /services/create` with that network in :api-field:`TaskTemplate.Networks` responds
    403 with a message that contains :api-error:`cannot be used with services`.

.. _network-common-endpoint-interfaces:

*********************
 Endpoint interfaces
*********************

The expectations in this section about an endpoint hold for an endpoint of any network driver that
adds an interface to the container (:ref:`network-common-driver-properties`). An endpoint requests
an interface name when its :drvopt:`com.docker.network.endpoint.ifname` is set to that name. The
section also holds the macvlan driver's expectations about its :drvopt:`parent` interface.

.. expectation:: An endpoint's interface has the name that it requests
    :id: NET-IFACE-NAME-REQUESTED
    :observed_from: task, ctr
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-NET-IFACE-NAME
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* An endpoint that requests an interface name, on a container none of whose
    endpoints that request no name is on a network whose interface prefix (``eth``, or the value of
    its :drvopt:`com.docker.network.container_iface_prefix`), followed by a number, is that name

    The interface has that name.

.. expectation:: Whether a start succeeds depends on the order of attaching endpoints
    :id: NET-IFACE-NAME-COLLISION-ORDER
    :observed_from: api, ctr
    :depends_on: os-linux
    :platforms: linux
    :concerns: NET-IFACE-NAME-REQUESTED
    :violates: PRIN-REPEATABLE

    *Applies to:* Linux nodes

    At API version 1.44 or later, :api-op:`POST /containers/create` for a container with two
    endpoints on networks of drivers that add an interface, one that requests the interface name
    :netdev:`eth0` and one that requests no name, responds 201. Repeated :api-op:`POST
    /containers/{id}/start` for the container, each after the container stops, give both outcomes:
    some respond 204 with the endpoint that requests :netdev:`eth0` named :netdev:`eth0`, and some
    respond with an error that the name is in use. This holds whatever the names of the container
    and of its networks.

.. expectation:: A requested name with %d is accepted
    :id: NET-IFACE-NAME-PATTERN
    :observed_from: api, ctr
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-NET-API-BACKWARD-COMPATIBLE

    *Applies to:* Linux nodes with an endpoint that requests an interface name that contains ``%d``

    :api-op:`POST /containers/{id}/start` for a created container with such an endpoint responds
    204, and :api-op:`POST /networks/{id}/connect` for a running container with such an endpoint
    responds 200. The container then has an interface on the network.

.. expectation:: A requested name with %d is accepted, and the kernel numbers it
    :id: NET-IFACE-NAME-KERNEL-REWRITE
    :observed_from: api, ctr
    :depends_on: os-linux
    :platforms: linux
    :narrows: NET-IFACE-NAME-REQUESTED
    :concerns: NET-IFACE-NAME-PATTERN
    :violates: PRIN-FAILS-CLOSED, REQ-NET-IFACE-NAME

    :api-op:`POST /containers/{id}/start` for a created container with an endpoint that requests an
    interface name that contains ``%d`` responds 204, and :api-op:`POST /networks/{id}/connect` for
    a running container with such an endpoint responds 200. The kernel gives the interface the first
    free name that matches, such as :netdev:`eth0`.

.. expectation:: On Windows, a container whose endpoint requests an interface name starts
    :id: NET-IFACE-NAME-REQUESTED-WINDOWS
    :observed_from: api
    :depends_on: os-windows
    :platforms: windows
    :refines: REQ-NET-API-BACKWARD-COMPATIBLE

    *Applies to:* Windows nodes with a created or stopped container with an endpoint that requests
    an interface name

    :api-op:`POST /containers/{id}/start` for the container responds 204. :api-op:`GET
    /containers/{id}/json` then reports :api-field:`State.Running` true, and a non-empty
    :api-field:`EndpointID` for the network.

.. expectation:: On Windows, the interface does not get the requested ifname
    :id: NET-IFACE-WINDOWS-NAME-IGNORED
    :observed_from: ctr
    :depends_on: none
    :platforms: windows
    :concerns: NET-IFACE-NAME-REQUESTED-WINDOWS, NET-IFACE-NAME-REQUESTED
    :violates: PRIN-FAILS-CLOSED

    *Applies to:* A Windows container with an endpoint whose
    :drvopt:`com.docker.network.endpoint.ifname` names a name other than the one that Windows gives
    the interface

    The container's interface on that network does not have the requested name.

.. expectation:: An interface without a requested name is named eth<N>
    :id: NET-IFACE-NAME-DEFAULT
    :observed_from: task, ctr
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-NET-IFACE-NAME-DEFAULT
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* Linux nodes with an endpoint that does not request an interface name, on a network
    of a built-in driver without :drvopt:`com.docker.network.container_iface_prefix`

    The interface name is :netdev:`eth<N>`. When the endpoint is the container's only endpoint, the
    name is :netdev:`eth0`.

.. expectation:: A container whose requested interface name is refused does not start
    :id: NET-IFACE-NAME-REFUSED-START
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :refines: PRIN-FAILS-CLOSED

    *Applies to:* Linux nodes with a created or stopped container with an endpoint whose
    :drvopt:`com.docker.network.endpoint.ifname` the OS refuses

    :api-op:`POST /containers/{id}/start` for the container responds with a 4xx or 5xx status.

.. expectation:: A connect whose requested interface name is refused fails
    :id: NET-IFACE-NAME-REFUSED-CONNECT
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :refines: PRIN-FAILS-CLOSED

    *Applies to:* Linux nodes with a running container

    :api-op:`POST /networks/{id}/connect` for the container, with an endpoint whose
    :drvopt:`com.docker.network.endpoint.ifname` the OS refuses, responds with a 4xx or 5xx status.

.. expectation:: Starting with a refused interface name responds 500
    :id: NET-IFACE-NAME-REFUSED-START-HTTP500
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :exact_form_of: NET-IFACE-NAME-REFUSED-START
    :pins: NET-IFACE-HTTP500-REFUSED

    *Applies to:* Linux nodes with a created or stopped container with an endpoint whose
    :drvopt:`com.docker.network.endpoint.ifname` the OS refuses

    :api-op:`POST /containers/{id}/start` for the container responds 500, with a message that
    contains :api-error:`error renaming interface` and the requested name.

.. expectation:: A connect with a refused interface name responds 500
    :id: NET-IFACE-NAME-REFUSED-CONNECT-HTTP500
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :exact_form_of: NET-IFACE-NAME-REFUSED-CONNECT
    :pins: NET-IFACE-HTTP500-REFUSED

    *Applies to:* Linux nodes with a running container

    :api-op:`POST /networks/{id}/connect` for the container, with an endpoint whose
    :drvopt:`com.docker.network.endpoint.ifname` the OS refuses, responds 500. The message contains
    :api-error:`error renaming interface` and the requested name.

.. expectation:: docker run -d with a refused interface name fails
    :id: NET-IFACE-NAME-REFUSED-START-CLI
    :observed_from: api
    :depends_on: cli
    :platforms: linux
    :refines: PRIN-FAILS-CLOSED
    :cli_for: NET-IFACE-NAME-REFUSED-START

    :cli:`docker run -d` with a :drvopt:`com.docker.network.endpoint.ifname` that the OS refuses
    exits non-zero and prints the daemon's error.

.. expectation:: An interface name that the OS refuses is not checked at create or connect
    :id: NET-IFACE-NAME-REFUSED-UNCHECKED
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :concerns: NET-IFACE-NAME-REFUSED-START
    :violates: PRIN-FAIL-FAST

    :api-op:`POST /containers/create` for a container with an endpoint whose
    :drvopt:`com.docker.network.endpoint.ifname` has 16 characters or more responds 201.
    :api-op:`POST /networks/{id}/connect` for a created or stopped container, with such an endpoint,
    responds 200. Only :api-op:`POST /containers/{id}/start` for the container refuses the endpoint.

.. expectation:: docker network connect with a refused interface name fails
    :id: NET-IFACE-NAME-REFUSED-CONNECT-CLI
    :observed_from: api
    :depends_on: cli
    :platforms: linux
    :refines: PRIN-FAILS-CLOSED
    :cli_for: NET-IFACE-NAME-REFUSED-CONNECT

    *Applies to:* Linux nodes with a running container

    :cli:`docker network connect` for the container, with a
    :drvopt:`com.docker.network.endpoint.ifname` that the OS refuses, exits non-zero and prints the
    daemon's error.

.. expectation:: A container whose endpoints request one name, or lo, does not start
    :id: NET-IFACE-NAME-CONFLICT-START
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :refines: PRIN-FAILS-CLOSED

    *Applies to:* Linux nodes with a created or stopped container with two endpoints that request
    the same interface name, or with an endpoint that requests :netdev:`lo`

    :api-op:`POST /containers/{id}/start` for the container responds with a 4xx or 5xx status.

.. expectation:: A connect requesting lo or an existing interface name fails
    :id: NET-IFACE-NAME-CONFLICT-CONNECT
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :refines: PRIN-FAILS-CLOSED

    *Applies to:* Linux nodes with a running container

    :api-op:`POST /networks/{id}/connect` for the container, with an endpoint that requests
    :netdev:`lo`, responds with a 4xx or 5xx status. It does the same with an endpoint that requests
    the name of one of the container's interfaces.

.. expectation:: A duplicate interface name or lo at start responds 500
    :id: NET-IFACE-NAME-CONFLICT-START-HTTP500
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :exact_form_of: NET-IFACE-NAME-CONFLICT-START
    :pins: NET-IFACE-HTTP500-REFUSED

    *Applies to:* Linux nodes with a created or stopped container with two endpoints that request
    the same interface name, or with one that requests :netdev:`lo`

    :api-op:`POST /containers/{id}/start` for the container responds 500, with a message that names
    the interface.

.. expectation:: A connect requesting lo or an existing name responds 500
    :id: NET-IFACE-NAME-CONFLICT-CONNECT-HTTP500
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :exact_form_of: NET-IFACE-NAME-CONFLICT-CONNECT
    :pins: NET-IFACE-HTTP500-REFUSED

    *Applies to:* Linux nodes with a running container

    :api-op:`POST /networks/{id}/connect` for the container, with an endpoint that requests
    :netdev:`lo` or the name of one of its interfaces, responds 500. The message names the
    interface.

.. expectation:: docker run -d with a duplicate interface name or lo fails
    :id: NET-IFACE-NAME-CONFLICT-START-CLI
    :observed_from: api
    :depends_on: cli
    :platforms: linux
    :refines: PRIN-FAILS-CLOSED
    :cli_for: NET-IFACE-NAME-CONFLICT-START

    :cli:`docker run -d` exits non-zero and prints the daemon's error when two of the container's
    endpoints request the same interface name. It does the same when an endpoint requests
    :netdev:`lo`.

.. expectation:: Interface-name conflicts are not checked at container create
    :id: NET-IFACE-NAME-CONFLICT-UNCHECKED
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :concerns: NET-IFACE-NAME-CONFLICT-START
    :violates: PRIN-FAIL-FAST

    At API version 1.44 or later, :api-op:`POST /containers/create` for a container with two
    endpoints that request the same interface name responds 201. So does a request for a container
    with an endpoint that requests :netdev:`lo`. Only :api-op:`POST /containers/{id}/start` refuses
    the container.

.. expectation:: docker network connect requesting lo or an existing name fails
    :id: NET-IFACE-NAME-CONFLICT-CONNECT-CLI
    :observed_from: api
    :depends_on: cli
    :platforms: linux
    :refines: PRIN-FAILS-CLOSED
    :cli_for: NET-IFACE-NAME-CONFLICT-CONNECT

    *Applies to:* Linux nodes with a running container

    :cli:`docker network connect` for the container exits non-zero and prints the daemon's error
    when the endpoint requests :netdev:`lo` or the name of one of its interfaces.

.. expectation:: Refused disconnect and ifname requests respond 500
    :id: NET-IFACE-HTTP500-REFUSED
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows
    :concerns: NET-API-DISCONNECT-NOT-ATTACHED, NET-API-DISCONNECT-NOT-ATTACHED-HTTP500, NET-IFACE-NAME-REFUSED-CONNECT, NET-IFACE-NAME-REFUSED-START, NET-IFACE-NAME-CONFLICT-CONNECT, NET-IFACE-NAME-CONFLICT-START, NET-IFACE-NAME-REFUSED-CONNECT-HTTP500, NET-IFACE-NAME-REFUSED-START-HTTP500, NET-IFACE-NAME-CONFLICT-CONNECT-HTTP500, NET-IFACE-NAME-CONFLICT-START-HTTP500
    :violates: REQ-NET-API-ERROR-STATUS

    Each request below is refused, and responds 500: (1) :api-op:`POST /networks/{id}/disconnect`
    for a container that is not attached to the network. (2) On a Linux node, :api-op:`POST
    /containers/{id}/start` and :api-op:`POST /networks/{id}/connect` for an endpoint with a
    :drvopt:`com.docker.network.endpoint.ifname` that the OS refuses, that is :netdev:`lo`, or that
    is the name of an interface that the container already has or that another of its endpoints
    requests.

.. expectation:: A standalone container's interface has its requested static address
    :id: NET-IFACE-STATIC-ADDRESS
    :observed_from: ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-NET-STATIC-ADDRESS

    *Applies to:* A running standalone container whose endpoint on a network has an
    :api-field:`IPv4Address` or :api-field:`IPv6Address` in :api-field:`IPAMConfig`

    Its interface on that network has that address.

.. expectation:: A requested static address is reported in the container's settings
    :id: NET-IFACE-STATIC-ADDRESS-INSPECT
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-NET-STATIC-ADDRESS

    *Applies to:* A running standalone container whose endpoint on a network has an
    :api-field:`IPv4Address` or :api-field:`IPv6Address` in :api-field:`IPAMConfig`

    :api-op:`GET /containers/{id}/json` reports the address as :api-field:`IPAddress` or
    :api-field:`GlobalIPv6Address` in :api-field:`NetworkSettings.Networks.<network>`.

.. expectation:: A standalone container gets its static address again when it starts again
    :id: NET-IFACE-STATIC-ADDRESS-RESTART
    :observed_from: ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-NET-STATIC-ADDRESS

    *Applies to:* A stopped standalone container that has run, whose endpoint on a network has an
    :api-field:`IPv4Address` or :api-field:`IPv6Address` in :api-field:`IPAMConfig`, while no other
    endpoint on the network has that address

    After :api-op:`POST /containers/{id}/start` for the container responds 204, its interface on
    that network has that address.

.. expectation:: With IPv6 disabled on its interface, a static IPv6 address is dropped
    :id: NET-IFACE-STATIC-ADDRESS-IPV6-DISABLED
    :observed_from: api, ctr
    :depends_on: os-linux
    :platforms: linux
    :narrows: NET-IFACE-STATIC-ADDRESS, NET-IFACE-STATIC-ADDRESS-INSPECT, NET-IFACE-STATIC-ADDRESS-RESTART
    :violates: REQ-NET-STATIC-ADDRESS

    *Applies to:* A created or stopped standalone container whose endpoint on a network has an
    :api-field:`IPv6Address` in :api-field:`IPAMConfig`, and that has
    :sysctl:`net.ipv6.conf.all.disable_ipv6` set to 1 in :api-field:`HostConfig.Sysctls`, or
    ``net.ipv6.conf.IFNAME.disable_ipv6=1`` in that endpoint's
    :drvopt:`com.docker.network.endpoint.sysctls`

    :api-op:`POST /containers/{id}/start` for the container responds 204. Its interface on that
    network then has no IPv6 address, and :api-op:`GET /containers/{id}/json` reports no
    :api-field:`GlobalIPv6Address` in :api-field:`NetworkSettings.Networks.<network>`.

.. expectation:: A container's interface has the link-local addresses that it requests
    :id: NET-IFACE-LINK-LOCAL-ADDRESS
    :observed_from: ctr
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-NET-LINK-LOCAL-ADDRESS

    *Applies to:* Linux nodes with a running container whose endpoint on a network has addresses in
    :api-field:`IPAMConfig.LinkLocalIPs`, from its create or from a later connect. An IPv6 address
    among them is for a network with :api-field:`EnableIPv6` true.

    The container's interface on that network has each of those addresses.

.. expectation:: docker run and docker network connect apply --link-local-ip
    :id: NET-IFACE-LINK-LOCAL-ADDRESS-CLI
    :observed_from: api, ctr
    :depends_on: cli, os-linux
    :platforms: linux
    :refines: REQ-NET-LINK-LOCAL-ADDRESS
    :cli_for: NET-IFACE-LINK-LOCAL-ADDRESS

    *Applies to:* Linux nodes with a network that is not Swarm-scoped, and an address in
    ``169.254.0.0/16``, or in ``fe80::/10`` when the network has IPv6 enabled

    :cli:`docker run -d` with :cli-flag:`--network` naming the network and
    :cli-flag:`--link-local-ip` set to the address exits zero. The container's interface on the
    network then has the address. :cli:`docker network connect` for a running container, with
    :cli-flag:`--link-local-ip` set to the address, does the same.

.. expectation:: An endpoint MacAddress set at create is the interface's MAC address
    :id: NET-IFACE-MAC-ADDRESS-CREATE
    :observed_from: ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-NET-ENDPOINT-MAC-ADDRESS
    :assumes: NET-DRVPROP-ADDS-INTERFACE, NET-DRVPROP-ENDPOINT-MAC

    *Applies to:* A running container created at API version 1.44 or later with an endpoint
    :api-field:`MacAddress` on a network whose driver accepts an endpoint MAC address

    Its interface on that network has that MAC address.

.. expectation:: An endpoint MacAddress set at connect is the interface's MAC address
    :id: NET-IFACE-MAC-ADDRESS-CONNECT
    :observed_from: ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-NET-ENDPOINT-MAC-ADDRESS
    :assumes: NET-DRVPROP-ADDS-INTERFACE, NET-DRVPROP-ENDPOINT-MAC

    *Applies to:* A running container, and a network whose driver accepts an endpoint MAC address

    After :api-op:`POST /networks/{id}/connect` with :api-field:`MacAddress` set, at API version
    1.54 or later, responds 200, the container's interface on the network has that MAC address.

.. expectation:: Before API 1.54, a connect ignores MacAddress
    :id: NET-IFACE-MAC-ADDRESS-CONNECT-API-VERSION
    :observed_from: ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-NET-ENDPOINT-MAC-ADDRESS, REQ-NET-API-BACKWARD-COMPATIBLE
    :assumes: NET-DRVPROP-ADDS-INTERFACE, NET-DRVPROP-ENDPOINT-MAC

    *Applies to:* A running container, and a network whose driver accepts an endpoint MAC address,
    and a :api-field:`MacAddress` other than the one that the driver gives the interface when no
    :api-field:`MacAddress` is set

    At API versions before 1.54, after :api-op:`POST /networks/{id}/connect` with
    :api-field:`MacAddress` set responds 200, the container's interface on the network does not have
    that MAC address.

.. expectation:: Before API 1.44, a create ignores an endpoint's MacAddress
    :id: NET-IFACE-MAC-ADDRESS-CREATE-API-VERSION
    :observed_from: api, ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-NET-ENDPOINT-MAC-ADDRESS, REQ-NET-API-BACKWARD-COMPATIBLE
    :assumes: NET-DRVPROP-ADDS-INTERFACE, NET-DRVPROP-ENDPOINT-MAC

    *Applies to:* A network whose driver accepts an endpoint MAC address, and a
    :api-field:`MacAddress` other than the one that the driver gives the interface when no
    :api-field:`MacAddress` is set

    At API versions before 1.44, :api-op:`POST /containers/create` that sets :api-field:`MacAddress`
    in the endpoint settings of that network, and does not set the container-wide
    :api-field:`MacAddress`, responds 201. Once the container runs, its interface on the network
    does not have that MAC address.

.. expectation:: A connect with MacAddress fails where the driver does not accept it
    :id: NET-IFACE-MAC-ADDRESS-REFUSED-CONNECT
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows
    :refines: PRIN-FAILS-CLOSED
    :assumes: NET-DRVPROP-ENDPOINT-MAC

    *Applies to:* A running container, and a network whose driver does not accept an endpoint MAC
    address

    :api-op:`POST /networks/{id}/connect` to that network, for the container, with
    :api-field:`MacAddress` set, at API version 1.54 or later, responds with a 4xx or 5xx status.
    :api-op:`GET /networks/{id}` then does not list the container.

.. expectation:: A container with a MacAddress its driver refuses does not start
    :id: NET-IFACE-MAC-ADDRESS-REFUSED-START
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows
    :refines: PRIN-FAILS-CLOSED, REQ-NET-START-FAILED-CONTAINER-KEPT
    :assumes: NET-DRVPROP-ENDPOINT-MAC

    *Applies to:* A network whose driver does not accept an endpoint MAC address, and a created
    container without :api-field:`HostConfig.AutoRemove`, created at API version 1.44 or later,
    whose endpoint on that network sets :api-field:`MacAddress`

    :api-op:`POST /containers/{id}/start` for the container responds with a 4xx or 5xx status.
    :api-op:`GET /containers/{id}/json` then reports :api-field:`State.Status` as ``created``.

.. expectation:: docker run -d with --mac-address fails where the driver refuses it
    :id: NET-IFACE-MAC-ADDRESS-REFUSED-START-CLI
    :observed_from: api
    :depends_on: cli
    :platforms: linux, windows
    :refines: PRIN-FAILS-CLOSED
    :cli_for: NET-IFACE-MAC-ADDRESS-REFUSED-START
    :assumes: NET-DRVPROP-ENDPOINT-MAC

    :cli:`docker run -d` with :cli-flag:`--mac-address` and with :cli-flag:`--network` naming a
    network whose driver does not accept an endpoint MAC address exits non-zero and prints the
    daemon's error.

.. expectation:: Creating a macvlan network on a missing VLAN parent creates the sub-interface
    :id: NET-IFACE-MACVLAN-VLAN-PARENT-CREATED
    :observed_from: api, node
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-NET-MACVLAN-VLAN-PARENT
    :needs: NET-HOST-VLAN

    *Applies to:* An interface ``<interface>`` on the node other than :netdev:`lo`, and a VLAN ID
    from 1 to 4094 that no 802.1Q VLAN link on ``<interface>`` has, where ``<interface>.<VLAN ID>``
    has at most 15 characters, is not an interface of the node, and is not the :drvopt:`parent` of
    another :driver:`macvlan` network

    :api-op:`POST /networks/create` for a :driver:`macvlan` network that is not Swarm-scoped, that
    uses the default IPAM driver, whose :drvopt:`parent` is ``<interface>.<VLAN ID>``, and whose
    :drvopt:`macvlan_mode` is unset, empty, ``bridge``, ``private``, ``passthru`` or ``vepa``,
    responds 201. The node then has an interface ``<interface>.<VLAN ID>``: a VLAN link on
    ``<interface>`` with that VLAN ID.

.. expectation:: Deleting a macvlan network deletes the VLAN parent that its create added
    :id: NET-IFACE-MACVLAN-VLAN-PARENT-REMOVED
    :observed_from: api, node
    :depends_on: os-linux
    :platforms: linux
    :refines: PRIN-NO-LEAKS
    :needs: NET-HOST-VLAN

    *Applies to:* A :driver:`macvlan` network that is not Swarm-scoped, without containers, whose
    create added its :drvopt:`parent` ``<interface>.<VLAN ID>``, where no other :driver:`macvlan`
    network and no :driver:`ipvlan` network on the node, including a Swarm-scoped one that the node
    has a local attachment to, has that :drvopt:`parent`

    :api-op:`DELETE /networks/{id}` for the network responds 204. The node then does not have the
    interface ``<interface>.<VLAN ID>``.

.. expectation:: Deleting a macvlan network removes a VLAN parent an ipvlan network uses
    :id: NET-IFACE-MACVLAN-VLAN-PARENT-REMOVED-IPVLAN
    :observed_from: api, node, ctr
    :depends_on: os-linux
    :platforms: linux
    :needs: NET-HOST-VLAN, IPVL-HOST-VLAN, IPVL-HOST-IPVLAN
    :concerns: NET-IFACE-MACVLAN-VLAN-PARENT-REMOVED
    :violates: PRIN-FAITHFUL

    *Applies to:* A :driver:`macvlan` network that is not Swarm-scoped, without containers, whose
    create added its :drvopt:`parent` ``<interface>.<VLAN ID>``, where no other :driver:`macvlan`
    network on the node has that :drvopt:`parent`, and an :driver:`ipvlan` network that is not
    Swarm-scoped, with that :drvopt:`parent` and a running container

    :api-op:`DELETE /networks/{id}` for the macvlan network responds 204. The node then does not
    have the interface ``<interface>.<VLAN ID>``, and the container has no interface on the ipvlan
    network, while :api-op:`GET /networks/{id}` for the ipvlan network still lists the container.

.. expectation:: Deleting a macvlan network keeps a VLAN parent that existed before it
    :id: NET-IFACE-MACVLAN-VLAN-PARENT-KEPT
    :observed_from: api, node
    :depends_on: none
    :platforms: linux
    :refines: PRIN-OWNS-ONLY-ITS-OWN
    :needs: NET-HOST-VLAN

    *Applies to:* A :driver:`macvlan` network that is not Swarm-scoped, without containers, whose
    :drvopt:`parent` is a VLAN sub-interface ``<interface>.<VLAN ID>`` that existed before the
    network was created and that no :driver:`macvlan` network's create added

    :api-op:`DELETE /networks/{id}` for the network responds 204. The node then still has the
    interface ``<interface>.<VLAN ID>``.

.. expectation:: Deleting a macvlan network without a parent removes what its create added
    :id: NET-IFACE-MACVLAN-NO-PARENT-REMOVED
    :observed_from: api, node
    :depends_on: os-linux
    :platforms: linux
    :refines: PRIN-NO-LEAKS
    :needs: NET-HOST-DUMMY

    *Applies to:* A :driver:`macvlan` network that is not Swarm-scoped, created without a
    :drvopt:`parent`, without containers

    :api-op:`DELETE /networks/{id}` for the network responds 204. The node then has no interface
    that the network's create added.

.. expectation:: A container on a macvlan network without a parent reaches nothing outside it
    :id: NET-IFACE-MACVLAN-NO-PARENT-ISOLATED
    :observed_from: ctr
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-NET-MACVLAN-NO-PARENT-INTERNAL
    :needs: NET-HOST-MACVLAN, NET-HOST-DUMMY

    *Applies to:* A container attached only to a :driver:`macvlan` network that was created without
    a :drvopt:`parent`

    The container cannot reach addresses outside that network, including every address of the node.

.. expectation:: Containers on a macvlan network without a parent reach each other
    :id: NET-IFACE-MACVLAN-NO-PARENT-SAME-NETWORK
    :observed_from: ctr → ctr
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-NET-MACVLAN-NO-PARENT-INTERNAL
    :needs: NET-HOST-MACVLAN, NET-HOST-DUMMY

    *Applies to:* Two containers attached to a :driver:`macvlan` network that was created without a
    :drvopt:`parent`, with :drvopt:`macvlan_mode` unset, empty or ``bridge``

    Each container reaches the other at that container's address on the network.

.. expectation:: Without macvlan links, starting a macvlan container responds 500
    :id: NET-IFACE-NO-MACVLAN
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :pins: NET-HOST-MACVLAN

    *Applies to:* A Linux host whose kernel does not offer macvlan links, and a created or stopped
    container with an endpoint on a :driver:`macvlan` network

    :api-op:`POST /containers/{id}/start` for the container responds 500, with a message that
    contains :api-error:`failed to create the macvlan port`.

.. expectation:: Without VLAN links, creating a macvlan network on a VLAN parent responds 500
    :id: NET-IFACE-NO-VLAN
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :pins: NET-HOST-VLAN

    *Applies to:* A Linux host whose kernel does not offer VLAN links, an existing interface
    ``<interface>``, and a VLAN ID from 1 to 4094, where ``<interface>.<VLAN ID>`` is not an
    interface of the node and is not the :drvopt:`parent` of another :driver:`macvlan` network

    :api-op:`POST /networks/create` for a :driver:`macvlan` network that is not Swarm-scoped, that
    uses the default IPAM driver, whose :drvopt:`parent` is ``<interface>.<VLAN ID>``, and whose
    :drvopt:`macvlan_mode` is unset, empty, ``bridge``, ``private``, ``passthru`` or ``vepa``,
    responds 500, with a message that contains :api-error:`vlan link`.

.. expectation:: A macvlan VLAN parent on an interface whose name has a dot responds 500
    :id: NET-IFACE-STACKED-VLAN-PARENT-REFUSED
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :narrows: NET-IFACE-NO-VLAN, NET-IFACE-MACVLAN-VLAN-PARENT-CREATED
    :violates: REQ-NET-API-ERROR-STATUS, REQ-NET-MACVLAN-VLAN-PARENT

    *Applies to:* An interface ``<interface>`` of the node whose name contains a dot, such as a VLAN
    sub-interface, and a name ``<interface>.<VLAN ID>`` that is not an interface of the node and is
    not the :drvopt:`parent` of another :driver:`macvlan` network

    :api-op:`POST /networks/create` for a :driver:`macvlan` network that is not Swarm-scoped, that
    uses the default IPAM driver, whose :drvopt:`parent` is ``<interface>.<VLAN ID>``, and whose
    :drvopt:`macvlan_mode` is unset, empty, ``bridge``, ``private``, ``passthru`` or ``vepa``,
    responds 500, with a message that contains :api-error:`required interface name format is:
    name.vlan_id`. :api-op:`GET /networks` then does not list a network with the request's
    :api-field:`Name`.

.. _network-common-default-gateway:

*****************
 Default gateway
*****************

.. expectation:: A container on internal networks only cannot reach addresses outside them
    :id: NET-GW-INTERNAL-ONLY-NO-EXTERNAL-ACCESS
    :observed_from: task, ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :refines: REQ-NET-INTERNAL-NO-EXTERNAL-ACCESS
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A container attached only to ``internal`` networks, whose routes are the routes
    that the daemon gives it

    The container cannot reach addresses outside the subnets of those networks.

.. expectation:: The daemon gives a container on internal networks only no default route
    :id: NET-GW-INTERNAL-ONLY
    :observed_from: task, ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :exact_form_of: NET-GW-INTERNAL-ONLY-NO-EXTERNAL-ACCESS
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A container attached only to ``internal`` networks

    The daemon does not give the container a default route.

.. expectation:: The default route goes through the endpoint with the highest GwPriority
    :id: NET-GW-PRIORITY
    :observed_from: ctr
    :depends_on: none
    :platforms: linux
    :refines: REQ-NET-GW-PRIORITY

    *Applies to:* Linux nodes with a container with endpoints on two or more networks that are not
    ``internal``, one with a higher :api-field:`GwPriority` than the others

    For each IP version that the endpoint gives the container a gateway or default route for, the
    container's default route goes through that endpoint.

.. expectation:: With equal GwPriority, the default route follows a fixed endpoint order
    :id: NET-GW-PRIORITY-TIE
    :observed_from: ctr
    :depends_on: none
    :platforms: linux
    :refines: REQ-NET-GW-PRIORITY

    *Applies to:* Linux nodes with a container with endpoints on two or more networks that are not
    ``internal``, all with the same :api-field:`GwPriority`

    For each IP version, the container's default route is through the first endpoint that gives a
    gateway or default route for that version, in this order: (1) endpoints that give one for both
    IPv4 and IPv6, then endpoints that give one for a single version; (2) within each group, network
    names in byte order.

.. expectation:: At API version 1.47 or earlier, container create ignores GwPriority
    :id: NET-GW-PRIORITY-IGNORED-CREATE
    :observed_from: api, ctr
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-NET-GW-PRIORITY, REQ-NET-API-BACKWARD-COMPATIBLE

    *Applies to:* Two networks that are not ``internal`` and that each give a container a default
    route, where a container with endpoints on both and the same :api-field:`GwPriority` for each
    has its default route through the first

    At API version 1.47 or earlier, :api-op:`POST /containers/create` with endpoints on both
    networks in :api-field:`EndpointsConfig`, and a :api-field:`GwPriority` higher than 0 for the
    endpoint on the second, responds 201. :api-op:`GET /containers/{id}/json` then reports
    :api-field:`GwPriority` 0 for each of the container's networks. On Linux nodes, once the
    container runs, its default route goes through the first network.

.. expectation:: From API 1.48, a connect applies GwPriority
    :id: NET-GW-PRIORITY-CONNECT
    :observed_from: api, ctr
    :depends_on: none
    :platforms: linux, windows
    :refines: REQ-NET-GW-PRIORITY

    At API version 1.48 or later, after :api-op:`POST /networks/{id}/connect` for a running
    container with :api-field:`GwPriority` set responds 200, :api-op:`GET /containers/{id}/json`
    reports that :api-field:`GwPriority` for the network. On Linux nodes, when that value is higher
    than the :api-field:`GwPriority` of the container's other endpoints, the container's default
    route goes through the new endpoint, for each IP version that the endpoint gives a gateway or
    default route for.

.. expectation:: Before API 1.48, a connect applies GwPriority too
    :id: NET-GW-PRIORITY-CONNECT-API-VERSION
    :observed_from: api, ctr
    :depends_on: none
    :platforms: linux, windows
    :concerns: NET-GW-PRIORITY-CONNECT
    :violates: REQ-NET-API-BACKWARD-COMPATIBLE

    At API version 1.47 or earlier, after :api-op:`POST /networks/{id}/connect` for a running
    container with :api-field:`GwPriority` set responds 200, :api-op:`GET /containers/{id}/json`
    reports that :api-field:`GwPriority` for the network. On Linux nodes, when that value is higher
    than the :api-field:`GwPriority` of the container's other endpoints, the container's default
    route goes through the new endpoint, for each IP version that the endpoint gives a gateway or
    default route for.

.. _network-common-default-gateway-in-a-swarm:

In a Swarm
==========

.. expectation:: A Windows container on internal overlays only reaches outside addresses
    :id: NET-GW-WINDOWS-INTERNAL-OVERLAY-EGRESS
    :observed_from: task, ctr
    :depends_on: os-windows
    :platforms: windows
    :needs: SWARM-HOST-HNS-OVERLAY, SWARM-HOST-FIREWALL-CLUSTER-PORTS
    :within: SWARM-CEIL-REASSIGNMENT
    :narrows: NET-GW-INTERNAL-ONLY-NO-EXTERNAL-ACCESS, NET-GW-INTERNAL-ONLY
    :concerns: SWARM-EGRESS-INTERNAL-ONLY
    :violates: REQ-NET-INTERNAL-NO-EXTERNAL-ACCESS

    A Windows container attached only to ``internal`` overlays reaches addresses outside those
    networks.

.. expectation:: A Windows container on internal overlays only has a default route
    :id: NET-GW-WINDOWS-INTERNAL-OVERLAY-ROUTE
    :observed_from: task, ctr
    :depends_on: os-windows
    :platforms: windows
    :pins: NET-GW-WINDOWS-INTERNAL-OVERLAY-EGRESS
    :needs: SWARM-HOST-HNS-OVERLAY, SWARM-HOST-FIREWALL-CLUSTER-PORTS
    :within: SWARM-CEIL-REASSIGNMENT

    A Windows container attached only to ``internal`` overlays has a default route.

.. _network-common-file-etc-hosts:

********************
 :file:`/etc/hosts`
********************

In this section, a container's addresses on a network are the IPv4 address and the IPv6 address that
:api-op:`GET /containers/{id}/json` reports for that network in
:api-field:`NetworkSettings.Networks`, as :api-field:`IPAddress` and :api-field:`GlobalIPv6Address`.
The addresses in :api-field:`IPAMConfig.LinkLocalIPs`, and the link-local addresses that the kernel
assigns, are not among them. The container's addresses are its addresses on each of its networks.

.. expectation:: /etc/hosts maps each of the container's addresses to its hostname
    :id: NET-HOSTS-ADDRESSES
    :observed_from: task, ctr
    :depends_on: none
    :platforms: linux
    :refines: REQ-NET-HOSTS-OWN-NAME
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A container that does not use the host's network

    For each of the container's addresses, :file:`/etc/hosts` has a line that maps the address to
    the container's hostname. The exceptions are addresses on the ingress network and on
    :network:`docker_gwbridge`. IPv6 addresses are also exceptions while IPv6 is disabled in the
    container, so that its loopback interface does not have an IPv6 address, for example with
    :sysctl:`net.ipv6.conf.all.disable_ipv6` set to 1.

.. expectation:: /etc/hosts maps the loopback addresses to localhost
    :id: NET-HOSTS-LOCALHOST
    :observed_from: task, ctr
    :depends_on: none
    :platforms: linux
    :refines: REQ-NET-HOSTS-OWN-NAME
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A container that does not use the host's network

    :file:`/etc/hosts` has a line that maps ``127.0.0.1`` to ``localhost``. It also has a line that
    maps ``::1`` to ``localhost``, except while IPv6 is disabled in the container, as for
    :need:`NET-HOSTS-ADDRESSES`.

.. expectation:: A dotted hostname or a domain gives full and short names in /etc/hosts
    :id: NET-HOSTS-DOMAIN-NAME
    :observed_from: ctr
    :depends_on: none
    :platforms: linux
    :refines: REQ-NET-HOSTS-FQDN

    *Applies to:* A container whose :api-field:`Hostname` contains a dot, or that has a
    :api-field:`Domainname`

    Each line for the container's addresses gives two names. The first is the full name: the
    hostname, then a dot and the domain name if the container has one. The second is the part of the
    hostname before its first dot.

.. expectation:: A connect adds the container's addresses on the network to /etc/hosts
    :id: NET-HOSTS-CONNECT
    :observed_from: ctr
    :depends_on: none
    :platforms: linux
    :refines: REQ-NET-HOSTS-OWN-NAME

    *Applies to:* A running container, and a network other than the ingress network and
    :network:`docker_gwbridge`

    After :api-op:`POST /networks/{id}/connect` to that network for the container responds 200,
    :file:`/etc/hosts` has the lines for the container's addresses on that network. IPv6 addresses
    are exceptions while IPv6 is disabled in the container, as for :need:`NET-HOSTS-ADDRESSES`.

.. expectation:: A disconnect removes the network's addresses from /etc/hosts
    :id: NET-HOSTS-DISCONNECT
    :observed_from: ctr
    :depends_on: none
    :platforms: linux
    :refines: REQ-NET-HOSTS-OWN-NAME

    *Applies to:* A running container with an endpoint on a network, from its start or from a later
    connect

    After :api-op:`POST /networks/{id}/disconnect` for the container responds 200,
    :file:`/etc/hosts` does not have the lines for the container's addresses on that network.

.. expectation:: An ExtraHosts entry adds a line to /etc/hosts
    :id: NET-HOSTS-EXTRA-HOSTS
    :observed_from: ctr
    :depends_on: none
    :platforms: linux
    :refines: REQ-NET-EXTRA-HOSTS

    An entry ``<name>:<address>`` in :api-field:`HostConfig.ExtraHosts` adds a line to
    :file:`/etc/hosts` that maps ``<address>`` to ``<name>``. IPv6 addresses are exceptions while
    IPv6 is disabled in the container, as for :need:`NET-HOSTS-ADDRESSES`.

.. expectation:: docker run --add-host adds a line to /etc/hosts
    :id: NET-HOSTS-EXTRA-HOSTS-CLI
    :observed_from: api, ctr
    :depends_on: cli
    :platforms: linux
    :refines: REQ-NET-EXTRA-HOSTS
    :cli_for: NET-HOSTS-EXTRA-HOSTS

    :cli:`docker run -d` with :cli-flag:`--add-host` ``<name>:<address>`` exits zero. The
    container's :file:`/etc/hosts` then has a line that maps ``<address>`` to ``<name>``. IPv6
    addresses are exceptions while IPv6 is disabled in the container, as for
    :need:`NET-HOSTS-ADDRESSES`.

.. expectation:: A host-gateway entry maps to each address in host-gateway-ips
    :id: NET-HOSTS-HOST-GATEWAY
    :observed_from: ctr
    :depends_on: none
    :platforms: linux
    :refines: REQ-NET-HOST-GATEWAY

    *Applies to:* A daemon with :daemon-json:`host-gateway-ips` set

    An entry ``<name>:host-gateway`` in :api-field:`HostConfig.ExtraHosts` adds a line to
    :file:`/etc/hosts` for each address in :daemon-json:`host-gateway-ips`. Each line maps the
    address to ``<name>``. IPv6 addresses are exceptions while IPv6 is disabled in the container, as
    for :need:`NET-HOSTS-ADDRESSES`.

.. expectation:: Without host-gateway-ips, host-gateway is the default bridge's addresses
    :id: NET-HOSTS-HOST-GATEWAY-DEFAULT-BRIDGE
    :observed_from: ctr
    :depends_on: none
    :platforms: linux
    :refines: REQ-NET-HOST-GATEWAY

    *Applies to:* A daemon without :daemon-json:`host-gateway-ips`

    An entry ``<name>:host-gateway`` in :api-field:`HostConfig.ExtraHosts` adds a line to
    :file:`/etc/hosts` for the default bridge's own IPv4 address, and one for its IPv6 address if it
    has one: the :api-field:`Gateway` values that :api-op:`GET /networks/{id}` for the network
    :network:`bridge` reports in :api-field:`IPAM.Config`. This holds whatever networks the
    container is attached to. IPv6 addresses are exceptions while IPv6 is disabled in the container,
    as for :need:`NET-HOSTS-ADDRESSES`.

.. expectation:: Without a host-gateway address, a host-gateway entry is accepted without a warning
    :id: NET-HOSTS-HOST-GATEWAY-NO-ADDRESS-CREATE
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows
    :concerns: NET-HOSTS-HOST-GATEWAY-NO-BRIDGE
    :violates: PRIN-WARN-AHEAD

    *Applies to:* A daemon without a host-gateway address: a Linux daemon without
    :daemon-json:`host-gateway-ips` and with :daemon-json:`bridge` set to ``none``, or a Windows
    daemon without :daemon-json:`host-gateway-ips`

    :api-op:`POST /containers/create` for a container with an entry ``<name>:host-gateway`` in
    :api-field:`HostConfig.ExtraHosts` responds 201 without a warning about the host-gateway entry.

.. expectation:: Without a host-gateway address, the container does not start
    :id: NET-HOSTS-HOST-GATEWAY-NO-BRIDGE
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows
    :refines: PRIN-FAILS-CLOSED, REQ-NET-START-FAILED-CONTAINER-KEPT

    *Applies to:* A daemon without a host-gateway address (on a Linux node, one without
    :daemon-json:`host-gateway-ips` and with :daemon-json:`bridge` set to ``none``; on a Windows
    node, one without :daemon-json:`host-gateway-ips`), and a created container without
    :api-field:`HostConfig.AutoRemove` and with an entry ``<name>:host-gateway`` in
    :api-field:`HostConfig.ExtraHosts`

    :api-op:`POST /containers/{id}/start` for the container responds with a 4xx or 5xx status.
    :api-op:`GET /containers/{id}/json` then reports :api-field:`State.Status` as ``created``.

.. expectation:: Without a host-gateway address, starting the container responds 500
    :id: NET-HOSTS-HOST-GATEWAY-NO-BRIDGE-HTTP500
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows
    :exact_form_of: NET-HOSTS-HOST-GATEWAY-NO-BRIDGE

    *Applies to:* A daemon without a host-gateway address (on a Linux node, one without
    :daemon-json:`host-gateway-ips` and with :daemon-json:`bridge` set to ``none``; on a Windows
    node, one without :daemon-json:`host-gateway-ips`), and a created container with an entry
    ``<name>:host-gateway`` in :api-field:`HostConfig.ExtraHosts`

    :api-op:`POST /containers/{id}/start` for the container responds 500, with a message that
    contains :api-error:`unable to derive the IP value for host-gateway`.

.. expectation:: docker run with --add-host <name>:host-gateway and no address fails
    :id: NET-HOSTS-HOST-GATEWAY-NO-BRIDGE-CLI
    :observed_from: api
    :depends_on: cli
    :platforms: linux, windows
    :refines: PRIN-FAILS-CLOSED
    :cli_for: NET-HOSTS-HOST-GATEWAY-NO-BRIDGE

    *Applies to:* A daemon without a host-gateway address (on a Linux node, one without
    :daemon-json:`host-gateway-ips` and with :daemon-json:`bridge` set to ``none``; on a Windows
    node, one without :daemon-json:`host-gateway-ips`)

    :cli:`docker run` with :cli-flag:`--add-host` ``<name>:host-gateway`` exits non-zero and prints
    the daemon's error.

.. expectation:: On Windows, extra hosts entries are accepted and not written to the hosts file
    :id: NET-HOSTS-WINDOWS-EXTRA-HOSTS-IGNORED
    :observed_from: api, ctr, task
    :depends_on: none
    :platforms: windows
    :within: SWARM-CEIL-REASSIGNMENT
    :concerns: NET-HOSTS-SERVICE-ALIASES, NET-HOSTS-EXTRA-HOSTS, NET-HOSTS-HOST-GATEWAY, NET-HOSTS-HOST-GATEWAY-DEFAULT-BRIDGE, NET-HOSTS-EXTRA-HOSTS-CLI
    :violates: PRIN-FAILS-CLOSED, REQ-NET-HOSTS-SERVICE-ENTRIES, REQ-NET-EXTRA-HOSTS

    :api-op:`POST /containers/create` with entries in :api-field:`HostConfig.ExtraHosts` responds
    201. Once the container runs, its hosts file has no line for those entries. In the hosts file of
    each task of a service whose :api-field:`ContainerSpec.Hosts` has entries, no line has those
    entries.

.. _network-common-file-etc-hosts-in-a-swarm:

In a Swarm
==========

.. expectation:: A task's /etc/hosts maps each alias of a Hosts entry to its address
    :id: NET-HOSTS-SERVICE-ALIASES
    :observed_from: task
    :depends_on: none
    :platforms: linux
    :refines: REQ-NET-HOSTS-SERVICE-ENTRIES
    :within: SWARM-CEIL-REASSIGNMENT

    *Applies to:* A service whose :api-field:`ContainerSpec.Hosts` has an entry ``<address> <name>
    <alias>...`` with one or more aliases

    In the :file:`/etc/hosts` of each task of the service, ``<address>`` maps to ``<name>`` and to
    each alias.

.. expectation:: A task's /etc/hosts drops the aliases of a Hosts entry
    :id: NET-HOSTS-SERVICE-ALIASES-DROPPED
    :observed_from: task
    :depends_on: none
    :platforms: linux
    :within: SWARM-CEIL-REASSIGNMENT
    :narrows: NET-HOSTS-SERVICE-ALIASES
    :violates: REQ-NET-HOSTS-SERVICE-ENTRIES

    *Applies to:* A service whose :api-field:`ContainerSpec.Hosts` has an entry ``<address> <name>
    <alias>...`` with one or more aliases

    In the :file:`/etc/hosts` of each task of the service, ``<address>`` maps to ``<name>`` only,
    and no line has the aliases.

.. _network-common-driver-properties:

*******************
 Driver properties
*******************

Expectations in this spec name a property of a network driver, not the driver itself, where the set
of drivers with the property can change. The two detectors below give the properties of the built-in
drivers. A plugin driver has the properties that it implements. The expectations hold for the
:driver:`l2tunnel` and :driver:`ics` drivers on a Windows node where :api-op:`POST /networks/create`
for a network of that driver responds 201.

.. expectation:: Every built-in driver but host and null adds an interface
    :id: NET-DRVPROP-ADDS-INTERFACE
    :observed_from: ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :needs: NET-HOST-MACVLAN, IPVL-HOST-IPVLAN, BR-HOST-NETNS-VETH-BRIDGE, SWARM-HOST-NETNS-VETH-BRIDGE, SWARM-HOST-VXLAN, SWARM-HOST-HNS-OVERLAY
    :rationale: RAT-NET-DRIVER-PROPERTY-SETS

    A container on a network of one of these drivers has an interface on the network:

    - :driver:`bridge`
    - :driver:`overlay`
    - :driver:`macvlan`
    - :driver:`ipvlan`
    - Windows: :driver:`nat`, :driver:`transparent`, :driver:`l2bridge`, :driver:`l2tunnel`,
      :driver:`ics`, :driver:`internal`, :driver:`private`

    A container on a network of one of these drivers does not:

    - :driver:`host`
    - :driver:`null` (the :network:`none` network)

.. expectation:: Built-in drivers that add an interface accept a MAC, except ipvlan
    :id: NET-DRVPROP-ENDPOINT-MAC
    :observed_from: api, ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :needs: NET-HOST-MACVLAN, IPVL-HOST-IPVLAN, BR-HOST-NETNS-VETH-BRIDGE, SWARM-HOST-NETNS-VETH-BRIDGE, SWARM-HOST-VXLAN, SWARM-HOST-HNS-OVERLAY
    :rationale: RAT-NET-DRIVER-PROPERTY-SETS

    These drivers accept an endpoint MAC address:

    - :driver:`bridge`
    - :driver:`overlay`
    - :driver:`macvlan`
    - Windows: :driver:`nat`, :driver:`transparent`, :driver:`l2bridge`, :driver:`l2tunnel`,
      :driver:`ics`, :driver:`internal`, :driver:`private`

    These drivers do not accept one:

    - :driver:`ipvlan`

    For these drivers, the property does not apply, because they add no interface:

    - :driver:`host`
    - :driver:`null` (the :network:`none` network)

.. _network-common-daemon-restart:

****************
 Daemon restart
****************

.. expectation:: After SIGKILL and restart, the daemon's old namespaces are removed
    :id: NET-RESTART-NAMESPACES-REMOVED
    :observed_from: node
    :depends_on: os-linux
    :platforms: linux
    :refines: PRIN-NO-LEAKS

    After :signal:`SIGKILL` and restart, every network namespace that the daemon created before the
    kill is removed, other than an overlay network's namespace and the namespace of a container that
    live-restore keeps running. This includes the namespaces of load-balancer and ingress sandboxes,
    and of containers that the daemon does not keep running. An overlay network's namespace is the
    one that holds the network's VXLAN device. A namespace that the daemon creates after the restart
    is a different namespace, even at the same path. A namespace is removed once no mount of it
    remains in the mount namespace that the daemon runs in, even while a process still runs in it or
    another mount namespace keeps a copy of a mount of it.

.. expectation:: With live-restore, a container keeps its namespace and interfaces
    :id: NET-RESTART-LIVE-RESTORE
    :observed_from: node, ctr
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-NET-LIVE-RESTORE

    *Applies to:* Linux nodes with a running container, whose daemon has :daemon-json:`live-restore`
    true

    After :signal:`SIGKILL` and restart, the container keeps its network namespace and its
    interfaces.
