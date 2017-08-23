***Draft***

## Moby Core Component Architecture

Moby Core is a container platform built for modularity.
Moby Core uses a component architecture which allows moby components to interact
with each other with well defined API's in a way that enables components
to be swapped out for different implementations at packaging time.

Moby Core defines several of its own [components](#moby-core-services) along with
a default stack which implements those services. Packagers can take Moby Core as
defined, swap out components for different ones, and even supply completely new
components not specifically defined in the core architecture to extend
Moby Core's functionality.

### Status

This architecture is still in heavy development and this document should be considered fluid
for the time being.

While the v1 HTTP API is considered stable, the components and component APIs are not finalized
and should NOT be considered stable. This document may contain things that do not even exist yet.

***Use at your own risk. Expect frequent breakages.***

### Components

A component is an isolated set of functionality that is likely useful in its
own right.

A component:
- MUST provide a GRPC endpoint with the following services on a node-local socket (such as a unix socket or named pipe):
	- MUST provide [GRPC healthcheck service](https://github.com/grpc/grpc/blob/master/doc/health-checking.md)
	- MUST provide [Version service](services/component/version/v1)
	- SHOULD provide [Metrics service](services/component/metrics/v1)
- MAY expose a separate API for external consumption
- SHOULD use GRPC to publish component-level services
- SHOULD be easily deployed as a container (the primary packaging format for
Moby Core, see [Packaging](#Packaging)).
	- SHOULD log to stdout and/or stderr
	- SHOULD NOT depend on shared filesystem access with other components
		- If shared filesystem access is required, this SHOULD be well documented

### Moby Core Services

Moby Core defines a base set of services to be used with components along with
default implementations of those services.
A "service" in this case refers to a [GRPC service](https://grpc.io/docs/guides/concepts.html#service-definition).
A component may consist of one or more services.

All Moby Core services MUST use GRPC for cross-component communication.
Proto files MUST be separate from the implementation, however generated
code MUST stay with the proto.

While these services are split up here, multiple services MAY be part of
a single component.

#### Public API's

- moby-http-api-v1 - Provides the v1.* HTTP API
	- Dependencies:
		- moby-container-store
		- moby-container-rootfs
		- moby-plugin-store
		- moby-network-store
		- moby-volume-store
		- moby-image-store

#### Events

- moby-events - Pub/sub for component level events.

#### Container management

- moby-container-store - Provides basic CRUD operations for container
specs/metadata. May be unnecessary or at least reduced scope after
[containerd#1378](https://github.com/containerd/containerd/pull/1378)
- moby-container-runtime - start/stop/kill/etc operations on containers
	- Dependencies:
		- moby-network-runtime
		- moby-volume-runtime
- moby-container-rootfs - Provides access to container filesystems.
- moby-container-logs - Collects container stdio and forwards to a centralized logging service

#### Plugins

- moby-plugin-store - Provides basic CRUD operations for plugins

#### Networking

- moby-network-store - Provides basic CRUD operations for networks
	- Dependencies:
		- moby-plugin-store
- moby-network-runtime - Sets up networking for containers
	- Dependencies:
		- moby-network-store

#### Persistent storage

- moby-volume-store - Provides basic CRUD operations for volumes
	- Dependencies:
		- moby-plugin-store
- moby-volume-runtime - Sets up volumes for a container
	- Dependencies:
		- moby-volume-store

#### Image Management

- moby-layer-store - Storage for image layers
- moby-image-store - Provides basic CRUD operations for images
	- Dependencies:
		- moby-layer-store

#### Component Management

- moby-component-index - Provides registration and metadata storage for moby components
- moby-component-init - Initializes moby components, may not be used in some packaging formats.
	- Dependencies:
		- moby-component-registry
- moby-component-healthcheck - Runs healthchecks on moby components
	- Dependencies:
		- moby-component-registry

#### Cluster Management

TBD

#### Other

See the [service definitions](services) for these services.

TODO:
 - metrics
 - tracing

### Discovery

Discovery is handled through the moby-component-registry.
Components are registered via a static component manifest config at startup.
The registry service is a GRPC service which allows you to look up a component's address,
version, and health status.

### Packaging

One goal of the component architecture is to easily package Moby Core via the
[Moby Tool](github.com/moby/tool) as "services". This enables packaging Moby
Core in multiple formats, and easily adding/removing components to the system.