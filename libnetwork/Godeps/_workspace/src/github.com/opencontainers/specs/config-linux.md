# Linux-specific configuration

The Linux container specification uses various kernel features like namespaces,
cgroups, capabilities, LSM, and file system jails to fulfill the spec.
Additional information is needed for Linux over the [default spec configuration](config.md)
in order to configure these various kernel features.

## Linux namespaces

A namespace wraps a global system resource in an abstraction that makes it 
appear to the processes within the namespace that they have their own isolated 
instance of the global resource.  Changes to the global resource are visible to 
other processes that are members of the namespace, but are invisible to other 
processes. For more information, see [the man page](http://man7.org/linux/man-pages/man7/namespaces.7.html)

Namespaces are specified in the spec as an array of entries. Each entry has a 
type field with possible values described below and an optional path element. 
If a path is specified, that particular file is used to join that type of namespace.

```json
    "namespaces": [
        {
            "type": "pid",
            "path": "/proc/1234/ns/pid"
        },
        {
            "type": "net",
            "path": "/var/run/netns/neta"
        },
        {
            "type": "mnt",
        },
        {
            "type": "ipc",
        },
        {
            "type": "uts",
        },
        {
            "type": "user",
        },
    ]
```

#### Namespace types

* **pid** processes inside the container will only be able to see other processes inside the same container.
* **network** the container will have it's own network stack.
* **mnt** the container will have an isolated mount table.
* **ipc** processes inside the container will only be able to communicate to other processes inside the same
container via system level IPC.
* **uts** the container will be able to have it's own hostname and domain name.
* **user** the container will be able to remap user and group IDs from the host to local users and groups
within the container.

### Access to devices

Devices is an array specifying the list of devices from the host to make available in the container.
By providing a device name within the list the runtime should look up the same device on the host's `/dev`
and collect information about the device node so that it can be recreated for the container.  The runtime
should not only create the device inside the container but ensure that the root user inside 
the container has access rights for the device.

```json
   "devices": [
        "null",
        "random",
        "full",
        "tty",
        "zero",
        "urandom"
    ]
```

## Linux control groups

Also known as cgroups, they are used to restrict resource usage for a container and handle
device access.  cgroups provide controls to restrict cpu, memory, IO, and network for
the container. For more information, see the [kernel cgroups documentation](https://www.kernel.org/doc/Documentation/cgroups/cgroups.txt)

## Linux capabilities

Capabilities is an array that specifies Linux capabilities that can be provided to the process
inside the container. Valid values are the string after `CAP_` for capabilities defined 
in [the man page](http://man7.org/linux/man-pages/man7/capabilities.7.html)

```json
   "capabilities": [
        "AUDIT_WRITE",
        "KILL",
        "NET_BIND_SERVICE"
    ]
```

## Linux sysctl

sysctl allows kernel parameters to be modified at runtime for the container.
For more information, see [the man page](http://man7.org/linux/man-pages/man8/sysctl.8.html)

```json
   "sysctl": {
        "net.ipv4.ip_forward": "1",
        "net.core.somaxconn": "256"
   }
```

## Linux rlimits

```json
   "rlimits": [
        {
            "type": "RLIMIT_NPROC",
            "soft": 1024,
            "hard": 102400
        }
   ]
```

rlimits allow setting resource limits. The type is from the values defined in [the man page](http://man7.org/linux/man-pages/man2/setrlimit.2.html). The kernel enforces the soft limit for a resource while the hard limit acts as a ceiling for that value that could be set by an unprivileged process.

## Linux user namespace mappings

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

uid/gid mappings describe the user namespace mappings from the host to the container. *hostID* is the starting uid/gid on the host to be mapped to *containerID* which is the starting uid/gid in the container and *size* refers to the number of ids to be mapped. The Linux kernel has a limit of 5 such mappings that can be specified.

## Rootfs Mount Propagation
rootfsPropagation sets the rootfs's mount propagation. Its value is either slave, private, or shared. [The kernel doc](https://www.kernel.org/doc/Documentation/filesystems/sharedsubtree.txt) has more information about mount propagation.

```json
    "rootfsPropagation": "slave",
```

## Security

**TODO:** security profiles

