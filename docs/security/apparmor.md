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

A profile for the Docker Engine daemon exists but it is not currently installed
with the `deb` packages. If you are interested in the source for the daemon
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
$ docker run --rm -it --security-opt apparmor=docker-default hello-world
```

## Load and unload profiles

To load a new profile into AppArmor for use with containers:

```bash
$ apparmor_parser -r -W /path/to/your_profile
```

Then, run the custom profile with `--security-opt` like so:

```bash
$ docker run --rm -it --security-opt apparmor=your_profile hello-world
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

### Resources for writing profiles

The syntax for file globbing in AppArmor is a bit different than some other
globbing implementations. It is highly suggested you take a look at some of the
below resources with regard to AppArmor profile syntax.

- [Quick Profile Language](http://wiki.apparmor.net/index.php/QuickProfileLanguage)
- [Globbing Syntax](http://wiki.apparmor.net/index.php/AppArmor_Core_Policy_Reference#AppArmor_globbing_syntax)

## Nginx example profile

In this example, you create a custom AppArmor profile for Nginx. Below is the
custom profile.

```
#include <tunables/global>


profile docker-nginx flags=(attach_disconnected,mediate_deleted) {
  #include <abstractions/base>

  network inet tcp,
  network inet udp,
  network inet icmp,

  deny network raw,

  deny network packet,

  file,
  umount,

  deny /bin/** wl,
  deny /boot/** wl,
  deny /dev/** wl,
  deny /etc/** wl,
  deny /home/** wl,
  deny /lib/** wl,
  deny /lib64/** wl,
  deny /media/** wl,
  deny /mnt/** wl,
  deny /opt/** wl,
  deny /proc/** wl,
  deny /root/** wl,
  deny /sbin/** wl,
  deny /srv/** wl,
  deny /tmp/** wl,
  deny /sys/** wl,
  deny /usr/** wl,

  audit /** w,

  /var/run/nginx.pid w,

  /usr/sbin/nginx ix,

  deny /bin/dash mrwklx,
  deny /bin/sh mrwklx,
  deny /usr/bin/top mrwklx,


  capability chown,
  capability dac_override,
  capability setuid,
  capability setgid,
  capability net_bind_service,

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

1. Save the custom profile to disk in the
`/etc/apparmor.d/containers/docker-nginx` file.

    The file path in this example is not a requirement. In production, you could
    use another.

2. Load the profile.

    ```bash
    $ sudo apparmor_parser -r -W /etc/apparmor.d/containers/docker-nginx
    ```

3. Run a container with the profile.

    To run nginx in detached mode:

    ```bash
    $ docker run --security-opt "apparmor=docker-nginx" \
        -p 80:80 -d --name apparmor-nginx nginx
    ```

4. Exec into the running container

    ```bash
    $ docker exec -it apparmor-nginx bash
    ```

5. Try some operations to test the profile.

    ```bash
    root@6da5a2a930b9:~# ping 8.8.8.8
    ping: Lacking privilege for raw socket.

    root@6da5a2a930b9:/# top
    bash: /usr/bin/top: Permission denied

    root@6da5a2a930b9:~# touch ~/thing
    touch: cannot touch 'thing': Permission denied

    root@6da5a2a930b9:/# sh
    bash: /bin/sh: Permission denied

    root@6da5a2a930b9:/# dash
    bash: /bin/dash: Permission denied
    ```


Congrats! You just deployed a container secured with a custom apparmor profile!


## Debug AppArmor

You can use `dmesg` to debug problems and `aa-status` check the loaded profiles.

### Use dmesg

Here are some helpful tips for debugging any problems you might be facing with
regard to AppArmor.

AppArmor sends quite verbose messaging to `dmesg`. Usually an AppArmor line
looks like the following:

```
[ 5442.864673] audit: type=1400 audit(1453830992.845:37): apparmor="ALLOWED" operation="open" profile="/usr/bin/docker" name="/home/jessie/docker/man/man1/docker-attach.1" pid=10923 comm="docker" requested_mask="r" denied_mask="r" fsuid=1000 ouid=0
```

In the above example, you can see `profile=/usr/bin/docker`. This means the
user has the `docker-engine` (Docker Engine Daemon) profile loaded.

> **Note:** On version of Ubuntu > 14.04 this is all fine and well, but Trusty
> users might run into some issues when trying to `docker exec`.

Look at another log line:

```
[ 3256.689120] type=1400 audit(1405454041.341:73): apparmor="DENIED" operation="ptrace" profile="docker-default" pid=17651 comm="docker" requested_mask="receive" denied_mask="receive"
```

This time the profile is `docker-default`, which is run on containers by
default unless in `privileged` mode. This line shows that apparmor has denied
`ptrace` in the container. This is exactly as expected.

### Use aa-status

If you need to check which profiles are loaded,  you can use `aa-status`. The
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

The above output shows that the `docker-default` profile running on various
container PIDs is in `enforce` mode. This means AppArmor is actively blocking
and auditing in `dmesg` anything outside the bounds of the `docker-default`
profile.

The output above also shows the `/usr/bin/docker` (Docker Engine daemon) profile
is running in `complain` mode. This means AppArmor _only_ logs to `dmesg`
activity outside the bounds of the profile. (Except in the case of Ubuntu
Trusty, where some interesting behaviors are enforced.)

## Contribute Docker's AppArmor code

Advanced users and package managers can find a profile for `/usr/bin/docker`
(Docker Engine Daemon) underneath
[contrib/apparmor](https://github.com/docker/docker/tree/master/contrib/apparmor)
in the Docker Engine source repository.

The `docker-default` profile for containers lives in
[profiles/apparmor](https://github.com/docker/docker/tree/master/profiles/apparmor).
