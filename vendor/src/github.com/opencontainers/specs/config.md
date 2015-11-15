# Container Configuration file

The container's top-level directory MUST contain a configuration file called `config.json`.
For now the canonical schema is defined in [config.go](config.go) and [config_linux.go](config_linux.go), but this will be moved to a formal JSON schema over time.

The configuration file contains metadata necessary to implement standard operations against the container.
This includes the process to run, environment variables to inject, sandboxing features to use, etc.

Below is a detailed description of each field defined in the configuration format.

## Manifest version

* **`version`** (string, required) must be in [SemVer v2.0.0](http://semver.org/spec/v2.0.0.html) format and specifies the version of the OCF specification with which the container bundle complies. The Open Container spec follows semantic versioning and retains forward and backward compatibility within major versions. For example, if an implementation is compliant with version 1.0.1 of the spec, it is compatible with the complete 1.x series.

*Example*

```json
    "version": "0.1.0"
```

## Root Configuration

Each container has exactly one *root filesystem*, specified in the *root* object:

* **`path`** (string, required) Specifies the path to the root filesystem for the container, relative to the path where the manifest is. A directory MUST exist at the relative path declared by the field.
* **`readonly`** (bool, optional) If true then the root filesystem MUST be read-only inside the container. Defaults to false.

*Example*

```json
"root": {
    "path": "rootfs",
    "readonly": true
}
```

## Mount Points

You can add array of mount points inside container as `mounts`.
Each record in this array must have configuration in [runtime config](runtime-config.md#mount-configuration).
The runtime MUST mount entries in the listed order.

* **`name`** (string, required) Name of mount point. Used for config lookup.
* **`path`** (string, required) Destination of mount point: path inside container.

*Example*

```json
"mounts": [
    {
        "name": "proc",
        "path": "/proc"
    },
    {
        "name": "dev",
        "path": "/dev"
    },
    {
        "name": "devpts",
        "path": "/dev/pts"
    },
    {
        "name": "data",
        "path": "/data"
    }
]
```

## Process configuration

* **`terminal`** (bool, optional) specifies whether you want a terminal attached to that process. Defaults to false.
* **`cwd`** (string, optional) is the working directory that will be set for the executable.
* **`env`** (array of strings, optional) contains a list of variables that will be set in the process's environment prior to execution. Elements in the array are specified as Strings in the form "KEY=value". The left hand side must consist solely of letters, digits, and underscores `_` as outlined in [IEEE Std 1003.1-2001](http://pubs.opengroup.org/onlinepubs/009695399/basedefs/xbd_chap08.html).
* **`args`** (string, required) executable to launch and any flags as an array. The executable is the first element and must be available at the given path inside of the rootfs. If the executable path is not an absolute path then the search $PATH is interpreted to find the executable.

The user for the process is a platform-specific structure that allows specific control over which user the process runs as.
For Linux-based systems the user structure has the following fields:

* **`uid`** (int, required) specifies the user id.
* **`gid`** (int, required) specifies the group id.
* **`additionalGids`** (array of ints, optional) specifies additional group ids to be added to the process.

*Example (Linux)*

```json
"process": {
    "terminal": true,
    "user": {
        "uid": 1,
        "gid": 1,
        "additionalGids": [5, 6]
    },
    "env": [
        "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
        "TERM=xterm"
    ],
    "cwd": "/root",
    "args": [
        "sh"
    ]
}
```


## Hostname

* **`hostname`** (string, optional) as it is accessible to processes running inside.

*Example*

```json
"hostname": "mrsdalloway"
```

## Platform-specific configuration

* **`os`** (string, required) specifies the operating system family this image must run on. Values for os must be in the list specified by the Go Language document for [`$GOOS`](https://golang.org/doc/install/source#environment).
* **`arch`** (string, required) specifies the instruction set for which the binaries in the image have been compiled. Values for arch must be in the list specified by the Go Language document for [`$GOARCH`](https://golang.org/doc/install/source#environment).

```json
"platform": {
    "os": "linux",
    "arch": "amd64"
}
```

Interpretation of the platform section of the JSON file is used to find which platform-specific sections may be available in the document.
For example, if `os` is set to `linux`, then a JSON object conforming to the [Linux-specific schema](config-linux.md) SHOULD be found at the key `linux` in the `config.json`.
