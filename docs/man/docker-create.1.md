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
[**-c**|**--cpu-shares**[=*0*]]
[**--cap-add**[=*[]*]]
[**--cap-drop**[=*[]*]]
[**--cidfile**[=*CIDFILE*]]
[**--cpu-period**[=*0*]]
[**--cpuset-cpus**[=*CPUSET-CPUS*]]
[**--cpuset-mems**[=*CPUSET-MEMS*]]
[**--cpu-quota**[=*0*]]
[**--device**[=*[]*]]
[**--dns-search**[=*[]*]]
[**--dns**[=*[]*]]
[**-e**|**--env**[=*[]*]]
[**--entrypoint**[=*ENTRYPOINT*]]
[**--env-file**[=*[]*]]
[**--expose**[=*[]*]]
[**-h**|**--hostname**[=*HOSTNAME*]]
[**--help**]
[**-i**|**--interactive**[=*false*]]
[**--ipc**[=*IPC*]]
[**-l**|**--label**[=*[]*]]
[**--label-file**[=*[]*]]
[**--link**[=*[]*]]
[**--lxc-conf**[=*[]*]]
[**--log-driver**[=*[]*]]
[**--log-opt**[=*[]*]]
[**-m**|**--memory**[=*MEMORY*]]
[**--memory-swap**[=*MEMORY-SWAP*]]
[**--mac-address**[=*MAC-ADDRESS*]]
[**--name**[=*NAME*]]
[**--net**[=*"bridge"*]]
[**--oom-kill-disable**[=*false*]]
[**-P**|**--publish-all**[=*false*]]
[**-p**|**--publish**[=*[]*]]
[**--pid**[=*[]*]]
[**--uts**[=*[]*]]
[**--privileged**[=*false*]]
[**--read-only**[=*false*]]
[**--restart**[=*RESTART*]]
[**--security-opt**[=*[]*]]
[**--mem-swappiness**[=*60*]]
[**-t**|**--tty**[=*false*]]
[**-u**|**--user**[=*USER*]]
[**-v**|**--volume**[=*[]*]]
[**--volumes-from**[=*[]*]]
[**-w**|**--workdir**[=*WORKDIR*]]
[**--cgroup-parent**[=*CGROUP-PATH*]]
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

**--blkio-weight**=0
   Block IO weight (relative weight) accepts a weight value between 10 and 1000.

**-c**, **--cpu-shares**=0
   CPU shares (relative weight)

**--cap-add**=[]
   Add Linux capabilities

**--cap-drop**=[]
   Drop Linux capabilities

**--cidfile**=""
   Write the container ID to the file

**--cgroup-parent**=""
   Path to cgroups under which the cgroup for the container will be created. If the path is not absolute, the path is considered to be relative to the cgroups path of the init process. Cgroups will be created if they do not already exist.

**--cpu-peroid**=0
    Limit the CPU CFS (Completely Fair Scheduler) period

**--cpuset-cpus**=""
   CPUs in which to allow execution (0-3, 0,1)

**--cpuset-mems**=""
   Memory nodes (MEMs) in which to allow execution (0-3, 0,1). Only effective on NUMA systems.

   If you have four memory nodes on your system (0-3), use `--cpuset-mems=0,1`
then processes in your Docker container will only use memory from the first
two memory nodes.

**-cpu-quota**=0
   Limit the CPU CFS (Completely Fair Scheduler) quota

**--device**=[]
   Add a host device to the container (e.g. --device=/dev/sdc:/dev/xvdc:rwm)

**--dns-search**=[]
   Set custom DNS search domains (Use --dns-search=. if you don't wish to set the search domain)

**--dns**=[]
   Set custom DNS servers

**-e**, **--env**=[]
   Set environment variables

**--entrypoint**=""
   Overwrite the default ENTRYPOINT of the image

**--env-file**=[]
   Read in a line delimited file of environment variables

**--expose**=[]
   Expose a port or a range of ports (e.g. --expose=3300-3310) from the container without publishing it to your host

**-h**, **--hostname**=""
   Container host name

**--help**
  Print usage statement

**-i**, **--interactive**=*true*|*false*
   Keep STDIN open even if not attached. The default is *false*.

**--ipc**=""
   Default is to create a private IPC namespace (POSIX SysV IPC) for the container
                               'container:<name|id>': reuses another container shared memory, semaphores and message queues
                               'host': use the host shared memory,semaphores and message queues inside the container.  Note: the host mode gives the container full access to local shared memory and is therefore considered insecure.

**-l**, **--label**=[]
   Adds metadata to a container (e.g., --label=com.example.key=value)

**--label-file**=[]
   Read labels from a file. Delimit each label with an EOL.

**--link**=[]
   Add link to another container in the form of <name or id>:alias or just
   <name or id> in which case the alias will match the name.

**--lxc-conf**=[]
   (lxc exec-driver only) Add custom lxc options --lxc-conf="lxc.cgroup.cpuset.cpus = 0,1"

**--log-driver**="|*json-file*|*syslog*|*journald*|*none*"
  Logging driver for container. Default is defined by daemon `--log-driver` flag.
  **Warning**: `docker logs` command works only for `json-file` logging driver.

**--log-opt**=[]
  Logging driver specific options.

**-m**, **--memory**=""
   Memory limit (format: <number><optional unit>, where unit = b, k, m or g)

   Allows you to constrain the memory available to a container. If the host
supports swap memory, then the **-m** memory setting can be larger than physical
RAM. If a limit of 0 is specified (not using **-m**), the container's memory is
not limited. The actual limit may be rounded up to a multiple of the operating
system's page size (the value would be very large, that's millions of trillions).

**--memory-swap**=""
   Total memory limit (memory + swap)

   Set `-1` to disable swap (format: <number><optional unit>, where unit = b, k, m or g).
This value should always larger than **-m**, so you should always use this with **-m**.

**--mac-address**=""
   Container MAC address (e.g. 92:d0:c6:0a:29:33)

**--name**=""
   Assign a name to the container

**--net**="bridge"
   Set the Network mode for the container
                               'bridge': creates a new network stack for the container on the docker bridge
                               'none': no networking for this container
                               'container:<name|id>': reuses another container network stack
                               'host': use the host network stack inside the container.  Note: the host mode gives the container full access to local system services such as D-bus and is therefore considered insecure.

**--oom-kill-disable**=*true*|*false*
	Whether to disable OOM Killer for the container or not.

**-P**, **--publish-all**=*true*|*false*
   Publish all exposed ports to random ports on the host interfaces. The default is *false*.

**-p**, **--publish**=[]
   Publish a container's port, or a range of ports, to the host
                               format: ip:hostPort:containerPort | ip::containerPort | hostPort:containerPort | containerPort
                               Both hostPort and containerPort can be specified as a range of ports. 
                               When specifying ranges for both, the number of container ports in the range must match the number of host ports in the range. (e.g., `-p 1234-1236:1234-1236/tcp`)
                               (use 'docker port' to see the actual mapping)

**--pid**=host
   Set the PID mode for the container
     **host**: use the host's PID namespace inside the container.
     Note: the host mode gives the container full access to local PID and is therefore considered insecure.

**--uts**=host
   Set the UTS mode for the container
     **host**: use the host's UTS namespace inside the container.
     Note: the host mode gives the container access to changing the host's hostname and is therefore considered insecure.

**--privileged**=*true*|*false*
   Give extended privileges to this container. The default is *false*.

**--read-only**=*true*|*false*
   Mount the container's root filesystem as read only.

**--restart**="no"
   Restart policy to apply when a container exits (no, on-failure[:max-retry], always)

**--security-opt**=[]
   Security Options

**--mem-swappiness**=*60*
   Tuning the memory swappiness of container. The value in the range of 0-100 is accepted.

**-t**, **--tty**=*true*|*false*
   Allocate a pseudo-TTY. The default is *false*.

**-u**, **--user**=""
   Username or UID

**-v**, **--volume**=[]
   Bind mount a volume (e.g., from the host: -v /host:/container, from Docker: -v /container)

**--volumes-from**=[]
   Mount volumes from the specified container(s)

**-w**, **--workdir**=""
   Working directory inside the container

# HISTORY
August 2014, updated by Sven Dowideit <SvenDowideit@home.org.au>
September 2014, updated by Sven Dowideit <SvenDowideit@home.org.au>
November 2014, updated by Sven Dowideit <SvenDowideit@home.org.au>
