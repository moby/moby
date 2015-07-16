# Roadmap

The Distribution Project consists of several components, some of which are still being defined. This document defines the high-level goals of the project, identifies the current components, and defines the release-relationship to the Docker Platform.

* [Distribution Goals](#distribution-goals)
* [Distribution Components](#distribution-components)
* [Project Planning](#project-planning): release-relationship to the Docker Platform.

## Distribution Goals

- Replace the existing [docker registry](github.com/docker/docker-registry)
  implementation as the primary implementation.
- Replace the existing push and pull code in the docker engine with the
  distribution package.
- Define a strong data model for distributing docker images
- Provide a flexible distribution tool kit for use in the docker platform
- Unlock new distribution models

## Distribution Components

Components of the Distribution Project are managed via github [milestones](https://github.com/docker/distribution/milestones). Upcoming
features and bugfixes for a component will be added to the relevant milestone. If a feature or
bugfix is not part of a milestone, it is currently unscheduled for
implementation. 

* [Registry](#registry)
* [Distribution Package](#distribution-package)

***

### Registry

Registry 2.0 is the first release of the next-generation registry. This is primarily
focused on implementing the [new registry
API](https://github.com/docker/distribution/blob/master/docs/spec/api.md), with
a focus on security and performance.

#### Registry 2.0

Features:

- Faster push and pull
- New, more efficient implementation
- Simplified deployment
- Full API specification for V2 protocol
- Pluggable storage system (s3, azure, filesystem and inmemory supported)
- Immutable manifest references ([#46](https://github.com/docker/distribution/issues/46))
- Webhook notification system ([#42](https://github.com/docker/distribution/issues/42))
- Native TLS Support ([#132](https://github.com/docker/distribution/pull/132))
- Pluggable authentication system
- Health Checks ([#230](https://github.com/docker/distribution/pull/230))

#### Registry 2.1

Planned Features:

> **NOTE:** This feature list is incomplete at this time.

- Support for Manifest V2, Schema 2 and explicit tagging objects ([#62](https://github.com/docker/distribution/issues/62), [#173](https://github.com/docker/distribution/issues/173))
- Mirroring ([#19](https://github.com/docker/distribution/issues/19))
- Flexible client package based on distribution interfaces ([#193](https://github.com/docker/distribution/issues/193)

#### Registry 2.2

TBD

***

### Distribution Package 

At its core, the Distribution Project is a set of Go packages that make up
Distribution Components. At this time, most of these packages make up the
Registry implementation. 

The package itself is considered unstable. If you're using it, please take care to vendor the dependent version. 

For feature additions, please see the Registry section. In the future, we may break out a
separate Roadmap for distribution-specific features that apply to more than
just the registry.

***

### Project Planning

Distribution Components map to Docker Platform Releases via the use of labels. Project Pages are used to define the set of features that are included in each Docker Platform Release.

| Platform Version | Label | Planning |
|-----------|------|-----|
| Docker 1.6 |  [Docker/1.6](https://github.com/docker/distribution/labels/docker%2F1.6) | [Project Page](https://github.com/docker/distribution/wiki/docker-1.6-Project-Page) |
| Docker 1.7|  [Docker/1.7](https://github.com/docker/distribution/labels/docker%2F1.7) | [Project Page](https://github.com/docker/distribution/wiki/docker-1.7-Project-Page) |
| Docker 1.8|  [Docker/1.8](https://github.com/docker/distribution/labels/docker%2F1.8) | [Project Page](https://github.com/docker/distribution/wiki/docker-1.8-Project-Page) |

