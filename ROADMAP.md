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
assigned on an issue you'd like to participate in, pinging them on GitHub to offer your help is
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

Over time we have accumulated a lot of functionality in the container runtime
aspect of Moby while also growing in other areas. Much of the container runtime
pieces are now duplicated work available in other, lower level components such
as [containerd](https://containerd.io).

Moby currently only utilizes containerd for basic runtime state management, e.g. starting
and stopping a container, which is what the pre-containerd 1.0 daemon provided.
Now that containerd is a full-fledged container runtime which supports full
container life-cycle management, we would like to start relying more on containerd
and removing the bits in Moby which are now duplicated. This will necessitate
a significant effort to refactor and even remove large parts of Moby's codebase.

Tracking issues:

- [#38043](https://github.com/moby/moby/issues/38043) Proposal: containerd image integration

## 1.2 Image Builder

Work is ongoing to integrate [BuildKit](https://github.com/moby/buildkit) into
Moby and replace the "v0" build implementation. Buildkit offers better cache
management, parallelizable build steps, and better extensibility while also
keeping builds portable, a chief tenent of Moby's builder.

Upon completion of this effort, users will have a builder that performs better
while also being more extensible, enabling users to provide their own custom
syntax which can be either Dockerfile-like or something completely different.

See [buildpacks on buildkit](https://github.com/tonistiigi/buildkit-pack) as an
example of this extensibility.

New features for the builder and Dockerfile should be implemented first in the
BuildKit backend using an external Dockerfile implementation from the container
images. This allows everyone to test and evaluate the feature without upgrading
their daemon. New features should go to the experimental channel first, and can be
part of the `docker/dockerfile:experimental` image. From there they graduate to
`docker/dockerfile:latest` and binary releases. The Dockerfile frontend source
code is temporarily located at
[https://github.com/moby/buildkit/tree/master/frontend/dockerfile](https://github.com/moby/buildkit/tree/master/frontend/dockerfile)
with separate new features defined with go build tags.

Tracking issues:

- [#32925](https://github.com/moby/moby/issues/32925) discussion: builder future: buildkit

## 1.3 Rootless Mode

Running the daemon requires elevated privileges for many tasks. We would like to
support running the daemon as a normal, unprivileged user without requiring `suid`
binaries.

Tracking issues:

- [#37375](https://github.com/moby/moby/issues/37375) Proposal: allow running `dockerd` as an unprivileged user (aka rootless mode)

## 1.4 Testing

Moby has many tests, both unit and integration. Moby needs more tests which can
cover the full spectrum functionality and edge cases out there.

Tests in the `integration-cli` folder should also be migrated into (both in
location and style) the `integration` folder. These newer tests are simpler to
run in isolation, simpler to read, simpler to write, and more fully exercise the
API. Meanwhile tests of the docker CLI should generally live in docker/cli.

Tracking issues:

- [#32866](https://github.com/moby/moby/issues/32866) Replace integration-cli suite with API test suite

## 1.5 Internal decoupling

A lot of work has been done in trying to decouple Moby internals. This process of creating
standalone projects with a well defined function that attract a dedicated community should continue.
As well as integrating `containerd` we would like to integrate [BuildKit](https://github.com/moby/buildkit)
as the next standalone component.
We see gRPC as the natural communication layer between decoupled components.

In addition to pushing out large components into other projects, much of the
internal code structure, and in particular the
["Daemon"](https://godoc.org/github.com/docker/docker/daemon#Daemon) object,
should be split into smaller, more manageable, and more testable components.
