############################
 Container DNS surface area
############################

Scope: name resolution inside a container on a user-defined network, whatever the network's driver.
This covers the :file:`/etc/resolv.conf` that the daemon writes for the container and the embedded
DNS resolver behind it, as observable from outside the daemon. In this spec, the user-defined
networks are the networks other than the predefined networks, ingress networks and
:network:`docker_gwbridge`. The embedded resolver does not have records for endpoints on an ingress
network. On a Swarm-scoped network of a node-local driver, such as :driver:`bridge`, the
expectations hold for containers on the same node. Each node attaches its containers to a network of
its own, whose subnet can overlap the subnets on other nodes, and the embedded resolver has records
only for the endpoints on its node. On Linux nodes, a :api-field:`Links` alias of the querying
container takes precedence over the records of its networks, and over forwarding, as the
:ref:`container-dns-names-and-answers` expectations about :api-field:`Links` say. The other
expectations of this spec, and the expectations of :ref:`swarm-traffic-service-discovery`, hold for
a name, and for the ``<name>`` of a :dns-name:`<name>.<network>` query, that is not such an alias.

This is a stub. It holds expectations that do not depend on any one driver. They come from the work
on the :doc:`swarm-networking` and from reviews of the specs' coverage. It is not yet a complete
account. :doc:`../README.md <../README>` gives the model that this spec follows: the items of a
spec, their categories and what their verdicts depend on. :doc:`README.md <README>` gives the
conventions of the networking specs, such as vantage points, daemon settings, timing and convergence
ceilings, and host requirements. Where an expectation of :ref:`swarm-traffic-service-discovery` says
something different about the endpoint of a service's task, that expectation holds.

A container sends its DNS queries to the embedded DNS resolver, which is a DNS server that the
daemon runs for the container. The expectations state what the embedded resolver answers, and what
it forwards to upstream nameservers. A name resolves to an address when the embedded resolver
answers a DNS query for the name with that address. A container has external access when its default
route is on a network that is not ``internal``, and replies to its egress traffic reach it. On a
bridge network with :drvopt:`com.docker.network.bridge.enable_ip_masquerade` set to ``false``,
replies can fail to reach it (:ref:`bridge-networking-egress`). The host's :file:`resolv.conf` is
the :file:`/etc/resolv.conf` that the daemon reads, even on a host that runs systemd-resolved.

On Linux nodes, the containers that follow do not use the embedded resolver, and nothing here
applies to them, other than the expectations whose state names the default bridge:

- A container attached only to the default bridge network, that has not had an endpoint on a
  user-defined network since it started.
- A container that uses the host's network.

.. _container-dns-rationales:

************
 Rationales
************

.. rationale:: Why a change to DNS-FWD-HOST-RESOLV-NOT-REGULAR should be deliberate
    :id: RAT-DNS-HOST-RESOLV-UNREADABLE

    It catches a change in how the daemon treats a host /etc/resolv.conf that is a directory, a
    socket or a FIFO, such as treating it as a file without nameservers; that decides whether
    containers on such a host start, and with which upstream servers, so the change should be
    deliberate.

.. rationale:: Why a change to DNS-RESOLV-NOT-UPDATED should be deliberate
    :id: RAT-DNS-RESOLV-WRITTEN-ONCE

    It catches a change that makes the daemon rewrite a running container's /etc/resolv.conf after
    the embedded resolver starts, such as removing 127.0.0.11 when the last user-defined network is
    disconnected; the file is editable at runtime, and a rewrite would replace the user's edits, so
    the change should be deliberate.

.. _container-dns-host-requirements:

*******************
 Host requirements
*******************

These requirements hold on Linux nodes. The conventions for this section are in :ref:`README.md
<README-host-requirements>`. The embedded resolver needs NAT rules in the container's network
namespace under the active firewall backend. It needs them whether or not the daemon's
:program:`iptables` setting is true.

.. _container-dns-kernel-facilities:

Kernel facilities
=================

.. hostreq:: nftables with the NAT expression
    :id: DNS-HOST-NFTABLES-NAT
    :kind: os
    :platforms: linux
    :kernel_options: NF_TABLES, NF_TABLES_IPV4, NFT_NAT, NFT_CHAIN_NAT_IPV4
    :when: nftables-backend

    *Needed when:* The nftables backend

    *Kernel options:* :kconfig:`NF_TABLES`, :kconfig:`NF_TABLES_IPV4`, :kconfig:`NFT_NAT`.
    :kconfig:`NFT_CHAIN_NAT_IPV4` on kernels that have it.

.. needextend:: type == "expectation" and docname in ['network/container-dns', 'network/swarm-networking', 'network/swarm-traffic', 'network/swarm-lifecycle'] and ("linux" in platforms) and ("Names and answers" in sections or "Forwarding" in sections or "Port 53 in the container" in sections or "Service discovery" in sections) and search("(^| )(ctr|task)(,| |$)", observed_from) and "DNS-HOST-NFTABLES-NAT" not in pins
    :+needs: DNS-HOST-NFTABLES-NAT

.. _container-dns-programs:

Programs
========

.. hostreq:: iptables with the nat table and the DNAT and SNAT targets
    :id: DNS-HOST-IPTABLES-COMMAND
    :kind: program
    :platforms: linux
    :when: iptables-backend

    *Facility:* :program:`iptables`, accepting the nat table and the :ipt-target:`DNAT` and
    :ipt-target:`SNAT` targets

    *Needed when:* The iptables backend

.. needextend:: type == "expectation" and docname in ['network/container-dns', 'network/swarm-networking', 'network/swarm-traffic', 'network/swarm-lifecycle'] and ("linux" in platforms) and ("Names and answers" in sections or "Forwarding" in sections or "Port 53 in the container" in sections or "Service discovery" in sections) and search("(^| )(ctr|task)(,| |$)", observed_from) and "DNS-HOST-IPTABLES-COMMAND" not in pins
    :+needs: DNS-HOST-IPTABLES-COMMAND

.. hostreq:: The nft command
    :id: DNS-HOST-NFT-COMMAND
    :kind: program
    :platforms: linux
    :when: nftables-backend

    *Facility:* :program:`nft`

    *Needed when:* The nftables backend

.. needextend:: type == "expectation" and docname in ['network/container-dns', 'network/swarm-networking', 'network/swarm-traffic', 'network/swarm-lifecycle'] and ("linux" in platforms) and ("Names and answers" in sections or "Forwarding" in sections or "Port 53 in the container" in sections or "Service discovery" in sections) and search("(^| )(ctr|task)(,| |$)", observed_from) and "DNS-HOST-NFT-COMMAND" not in pins
    :+needs: DNS-HOST-NFT-COMMAND

The daemon does not depend on which implementation of :program:`iptables` the host has. The kernel
options that the program needs depend on that implementation:

.. hostreq:: Kernel options for iptables
    :id: DNS-HOST-IPTABLES-KERNEL
    :kind: os
    :platforms: linux
    :kernel_options: NETFILTER, NETFILTER_XTABLES, NF_CONNTRACK, NF_NAT, NETFILTER_XT_NAT
    :when: iptables-backend

    The kernel options that :program:`iptables` needs, whichever implementation provides it. *Kernel
    options:* :kconfig:`NETFILTER`, :kconfig:`NETFILTER_XTABLES`, :kconfig:`NF_CONNTRACK`,
    :kconfig:`NF_NAT`, :kconfig:`NETFILTER_XT_NAT`

.. needextend:: type == "expectation" and docname in ['network/container-dns', 'network/swarm-networking', 'network/swarm-traffic', 'network/swarm-lifecycle'] and ("linux" in platforms) and ("Names and answers" in sections or "Forwarding" in sections or "Port 53 in the container" in sections or "Service discovery" in sections) and search("(^| )(ctr|task)(,| |$)", observed_from) and "DNS-HOST-IPTABLES-KERNEL" not in pins
    :+needs: DNS-HOST-IPTABLES-KERNEL

.. hostreq:: Kernel options for iptables from iptables-legacy
    :id: DNS-HOST-IPTABLES-LEGACY-KERNEL
    :kind: os
    :platforms: linux
    :kernel_options: IP_NF_IPTABLES, IP_NF_NAT, IP_NF_IPTABLES_LEGACY, NETFILTER_XTABLES_LEGACY
    :when: iptables-backend, iptables-legacy

    The kernel options that :program:`iptables` needs when iptables-legacy provides it. *Kernel
    options:* :kconfig:`IP_NF_IPTABLES`, :kconfig:`IP_NF_NAT`. :kconfig:`IP_NF_IPTABLES_LEGACY` from
    Linux 6.9, and :kconfig:`NETFILTER_XTABLES_LEGACY` from Linux 6.17.

.. needextend:: type == "expectation" and docname in ['network/container-dns', 'network/swarm-networking', 'network/swarm-traffic', 'network/swarm-lifecycle'] and ("linux" in platforms) and ("Names and answers" in sections or "Forwarding" in sections or "Port 53 in the container" in sections or "Service discovery" in sections) and search("(^| )(ctr|task)(,| |$)", observed_from) and "DNS-HOST-IPTABLES-LEGACY-KERNEL" not in pins
    :+needs: DNS-HOST-IPTABLES-LEGACY-KERNEL

.. hostreq:: Kernel options for iptables from iptables-nft
    :id: DNS-HOST-IPTABLES-NFT-KERNEL
    :kind: os
    :platforms: linux
    :kernel_options: NF_TABLES, NF_TABLES_IPV4, NFT_COMPAT, NFT_NAT, NFT_CHAIN_NAT_IPV4
    :when: iptables-backend, iptables-nft

    The kernel options that :program:`iptables` needs when iptables-nft provides it. *Kernel
    options:* :kconfig:`NF_TABLES`, :kconfig:`NF_TABLES_IPV4`, :kconfig:`NFT_COMPAT`,
    :kconfig:`NFT_NAT`. Before Linux 5.1, :kconfig:`NFT_CHAIN_NAT_IPV4` in place of
    :kconfig:`NFT_NAT`.

.. needextend:: type == "expectation" and docname in ['network/container-dns', 'network/swarm-networking', 'network/swarm-traffic', 'network/swarm-lifecycle'] and ("linux" in platforms) and ("Names and answers" in sections or "Forwarding" in sections or "Port 53 in the container" in sections or "Service discovery" in sections) and search("(^| )(ctr|task)(,| |$)", observed_from) and "DNS-HOST-IPTABLES-NFT-KERNEL" not in pins
    :+needs: DNS-HOST-IPTABLES-NFT-KERNEL

.. _container-dns-host-configuration:

Host configuration
==================

.. hostreq:: External access and host nameservers that resolve public names
    :id: DNS-HOST-EXTERNAL-ACCESS
    :kind: config
    :platforms: linux, windows

    *Facility:* External network access, and nameservers in the host's :file:`resolv.conf` that
    resolve public names

    *Needed when:* Always

.. _container-dns-file-resolv-conf:

*********************
 :file:`resolv.conf`
*********************

This section holds on Linux nodes. Its expectations, other than the expectation about a process's
changes to the file, hold for a :file:`/etc/resolv.conf` that has not changed since the daemon wrote
it.

.. expectation:: The resolv.conf nameserver is 127.0.0.11
    :id: DNS-RESOLV-NAMESERVER
    :observed_from: ctr
    :depends_on: none
    :platforms: linux

    The :file:`/etc/resolv.conf` nameserver is ``127.0.0.11``.

.. expectation:: resolv.conf has the ndots from the container's DNS options
    :id: DNS-RESOLV-NDOTS-CTR
    :observed_from: ctr
    :depends_on: none
    :platforms: linux

    *Applies to:* A container whose DNS options include a valid :resolv-opt:`ndots`

    :file:`/etc/resolv.conf` has the :resolv-opt:`ndots` value from the container's DNS options.

.. expectation:: Without DNS options, resolv.conf has the host's ndots
    :id: DNS-RESOLV-NDOTS-HOST
    :observed_from: ctr
    :depends_on: none
    :platforms: linux

    *Applies to:* A container whose DNS options are empty, and a host :file:`resolv.conf` with a
    valid :resolv-opt:`ndots`

    :file:`/etc/resolv.conf` has the :resolv-opt:`ndots` value from the host's :file:`resolv.conf`.

.. expectation:: With any container DNS option, resolv.conf has none of the host's
    :id: DNS-RESOLV-OPTIONS-NO-HOST
    :observed_from: ctr
    :depends_on: none
    :platforms: linux

    *Applies to:* A container whose DNS options include any option

    :file:`/etc/resolv.conf` has none of the options in the host's :file:`resolv.conf`,
    :resolv-opt:`ndots` included.

.. expectation:: Without a valid ndots in effect, resolv.conf has options ndots:0
    :id: DNS-RESOLV-NDOTS-ZERO
    :observed_from: ctr
    :depends_on: none
    :platforms: linux

    *Applies to:* A container whose options in effect do not include a valid :resolv-opt:`ndots`

    :file:`/etc/resolv.conf` contains :resolv-opt:`options ndots:0`. This applies when the options
    do not set :resolv-opt:`ndots`, and when all of the :resolv-opt:`ndots` values in them are
    invalid.

.. expectation:: resolv.conf has the container's search domains
    :id: DNS-RESOLV-SEARCH-CTR
    :observed_from: ctr
    :depends_on: none
    :platforms: linux

    *Applies to:* A container created with search domains

    :file:`/etc/resolv.conf` has the search domains that the container was created with, other than
    ``.``.

.. expectation:: Without its own search domains, resolv.conf has the daemon's dns-search
    :id: DNS-RESOLV-SEARCH-DAEMON
    :observed_from: ctr
    :depends_on: none
    :platforms: linux

    *Applies to:* A container created without search domains, and a daemon with
    :daemon-json:`dns-search` set

    :file:`/etc/resolv.conf` has the search domains in the daemon's :daemon-json:`dns-search`, other
    than ``.``.

.. expectation:: Without any dns-search, resolv.conf has the host's search list
    :id: DNS-RESOLV-SEARCH-HOST
    :observed_from: ctr
    :depends_on: none
    :platforms: linux

    *Applies to:* A container created without search domains, and a daemon without
    :daemon-json:`dns-search`

    :file:`/etc/resolv.conf` has the search list from the host's :file:`resolv.conf`.

.. expectation:: A search domain of . is left out of resolv.conf
    :id: DNS-RESOLV-SEARCH-DOT
    :observed_from: ctr
    :depends_on: none
    :platforms: linux

    :file:`/etc/resolv.conf` does not contain a search domain of ``.`` given with
    :cli-flag:`--dns-search` or in the daemon's :daemon-json:`dns-search`.

.. expectation:: With . as the only search domain, resolv.conf has no search line
    :id: DNS-RESOLV-SEARCH-DOT-ONLY
    :observed_from: ctr
    :depends_on: none
    :platforms: linux

    *Applies to:* A container whose only search domain, given with :cli-flag:`--dns-search` or in
    the daemon's :daemon-json:`dns-search`, is ``.``

    :file:`/etc/resolv.conf` does not have a :resolv-opt:`search` line, even if the host's
    :file:`resolv.conf` has one.

.. expectation:: Apart from ndots, resolv.conf lists the options as given
    :id: DNS-RESOLV-OPTIONS-AS-GIVEN
    :observed_from: ctr
    :depends_on: none
    :platforms: linux

    Apart from :resolv-opt:`ndots`, :file:`/etc/resolv.conf` lists the options in effect as they
    were given.

.. expectation:: A process's change to resolv.conf is kept, across connects too
    :id: DNS-RESOLV-CTR-EDIT-KEPT
    :observed_from: ctr
    :depends_on: none
    :platforms: linux

    *Applies to:* A running container

    After a process in the container changes the container's :file:`/etc/resolv.conf`, the file
    keeps the process's content. A connect or disconnect of the container does not change the file.

.. expectation:: After a connect from the default bridge, the container resolves names
    :id: DNS-RESOLV-DEFAULT-BRIDGE-CONNECT
    :observed_from: ctr
    :depends_on: none
    :platforms: linux

    *Applies to:* A host whose :file:`/etc/resolv.conf` names a nameserver other than 127.0.0.53,
    and a running container attached only to the default bridge, created without :cli-flag:`--dns`
    nameservers, whose daemon does not have a :daemon-json:`dns` setting, and whose
    :file:`/etc/resolv.conf` has not changed since the daemon wrote it

    After :api-op:`POST /networks/{id}/connect` to a user-defined network responds 200, the
    container resolves the names of the containers on that network. It forwards the queries that
    :ref:`container-dns-forwarding` says it forwards upstream to the nameservers in the host's
    :file:`resolv.conf`, loopback nameservers included. See
    :ref:`container-dns-known-divergences-non-goals`.

.. expectation:: After a connect from the default bridge, the nameserver is 127.0.0.11
    :id: DNS-RESOLV-DEFAULT-BRIDGE-CONNECT-NAMESERVER
    :observed_from: ctr
    :depends_on: none
    :platforms: linux

    *Applies to:* A running container attached only to the default bridge, whose
    :file:`/etc/resolv.conf` has not changed since the daemon wrote it

    After :api-op:`POST /networks/{id}/connect` to a user-defined network responds 200, the
    :file:`/etc/resolv.conf` nameserver is ``127.0.0.11``.

.. expectation:: A container started on a user-defined network keeps its resolv.conf
    :id: DNS-RESOLV-UNCHANGED-USER-DEFINED-START
    :observed_from: ctr
    :depends_on: none
    :platforms: linux

    *Applies to:* A running container that started with an endpoint on a user-defined network

    The container's :file:`/etc/resolv.conf` keeps the content that it had at start. This holds
    after later connects and disconnects.

.. expectation:: After a connect from the default bridge, resolv.conf stops changing
    :id: DNS-RESOLV-UNCHANGED-AFTER-CONNECT
    :observed_from: ctr
    :depends_on: none
    :platforms: linux

    *Applies to:* A running container that started attached only to the default bridge, and that a
    later connect attached to a user-defined network

    The container's :file:`/etc/resolv.conf` keeps the content that it had after that connect. This
    holds after later connects and disconnects.

.. expectation:: The embedded resolver keeps answering after the last disconnect
    :id: DNS-RESOLV-RESOLVER-AFTER-DISCONNECT
    :observed_from: ctr
    :depends_on: none
    :platforms: linux

    *Applies to:* A running container that has had an endpoint on a user-defined network at any time
    since it started

    After the container disconnects from every user-defined network, the embedded resolver continues
    to answer the container's queries. See :ref:`container-dns-known-divergences-non-goals`.

A container's DNS options are the options that it was created with. If it was created without DNS
options, they are the daemon's :daemon-json:`dns-opts`. The options in effect are the container's
DNS options when it has any. Otherwise, they are the options in the host's :file:`resolv.conf`. A
valid :resolv-opt:`ndots` is an integer of 0 or more. :file:`/etc/resolv.conf` keeps a valid
:resolv-opt:`ndots` above 15 as given, although glibc treats any :resolv-opt:`ndots` above 15 as 15.

.. _container-dns-names-and-answers:

*******************
 Names and answers
*******************

In this section, the querying container is the container that sends the DNS query. The embedded
resolver has records for a name on a network when an endpoint on that network has that name. On an
overlay network, the endpoint of a service's task counts only while the task is running, as
:ref:`swarm-traffic-service-discovery` defines it. An endpoint's names are its container's name,
short ID and hostname, and its network aliases. In :dns-name:`<name>.<network>`, ``<name>`` stands
for a name and ``<network>`` for the name of a network. Both are placeholders, not literal text.

An expectation in this section names ``none`` when the embedded DNS resolver answers the query from
its own records. It does this the same way on every platform.

.. expectation:: A container's name resolves to its address on a shared network
    :id: DNS-NAMES-CTR-NAME
    :observed_from: ctr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A container on a user-defined network that it shares with the querying container,
    where no other network of the querying container has an endpoint with that container's name

    The container's name resolves to its address on the shared network. See
    :ref:`container-dns-known-divergences-non-goals`.

.. expectation:: Names and aliases resolve whatever the case of the query
    :id: DNS-NAMES-CASE
    :observed_from: ctr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A container on a user-defined network that it shares with the querying container

    Its name and its network aliases resolve whatever the case of the query: a container named
    :dns-name:`Web` resolves as :dns-name:`web` and as :dns-name:`WEB`.

.. expectation:: A network alias resolves on its network only
    :id: DNS-NAMES-ALIAS
    :observed_from: ctr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A network alias given to a container's endpoint, where no other network of the
    querying container has an endpoint with that alias

    The alias resolves to the container's address on that network, for containers on that network
    only.

.. expectation:: An alias that several containers share resolves to all of them
    :id: DNS-NAMES-ALIAS-SHARED
    :observed_from: ctr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* Several containers on a network whose endpoints have the same network alias, where
    no other network of the querying container has an endpoint with that alias

    The alias resolves to the addresses of all of those containers on that network.

.. expectation:: A container's short ID and hostname resolve as its name does
    :id: DNS-NAMES-SHORT-ID-HOSTNAME
    :observed_from: ctr
    :depends_on: none
    :platforms: linux, windows

    A container's short ID (the first 12 hex digits of its ID) and its hostname resolve as its name
    does.

.. expectation:: DNSNames lists the name and then the aliases, which all resolve
    :id: DNS-NAMES-DNSNAMES
    :observed_from: api, ctr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A container on a user-defined network

    :api-op:`GET /containers/{id}/json` lists in
    :api-field:`NetworkSettings.Networks.<network>.DNSNames` the container's name, and then the
    aliases of its endpoint on that network. The name and each alias resolve on that network.

.. expectation:: DNSNames lists the short ID and the hostname after the aliases
    :id: DNS-NAMES-DNSNAMES-SHORT-ID
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A container on a user-defined network

    :api-field:`NetworkSettings.Networks.<network>.DNSNames` in :api-op:`GET /containers/{id}/json`
    lists the container's short ID and then its hostname after the aliases. It does not list a name
    more than once.

.. expectation:: After a rename, DNSNames has the new name and not the old one
    :id: DNS-NAMES-DNSNAMES-RENAME
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A running container whose hostname is not its name

    After :api-op:`POST /containers/{id}/rename` for the container responds 204,
    :api-field:`NetworkSettings.Networks.<network>.DNSNames` in :api-op:`GET /containers/{id}/json`
    has the new name for each user-defined network. It does not have the old name. Its other entries
    do not change.

.. expectation:: DNSNames on the default bridge is empty
    :id: DNS-NAMES-DNSNAMES-DEFAULT-BRIDGE
    :observed_from: api
    :depends_on: none
    :platforms: linux

    *Applies to:* Linux nodes with a container attached to the default bridge

    :api-field:`NetworkSettings.Networks.bridge.DNSNames` in :api-op:`GET /containers/{id}/json` is
    empty.

.. expectation:: After a rename, DNSNames on the default bridge lists the new names
    :id: DNS-NAMES-DNSNAMES-DEFAULT-BRIDGE-RENAME
    :observed_from: api
    :depends_on: none
    :platforms: linux

    *Applies to:* Linux nodes with a running container attached to the default bridge

    After :api-op:`POST /containers/{id}/rename` for the container responds 204,
    :api-field:`NetworkSettings.Networks.bridge.DNSNames` lists the new name, the short ID and, when
    it differs from the short ID, the hostname. See
    :ref:`container-dns-known-divergences-non-goals`.

.. expectation:: A name resolves if it exists on any of the querying container's networks
    :id: DNS-NAMES-ANY-NETWORK
    :observed_from: ctr
    :depends_on: none
    :platforms: linux, windows

    A name resolves when an endpoint with that name exists on any user-defined network that the
    querying container is attached to.

.. expectation:: A query gets no address from a network the querying container is not on
    :id: DNS-NAMES-OTHER-NETWORK
    :observed_from: ctr
    :depends_on: none
    :platforms: linux, windows

    A query for a name does not get the address of an endpoint on a network that the querying
    container is not attached to.

.. expectation:: After a start, a container's name and aliases resolve on its networks
    :id: DNS-NAMES-START
    :observed_from: ctr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A created or stopped standalone container

    After :api-op:`POST /containers/{id}/start` for the container responds 204, its name and its
    aliases on each of its user-defined networks resolve for the other containers on that network.
    This holds whether or not the container has a healthcheck.

.. expectation:: After a connect, a container's name and aliases resolve on the network
    :id: DNS-NAMES-CONNECT
    :observed_from: ctr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A running standalone container

    After :api-op:`POST /networks/{id}/connect` to a user-defined network for the container responds
    200, its name and its aliases on that network resolve for the other containers on the network.
    This holds whether or not the container has a healthcheck.

.. expectation:: After a disconnect, a container's name and aliases stop resolving there
    :id: DNS-NAMES-DISCONNECT
    :observed_from: ctr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A running container with an endpoint on a user-defined network

    After :api-op:`POST /networks/{id}/disconnect` for the container from that network responds 200,
    the other containers on that network do not resolve its name or aliases to its address on that
    network.

.. expectation:: After a stop, a container's name and aliases stop resolving
    :id: DNS-NAMES-STOP
    :observed_from: ctr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A running container with endpoints on user-defined networks

    After :api-op:`POST /containers/{id}/stop` for the container responds 204, the other containers
    on each of those networks do not resolve its name or aliases to its address on that network.

.. expectation:: After a rename, the new name resolves and the old one does not
    :id: DNS-NAMES-RENAME
    :observed_from: ctr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A node that is not in a Swarm, and a running container whose hostname is not its
    name

    After :api-op:`POST /containers/{id}/rename` for the container responds 204, its new name
    resolves and its old name does not resolve to its address, for containers on each of its
    networks.

.. expectation:: In a Swarm, a rename applies on a multi-host network on its own node
    :id: DNS-NAMES-RENAME-SWARM
    :observed_from: ctr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A node in a Swarm, and a running container whose hostname is not its name, with an
    endpoint on a Swarm-scoped network of a multi-host driver, such as :driver:`overlay`

    After :api-op:`POST /containers/{id}/rename` for the container responds 204, its new name
    resolves and its old name does not resolve to its address, for containers on that network on its
    node. See :ref:`container-dns-known-divergences-non-goals`. :need:`SWARM-SD-RENAME` covers
    containers on other nodes.

.. expectation:: A and AAAA queries for a known name are answered from its records
    :id: DNS-NAMES-ADDRESS-RECORDS
    :observed_from: ctr
    :depends_on: none
    :platforms: linux, windows

    The embedded resolver answers an A or AAAA query for a name that it has records for from those
    records. This holds for a single-label name too, whatever :resolv-opt:`ndots` the options in
    effect set (:ref:`container-dns-file-resolv-conf`).

.. expectation:: An AAAA query returns the container's IPv6 address on a shared network
    :id: DNS-NAMES-AAAA
    :observed_from: ctr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A container on a network with IPv6 that it shares with the querying container,
    other than an overlay network, where no other network of the querying container has an endpoint
    with that container's name

    An AAAA query for that name returns the container's IPv6 address on the shared network.

.. expectation:: A query of the address type a name lacks gets an empty NOERROR
    :id: DNS-NAMES-ADDRESS-TYPE-MISSING
    :observed_from: ctr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* Records in the embedded resolver for a name that are all A records, or all AAAA
    records

    A query of the other type for that name gets an empty :rcode:`NOERROR` response. The embedded
    resolver does not forward this query upstream.

.. expectation:: An MX query for a known name gets an empty NOERROR response
    :id: DNS-NAMES-MX
    :observed_from: ctr
    :depends_on: none
    :platforms: linux, windows

    An MX query for a name that the embedded resolver has A or AAAA records for gets an empty
    :rcode:`NOERROR` response. The embedded resolver does not forward this query upstream.

.. expectation:: Queries of types other than A, AAAA, MX, PTR and SRV are forwarded
    :id: DNS-NAMES-QUERY-TYPE-FORWARDED
    :observed_from: ctr
    :depends_on: none
    :platforms: linux, windows

    The embedded resolver forwards upstream a query of a type other than A, AAAA, MX, PTR and SRV.
    This holds for a name that it has records for too.

.. expectation:: Outside a Swarm, A and AAAA answers come from the first network only
    :id: DNS-NAMES-NETWORK-ORDER
    :observed_from: ctr
    :depends_on: none
    :platforms: linux

    *Applies to:* A Linux node that is not in a Swarm

    Several of the querying container's networks can have records for a name. The embedded resolver
    answers an A or AAAA query for that name from the records of the first network only. The order
    is: (1) Endpoints with a higher :api-field:`GwPriority` first. (2) Networks that are not
    ``internal`` first. (3) Endpoints with a gateway or default route for more IP versions first.
    (4) Network names in byte order. When that network does not have an address of the family asked
    for, the query gets an empty :rcode:`NOERROR` response. This holds even if another network has
    one.

.. expectation:: Answers list all addresses unless the UDP size limit truncates them
    :id: DNS-NAMES-ANSWER-SIZE
    :observed_from: ctr
    :depends_on: none
    :platforms: linux, windows

    Over UDP, an answer from the embedded resolver's own records lists all of the name's addresses
    on the network that it answers from when it fits in a size limit. The limit is the larger of 512
    bytes and the query's EDNS0 buffer size. When the answer does not fit, the embedded resolver
    truncates it and sets the TC bit. Over TCP, the answer lists all of those addresses.

.. expectation:: The order of a name's addresses varies from one answer to the next
    :id: DNS-NAMES-ADDRESS-ORDER
    :observed_from: ctr
    :depends_on: none
    :platforms: linux, windows

    In answers from the embedded resolver's own records, the order of a name's addresses varies from
    one query to the next.

.. expectation:: Answers from the embedded resolver's records have a TTL of 600 seconds
    :id: DNS-NAMES-TTL
    :observed_from: ctr
    :depends_on: none
    :platforms: linux, windows

    The records in an answer from the embedded resolver's own records have a TTL of 600 seconds.
    This holds for A, AAAA and PTR answers.

.. expectation:: Reverse lookups resolve the addresses of containers on shared networks
    :id: DNS-NAMES-PTR
    :observed_from: ctr
    :depends_on: none
    :platforms: linux, windows

    Reverse (PTR) lookups resolve for the IPv4 and IPv6 addresses of containers on the querying
    container's user-defined networks.

.. expectation:: A reverse lookup's <name>.<network> resolves forward to the address
    :id: DNS-NAMES-PTR-ROUND-TRIP
    :observed_from: ctr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A container's address whose reverse lookup returns :dns-name:`<name>.<network>.`,
    where no network of the querying container has records for :dns-name:`<name>.<network>`

    A forward lookup of :dns-name:`<name>.<network>` returns that address.

.. expectation:: A reverse lookup answers <name>.<network>. with the container name
    :id: DNS-NAMES-PTR-FORM
    :observed_from: ctr
    :depends_on: none
    :platforms: linux, windows

    The answer to a reverse lookup of a container's address is :dns-name:`<name>.<network>.`, where
    ``<name>`` is the container's name and ``<network>`` is the network's name. It is never an
    alias, the short ID or the hostname.

.. expectation:: <name>.<network> resolves to the addresses of <name> on that network
    :id: DNS-NAMES-NETWORK-QUALIFIED
    :observed_from: ctr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A querying container attached to the network ``<network>``, and records for
    ``<name>`` on that network, where no network of the querying container has records for
    :dns-name:`<name>.<network>`

    A query for :dns-name:`<name>.<network>` resolves to the addresses of ``<name>`` on that network
    only. See :ref:`container-dns-known-divergences-non-goals`.

.. expectation:: In <name>.<network>, the network part matches only in the same case
    :id: DNS-NAMES-NETWORK-QUALIFIED-CASE
    :observed_from: ctr
    :depends_on: none
    :platforms: linux, windows

    In a :dns-name:`<name>.<network>` query, ``<network>`` matches a network only when its case is
    the same as the case of the network's name. ``<name>`` matches whatever its case.

.. expectation:: <name>.<network> is forwarded when the querying container is not on it
    :id: DNS-NAMES-NETWORK-QUALIFIED-NOT-ATTACHED
    :observed_from: ctr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A network whose name is a valid DNS label and that the querying container is not
    attached to, where no network of the querying container has records for
    :dns-name:`<name>.<network>`, ``<network>`` being that network's name

    The embedded resolver forwards a query for :dns-name:`<name>.<network>` upstream.

.. expectation:: An endpoint named <name>.<network> wins over <name> on <network>
    :id: DNS-NAMES-NETWORK-QUALIFIED-WHOLE-NAME
    :observed_from: ctr
    :depends_on: none
    :platforms: linux, windows

    *Applies to:* A querying container attached to the network ``<network>``, where an endpoint is
    named ``<name>``, and records for :dns-name:`<name>.<network>` on one of the querying
    container's networks

    :dns-name:`<name>.<network>` resolves from the records for the whole name. It does not resolve
    to the address of ``<name>`` on the network ``<network>``.

.. expectation:: A link alias resolves to the linked container for the linking one only
    :id: DNS-NAMES-LINK-ALIAS
    :observed_from: ctr
    :depends_on: none
    :platforms: linux

    *Applies to:* Linux nodes with a container with ``<name>:<alias>`` in the :api-field:`Links` of
    its endpoint settings for a user-defined network

    The alias resolves to the named container's address on that network. It resolves only for
    queries from the container that has the link.

.. expectation:: A link alias takes precedence over names on the container's networks
    :id: DNS-NAMES-LINK-ALIAS-PRECEDENCE
    :observed_from: ctr
    :depends_on: none
    :platforms: linux

    An alias in the :api-field:`Links` of any of the querying container's endpoints takes precedence
    over the names and network aliases on the container's networks. The alias resolves to the linked
    container even when another container on one of those networks has that name.

.. _container-dns-forwarding:

************
 Forwarding
************

.. expectation:: Public DNS names resolve
    :id: DNS-FWD-PUBLIC-NAMES
    :observed_from: ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows

    *Applies to:* A container that has external access and was created without :cli-flag:`--dns`
    nameservers, whose daemon does not have a :daemon-json:`dns` setting

    Public DNS names resolve.

.. expectation:: An internal-only container never queries unreachable or host nameservers
    :id: DNS-FWD-INTERNAL
    :observed_from: ctr
    :depends_on: none
    :platforms: linux

    *Applies to:* Linux nodes with a container attached only to ``internal`` networks

    The embedded resolver does not forward the container's queries to any nameserver that the
    container cannot reach from its own network namespace. It also does not forward them to the
    nameservers in the host's :file:`resolv.conf`.

.. expectation:: Unknown names get SERVFAIL in an internal-only container without --dns
    :id: DNS-FWD-INTERNAL-SERVFAIL
    :observed_from: ctr
    :depends_on: none
    :platforms: linux

    *Applies to:* Linux nodes with a container attached only to ``internal`` networks and created
    without :cli-flag:`--dns` nameservers, whose daemon does not have a :daemon-json:`dns` setting

    A query for a name that the embedded resolver does not have a record for gets SERVFAIL. When the
    options in effect set :resolv-opt:`ndots`, a single-label A or AAAA query gets an empty
    :rcode:`NOERROR` response instead (:ref:`container-dns-file-resolv-conf`,
    :need:`DNS-FWD-NDOTS-ZERO-SINGLE-LABEL`).

.. expectation:: The embedded resolver forwards to the --dns nameservers
    :id: DNS-FWD-CTR-NAMESERVERS
    :observed_from: ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows

    *Applies to:* A container that has external access and was created with :cli-flag:`--dns`
    nameservers

    The embedded resolver forwards to the nameservers set with :cli-flag:`--dns`, other than
    ``127.0.0.11``, in place of the host's nameservers.

.. expectation:: Without --dns, the embedded resolver forwards to the daemon's dns
    :id: DNS-FWD-DAEMON-NAMESERVERS
    :observed_from: ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows

    *Applies to:* A daemon with a :daemon-json:`dns` setting, and a container that has external
    access and was created without :cli-flag:`--dns` nameservers

    The embedded resolver forwards to the nameservers in the daemon's :daemon-json:`dns` setting,
    other than ``127.0.0.11``, in place of the host's nameservers.

.. expectation:: An internal-only container resolves through a --dns nameserver
    :id: DNS-FWD-INTERNAL-CTR-NAMESERVER
    :observed_from: ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows

    *Applies to:* A container attached only to ``internal`` networks and created with
    :cli-flag:`--dns` set to the address of a nameserver in another container on one of those
    networks

    The container created with :cli-flag:`--dns` resolves names through that nameserver.

.. expectation:: An internal-only container resolves through the daemon's dns nameserver
    :id: DNS-FWD-INTERNAL-DAEMON-NAMESERVER
    :observed_from: ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows

    *Applies to:* A daemon with a :daemon-json:`dns` setting that holds the address of a nameserver
    in a container on an ``internal`` network, and another container attached only to ``internal``
    networks, that network among them, and created without :cli-flag:`--dns` nameservers

    The container created without :cli-flag:`--dns` nameservers resolves names through that
    nameserver.

.. expectation:: Without dns settings, the host's nameservers are queried from the host
    :id: DNS-FWD-HOST-NAMESERVERS
    :observed_from: ctr
    :depends_on: os-linux
    :platforms: linux

    *Applies to:* Linux nodes with a host :file:`resolv.conf` that names a nameserver by a valid IP
    address, and a container that started with an endpoint on a user-defined network, has external
    access and was created without :cli-flag:`--dns` nameservers, whose daemon does not have a
    :daemon-json:`dns` setting

    The embedded resolver forwards to the nameservers in the host's :file:`resolv.conf`, loopback
    nameservers included. It queries them from the host's network namespace.

.. expectation:: With no host nameserver, the embedded resolver uses 127.0.0.1 and ::1
    :id: DNS-FWD-HOST-NAMESERVERS-NONE
    :observed_from: ctr
    :depends_on: os-linux
    :platforms: linux

    *Applies to:* Linux nodes with a host :file:`resolv.conf` that does not name a nameserver by a
    valid IP address, whether the file does not have a :resolv-opt:`nameserver` line with one, is
    missing, or is not a regular file, such as a broken symlink, a directory or a FIFO, and a
    container that started with an endpoint on a user-defined network, has external access and was
    created without :cli-flag:`--dns` nameservers, whose daemon does not have a :daemon-json:`dns`
    setting

    The embedded resolver forwards to ``127.0.0.1`` and ``::1``. It queries them from the host's
    network namespace.

.. expectation:: Configured nameservers are queried from the container's namespace
    :id: DNS-FWD-CONFIGURED-NAMESERVERS-CTR-NS
    :observed_from: ctr
    :depends_on: os-linux
    :platforms: linux

    The embedded resolver queries nameservers set with :cli-flag:`--dns` or in the daemon's
    :daemon-json:`dns` setting from the container's network namespace.

.. expectation:: No response, SERVFAIL or REFUSED moves the query to the next nameserver
    :id: DNS-FWD-FAILOVER
    :observed_from: ctr
    :depends_on: none
    :platforms: linux, windows

    The embedded resolver forwards a query to the next nameserver when the current nameserver does
    not respond, or responds SERVFAIL or REFUSED. When each nameserver that it forwards to does not
    respond, or responds SERVFAIL or REFUSED, the client gets SERVFAIL.

.. expectation:: Over UDP, SERVFAIL comes within 4 seconds per unresponsive nameserver
    :id: DNS-FWD-FAILOVER-TIMEOUT
    :observed_from: ctr
    :depends_on: none
    :platforms: linux, windows

    Over UDP, when no nameserver that the embedded resolver forwards a query to responds, the client
    gets SERVFAIL within 4 seconds times the number of those nameservers.

.. expectation:: Any other nameserver response, NXDOMAIN included, goes to the client
    :id: DNS-FWD-RESPONSE-RETURNED
    :observed_from: ctr
    :depends_on: none
    :platforms: linux, windows

    The embedded resolver returns a nameserver's response other than SERVFAIL or REFUSED, NXDOMAIN
    included, to the client. It does not ask the next nameserver.

.. expectation:: The embedded resolver forwards to at most the first three nameservers
    :id: DNS-FWD-NAMESERVER-LIMIT
    :observed_from: ctr
    :depends_on: none
    :platforms: linux, windows

    The embedded resolver forwards to at most the first three nameservers.

.. expectation:: A configured nameserver 127.0.0.11 is skipped and not counted
    :id: DNS-FWD-NAMESERVER-SELF
    :observed_from: ctr
    :depends_on: none
    :platforms: linux

    The embedded resolver does not forward to a nameserver ``127.0.0.11`` set with :cli-flag:`--dns`
    or in the daemon's :daemon-json:`dns` setting. That nameserver does not count toward the three
    nameservers.

.. expectation:: With 127.0.0.11 as the only nameserver, unknown names get SERVFAIL
    :id: DNS-FWD-NAMESERVER-SELF-ONLY
    :observed_from: ctr
    :depends_on: none
    :platforms: linux

    *Applies to:* Linux nodes with a container whose only nameserver, set with :cli-flag:`--dns` or
    in the daemon's :daemon-json:`dns` setting, is ``127.0.0.11``

    A query for a name that the embedded resolver does not have a record for gets SERVFAIL. When the
    options in effect set :resolv-opt:`ndots`, a single-label A or AAAA query gets an empty
    :rcode:`NOERROR` response instead (:ref:`container-dns-file-resolv-conf`,
    :need:`DNS-FWD-NDOTS-ZERO-SINGLE-LABEL`).

.. expectation:: An unknown multi-label A or AAAA query is forwarded upstream
    :id: DNS-FWD-MULTI-LABEL
    :observed_from: ctr
    :depends_on: none
    :platforms: linux, windows

    The embedded resolver forwards upstream an A or AAAA query for a multi-label name that does not
    match any container, service or alias on the querying container's networks. This does not hold
    for :dns-name:`<name>.<network>` when the querying container is attached to a network named
    ``<network>`` and the embedded resolver has records for ``<name>`` on it
    (:need:`DNS-NAMES-NETWORK-QUALIFIED`).

.. expectation:: An unknown single-label name is forwarded on Windows or without ndots
    :id: DNS-FWD-SINGLE-LABEL
    :observed_from: ctr
    :depends_on: none
    :platforms: linux

    *Applies to:* A container on a Windows node, or a container on a Linux node whose options in
    effect, per :ref:`container-dns-file-resolv-conf`, do not set :resolv-opt:`ndots`

    The embedded resolver forwards a single-label A or AAAA query upstream when it does not have a
    record for the name.

.. expectation:: With ndots of 1 or more, an unknown single-label name gets empty NOERROR
    :id: DNS-FWD-SINGLE-LABEL-NDOTS
    :observed_from: ctr
    :depends_on: none
    :platforms: linux

    *Applies to:* A container on a Linux node whose options in effect, per
    :ref:`container-dns-file-resolv-conf`, set :resolv-opt:`ndots` to 1 or more

    A single-label A or AAAA query gets an empty :rcode:`NOERROR` response when the embedded
    resolver does not have a record for the name. The embedded resolver does not forward the query
    upstream. See :ref:`container-dns-known-divergences-non-goals`.

The expectation for an :resolv-opt:`ndots` of 1 or more is a detector because the behavior itself is
in question. `moby/moby#53157 <https://github.com/moby/moby/issues/53157>`__ proposes to put
container names in a domain on the search list. That change would make this behavior unnecessary. An
:resolv-opt:`ndots` of 0, or an invalid :resolv-opt:`ndots`, has the same effect. That effect is a
bug for these values. See :ref:`container-dns-known-divergences-non-goals`.

.. _container-dns-port-53-in-the-container:

**************************
 Port 53 in the container
**************************

This section holds on Linux nodes.

.. expectation:: A process binds port 53 on 127.0.0.11 without error and gets no packets
    :id: DNS-PORT53-BIND
    :observed_from: ctr
    :depends_on: os-linux
    :platforms: linux

    A process in the container can bind UDP and TCP port 53 on ``127.0.0.11`` without an error. No
    packet reaches that socket.

.. expectation:: Without the iptables command, queries to 127.0.0.11 get no answer
    :id: DNS-PORT53-NO-IPTABLES
    :observed_from: ctr
    :depends_on: os-linux
    :platforms: linux
    :pins: BR-HOST-IPTABLES-COMMAND, DNS-HOST-IPTABLES-COMMAND

    *Applies to:* A daemon with the iptables backend and :program:`iptables` false, on a host
    without the :program:`iptables` command, and a running container with an endpoint on a
    user-defined network

    A DNS query from the container to ``127.0.0.11`` does not get an answer. See :ref:`Host
    requirements <container-dns-host-requirements>`.

.. expectation:: Without the iptables command, the daemon logs the DNAT/SNAT failure
    :id: DNS-PORT53-NO-IPTABLES-LOG
    :observed_from: log
    :depends_on: none
    :platforms: linux
    :pins: BR-HOST-IPTABLES-COMMAND, DNS-HOST-IPTABLES-COMMAND

    *Applies to:* A daemon with the iptables backend and :program:`iptables` false, on a host
    without the :program:`iptables` command, and a running container with an endpoint on a
    user-defined network

    The daemon log contains :log:`setting up DNAT/SNAT rules failed`. See :ref:`Host requirements
    <container-dns-host-requirements>`.

.. _container-dns-known-divergences-non-goals:

*******************************
 Known divergences & non-goals
*******************************

.. expectation:: Single-label forwarding with an ndots of 0 or an invalid ndots
    :id: DNS-FWD-NDOTS-ZERO-SINGLE-LABEL
    :observed_from: ctr
    :depends_on: none
    :platforms: linux, windows

    Not forwarded. This applies when the options in effect set :resolv-opt:`ndots` to 0 or to an
    invalid value. A single-label A or AAAA query then gets an empty :rcode:`NOERROR` response when
    the embedded resolver does not have a record for the name. This is the same as for an
    :resolv-opt:`ndots` of 1 or more (:ref:`container-dns-forwarding`). Only an :resolv-opt:`ndots`
    of 1 or more is meant to have that effect (`moby/moby#53104
    <https://github.com/moby/moby/issues/53104>`__). The effect shows in Docker-in-Docker. The inner
    daemon reads the outer container's :file:`resolv.conf` as the :file:`resolv.conf` of its host.
    The inner daemon finds the :resolv-opt:`ndots:0` that the outer daemon added. The containers of
    the inner daemon cannot resolve names that only the outer resolver knows.

.. expectation:: Single-label queries after a live restore
    :id: DNS-FWD-LIVE-RESTORE-SINGLE-LABEL
    :observed_from: ctr
    :depends_on: none
    :platforms: linux, windows

    Forwarded. After the daemon restarts with :daemon-json:`live-restore` true, the embedded
    resolver forwards upstream a single-label A or AAAA query that it does not have a record for,
    from a container that kept running. This occurs whatever :resolv-opt:`ndots` the options in
    effect set (:ref:`container-dns-forwarding`).

.. expectation:: Renaming a container on a Swarm node
    :id: DNS-NAMES-RENAME-SWARM-NODE
    :observed_from: ctr
    :depends_on: none
    :platforms: linux, windows

    Not applied. On a node in a Swarm, :api-op:`POST /containers/{id}/rename` changes the
    container's records only on Swarm-scoped networks of multi-host drivers, such as :driver:`overlay`. It
    does not change them on other networks, such as user-defined bridge networks and Swarm-scoped
    networks of node-local drivers. The container's new name does not resolve there. Its old name
    continues to resolve there after the rename. The old name continues to resolve there after the
    container is removed (:ref:`container-dns-names-and-answers`).

.. expectation:: Records after a node leaves a Swarm
    :id: DNS-NAMES-SWARM-LEAVE-RECORDS
    :observed_from: ctr
    :depends_on: none
    :platforms: linux, windows

    Lost. Take a node in a Swarm, whether it created the Swarm with :api-op:`POST /swarm/init` or
    joined it, that leaves it with :api-op:`POST /swarm/leave`. After the leave, the names of the
    containers that were running on the node's user-defined networks before the leave stop resolving
    there, and so do reverse lookups of their addresses. A container that starts after the leave
    gets records that resolve (:ref:`container-dns-names-and-answers`).

.. expectation:: resolv.conf after the embedded resolver starts
    :id: DNS-RESOLV-NOT-UPDATED
    :observed_from: ctr
    :depends_on: none
    :platforms: linux, windows
    :rationale: RAT-DNS-RESOLV-WRITTEN-ONCE

    Not updated. After the embedded resolver starts for a container, :file:`/etc/resolv.conf` does
    not change again while the container runs. A container that disconnects from every user-defined
    network keeps :resolv-opt:`nameserver 127.0.0.11` and the embedded resolver. A container started
    on the default bridge alone gets the host's nameservers (:ref:`container-dns-file-resolv-conf`).

.. expectation:: Host resolv.conf that is a directory, a socket or a FIFO
    :id: DNS-FWD-HOST-RESOLV-NOT-REGULAR
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows
    :rationale: RAT-DNS-HOST-RESOLV-UNREADABLE

    Not handled as a file without nameservers. On a Linux node whose :file:`/etc/resolv.conf` is a
    directory, :api-op:`POST /containers/{id}/start` for a container that uses the embedded resolver
    fails with an error that contains :api-error:`is a directory`. When the file is a socket, the
    error contains :api-error:`no such device or address`. When it is a FIFO, the daemon does not
    finish starting: its API does not respond (:ref:`container-dns-forwarding`).

.. expectation:: Upstream nameservers for a container whose NetworkMode is the default bridge
    :id: DNS-FWD-DEFAULT-BRIDGE-CONNECT-UPSTREAM
    :observed_from: ctr
    :depends_on: os-linux, os-windows
    :platforms: linux, windows

    On a Linux node whose :file:`/etc/resolv.conf` names only 127.0.0.53, take a container whose
    :api-field:`HostConfig.NetworkMode` is the default bridge, created without :cli-flag:`--dns`
    nameservers, whose daemon does not have a :daemon-json:`dns` setting, and that has an endpoint
    on a user-defined network, from its start or after a connect. :api-op:`POST /containers/create`
    sets that :api-field:`NetworkMode` when the request does not give one, even when
    :api-field:`EndpointsConfig` names only a user-defined network. The container forwards to the
    nameservers in :file:`/run/systemd/resolve/resolv.conf`, not to 127.0.0.53
    (:ref:`container-dns-file-resolv-conf`, :ref:`container-dns-forwarding`).

.. expectation:: DNSNames on the default bridge after a rename
    :id: DNS-NAMES-DNSNAMES-DEFAULT-BRIDGE-RENAME-FILLED
    :observed_from: api
    :depends_on: none
    :platforms: linux, windows

    Filled in. On Linux nodes, :api-op:`GET /containers/{id}/json` does not report
    :api-field:`DNSNames` for the default bridge, which does not have name resolution. After a
    rename of the running container, it reports the new name, the short ID and the hostname there.
    These names do not resolve for a container attached to the default bridge alone. On a node that
    is not in a Swarm, a container attached to the default bridge and a user-defined network
    resolves the new name and the short ID to the renamed container's address on the default bridge
    (:ref:`container-dns-names-and-answers`).

.. expectation:: Reverse-lookup round trip on Windows
    :id: DNS-NAMES-WINDOWS-PTR-ROUND-TRIP
    :observed_from: ctr
    :depends_on: none
    :platforms: windows

    Fails. A reverse lookup returns :dns-name:`<name>.<network>`. The embedded resolver on a Windows
    node does not answer names in that form. It forwards a query for such a name upstream
    (:ref:`container-dns-names-and-answers`).

.. expectation:: Nameservers on internal networks on Windows
    :id: DNS-FWD-WINDOWS-INTERNAL-NAMESERVERS
    :observed_from: ctr
    :depends_on: none
    :platforms: windows

    Not applied. A Windows endpoint on an ``internal`` network does not get the nameservers set with
    :cli-flag:`--dns` or in the daemon's :daemon-json:`dns` setting. A Windows container attached
    only to ``internal`` networks cannot resolve names through a nameserver on one of them
    (:ref:`container-dns-forwarding`).

.. _container-dns-deliberately-not-asserted:

***************************
 Deliberately not asserted
***************************

- The comments the daemon writes into :file:`resolv.conf`, such as its header and its record of
  where :resolv-opt:`ndots` came from.
- The ports that the embedded resolver listens on.
- The firewall rules that the daemon programs in a container's network namespace: whether they
  exist, their content and names, and the hooks and priorities of their base chains.
