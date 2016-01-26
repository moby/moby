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

Docker automatically loads container profiles. The Docker binary installs
a `docker-default` profile in the `/etc/apparmor.d/docker` file. This profile
is used on containers, _not_ on the Docker Daemon.

A profile for the Docker Engine Daemon exists but it is not currently installed 
with the deb packages. If you are interested in the source for the Daemon
profile, it is located in
[contrib/apparmor](https://github.com/docker/docker/tree/master/contrib/apparmor)
in the Docker Engine source repository.

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

## Loading and Unloading Profiles

To load a new profile into AppArmor, for use with containers:

```
$ apparmor_parser -r -W /path/to/your_profile
```

Then you can run the custom profile with `--security-opt` like so:

```bash
$ docker run --rm -it --security-opt apparmor:your_profile hello-world
```

To unload a profile from AppArmor:

```bash
# stop apparmor
$ /etc/init.d/apparmor stop
# unload the profile
$ apparmor_parser -R /path/to/profile
# start apparmor
$ /etc/init.d/apparmor start
```

## Debugging AppArmor

### Using `dmesg`

Here are some helpful tips for debugging any problems you might be facing with
regard to AppArmor.

AppArmor sends quite verbose messaging to `dmesg`. Usually an AppArmor line
will look like the following:

```
[ 5442.864673] audit: type=1400 audit(1453830992.845:37): apparmor="ALLOWED" operation="open" profile="/usr/bin/docker" name="/home/jessie/docker/man/man1/docker-attach.1" pid=10923 comm="docker" requested_mask="r" denied_mask="r" fsuid=1000 ouid=0
```

In the above example, the you can see `profile=/usr/bin/docker`. This means the
user has the `docker-engine` (Docker Engine Daemon) profile loaded.

> **Note:** On version of Ubuntu > 14.04 this is all fine and well, but Trusty
> users might run into some issues when trying to `docker exec`.

Let's look at another log line:

```
[ 3256.689120] type=1400 audit(1405454041.341:73): apparmor="DENIED" operation="ptrace" profile="docker-default" pid=17651 comm="docker" requested_mask="receive" denied_mask="receive"
```

This time the profile is `docker-default`, which is run on containers by
default unless in `privileged` mode. It is telling us, that apparmor has denied
`ptrace` in the container. This is great.

### Using `aa-status`

If you need to check which profiles are loaded you can use `aa-status`. The
output looks like:

```bash
$ sudo aa-status
apparmor module is loaded.
14 profiles are loaded.
1 profiles are in enforce mode.
   docker-default
13 profiles are in complain mode.
   /usr/bin/docker
   /usr/bin/docker///bin/cat
   /usr/bin/docker///bin/ps
   /usr/bin/docker///sbin/apparmor_parser
   /usr/bin/docker///sbin/auplink
   /usr/bin/docker///sbin/blkid
   /usr/bin/docker///sbin/iptables
   /usr/bin/docker///sbin/mke2fs
   /usr/bin/docker///sbin/modprobe
   /usr/bin/docker///sbin/tune2fs
   /usr/bin/docker///sbin/xtables-multi
   /usr/bin/docker///sbin/zfs
   /usr/bin/docker///usr/bin/xz
38 processes have profiles defined.
37 processes are in enforce mode.
   docker-default (6044)
   ...
   docker-default (31899)
1 processes are in complain mode.
   /usr/bin/docker (29756)
0 processes are unconfined but have a profile defined.
```

In the above output you can tell that the `docker-default` profile running on
various container PIDs is in `enforce` mode. This means AppArmor will actively
block and audit in `dmesg` anything outside the bounds of the `docker-default`
profile.

The output above also shows the `/usr/bin/docker` (Docker Engine Daemon)
profile is running in `complain` mode. This means AppArmor will _only_ log to
`dmesg` activity outside the bounds of the profile. (Except in the case of
Ubuntu Trusty, where we have seen some interesting behaviors being enforced.)

## Contributing to AppArmor code in Docker

Advanced users and package managers can find a profile for `/usr/bin/docker`
(Docker Engine Daemon) underneath
[contrib/apparmor](https://github.com/docker/docker/tree/master/contrib/apparmor)
in the Docker Engine source repository.

The `docker-default` profile for containers lives in
[profiles/apparmor](https://github.com/docker/docker/tree/master/profiles/apparmor).
