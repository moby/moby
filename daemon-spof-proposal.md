**Background**:
===============================
As per the current architecture of Docker, all containers are spawned as children of docker daemon. This parent-child relationship between the docker daemon and containers provides a straight forward way of signalling/communication between them. However, this tight coupling between the containers and daemon results in some issues which are critical for containers' up-time, stability and high availability. Few of the issues with this approach are:

1. Daemon's death (restart, kill, abrupt crash in daemon's code) causes all the running containers to die as well.
2. An upgrade to daemon can not be performed without impacting the up-time/stability of containers. 

Both of these issues become even more important in production environments such as containers cloud where different containers running on a server might belong to same or different clients and might host highly available or stateless services. In these scenarios, a container downtime caused by external factors such as daemon's death/upgrade, is highly undesirable.  

The 2nd issue was opened by @shykes  in 2013, but this is still an open issue https://github.com/docker/docker/issues/2658 


**Goals:**
========================
1. Containers should run independently of docker daemon and should continue to function normally even if docker daemon dies/restarts
2. When containers die/exit their exit status should be appropriately communicated to docker daemon (whenever it restarts)
3. All commands that are used to interact with containers (start/stop/kill/exec/pause, etc) and I/O (stderr/stdout) redirection should work normally

**Findings:**
=============================
Based on our investigation and experimentation with docker we found that once started, a container can function stand-alone and does not require daemon's presence for the execution of an encapsulated service. 

We changed the daemon's code such that upon it's death, containers would become orphaned and shall be adopted by INIT.  We ran the official mysql image. We were able to connect to, and use the mysql service even when the daemon was not running and container had become orphaned. 

The namespaces (pid,network, ipc, mnt, uts) and cgroups etc, which are the building blocks for container creation and execution, continue to exist and function normally as they are provided by the linux kernel. Therefore, there does not seem to be any reason for a container to stop functioning when the daemon is not present.

**Proposal:**
====================
In context of our findings and our goals, we propose two alternative design models through which containers will no longer be tightly coupled with the daemon either always (*proposal A*) or after daemon's death (*proposal B*). Both of these models require an external communication/signalling mechanism between the daemon and the containers. 
 
* **Proposal A**:  Docker daemon starts the containers in such a way that they are the children of INIT from the very beginning and are decoupled from the daemon.  All the running containers and daemon shall be siblings in process tree and will not be impacted by daemon's absence.  

* **Proposal B**:  A hybrid model  where newly spawned containers shall be the children of docker daemon as in the current design. However, upon abrupt daemon failure or upgrade, all the running containers shall become orphaned and will move to INIT.  When the daemon restarts, it will detect that there are old running containers (current code detects and kills such containers) and would manage them by external communication means (new feature to be added). Consequently, all the containers that are spawned by the current instance of daemon will be its decedents and other containers that were started by previous daemon instances will be independent processes and would require external mode of communication. 

**Communication between containers and the daemon:**
-----------------------
Both of these proposals require some sort of two-way communication mechanism between the daemon and the containers. For example, how would a daemon get notified when a container finishes its execution? Also, how would the daemon pass the commands to containers. In current design, daemon does "wait" on child process (container) in a go routine. This can be tackled by having a dedicated parent monitor process for each container, whose job will be to wait on the container and communicate with the daemon.  Below describes the communication: 

* **From a container to daemon**: Once the container terminates, waiting monitor parent process can communicate the exit status to the docker daemon either by connecting through sockets or by sending special signals to the daemon process (SIGRTMIN-SIGRTMAX, SIGUSR1,SIGUSR2) and writing to a special file from where daemon can read the status upon receiving a special signal from the container. 
* **From daemon to a container**: Similarly, daemon can be made aware of container's pid by monitor process so that it can directly interact with the containers using signals (SIGKILL,SIGTERM,SIGSTP, SIGCONT etc).
 
* **For I/O**: Interprocess pipes can be used to redirect stdout, stderr etc. 

 
**Implementation:**
=======================================
**Proposal A**

* Daemon first spawns a monitor process and that monitor process spawns the container. Once the container is fully spawned, monitor process replaces itself with an *ultra lightweight code* (unix exec) whose job is to wait on its child process (container). 

* Daemon then daemonise the monitor process so that *monitor+container* become orphan and are adopted by INIT. 

* Daemon interacts with the monitor process using sockets or send direct signals to container process (monitor can communicate the container pid to daemon) for management (stop, start, pause, kill, etc).


* Once the container finishes its execution, the waiting monitor process can communicate the exit status to the daemon. There are two possibilities here 
    * Daemon is alive:- communication is straight forward and the monitoring process can exit after supplying the the status to the daemon 
    * Daemon is dead:- Monitoring process will wait until the daemon is up, communicate the status and exit. Alternatively it can write to a file, which the new instance of daemon when it would come back up, will read from. 
![screen shot 2015-06-11 at 3 46 06 pm](https://cloud.githubusercontent.com/assets/4323340/8104805/5c528a7e-1051-11e5-9ba3-e33b988f5f5c.png)

**Proposal B**

*  Monitor+container move to INIT only after the daemon dies (becomes orphaned). This would require a change in cleanup code. 

* New instance of daemon becomes aware of old running containers. And communicate to them using the techniques described in proposal A. 

* Monitor process can look for its ppid (parent pid) and if it is 1, it can assume that it is orphaned and must communicate with daemon by other means. If not, it would have daemon as it's parent and communication flow be similar to what we have in current design. 

![screen shot 2015-06-11 at 3 46 45 pm](https://cloud.githubusercontent.com/assets/4323340/8104987/abddc2d8-1052-11e5-98b1-956268d70ee0.png)


**Proposal A vs Proposal B**

* *Proposal A* requires daemonising the monitor just after it is spawned. (Daemonising a child process is not inherently supported by golang which docker uses but can be worked around by using double fork).
https://github.com/golang/go/issues/227

* *Proposal B* does not require any such functionality, and upon daemon's death, all the child monitors (orphaned now) are moved to INIT along with the containers. However,  
    * Proposal B is easier to code although slightly less cleaner from a design perspective as it would require separate maintenance (different codes paths/checks) for the container that daemon owns (descendants) and the containers that it manages (siblings)


**Use Cases:**
=============================
* Containers' high availability
* Hot upgrades
* Scalability
    * Scalability is possible because a particular daemon will not be the owner of containers. Therefore, multiple daemons can be allowed to run on same host doing load balancing of requests coming from different channels (CLI, remote API etc).
    * However, we don't know if docker daemon is a performance bottleneck (most likely not). Daemon seems just to be a control channel/management point and does not picture in data/critical path. 
    * Nonetheless, one can have a redundant docker daemon sharing the same DB, so that even upon upgrades or failures, daemon continue to serve requests - **Daemon's high availability**


Commands' analysis
===============================
Following table lists all the commands that will be (not) impacted and would require a code change. 

|Docker command |	Description  |	Impact |
-------------------------|------------------|-------------
exec	 | run a new command in a running container	| impacted, doable (using setns)
Attach | attach to a running container |	Impacted, doable (using pipes or by connecting to stdout of the container by other means)
build	 | build an image from docker file | Not impacted, unrelated
commit  |	create a new images from a container's changes | Not impacted, related (read file system)
cp |	copy files from a location to another  |	not impacted, related (read file system)
diff |	inspect changes on a container's file system	| not impacted, related (read file system)
events  |	global daemon events	| not impacted, unrelated
export/save |	create a  tar archive | 	not impacted, unrelated
history |	history of an image	| not impacted, unrelated
images |	list images  |	not impacted, unrelated
import/load | tar to image	 | not impacted, unrelated
info	| daemon version build etc	| not impacted, unrelated
inspect	| return container's json	| not impacted, related (read json from file system)
kill	| kill the running container	| impacted, doable (need to pass the signal)
logs	| fetch the logs of a container	| impacted, doable (reading from a special filefd/pipe)
port	| shows public facing port-> NATed to private port	| not impacted, related
pause | Pause all running processes in a container   |	impacted, doable (need to send SIGSTOP)
start/unpause	| start all stopped processes in a container |	impacted, doable (need to send SIGCONT)
ps | list all containers  |	not impacted, unrelated
pull/push	| pull and push images from/to repo |	not impacted, unrelated
restart |	restart a running container	| impacted, doable (kill the old container, start afresh)
rm	| remove the container	| impacted, doable
rmi	| remove the image	| not impacted, unrelated
run	| run a command in a new container | impacted, doable (start a new container)
search |	search repo |	not impacted, unrelated
stop	| stop a running container  |	impacted, doable (need to send SIGTERM/KILL)
tag	| tag an image	| not impacted, unrelated
top	| lookup the running processes of a container	 | impacted, doable (using setns + top) 
version	| docker version info |	not impacted, unrelated
wait	| block until a container stops and print exit code	| impacted, doable using our external communication mode


Limitations:
===============================
We will have the overhead of extra monitor processes (as many as the number of running containers). 





