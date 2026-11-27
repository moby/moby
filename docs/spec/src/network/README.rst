####################################
 Networking behavior specifications
####################################

Each document here specifies a part of Moby's networking as it is observable from outside the
daemon. The documents follow the model that :doc:`../README.md <../README>` gives for every spec:
what a spec states, its items, how an expectation is read and how the items link. This document
gives the conventions that only the networking specs use. It covers these documents:

.. list-table::
    :header-rows: 1

    - - Spec
      - Scope
    - - :doc:`network-common.md <network-common>`
      - What every network driver must do, the Engine API requests for networks and endpoints that
        do not depend on the driver, and the macvlan driver, which has no spec of its own. Not yet
        complete.
    - - :doc:`bridge-networking.md <bridge-networking>`
      - The bridge network driver, and the parts of the daemon's firewall integration that are not
        specific to Swarm. Not yet complete.
    - - :doc:`ipvlan-networking.md <ipvlan-networking>`
      - The ipvlan network driver. Not yet complete.
    - - :doc:`container-dns.md <container-dns>`
      - Name resolution inside a container on a user-defined network, whatever the network's driver.
        Not yet complete.
    - - :doc:`swarm-networking.md <swarm-networking>`
      - Swarm networking: the configuration of the cluster and its nodes, Swarm-scoped networks, the
        overlay network driver's options and networks, and the ingress network
    - - :doc:`swarm-traffic.md <swarm-traffic>`
      - Traffic in a Swarm: what a container observes, reachability between containers, service
        discovery, load balancing, published ports and egress
    - - :doc:`swarm-lifecycle.md <swarm-lifecycle>`
      - A Swarm over time and on the wire: encryption, the wire format, restarts, convergence,
        churn, exhaustion, failure reporting and the firewall integration that is specific to Swarm

In a spec that is not about Swarm, an expectation or defect whose case needs a Swarm, such as a node
in a Swarm, a Swarm-scoped network, a service or a task, sits in a subsection titled "In a Swarm" at
the end of the section that holds the expectations of its subject, with its exact and CLI forms.

.. _readme-nodes:

*******
 Nodes
*******

A Linux node can be a system container: a container that boots the platform's own init system and
userland, with the daemon's package installed, on the platform's kernel. Examples are a
systemd-nspawn machine and a privileged container that runs systemd. The expectations hold on such
nodes as on hosts, when the nodes share a Linux bridge as the physical network between them and the
host's own firewall does not filter the traffic on that bridge. A Windows node is a Windows host or
virtual machine.

.. _readme-settings-and-scope:

********************
 Settings and scope
********************

The iptables backend means that the daemon's :daemon-json:`firewall-backend` is ``iptables``, the
default. The nftables backend means that it is ``nftables``.

An expectation holds with each daemon setting that it does not name as the baseline leaves it
(:ref:`readme-expectations` in :doc:`../README.md <../README>`). The firewall backend is the
exception: an expectation that does not name a backend holds under both, in the same category,
because the nftables backend behaves like the iptables backend except where a spec says otherwise.
An expectation that names the nftables backend has the category that its links give it, like any
other expectation. Where the only source for its behavior says that the behavior may change, as the
documentation of the nftables backend does, the expectation is provisional (:ref:`Categories
<readme-categories>`).

A driver's expectations hold in every scope that the driver supports, local and Swarm, unless an
expectation or its spec says otherwise. An expectation about a standalone container on a
Swarm-scoped network holds where the expectations of :ref:`swarm-networking-swarm-scoped-networks`
in :doc:`swarm-networking.md <swarm-networking>` let the container attach to the network. An
expectation about a network holds whatever options the network has, unless the expectation or its
section names them. The options are those of the network that the expectation names. The options,
the :api-field:`EnableIPv4` and :api-field:`EnableIPv6` settings and the IPAM configuration of a
Swarm-scoped network of a node-local driver are those of the config-only network that its
:api-field:`ConfigFrom` names on the node. Without :api-field:`ConfigFrom`, the network has none of
the :api-field:`Options` given at create. A network created with :api-field:`ConfigOnly` true is not
a network of the driver that the request names: the daemon does not pass it to that driver, and
:api-op:`GET /networks/{id}` reports its :api-field:`Driver` as :driver:`null`, the driver of the
predefined network :network:`none`. An expectation about a network of a driver, or about a request
to create one, does not cover it unless the expectation names it. The :driver:`host` driver has the
predefined network :network:`host` on each node and, in a Swarm, the managers' record of it. A
request to create another network of the host driver that is not Swarm-scoped is refused. A
requirement or an expectation about the networks of a driver, or about a request to create one, does
not cover a request for another network of the host driver, Swarm-scoped or not, unless it names
such a request. A spec can say that a section's expectations hold in several cases, such as the two
ends on the same node or on different nodes, or each pairing of the nodes' platforms. An expectation
then holds in every combination of those cases, the network's options and the daemon settings under
which the expectation holds.

Unless an expectation or its section says otherwise, the rule has these limits. An expectation that
names a case that a limit leaves out, in its state or its statement, such as an ``internal``
network, covers that case:

- The daemon does not publish ports on an ``internal`` network, and does not forward traffic between
  such a network and addresses outside it. An expectation about a published port, or about traffic
  between a container and an address outside the container's networks, holds where the network that
  carries the traffic is not ``internal``. An expectation about a query that the embedded resolver
  forwards upstream to a nameserver outside the querying container's networks holds for a querying
  container that has external access, as :doc:`container-dns.md <container-dns>` defines it.
- An expectation about the addresses that a network allocates, or about its
  :api-field:`Status.IPAM`, holds for a network that uses the default IPAM driver.
- An expectation does not cover a network that a defect says is not allocated, or that a container
  cannot attach to.

.. _readme-timing:

********
 Timing
********

An effect in a Swarm that the general timing rule (:ref:`readme-general-timing` in
:doc:`../README.md <../README>`) does not cover holds within a convergence ceiling. A ceiling is an
item of :ref:`swarm-lifecycle-convergence` in :doc:`swarm-lifecycle.md <swarm-lifecycle>`, which
also says when each kind of change is complete. There is one ceiling for each convergence class. An
expectation whose verdict waits on convergence links each ceiling that bounds it with ``within``.
Where the body of such an expectation mentions a ceiling, it refers to the item, so that the text
shows the ceiling's name and its seconds, as in "allocation ceiling (30 s)". The body writes that
reference as follows:

.. code-block:: rst

    :need:`[[title|lower]] ceiling ([[seconds]] s) <SWARM-CEIL-ALLOCATION>`

A ceiling that an expectation's body refers to is one that the expectation links. Every ceiling
bounds at least one expectation.

A ceiling is timed from the change that the expectation is about, and is the ceiling of that
change's class. The classes are these:

- **Reassignment** (``SWARM-CEIL-REASSIGNMENT``). The effect of a removal is in this class, timed
  from when the request that removes the object responds. So is the effect of an addition, such as a
  new task, service or endpoint, or a task's change to healthy. So is the effect of a change that is
  neither, such as a change of a service's endpoint mode, networks, published ports or aliases. An
  effect on the requesting node that depends on state from other nodes is in this class too, timed
  from when the request responds. An example is the records of the other nodes' endpoints after the
  request gives the node its first local attachment to a Swarm-scoped network. What a node reports
  from records that other nodes publish is in this class. A node's loss of its local attachment to a
  Swarm-scoped network, after its last container on that network stops or disconnects, is a removal,
  timed from when the request responds. A standalone container that stops without a request, because
  its process exits, is a removal too, timed from the container's :api-event:`die` event. A task's
  own state inside its container, such as its interfaces, addresses and routes, is in this class,
  timed from when the task runs. A task's :file:`/etc/resolv.conf`, which the daemon writes before
  the task's process starts, is not: it holds once the task runs, without a ceiling. The same state
  of a standalone container holds once the request that starts the container, or connects it to the
  network, responds. An expectation about such state that is observed from ``ctr`` as well as
  ``task`` and links this ceiling is bound by the ceiling in its task case only.
- **Allocation** (``SWARM-CEIL-ALLOCATION``). The values that the allocator computes and the API
  reports are in this class: the subnets and addresses of a Swarm-scoped network, and the addresses,
  VIPs, published ports and address counts of a service. So is a task's change to unhealthy, the
  exit of its container, or a change of its desired state away from ``running``. Such a change takes
  effect on every node, the task's own node included. The ceiling starts when the task's node emits
  the container's :api-event:`health_status` or :api-event:`die` event, or when :api-op:`GET /tasks`
  first reports the new :api-field:`DesiredState`. What holds once no container of a service is
  running is in this class, from the last of those :api-event:`die` events. The effect of a change
  of leader is in this class, from the change.
- **Node leave and rejoin** (``SWARM-CEIL-REJOIN``). The effects of :api-op:`POST /swarm/init`,
  :api-op:`POST /swarm/join` and :api-op:`POST /swarm/leave`, on the node that sent the request and
  on the other nodes, are in this class, timed from when the request responds. So are the effects of
  a restart of a node's daemon.
- **Failure detection** (``SWARM-CEIL-FAILURE-DETECTION``) and **partition heal**
  (``SWARM-CEIL-PARTITION-HEAL``). What follows from a node partition is in the first class, timed
  from when the partition starts. What follows from its removal is in the second, timed from when
  the partition is removed.

What a Swarm request stored holds once the request responds, on every manager, whichever manager
handled the request. An example is a network or a service that :api-op:`GET /networks` or
:api-op:`GET /services` lists with the settings that the request gave. Only the values that the
allocator computes take the allocation ceiling. Every other effect of a Swarm request, such as a
request about a service, a Swarm-scoped network or a node, holds within the ceiling of its class on
every node, the manager that handled the request included.

An expectation whose verdict is read from traffic also holds within the reassignment ceiling, timed
from when the last endpoint that the traffic involves runs. When the expectation is about a change
of another class, it links both ceilings. An example is an expectation about the outer addresses of
overlay traffic after a node joins: it links the leave-and-rejoin ceiling for the join, and the
reassignment ceiling for the endpoints that send the traffic.

The effect of a firewalld reload, or of firewalld starting or stopping while the daemon runs, holds
within 30 seconds. An expectation about such an effect states what holds from 30 seconds after the
reload, start or stop, as an expectation that links a ceiling does: from then on, its statement
holds at every moment until a later change to the objects that it is about or to firewalld, and an
observation made earlier does not decide its verdict. An expectation about the effect of a firewalld
reload, or of firewalld starting or stopping, covers a firewalld that on that event removes the
iptables rules and chains that the daemon created, as firewalld with its iptables backend does, as
well as one that keeps them. An expectation that names none of these events does not cover a
firewalld reload, start or stop while the daemon runs.

An expectation has no ceiling where nothing converges:

- The state in an expectation's *Applies to* paragraph is a precondition. It holds before the
  expectation's action. The expectation links no ceiling for it, whatever change produced it. An
  *Applies to* paragraph can use a ceiling as a length of time, as in a task that has been running
  for longer than a ceiling. The expectation links that ceiling. The time has passed before the
  action, so the ceiling does not delay the verdict. A statement can also use a ceiling as a length,
  as in a name that does not stop resolving for longer than a ceiling. The expectation links that
  ceiling, and the statement holds by that length over the interval that it states: the verdict does
  not wait for the ceiling to pass.
- An expectation about the state that a task reaches, or the error that it reports, holds when
  :api-op:`GET /tasks` reports that state for the task. It does not hold when :api-op:`GET /tasks`
  reports a state that excludes it: a final state that the task reaches without the expected one,
  such as ``failed`` or ``rejected`` for an expectation that the task runs, or ``running`` for an
  expectation that the task fails or is rejected. While the task is in an earlier state, the verdict
  is not decided. An expectation that says that a task fails, or fails at start, is about the state
  ``failed``. One that says that a task is rejected, or that a node rejects it, is about the state
  ``rejected``. No ceiling bounds how long a task takes to reach a state, which includes pulling its
  image.
- An expectation about a daemon that does not start holds when the daemon's process exits.

An expectation that says that a task stays in a state, such as ``pending``, holds once the
allocation ceiling has passed from the response to the request that created or updated the service,
and links ``SWARM-CEIL-ALLOCATION``.

Some expectations send a request that succeeds only once an earlier effect in the expectation holds.
Such an expectation holds when one attempt at the request, before the ceiling of that effect has
passed, gets the expected response. Any other expectation that links a ceiling states what holds
once the ceilings that it links have passed, each timed from the point that the expectation or this
section gives. From then on, its statement holds at every moment until a later change to the objects
that the expectation is about. This is the same for an expectation that says that something happens
and for an expectation that says that something does not happen. Before then, the expectation states
nothing about the effects that its ceilings bound, so an observation made earlier does not decide
its verdict. A timer override does not shorten a ceiling.

An expectation, or a sentence of one, that says that something never happens, or that something
holds at every moment, for an object that it names holds from the moment its case exists, such as
from when the task runs or the network is created. Examples are no traffic or name for a task that
is not yet healthy, and no forged VXLAN packet accepted. A ceiling that such an expectation links
bounds only the effect of a change that has to converge, such as a new route, a task removed from
load balancing, or state propagated to another node. Before that ceiling has passed, the expectation
states nothing about that effect, and the rest of it holds.

An expectation that says that nothing interrupts something across events, such as a flow, a service
or an answer, holds, for each event, from when the event starts until the ceiling of the event's
class has passed, timed as this section times that event. It links the ceiling of each such class,
besides the reassignment ceiling that traffic takes.

.. _readme-traffic:

*********
 Traffic
*********

Traffic reaches a destination when the destination receives it. A response from the destination
shows that the traffic reached it. Traffic cannot reach a destination when the destination does not
receive it within 5 seconds of the send, whatever the sender gets back. This also holds for an
expectation that says that traffic does not reach a destination, or that a destination is not
reachable. A TCP connection or SCTP association reaches a destination when it is established within
5 seconds, with the protocol's own retransmissions. An expectation that says that UDP traffic
reaches a destination holds when the destination receives datagrams of it, and no two datagrams of
it sent more than 1 second apart are lost together with every datagram of it sent between them. A
datagram lost on its own, or several lost within 1 second, do not make such an expectation false.
The 5-second rule above decides an expectation that says that traffic does not reach a destination.
A destination does not answer when traffic to it gets a refusal or gets no response within 5
seconds. A refusal is a TCP reset or an ICMP unreachable error. An expectation that names the error
holds only for that error. A published port answers when a connection to it reaches a task or
container that the port is published for. When a published port accepts a connection and then closes
it without reaching such a task or container, the port neither answers nor fails to answer. The port
does not answer only when it refuses or gives no response. An expectation that says that a published
port answers within a ceiling holds when every new connection to it, once the ceiling has passed,
reaches such a task or container. An expectation that says that a published port answers throughout
an interval holds when every new connection to it in that interval does. An expectation that says
that traffic reaches a destination throughout an interval holds when every new connection, and the
UDP traffic, that the sender sends to the destination in that interval reaches it. An expectation
that says that a published port is reachable, or stays reachable, says that it answers. For a UDP
port, a new connection is the UDP traffic that a sender sends to the port, and it reaches a task or
container by the rule for UDP traffic.

A port is a TCP, UDP or SCTP port. Its number is from 1 to 65535. An expectation about a port, a
published port or a target port does not cover a request that sets a larger number for it, unless
the expectation names such a number. An expectation about the ports at an address, such as one that
says that traffic reaches a container's published ports and not its other ports, or that it does not
reach a container on any port, is about TCP, UDP and SCTP traffic. It states nothing about traffic
without ports, such as ICMP.

A flow is a TCP connection, an SCTP association, or the UDP datagrams between one address and port
and another address and port, in either direction, each sent within 5 seconds of the one before it.
An expectation about a flow does not cover UDP datagrams between the same addresses and ports that
follow a longer gap, and they do not count as a new flow.

An expectation that says that a flow has no gap in delivery longer than 1 second does not hold when
data written to the flow has not arrived 1 second after it was written, retransmissions included, or
when datagrams of the flow sent more than 1 second apart, and every datagram of the flow sent
between them, are all lost.

.. _readme-observed-from:

***************
 Observed from
***************

The networking specs use these vantage points (:ref:`readme-vantage-points` in :doc:`../README.md
<../README>`):

.. needreport::
    :types:
    :template: /templates/network-observed-from.rst.j2

An expectation's ``observed_from`` lists these vantage points as :ref:`readme-vantage-points` in
:doc:`../README.md <../README>` says, as in ``underlay → task or ctr``: traffic sent from the
underlay, observed in a task or a container. Words after a vantage point say which node it is on, as
in ``task on two nodes``, or what it observes, as in ``underlay (outer destination address)``. An
expectation about the API of any node of a Swarm lists ``api:mgr, api:wkr``. It does not list
``api``, which also covers a daemon that is not in a Swarm. An expectation whose outcome is the
response to :api-op:`POST /swarm/init` or :api-op:`POST /swarm/join` is observed from ``api:solo``.

Where an expectation refers to the node's address, it holds at each address assigned to an interface
in the node's host network namespace that the sender can reach. A client can reach the node's
addresses on the networks that it shares with the node, and an address of the node that the
expectation gives the client a route to. A route that the expectation does not name is not part of
its setup. The sender is the ``client``, or the container or task that sends in an expectation
observed from ``ctr`` or ``task``. From a container or a task, the node's addresses include the
gateway addresses of the node's bridge networks, its own networks' and the others', and do not
include a loopback address, which is the container's own. For a port published on a host address,
the node's address is that address. An expectation that says "any address of the node" holds at
every one of them. Two limits apply at an IPv6 link-local address of the node. An expectation that
says that a published port answers or is reachable does not hold there when the daemon option
:daemon-json:`userland-proxy` is false (:ref:`bridge-networking-port-publishing` in
:doc:`bridge-networking.md <bridge-networking>`). An expectation about the source address that a
container observes does not hold there. An expectation that says that a published port answers or is
reachable holds at an address of the node only when the endpoint that the daemon publishes the port
on has an address of that family. With :daemon-json:`userland-proxy` true, a port published without
a host address, on an endpoint without an IPv6 address, is also reachable at the node's IPv6
addresses, where an expectation about the source address that a container or task observes does not
hold. This holds when the endpoint is the container's IPv4 gateway and the container does not have
an IPv6 gateway, and when the network's :drvopt:`com.docker.network.bridge.host_binding_ipv4`, which
the daemon option :daemon-json:`ip` sets for the default bridge, is unset or 0.0.0.0. An expectation
that names one of these cases, such as an IPv6 address of the node with
:daemon-json:`userland-proxy` false, covers that case.

.. _readme-network-platforms:

***********
 Platforms
***********

An expectation about traffic between nodes holds for each pairing of the two nodes' platforms in
which the node whose behavior it states runs a platform that its ``platforms`` lists. For an
expectation that lists both platforms, these are Linux to Linux, Linux to Windows, Windows to Linux
and Windows to Windows.

These specs cover only Swarms whose managers are Linux nodes, so an expectation about what a manager
does lists ``linux``.

.. _readme-host-requirements:

*******************
 Host requirements
*******************

The networking specs list their host requirements as :ref:`readme-general-host-requirements` in
:doc:`../README.md <../README>` says. The settings that their host requirements use are these:

.. needreport::
    :types:
    :template: /templates/network-when.rst.j2

An expectation that holds under both firewall backends links the requirements of both. What an
expectation itself sets up, and that no setting names, includes an encrypted overlay network, an
IPv6 data path and a published SCTP port.

The case that a host requirement's *Needed when* names is a driver, a driver option or a form of
:drvopt:`parent`, an IPv6 network or data path, an SCTP port, traffic that the node forwards,
traffic to a VIP or to a port that a service publishes in ingress mode, an encrypted network, a
query to the embedded resolver, and the like. An expectation about a network of any driver, or of
every driver with a property, links no driver's requirements. An expectation about a driver's
networks that names none of the driver's forms links only what every network of that driver needs.
An expectation that states the behavior of each of several drivers that it names links the
requirements of each of them.

The requirements of :doc:`network-common.md <network-common>` cover every networking spec, and those
of :doc:`swarm-networking.md <swarm-networking>` cover the three Swarm documents and the "In a
Swarm" subsections of the other specs.

.. toctree::
    :maxdepth: 2

    network-common
    bridge-networking
    ipvlan-networking
    container-dns
    swarm-networking
    swarm-traffic
    swarm-lifecycle
