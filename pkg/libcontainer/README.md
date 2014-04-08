## libcontainer - reference implementation for containers

#### background

libcontainer specifies configuration options for what a container is.  It provides a native Go implementation 
for using Linux namespaces with no external dependencies.  libcontainer provides many convenience functions for working with namespaces, networking, and management.  


#### container
A container is a self contained directory that is able to run one or more processes without 
affecting the host system.  The directory is usually a full system tree.  Inside the directory
a `container.json` file is placed with the runtime configuration for how the processes 
should be contained and ran.  Environment, networking, and different capabilities for the 
process are specified in this file.  The configuration is used for each process executed inside the container.

Sample `container.json` file:
```json
{
   "hostname" : "koye",
   "networks" : [
      {
         "gateway" : "172.17.42.1",
         "context" : {
            "bridge" : "docker0",
            "prefix" : "veth"
         },
         "address" : "172.17.0.2/16",
         "type" : "veth",
         "mtu" : 1500
      }
   ],
   "cgroups" : {
      "parent" : "docker",
      "name" : "11bb30683fb0bdd57fab4d3a8238877f1e4395a2cfc7320ea359f7a02c1a5620"
   },
   "tty" : true,
   "environment" : [
      "HOME=/",
      "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
      "HOSTNAME=11bb30683fb0",
      "TERM=xterm"
   ],
   "capabilities_mask" : [
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
      "MAC_ADMIN",
      "NET_ADMIN"
   ],
   "context" : {
      "apparmor_profile" : "docker-default"
   },
   "mounts" : [
      {
         "source" : "/var/lib/docker/containers/11bb30683fb0bdd57fab4d3a8238877f1e4395a2cfc7320ea359f7a02c1a5620/resolv.conf",
         "writable" : false,
         "destination" : "/etc/resolv.conf",
         "private" : true
      },
      {
         "source" : "/var/lib/docker/containers/11bb30683fb0bdd57fab4d3a8238877f1e4395a2cfc7320ea359f7a02c1a5620/hostname",
         "writable" : false,
         "destination" : "/etc/hostname",
         "private" : true
      },
      {
         "source" : "/var/lib/docker/containers/11bb30683fb0bdd57fab4d3a8238877f1e4395a2cfc7320ea359f7a02c1a5620/hosts",
         "writable" : false,
         "destination" : "/etc/hosts",
         "private" : true
      }
   ],
   "namespaces" : [
      "NEWNS",
      "NEWUTS",
      "NEWIPC",
      "NEWPID",
      "NEWNET"
   ]
}
```

Using this configuration and the current directory holding the rootfs for a process, one can use libcontainer to exec the container. Running the life of the namespace, a `pid` file 
is written to the current directory with the pid of the namespaced process to the external world.  A client can use this pid to wait, kill, or perform other operation with the container.  If a user tries to run a new process inside an existing container with a live namespace, the namespace will be joined by the new process.


You may also specify an alternate root place where the `container.json` file is read and where the `pid` file will be saved.

#### nsinit

`nsinit` is a cli application used as the reference implementation of libcontainer.  It is able to 
spawn or join new containers giving the current directory.  To use `nsinit` cd into a Linux 
rootfs and copy a `container.json` file into the directory with your specified configuration.

To execute `/bin/bash` in the current directory as a container just run:
```bash
nsinit exec /bin/bash
```

If you wish to spawn another process inside the container while your current bash session is 
running just run the exact same command again to get another bash shell or change the command.  If the original process dies, PID 1, all other processes spawned inside the container will also be killed and the namespace will be removed. 

You can identify if a process is running in a container by looking to see if `pid` is in the root of the directory.   
