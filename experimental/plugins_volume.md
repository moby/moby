# Experimental: Docker volume plugins

Docker volume plugins enable Docker deployments to be integrated with external
storage systems, such as Amazon EBS, and enable data volumes to persist beyond
the lifetime of a single Docker host. See the [plugin documentation](/experimental/plugins.md)
for more information.

This is an experimental feature. For information on installing and using experimental features, see [the experimental feature overview](README.md).

# Command-line changes

This experimental feature introduces two changes to the `docker run` command:

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

# Volume plugin protocol

If a plugin registers itself as a `VolumeDriver` when activated, then it is
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

# Related GitHub PRs and issues

- [#13161](https://github.com/docker/docker/pull/13161) Volume refactor and external volume plugins

Send us feedback and comments on [#13420](https://github.com/docker/docker/issues/13420),
or on the usual Google Groups (docker-user, docker-dev) and IRC channels.
