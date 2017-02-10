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
For enabling experimental features, you need to start the Docker daemon with
`--experimental` flag.
You can also enable the daemon flag via `/etc/docker/daemon.json`. e.g.

```json
{
    "experimental": true
}
```

Then make sure the experimental flag is enabled:

```bash
$ docker version -f '{{.Server.Experimental}}'
true
```

## Current experimental features

 * [External graphdriver plugins](../docs/extend/plugins_graphdriver.md)
 * [Ipvlan Network Drivers](vlan-networks.md)
 * [Docker Stacks and Distributed Application Bundles](docker-stacks-and-bundles.md)
 * [Checkpoint & Restore](checkpoint-restore.md)

## How to comment on an experimental feature

Each feature's documentation includes a list of proposal pull requests or PRs associated with the feature. If you want to comment on or suggest a change to a feature, please add it to the existing feature PR.

Issues or problems with a feature? Inquire for help on the `#docker` IRC channel or on the [Docker Google group](https://groups.google.com/forum/#!forum/docker-user).
