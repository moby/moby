<!--[metadata]>
+++
draft=true
title = "Saved text"
keywords = ["docker, bridge, docker0, network"]
[menu.main]
parent = "smn_networking_def"
+++
<![end-metadata]-->

<!--[metadata]>
This content was extracted from the original introduction. We may want to add it back in later under another form. Labeled DRAFT for now. Won't be built.
<![end-metadata]-->


## A Brief introduction to networking and docker
When Docker starts, it creates a virtual interface named `docker0` on the host machine.  It randomly chooses an address and subnet from the private range defined by [RFC 1918](http://tools.ietf.org/html/rfc1918) that are not in use on the host machine, and assigns it to `docker0`. Docker made the choice `172.17.42.1/16` when I started it a few minutes ago, for example -- a 16-bit netmask providing 65,534 addresses for the host machine and its containers. The MAC address is generated using the IP address allocated to the container to avoid ARP collisions, using a range from `02:42:ac:11:00:00` to `02:42:ac:11:ff:ff`.

> **Note:** This document discusses advanced networking configuration and options for Docker. In most cases you won't need this information. If you're looking to get started with a simpler explanation of Docker networking and an introduction to the concept of container linking see the [Docker User Guide](dockerlinks.md).

But `docker0` is no ordinary interface.  It is a virtual _Ethernet bridge_ that automatically forwards packets between any other network interfaces that are attached to it.  This lets containers communicate both with the host machine and with each other.  Every time Docker creates a container, it creates a pair of "peer" interfaces that are like opposite ends of a pipe -- a packet sent on one will be received on the other.  It gives one of the peers to the container to become its `eth0` interface and keeps the other peer, with a unique name like `vethAQI2QT`, out in the namespace of the host machine.  By binding every `veth*` interface to the `docker0` bridge, Docker creates a virtual subnet shared between the host machine and every Docker container.

The remaining sections of this document explain all of the ways that you can use Docker options and -- in advanced cases -- raw Linux networking commands to tweak, supplement, or entirely replace Docker's default networking configuration.

## Editing networking config files
Starting with Docker v.1.2.0, you can now edit `/etc/hosts`, `/etc/hostname` and `/etc/resolve.conf` in a running container. This is useful if you need to install bind or other services that might override one of those files.

Note, however, that changes to these files will not be saved by `docker commit`, nor will they be saved during `docker run`. That means they won't be saved in the image, nor will they persist when a container is restarted; they will only "stick" in a running container.
