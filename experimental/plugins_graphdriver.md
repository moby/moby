# Experimental: Docker graph driver plugins

Docker graph driver plugins enable admins to use an external/out-of-process
graph driver for use with Docker engine. This is an alternative to using the
built-in storage drivers, such as aufs/overlay/devicemapper/btrfs.

A graph driver plugin is used for image and container fs storage, as such
the plugin must be started and available for connections prior to Docker Engine
being started.

# Write a graph driver plugin

See the [plugin documentation](/docs/extend/plugins.md) for detailed information
on the underlying plugin protocol.


## Graph Driver plugin protocol

If a plugin registers itself as a `GraphDriver` when activated, then it is
expected to provide the rootfs for containers as well as image layer storage.

### /GraphDriver.Init

**Request**:
```
{
  "Home": "/graph/home/path",
  "Opts": []
}
```

Initialize the graph driver plugin with a home directory and array of options.
Plugins are not required to accept these options as the Docker Engine does not
require that the plugin use this path or options, they are only being passed
through from the user.

**Response**:
```
{
  "Err": null
}
```

Respond with a string error if an error occurred.


### /GraphDriver.Create

**Request**:
```
{
  "ID": "46fe8644f2572fd1e505364f7581e0c9dbc7f14640bd1fb6ce97714fb6fc5187",
  "Parent": "2cd9c322cb78a55e8212aa3ea8425a4180236d7106938ec921d0935a4b8ca142"
}
```

Create a new, empty, filesystem layer with the specified `ID` and `Parent`.
`Parent` may be an empty string, which would indicate that there is no parent
layer.

**Response**:
```
{
  "Err: null
}
```

Respond with a string error if an error occurred.


### /GraphDriver.Remove

**Request**:
```
{
  "ID": "46fe8644f2572fd1e505364f7581e0c9dbc7f14640bd1fb6ce97714fb6fc5187"
}
```

Remove the filesystem layer with this given `ID`.

**Response**:
```
{
  "Err: null
}
```

Respond with a string error if an error occurred.

### /GraphDriver.Get

**Request**:
```
{
  "ID": "46fe8644f2572fd1e505364f7581e0c9dbc7f14640bd1fb6ce97714fb6fc5187"
  "MountLabel": ""
}
```

Get the mountpoint for the layered filesystem referred to by the given `ID`.

**Response**:
```
{
  "Dir": "/var/mygraph/46fe8644f2572fd1e505364f7581e0c9dbc7f14640bd1fb6ce97714fb6fc5187",
  "Err": ""
}
```

Respond with the absolute path to the mounted layered filesystem.
Respond with a string error if an error occurred.

### /GraphDriver.Put

**Request**:
```
{
  "ID": "46fe8644f2572fd1e505364f7581e0c9dbc7f14640bd1fb6ce97714fb6fc5187"
}
```

Release the system resources for the specified `ID`, such as unmounting the
filesystem layer.

**Response**:
```
{
  "Err: null
}
```

Respond with a string error if an error occurred.

### /GraphDriver.Exists

**Request**:
```
{
  "ID": "46fe8644f2572fd1e505364f7581e0c9dbc7f14640bd1fb6ce97714fb6fc5187"
}
```

Determine if a filesystem layer with the specified `ID` exists.

**Response**:
```
{
  "Exists": true
}
```

Respond with a boolean for whether or not the filesystem layer with the specified
`ID` exists.

### /GraphDriver.Status

**Request**:
```
{}
```

Get low-level diagnostic information about the graph driver.

**Response**:
```
{
  "Status": [[]]
}
```

Respond with a 2-D array with key/value pairs for the underlying status
information.


### /GraphDriver.GetMetadata

**Request**:
```
{
  "ID": "46fe8644f2572fd1e505364f7581e0c9dbc7f14640bd1fb6ce97714fb6fc5187"
}
```

Get low-level diagnostic information about the layered filesystem with the
with the specified `ID`

**Response**:
```
{
  "Metadata": {},
  "Err": null
}
```

Respond with a set of key/value pairs containing the low-level diagnostic
information about the layered filesystem.
Respond with a string error if an error occurred.

### /GraphDriver.Cleanup

**Request**:
```
{}
```

Perform neccessary tasks to release resources help by the plugin, for example
unmounting all the layered file systems.

**Response**:
```
{
  "Err: null
}
```

Respond with a string error if an error occurred.


### /GraphDriver.Diff

**Request**:
```
{
  "ID": "46fe8644f2572fd1e505364f7581e0c9dbc7f14640bd1fb6ce97714fb6fc5187",
  "Parent": "2cd9c322cb78a55e8212aa3ea8425a4180236d7106938ec921d0935a4b8ca142"
}
```

Get an archive of the changes between the filesystem layers specified by the `ID`
and `Parent`. `Parent` may be an empty string, in which case there is no parent.

**Response**:
```
{{ TAR STREAM }}
```

### /GraphDriver.Changes

**Request**:
```
{
  "ID": "46fe8644f2572fd1e505364f7581e0c9dbc7f14640bd1fb6ce97714fb6fc5187",
  "Parent": "2cd9c322cb78a55e8212aa3ea8425a4180236d7106938ec921d0935a4b8ca142"
}
```

Get a list of changes between the filesystem layers specified by the `ID` and
`Parent`. `Parent` may be an empty string, in which case there is no parent.

**Response**:
```
{
  "Changes": [{}],
  "Err": null
}
```

Responds with a list of changes. The structure of a change is:
```
  "Path": "/some/path",
  "Kind": 0,
```

Where teh `Path` is the filesystem path within the layered filesystem that is
changed and `Kind` is an integer specifying the type of change that occurred:

- 0 - Modified
- 1 - Added
- 2 - Deleted

Respond with a string error if an error occurred.

### /GraphDriver.ApplyDiff

**Request**:
```
{{ TAR STREAM }}
```

Extract the changeset from the given diff into the layer with the specified `ID`
and `Parent`

**Query Parameters**:

- id (required)- the `ID` of the new filesystem layer to extract the diff to
- parent (required)- the `Parent` of the given `ID`

**Response**:
```
{
  "Size": 512366,
  "Err": null
}
```

Respond with the size of the new layer in bytes.
Respond with a string error if an error occurred.

### /GraphDriver.DiffSize

**Request**:
```
{
  "ID": "46fe8644f2572fd1e505364f7581e0c9dbc7f14640bd1fb6ce97714fb6fc5187",
  "Parent": "2cd9c322cb78a55e8212aa3ea8425a4180236d7106938ec921d0935a4b8ca142"
}
```

Calculate the changes between the specified `ID`

**Response**:
```
{
  "Size": 512366,
  "Err": null
}
```

Respond with the size changes between the specified `ID` and `Parent`
Respond with a string error if an error occurred.
