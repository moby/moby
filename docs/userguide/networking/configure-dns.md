<!--[metadata]>
+++
title = "Configure container DNS in user-defined networks"
description = "Learn how to configure DNS in user-defined networks"
keywords = ["docker, DNS, network"]
[menu.main]
parent = "smn_networking"
+++
<![end-metadata]-->

# Embedded DNS server in user-defined networks

The information in this section covers the embedded DNS server operation for
containers in user-defined networks. DNS lookup for containers connected to
user-defined networks works differently compared to the containers connected
to `default bridge` network.

> **Note**: In order to maintain backward compatibility, the DNS configuration
> in `default bridge` network is retained with no behavioral change.
> Please refer to the [DNS in default bridge network](default_network/configure-dns.md)
> for more information on DNS configuration in the `default bridge` network.

As of Docker 1.10, the docker daemon implements an embedded DNS server which
provides built-in service discovery for any container created with a valid
`name` or `net-alias` or aliased by `link`. The exact details of how Docker
manages the DNS configurations inside the container can change from one Docker
version to the next. So you should not assume the way the files such as
`/etc/hosts`, `/etc/resolv.conf` are managed inside the containers and leave
the files alone and use the following Docker options instead.

Various container options that affect container domain name services.

<table>
  <tr>
    <td>
    <p>
    <code>--name=CONTAINER-NAME</code>
    </p>
    </td>
    <td>
    <p>
     Container name configured using <code>--name</code> is used to discover a container within
     an user-defined docker network. The embedded DNS server maintains the mapping between
     the container name and its IP address (on the network the container is connected to).
    </p>
    </td>
  </tr>
  <tr>
    <td>
    <p>
    <code>--net-alias=ALIAS</code>
    </p>
    </td>
    <td>
    <p>
     In addition to <code>--name</code> as described above, a container is discovered by one or more 
     of its configured <code>--net-alias</code> (or <code>--alias</code> in <code>docker network connect</code> command)
     within the user-defined network. The embedded DNS server maintains the mapping between
     all of the container aliases and its IP address on a specific user-defined network.
     A container can have different aliases in different networks by using the <code>--alias</code>
     option in <code>docker network connect</code> command.
    </p>
    </td>
  </tr>
  <tr>
    <td>
    <p>
    <code>--link=CONTAINER_NAME:ALIAS</code>
    </p>
    </td>
    <td>
    <p>
      Using this option as you <code>run</code> a container gives the embedded DNS
      an extra entry named <code>ALIAS</code> that points to the IP address
      of the container identified by <code>CONTAINER_NAME</code>. When using <code>--link</code>
      the embedded DNS will guarantee that localized lookup result only on that
      container where the <code>--link</code> is used. This lets processes inside the new container 
      connect to container without without having to know its name or IP.
    </p>
    </td>
  </tr>
  <tr>
    <td><p>
    <code>--dns=[IP_ADDRESS...]</code>
    </p></td>
    <td><p>
     The IP addresses passed via the <code>--dns</code> option is used by the embedded DNS
     server to forward the DNS query if embedded DNS server is unable to resolve a name
     resolution request from the containers.
     These  <code>--dns</code> IP addresses are managed by the embedded DNS server and
     will not be updated in the container's <code>/etc/resolv.conf</code> file.
  </tr>
  <tr>
    <td><p>
    <code>--dns-search=DOMAIN...</code>
    </p></td>
    <td><p>
    Sets the domain names that are searched when a bare unqualified hostname is
    used inside of the container. These <code>--dns-search</code> options are managed by the
    embedded DNS server and will not be updated in the container's <code>/etc/resolv.conf</code> file.
    When a container process attempts to access <code>host</code> and the search
    domain <code>example.com</code> is set, for instance, the DNS logic will not only
    look up <code>host</code> but also <code>host.example.com</code>.
    </p>
    </td>
  </tr>
  <tr>
    <td><p>
    <code>--dns-opt=OPTION...</code>
    </p></td>
    <td><p>
      Sets the options used by DNS resolvers. These options are managed by the embedded
      DNS server and will not be updated in the container's <code>/etc/resolv.conf</code> file.
    </p>
    <p>
    See documentation for <code>resolv.conf</code> for a list of valid options
    </p></td>
  </tr>
</table>


In the absence of the `--dns=IP_ADDRESS...`, `--dns-search=DOMAIN...`, or
`--dns-opt=OPTION...` options, Docker uses the `/etc/resolv.conf` of the
host machine (where the `docker` daemon runs). While doing so the daemon
filters out all localhost IP address `nameserver` entries from the host's
original file.

Filtering is necessary because all localhost addresses on the host are
unreachable from the container's network. After this filtering, if there are
no more `nameserver` entries left in the container's `/etc/resolv.conf` file,
the daemon adds public Google DNS nameservers (8.8.8.8 and 8.8.4.4) to the
container's DNS configuration. If IPv6 is enabled on the daemon, the public
IPv6 Google DNS nameservers will also be added (2001:4860:4860::8888 and
2001:4860:4860::8844).

> **Note**: If you need access to a host's localhost resolver, you must modify
> your DNS service on the host to listen on a non-localhost address that is
> reachable from within the container.
