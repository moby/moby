# Rootless mode (Experimental)

The rootless mode allows running `dockerd` as an unprivileged user, using `user_namespaces(7)`, `mount_namespaces(7)`, `network_namespaces(7)`.

No SETUID/SETCAP binary is required except `newuidmap` and `newgidmap`.

## Requirements
* `newuidmap` and `newgidmap` need to be installed on the host. These commands are provided by the `uidmap` package on most distros.

* `/etc/subuid` and `/etc/subgid` should contain >= 65536 sub-IDs. e.g. `penguin:231072:65536`.

```console
$ id -u
1001
$ whoami
penguin
$ grep ^$(whoami): /etc/subuid
penguin:231072:65536
$ grep ^$(whoami): /etc/subgid
penguin:231072:65536
```

### Distribution-specific hint

Using Ubuntu kernel is recommended.

#### Ubuntu
* No preparation is needed.
* `overlay2` is enabled by default ([Ubuntu-specific kernel patch](https://kernel.ubuntu.com/git/ubuntu/ubuntu-bionic.git/commit/fs/overlayfs?id=3b7da90f28fe1ed4b79ef2d994c81efbc58f1144)).
* Known to work on Ubuntu 16.04 and 18.04.

#### Debian GNU/Linux
* Add `kernel.unprivileged_userns_clone=1` to `/etc/sysctl.conf` (or `/etc/sysctl.d`) and run `sudo sysctl -p`
* To use `overlay2` storage driver (recommended), run `sudo modprobe overlay permit_mounts_in_userns=1` ([Debian-specific kernel patch, introduced in Debian 10](https://salsa.debian.org/kernel-team/linux/blob/283390e7feb21b47779b48e0c8eb0cc409d2c815/debian/patches/debian/overlayfs-permit-mounts-in-userns.patch)). Put the configuration to `/etc/modprobe.d` for persistence.
* Known to work on Debian 9 and 10. `overlay2` is only supported since Debian 10 and needs `modprobe` configuration described above.

#### Arch Linux
* Add `kernel.unprivileged_userns_clone=1` to `/etc/sysctl.conf` (or `/etc/sysctl.d`) and run `sudo sysctl -p`

#### openSUSE
* `sudo modprobe ip_tables iptable_mangle iptable_nat iptable_filter` is required. (This is likely to be required on other distros as well)
* Known to work on openSUSE 15.

#### Fedora 31 and later
* Run `sudo grubby --update-kernel=ALL --args="systemd.unified_cgroup_hierarchy=0"` and reboot.

#### Fedora 30
* No preparation is needed

#### RHEL/CentOS 8
* No preparation is needed

#### RHEL/CentOS 7
* Add `user.max_user_namespaces=28633` to `/etc/sysctl.conf` (or `/etc/sysctl.d`) and run `sudo sysctl -p`
* `systemctl --user` does not work by default. Run the daemon directly without systemd: `dockerd-rootless.sh --experimental --storage-driver vfs`
* Known to work on RHEL/CentOS 7.7. Older releases require extra configuration steps.
* RHEL/CentOS 7.6 and older releases require [COPR package `vbatts/shadow-utils-newxidmap`](https://copr.fedorainfracloud.org/coprs/vbatts/shadow-utils-newxidmap/) to be installed.
* RHEL/CentOS 7.5 and older releases require running `sudo grubby --update-kernel=ALL --args="user_namespace.enable=1"` and reboot.

## Known limitations

* Only `vfs` graphdriver is supported. However, on Ubuntu and Debian 10, `overlay2` and `overlay` are also supported.
* Following features are not supported:
  * Cgroups (including `docker top`, which depends on the cgroups device controller)
  * Apparmor
  * Checkpoint
  * Overlay network
  * Exposing SCTP ports
* To use `ping` command, see [Routing ping packets](#routing-ping-packets)
* To expose privileged TCP/UDP ports (< 1024), see [Exposing privileged ports](#exposing-privileged-ports)

## Install

The installation script is available at https://get.docker.com/rootless .

```console
$ curl -fsSL https://get.docker.com/rootless | sh
```

Make sure to run the script as a non-root user.

The script will show the environment variables that are needed to be set:

```console
$ curl -fsSL https://get.docker.com/rootless | sh
...
# Docker binaries are installed in /home/penguin/bin
# WARN: dockerd is not in your current PATH or pointing to /home/penguin/bin/dockerd
# Make sure the following environment variables are set (or add them to ~/.bashrc):

export PATH=/home/penguin/bin:$PATH
export PATH=$PATH:/sbin
export DOCKER_HOST=unix:///run/user/1001/docker.sock

#
# To control docker service run:
# systemctl --user (start|stop|restart) docker
#
```

To install the binaries manually without using the installer, extract `docker-rootless-extras-<version>.tar.gz` along with `docker-<version>.tar.gz`: https://download.docker.com/linux/static/stable/x86_64/

## Usage

### Daemon

Use `systemctl --user` to manage the lifecycle of the daemon:
```console
$ systemctl --user start docker
```

To launch the daemon on system startup, enable systemd lingering:
```console
$ sudo loginctl enable-linger $(whoami)
```

To run the daemon directly without systemd, you need to run `dockerd-rootless.sh` instead of `dockerd`:
```console
$ dockerd-rootless.sh --experimental --storage-driver vfs
```

As Rootless mode is experimental, currently you always need to run `dockerd-rootless.sh` with `--experimental`.
You also need `--storage-driver vfs` unless using Ubuntu or Debian 10 kernel.

Remarks:
* The socket path is set to `$XDG_RUNTIME_DIR/docker.sock` by default. `$XDG_RUNTIME_DIR` is typically set to `/run/user/$UID`.
* The data dir is set to `~/.local/share/docker` by default.
* The exec dir is set to `$XDG_RUNTIME_DIR/docker` by default.
* The daemon config dir is set to `~/.config/docker` (not `~/.docker`, which is used by the client) by default.
* The `dockerd-rootless.sh` script executes `dockerd` in its own user, mount, and network namespaces. You can enter the namespaces by running `nsenter -U --preserve-credentials -n -m -t $(cat $XDG_RUNTIME_DIR/docker.pid)`.
* `docker info` shows `rootless` in `SecurityOptions`
* `docker info` shows `none` as `Cgroup Driver`

### Client

You need to set the socket path explicitly.

```console
$ export DOCKER_HOST=unix://$XDG_RUNTIME_DIR/docker.sock
$ docker run -d nginx
```

### Rootless Docker in Docker

To run Rootless Docker inside "rootful" Docker, use `docker:<version>-dind-rootless` image instead of `docker:<version>-dind` image.

```console
$ docker run -d --name dind-rootless --privileged docker:19.03-dind-rootless --experimental
```

`docker:<version>-dind-rootless` image runs as a non-root user (UID 1000).
However, `--privileged` is required for disabling seccomp, AppArmor, and mount masks.

### Expose Docker API socket via TCP

To expose the Docker API socket via TCP, you need to launch `dockerd-rootless.sh` with `DOCKERD_ROOTLESS_ROOTLESSKIT_FLAGS="-p 0.0.0.0:2376:2376/tcp"`.

```console
$ DOCKERD_ROOTLESS_ROOTLESSKIT_FLAGS="-p 0.0.0.0:2376:2376/tcp" \
 dockerd-rootless.sh --experimental \
 -H tcp://0.0.0.0:2376 \
 --tlsverify --tlscacert=ca.pem --tlscert=cert.pem --tlskey=key.pem
```

### Routing ping packets

Add `net.ipv4.ping_group_range = 0   2147483647` to `/etc/sysctl.conf` (or `/etc/sysctl.d`) and run `sudo sysctl -p`.

### Exposing privileged ports

To expose privileged ports (< 1024), set `CAP_NET_BIND_SERVICE` on `rootlesskit` binary.

```console
$ sudo setcap cap_net_bind_service=ep $HOME/bin/rootlesskit
```

Or add `net.ipv4.ip_unprivileged_port_start=0` to `/etc/sysctl.conf` (or `/etc/sysctl.d`) and run `sudo sysctl -p`.

### Limiting resources

Currently rootless mode ignores cgroup-related `docker run` flags such as `--cpus` and `memory`.
However, traditional `ulimit` and [`cpulimit`](https://github.com/opsengine/cpulimit) can be still used, though it works in process-granularity rather than container-granularity.

### Changing network stack

`dockerd-rootless.sh` uses [slirp4netns](https://github.com/rootless-containers/slirp4netns) (if installed) or [VPNKit](https://github.com/moby/vpnkit) as the network stack by default.
These network stacks run in userspace and might have performance overhead. See [RootlessKit documentation](https://github.com/rootless-containers/rootlesskit/tree/v0.6.0#network-drivers) for further information.

Optionally, you can use `lxc-user-nic` instead for the best performance.
To use `lxc-user-nic`, you need to edit [`/etc/lxc/lxc-usernet`](https://github.com/rootless-containers/rootlesskit/tree/v0.6.0#--netlxc-user-nic-experimental) and set `$DOCKERD_ROOTLESS_ROOTLESSKIT_NET=lxc-user-nic`.

