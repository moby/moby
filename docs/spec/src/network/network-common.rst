################################
 Common networking surface area
################################

Scope: the behavior that every network driver must have, and the Engine API requests for networks
and endpoints that do not depend on the network's driver. This spec describes them as observable
from outside the daemon.

This spec is a stub. It holds expectations that every network driver shares. They come from the work
on the other specs and from reviews of the specs' coverage. It is not yet a complete account.
:doc:`../README.md <../README>` gives the model that this spec follows: the items of a spec, their
categories and what their verdicts depend on. :doc:`README.md <README>` gives the conventions of the
networking specs, such as vantage points, daemon settings, timing and convergence ceilings, and host
requirements. :doc:`swarm-networking` and :doc:`bridge-networking.md <bridge-networking>` cover what
is specific to their drivers. :doc:`container-dns.md <container-dns>` covers name resolution.

In this spec, a predefined network is a network that :api-op:`GET /networks` with the filter
:api-query:`type=builtin` lists. The expectations here also hold for Swarm-scoped networks and on
nodes in a Swarm, unless an expectation narrows them.
:ref:`swarm-networking-network-lifecycle-api-surface` covers the requests that are specific to
Swarm-scoped networks. Where an expectation of swarm-networking.md says something different about
the same request or state, that expectation holds.

.. _network-common-rationales:

************
 Rationales
************

.. rationale:: Why a change to NET-API-CONNECT-ALREADY-ATTACHED-TWO-STATUSES should be deliberate
    :id: RAT-NET-REPEAT-CONNECT-STATUS-BY-SCOPE

    It catches a change that makes a repeat connect respond with one status on every network, in
    place of 403 on a network that is not Swarm-scoped and 409 on an attachable Swarm-scoped
    network; clients branch on either status, so the change should be deliberate.

.. rationale:: Why a change to NET-API-GWBRIDGE-PRUNED should be deliberate
    :id: RAT-NET-GWBRIDGE-NOT-PREDEFINED

    It catches a change that makes network prune keep docker_gwbridge on a node that is not in a
    Swarm; that would make it a system network that the documentation does not list, and an operator
    would then have to remove a gateway network that they created by hand, so the change should be
    deliberate.

.. rationale:: Why a change to NET-API-HTTP404-CONFIG-FROM-MISSING should be deliberate
    :id: RAT-NET-API-HTTP404-CONFIG-FROM-MISSING

    It catches a change of the status for a ConfigFrom that names a missing network, such as from
    404 to 400; clients branch on 404 to tell a missing dependency from a malformed request, so the
    change should be deliberate.

.. rationale:: Why a change to NET-IFACE-NAME-COLLISION-ORDER should be deliberate
    :id: RAT-NET-IFACE-NAME-COLLISION-ORDER

    It catches a change in how requested and generated interface names are assigned, such as
    reserving the requested names before the others are generated; that decides which containers
    start, so the change should be deliberate.

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

.. _network-common-host-requirements:

*******************
 Host requirements
*******************

The kernel facilities are requirements of each Linux node. The host configuration is a requirement
of each node. The conventions for this section are in :ref:`README.md <README-host-requirements>`.
The specs of the drivers list what each driver needs: :ref:`bridge-networking-host-requirements`,
:ref:`swarm-networking-host-requirements` and :ref:`ipvlan-networking-host-requirements`.

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

    *Needed when:* A :driver:`macvlan` network

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

    *Needed when:* Always. In an expectation whose state names firewalld, the host's firewall is
    firewalld, and it passes only the traffic that a host configuration expectation of the
    expectation's spec names, such as the Swarm ports in :ref:`swarm-networking-host-configuration`,
    with configuration that a firewalld reload keeps.

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

    *Applies to:* An existing network that is not Swarm-scoped and not predefined

    :api-op:`POST /networks/create` with the network's name responds 409. :api-op:`GET /networks`
    then lists only one network with that name.

.. expectation:: docker network create with an existing network's name fails
    :id: NET-API-CREATE-DUPLICATE-NAME-CLI
    :observed_from: api
    :depends_on: cli
    :platforms: linux, windows
    :cli_for: NET-API-CREATE-DUPLICATE-NAME

    *Applies to:* An existing network that is not Swarm-scoped and not predefined

    :cli:`docker network create` with the network's name exits non-zero and prints the daemon's
    error. :cli:`docker network ls` then lists only one network with that name.

.. expectation:: Outside a Swarm, a predefined or reserved network name responds 403
    :id: NET-API-CREATE-PREDEFINED-NAME
    :observed_from: api:solo
    :depends_on: none
    :platforms: linux

    *Applies to:* A Linux node that is not in a Swarm

    :api-op:`POST /networks/create` with the name of a predefined network, :network:`default` or
    ``container`` responds 403. :api-op:`POST /networks/create` with a name that starts with
    ``container:`` also responds 403. :api-op:`GET /networks` then does not list a new network with
    that name.

.. expectation:: On a manager, the names bridge and host respond 409, the rest 403
    :id: NET-API-CREATE-PREDEFINED-NAME-MANAGER
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A Linux manager

    :api-op:`POST /networks/create` with the name :network:`bridge` or :network:`host` responds 409.
    It responds 403 when the name is ``none``, :network:`default` or ``container``, or starts with
    ``container:``. This holds for the default driver and for :driver:`overlay`.

.. expectation:: docker network create with a predefined network's name fails
    :id: NET-API-CREATE-PREDEFINED-NAME-CLI
    :observed_from: api
    :depends_on: cli
    :platforms: linux
    :cli_for: NET-API-CREATE-PREDEFINED-NAME

    :cli:`docker network create` with the name of a predefined network exits non-zero and prints the
    daemon's error.

.. expectation:: Deleting a predefined network responds 403
    :id: NET-API-DELETE-PREDEFINED
    :observed_from: api
    :depends_on: none
    :platforms: linux

    :api-op:`DELETE /networks/{id}` for a predefined network responds 403. :api-op:`GET /networks`
    then still lists the network.

.. expectation:: docker network rm of a predefined network fails
    :id: NET-API-DELETE-PREDEFINED-CLI
    :observed_from: api
    :depends_on: cli
    :platforms: linux
    :cli_for: NET-API-DELETE-PREDEFINED

    :cli:`docker network rm` for a predefined network exits non-zero and prints the daemon's error.
    :cli:`docker network ls` then still lists the network.

.. expectation:: Deleting a network with a running container responds 403
    :id: NET-API-DELETE-IN-USE-HTTP403
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A network that is not Swarm-scoped, with a running container attached to it

    :api-op:`DELETE /networks/{id}` for the network responds 403. :api-op:`GET /networks` then still
    lists the network.

.. expectation:: docker network rm of a network with a running container fails
    :id: NET-API-DELETE-IN-USE-CLI
    :observed_from: api
    :depends_on: cli
    :platforms: linux, windows
    :cli_for: NET-API-DELETE-IN-USE-HTTP403

    *Applies to:* A network that is not Swarm-scoped, with a running container attached to it

    :cli:`docker network rm` for the network exits non-zero and prints the daemon's error.
    :cli:`docker network ls` then still lists the network.

.. expectation:: Connecting a running container to a network it is attached to fails
    :id: NET-API-CONNECT-ALREADY-ATTACHED
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A running container attached to a network

    :api-op:`POST /networks/{id}/connect` to that network, for the container, responds with a 4xx or
    5xx status. The container's addresses on that network do not change.

.. expectation:: A repeat connect to a network that is not Swarm-scoped responds 403
    :id: NET-API-CONNECT-ALREADY-ATTACHED-HTTP403
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A running container attached to a network that is not Swarm-scoped

    :api-op:`POST /networks/{id}/connect` to that network, for the container, responds 403. See
    :ref:`network-common-known-divergences-non-goals`.

.. expectation:: docker network connect to a network already attached fails
    :id: NET-API-CONNECT-ALREADY-ATTACHED-CLI
    :observed_from: api
    :depends_on: cli
    :platforms: linux, windows
    :cli_for: NET-API-CONNECT-ALREADY-ATTACHED

    *Applies to:* A running container attached to a network

    :cli:`docker network connect` naming that network and the container exits non-zero and prints
    the daemon's error.

.. expectation:: A repeat connect of a created or stopped container responds 200
    :id: NET-API-CONNECT-ALREADY-ATTACHED-STOPPED
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A created or stopped container whose :api-field:`NetworkSettings.Networks` lists a
    network that is not Swarm-scoped

    :api-op:`POST /networks/{id}/connect` to that network, for the container, responds 200. The
    container's endpoint settings for the network do not change.

.. expectation:: Starting a container connected to a missing network responds 404
    :id: NET-API-CONNECT-MISSING-START-FAILS
    :observed_from: api:solo
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A node that is not in a Swarm, with a created or stopped container

    :api-op:`POST /networks/{id}/connect` for the container, with an ``{id}`` that is not the name
    or an ID prefix of any network, responds 200. :api-op:`POST /containers/{id}/start` for the
    container then responds 404 while no network has that name. See
    :ref:`network-common-known-divergences-non-goals`.

.. expectation:: A container connected to a missing network uses it once it is created
    :id: NET-API-CONNECT-MISSING-CREATED-LATER
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A created or stopped container

    :api-op:`POST /networks/{id}/connect` for the container, with an ``{id}`` that is not the name
    or an ID prefix of any network, responds 200. A network with that name is then created.
    :api-op:`POST /containers/{id}/start` for the container then responds 204, and the container has
    an address on that network. See :ref:`network-common-known-divergences-non-goals`.

.. expectation:: A static address outside the network's subnets responds 400
    :id: NET-API-STATIC-ADDRESS-OUTSIDE-SUBNET-HTTP400
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A network that is not Swarm-scoped, and a container that is not attached to it

    :api-op:`POST /containers/create` with a static address in :api-field:`IPAMConfig` that is
    outside every subnet of that network responds 400, and does not create a container.
    :api-op:`POST /networks/{id}/connect` to that network, for the container, with that address,
    responds 400, and does not attach the container.

.. expectation:: A static address on the default bridge responds 400
    :id: NET-API-STATIC-ADDRESS-DEFAULT-BRIDGE-HTTP400
    :observed_from: api
    :depends_on: none
    :platforms: linux

    *Applies to:* Linux nodes with a container that is not attached to the network :network:`bridge`

    :api-op:`POST /containers/create` with a static address in :api-field:`IPAMConfig` for the
    network :network:`bridge` responds 400, and does not create a container. :api-op:`POST
    /networks/{id}/connect` to the network :network:`bridge`, for the container, with a static
    address in :api-field:`IPAMConfig`, responds 400, and does not attach the container.

.. expectation:: docker run and docker network connect with a refused --ip fail
    :id: NET-API-STATIC-ADDRESS-OUTSIDE-SUBNET-CLI
    :observed_from: api
    :depends_on: cli
    :platforms: linux, windows
    :cli_for: NET-API-STATIC-ADDRESS-OUTSIDE-SUBNET-HTTP400

    :cli:`docker run` and :cli:`docker network connect` with :cli-flag:`--ip` outside every subnet
    of a network that is not Swarm-scoped exit non-zero and print the daemon's error. On Linux
    nodes, they do the same with :cli-flag:`--ip` for the network :network:`bridge`.

.. expectation:: At API version 1.43 or earlier, create on several networks responds 400
    :id: NET-API-CTR-CREATE-MULTIPLE-NETWORKS-HTTP400
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows

    At API version 1.43 or earlier, :api-op:`POST /containers/create` with more than one network in
    :api-field:`NetworkingConfig.EndpointsConfig` responds 400, and does not create a container.

.. expectation:: Disconnecting a container from a network it is not attached to fails
    :id: NET-API-DISCONNECT-NOT-ATTACHED
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A container, and a network that it is not attached to

    :api-op:`POST /networks/{id}/disconnect` for the container from that network responds with a 4xx
    or 5xx status.

.. expectation:: Disconnecting from a network the container is not on responds 500
    :id: NET-API-DISCONNECT-NOT-ATTACHED-HTTP500
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A container, and a network that it is not attached to

    :api-op:`POST /networks/{id}/disconnect` for the container from that network responds 500, with
    a message that contains :api-error:`is not connected to`. See
    :ref:`network-common-known-divergences-non-goals`.

.. expectation:: docker network disconnect from a network not attached fails
    :id: NET-API-DISCONNECT-NOT-ATTACHED-CLI
    :observed_from: api
    :depends_on: cli
    :platforms: linux, windows
    :cli_for: NET-API-DISCONNECT-NOT-ATTACHED

    *Applies to:* A container, and a network that it is not attached to

    :cli:`docker network disconnect` naming that network and the container exits non-zero and prints
    the daemon's error.

.. expectation:: A running container disconnects from a network and keeps running
    :id: NET-API-DISCONNECT-RUNNING
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows

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

    *Applies to:* A running container with an endpoint on a network other than :network:`host`, from
    its start or from a later connect

    After :api-op:`POST /networks/{id}/disconnect` for the container responds 200, the container
    does not have its interface on that network.

.. expectation:: docker network disconnect of a running container succeeds
    :id: NET-API-DISCONNECT-RUNNING-CLI
    :observed_from: api
    :depends_on: cli
    :platforms: linux, windows
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
    :cli_for: NET-API-DISCONNECT-STOPPED

    *Applies to:* A created or stopped container whose :api-field:`NetworkSettings.Networks` lists a
    network

    :cli:`docker network disconnect` naming that network and the container exits zero. :cli:`docker
    inspect` for the container then does not list the network in
    :api-field:`NetworkSettings.Networks`.

.. expectation:: A failed MacAddress connect leaves the network in the container settings
    :id: NET-API-CONNECT-MAC-ADDRESS-REFUSED-LISTED
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows
    :assumes: NET-DRVPROP-ENDPOINT-MAC

    *Applies to:* A running container, and a network whose driver does not accept an endpoint MAC
    address (:ref:`network-common-driver-properties`)

    :api-op:`POST /networks/{id}/connect` to that network, for the container, with
    :api-field:`MacAddress` set, at API version 1.54 or later, fails. :api-op:`GET
    /containers/{id}/json` then does not list the network in :api-field:`NetworkSettings.Networks`,
    and :api-op:`GET /networks/{id}` does not list the container.

.. expectation:: A network is found by its full ID, its name or a unique ID prefix
    :id: NET-API-INSPECT-ID-NAME-PREFIX
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows

    :api-op:`GET /networks/{id}` responds 200 with the network when ``{id}`` is the network's full
    ID or its name. It does the same when ``{id}`` is a prefix of the network's ID that the ID of no
    other network starts with, and that is not the name of another network.

.. expectation:: docker network inspect finds a network by ID, name or unique prefix
    :id: NET-API-INSPECT-ID-NAME-PREFIX-CLI
    :observed_from: api
    :depends_on: cli
    :platforms: linux, windows
    :cli_for: NET-API-INSPECT-ID-NAME-PREFIX

    :cli:`docker network inspect` prints the network when it is given the network's full ID or its
    name. It does the same when it is given a prefix of the network's ID that the ID of no other
    network starts with, and that is not the name of another network.

.. expectation:: Inspecting a network that matches no ID, name or prefix responds 404
    :id: NET-API-INSPECT-NOT-FOUND-HTTP404
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows

    :api-op:`GET /networks/{id}` responds 404 when ``{id}`` is not the full ID, the name or a prefix
    of the ID of any network.

.. expectation:: Inspecting by an ID prefix that several networks share fails
    :id: NET-API-INSPECT-AMBIGUOUS-PREFIX
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* Two or more networks whose IDs start with a prefix that is not a network name

    :api-op:`GET /networks/{id}` with that prefix as ``{id}`` responds with a 4xx or 5xx status.

.. expectation:: An ID prefix that several networks share responds 400
    :id: NET-API-INSPECT-AMBIGUOUS-PREFIX-HTTP400
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* Two or more networks whose IDs start with a prefix that is not a network name

    :api-op:`GET /networks/{id}` with that prefix as ``{id}`` responds 400, with a message that
    contains :api-error:`is ambiguous`.

.. expectation:: A network's name takes precedence over another network's ID prefix
    :id: NET-API-INSPECT-NAME-OVER-PREFIX
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A network whose name is a prefix of the ID of another network that is not
    Swarm-scoped

    :api-op:`GET /networks/{id}` with that name as ``{id}`` returns the network with that name.

.. expectation:: The filter type=custom lists the networks that type=builtin does not
    :id: NET-API-FILTER-TYPE-CUSTOM
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows

    :api-op:`GET /networks` with the filter :api-query:`type=custom` lists the networks that the
    filter :api-query:`type=builtin` does not list.

.. expectation:: type=builtin lists host and none, and bridge while it exists
    :id: NET-API-FILTER-TYPE-BUILTIN-PREDEFINED
    :observed_from: api
    :depends_on: none
    :platforms: linux

    :api-op:`GET /networks` with the filter :api-query:`type=builtin` lists :network:`host` and
    :network:`none`. It also lists :driver:`bridge` while the default bridge network exists.

.. expectation:: The filter type=builtin lists only bridge, host and none
    :id: NET-API-FILTER-TYPE-BUILTIN
    :observed_from: api
    :depends_on: none
    :platforms: linux

    :api-op:`GET /networks` with the filter :api-query:`type=builtin` lists :driver:`bridge`,
    :network:`host` and ``none``, and no other network.

.. expectation:: An unknown filter term or a bad type or dangling value responds 400
    :id: NET-API-FILTER-INVALID
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows

    :api-op:`GET /networks` with a filter term that it does not support responds 400. It also
    responds 400 for a value of the :api-query:`type` or :api-query:`dangling` term that the term
    does not accept, and for more than one value for :api-query:`dangling`. A value of another
    supported term that matches no network gets a 200 response that lists no network.

.. expectation:: docker network ls with an unsupported filter term or value fails
    :id: NET-API-FILTER-INVALID-CLI
    :observed_from: api
    :depends_on: cli
    :platforms: linux, windows
    :cli_for: NET-API-FILTER-INVALID

    :cli:`docker network ls --filter` exits non-zero and prints the daemon's error for a filter term
    that the daemon does not support. It does the same for a value of the :api-query:`type` or
    :api-query:`dangling` term that the term does not accept.

.. expectation:: The type and dangling filter terms each accept a fixed set of values
    :id: NET-API-FILTER-TYPE-VALUES
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows

    The :api-query:`type` filter term of :api-op:`GET /networks` accepts the values ``builtin`` and
    ``custom``. The :api-query:`dangling` term accepts one value: ``true``, ``1``, ``false`` or
    ``0``.

.. expectation:: With both type values, the filter lists every network
    :id: NET-API-FILTER-TYPE-BOTH
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows

    :api-op:`GET /networks` with both :api-query:`type=builtin` and :api-query:`type=custom` lists
    every network.

.. expectation:: The filter dangling=true lists a network with no running container
    :id: NET-API-FILTER-DANGLING-TRUE
    :observed_from: api
    :depends_on: none
    :platforms: linux

    *Applies to:* A network that is not Swarm-scoped and not predefined, with no running container
    attached to it, other than :network:`docker_gwbridge` on a Linux node in a Swarm that has an
    ingress network created with :api-field:`Ingress` set

    :api-op:`GET /networks` with the filter :api-query:`dangling=true` lists the network.

.. expectation:: The filter dangling=false lists a predefined or in-use network
    :id: NET-API-FILTER-DANGLING-FALSE
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A network that is not Swarm-scoped, and that is predefined or has a running
    container attached to it

    :api-op:`GET /networks` with the filter :api-query:`dangling=false` lists the network.

.. expectation:: Network prune deletes unused networks and lists their names
    :id: NET-API-PRUNE-UNUSED
    :observed_from: api:solo
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A node that is not in a Swarm

    :api-op:`POST /networks/prune` responds 200. It deletes each network that is not Swarm-scoped,
    not predefined and not config-only, and that no running container is attached to.
    :api-field:`NetworksDeleted` in the response lists the names of the networks that it deletes.

.. expectation:: docker network prune --force deletes unused networks and names them
    :id: NET-API-PRUNE-UNUSED-CLI
    :observed_from: api:solo
    :depends_on: cli
    :platforms: linux, windows
    :cli_for: NET-API-PRUNE-UNUSED

    *Applies to:* A node that is not in a Swarm

    :cli:`docker network prune --force` exits zero, and prints the names of the networks that it
    deletes. :cli:`docker network ls` then does not list those networks.

.. expectation:: Outside a Swarm, network prune deletes an unused docker_gwbridge
    :id: NET-API-PRUNE-GWBRIDGE-NO-SWARM
    :observed_from: api:solo
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A node that is not in a Swarm, and a :network:`docker_gwbridge` network that no
    running container is attached to

    :api-op:`POST /networks/prune` deletes :network:`docker_gwbridge`. See
    :ref:`network-common-known-divergences-non-goals`.

.. expectation:: In a Swarm with an ingress network, prune keeps docker_gwbridge
    :id: NET-API-PRUNE-GWBRIDGE-SWARM
    :observed_from: api:mgr, api:wkr
    :depends_on: none
    :platforms: linux

    *Applies to:* A Linux node in a Swarm that has an ingress network created with
    :api-field:`Ingress` set

    :api-op:`POST /networks/prune` does not delete :network:`docker_gwbridge`. :api-op:`GET
    /networks/docker_gwbridge` lists ``ingress-sbox`` in :api-field:`Containers`.

.. expectation:: A network created with ConfigOnly reports ConfigOnly true
    :id: NET-API-CONFIG-ONLY-CREATE
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows

    :api-op:`POST /networks/create` with :api-field:`ConfigOnly` true responds 201. :api-op:`GET
    /networks/{id}` then reports :api-field:`ConfigOnly` true.

.. expectation:: A ConfigFrom network reports the config-only network's IPAM
    :id: NET-API-CONFIG-FROM-IPAM
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A network that is not Swarm-scoped, created with :api-field:`ConfigFrom` naming a
    config-only network

    :api-op:`GET /networks/{id}` for the network reports the config-only network's :api-field:`IPAM`
    configuration. :need:`SWARM-SCOPED-CONFIG-FROM-INSPECT-ATTACHED` and
    :need:`SWARM-SCOPED-CONFIG-FROM-INSPECT-UNATTACHED` cover a Swarm-scoped network.

.. expectation:: A ConfigFrom network's containers get addresses in the config subnets
    :id: NET-API-CONFIG-FROM-ADDRESSES
    :observed_from: ctr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A network created with :api-field:`ConfigFrom` naming a config-only network

    The network's containers get addresses in the subnets of the config-only network's
    :api-field:`IPAM` configuration.

.. expectation:: Creating a network with a missing ConfigFrom network fails
    :id: NET-API-CONFIG-FROM-MISSING
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows

    :api-op:`POST /networks/create` with :api-field:`ConfigFrom` naming a network that does not
    exist responds with a 4xx or 5xx status. :api-op:`GET /networks` then does not list a network
    with the request's :api-field:`Name`.

.. expectation:: A missing ConfigFrom network responds 404
    :id: NET-API-CONFIG-FROM-MISSING-HTTP404
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows

    :api-op:`POST /networks/create` with :api-field:`ConfigFrom` naming a network that does not
    exist responds 404, with a message that contains :api-error:`configuration network "<name>" does
    not exist`. See :ref:`network-common-known-divergences-non-goals`.

.. expectation:: docker network create --config-from with a missing network fails
    :id: NET-API-CONFIG-FROM-MISSING-CLI
    :observed_from: api
    :depends_on: cli
    :platforms: linux, windows
    :cli_for: NET-API-CONFIG-FROM-MISSING

    :cli:`docker network create --config-from` naming a network that does not exist exits non-zero
    and prints the daemon's error.

.. expectation:: Deleting a config-only network that a network uses responds 403
    :id: NET-API-CONFIG-ONLY-DELETE-USED
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A config-only network, and a network that is not Swarm-scoped whose
    :api-field:`ConfigFrom` names it

    :api-op:`DELETE /networks/{id}` for the config-only network responds 403. :api-op:`GET
    /networks` then still lists the config-only network.
    :need:`SWARM-SCOPED-CONFIG-ONLY-DELETE-REFUSED` covers a Swarm-scoped network.

.. expectation:: docker network rm of a config-only network in use fails
    :id: NET-API-CONFIG-ONLY-DELETE-USED-CLI
    :observed_from: api
    :depends_on: cli
    :platforms: linux, windows
    :cli_for: NET-API-CONFIG-ONLY-DELETE-USED

    *Applies to:* A config-only network, and a network that is not Swarm-scoped whose
    :api-field:`ConfigFrom` names it

    :cli:`docker network rm` for the config-only network exits non-zero and prints the daemon's
    error.

.. expectation:: Creating a container on a config-only network responds 201
    :id: NET-API-CONFIG-ONLY-CTR-CREATE
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A config-only network

    :api-op:`POST /containers/create` for a container on that network responds 201. See
    :ref:`network-common-known-divergences-non-goals`.

.. expectation:: A created or stopped container connects to a config-only network
    :id: NET-API-CONFIG-ONLY-CONNECT-STOPPED
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A config-only network, and a created or stopped container that is not attached to
    it

    :api-op:`POST /networks/{id}/connect` to that network, for the container, responds 200. See
    :ref:`network-common-known-divergences-non-goals`.

.. expectation:: A container on a config-only network does not start
    :id: NET-API-CONFIG-ONLY-START
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A created container on a config-only network

    :api-op:`POST /containers/{id}/start` for the container responds with a 4xx or 5xx status. The
    container stays in the created state.

.. expectation:: Starting a container on a config-only network responds 403
    :id: NET-API-CONFIG-ONLY-START-HTTP403
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A created container on a config-only network

    :api-op:`POST /containers/{id}/start` for the container responds 403. The container stays in the
    created state. See :ref:`network-common-known-divergences-non-goals`.

.. expectation:: docker run -d on a config-only network fails
    :id: NET-API-CONFIG-ONLY-START-CLI
    :observed_from: api
    :depends_on: cli
    :platforms: linux, windows
    :cli_for: NET-API-CONFIG-ONLY-START

    *Applies to:* A config-only network

    :cli:`docker run -d --network` naming that network exits non-zero and prints the daemon's error.

.. expectation:: Connecting a running container to a config-only network fails
    :id: NET-API-CONFIG-ONLY-CONNECT-RUNNING
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A running container, and a config-only network

    :api-op:`POST /networks/{id}/connect` to that network, for the container, responds with a 4xx or
    5xx status. :api-op:`GET /networks/{id}` then does not list the container.

.. expectation:: A running container's connect to a config-only network responds 403
    :id: NET-API-CONFIG-ONLY-CONNECT-RUNNING-HTTP403
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A running container, and a config-only network

    :api-op:`POST /networks/{id}/connect` to that network, for the container, responds 403, with a
    message that contains :api-error:`cannot create endpoint on configuration-only network`.

.. expectation:: After a failed config-only connect, the container lists the network
    :id: NET-API-CONFIG-ONLY-CONNECT-RUNNING-LISTED
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A running container, and a config-only network

    After :api-op:`POST /networks/{id}/connect` to that network, for the container, fails,
    :api-op:`GET /containers/{id}/json` does not list the network in
    :api-field:`NetworkSettings.Networks`.

.. expectation:: A failed connect leaves the network out of the container's settings
    :id: NET-API-CONNECT-FAILED-NOT-LISTED
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A running container, and a network that it is not attached to

    After :api-op:`POST /networks/{id}/connect` to that network, for the container, fails, whatever
    the reason, :api-op:`GET /containers/{id}/json` does not list the network in
    :api-field:`NetworkSettings.Networks`, and :api-op:`GET /networks/{id}` does not list the
    container. :need:`A known divergence <NET-API-FAILED-CONNECT-IFACE-LEFT>` covers the interface
    that a failure at the rename leaves in the container.

.. expectation:: docker network connect of a running container to config-only fails
    :id: NET-API-CONFIG-ONLY-CONNECT-RUNNING-CLI
    :observed_from: api
    :depends_on: cli
    :platforms: linux, windows
    :cli_for: NET-API-CONFIG-ONLY-CONNECT-RUNNING

    *Applies to:* A running container, and a config-only network

    :cli:`docker network connect` naming that network and the container exits non-zero and prints
    the daemon's error.

.. _network-common-endpoint-interfaces:

*********************
 Endpoint interfaces
*********************

The expectations in this section hold for an endpoint of any network driver that adds an interface
to the container (:ref:`network-common-driver-properties`). An endpoint requests an interface name
when its :drvopt:`com.docker.network.endpoint.ifname` is set to that name.

.. expectation:: An endpoint's interface has the name that it requests
    :id: NET-IFACE-NAME-REQUESTED
    :observed_from: task, ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows

    *Applies to:* An endpoint that requests an interface name

    The interface has that name. See :ref:`network-common-known-divergences-non-goals`.

.. expectation:: An interface without a requested name is named eth<N>
    :id: NET-IFACE-NAME-DEFAULT
    :observed_from: task, ctr
    :depends_on: os-linux
    :platforms: linux

    *Applies to:* Linux nodes with an endpoint that does not request an interface name, on a network
    of a built-in driver without :drvopt:`com.docker.network.container_iface_prefix`

    The interface name is :netdev:`eth<N>`.

.. expectation:: A container whose requested interface name is refused does not start
    :id: NET-IFACE-NAME-REFUSED-START
    :observed_from: api
    :depends_on: none
    :platforms: linux

    *Applies to:* Linux nodes with a created or stopped container with an endpoint whose
    :drvopt:`com.docker.network.endpoint.ifname` the OS refuses

    :api-op:`POST /containers/{id}/start` for the container responds with a 4xx or 5xx status.

.. expectation:: A connect whose requested interface name is refused fails
    :id: NET-IFACE-NAME-REFUSED-CONNECT
    :observed_from: api
    :depends_on: none
    :platforms: linux

    *Applies to:* Linux nodes with a running container

    :api-op:`POST /networks/{id}/connect` for the container, with an endpoint whose
    :drvopt:`com.docker.network.endpoint.ifname` the OS refuses, responds with a 4xx or 5xx status.

.. expectation:: Starting with a refused interface name responds 500
    :id: NET-IFACE-NAME-REFUSED-START-HTTP500
    :observed_from: api
    :depends_on: none
    :platforms: linux

    *Applies to:* Linux nodes with a created or stopped container with an endpoint whose
    :drvopt:`com.docker.network.endpoint.ifname` the OS refuses

    :api-op:`POST /containers/{id}/start` for the container responds 500, with a message that
    contains :api-error:`error renaming interface` and the requested name. See
    :ref:`network-common-known-divergences-non-goals`.

.. expectation:: A connect with a refused interface name responds 500
    :id: NET-IFACE-NAME-REFUSED-CONNECT-HTTP500
    :observed_from: api
    :depends_on: none
    :platforms: linux

    *Applies to:* Linux nodes with a running container

    :api-op:`POST /networks/{id}/connect` for the container, with an endpoint whose
    :drvopt:`com.docker.network.endpoint.ifname` the OS refuses, responds 500. The message contains
    :api-error:`error renaming interface` and the requested name. See
    :ref:`network-common-known-divergences-non-goals`.

.. expectation:: docker run -d with a refused interface name fails
    :id: NET-IFACE-NAME-REFUSED-START-CLI
    :observed_from: api
    :depends_on: cli
    :platforms: linux
    :cli_for: NET-IFACE-NAME-REFUSED-START

    :cli:`docker run -d` with a :drvopt:`com.docker.network.endpoint.ifname` that the OS refuses
    exits non-zero and prints the daemon's error.

.. expectation:: docker network connect with a refused interface name fails
    :id: NET-IFACE-NAME-REFUSED-CONNECT-CLI
    :observed_from: api
    :depends_on: cli
    :platforms: linux
    :cli_for: NET-IFACE-NAME-REFUSED-CONNECT

    *Applies to:* Linux nodes with a running container

    :cli:`docker network connect` for the container, with a
    :drvopt:`com.docker.network.endpoint.ifname` that the OS refuses, exits non-zero and prints the
    daemon's error.

.. expectation:: A container requesting a duplicate interface name or lo is created
    :id: NET-IFACE-NAME-CONFLICT-CREATE
    :observed_from: api
    :depends_on: none
    :platforms: linux

    At API version 1.44 or later, :api-op:`POST /containers/create` for a container with two
    endpoints that request the same interface name responds 201. So does a request for a container
    with an endpoint that requests :netdev:`lo`.

.. expectation:: A container whose endpoints request the same name does not start
    :id: NET-IFACE-NAME-CONFLICT-START
    :observed_from: api
    :depends_on: none
    :platforms: linux

    *Applies to:* Linux nodes with a created or stopped container with two endpoints that request
    the same interface name

    :api-op:`POST /containers/{id}/start` for the container responds with a 4xx or 5xx status.

.. expectation:: A container with an endpoint that requests lo does not start
    :id: NET-IFACE-NAME-CONFLICT-START-LO
    :observed_from: api
    :depends_on: none
    :platforms: linux

    *Applies to:* Linux nodes with a created or stopped container with an endpoint that requests
    :netdev:`lo`

    :api-op:`POST /containers/{id}/start` for the container responds with a 4xx or 5xx status.

.. expectation:: A connect requesting lo or an existing interface name fails
    :id: NET-IFACE-NAME-CONFLICT-CONNECT
    :observed_from: api
    :depends_on: none
    :platforms: linux

    *Applies to:* Linux nodes with a running container

    :api-op:`POST /networks/{id}/connect` for the container, with an endpoint that requests
    :netdev:`lo`, responds with a 4xx or 5xx status. It does the same with an endpoint that requests
    the name of one of the container's interfaces.

.. expectation:: A duplicate interface name or lo at start responds 500
    :id: NET-IFACE-NAME-CONFLICT-START-HTTP500
    :observed_from: api
    :depends_on: none
    :platforms: linux

    *Applies to:* Linux nodes with a created or stopped container with two endpoints that request
    the same interface name, or with one that requests :netdev:`lo`

    :api-op:`POST /containers/{id}/start` for the container responds 500, with a message that names
    the interface. See :ref:`network-common-known-divergences-non-goals`.

.. expectation:: A connect requesting lo or an existing name responds 500
    :id: NET-IFACE-NAME-CONFLICT-CONNECT-HTTP500
    :observed_from: api
    :depends_on: none
    :platforms: linux

    *Applies to:* Linux nodes with a running container

    :api-op:`POST /networks/{id}/connect` for the container, with an endpoint that requests
    :netdev:`lo` or the name of one of its interfaces, responds 500. The message names the
    interface. See :ref:`network-common-known-divergences-non-goals`.

.. expectation:: docker run -d with a duplicate interface name or lo fails
    :id: NET-IFACE-NAME-CONFLICT-START-CLI
    :observed_from: api
    :depends_on: cli
    :platforms: linux
    :cli_for: NET-IFACE-NAME-CONFLICT-START

    :cli:`docker run -d` exits non-zero and prints the daemon's error when two of the container's
    endpoints request the same interface name. It does the same when an endpoint requests
    :netdev:`lo`.

.. expectation:: docker network connect requesting lo or an existing name fails
    :id: NET-IFACE-NAME-CONFLICT-CONNECT-CLI
    :observed_from: api
    :depends_on: cli
    :platforms: linux
    :cli_for: NET-IFACE-NAME-CONFLICT-CONNECT

    *Applies to:* Linux nodes with a running container

    :cli:`docker network connect` for the container exits non-zero and prints the daemon's error
    when the endpoint requests :netdev:`lo` or the name of one of its interfaces.

.. expectation:: A standalone container's interface has its requested static address
    :id: NET-IFACE-STATIC-ADDRESS
    :observed_from: ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows

    *Applies to:* A running standalone container whose endpoint on a network has an
    :api-field:`IPv4Address` or :api-field:`IPv6Address` in :api-field:`IPAMConfig`

    Its interface on that network has that address.

.. expectation:: A requested static address is reported in the container's settings
    :id: NET-IFACE-STATIC-ADDRESS-INSPECT
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A running standalone container whose endpoint on a network has an
    :api-field:`IPv4Address` or :api-field:`IPv6Address` in :api-field:`IPAMConfig`

    :api-op:`GET /containers/{id}/json` reports the address as :api-field:`IPAddress` or
    :api-field:`GlobalIPv6Address` in :api-field:`NetworkSettings.Networks.<network>`.

.. expectation:: An endpoint MacAddress set at create is the interface's MAC address
    :id: NET-IFACE-MAC-ADDRESS-CREATE
    :observed_from: ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :assumes: NET-DRVPROP-ADDS-INTERFACE, NET-DRVPROP-ENDPOINT-MAC

    *Applies to:* A running container created at API version 1.44 or later with an endpoint
    :api-field:`MacAddress` on a network whose driver accepts an endpoint MAC address

    Its interface on that network has that MAC address.

.. expectation:: An endpoint MacAddress set at connect is the interface's MAC address
    :id: NET-IFACE-MAC-ADDRESS-CONNECT
    :observed_from: ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :assumes: NET-DRVPROP-ADDS-INTERFACE, NET-DRVPROP-ENDPOINT-MAC

    *Applies to:* A running container, and a network whose driver accepts an endpoint MAC address

    After :api-op:`POST /networks/{id}/connect` with :api-field:`MacAddress` set, at API version
    1.54 or later, responds 200, the container's interface on the network has that MAC address.

.. expectation:: A connect with MacAddress fails where the driver does not accept it
    :id: NET-IFACE-MAC-ADDRESS-REFUSED-CONNECT
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows
    :assumes: NET-DRVPROP-ENDPOINT-MAC

    *Applies to:* A running container, and a network whose driver does not accept an endpoint MAC
    address

    :api-op:`POST /networks/{id}/connect` to that network, for the container, with
    :api-field:`MacAddress` set, at API version 1.54 or later, responds with a 4xx or 5xx status.
    :api-op:`GET /networks/{id}` then does not list the container.

.. expectation:: A container with a MacAddress its driver refuses is created, not started
    :id: NET-IFACE-MAC-ADDRESS-REFUSED-START
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows
    :assumes: NET-DRVPROP-ENDPOINT-MAC

    *Applies to:* A network whose driver does not accept an endpoint MAC address
    (:ref:`network-common-driver-properties`)

    At API version 1.44 or later, :api-op:`POST /containers/create` for a container whose endpoint
    on that network sets :api-field:`MacAddress` responds 201. :api-op:`POST /containers/{id}/start`
    for the container then responds with a 4xx or 5xx status, and the container stays in the created
    state.

.. expectation:: docker run -d with --mac-address fails where the driver refuses it
    :id: NET-IFACE-MAC-ADDRESS-REFUSED-START-CLI
    :observed_from: api
    :depends_on: cli
    :platforms: linux, windows
    :assumes: NET-DRVPROP-ENDPOINT-MAC

    :cli:`docker run -d` with :cli-flag:`--mac-address` and with :cli-flag:`--network` naming a
    network whose driver does not accept an endpoint MAC address exits non-zero and prints the
    daemon's error.

.. expectation:: Without macvlan links, starting a macvlan container responds 500
    :id: NET-IFACE-NO-MACVLAN
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :pins: NET-HOST-MACVLAN

    *Applies to:* A Linux host whose kernel does not offer macvlan links, and a created or stopped
    container with an endpoint on a :driver:`macvlan` network

    :api-op:`POST /containers/{id}/start` for the container responds 500, with a message that
    contains :api-error:`failed to create the macvlan port`. See :ref:`Host requirements
    <network-common-host-requirements>`.

.. expectation:: Without VLAN links, a macvlan network on a VLAN parent responds 500
    :id: NET-IFACE-NO-VLAN
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :pins: NET-HOST-VLAN

    *Applies to:* A Linux host whose kernel does not offer VLAN links

    :api-op:`POST /networks/create` for a :driver:`macvlan` network whose :drvopt:`parent` is
    ``<interface>.<VLAN ID>``, for an existing interface ``<interface>`` that does not yet have that
    VLAN sub-interface, responds 500, with a message that contains :api-error:`vlan link`. See
    :ref:`Host requirements <network-common-host-requirements>`.

.. _network-common-default-gateway:

*****************
 Default gateway
*****************

.. expectation:: The daemon gives a container on internal networks only no default route
    :id: NET-GW-INTERNAL-ONLY
    :observed_from: task, ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows

    *Applies to:* A container attached only to ``internal`` networks

    The daemon does not give the container a default route. See
    :ref:`network-common-known-divergences-non-goals`.

.. expectation:: The default route goes through the endpoint with the highest GwPriority
    :id: NET-GW-PRIORITY
    :observed_from: ctr
    :depends_on: os-linux
    :platforms: linux

    *Applies to:* Linux nodes with a container with endpoints on two or more networks that are not
    ``internal``, one with a higher :api-field:`GwPriority` than the others

    For each IP version that the endpoint gives the container a gateway or default route for, the
    container's default route goes through that endpoint.

.. expectation:: With equal GwPriority, the default route follows a fixed endpoint order
    :id: NET-GW-PRIORITY-TIE
    :observed_from: ctr
    :depends_on: none
    :platforms: linux

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

    At API version 1.47 or earlier, :api-op:`POST /containers/create` ignores
    :api-field:`GwPriority` in :api-field:`EndpointsConfig`.

.. expectation:: A connect applies GwPriority at every API version
    :id: NET-GW-PRIORITY-CONNECT
    :observed_from: api, ctr
    :depends_on: none
    :platforms: linux, windows

    :api-op:`POST /networks/{id}/connect` applies :api-field:`GwPriority` at every API version.

.. _network-common-file-etc-hosts:

********************
 :file:`/etc/hosts`
********************

This section holds on Linux nodes.

.. expectation:: /etc/hosts maps each of the container's addresses to its hostname
    :id: NET-HOSTS-ADDRESSES
    :observed_from: task, ctr
    :depends_on: none
    :platforms: linux

    *Applies to:* A container that does not use the host's network

    For each of the container's addresses, :file:`/etc/hosts` has a line that maps the address to
    the container's hostname. The exceptions are addresses on the ingress network and on
    :network:`docker_gwbridge`. IPv6 addresses are also exceptions while IPv6 is disabled in the
    container, so that its loopback interface does not have an IPv6 address, for example with
    :sysctl:`net.ipv6.conf.all.disable_ipv6` set to 1.

.. expectation:: A dotted hostname or a domain gives full and short names in /etc/hosts
    :id: NET-HOSTS-DOMAIN-NAME
    :observed_from: ctr
    :depends_on: none
    :platforms: linux

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

    *Applies to:* A running container, and a network other than the ingress network and
    :network:`docker_gwbridge`

    After :api-op:`POST /networks/{id}/connect` to that network for the container responds 200,
    :file:`/etc/hosts` has the lines for the container's addresses on that network. IPv6 addresses
    are exceptions while IPv6 is disabled in the container, as for the expectation above.

.. expectation:: A disconnect removes the network's addresses from /etc/hosts
    :id: NET-HOSTS-DISCONNECT
    :observed_from: ctr
    :depends_on: none
    :platforms: linux

    *Applies to:* A running container with an endpoint on a network, from its start or from a later
    connect

    After :api-op:`POST /networks/{id}/disconnect` for the container responds 200,
    :file:`/etc/hosts` does not have the lines for the container's addresses on that network.

.. expectation:: A host-gateway entry maps to each address in host-gateway-ips
    :id: NET-HOSTS-HOST-GATEWAY
    :observed_from: ctr
    :depends_on: none
    :platforms: linux

    *Applies to:* A daemon with :daemon-json:`host-gateway-ips` set

    An entry ``<name>:host-gateway`` in :api-field:`HostConfig.ExtraHosts` adds a line to
    :file:`/etc/hosts` for each address in :daemon-json:`host-gateway-ips`. Each line maps the
    address to ``<name>``. IPv6 addresses are exceptions while IPv6 is disabled in the container, as
    for the expectation about the container's addresses.

.. expectation:: Without host-gateway-ips, host-gateway is the default bridge's gateways
    :id: NET-HOSTS-HOST-GATEWAY-DEFAULT-BRIDGE
    :observed_from: ctr
    :depends_on: none
    :platforms: linux

    *Applies to:* A daemon without :daemon-json:`host-gateway-ips`

    An entry ``<name>:host-gateway`` in :api-field:`HostConfig.ExtraHosts` adds a line to
    :file:`/etc/hosts` for each gateway address of the default bridge. This holds whatever networks
    the container is attached to. IPv6 addresses are exceptions while IPv6 is disabled in the
    container, as for the expectation about the container's addresses.

.. expectation:: Without a host-gateway address, starting the container responds 500
    :id: NET-HOSTS-HOST-GATEWAY-NO-BRIDGE
    :observed_from: api
    :depends_on: none
    :platforms: linux

    *Applies to:* A daemon without :daemon-json:`host-gateway-ips` and with :driver:`bridge` set to
    ``none``, and a created container with an entry ``<name>:host-gateway`` in
    :api-field:`HostConfig.ExtraHosts`

    :api-op:`POST /containers/{id}/start` for the container responds 500, with a message that
    contains :api-error:`unable to derive the IP value for host-gateway`. The container stays in the
    created state. See :ref:`network-common-known-divergences-non-goals`.

.. expectation:: docker run with --add-host <name>:host-gateway and no address fails
    :id: NET-HOSTS-HOST-GATEWAY-NO-BRIDGE-CLI
    :observed_from: api
    :depends_on: cli
    :platforms: linux
    :cli_for: NET-HOSTS-HOST-GATEWAY-NO-BRIDGE

    *Applies to:* A daemon without :daemon-json:`host-gateway-ips` and with :driver:`bridge` set to
    ``none``

    :cli:`docker run` with :cli-flag:`--add-host` ``<name>:host-gateway`` exits non-zero and prints the
    daemon's error.

.. _network-common-driver-properties:

*******************
 Driver properties
*******************

Expectations in this spec name a property of a network driver, not the driver itself, where the set
of drivers with the property can change. The two detectors below give the properties of the built-in
drivers. A plugin driver has the properties that it implements. The :driver:`l2tunnel` and
:driver:`ics` drivers have them on platforms that can create those networks.

.. expectation:: Which built-in drivers add an interface to the container
    :id: NET-DRVPROP-ADDS-INTERFACE
    :observed_from: ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :rationale: RAT-NET-DRIVER-PROPERTY-SETS

    A container on a network of one of these drivers has an interface on the network:

    - :network:`bridge`
    - :driver:`overlay`
    - :driver:`macvlan`
    - :driver:`ipvlan`
    - Windows: :driver:`nat`, :driver:`transparent`, :driver:`l2bridge`, :driver:`l2tunnel`, :driver:`ics`

    A container on a network of one of these drivers does not:

    - :driver:`host`
    - :driver:`null` (the :network:`none` network)

.. expectation:: Which built-in drivers accept an endpoint MAC address
    :id: NET-DRVPROP-ENDPOINT-MAC
    :observed_from: ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows
    :rationale: RAT-NET-DRIVER-PROPERTY-SETS

    These drivers accept an endpoint MAC address:

    - :driver:`bridge`
    - :driver:`overlay`
    - :driver:`macvlan`
    - Windows: :driver:`nat`, :driver:`transparent`, :driver:`l2bridge`, :driver:`l2tunnel`, :driver:`ics`

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

    After :signal:`SIGKILL` and restart, every network namespace that the daemon created before the
    kill is removed, other than an overlay network's namespace and the namespace of a container that
    live-restore keeps running. This includes the namespaces of load-balancer and ingress sandboxes,
    and of containers that the daemon does not keep running. An overlay network's namespace is the
    one that holds the network's VXLAN device. A namespace that the daemon creates after the restart
    is a different namespace, even at the same path. A namespace is removed once no mount of it
    remains on the node, even while a process still runs in it. The expectation is checked as soon
    as the daemon's API responds again, without a ceiling, on a node in a Swarm too.
    :need:`SWARM-RESTART-STALE-OVERLAY` and :need:`SWARM-RESTART-STALE-OVERLAY-REMOVAL` say when an
    overlay network's namespace is removed.

.. expectation:: With live-restore, a container keeps its namespace and interfaces
    :id: NET-RESTART-LIVE-RESTORE
    :observed_from: node, ctr
    :depends_on: os-linux
    :platforms: linux

    *Applies to:* Linux nodes with a running container, whose daemon has :daemon-json:`live-restore`
    true

    After :signal:`SIGKILL` and restart, the container keeps its network namespace and its
    interfaces.

.. _network-common-known-divergences-non-goals:

*******************************
 Known divergences & non-goals
*******************************

.. expectation:: HTTP 500 for refused requests
    :id: NET-IFACE-HTTP500-REFUSED
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows

    Each request below is refused on purpose, but it responds 500. 500 is the status for a fault in
    the daemon. The requests are: (1) :api-op:`POST /networks/{id}/disconnect` for a container that
    is not attached to the network. (2) :api-op:`POST /containers/{id}/start` and :api-op:`POST
    /networks/{id}/connect` for an endpoint with a :drvopt:`com.docker.network.endpoint.ifname` that
    the OS refuses, or that another endpoint of the container already uses. (3) :api-op:`POST
    /containers/{id}/start` for a created or stopped container with a ``host-gateway`` entry in
    :api-field:`HostConfig.ExtraHosts`, when the daemon does not have an address for
    ``host-gateway``. (4) On a node in a Swarm, :api-op:`POST /containers/{id}/start` for a created
    or stopped container that :api-op:`POST /networks/{id}/connect` connected to a network that does
    not exist (:ref:`network-common-network-api`, :ref:`swarm-networking-swarm-scoped-networks`,
    :ref:`network-common-endpoint-interfaces`, :ref:`network-common-file-etc-hosts`).

.. expectation:: Networks that a created or stopped container cannot use
    :id: NET-API-STOPPED-CTR-UNUSABLE-NETWORK
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows

    Accepted. For a created or stopped container, :api-op:`POST /networks/{id}/connect` to a network
    that does not exist, or to a config-only network, responds 200. :api-op:`POST
    /containers/create` for a container on a config-only network also responds 201. Only
    :api-op:`POST /containers/{id}/start` refuses the network. For a network that does not exist, it
    responds 404 with :api-error:`network <name> not found` on a node that is not in a Swarm. On a
    node in a Swarm, it responds 500 with a message that contains :api-error:`network <name> not
    found`. For a config-only network, it responds 403 with :api-error:`cannot create endpoint on
    configuration-only network`. If a network with the missing name is created before the start, the
    start attaches the container to it (:ref:`network-common-network-api`).

.. expectation:: Connecting a running container to a network it is already attached to
    :id: NET-API-CONNECT-ALREADY-ATTACHED-TWO-STATUSES
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows
    :rationale: RAT-NET-REPEAT-CONNECT-STATUS-BY-SCOPE

    Refused with two different statuses. The request responds 403 on a network that is not
    Swarm-scoped, and 409 on an attachable Swarm-scoped network (:ref:`network-common-network-api`,
    :ref:`swarm-networking-swarm-scoped-networks`).

.. expectation:: HTTP 404 for a missing ConfigFrom network
    :id: NET-API-HTTP404-CONFIG-FROM-MISSING
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows
    :rationale: RAT-NET-API-HTTP404-CONFIG-FROM-MISSING

    :api-op:`POST /networks/create` with :api-field:`ConfigFrom` naming a network that does not
    exist responds 404. 404 is the status for a request whose own resource does not exist. Here the
    request is valid, and its body names the missing network (:ref:`network-common-network-api`).

.. expectation:: Default route on internal overlays on Windows
    :id: NET-GW-WINDOWS-INTERNAL-OVERLAY-ROUTE
    :observed_from: ctr
    :depends_on: os-windows
    :platforms: windows

    A Windows container attached only to ``internal`` overlays has a default route
    (:ref:`network-common-default-gateway`).

.. expectation:: ifname option on Windows
    :id: NET-IFACE-WINDOWS-NAME-IGNORED
    :observed_from: ctr
    :depends_on: none
    :platforms: windows

    Ignored. A Windows container's interface name does not follow
    :drvopt:`com.docker.network.endpoint.ifname` (:ref:`network-common-endpoint-interfaces`).

.. expectation:: Interface left behind after a failed connect
    :id: NET-API-FAILED-CONNECT-IFACE-LEFT
    :observed_from: ctr
    :depends_on: none
    :platforms: linux

    If :api-op:`POST /networks/{id}/connect` for a running container fails at the rename, the
    interface stays in the container under its generated :link-kind:`veth` name. The interface is
    down and does not have addresses. Its host-side peer stays attached. The interface and the peer
    stay until the container's network namespace is destroyed. A failed connect is meant to leave
    nothing in place (:ref:`network-common-endpoint-interfaces`).

.. expectation:: Interface names the kernel rewrites
    :id: NET-IFACE-NAME-KERNEL-REWRITE
    :observed_from: ctr
    :depends_on: none
    :platforms: linux, windows

    Accepted without an error. For a name that contains ``%d``, the kernel gives the interface the
    first free name that matches, such as :netdev:`eth0`. A later disconnect responds 200, but the
    interface stays in the container. The interface stays up and keeps its address
    (:ref:`network-common-endpoint-interfaces`).

.. expectation:: Interface-name collisions depend on attach order
    :id: NET-IFACE-NAME-COLLISION-ORDER
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows
    :rationale: RAT-NET-IFACE-NAME-COLLISION-ORDER

    Take a container with two endpoints. One endpoint asks for :netdev:`eth0`. The name of the other
    endpoint is generated. The container starts or fails, depending on the order in which its
    endpoints are attached. This order is random. The endpoint that asks for the name is meant to
    get it, whatever the order (:ref:`network-common-endpoint-interfaces`).

.. expectation:: docker_gwbridge outside a Swarm
    :id: NET-API-GWBRIDGE-PRUNED
    :observed_from: api:solo
    :depends_on: none
    :platforms: linux, windows
    :rationale: RAT-NET-GWBRIDGE-NOT-PREDEFINED

    Pruned. :network:`docker_gwbridge` is not a predefined network. On a node that is not in a
    Swarm, :api-op:`POST /networks/prune` deletes it while no running container is attached to it.
    This includes a :network:`docker_gwbridge` that an operator creates before the node joins a
    Swarm, to configure the gateway network (:ref:`network-common-network-api`,
    :ref:`swarm-traffic-gateway-network`).

.. expectation:: dangling filter for Swarm-scoped networks
    :id: NET-API-FILTER-DANGLING-SWARM-SCOPED
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    Wrong. On a manager, :api-op:`GET /networks` with the filter :api-query:`dangling=true` lists
    every Swarm-scoped network, in use or not. A Swarm-scoped network with a local attachment is
    listed for :api-query:`dangling=false` too. A network that services or containers use is meant
    to be listed only for :api-query:`dangling=false`
    (:ref:`swarm-networking-swarm-scoped-networks`).

.. expectation:: Names that are a prefix of a Swarm-scoped network's ID
    :id: NET-API-NAME-SWARM-ID-PREFIX
    :observed_from: api:mgr
    :depends_on: none
    :platforms: linux, windows

    Resolved to the Swarm-scoped network on a manager. On a manager, :api-op:`GET /networks/{id}`
    and :cli:`docker network inspect` with the name of a network that is not Swarm-scoped return a
    Swarm-scoped network whose ID starts with that name. On other nodes, and for networks that are
    not Swarm-scoped, a full-name match takes precedence over a partial ID
    (:ref:`swarm-networking-swarm-scoped-networks`, :ref:`network-common-network-api`).

.. expectation:: Aliases in a service's Hosts
    :id: NET-HOSTS-SERVICE-ALIASES-DROPPED
    :observed_from: task
    :depends_on: none
    :platforms: linux, windows

    Dropped. :api-field:`ContainerSpec.Hosts` takes entries in the form ``<address> <name>
    <alias>...``, as in hosts(5). A task's :file:`/etc/hosts` has a line that maps ``<address>`` to
    ``<name>`` only. The aliases are meant to map to ``<address>`` too
    (:ref:`network-common-file-etc-hosts`).
