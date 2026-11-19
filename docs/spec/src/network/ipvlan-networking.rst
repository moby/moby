################################
 ipvlan networking surface area
################################

Scope: the ipvlan network driver, as observable from outside the daemon. Every expectation here
holds on Linux nodes. An expectation about :api-op:`POST /networks/create` or :cli:`docker network
create` holds for a network that is not Swarm-scoped.

This spec is a stub. It holds expectations that belong to the ipvlan driver. They come from reviews
of the specs' coverage. It is not yet a complete account of the ipvlan driver. :doc:`../README.md
<../README>` gives the model that this spec follows: the items of a spec, their categories and what
their verdicts depend on. :doc:`README.md <README>` gives the conventions of the networking specs,
such as vantage points, daemon settings, timing and convergence ceilings, and host requirements.
:doc:`network-common.md <network-common>` covers what every network driver must do, including the
expectations for a driver that does not accept an endpoint MAC address.

.. _ipvlan-networking-host-requirements:

*******************
 Host requirements
*******************

The conventions for this section are in :ref:`README.md <README-host-requirements>`. The ipvlan
driver also needs the requirements in :ref:`network-common-host-requirements`. The daemon needs
Linux 4.2 or later for an ipvlan network. It checks the kernel version, and does not probe for
ipvlan links. The check guards against defects in the L2 mode of ipvlan before Linux 4.2, which a
probe cannot detect.

.. _ipvlan-networking-kernel-facilities:

Kernel facilities
=================

.. hostreq:: ipvlan links in Linux 4.2 or later
    :id: IPVL-HOST-IPVLAN
    :kind: os
    :platforms: linux
    :kernel_options: IPVLAN

    *Facility:* ipvlan links, in Linux 4.2 or later

    *Needed when:* Always

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

.. expectation:: An ipvlan endpoint with a MacAddress responds 500 at start or connect
    :id: IPVL-OPT-MAC-ADDRESS-REFUSED-CONNECT-HTTP500
    :observed_from: api
    :depends_on: none
    :platforms: linux

    *Applies to:* An ipvlan network

    :api-op:`POST /containers/{id}/start` for a created or stopped container whose endpoint on that
    network sets :api-field:`MacAddress`, created at API version 1.44 or later, responds 500.
    :api-op:`POST /networks/{id}/connect` to that network for a running container, with
    :api-field:`MacAddress` set, at API version 1.54 or later, also responds 500. Both messages
    contain :api-error:`ipvlan interfaces do not support custom mac address assignment`. See
    :ref:`ipvlan-networking-known-divergences-non-goals`.

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
    :api-error:`kernel version failed to meet the minimum ipvlan kernel requirement`. See :ref:`Host
    requirements <ipvlan-networking-host-requirements>`.

.. expectation:: Without ipvlan links, starting an ipvlan container responds 500
    :id: IPVL-NETIF-NO-IPVLAN
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :pins: IPVL-HOST-IPVLAN

    *Applies to:* A Linux host whose kernel does not offer ipvlan links, and a created or stopped
    container with an endpoint on an ipvlan network

    :api-op:`POST /containers/{id}/start` for the container responds 500, with a message that
    contains :api-error:`failed to create the ipvlan port`. See :ref:`Host requirements
    <ipvlan-networking-host-requirements>`.

.. expectation:: Without VLAN links, an ipvlan network on a VLAN parent responds 500
    :id: IPVL-NETIF-NO-VLAN
    :observed_from: api
    :depends_on: none
    :platforms: linux
    :pins: IPVL-HOST-VLAN

    *Applies to:* A Linux host whose kernel does not offer VLAN links

    :api-op:`POST /networks/create` for an ipvlan network whose :drvopt:`parent` is
    ``<interface>.<VLAN ID>``, for an existing interface ``<interface>`` that does not yet have that
    VLAN sub-interface, responds 500, with a message that contains :api-error:`vlan link`. See
    :ref:`Host requirements <ipvlan-networking-host-requirements>`.

.. _ipvlan-networking-known-divergences-non-goals:

*******************************
 Known divergences & non-goals
*******************************

.. expectation:: HTTP 500 for refused requests
    :id: IPVL-NETIF-HTTP500-REFUSED
    :observed_from: api
    :depends_on: none
    :platforms: linux

    The daemon refuses the requests below on purpose, but they respond 500. 500 is the status for a
    fault in the daemon. The first request is :api-op:`POST /containers/{id}/start` for a container
    whose endpoint on an ipvlan network sets :api-field:`MacAddress`. The second is :api-op:`POST
    /networks/{id}/connect` to an ipvlan network for a running container, with
    :api-field:`MacAddress` set, at API version 1.54 or later
    (:ref:`ipvlan-networking-endpoint-options`).
