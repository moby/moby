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
   "mounts" : [
      {
         "type" : "devtmpfs"
      }
   ],
   "tty" : true,
   "environment" : [
      "HOME=/",
      "PATH=PATH=$PATH:/bin:/usr/bin:/sbin:/usr/sbin",
      "container=docker",
      "TERM=xterm-256color"
   ],
   "hostname" : "koye",
   "cgroups" : {
      "parent" : "docker",
      "name" : "docker-koye"
   },
   "capabilities_mask" : [
      {
         "value" : 8,
         "key" : "SETPCAP",
         "enabled" : false
      },
      {
         "enabled" : false,
         "value" : 16,
         "key" : "SYS_MODULE"
      },
      {
         "value" : 17,
         "key" : "SYS_RAWIO",
         "enabled" : false
      },
      {
         "key" : "SYS_PACCT",
         "value" : 20,
         "enabled" : false
      },
      {
         "value" : 21,
         "key" : "SYS_ADMIN",
         "enabled" : false
      },
      {
         "value" : 23,
         "key" : "SYS_NICE",
         "enabled" : false
      },
      {
         "value" : 24,
         "key" : "SYS_RESOURCE",
         "enabled" : false
      },
      {
         "key" : "SYS_TIME",
         "value" : 25,
         "enabled" : false
      },
      {
         "enabled" : false,
         "value" : 26,
         "key" : "SYS_TTY_CONFIG"
      },
      {
         "key" : "AUDIT_WRITE",
         "value" : 29,
         "enabled" : false
      },
      {
         "value" : 30,
         "key" : "AUDIT_CONTROL",
         "enabled" : false
      },
      {
         "enabled" : false,
         "key" : "MAC_OVERRIDE",
         "value" : 32
      },
      {
         "enabled" : false,
         "key" : "MAC_ADMIN",
         "value" : 33
      },
      {
         "key" : "NET_ADMIN",
         "value" : 12,
         "enabled" : false
      },
      {
         "value" : 27,
         "key" : "MKNOD",
         "enabled" : true
      }
   ],
   "networks" : [
      {
         "mtu" : 1500,
         "address" : "127.0.0.1/0",
         "type" : "loopback",
         "gateway" : "localhost"
      },
      {
         "mtu" : 1500,
         "address" : "172.17.42.2/16",
         "type" : "veth",
         "context" : {
            "bridge" : "docker0",
            "prefix" : "veth"
         },
         "gateway" : "172.17.42.1"
      }
   ],
   "namespaces" : [
      {
         "key" : "NEWNS",
         "value" : 131072,
         "enabled" : true,
         "file" : "mnt"
      },
      {
         "key" : "NEWUTS",
         "value" : 67108864,
         "enabled" : true,
         "file" : "uts"
      },
      {
         "enabled" : true,
         "file" : "ipc",
         "key" : "NEWIPC",
         "value" : 134217728
      },
      {
         "file" : "pid",
         "enabled" : true,
         "value" : 536870912,
         "key" : "NEWPID"
      },
      {
         "enabled" : true,
         "file" : "net",
         "key" : "NEWNET",
         "value" : 1073741824
      }
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
