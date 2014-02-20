## libcontainer - reference implementation for containers

#### background

libcontainer specifies configuration options for what a container is.  It provides a native Go implementation 
for using linux namespaces with no external dependencies.  libcontainer provides many convience functions for working with namespaces, networking, and management.  


#### container
A container is a self contained directory that is able to run one or more processes inside without 
affecting the host system.  The directory is usually a full system tree.  Inside the directory
a `container.json` file just be placed with the runtime configuration for how the process 
should be contained and run.  Environment, networking, and different capabilities for the 
process are specified in this file.

Sample `container.json` file:
```json
{
    "hostname": "koye",
    "environment": [
        "HOME=/",
        "PATH=PATH=$PATH:/bin:/usr/bin:/sbin:/usr/sbin",
        "container=docker",
        "TERM=xterm-256color"
    ],
    "namespaces": [
        "NEWIPC",
        "NEWNS",
        "NEWPID",
        "NEWUTS",
        "NEWNET"
    ],
    "capabilities": [
        "SETPCAP",
        "SYS_MODULE",
        "SYS_RAWIO",
        "SYS_PACCT",
        "SYS_ADMIN",
        "SYS_NICE",
        "SYS_RESOURCE",
        "SYS_TIME",
        "SYS_TTY_CONFIG",
        "MKNOD",
        "AUDIT_WRITE",
        "AUDIT_CONTROL",
        "MAC_OVERRIDE",
        "MAC_ADMIN"
    ],
    "network": {
        "ip": "172.17.0.100/16",
        "gateway": "172.17.42.1",
        "bridge": "docker0",
        "mtu": 1500
    }
}
```

Using this configuration and the current directory holding the rootfs for a process to live, one can se libcontainer to exec the container. Running the life of the namespace a `.nspid` file 
is written to the current directory with the pid of the namespace'd process to the external word.  A client can use this pid to wait, kill, or perform other operation with the container.  If a user tries to run an new process inside an existing container with a live namespace with namespace will be joined by the new process.


#### nsinit

`nsinit` is a cli application used as the reference implementation of libcontainer.  It is able to 
spawn or join new containers giving the current directory.  To use `nsinit` cd into a linux 
rootfs and copy a `container.json` file into the directory with your specified configuration.

To execution `/bin/bash` in the current directory as a container just run:
```bash
nsinit exec /bin/bash
```

If you wish to spawn another process inside the container while your current bash session is 
running just run the exact same command again to get another bash shell or change the command.  If the original process dies, PID 1, all other processes spawned inside the container will also be killed and the namespace will be removed. 

You can identify if a process is running in a container by looking to see if `.nspid` is in the root of the directory.   
