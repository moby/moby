<!-- [metadata]>
+++
title = "Seccomp security profiles for Docker"
description = "Enabling seccomp in Docker"
keywords = ["seccomp, security, docker, documentation"]
+++
<![end-metadata]-->

Seccomp security profiles for Docker
------------------------------------

The seccomp() system call operates on the Secure Computing (seccomp)
state of the calling process.

This operation is available only if the kernel is configured
with `CONFIG_SECCOMP` enabled.

This allows for allowing or denying of certain syscalls in a container.

Passing a profile for a container
---------------------------------

Users may pass a seccomp profile using the `security-opt` option
(per-container).

The profile has layout in the following form:

```
{
    "defaultAction": "SCMP_ACT_ALLOW",
    "syscalls": [
        {
            "name": "getcwd",
            "action": "SCMP_ACT_ERRNO"
        },
        {
            "name": "mount",
            "action": "SCMP_ACT_ERRNO"
        },
        {
            "name": "setns",
            "action": "SCMP_ACT_ERRNO"
        },
        {
            "name": "create_module",
            "action": "SCMP_ACT_ERRNO"
        },
        {
            "name": "chown",
            "action": "SCMP_ACT_ERRNO"
        },
        {
            "name": "chmod",
            "action": "SCMP_ACT_ERRNO"
        }
    ]
}
```

Then you can run with:

```
$ docker run --rm -it --security-opt seccomp:/path/to/seccomp/profile.json hello-world
```
