# Docker Experimental Features

This page contains a list of features in the Docker engine which are
experimental. Experimental features are **not** ready for production. They are
provided for test and evaluation in your sandbox environments.

The information below describes each feature and the GitHub pull requests and
issues associated with it. If necessary, links are provided to additional
documentation on an issue.  As an active Docker user and community member,
please feel free to provide any feedback on these features you wish.

## Use Docker experimental

Experimental features are now included in the standard Docker binaries as of
version 1.13.0.
To enable experimental features, start the Docker daemon with the
`--experimental` flag or enable the daemon flag in the
`/etc/docker/daemon.json` configuration file:

```json
{
    "experimental": true
}
```

You can check to see if experimental features are enabled on a running daemon
using the following command:

```bash
$ docker version -f '{{.Server.Experimental}}'
true
```

## Current experimental features

Docker service logs command to view logs for a Docker service. This is needed in Swarm mode.
Option to squash image layers to the base image after successful builds.
Checkpoint and restore support for Containers.
Metrics (Prometheus) output for basic container, image, and daemon operations.

 * The top-level [docker deploy](../docs/reference/commandline/deploy.md) command. The
   `docker stack deploy` command is **not** experimental.
 * [`--squash` option to `docker build` command](../docs/reference/commandline/build.md##squash-an-images-layers---squash-experimental-only)
 * [External graphdriver plugins](../docs/extend/plugins_graphdriver.md)
 * [Ipvlan Network Drivers](vlan-networks.md)
 * [Distributed Application Bundles](docker-stacks-and-bundles.md)
 * [Checkpoint & Restore](checkpoint-restore.md)
 * [Docker build with --squash argument](../docs/reference/commandline/build.md##squash-an-images-layers---squash-experimental-only)

## How to comment on an experimental feature

Each feature's documentation includes a list of proposal pull requests or PRs associated with the feature. If you want to comment on or suggest a change to a feature, please add it to the existing feature PR.

Issues or problems with a feature? Inquire for help on the `#docker` IRC channel or on the [Docker Google group](https://groups.google.com/forum/#!forum/docker-user).
