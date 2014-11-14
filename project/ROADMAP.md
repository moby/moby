# Docker: Statement of Direction

This document is a high-level overview of where we want to take Docker.
It is a curated selection of planned improvements which are either important, difficult, or both.

For a more complete view of planned and requested improvements, see [the Github issues](https://github.com/docker/docker/issues).

To suggest changes to the roadmap, including additions, please write the change as if it were already in effect, and make a pull request.


## Orchestration

Orchestration touches on several aspects of multi-container applications.  These include provisioning hosts with the Docker daemon, organizing and maintaining multiple Docker hosts as a cluster, composing an application using multiple containers, and handling the networking between the containers across the hosts.

Today, users accomplish this using a combination of glue scripts and various tools, like Shipper, Deis, Pipeworks, etc.

We want the Docker API to support all aspects of orchestration natively, so that these tools can cleanly and seamlessly integrate into the Docker user experience, and remain interoperable with each other.

## Networking

The current Docker networking model works for communication between containers all residing on the same host.  Since Docker applications in production are made up of many containers deployed across multiple hosts (and sometimes multiple data centers), Docker’s networking model will evolve to accommodate this.  An aspect of this evolution includes providing a Networking API to enable alternative implementations.

## Storage

Currently, stateful Docker containers are pinned to specific hosts during their lifetime.  To support additional resiliency, capacity management, and load balancing we want to enable live stateful containers to dynamically migrate between hosts.  While the Docker Project will provide a “batteries included” implementation for a great out-of-box experience, we will also provide an API for alternative implementations.

## Microsoft Windows

The next Microsoft Windows Server will ship with primitives to support container-based process isolation and resource management.  The Docker Project will guide contributors and maintainers developing native Microsoft versions of the Docker Remote API client and Docker daemon to take advantage of these primitives.

## Provenance

When assembling Docker applications we want users to be confident that images they didn’t create themselves are safe to use and build upon.  Provenance gives users the capability to digitally verify the inputs and processes constituting an image’s origins and lifecycle events.

## Plugin API

We want Docker to run everywhere, and to integrate with every devops tool. Those are ambitious goals, and the only way to reach them is with the Docker community. For the community to participate fully, we need an API which allows Docker to be deeply and easily customized.

We are working on a plugin API which will make Docker very customization-friendly. We believe it will facilitate the integrations listed above – and many more we didn’t even think about.

## Multi-Architecture Support

Our goal is to make Docker run everywhere. However, currently Docker only runs on x86_64 systems. We plan on expanding architecture support, so that Docker containers can be created and used on more architectures, including ARM, Joyent SmartOS, and Microsoft.
