# Runtime Configuration

## Hooks

Lifecycle hooks allow custom events for different points in a container's runtime.
Presently there are `Prestart`, `Poststart` and `Poststop`.

* [`Prestart`](#prestart) is a list of hooks to be run before the container process is executed
* [`Poststart`](#poststart) is a list of hooks to be run immediately after the container process is started
* [`Poststop`](#poststop) is a list of hooks to be run after the container process exits

Hooks allow one to run code before/after various lifecycle events of the container.
Hooks MUST be called in the listed order.
The state of the container is passed to the hooks over stdin, so the hooks could get the information they need to do their work.

Hook paths are absolute and are executed from the host's filesystem.

### Prestart

The pre-start hooks are called after the container process is spawned, but before the user supplied command is executed.
They are called after the container namespaces are created on Linux, so they provide an opportunity to customize the container.
In Linux, for e.g., the network namespace could be configured in this hook.

If a hook returns a non-zero exit code, then an error including the exit code and the stderr is returned to the caller and the container is torn down.

### Poststart

The post-start hooks are called after the user process is started.
For example this hook can notify user that real process is spawned.

If a hook returns a non-zero exit code, then an error is logged and the remaining hooks are executed.

### Poststop

The post-stop hooks are called after the container process is stopped.
Cleanup or debugging could be performed in such a hook.
If a hook returns a non-zero exit code, then an error is logged and the remaining hooks are executed.

*Example*

```json
    "hooks" : {
        "prestart": [
            {
                "path": "/usr/bin/fix-mounts",
                "args": ["arg1", "arg2"],
                "env":  [ "key1=value1"]
            },
            {
                "path": "/usr/bin/setup-network"
            }
        ],
        "poststart": [
            {
                "path": "/usr/bin/notify-start"
            }
        ],
        "poststop": [
            {
                "path": "/usr/sbin/cleanup.sh",
                "args": ["-f"]
            }
        ]
    }
```

`path` is required for a hook.
`args` and `env` are optional.

## Mount Configuration

Additional filesystems can be declared as "mounts", specified in the *mounts* object.
Keys in this object are names of mount points from portable config.
Values are objects with configuration of mount points.
The parameters are similar to the ones in [the Linux mount system call](http://man7.org/linux/man-pages/man2/mount.2.html).
Only [mounts from the portable config](config.md#mount-points) will be mounted.

* **`type`** (string, required) Linux, *filesystemtype* argument supported by the kernel are listed in */proc/filesystems* (e.g., "minix", "ext2", "ext3", "jfs", "xfs", "reiserfs", "msdos", "proc", "nfs", "iso9660"). Windows: ntfs
* **`source`** (string, required) a device name, but can also be a directory name or a dummy. Windows, the volume name that is the target of the mount point. \\?\Volume\{GUID}\ (on Windows source is called target)
* **`options`** (list of strings, optional) in the fstab format [https://wiki.archlinux.org/index.php/Fstab](https://wiki.archlinux.org/index.php/Fstab).

*Example (Linux)*

```json
"mounts": {
    "proc": {
        "type": "proc",
        "source": "proc",
        "options": []
    },
    "dev": {
        "type": "tmpfs",
        "source": "tmpfs",
        "options": ["nosuid","strictatime","mode=755","size=65536k"]
    },
    "devpts": {
        "type": "devpts",
        "source": "devpts",
        "options": ["nosuid","noexec","newinstance","ptmxmode=0666","mode=0620","gid=5"]
    },
    "data": {
        "type": "bind",
        "source": "/volumes/testing",
        "options": ["rbind","rw"]
    }
}
```

*Example (Windows)*

```json
"mounts": {
    "myfancymountpoint": {
        "type": "ntfs",
        "source": "\\\\?\\Volume\\{2eca078d-5cbc-43d3-aff8-7e8511f60d0e}\\",
        "options": []
    }
}
```

See links for details about [mountvol](http://ss64.com/nt/mountvol.html) and [SetVolumeMountPoint](https://msdn.microsoft.com/en-us/library/windows/desktop/aa365561(v=vs.85).aspx) in Windows.
