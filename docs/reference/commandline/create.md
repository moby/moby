<!--[metadata]>
+++
title = "create"
description = "The create command description and usage"
keywords = ["docker, create, container"]
[menu.main]
parent = "smn_cli"
+++
<![end-metadata]-->

# create

Creates a new container.

    Usage: docker create [OPTIONS] IMAGE [COMMAND] [ARG...]

    Create a new container

      -a, --attach=[]               Attach to STDIN, STDOUT or STDERR
      --add-host=[]                 Add a custom host-to-IP mapping (host:ip)
      --blkio-weight=0              Block IO weight (relative weight)
      --blkio-weight-device=[]      Block IO weight (relative device weight, format: `DEVICE_NAME:WEIGHT`)
      --cpu-shares=0                CPU shares (relative weight)
      --cap-add=[]                  Add Linux capabilities
      --cap-drop=[]                 Drop Linux capabilities
      --cgroup-parent=""            Optional parent cgroup for the container
      --cidfile=""                  Write the container ID to the file
      --cpu-period=0                Limit CPU CFS (Completely Fair Scheduler) period
      --cpu-quota=0                 Limit CPU CFS (Completely Fair Scheduler) quota
      --cpuset-cpus=""              CPUs in which to allow execution (0-3, 0,1)
      --cpuset-mems=""              Memory nodes (MEMs) in which to allow execution (0-3, 0,1)
      --device=[]                   Add a host device to the container
      --device-read-bps=[]          Limit read rate (bytes per second) from a device (e.g., --device-read-bps=/dev/sda:1mb)
      --device-read-iops=[]         Limit read rate (IO per second) from a device (e.g., --device-read-iops=/dev/sda:1000)
      --device-write-bps=[]         Limit write rate (bytes per second) to a device (e.g., --device-write-bps=/dev/sda:1mb)
      --device-write-iops=[]        Limit write rate (IO per second) to a device (e.g., --device-write-iops=/dev/sda:1000)
      --disable-content-trust=true  Skip image verification
      --dns=[]                      Set custom DNS servers
      --dns-opt=[]                  Set custom DNS options
      --dns-search=[]               Set custom DNS search domains
      -e, --env=[]                  Set environment variables
      --entrypoint=""               Overwrite the default ENTRYPOINT of the image
      --env-file=[]                 Read in a file of environment variables
      --expose=[]                   Expose a port or a range of ports
      --group-add=[]                Add additional groups to join
      -h, --hostname=""             Container host name
      --help                        Print usage
      -i, --interactive             Keep STDIN open even if not attached
      --ip=""                       Container IPv4 address (e.g. 172.30.100.104)
      --ip6=""                      Container IPv6 address (e.g. 2001:db8::33)
      --ipc=""                      IPC namespace to use
      --isolation=""                Container isolation technology
      --kernel-memory=""            Kernel memory limit
      -l, --label=[]                Set metadata on the container (e.g., --label=com.example.key=value)
      --label-file=[]               Read in a line delimited file of labels
      --link=[]                     Add link to another container
      --log-driver=""               Logging driver for container
      --log-opt=[]                  Log driver specific options
      -m, --memory=""               Memory limit
      --mac-address=""              Container MAC address (e.g. 92:d0:c6:0a:29:33)
      --memory-reservation=""       Memory soft limit
      --memory-swap=""              A positive integer equal to memory plus swap. Specify -1 to enable unlimited swap.
      --memory-swappiness=""        Tune a container's memory swappiness behavior. Accepts an integer between 0 and 100.
      --name=""                     Assign a name to the container
      --net="bridge"                Connect a container to a network
                                    'bridge': create a network stack on the default Docker bridge
                                    'none': no networking
                                    'container:<name|id>': reuse another container's network stack
                                    'host': use the Docker host network stack
                                    '<network-name>|<network-id>': connect to a user-defined network
      --net-alias=[]                Add network-scoped alias for the container
      --oom-kill-disable            Whether to disable OOM Killer for the container or not
      --oom-score-adj=0             Tune the host's OOM preferences for containers (accepts -1000 to 1000)
      -P, --publish-all             Publish all exposed ports to random ports
      -p, --publish=[]              Publish a container's port(s) to the host
      --pid=""                      PID namespace to use
      --pids-limit=-1                Tune container pids limit (set -1 for unlimited), kernel >= 4.3
      --privileged                  Give extended privileges to this container
      --read-only                   Mount the container's root filesystem as read only
      --restart="no"                Restart policy (no, on-failure[:max-retry], always, unless-stopped)
      --security-opt=[]             Security options
      --stop-signal="SIGTERM"       Signal to stop a container
      --shm-size=[]                 Size of `/dev/shm`. The format is `<number><unit>`. `number` must be greater than `0`.  Unit is optional and can be `b` (bytes), `k` (kilobytes), `m` (megabytes), or `g` (gigabytes). If you omit the unit, the system uses bytes. If you omit the size entirely, the system uses `64m`.
      --storage-opt=[]              Set storage driver options per container
      --sysctl[=*[]*]]              Configure namespaced kernel parameters at runtime
      -t, --tty                     Allocate a pseudo-TTY
      -u, --user=""                 Username or UID
      --userns=""                   Container user namespace
                                    'host': Use the Docker host user namespace
                                    '': Use the Docker daemon user namespace specified by `--userns-remap` option.
      --ulimit=[]                   Ulimit options
      --uts=""                      UTS namespace to use
      -v, --volume=[host-src:]container-dest[:<options>]
                                    Bind mount a volume. The comma-delimited
                                    `options` are [rw|ro], [z|Z],
                                    [[r]shared|[r]slave|[r]private], and
                                    [nocopy]. The 'host-src' is an absolute path
                                    or a name value.
      --volume-driver=""            Container's volume driver
      --volumes-from=[]             Mount volumes from the specified container(s)
      -w, --workdir=""              Working directory inside the container

The `docker create` command creates a writeable container layer over the
specified image and prepares it for running the specified command.  The
container ID is then printed to `STDOUT`.  This is similar to `docker run -d`
except the container is never started.  You can then use the
`docker start <container_id>` command to start the container at any point.

This is useful when you want to set up a container configuration ahead of time
so that it is ready to start when you need it. The initial status of the
new container is `created`.

Please see the [run command](run.md) section and the [Docker run reference](../run.md) for more details.

## Examples

    $ docker create -t -i fedora bash
    6d8af538ec541dd581ebc2a24153a28329acb5268abe5ef868c1f1a261221752
    $ docker start -a -i 6d8af538ec5
    bash-4.2#

As of v1.4.0 container volumes are initialized during the `docker create` phase
(i.e., `docker run` too). For example, this allows you to `create` the `data`
volume container, and then use it from another container:

    $ docker create -v /data --name data ubuntu
    240633dfbb98128fa77473d3d9018f6123b99c454b3251427ae190a7d951ad57
    $ docker run --rm --volumes-from data ubuntu ls -la /data
    total 8
    drwxr-xr-x  2 root root 4096 Dec  5 04:10 .
    drwxr-xr-x 48 root root 4096 Dec  5 04:11 ..

Similarly, `create` a host directory bind mounted volume container, which can
then be used from the subsequent container:

    $ docker create -v /home/docker:/docker --name docker ubuntu
    9aa88c08f319cd1e4515c3c46b0de7cc9aa75e878357b1e96f91e2c773029f03
    $ docker run --rm --volumes-from docker ubuntu ls -la /docker
    total 20
    drwxr-sr-x  5 1000 staff  180 Dec  5 04:00 .
    drwxr-xr-x 48 root root  4096 Dec  5 04:13 ..
    -rw-rw-r--  1 1000 staff 3833 Dec  5 04:01 .ash_history
    -rw-r--r--  1 1000 staff  446 Nov 28 11:51 .ashrc
    -rw-r--r--  1 1000 staff   25 Dec  5 04:00 .gitconfig
    drwxr-sr-x  3 1000 staff   60 Dec  1 03:28 .local
    -rw-r--r--  1 1000 staff  920 Nov 28 11:51 .profile
    drwx--S---  2 1000 staff  460 Dec  5 00:51 .ssh
    drwxr-xr-x 32 1000 staff 1140 Dec  5 04:01 docker

Set storage driver options per container. 

    $ docker create -it --storage-opt size=120G fedora /bin/bash

This (size) will allow to set the container rootfs size to 120G at creation time. 
User cannot pass a size less than the Default BaseFS Size. 

### Specify isolation technology for container (--isolation)

This option is useful in situations where you are running Docker containers on
Windows. The `--isolation=<value>` option sets a container's isolation
technology. On Linux, the only supported is the `default` option which uses
Linux namespaces. On Microsoft Windows, you can specify these values:


| Value     | Description                                                                                                                                                   |
|-----------|---------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `default` | Use the value specified by the Docker daemon's `--exec-opt` . If the `daemon` does not specify an isolation technology, Microsoft Windows uses `process` as its default value if the
daemon is running on Windows server, or `hyperv` if running on Windows client.  |
| `process` | Namespace isolation only.                                                                                                                                     |
| `hyperv`   | Hyper-V hypervisor partition-based isolation.                                                                                                                  |

Specifying the `--isolation` flag without a value is the same as setting `--isolation="default"`.
