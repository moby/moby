# Experimental: Docker Plugin API

Docker plugins are out-of-process extensions which add capabilities to the
Docker Engine.

This page is intended for people who want to develop their own Docker plugin.
If you just want to learn about or use Docker plugins, look
[here](/experimental/plugins.md).

This is an experimental feature. For information on installing and using experimental features, see [the experimental feature overview](README.md).

## What plugins are

A plugin is a process running on the same docker host as the docker daemon,
which registers itself by placing a file in `/usr/share/docker/plugins` (the
"plugin directory").

Plugins have human-readable names, which are short, lowercase strings. For
example, `flocker` or `weave`.

Plugins can run inside or outside containers. Currently running them outside
containers is recommended.

## Plugin discovery

Docker discovers plugins by looking for them in the plugin directory whenever a
user or container tries to use one by name.

There are two types of files which can be put in the plugin directory.

* `.sock` files are UNIX domain sockets.
* `.spec` files are text files containing a URL, such as `unix:///other.sock`.

The name of the file (excluding the extension) determines the plugin name.

For example, the `flocker` plugin might create a UNIX socket at
`/usr/share/docker/plugins/flocker.sock`.

Plugins must be run locally on the same machine as the Docker daemon.  UNIX
domain sockets are strongly encouraged for security reasons.

## Plugin lifecycle

Plugins should be started before Docker, and stopped after Docker.  For
example, when packaging a plugin for a platform which supports `systemd`, you
might use [`systemd` dependencies](
http://www.freedesktop.org/software/systemd/man/systemd.unit.html#Before=) to
manage startup and shutdown order.

When upgrading a plugin, you should first stop the Docker daemon, upgrade the
plugin, then start Docker again.

If a plugin is packaged as a container, this may cause issues. Plugins as
containers are currently considered experimental due to these shutdown/startup
ordering issues. These issues are mitigated by plugin retries (see below).

## Plugin activation

When a plugin is first referred to -- either by a user referring to it by name
(e.g.  `docker run --volume-driver=foo`) or a container already configured to
use a plugin being started -- Docker looks for the named plugin in the plugin
directory and activates it with a handshake. See Handshake API below.

Plugins are *not* activated automatically at Docker daemon startup. Rather,
they are activated only lazily, or on-demand, when they are needed.

## API design

The Plugin API is RPC-style JSON over HTTP, much like webhooks.

Requests flow *from* the Docker daemon *to* the plugin.  So the plugin needs to
implement an HTTP server and bind this to the UNIX socket mentioned in the
"plugin discovery" section.

All requests are HTTP `POST` requests.

The API is versioned via an Accept header, which currently is always set to
`application/vnd.docker.plugins.v1+json`.

## Handshake API

Plugins are activated via the following "handshake" API call.

### /Plugin.Activate

**Request:** empty body

**Response:**
```
{
    "Implements": ["VolumeDriver"]
}
```

Responds with a list of Docker subsystems which this plugin implements.
After activation, the plugin will then be sent events from this subsystem.

## Volume API

If a plugin registers itself as a `VolumeDriver` (see above) then it is
expected to provide writeable paths on the host filesystem for the Docker
daemon to provide to containers to consume.

The Docker daemon handles bind-mounting the provided paths into user
containers.

### /VolumeDriver.Create

**Request**:
```
{
    "Name": "volume_name"
}
```

Instruct the plugin that the user wants to create a volume, given a user
specified volume name.  The plugin does not need to actually manifest the
volume on the filesystem yet (until Mount is called).

**Response**:
```
{
    "Err": null
}
```

Respond with a string error if an error occurred.

### /VolumeDriver.Remove

**Request**:
```
{
    "Name": "volume_name"
}
```

Create a volume, given a user specified volume name.

**Response**:
```
{
    "Err": null
}
```

Respond with a string error if an error occurred.

### /VolumeDriver.Mount

**Request**:
```
{
    "Name": "volume_name"
}
```

Docker requires the plugin to provide a volume, given a user specified volume
name. This is called once per container start.

**Response**:
```
{
    "Mountpoint": "/path/to/directory/on/host",
    "Err": null
}
```

Respond with the path on the host filesystem where the volume has been made
available, and/or a string error if an error occurred.

### /VolumeDriver.Path

**Request**:
```
{
    "Name": "volume_name"
}
```

Docker needs reminding of the path to the volume on the host.

**Response**:
```
{
    "Mountpoint": "/path/to/directory/on/host",
    "Err": null
}
```

Respond with the path on the host filesystem where the volume has been made
available, and/or a string error if an error occurred.

### /VolumeDriver.Unmount

**Request**:
```
{
    "Name": "volume_name"
}
```

Indication that Docker no longer is using the named volume. This is called once
per container stop.  Plugin may deduce that it is safe to deprovision it at
this point.

**Response**:
```
{
    "Err": null
}
```

Respond with a string error if an error occurred.

## Plugin retries

Attempts to call a method on a plugin are retried with an exponential backoff
for up to 30 seconds. This may help when packaging plugins as containers, since
it gives plugin containers a chance to start up before failing any user
containers which depend on them.
