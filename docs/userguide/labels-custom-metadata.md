<!--[metadata]>
+++
title = "Managing Docker object labels"
description = "Description of labels, which are used to manage metadata on Docker objects."
keywords = ["Usage, user guide, labels, metadata, docker, documentation, examples, annotating"]
[menu.main]
parent = "engine_guide"
weight=100
+++
<![end-metadata]-->

# About labels

Labels are a mechanism for applying metadata to Docker objects, including:

- Images
- Containers
- Local daemons
- Volumes
- Networks
- Swarm nodes
- Swarm services

You can use labels to organize your images, record licensing information, annotate
relationships between containers, volumes, and networks, or in any way that makes
sense for your business or application.

# Label keys and values

A label is a key-value pair, stored as a string. You can specify multiple labels
for an object, but each key-value pair must be unique within an object. If the
same key is given multiple values, the most-recently-written value overwrites
all previous values.

## Key format recommendations

A label _key_ is the left-hand side of the key-value pair. Keys are alphanumeric
strings which may contain periods (`.`) and hyphens (`-`). Most Docker users use
images created by other organizations, and the following guidelines help to
prevent inadvertent duplication of labels across objects, especially if you plan
to use labels as a mechanism for automation.

- Authors of third-party tools should prefix each label key with the
  reverse DNS notation of a domain they own, such as `com.example.some-label`.

- Do not use a domain in your label key without the domain owner's permission.

- The `com.docker.*`, `io.docker.*` and `org.dockerproject.*` namespaces are
  reserved by Docker for internal use.

- Label keys should begin and end with a lower-case letter and should only
  contain lower-case alphanumeric characters, the period character (`.`), and
  the hyphen character (`-`). Consecutive periods or hyphens are not allowed.

- The period character (`.`) separates namespace "fields". Label keys without
  namespaces are reserved for CLI use, allowing users of the CLI to interactively
  label Docker objects using shorter typing-friendly strings.

These guidelines are not currently enforced and additional guidelines may apply
to specific use cases.

## Value guidelines

Label values can contain any data type that can be represented as a string,
including (but not limited to) JSON, XML, CSV, or YAML. The only requirement is
that the value be serialized to a string first, using a mechanism specific to
the type of structure. For instance, to serialize JSON into a string, you might
use the `JSON.stringify()` JavaScript method.

Since Docker does not deserialize the value, you cannot treat a JSON or XML
document as a nested structure when querying or filtering by label value unless
you build this functionality into third-party tooling.

# Managing labels on objects

Each type of object with support for labels has mechanisms for adding and
managing them and using them as they relate to that type of object. These links
provide a good place to start learning about how you can use labels in your
Docker deployments.

Labels on images, containers, local daemons, volumes, and networks are static for
the lifetime of the object. To change these labels you must recreate the object.
Labels on swarm nodes and services can be updated dynamically.


- Images and containers
  - [Adding labels to images](../reference/builder.md#label)
  - [Overriding a container's labels at runtime](../reference/commandline/run.md#set-metadata-on-container-l-label-label-file)
  - [Inspecting labels on images or containers](../reference/commandline/inspect.md)
  - [Filtering images by label](../reference/commandline/inspect.md#filtering)
  - [Filtering containers by label](../reference/commandline/ps.md#filtering)

- Local Docker daemons
  - [Adding labels to a Docker daemon at runtime](../reference/commandline/dockerd.md)
  - [Inspecting a Docker daemon's labels](../reference/commandline/info.md)

- Volumes
  - [Adding labels to volumes](../reference/commandline/volume_create.md)
  - [Inspecting a volume's labels](../reference/commandline/volume_inspect.md)
  - [Filtering volumes by label](../reference/commandline/volume_ls.md#filtering)

- Networks
  - [Adding labels to a network](../reference/commandline/network_create.md)
  - [Inspecting a network's labels](../reference/commandline/network_inspect.md)
  - [Filtering networks by label](../reference/commandline/network_ls.md#filtering)

- Swarm nodes
  - [Adding or updating a swarm node's labels](../reference/commandline/node_update.md#add-label-metadata-to-a-node)
  - [Inspecting a swarm node's labels](../reference/commandline/node_inspect.md)
  - [Filtering swarm nodes by label](../reference/commandline/node_ls.md#filtering)

- Swarm services
  - [Adding labels when creating a swarm service](../reference/commandline/service_create.md#set-metadata-on-a-service-l-label)
  - [Updating a swarm service's labels](../reference/commandline/service_update.md)
  - [Inspecting a swarm service's labels](../reference/commandline/service_inspect.md)
  - [Filtering swarm services by label](../reference/commandline/service_ls.md#filtering)
