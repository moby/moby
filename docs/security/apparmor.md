<!-- [metadata]>
+++
title = "AppArmor security profiles for Docker"
description = "Enabling AppArmor in Docker"
keywords = ["AppArmor, security, docker, documentation"]
[menu.main]
parent= "smn_secure_docker"
weight=5
+++
<![end-metadata]-->

# AppArmor security profiles for Docker

AppArmor (Application Armor) is a Linux security module that protects an
operating system and its applications from security threats. To use it, a system
administrator associates an AppArmor security profile with each program. Docker
expects to find an AppArmor policy loaded and enforced.

Docker automatically loads container profiles. A profile for the Docker Engine
itself also exists and is installed with the official *.deb* packages in
`/etc/apparmor.d/docker` file.


## Understand the policies

The `docker-default` profile is the default for running containers. It is
moderately protective while providing wide application compatibility. The
profile is the following:

```
#include <tunables/global>


profile docker-default flags=(attach_disconnected,mediate_deleted) {

  #include <abstractions/base>


  network,
  capability,
  file,
  umount,

  deny @{PROC}/{*,**^[0-9*],sys/kernel/shm*} wkx,
  deny @{PROC}/sysrq-trigger rwklx,
  deny @{PROC}/mem rwklx,
  deny @{PROC}/kmem rwklx,
  deny @{PROC}/kcore rwklx,

  deny mount,

  deny /sys/[^f]*/** wklx,
  deny /sys/f[^s]*/** wklx,
  deny /sys/fs/[^c]*/** wklx,
  deny /sys/fs/c[^g]*/** wklx,
  deny /sys/fs/cg[^r]*/** wklx,
  deny /sys/firmware/efi/efivars/** rwklx,
  deny /sys/kernel/security/** rwklx,
}
```

When you run a container, it uses the `docker-default` policy unless you
override it with the `security-opt` option. For example, the following
explicitly specifies the default policy:

```bash
$ docker run --rm -it --security-opt apparmor:docker-default hello-world
```

## Contributing to AppArmor code in Docker

Advanced users and package managers can find a profile for `/usr/bin/docker`
underneath
[contrib/apparmor](https://github.com/docker/docker/tree/master/contrib/apparmor)
in the Docker Engine source repository.
