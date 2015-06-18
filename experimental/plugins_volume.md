# Experimental: Docker volume plugins

Docker volume plugins enable Docker deployments to be integrated with external
storage systems, such as Amazon EBS, and enable data volumes to persist beyond
the lifetime of a single Docker host. See the [plugin documentation](/experimental/plugins.md)
for more information.

This is an experimental feature. For information on installing and using experimental features, see [the experimental feature overview](README.md).

# Command-line changes

This experimental features introduces two changes to the `docker run` command:

- The `--volume-driver` flag is introduced.
- The `-v` syntax is changed to accept a volume name a first component.

Example:

    $ docker run -ti -v volumename:/data --volume-driver=flocker busybox sh

By specifying a volume name in conjunction with a volume driver, volume plugins
such as [Flocker](https://clusterhq.com/docker-plugin/), once installed, can be
used to manage volumes external to a single host, such as those on EBS. In this
example, "volumename" is passed through to the volume plugin as a user-given
name for the volume which allows the plugin to associate it with an external
volume beyond the lifetime of a single container or container host. This can be
used, for example, to move a stateful container from one server to another.

The `volumename` must not begin with a `/`.

# API changes

The container creation endpoint (`/containers/create`) accepts a `VolumeDriver`
field of type `string` allowing to specify the name of the driver. It's default
value of `"local"` (the default driver for local volumes).

# Related GitHub PRs and issues

- [#13161](https://github.com/docker/docker/pull/13161) Volume refactor and external volume plugins

Send us feedback and comments on [#13420](https://github.com/docker/docker/issues/13420),
or on the usual Google Groups (docker-user, docker-dev) and IRC channels.
