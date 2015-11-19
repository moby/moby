<!--[metadata]>
+++
title = "Network configuration"
description = "Docker networking feature is introduced"
keywords = ["network, networking, bridge, docker,  documentation"]
[menu.main]
identifier="smn_networking"
parent= "mn_use_docker"
weight=7
+++
<![end-metadata]-->

# Docker networks feature overview

This sections explains how to use the Docker networks feature. This feature allows users to define their own networks and connect containers to them. Using this feature you can create a network on a single host or a network that spans across multiple hosts.

- [Understand Docker container networks](dockernetworks.md)
- [Work with network commands](work-with-networks.md)
- [Get started with multi-host networking](get-started-overlay.md)

If you are already familiar with Docker's default bridge network, `docker0` that network continues to be supported. It is created automatically in every installation. The default bridge network is also named `bridge`. To see a list of topics related to that network, read the articles listed in the [Docker default bridge network](default_network/index.md).
