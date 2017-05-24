---
title: "Mount point interposition plugin"
description: "How to create mount point plugins to interpose on container mount point attachments and detachments."
keywords: "bind, mount, volume, bind mount, mount point, docker, documentation, plugin, extend"
---

<!-- This file is maintained within the docker/docker Github
     repository at https://github.com/docker/docker/. Make all
     pull requests against that repo. If you see this file in
     another repository, consider it read-only there, as it will
     periodically be overwritten by the definitive file. Pull
     requests which include edits to this file in other repositories
     will be rejected.
-->

# Create a mount point plugin

This document describes the Docker Engine plugins generally available in
Docker Engine. To view information on plugins managed by Docker Engine,
refer to [Docker Engine plugin system](index.md).

Docker's out-of-the-box bind mount model sources file system trees
directly from the host operating system. If you require greater control
of the mounting process, you can create mount point plugins and add them
to your Docker daemon configuration. Using a mount point plugin, you can
redirect mounts to alternative locations, block container start until
setup is complete, block or abort container shutdown until teardown is
complete, and receive mount attachment and detachment events including
mount options like consistency levels.

Anyone with the appropriate skills can develop a mount point
plugin. These skills, at their most basic, are knowledge of Docker,
understanding of REST, and sound programming knowledge. This document
describes the architecture, state, and methods available to a mount point
plugin developer.

## Basic principles

Docker's [plugin infrastructure](plugin_api.md) enables extending Docker
by loading, removing and communicating with third-party components using
a generic API. The mount point interposition subsystem was built using
this mechanism.

Using this subsystem, you don't need to rebuild the Docker daemon to add
a mount point plugin.  You can add a plugin to an installed Docker
daemon. You do not need to restart the Docker daemon to add a new plugin.

A mount point plugin intercepts container mount attachments and
container mount detachments to rewrite them, prepare for them, or tear
them down. The mount point context contains a container identifier and a
full description of the mounts being requested.

Mount point plugins must follow the rules described in [Docker Plugin
API](plugin_api.md).  Each plugin must reside within directories
described under the [Plugin discovery](plugin_api.md#plugin-discovery)
section.

## Default bind mount mechanism

The default bind mount behavior sources bind mounts from the mount
namespace of the Docker daemon and its containerd subprocess. This
namespace is typically the root namespace of the host operating system
but may differ if the Docker daemon itself is running inside of a
container.

## Basic architecture

You are responsible for registering your plugin as part of the Docker
daemon startup. You can install multiple plugins and chain them
together. This chain is ordered. Each mount *attachment* request passes
in order through the chain.  Only when all the plugins indicate mount
point setup is complete is a container started. Each mount point
*detachment* request passes in reverse order through the plugins used
during that mount point's attachment. Only when all the plugins indicate
mount point teardown is complete and successful is a container
successfully stopped.

When a container start request that includes a mount attachment is made
to the Docker daemon through the CLI or via the Engine API, the mount
point subsystem passes the request to the installed mount point
plugin(s). The request contains a container ID and a full description of
the mount being requested. The plugin may allow or deny the mount,
change the mount location, or perform arbitrary actions to setup the
mount.

<!-- TODO flow diagrams? -->

## Docker client flows

To enable and configure the mount point plugin, the plugin developer must
support the Docker client interactions detailed in this section.

### Setting up Docker daemon

Enable the mount point plugin with a dedicated command line flag in the
`--mount-point-plugin=PLUGIN_ID` format. The flag supplies a `PLUGIN_ID`
value. This value can be the plugin’s socket or a path to a specification file.
Mount point plugins can be loaded without restarting the daemon. Refer
to the [`dockerd` documentation](../reference/commandline/dockerd.md#configuration-reloading) for more information.

```bash
$ dockerd --mount-point-plugin=plugin1 --mount-point-plugin=plugin2,...
```

Docker's mount point subsystem supports multiple `--mount-point-plugin` parameters.

<!-- TODO request/response examples? -->

## API schema and implementation

In addition to Docker's standard plugin registration method, each plugin
should implement the following three methods:

* `/MountPointPlugin.MountPointProperties` The mount point plugin
  properties method is called when mount point plugins are loaded at
  daemon start- or reload-time. If containers are running which used a
  now unloaded plugin during attachment, this method may also be called
  before detachment if the plugin hasn't been initialized in the current
  daemon instance (e.g. when using `--live-restore` and removing mount
  point plugins while containers are running). The response to
  `/MountPointPlugin.MountPointProperties` contains mount point filter
  definitions which can be used to select which mount points (if any)
  should be passed to this mount point plugin. This functionality
  reduces container start-up latency even when many mount point plugins
  may be in use.

* `/MountPointPlugin.MountPointAttach` This mount point attachment method
  is called before the Docker daemon starts a container. All applicable
  mounts into the container are passed to the plugin simultaneously.

* `/MountPointPlugin.MountPointDetach` This mount point detachment method
  is called before the Docker daemon reports the shutdown status of a
  container. Only the ID of the container causing the detachment is provided.

#### /MountPointPlugin.MountPointProperties

**Request**:

```json
{
}
```

**Response**:

```json
{
    "Success": "Indicates whether the properties query was successful (bool)",
    "Types":   "Enumerates the types of mount points ('bind', 'volume') this mount point plugin interposes ({ string -> bool })",
    "VolumePatterns": [
        {
            "VolumePlugin":  "A volume plugin name to require (string)",
            "OptionPattern": "All of the keys of the OptionPattern must
            be present and each value must match a comma-separated
            segment of the corresponding key, keys and values may begin
            with a '!' to invert the match or '\!' to match a key or
            value beginning with '!'. ({ string -> string })"
        }
    ],
    "Err":     "If Success is false, contains a descriptive error message (string)"
}
```

#### /MountPointPlugin.MountPointAttach

**Request**:

```json
{
    "ID":     "The container ID that is attaching mounts (string)",
    "Mounts": "The applicable mount points being attached ([mountpoint.MountPoint])"
}
```

**Response**:

```json
{
   "Success":     "Indicates whether the attachment request was successful (bool)",
   "Attachments": [
       {
           "Attach": "Indicates whether the mount point plugin would
           like to be notified when the mount is later detached (bool)",
           "NewMountPoint": "If not omitted or the empty string,
           indicates a new path to use as the source of this mount point
           (string)"
       }
   ],
   "Err":         "If Success is false, contains a descriptive error message (string)"
}
```

#### /MountPointPlugin.MountPointDetach

**Request**:

```json
{
    "ID":     "The container ID that is detaching mounts (string)",
}
```

**Response**:

```json
{
   "Success":     "Indicates whether the detachment request was successful (bool)",
   "Recoverable": "If Success is false, indicates whether the failure is
   fatal to the container mount detachment process (false, default) or
   if the failure should cause the detaching container to return an
   error code but otherwise continue unwinding the container's mount
   point plugin stack (bool)",
   "Err":         "If Success is false, contains a descriptive error message (string)"
}
```
