# Docker Plugins

Docker plugins offer a command centered approach to configuration.  Each plugin allows default options to be set an 
initialization and at execution.  



## Commands

### Set
Set allows the user to set specific plugin options based on a key value format. `<plugin> <key>=<value>`.  

-------------

## Plugins

### native
The native driver provides the container execution and namespace control from libcontainer

#### Options
* net.join - set the network namespace to an existing container
* fs.readonly - set the containers root filesystem as readonly

### selinux
Manage the selinux profile, mount and process labels for processes

**Supported drivers**
* native
* lxc

#### Options
* enabled - enable selinux support in docker
* label.process - set the process label 
* label.mount - set the mount label

### apparmor
Manage the apparmor profiles used for processes

**Supported drivers**
* native
* lxc

#### Options
* profile - set the apparmor profile

### cgroups
Manage the cgroup resource limits for processes

**Supported drivers**
* native
* lxc

#### Options
* cpu_shares - set the cpu shares for a process
* memory - set the memory limit for a process
* memory_swap - set the swap limit for a process
* cpuset.cpus - set the cpu affinity for a process


### ns
Manage the underlying linux namespace configuration

**Supported drivers**
* native

#### Options
* enable - set the container configuration to enable this namespace at clone
* disable - set the container configuration to disable add a new namespace at clone

### cap
Manage linux capabilities for the cotnainer

**Supported drivers**
* native

#### Options
* enable - set the linux capability enabled for a container
* disable - set the linux capability disabled for a container

### devicemapper
Device mapper storage plugin for containers and images

#### Options
* basesize - set the base size for the devicemapper initialization
* loopbackdatasize - set the loopback file size 
* loopbackmetadatasize - set the loopback data meta file size

### bridge
The default docker0 bridge and veth networking option for containers

#### Options
* name - name of the bridge to create or use
* ip - ip and mask for the bridge
* icc.enabled - enable inter container communication across the bridge
* mac - set the mac address for the bridge at creation
* mtu - set the mtu value for the host pair of the veth interfaces
* port_range.begin - set the starting value for the port allocator 
* port_range.end - set the ending value for the port allocator
* ip_range.begin - set the beginning ip range value for the ip allocator 
* ip_range.end - set the ending ip range value for the ip allocator
* veth.gateway - set the gateway address of the veth a container
* veth.name - set the name of the veth inside a container
* veth.ip - set the ip address of the veth inside a container
* veth.mac - set hte mac address of the veth inside a container
* veth.mtu - set the mtu value of the veth inside a container

### lxc
The lxc driver provides execution services for running processes inside of a container

#### Options
* lxc.\* - set any value to be passed to the lxc template
