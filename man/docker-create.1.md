% DOCKER(1) Docker User Manuals
% Docker Community
% JUNE 2014
# NAME
docker-create - Create a new container

# SYNOPSIS
**docker create**
[**-a**|**--attach**[=*[]*]]
[**--add-host**[=*[]*]]
[**--blkio-weight**[=*[BLKIO-WEIGHT]*]]
[**--blkio-weight-device**[=*[]*]]
[**--cpu-shares**[=*0*]]
[**--cap-add**[=*[]*]]
[**--cap-drop**[=*[]*]]
[**--cgroup-parent**[=*CGROUP-PATH*]]
[**--cidfile**[=*CIDFILE*]]
[**--cpu-period**[=*0*]]
[**--cpu-quota**[=*0*]]
[**--cpuset-cpus**[=*CPUSET-CPUS*]]
[**--cpuset-mems**[=*CPUSET-MEMS*]]
[**--device**[=*[]*]]
[**--device-read-bps**[=*[]*]]
[**--device-read-iops**[=*[]*]]
[**--device-write-bps**[=*[]*]]
[**--device-write-iops**[=*[]*]]
[**--dns**[=*[]*]]
[**--dns-search**[=*[]*]]
[**--dns-opt**[=*[]*]]
[**-e**|**--env**[=*[]*]]
[**--entrypoint**[=*ENTRYPOINT*]]
[**--env-file**[=*[]*]]
[**--expose**[=*[]*]]
[**--group-add**[=*[]*]]
[**-h**|**--hostname**[=*HOSTNAME*]]
[**--help**]
[**-i**|**--interactive**]
[**--ip**[=*IPv4-ADDRESS*]]
[**--ip6**[=*IPv6-ADDRESS*]]
[**--ipc**[=*IPC*]]
[**--isolation**[=*default*]]
[**--kernel-memory**[=*KERNEL-MEMORY*]]
[**-l**|**--label**[=*[]*]]
[**--label-file**[=*[]*]]
[**--link**[=*[]*]]
[**--log-driver**[=*[]*]]
[**--log-opt**[=*[]*]]
[**-m**|**--memory**[=*MEMORY*]]
[**--mac-address**[=*MAC-ADDRESS*]]
[**--memory-reservation**[=*MEMORY-RESERVATION*]]
[**--memory-swap**[=*LIMIT*]]
[**--memory-swappiness**[=*MEMORY-SWAPPINESS*]]
[**--name**[=*NAME*]]
[**--net**[=*"bridge"*]]
[**--net-alias**[=*[]*]]
[**--oom-kill-disable**]
[**--oom-score-adj**[=*0*]]
[**-P**|**--publish-all**]
[**-p**|**--publish**[=*[]*]]
[**--pid**[=*[]*]]
[**--pids-limit**[=*PIDS_LIMIT*]]
[**--privileged**]
[**--read-only**]
[**--restart**[=*RESTART*]]
[**--security-opt**[=*[]*]]
[**--stop-signal**[=*SIGNAL*]]
[**--shm-size**[=*[]*]]
[**-t**|**--tty**]
[**--tmpfs**[=*[CONTAINER-DIR[:<OPTIONS>]*]]
[**-u**|**--user**[=*USER*]]
[**--ulimit**[=*[]*]]
[**--uts**[=*[]*]]
[**-v**|**--volume**[=*[[HOST-DIR:]CONTAINER-DIR[:OPTIONS]]*]]
[**--volume-driver**[=*DRIVER*]]
[**--volumes-from**[=*[]*]]
[**-w**|**--workdir**[=*WORKDIR*]]
IMAGE [COMMAND] [ARG...]

# DESCRIPTION

Creates a writeable container layer over the specified image and prepares it for
running the specified command. The container ID is then printed to STDOUT. This
is similar to **docker run -d** except the container is never started. You can 
then use the **docker start <container_id>** command to start the container at
any point.

The initial status of the container created with **docker create** is 'created'.

# OPTIONS
**-a**, **--attach**=[]
   Attach to STDIN, STDOUT or STDERR.

**--add-host**=[]
   Add a custom host-to-IP mapping (host:ip)

**--blkio-weight**=*0*
   Block IO weight (relative weight) accepts a weight value between 10 and 1000.

**--blkio-weight-device**=[]
   Block IO weight (relative device weight, format: `DEVICE_NAME:WEIGHT`).

**--cpu-shares**=*0*
   CPU shares (relative weight)

**--cap-add**=[]
   Add Linux capabilities

**--cap-drop**=[]
   Drop Linux capabilities

**--cgroup-parent**=""
   Path to cgroups under which the cgroup for the container will be created. If the path is not absolute, the path is considered to be relative to the cgroups path of the init process. Cgroups will be created if they do not already exist.

**--cidfile**=""
   Write the container ID to the file

**--cpu-period**=*0*
    Limit the CPU CFS (Completely Fair Scheduler) period

**--cpuset-cpus**=""
   CPUs in which to allow execution (0-3, 0,1)

**--cpuset-mems**=""
   Memory nodes (MEMs) in which to allow execution (0-3, 0,1). Only effective on NUMA systems.

   If you have four memory nodes on your system (0-3), use `--cpuset-mems=0,1`
then processes in your Docker container will only use memory from the first
two memory nodes.

**--cpu-quota**=*0*
   Limit the CPU CFS (Completely Fair Scheduler) quota

**--device**=[]
   Add a host device to the container (e.g. --device=/dev/sdc:/dev/xvdc:rwm)

**--device-read-bps**=[]
    Limit read rate (bytes per second) from a device (e.g. --device-read-bps=/dev/sda:1mb)

**--device-read-iops**=[]
    Limit read rate (IO per second) from a device (e.g. --device-read-iops=/dev/sda:1000)

**--device-write-bps**=[]
    Limit write rate (bytes per second) to a device (e.g. --device-write-bps=/dev/sda:1mb)

**--device-write-iops**=[]
    Limit write rate (IO per second) to a device (e.g. --device-write-iops=/dev/sda:1000)

**--dns**=[]
   Set custom DNS servers

**--dns-opt**=[]
   Set custom DNS options

**--dns-search**=[]
   Set custom DNS search domains (Use --dns-search=. if you don't wish to set the search domain)

**-e**, **--env**=[]
   Set environment variables

**--entrypoint**=""
   Overwrite the default ENTRYPOINT of the image

**--env-file**=[]
   Read in a line-delimited file of environment variables

**--expose**=[]
   Expose a port or a range of ports (e.g. --expose=3300-3310) from the container without publishing it to your host

**--group-add**=[]
   Add additional groups to run as

**-h**, **--hostname**=""
   Container host name

**--help**
  Print usage statement

**-i**, **--interactive**=*true*|*false*
   Keep STDIN open even if not attached. The default is *false*.

**--ip**=""
   Sets the container's interface IPv4 address (e.g. 172.23.0.9)

   It can only be used in conjunction with **--net** for user-defined networks

**--ip6**=""
   Sets the container's interface IPv6 address (e.g. 2001:db8::1b99)

   It can only be used in conjunction with **--net** for user-defined networks

**--ipc**=""
   Default is to create a private IPC namespace (POSIX SysV IPC) for the container
                               'container:<name|id>': reuses another container shared memory, semaphores and message queues
                               'host': use the host shared memory,semaphores and message queues inside the container.  Note: the host mode gives the container full access to local shared memory and is therefore considered insecure.

**--isolation**="*default*"
   Isolation specifies the type of isolation technology used by containers. 

**--kernel-memory**=""
   Kernel memory limit (format: `<number>[<unit>]`, where unit = b, k, m or g)

   Constrains the kernel memory available to a container. If a limit of 0
is specified (not using `--kernel-memory`), the container's kernel memory
is not limited. If you specify a limit, it may be rounded up to a multiple
of the operating system's page size and the value can be very large,
millions of trillions.

**-l**, **--label**=[]
   Adds metadata to a container (e.g., --label=com.example.key=value)

**--label-file**=[]
   Read labels from a file. Delimit each label with an EOL.

**--link**=[]
   Add link to another container in the form of <name or id>:alias or just
   <name or id> in which case the alias will match the name.

**--log-driver**="*json-file*|*syslog*|*journald*|*gelf*|*fluentd*|*awslogs*|*splunk*|*etwlogs*|*gcplogs*|*none*"
  Logging driver for container. Default is defined by daemon `--log-driver` flag.
  **Warning**: the `docker logs` command works only for the `json-file` and
  `journald` logging drivers.

**--log-opt**=[]
  Logging driver specific options.

**-m**, **--memory**=""
   Memory limit (format: <number>[<unit>], where unit = b, k, m or g)

   Allows you to constrain the memory available to a container. If the host
supports swap memory, then the **-m** memory setting can be larger than physical
RAM. If a limit of 0 is specified (not using **-m**), the container's memory is
not limited. The actual limit may be rounded up to a multiple of the operating
system's page size (the value would be very large, that's millions of trillions).

**--mac-address**=""
   Container MAC address (e.g. 92:d0:c6:0a:29:33)

**--memory-reservation**=""
   Memory soft limit (format: <number>[<unit>], where unit = b, k, m or g)

   After setting memory reservation, when the system detects memory contention
or low memory, containers are forced to restrict their consumption to their
reservation. So you should always set the value below **--memory**, otherwise the
hard limit will take precedence. By default, memory reservation will be the same
as memory limit.

**--memory-swap**="LIMIT"
   A limit value equal to memory plus swap. Must be used with the  **-m**
(**--memory**) flag. The swap `LIMIT` should always be larger than **-m**
(**--memory**) value.

   The format of `LIMIT` is `<number>[<unit>]`. Unit can be `b` (bytes),
`k` (kilobytes), `m` (megabytes), or `g` (gigabytes). If you don't specify a
unit, `b` is used. Set LIMIT to `-1` to enable unlimited swap.

**--memory-swappiness**=""
   Tune a container's memory swappiness behavior. Accepts an integer between 0 and 100.

**--name**=""
   Assign a name to the container

**--net**="*bridge*"
   Set the Network mode for the container
                               'bridge': create a network stack on the default Docker bridge
                               'none': no networking
                               'container:<name|id>': reuse another container's network stack
                               'host': use the Docker host network stack.  Note: the host mode gives the container full access to local system services such as D-bus and is therefore considered insecure.
                               '<network-name>|<network-id>': connect to a user-defined network

**--net-alias**=[]
   Add network-scoped alias for the container

**--oom-kill-disable**=*true*|*false*
	Whether to disable OOM Killer for the container or not.

**--oom-score-adj**=""
    Tune the host's OOM preferences for containers (accepts -1000 to 1000)

**-P**, **--publish-all**=*true*|*false*
   Publish all exposed ports to random ports on the host interfaces. The default is *false*.

**-p**, **--publish**=[]
   Publish a container's port, or a range of ports, to the host
                               format: ip:hostPort:containerPort | ip::containerPort | hostPort:containerPort | containerPort
                               Both hostPort and containerPort can be specified as a range of ports. 
                               When specifying ranges for both, the number of container ports in the range must match the number of host ports in the range. (e.g., `-p 1234-1236:1234-1236/tcp`)
                               (use 'docker port' to see the actual mapping)

**--pid**=*host*
   Set the PID mode for the container
     **host**: use the host's PID namespace inside the container.
     Note: the host mode gives the container full access to local PID and is therefore considered insecure.

**--pids-limit**=""
   Tune the container's pids limit. Set `-1` to have unlimited pids for the container.

**--privileged**=*true*|*false*
   Give extended privileges to this container. The default is *false*.

**--read-only**=*true*|*false*
   Mount the container's root filesystem as read only.

**--restart**="*no*"
   Restart policy to apply when a container exits (no, on-failure[:max-retry], always, unless-stopped).

**--shm-size**=""
   Size of `/dev/shm`. The format is `<number><unit>`. `number` must be greater than `0`.
   Unit is optional and can be `b` (bytes), `k` (kilobytes), `m` (megabytes), or `g` (gigabytes). If you omit the unit, the system uses bytes.
   If you omit the size entirely, the system uses `64m`.

**--security-opt**=[]
   Security Options

**--stop-signal**=*SIGTERM*
  Signal to stop a container. Default is SIGTERM.

**-t**, **--tty**=*true*|*false*
   Allocate a pseudo-TTY. The default is *false*.

**--tmpfs**=[] Create a tmpfs mount

   Mount a temporary filesystem (`tmpfs`) mount into a container, for example:

   $ docker run -d --tmpfs /tmp:rw,size=787448k,mode=1777 my_image

   This command mounts a `tmpfs` at `/tmp` within the container.  The supported mount
options are the same as the Linux default `mount` flags. If you do not specify
any options, the systems uses the following options:
`rw,noexec,nosuid,nodev,size=65536k`.

**-u**, **--user**=""
   Username or UID

**--ulimit**=[]
   Ulimit options

**--uts**=*host*
   Set the UTS mode for the container
     **host**: use the host's UTS namespace inside the container.
     Note: the host mode gives the container access to changing the host's hostname and is therefore considered insecure.

**-v**|**--volume**[=*[[HOST-DIR:]CONTAINER-DIR[:OPTIONS]]*]
   Create a bind mount. If you specify, ` -v /HOST-DIR:/CONTAINER-DIR`, Docker
   bind mounts `/HOST-DIR` in the host to `/CONTAINER-DIR` in the Docker
   container. If 'HOST-DIR' is omitted,  Docker automatically creates the new
   volume on the host.  The `OPTIONS` are a comma delimited list and can be:

   * [rw|ro]
   * [z|Z]
   * [`[r]shared`|`[r]slave`|`[r]private`]

The `CONTAINER-DIR` must be an absolute path such as `/src/docs`. The `HOST-DIR`
can be an absolute path or a `name` value. A `name` value must start with an
alphanumeric character, followed by `a-z0-9`, `_` (underscore), `.` (period) or
`-` (hyphen). An absolute path starts with a `/` (forward slash).

If you supply a `HOST-DIR` that is an absolute path,  Docker bind-mounts to the
path you specify. If you supply a `name`, Docker creates a named volume by that
`name`. For example, you can specify either `/foo` or `foo` for a `HOST-DIR`
value. If you supply the `/foo` value, Docker creates a bind-mount. If you
supply the `foo` specification, Docker creates a named volume.

You can specify multiple  **-v** options to mount one or more mounts to a
container. To use these same mounts in other containers, specify the
**--volumes-from** option also.

You can add `:ro` or `:rw` suffix to a volume to mount it  read-only or
read-write mode, respectively. By default, the volumes are mounted read-write.
See examples.

Labeling systems like SELinux require that proper labels are placed on volume
content mounted into a container. Without a label, the security system might
prevent the processes running inside the container from using the content. By
default, Docker does not change the labels set by the OS.

To change a label in the container context, you can add either of two suffixes
`:z` or `:Z` to the volume mount. These suffixes tell Docker to relabel file
objects on the shared volumes. The `z` option tells Docker that two containers
share the volume content. As a result, Docker labels the content with a shared
content label. Shared volume labels allow all containers to read/write content.
The `Z` option tells Docker to label the content with a private unshared label.
Only the current container can use a private volume.

By default bind mounted volumes are `private`. That means any mounts done
inside container will not be visible on host and vice-a-versa. One can change
this behavior by specifying a volume mount propagation property. Making a
volume `shared` mounts done under that volume inside container will be
visible on host and vice-a-versa. Making a volume `slave` enables only one
way mount propagation and that is mounts done on host under that volume
will be visible inside container but not the other way around.

To control mount propagation property of volume one can use `:[r]shared`,
`:[r]slave` or `:[r]private` propagation flag. Propagation property can
be specified only for bind mounted volumes and not for internal volumes or
named volumes. For mount propagation to work source mount point (mount point
where source dir is mounted on) has to have right propagation properties. For
shared volumes, source mount point has to be shared. And for slave volumes,
source mount has to be either shared or slave.

Use `df <source-dir>` to figure out the source mount and then use
`findmnt -o TARGET,PROPAGATION <source-mount-dir>` to figure out propagation
properties of source mount. If `findmnt` utility is not available, then one
can look at mount entry for source mount point in `/proc/self/mountinfo`. Look
at `optional fields` and see if any propagaion properties are specified.
`shared:X` means mount is `shared`, `master:X` means mount is `slave` and if
nothing is there that means mount is `private`.

To change propagation properties of a mount point use `mount` command. For
example, if one wants to bind mount source directory `/foo` one can do
`mount --bind /foo /foo` and `mount --make-private --make-shared /foo`. This
will convert /foo into a `shared` mount point. Alternatively one can directly
change propagation properties of source mount. Say `/` is source mount for
`/foo`, then use `mount --make-shared /` to convert `/` into a `shared` mount.

> **Note**:
> When using systemd to manage the Docker daemon's start and stop, in the systemd
> unit file there is an option to control mount propagation for the Docker daemon
> itself, called `MountFlags`. The value of this setting may cause Docker to not
> see mount propagation changes made on the mount point. For example, if this value
> is `slave`, you may not be able to use the `shared` or `rshared` propagation on
> a volume.

**--volume-driver**=""
   Container's volume driver. This driver creates volumes specified either from
   a Dockerfile's `VOLUME` instruction or from the `docker run -v` flag.
   See **docker-volume-create(1)** for full details.

**--volumes-from**=[]
   Mount volumes from the specified container(s)

**-w**, **--workdir**=""
   Working directory inside the container

# EXAMPLES

## Specify isolation technology for container (--isolation)

This option is useful in situations where you are running Docker containers on
Windows. The `--isolation=<value>` option sets a container's isolation
technology. On Linux, the only supported is the `default` option which uses
Linux namespaces. On Microsoft Windows, you can specify these values:

* `default`: Use the value specified by the Docker daemon's `--exec-opt` . If the `daemon` does not specify an isolation technology, Microsoft Windows uses `process` as its default value.
* `process`: Namespace isolation only.
* `hyperv`: Hyper-V hypervisor partition-based isolation.

Specifying the `--isolation` flag without a value is the same as setting `--isolation="default"`.

# HISTORY
August 2014, updated by Sven Dowideit <SvenDowideit@home.org.au>
September 2014, updated by Sven Dowideit <SvenDowideit@home.org.au>
November 2014, updated by Sven Dowideit <SvenDowideit@home.org.au>
