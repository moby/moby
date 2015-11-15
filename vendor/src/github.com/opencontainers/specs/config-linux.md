# Linux-specific Container Configuration

The Linux container specification uses various kernel features like namespaces, cgroups, capabilities, LSM, and file system jails to fulfill the spec.
Additional information is needed for Linux over the [default spec configuration](config.md) in order to configure these various kernel features.

## Capabilities

Capabilities is an array that specifies Linux capabilities that can be provided to the process inside the container.
Valid values are the strings for capabilities defined in [the man page](http://man7.org/linux/man-pages/man7/capabilities.7.html)

```json
   "capabilities": [
        "CAP_AUDIT_WRITE",
        "CAP_KILL",
        "CAP_NET_BIND_SERVICE"
    ]
```

## User namespace mappings

```json
    "uidMappings": [
        {
            "hostID": 1000,
            "containerID": 0,
            "size": 10
        }
    ],
    "gidMappings": [
        {
            "hostID": 1000,
            "containerID": 0,
            "size": 10
        }
    ]
```

uid/gid mappings describe the user namespace mappings from the host to the container.
The mappings represent how the bundle `rootfs` expects the user namespace to be setup and the runtime SHOULD NOT modify the permissions on the rootfs to realize the mapping.
*hostID* is the starting uid/gid on the host to be mapped to *containerID* which is the starting uid/gid in the container and *size* refers to the number of ids to be mapped.
There is a limit of 5 mappings which is the Linux kernel hard limit.

## Default Devices and File Systems

The Linux ABI includes both syscalls and several special file paths.
Applications expecting a Linux environment will very likely expect these files paths to be setup correctly.

The following devices and filesystems MUST be made available in each application's filesystem

|     Path     |  Type  |  Notes  |
| ------------ | ------ | ------- |
| /proc        | [procfs](https://www.kernel.org/doc/Documentation/filesystems/proc.txt)    | |
| /sys         | [sysfs](https://www.kernel.org/doc/Documentation/filesystems/sysfs.txt)    | |
| /dev/null    | [device](http://man7.org/linux/man-pages/man4/null.4.html)                 | |
| /dev/zero    | [device](http://man7.org/linux/man-pages/man4/zero.4.html)                 | |
| /dev/full    | [device](http://man7.org/linux/man-pages/man4/full.4.html)                 | |
| /dev/random  | [device](http://man7.org/linux/man-pages/man4/random.4.html)               | |
| /dev/urandom | [device](http://man7.org/linux/man-pages/man4/random.4.html)               | |
| /dev/tty     | [device](http://man7.org/linux/man-pages/man4/tty.4.html)                  | |
| /dev/console | [device](http://man7.org/linux/man-pages/man4/console.4.html)              | |
| /dev/pts     | [devpts](https://www.kernel.org/doc/Documentation/filesystems/devpts.txt)  | |
| /dev/ptmx    | [device](https://www.kernel.org/doc/Documentation/filesystems/devpts.txt)  | Bind-mount or symlink of /dev/pts/ptmx |
| /dev/shm     | [tmpfs](https://www.kernel.org/doc/Documentation/filesystems/tmpfs.txt)    | |
