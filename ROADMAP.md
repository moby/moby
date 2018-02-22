Moby Project Roadmap
====================

### How should I use this document?

This document provides description of items that the project decided to prioritize. This should
serve as a reference point for Moby contributors to understand where the project is going, and
help determine if a contribution could be conflicting with some longer term plans.

The fact that a feature isn't listed here doesn't mean that a patch for it will automatically be
refused! We are always happy to receive patches for new cool features we haven't thought about,
or didn't judge to be a priority. Please however understand that such patches might take longer
for us to review.

### How can I help?

Short term objectives are listed in
[Issues](https://github.com/moby/moby/issues?q=is%3Aopen+is%3Aissue+label%3Aroadmap). Our
goal is to split down the workload in such way that anybody can jump in and help. Please comment on
issues if you want to work on it to avoid duplicating effort! Similarly, if a maintainer is already
assigned on an issue you'd like to participate in, pinging him on GitHub to offer your help is
the best way to go.

### How can I add something to the roadmap?

The roadmap process is new to the Moby Project: we are only beginning to structure and document the
project objectives. Our immediate goal is to be more transparent, and work with our community to
focus our efforts on fewer prioritized topics.

We hope to offer in the near future a process allowing anyone to propose a topic to the roadmap, but
we are not quite there yet. For the time being, it is best to discuss with the maintainers on an
issue, in the Slack channel, or in person at the Moby Summits that happen every few months.

# 1. Features and refactoring

## 1.1 Runtime improvements

We introduced [`runC`](https://runc.io) as a standalone low-level tool for container
execution in 2015, the first stage in spinning out parts of the Engine into standalone tools.

As runC continued evolving, and the OCI specification along with it, we created
[`containerd`](https://github.com/containerd/containerd), a daemon to control and monitor `runC`.
In late 2016 this was relaunched as the `containerd` 1.0 track, aiming to provide a common runtime
for the whole spectrum of container systems, including Kubernetes, with wide community support.
This change meant that there was an increased scope for `containerd`, including image management
and storage drivers.

Moby will rely on a long-running `containerd` companion daemon for all container execution
related operations. This could open the door in the future for Engine restarts without interrupting
running containers. The switch over to containerd 1.0 is an important goal for the project, and
will result in a significant simplification of the functions implemented in this repository.

## 1.2 Internal decoupling

A lot of work has been done in trying to decouple Moby internals. This process of creating
standalone projects with a well defined function that attract a dedicated community should continue.
As well as integrating `containerd` we would like to integrate [BuildKit](https://github.com/moby/buildkit)
as the next standalone component.

We see gRPC as the natural communication layer between decoupled components.

## 1.3 Custom assembly tooling

We have been prototyping the Moby [assembly tool](https://github.com/moby/tool) which was originally
developed for LinuxKit and intend to turn it into a more generic packaging and assembly mechanism
that can build not only the default version of Moby, as distribution packages or other useful forms,
but can also build very different container systems, themselves built of cooperating daemons built in
and running in containers. We intend to merge this functionality into this repo.
