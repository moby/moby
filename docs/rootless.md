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

* Either [slirp4netns](https://github.com/rootless-containers/slirp4netns) (v0.3+) or [VPNKit](https://github.com/moby/vpnkit) needs to be installed. slirp4netns is preferred for the best performance.

### Distribution-specific hint

#### Debian (excluding Ubuntu)
* `sudo sh -c "echo 1 > /proc/sys/kernel/unprivileged_userns_clone"` is required

#### Arch Linux
* `sudo sh -c "echo 1 > /proc/sys/kernel/unprivileged_userns_clone"` is required

#### openSUSE
* `sudo modprobe ip_tables iptable_mangle iptable_nat iptable_filter` is required. (This is likely to be required on other distros as well)

#### RHEL/CentOS 7
* `sudo sh -c "echo 28633 > /proc/sys/user/max_user_namespaces"` is required
* [COPR package `vbatts/shadow-utils-newxidmap`](https://copr.fedorainfracloud.org/coprs/vbatts/shadow-utils-newxidmap/) needs to be installed

## Restrictions

* Only `vfs` graphdriver is supported. However, on [Ubuntu](http://kernel.ubuntu.com/git/ubuntu/ubuntu-artful.git/commit/fs/overlayfs?h=Ubuntu-4.13.0-25.29&id=0a414bdc3d01f3b61ed86cfe3ce8b63a9240eba7) and a few distros, `overlay2` and `overlay` are also supported.
* Following features are not supported:
  * Cgroups (including `docker top`, which depends on the cgroups device controller)
  * Apparmor
  * Checkpoint
  * Overlay network

## Usage

### Daemon

You need to run `dockerd-rootless.sh` instead of `dockerd`.

```console
$ dockerd-rootless.sh --experimental"
```
As Rootless mode is experimental per se, currently you always need to run `dockerd-rootless.sh` with `--experimental`.

Remarks:
* The socket path is set to `$XDG_RUNTIME_DIR/docker.sock` by default. `$XDG_RUNTIME_DIR` is typically set to `/run/user/$UID`.
* The data dir is set to `~/.local/share/docker` by default.
* The exec dir is set to `$XDG_RUNTIME_DIR/docker` by default.
* The daemon config dir is set to `~/.config/docker` (not `~/.docker`, which is used by the client) by default.
* The `dockerd-rootless.sh` script executes `dockerd` in its own user, mount, and network namespaces. You can enter the namespaces by running `nsenter -U --preserve-credentials -n -m -t $(cat $XDG_RUNTIME_DIR/docker.pid)`.

### Client

You can just use the upstream Docker client but you need to set the socket path explicitly.

```console
$ docker -H unix://$XDG_RUNTIME_DIR/docker.sock run -d nginx
```

### Exposing ports

In addition to exposing container ports to the `dockerd` network namespace, you also need to expose the ports in the `dockerd` network namespace to the host network namespace.

```console
$ docker -H unix://$XDG_RUNTIME_DIR/docker.sock run -d -p 80:80 nginx
$ socat -t -- TCP-LISTEN:8080,reuseaddr,fork EXEC:"nsenter -U -n -t $(cat $XDG_RUNTIME_DIR/docker.pid) socat -t -- STDIN TCP4\:127.0.0.1\:80"
```

In future, `dockerd` will be able to expose the ports automatically.

### Routing ping packets

To route ping packets, you need to set up `net.ipv4.ping_group_range` properly as the root.

```console
$ sudo sh -c "echo 0   2147483647  > /proc/sys/net/ipv4/ping_group_range"
```
