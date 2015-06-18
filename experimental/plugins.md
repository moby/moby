# Experimental: Extend Docker with a plugin 

You can extend the capabilities of the Docker Engine by loading third-party
plugins. 

This is an experimental feature. For information on installing and using experimental features, see [the experimental feature overview](README.md).

## Types of plugins

Plugins extend Docker's functionality.  They come in specific types.  For
example, a [volume plugin](/experimental/plugins_volume.md) might enable Docker
volumes to persist across multiple Docker hosts.

Currently Docker supports volume plugins. In the future it will support
additional plugin types.

## Installing a plugin

Follow the instructions in the plugin's documentation.

## Finding a plugin

The following plugins exist:

* The [Flocker plugin](https://clusterhq.com/docker-plugin/) is a volume plugin
which provides multi-host portable volumes for Docker, enabling you to run
  databases and other stateful containers and move them around across a cluster
  of machines.

## Troubleshooting a plugin

If you are having problems with Docker after loading a plugin, ask the authors
of the plugin for help. The Docker team may not be able to assist you.

## Writing a plugin

If you are interested in writing a plugin for Docker, or seeing how they work
under the hood, see the [docker plugins reference](/experimental/plugin_api.md).

# Related GitHub PRs and issues

- [#13222](https://github.com/docker/docker/pull/13222) Plugins plumbing

Send us feedback and comments on [#13419](https://github.com/docker/docker/issues/13419),
or on the usual Google Groups (docker-user, docker-dev) and IRC channels.
