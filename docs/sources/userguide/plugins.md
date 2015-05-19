page_title: Docker Plugins
page_description: Learn what Docker Plugins are and how to use them.
page_keywords: plugins, extensions, extensibility

# Understanding Docker Plugins

You can extend the capabilities of the Docker Engine by loading third-party
plugins.

## Types of plugins

Plugins extend Docker's functionality.  They come in specific types.  For
example, a **volume plugin** might enable Docker volumes to persist across
multiple Docker hosts.

Currently Docker supports **volume plugins**. In the future it will support
additional plugin types.

## Installing a plugin

Follow the instructions in the plugin's documentation.

## Finding a plugin

The following plugins exist:

* The [Flocker plugin](https://clusterhq.com/docker-plugin/) is a volume plugin
  which provides multi-host portable volumes for Docker, enabling you to run
  databases and other stateful containers and move them around across a cluster
  of machines.

## Using a plugin

Depending on the plugin type, there are additional arguments to `docker` CLI
commands.

* For example `docker run` has a [`--volume-driver` argument](
  /reference/commandline/cli/#run).

You can also use plugins via the [Docker Remote API](
/reference/api/docker_remote_api/).

## Troubleshooting a plugin

If you are having problems with Docker after loading a plugin, ask the authors
of the plugin for help. The Docker team may not be able to assist you.

## Writing a plugin

If you are interested in writing a plugin for Docker, or seeing how they work
under the hood, see the [docker plugins reference](/reference/api/plugin_api).
