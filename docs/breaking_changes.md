<!--[metadata]>
+++
aliases = ["/engine/misc/breaking/"]
title = "Breaking changes"
description = "Breaking changes"
keywords = ["docker, documentation, about, technology, breaking",
"incompatibilities"]
[menu.main]
parent = "engine_use"
weight=80
+++
<![end-metadata]-->

# Breaking changes and incompatibilities

Every Engine release strives to be backward compatible with its predecessors.
In all cases, the policy is that feature removal is communicated two releases
in advance and documented as part of the [deprecated features](deprecated.md)
page.

Unfortunately, Docker is a fast moving project, and newly introduced features
may sometime introduce breaking changes and/or incompatibilities. This page
documents these by Engine version.

# Engine 1.10

There were two breaking changes in the 1.10 release.

## Registry

Registry 2.3 includes improvements to the image manifest that have caused a
breaking change. Images pushed by Engine 1.10 to a Registry 2.3 cannot be
pulled by digest by older Engine versions. A `docker pull` that encounters this
situation returns the following error:

``` Error response from daemon: unsupported schema version 2 for tag TAGNAME
```

Docker Content Trust heavily relies on pull by digest. As a result, images
pushed from the Engine 1.10 CLI to a 2.3 Registry cannot be pulled by older
Engine CLIs (< 1.10) with Docker Content Trust enabled.

If you are using an older Registry version (< 2.3), this problem does not occur
with any version of the Engine CLI; push, pull, with and without content trust
work as you would expect.

## Docker Content Trust

Engine older than the current 1.10 cannot pull images from repositories that
have enabled key delegation. Key delegation is an experimental feature which
requires a manual action to enable.
