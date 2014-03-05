title
:   Registry API

description
:   API Documentation for Docker Registry

keywords
:   API, Docker, index, registry, REST, documentation

Docker Registry API
===================

1. Brief introduction
---------------------

-   This is the REST API for the Docker Registry
-   It stores the images and the graph for a set of repositories
-   It does not have user accounts data
-   It has no notion of user accounts or authorization
-   It delegates authentication and authorization to the Index Auth
    service using tokens
-   It supports different storage backends (S3, cloud files, local FS)
-   It doesn’t have a local database
-   It will be open-sourced at some point

We expect that there will be multiple registries out there. To help to
grasp the context, here are some examples of registries:

-   **sponsor registry**: such a registry is provided by a third-party
    hosting infrastructure as a convenience for their customers and the
    docker community as a whole. Its costs are supported by the third
    party, but the management and operation of the registry are
    supported by dotCloud. It features read/write access, and delegates
    authentication and authorization to the Index.
-   **mirror registry**: such a registry is provided by a third-party
    hosting infrastructure but is targeted at their customers only. Some
    mechanism (unspecified to date) ensures that public images are
    pulled from a sponsor registry to the mirror registry, to make sure
    that the customers of the third-party provider can “docker pull”
    those images locally.
-   **vendor registry**: such a registry is provided by a software
    vendor, who wants to distribute docker images. It would be operated
    and managed by the vendor. Only users authorized by the vendor would
    be able to get write access. Some images would be public (accessible
    for anyone), others private (accessible only for authorized users).
    Authentication and authorization would be delegated to the Index.
    The goal of vendor registries is to let someone do “docker pull
    basho/riak1.3” and automatically push from the vendor registry
    (instead of a sponsor registry); i.e. get all the convenience of a
    sponsor registry, while retaining control on the asset distribution.
-   **private registry**: such a registry is located behind a firewall,
    or protected by an additional security layer (HTTP authorization,
    SSL client-side certificates, IP address authorization...). The
    registry is operated by a private entity, outside of dotCloud’s
    control. It can optionally delegate additional authorization to the
    Index, but it is not mandatory.

> **note**
>
> Mirror registries and private registries which do not use the Index
> don’t even need to run the registry code. They can be implemented by
> any kind of transport implementing HTTP GET and PUT. Read-only
> registries can be powered by a simple static HTTP server.

> **note**
>
> The latter implies that while HTTP is the protocol of choice for a registry, multiple schemes are possible (and in some cases, trivial):
> :   -   HTTP with GET (and PUT for read-write registries);
>     -   local mount point;
>     -   remote docker addressed through SSH.
>
The latter would only require two new commands in docker, e.g.
`registryget` and `registryput`, wrapping access to the local filesystem
(and optionally doing consistency checks). Authentication and
authorization are then delegated to SSH (e.g. with public keys).

2. Endpoints
------------

### 2.1 Images

#### Layer

#### Image

#### Ancestry

### 2.2 Tags

### 2.3 Repositories

### 2.4 Status

3 Authorization
---------------

This is where we describe the authorization process, including the
tokens and cookies.

TODO: add more info.
