% DOCKER(1) Docker User Manuals
% Docker Community
% JUNE 2014
# NAME
docker-create - Create a new container

# SYNOPSIS
**docker create**
[**-a**|**--attach**[=*[]*]]
[**--add-host**[=*[]*]]
[**-c**|**--cpu-shares**[=*0*]]
[**--cap-add**[=*[]*]]
[**--cap-drop**[=*[]*]]
[**--cidfile**[=*CIDFILE*]]
[**--cpuset**[=*CPUSET*]]
[**--device**[=*[]*]]
[**--dns-search**[=*[]*]]
[**--dns**[=*[]*]]
[**-e**|**--env**[=*[]*]]
[**--entrypoint**[=*ENTRYPOINT*]]
[**--env-file**[=*[]*]]
[**--expose**[=*[]*]]
[**-h**|**--hostname**[=*HOSTNAME*]]
[**-i**|**--interactive**[=*false*]]
[**--ipc**[=*IPC*]]
[**--link**[=*[]*]]
[**--lxc-conf**[=*[]*]]
[**-m**|**--memory**[=*MEMORY*]]
[**--mac-address**[=*MAC-ADDRESS*]]
[**--name**[=*NAME*]]
[**--net**[=*"bridge"*]]
[**-P**|**--publish-all**[=*false*]]
[**-p**|**--publish**[=*[]*]]
[**--privileged**[=*false*]]
[**--restart**[=*RESTART*]]
[**--security-opt**[=*[]*]]
[**-t**|**--tty**[=*false*]]
[**-u**|**--user**[=*USER*]]
[**-v**|**--volume**[=*[]*]]
[**--volumes-from**[=*[]*]]
[**-w**|**--workdir**[=*WORKDIR*]]
IMAGE [COMMAND] [ARG...]

# OPTIONS
**-a**, **--attach**=[]
   Attach to STDIN, STDOUT or STDERR.

**--add-host**=[]
   Add a custom host-to-IP mapping (host:ip)

**-c**, **--cpu-shares**=0
   CPU shares (relative weight)

**--cap-add**=[]
   Add Linux capabilities

**--cap-drop**=[]
   Drop Linux capabilities

**--cidfile**=""
   Write the container ID to the file

**--cpuset**=""
   CPUs in which to allow execution (0-3, 0,1)

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

**-i**, **--interactive**=*true*|*false*
   Keep STDIN open even if not attached. The default is *false*.

**--ipc**=""
   Default is to create a private IPC namespace (POSIX SysV IPC) for the container
                               'container:<name|id>': reuses another container shared memory, semaphores and message queues
                               'host': use the host shared memory,semaphores and message queues inside the container.  Note: the host mode gives the container full access to local shared memory and is therefore considered insecure.

**--link**=[]
   Add link to another container in the form of name:alias

**--lxc-conf**=[]
   (lxc exec-driver only) Add custom lxc options --lxc-conf="lxc.cgroup.cpuset.cpus = 0,1"

**-m**, **--memory**=""
   Memory limit (format: <number><optional unit>, where unit = b, k, m or g)

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

**-P**, **--publish-all**=*true*|*false*
   Publish all exposed ports to the host interfaces. The default is *false*.

**-p**, **--publish**=[]
   Publish a container's port to the host
                               format: ip:hostPort:containerPort | ip::containerPort | hostPort:containerPort | containerPort
                               (use 'docker port' to see the actual mapping)

**--privileged**=*true*|*false*
   Give extended privileges to this container. The default is *false*.

**--restart**=""
   Restart policy to apply when a container exits (no, on-failure[:max-retry], always)

**--security-opt**=[]
   Security Options

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
