################################
 ipvlan networking surface area
################################

Scope: the ipvlan network driver, as observable from outside the daemon. Every expectation here
holds on Linux nodes. An expectation about :api-op:`POST /networks/create`, :api-op:`DELETE
/networks/{id}` or :cli:`docker network create` holds for a network that is not Swarm-scoped.

This spec holds expectations that belong to the ipvlan driver. It is not yet a complete account of
the ipvlan driver. :doc:`../README.md <../README>` gives the model that this spec follows: the items
of a spec, their categories and what their verdicts depend on. :doc:`README.md <README>` gives the
conventions of the networking specs, such as vantage points, daemon settings, timing and convergence
ceilings, and host requirements. :doc:`network-common.md <network-common>` covers what every network
driver must do, including the expectations for a driver that does not accept an endpoint MAC
address.

.. _ipvlan-networking-requirements:

**************
 Requirements
**************

.. req:: An ipvlan network without a parent is isolated as an internal network is
    :id: REQ-IPVL-NO-PARENT-INTERNAL
    :source_kind: docs
    :source: https://docs.docker.com/engine/network/drivers/ipvlan/#ipvlan-l2-mode-example-usage

    Leaving out the parent of an ipvlan network has the same result as --internal: it isolates the
    network completely. Containers attached to the network communicate with each other, and with
    nothing external to it.

.. req:: A VLAN parent that the network create adds is deleted with the network
    :id: REQ-IPVL-VLAN-PARENT
    :source_kind: docs
    :source: https://docs.docker.com/engine/network/drivers/ipvlan/#ipvlan-8021q-trunk-l2-mode-example-usage
    :refines: PRIN-OWNS-ONLY-ITS-OWN

    An ipvlan network's parent can be a VLAN sub-interface named <interface>.<VLAN ID>. If the
    sub-interface does not exist, docker network create creates it, and docker network rm deletes
    it. The operator can instead create and manage the sub-interface completely; docker network rm
    deletes only a sub-interface that Docker manages.

.. _ipvlan-networking-host-requirements:

*******************
 Host requirements
*******************

The conventions for this section are in :ref:`README.md <README-host-requirements>`. The ipvlan
driver also needs the host requirements of :ref:`network-common.md
<network-common-host-requirements>`. The daemon needs Linux 4.2 or later for an ipvlan network. It
checks the kernel version, and does not probe for ipvlan links. The ipvlan driver documentation
gives Linux 4.2 as the minimum, and says that support in earlier kernels is buggy.

.. _ipvlan-networking-kernel-facilities:

Kernel facilities
=================

.. hostreq:: ipvlan links in Linux 4.2 or later
    :id: IPVL-HOST-IPVLAN
    :kind: os
    :platforms: linux
    :kernel_options: IPVLAN

    *Facility:* ipvlan links, in Linux 4.2 or later

    *Needed when:* An ipvlan network

.. needextend:: type == "expectation" and docname in ['network/ipvlan-networking'] and ("linux" in platforms) and "IPVL-HOST-IPVLAN" not in pins
    :+needs: IPVL-HOST-IPVLAN

.. hostreq:: Dummy links
    :id: IPVL-HOST-DUMMY
    :kind: os
    :platforms: linux
    :kernel_options: DUMMY
    :tags: no-parent

    *Needed when:* A network without a :drvopt:`parent`

    Not pinned. The error for a missing dummy link does not name the facility.

.. hostreq:: VLAN links
    :id: IPVL-HOST-VLAN
    :kind: os
    :platforms: linux
    :kernel_options: VLAN_8021Q
    :tags: vlan-parent

    *Needed when:* A network with a :drvopt:`parent` of the form ``<interface>.<VLAN ID>``

.. _ipvlan-networking-endpoint-options:

******************
 Endpoint options
******************

.. expectation:: An ipvlan connect with a MacAddress responds 500
    :id: IPVL-OPT-MAC-ADDRESS-REFUSED-CONNECT-HTTP500
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :exact_form_of: NET-IFACE-MAC-ADDRESS-REFUSED-CONNECT
    :pins: IPVL-NETIF-HTTP500-REFUSED

    *Applies to:* An ipvlan network, and a running container that is not attached to it

    :api-op:`POST /networks/{id}/connect` to that network, for the container, with
    :api-field:`MacAddress` set, at API version 1.54 or later, responds 500, with a message that
    contains :api-error:`ipvlan interfaces do not support custom mac address assignment`.

.. expectation:: Starting an ipvlan container with a MacAddress responds 500
    :id: IPVL-OPT-MAC-ADDRESS-REFUSED-START-HTTP500
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :exact_form_of: NET-IFACE-MAC-ADDRESS-REFUSED-START
    :pins: IPVL-NETIF-HTTP500-REFUSED

    *Applies to:* An ipvlan network, and a created or stopped container created at API version 1.44
    or later whose endpoint on that network sets :api-field:`MacAddress`

    :api-op:`POST /containers/{id}/start` for the container responds 500, with a message that
    contains :api-error:`ipvlan interfaces do not support custom mac address assignment`.

.. expectation:: MacAddress on ipvlan endpoints is not checked at create or connect
    :id: IPVL-OPT-MAC-ADDRESS-UNCHECKED
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :concerns: NET-IFACE-MAC-ADDRESS-REFUSED-START, IPVL-OPT-MAC-ADDRESS-REFUSED-START-HTTP500
    :violates: PRIN-FAIL-FAST

    At API version 1.44 or later, :api-op:`POST /containers/create` for a container whose endpoint
    on an ipvlan network sets :api-field:`MacAddress` responds 201. At API version 1.54 or later,
    :api-op:`POST /networks/{id}/connect` to an ipvlan network, for a created or stopped container,
    with :api-field:`MacAddress` set, responds 200. Only :api-op:`POST /containers/{id}/start` for
    the container refuses the endpoint.

.. _ipvlan-networking-networks-and-interfaces:

*************************
 Networks and interfaces
*************************

.. expectation:: On a kernel older than 4.2, creating an ipvlan network responds 500
    :id: IPVL-NETIF-KERNEL-TOO-OLD
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :pins: IPVL-HOST-IPVLAN

    *Applies to:* A Linux host whose kernel version is older than 4.2

    :api-op:`POST /networks/create` for an ipvlan network responds 500, with a message that contains
    :api-error:`kernel version failed to meet the minimum ipvlan kernel requirement`.

.. expectation:: Without ipvlan links, starting an ipvlan container responds 500
    :id: IPVL-NETIF-NO-IPVLAN
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :pins: IPVL-HOST-IPVLAN

    *Applies to:* A Linux host whose kernel does not offer ipvlan links, and a created or stopped
    container with an endpoint on an ipvlan network

    :api-op:`POST /containers/{id}/start` for the container responds 500, with a message that
    contains :api-error:`failed to create the ipvlan port`.

.. expectation:: Without VLAN links, creating an ipvlan network on a VLAN parent responds 500
    :id: IPVL-NETIF-NO-VLAN
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :pins: IPVL-HOST-VLAN

    *Applies to:* A Linux host whose kernel does not offer VLAN links, an existing interface
    ``<interface>``, and a VLAN ID from 1 to 4094, where ``<interface>.<VLAN ID>`` is not an
    interface of the node and is not the :drvopt:`parent` of another ipvlan network that is not
    Swarm-scoped, or of a Swarm-scoped ipvlan network that the node has a local attachment to

    :api-op:`POST /networks/create` for an ipvlan network that uses the default IPAM driver, whose
    :drvopt:`parent` is ``<interface>.<VLAN ID>``, whose :drvopt:`ipvlan_mode` is unset, empty,
    ``l2``, ``l3`` or ``l3s``, and whose :drvopt:`ipvlan_flag` is unset, empty, ``bridge``,
    ``private`` or ``vepa``, responds 500, with a message that contains :api-error:`vlan link`.

.. expectation:: An ipvlan network with an unknown mode or flag is not created
    :id: IPVL-NETIF-MODE-FLAG-INVALID
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :refines: PRIN-FAILS-CLOSED, REQ-NET-NETWORK-CREATE-ERRORS

    :api-op:`POST /networks/create` for an ipvlan network with :drvopt:`ipvlan_mode` set to a
    non-empty value other than ``l2``, ``l3`` or ``l3s`` responds with a 4xx or 5xx status. So does
    a request with :drvopt:`ipvlan_flag` set to a non-empty value other than ``bridge``, ``private``
    or ``vepa``. :api-op:`GET /networks` then does not list a network with the request's
    :api-field:`Name`.

.. expectation:: An unknown ipvlan mode or flag responds 500
    :id: IPVL-NETIF-MODE-FLAG-INVALID-HTTP500
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :exact_form_of: IPVL-NETIF-MODE-FLAG-INVALID
    :pins: IPVL-NETIF-HTTP500-REFUSED

    :api-op:`POST /networks/create` for an ipvlan network with :drvopt:`ipvlan_mode` set to a
    non-empty value other than ``l2``, ``l3`` or ``l3s`` responds 500, with a message that contains
    :api-error:`requested ipvlan mode`. A request with :drvopt:`ipvlan_flag` set to a non-empty
    value other than ``bridge``, ``private`` or ``vepa`` responds 500, with a message that contains
    :api-error:`requested ipvlan flag`.

.. expectation:: An ipvlan network on a parent that the driver refuses is not created
    :id: IPVL-NETIF-PARENT-INVALID
    :observed_from: api
    :depends_on: os-linux
    :platforms: linux
    :refines: PRIN-FAILS-CLOSED, REQ-NET-NETWORK-CREATE-ERRORS

    :api-op:`POST /networks/create` for an ipvlan network responds with a 4xx or 5xx status when its
    :drvopt:`parent` is :netdev:`lo`, or is the :drvopt:`parent` of another ipvlan network that is
    not Swarm-scoped, or of a Swarm-scoped ipvlan network that the node has a local attachment to.
    So does a request whose :drvopt:`parent` is not an interface of the node and is one of these
    names: ``lo.<VLAN ID>``; a name ``<interface>.<VLAN ID>`` of more than 15 characters; or
    ``<interface>.<VLAN ID>`` for an interface ``<interface>`` that already has an 802.1Q VLAN link
    with that VLAN ID under another name. So does a request whose :drvopt:`parent` is non-empty, is
    not an interface of the node, and is not ``<interface>.<VLAN ID>`` for an interface
    ``<interface>`` of the node and a VLAN ID from 1 to 4094. :api-op:`GET /networks` then does not
    list a network with the request's :api-field:`Name`.

.. expectation:: Refused ipvlan MAC address, mode, flag and parent requests respond 500
    :id: IPVL-NETIF-HTTP500-REFUSED
    :observed_from: api
    :depends_on: os-linux
    :platforms: linux
    :concerns: NET-IFACE-MAC-ADDRESS-REFUSED-CONNECT, NET-IFACE-MAC-ADDRESS-REFUSED-START, IPVL-OPT-MAC-ADDRESS-REFUSED-CONNECT-HTTP500, IPVL-OPT-MAC-ADDRESS-REFUSED-START-HTTP500, IPVL-NETIF-MODE-FLAG-INVALID, IPVL-NETIF-MODE-FLAG-INVALID-HTTP500, IPVL-NETIF-PARENT-INVALID
    :violates: REQ-NET-API-ERROR-STATUS

    Each request below is refused, and responds 500. The first request is :api-op:`POST
    /containers/{id}/start` for a container whose endpoint on an ipvlan network sets
    :api-field:`MacAddress`. The second is :api-op:`POST /networks/{id}/connect` to an ipvlan
    network for a running container, with :api-field:`MacAddress` set, at API version 1.54 or later.
    The third is :api-op:`POST /networks/create` for an ipvlan network with an :drvopt:`ipvlan_mode`
    or :drvopt:`ipvlan_flag` value that the driver does not accept, or with a :drvopt:`parent` that
    :need:`IPVL-NETIF-PARENT-INVALID` names.

.. expectation:: A container on an ipvlan network without a parent reaches nothing outside it
    :id: IPVL-NETIF-NO-PARENT-ISOLATED
    :observed_from: ctr
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-IPVL-NO-PARENT-INTERNAL
    :needs: IPVL-HOST-DUMMY

    *Applies to:* A container attached only to an ipvlan network that was created without a
    :drvopt:`parent`

    The container cannot reach addresses outside that network, including every address of the node.

.. expectation:: Containers on an ipvlan network without a parent reach each other
    :id: IPVL-NETIF-NO-PARENT-SAME-NETWORK
    :observed_from: ctr → ctr
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-IPVL-NO-PARENT-INTERNAL
    :needs: IPVL-HOST-DUMMY

    *Applies to:* Two containers on the same node attached to an ipvlan network that was created
    without a :drvopt:`parent`, with :drvopt:`ipvlan_flag` unset, empty or ``bridge``

    Each container reaches the other at that container's address on the network.

.. expectation:: Containers in different subnets of a no-parent ipvlan network do not reach each other
    :id: IPVL-NETIF-NO-PARENT-OTHER-SUBNET
    :observed_from: ctr → ctr
    :depends_on: os-linux
    :platforms: linux
    :narrows: IPVL-NETIF-NO-PARENT-SAME-NETWORK
    :violates: REQ-IPVL-NO-PARENT-INTERNAL
    :needs: IPVL-HOST-DUMMY

    *Applies to:* Two containers on the same node attached to an ipvlan network that was created
    without a :drvopt:`parent` and that has two or more subnets of one IP version, with addresses in
    different subnets of that version

    Neither container reaches the other at the other container's address in that version.

.. expectation:: Creating a network on a missing VLAN parent creates the sub-interface
    :id: IPVL-NETIF-VLAN-PARENT-CREATED
    :observed_from: api, node
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-IPVL-VLAN-PARENT
    :needs: IPVL-HOST-VLAN

    *Applies to:* An interface ``<interface>`` on the node other than :netdev:`lo`, and a VLAN ID
    from 1 to 4094 that no 802.1Q VLAN link on ``<interface>`` has, where ``<interface>.<VLAN ID>``
    has at most 15 characters, is not an interface of the node, and is not the :drvopt:`parent` of
    another ipvlan network that is not Swarm-scoped, or of a Swarm-scoped ipvlan network that the
    node has a local attachment to

    :api-op:`POST /networks/create` for an ipvlan network that uses the default IPAM driver, whose
    :drvopt:`parent` is ``<interface>.<VLAN ID>``, whose :drvopt:`ipvlan_mode` is unset, empty,
    ``l2``, ``l3`` or ``l3s``, and whose :drvopt:`ipvlan_flag` is unset, empty, ``bridge``,
    ``private`` or ``vepa``, responds 201. The node then has an interface ``<interface>.<VLAN ID>``:
    a VLAN link on ``<interface>`` with that VLAN ID.

.. expectation:: A VLAN parent on an interface whose name has a dot responds 500
    :id: IPVL-NETIF-STACKED-VLAN-PARENT-REFUSED
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :narrows: IPVL-NETIF-VLAN-PARENT-CREATED, IPVL-NETIF-NO-VLAN
    :violates: REQ-IPVL-VLAN-PARENT, REQ-NET-API-ERROR-STATUS

    *Applies to:* An interface ``<interface>`` of the node whose name contains a dot, such as a VLAN
    sub-interface, and a name ``<interface>.<VLAN ID>`` that is not an interface of the node and is
    not the :drvopt:`parent` of another ipvlan network that is not Swarm-scoped, or of a
    Swarm-scoped ipvlan network that the node has a local attachment to

    :api-op:`POST /networks/create` for an ipvlan network that uses the default IPAM driver, whose
    :drvopt:`parent` is ``<interface>.<VLAN ID>``, whose :drvopt:`ipvlan_mode` is unset, empty,
    ``l2``, ``l3`` or ``l3s``, and whose :drvopt:`ipvlan_flag` is unset, empty, ``bridge``,
    ``private`` or ``vepa``, responds 500, with a message that contains :api-error:`required
    interface name format is: name.vlan_id`. :api-op:`GET /networks` then does not list a network
    with the request's :api-field:`Name`.

.. expectation:: Deleting the network deletes the VLAN parent that its create added
    :id: IPVL-NETIF-VLAN-PARENT-REMOVED
    :observed_from: api, node
    :depends_on: os-linux
    :platforms: linux
    :refines: REQ-IPVL-VLAN-PARENT, PRIN-NO-LEAKS
    :needs: IPVL-HOST-VLAN

    *Applies to:* An ipvlan network without containers, whose create added its :drvopt:`parent`
    ``<interface>.<VLAN ID>``, where no :driver:`macvlan` network on the node, including a
    Swarm-scoped one that the node has a local attachment to, has that :drvopt:`parent`

    :api-op:`DELETE /networks/{id}` for the network responds 204. The node then does not have the
    interface ``<interface>.<VLAN ID>``.

.. expectation:: Deleting an ipvlan network removes a VLAN parent a macvlan network uses
    :id: IPVL-NETIF-VLAN-PARENT-REMOVED-MACVLAN
    :observed_from: api, node, ctr
    :depends_on: os-linux
    :platforms: linux
    :needs: IPVL-HOST-VLAN, NET-HOST-VLAN, NET-HOST-MACVLAN
    :concerns: IPVL-NETIF-VLAN-PARENT-REMOVED
    :violates: PRIN-FAITHFUL

    *Applies to:* An ipvlan network that is not Swarm-scoped, without containers, whose create added
    its :drvopt:`parent` ``<interface>.<VLAN ID>``, and a :driver:`macvlan` network that is not
    Swarm-scoped, with that :drvopt:`parent` and a running container

    :api-op:`DELETE /networks/{id}` for the ipvlan network responds 204. The node then does not have
    the interface ``<interface>.<VLAN ID>``, and the container has no interface on the macvlan
    network, while :api-op:`GET /networks/{id}` for the macvlan network still lists the container.

.. expectation:: Deleting the network keeps a VLAN parent that existed before it
    :id: IPVL-NETIF-VLAN-PARENT-KEPT
    :observed_from: api, node
    :depends_on: none
    :platforms: linux
    :refines: REQ-IPVL-VLAN-PARENT, PRIN-OWNS-ONLY-ITS-OWN
    :needs: IPVL-HOST-VLAN

    *Applies to:* An ipvlan network without containers, whose :drvopt:`parent` is a VLAN
    sub-interface ``<interface>.<VLAN ID>`` that existed before the network was created

    :api-op:`DELETE /networks/{id}` for the network responds 204. The node then still has the
    interface ``<interface>.<VLAN ID>``.

.. expectation:: Deleting an ipvlan network without a parent removes what its create added
    :id: IPVL-NETIF-NO-PARENT-REMOVED
    :observed_from: api, node
    :depends_on: os-linux
    :platforms: linux
    :refines: PRIN-NO-LEAKS
    :needs: IPVL-HOST-DUMMY

    *Applies to:* An ipvlan network that is not Swarm-scoped, created without a :drvopt:`parent`,
    without containers

    :api-op:`DELETE /networks/{id}` for the network responds 204. The node then has no interface
    that the network's create added.
