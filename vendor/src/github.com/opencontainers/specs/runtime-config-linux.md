# Linux-specific Runtime Configuration

## Namespaces

A namespace wraps a global system resource in an abstraction that makes it appear to the processes within the namespace that they have their own isolated instance of the global resource.
Changes to the global resource are visible to other processes that are members of the namespace, but are invisible to other processes.
For more information, see [the man page](http://man7.org/linux/man-pages/man7/namespaces.7.html).

Namespaces are specified as an array of entries inside the `namespaces` root field.
The following parameters can be specified to setup namespaces:

* **`type`** *(string, required)* - namespace type. The following namespaces types are supported:
    * **`pid`** processes inside the container will only be able to see other processes inside the same container
    * **`network`** the container will have its own network stack
    * **`mount`** the container will have an isolated mount table
    * **`ipc`** processes inside the container will only be able to communicate to other processes inside the same container via system level IPC
    * **`uts`** the container will be able to have its own hostname and domain name
    * **`user`** the container will be able to remap user and group IDs from the host to local users and groups within the container

* **`path`** *(string, optional)* - path to namespace file

If a path is specified, that particular file is used to join that type of namespace.
Also, when a path is specified, a runtime MUST assume that the setup for that particular namespace has already been done and error out if the config specifies anything else related to that namespace.

###### Example

```json
    "namespaces": [
        {
            "type": "pid",
            "path": "/proc/1234/ns/pid"
        },
        {
            "type": "network",
            "path": "/var/run/netns/neta"
        },
        {
            "type": "mount"
        },
        {
            "type": "ipc"
        },
        {
            "type": "uts"
        },
        {
            "type": "user"
        }
    ]
```

## Devices

`devices` is an array specifying the list of devices to be created in the container.

The following parameters can be specified:

* **`type`** *(char, required)* - type of device: `c`, `b`, `u` or `p`. More info in `man mknod`.

* **`path`** *(string, optional)* - full path to device inside container

* **`major, minor`** *(int64, required)* - major, minor numbers for device. More info in `man mknod`. There is a special value: `-1`, which means `*` for `device` cgroup setup.

* **`permissions`** *(string, optional)* - cgroup permissions for device. A composition of `r` (*read*), `w` (*write*), and `m` (*mknod*).

* **`fileMode`** *(uint32, optional)* - file mode for device file

* **`uid`** *(uint32, optional)* - uid of device owner

* **`gid`** *(uint32, optional)* - gid of device owner

**`fileMode`**, **`uid`** and **`gid`** are required if **`path`** is given and are otherwise not allowed.

###### Example

```json
   "devices": [
        {
            "path": "/dev/random",
            "type": "c",
            "major": 1,
            "minor": 8,
            "permissions": "rwm",
            "fileMode": 0666,
            "uid": 0,
            "gid": 0
        },
        {
            "path": "/dev/urandom",
            "type": "c",
            "major": 1,
            "minor": 9,
            "permissions": "rwm",
            "fileMode": 0666,
            "uid": 0,
            "gid": 0
        },
        {
            "path": "/dev/null",
            "type": "c",
            "major": 1,
            "minor": 3,
            "permissions": "rwm",
            "fileMode": 0666,
            "uid": 0,
            "gid": 0
        },
        {
            "path": "/dev/zero",
            "type": "c",
            "major": 1,
            "minor": 5,
            "permissions": "rwm",
            "fileMode": 0666,
            "uid": 0,
            "gid": 0
        },
        {
            "path": "/dev/tty",
            "type": "c",
            "major": 5,
            "minor": 0,
            "permissions": "rwm",
            "fileMode": 0666,
            "uid": 0,
            "gid": 0
        },
        {
            "path": "/dev/full",
            "type": "c",
            "major": 1,
            "minor": 7,
            "permissions": "rwm",
            "fileMode": 0666,
            "uid": 0,
            "gid": 0
        }
    ]
```

## Control groups

Also known as cgroups, they are used to restrict resource usage for a container and handle device access.
cgroups provide controls to restrict cpu, memory, IO, pids and network for the container.
For more information, see the [kernel cgroups documentation](https://www.kernel.org/doc/Documentation/cgroups/cgroups.txt).

The path to the cgroups can be specified in the Spec via `cgroupsPath`.
`cgroupsPath` is expected to be relative to the cgroups mount point.
If not specified, cgroups will be created under '/'.
Implementations of the Spec can choose to name cgroups in any manner.
The Spec does not include naming schema for cgroups.
The Spec does not support [split hierarchy](https://www.kernel.org/doc/Documentation/cgroups/unified-hierarchy.txt).
The cgroups will be created if they don't exist.

```json
   "cgroupsPath": "/myRuntime/myContainer"
```

`cgroupsPath` can be used to either control the cgroups hierarchy for containers or to run a new process in an existing container.

You can configure a container's cgroups via the `resources` field of the Linux configuration.
Do not specify `resources` unless limits have to be updated.
For example, to run a new process in an existing container without updating limits, `resources` need not be specified.

#### Disable out-of-memory killer

`disableOOMKiller` contains a boolean (`true` or `false`) that enables or disables the Out of Memory killer for a cgroup.
If enabled (`false`), tasks that attempt to consume more memory than they are allowed are immediately killed by the OOM killer.
The OOM killer is enabled by default in every cgroup using the `memory` subsystem.
To disable it, specify a value of `true`.
For more information, see [the memory cgroup man page](https://www.kernel.org/doc/Documentation/cgroups/memory.txt).

* **`disableOOMKiller`** *(bool, optional)* - enables or disables the OOM killer

###### Example

```json
    "disableOOMKiller": false
```

#### Set oom_score_adj

More information on `oom_score_adj` available [here](https://www.kernel.org/doc/Documentation/filesystems/proc.txt).

```json
    "oomScoreAdj": 0
```

#### Memory

`memory` represents the cgroup subsystem `memory` and it's used to set limits on the container's memory usage.
For more information, see [the memory cgroup man page](https://www.kernel.org/doc/Documentation/cgroups/memory.txt).

The following parameters can be specified to setup the controller:

* **`limit`** *(uint64, optional)* - sets limit of memory usage

* **`reservation`** *(uint64, optional)* - sets soft limit of memory usage

* **`swap`** *(uint64, optional)* - sets limit of memory+Swap usage

* **`kernel`** *(uint64, optional)* - sets hard limit for kernel memory

* **`swappiness`** *(uint64, optional)* - sets swappiness parameter of vmscan (See sysctl's vm.swappiness)

###### Example

```json
    "memory": {
        "limit": 0,
        "reservation": 0,
        "swap": 0,
        "kernel": 0,
        "swappiness": -1
    }
```

#### CPU

`cpu` represents the cgroup subsystems `cpu` and `cpusets`.
For more information, see [the cpusets cgroup man page](https://www.kernel.org/doc/Documentation/cgroups/cpusets.txt).

The following parameters can be specified to setup the controller:

* **`shares`** *(uint64, optional)* - specifies a relative share of CPU time available to the tasks in a cgroup

* **`quota`** *(uint64, optional)* - specifies the total amount of time in microseconds for which all tasks in a cgroup can run during one period (as defined by **`period`** below)

* **`period`** *(uint64, optional)* - specifies a period of time in microseconds for how regularly a cgroup's access to CPU resources should be reallocated (CFS scheduler only)

* **`realtimeRuntime`** *(uint64, optional)* - specifies a period of time in microseconds for the longest continuous period in which the tasks in a cgroup have access to CPU resources

* **`realtimePeriod`** *(uint64, optional)* - same as **`period`** but applies to realtime scheduler only

* **`cpus`** *(cpus, optional)* - list of CPUs the container will run in

* **`mems`** *(mems, optional)* - list of Memory Nodes the container will run in

###### Example

```json
    "cpu": {
        "shares": 0,
        "quota": 0,
        "period": 0,
        "realtimeRuntime": 0,
        "realtimePeriod": 0,
        "cpus": "",
        "mems": ""
    }
```

#### Block IO Controller

`blockIO` represents the cgroup subsystem `blkio` which implements the block io controller.
For more information, see [the kernel cgroups documentation about blkio](https://www.kernel.org/doc/Documentation/cgroups/blkio-controller.txt).

The following parameters can be specified to setup the controller:

* **`blkioWeight`** *(uint16, optional)* - specifies per-cgroup weight. This is default weight of the group on all devices until and unless overridden by per-device rules. The range is from 10 to 1000.

* **`blkioLeafWeight`** *(uint16, optional)* - equivalents of `blkioWeight` for the purpose of deciding how much weight tasks in the given cgroup has while competing with the cgroup's child cgroups. The range is from 10 to 1000.

* **`blkioWeightDevice`** *(array, optional)* - specifies the list of devices which will be bandwidth rate limited. The following parameters can be specified per-device:
    * **`major, minor`** *(int64, required)* - major, minor numbers for device. More info in `man mknod`.
    * **`weight`** *(uint16, optional)* - bandwidth rate for the device, range is from 10 to 1000
    * **`leafWeight`** *(uint16, optional)* - bandwidth rate for the device while competing with the cgroup's child cgroups, range is from 10 to 1000, CFQ scheduler only

    You must specify at least one of `weight` or `leafWeight` in a given entry, and can specify both.

* **`blkioThrottleReadBpsDevice`**, **`blkioThrottleWriteBpsDevice`**, **`blkioThrottleReadIOPSDevice`**, **`blkioThrottleWriteIOPSDevice`** *(array, optional)* - specify the list of devices which will be IO rate limited. The following parameters can be specified per-device:
    * **`major, minor`** *(int64, required)* - major, minor numbers for device. More info in `man mknod`.
    * **`rate`** *(uint64, required)* - IO rate limit for the device

###### Example

```json
    "blockIO": {
        "blkioWeight": 0,
        "blkioLeafWeight": 0,
        "blkioWeightDevice": [
            {
                "major": 8,
                "minor": 0,
                "weight": 500,
                "leafWeight": 300
            },
            {
                "major": 8,
                "minor": 16,
                "weight": 500
            }
        ],
        "blkioThrottleReadBpsDevice": [
            {
                "major": 8,
                "minor": 0,
                "rate": 600
            }
        ],
        "blkioThrottleWriteIOPSDevice": [
            {
                "major": 8,
                "minor": 16,
                "rate": 300
            }
        ]
    }
```

#### Huge page limits

`hugepageLimits` represents the `hugetlb` controller which allows to limit the
HugeTLB usage per control group and enforces the controller limit during page fault.
For more information, see the [kernel cgroups documentation about HugeTLB](https://www.kernel.org/doc/Documentation/cgroups/hugetlb.txt).

`hugepageLimits` is an array of entries, each having the following structure:

* **`pageSize`** *(string, required)* - hugepage size

* **`limit`** *(uint64, required)* - limit in bytes of *hugepagesize* HugeTLB usage

###### Example

```json
   "hugepageLimits": [
        {
            "pageSize": "2MB",
            "limit": 9223372036854771712
        }
   ]
```

#### Network

`network` represents the cgroup subsystems `net_cls` and `net_prio`.
For more information, see [the net\_cls cgroup man page](https://www.kernel.org/doc/Documentation/cgroups/net_cls.txt) and [the net\_prio cgroup man page](https://www.kernel.org/doc/Documentation/cgroups/net_prio.txt).

The following parameters can be specified to setup these cgroup controllers:

* **`classID`** *(string, optional)* - is the network class identifier the cgroup's network packets will be tagged with

* **`priorities`** *(array, optional)* - specifies a list of objects of the priorities assigned to traffic originating from
processes in the group and egressing the system on various interfaces. The following parameters can be specified per-priority:
    * **`name`** *(string, required)* - interface name
    * **`priority`** *(uint32, required)* - priority applied to the interface

###### Example

```json
   "network": {
        "classID": "0x100001",
        "priorities": [
            {
                "name": "eth0",
                "priority": 500
            },
            {
                "name": "eth1",
                "priority": 1000
            }
        ]
   }
```

#### PIDs

`pids` represents the cgroup subsystem `pids`.
For more information, see [the pids cgroup man page](https://www.kernel.org/doc/Documentation/cgroups/pids.txt
).

The following paramters can be specified to setup the controller:

* **`limit`** *(int64, required)* - specifies the maximum number of tasks in the cgroup

###### Example

```json
   "pids": {
        "limit": 32771
   }
```

## Sysctl

sysctl allows kernel parameters to be modified at runtime for the container.
For more information, see [the man page](http://man7.org/linux/man-pages/man8/sysctl.8.html)

###### Example

```json
   "sysctl": {
        "net.ipv4.ip_forward": "1",
        "net.core.somaxconn": "256"
   }
```

## Rlimits

rlimits allow setting resource limits.
`type` is a string with a value from those defined in [the man page](http://man7.org/linux/man-pages/man2/setrlimit.2.html).
The kernel enforces the `soft` limit for a resource while the `hard` limit acts as a ceiling for that value that could be set by an unprivileged process.

###### Example

```json
   "rlimits": [
        {
            "type": "RLIMIT_NPROC",
            "soft": 1024,
            "hard": 102400
        }
   ]
```

## SELinux process label

SELinux process label specifies the label with which the processes in a container are run.
For more information about SELinux, see  [Selinux documentation](http://selinuxproject.org/page/Main_Page)

###### Example

```json
   "selinuxProcessLabel": "system_u:system_r:svirt_lxc_net_t:s0:c124,c675"
```

## Apparmor profile

Apparmor profile specifies the name of the apparmor profile that will be used for the container.
For more information about Apparmor, see [Apparmor documentation](https://wiki.ubuntu.com/AppArmor)

###### Example

```json
   "apparmorProfile": "acme_secure_profile"
```

## seccomp

Seccomp provides application sandboxing mechanism in the Linux kernel.
Seccomp configuration allows one to configure actions to take for matched syscalls and furthermore also allows matching on values passed as arguments to syscalls.
For more information about Seccomp, see [Seccomp kernel documentation](https://www.kernel.org/doc/Documentation/prctl/seccomp_filter.txt)
The actions, architectures, and operators are strings that match the definitions in seccomp.h from [libseccomp](https://github.com/seccomp/libseccomp) and are translated to corresponding values.
A valid list of constants as of Libseccomp v2.2.3 is contained below.

Architecture Constants
* `SCMP_ARCH_X86`
* `SCMP_ARCH_X86_64`
* `SCMP_ARCH_X32`
* `SCMP_ARCH_ARM`
* `SCMP_ARCH_AARCH64`
* `SCMP_ARCH_MIPS`
* `SCMP_ARCH_MIPS64`
* `SCMP_ARCH_MIPS64N32`
* `SCMP_ARCH_MIPSEL`
* `SCMP_ARCH_MIPSEL64`
* `SCMP_ARCH_MIPSEL64N32`

Action Constants:
* `SCMP_ACT_KILL`
* `SCMP_ACT_TRAP`
* `SCMP_ACT_ERRNO`
* `SCMP_ACT_TRACE`
* `SCMP_ACT_ALLOW`

Operator Constants:
* `SCMP_CMP_NE`
* `SCMP_CMP_LT`
* `SCMP_CMP_LE`
* `SCMP_CMP_EQ`
* `SCMP_CMP_GE`
* `SCMP_CMP_GT`
* `SCMP_CMP_MASKED_EQ`

###### Example

```json
   "seccomp": {
       "defaultAction": "SCMP_ACT_ALLOW",
       "architectures": [
           "SCMP_ARCH_X86"
       ],
       "syscalls": [
           {
               "name": "getcwd",
               "action": "SCMP_ACT_ERRNO"
           }
       ]
   }
```

## Rootfs Mount Propagation

rootfsPropagation sets the rootfs's mount propagation.
Its value is either slave, private, or shared.
[The kernel doc](https://www.kernel.org/doc/Documentation/filesystems/sharedsubtree.txt) has more information about mount propagation.

###### Example

```json
    "rootfsPropagation": "slave",
```
