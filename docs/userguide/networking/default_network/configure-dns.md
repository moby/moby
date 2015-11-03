<!--[metadata]>
+++
title = "Configure container DNS"
description = "Learn how to configure DNS in Docker"
keywords = ["docker, bridge, docker0, network"]
[menu.main]
parent = "smn_networking_def"
+++
<![end-metadata]-->

# Configure container DNS

The information in this section explains configuring container DNS within
the Docker default bridge. This is a `bridge` network named `bridge` created
automatically when you install Docker.  

**Note**: The [Docker networks feature](../dockernetworks.md) allows you to create user-defined networks in addition to the default bridge network.

How can Docker supply each container with a hostname and DNS configuration, without having to build a custom image with the hostname written inside?  Its trick is to overlay three crucial `/etc` files inside the container with virtual files where it can write fresh information.  You can see this by running `mount` inside a container:

```
$$ mount
...
/dev/disk/by-uuid/1fec...ebdf on /etc/hostname type ext4 ...
/dev/disk/by-uuid/1fec...ebdf on /etc/hosts type ext4 ...
/dev/disk/by-uuid/1fec...ebdf on /etc/resolv.conf type ext4 ...
...
```

This arrangement allows Docker to do clever things like keep `resolv.conf` up to date across all containers when the host machine receives new configuration over DHCP later.  The exact details of how Docker maintains these files inside the container can change from one Docker version to the next, so you should leave the files themselves alone and use the following Docker options instead.

Four different options affect container domain name services.

<table>
  <tr>
    <td>
    <p>
    <code>-h HOSTNAME</code> or <code>--hostname=HOSTNAME</code>
    </p>
    </td>
    <td>
    <p>
      Sets the hostname by which the container knows itself.  This is written
      into <code>/etc/hostname</code>, into <code>/etc/hosts</code> as the name
      of the container's host-facing IP address, and is the name that
      <code>/bin/bash</code> inside the container will display inside its
      prompt.  But the hostname is not easy to see from outside the container.
      It will not appear in <code>docker ps</code> nor in the
      <code>/etc/hosts</code> file of any other container.
    </p>
    </td>
  </tr>
  <tr>
    <td>
    <p>
    <code>--link=CONTAINER_NAME</code> or <code>ID:ALIAS</code>
    </p>
    </td>
    <td>
    <p>
      Using this option as you <code>run</code> a container gives the new
      container's <code>/etc/hosts</code> an extra entry named
      <code>ALIAS</code> that points to the IP address of the container
      identified by <code>CONTAINER_NAME_or_ID<c/ode>. This lets processes
      inside the new container connect to the hostname <code>ALIAS</code>
      without having to know its IP.  The <code>--link=</code> option is
      discussed in more detail below. Because Docker may assign a different IP
      address to the linked containers on restart, Docker updates the
      <code>ALIAS</code> entry in the <code>/etc/hosts</code> file of the
      recipient containers.   
</p>
    </td>
  </tr>
  <tr>
    <td><p>
    <code>--dns=IP_ADDRESS...</code>
    </p></td>
    <td><p>
     Sets the IP addresses added as <code>server</code> lines to the container's
     <code>/etc/resolv.conf</code> file.  Processes in the container, when
     confronted with a hostname not in <code>/etc/hosts</code>, will connect to
     these IP addresses on port 53 looking for name resolution services.     </p></td>
  </tr>
  <tr>
    <td><p>
    <code>--dns-search=DOMAIN...</code>
    </p></td>
    <td><p>
    Sets the domain names that are searched when a bare unqualified hostname is
    used inside of the container, by writing <code>search</code> lines into the
    container's <code>/etc/resolv.conf</code>. When a container process attempts
    to access <code>host</code> and the search domain <code>example.com</code>
    is set, for instance, the DNS logic will not only look up <code>host</code>
    but also <code>host.example.com</code>.
    </p>
    <p>
    Use <code>--dns-search=.</code> if you don't wish to set the search domain.
    </p>
    </td>
  </tr>
  <tr>
    <td><p>
    <code>--dns-opt=OPTION...</code>
    </p></td>
    <td><p>
      Sets the options used by DNS resolvers by writing an <code>options<code>
      line into the container's <code>/etc/resolv.conf<code>.
    </p>
    <p>
    See documentation for <code>resolv.conf<code> for a list of valid options
    </p></td>
  </tr>
  <tr>
    <td><p></p></td>
    <td><p></p></td>
  </tr>
</table>


Regarding DNS settings, in the absence of the `--dns=IP_ADDRESS...`, `--dns-search=DOMAIN...`, or `--dns-opt=OPTION...` options, Docker makes each container's `/etc/resolv.conf` look like the `/etc/resolv.conf` of the host machine (where the `docker` daemon runs).  When creating the container's `/etc/resolv.conf`, the daemon filters out all localhost IP address `nameserver` entries from the host's original file.

Filtering is necessary because all localhost addresses on the host are unreachable from the container's network.  After this filtering, if there  are no more `nameserver` entries left in the container's `/etc/resolv.conf` file, the daemon adds public Google DNS nameservers (8.8.8.8 and 8.8.4.4) to the container's DNS configuration.  If IPv6 is enabled on the daemon, the public IPv6 Google DNS nameservers will also be added (2001:4860:4860::8888 and 2001:4860:4860::8844).

> **Note**: If you need access to a host's localhost resolver, you must modify your DNS service on the host to listen on a non-localhost address that is reachable from within the container.

You might wonder what happens when the host machine's `/etc/resolv.conf` file changes.  The `docker` daemon has a file change notifier active which will watch for changes to the host DNS configuration.

> **Note**: The file change notifier relies on the Linux kernel's inotify feature. Because this feature is currently incompatible with the overlay filesystem  driver, a Docker daemon using "overlay" will not be able to take advantage of the `/etc/resolv.conf` auto-update feature.

When the host file changes, all stopped containers which have a matching `resolv.conf` to the host will be updated immediately to this newest host configuration.  Containers which are running when the host configuration changes will need to stop and start to pick up the host changes due to lack of a facility to ensure atomic writes of the `resolv.conf` file while the container is running. If the container's `resolv.conf` has been edited since it was started with the default configuration, no replacement will be attempted as it would overwrite the changes performed by the container. If the options (`--dns`, `--dns-search`, or `--dns-opt`) have been used to modify the default host configuration, then the replacement with an updated host's `/etc/resolv.conf` will not happen as well.

> **Note**: For containers which were created prior to the implementation of the `/etc/resolv.conf` update feature in Docker 1.5.0: those containers will **not** receive updates when the host `resolv.conf` file changes. Only containers created with Docker 1.5.0 and above will utilize this auto-update feature.
