# Docker Cpuhotplug extention

Docker uses *cgroups* in order to achieve isolation and limits container resources.

However, the Docker cpuset.cpus is not automatically updated when a cpu is set offline and online again. The goal of this docker extention is to update the cpuset of the docker daemon and each Docker container.

A container is defined *restricted*, if it was started with the flag `--cpuset-cpus`, otherwise is called *unrestricted*. A restricted container mantains the initial cpuset restriction and it is update consequently.

In linux system the updated cpuset can be find in `/sys/fs/cgroup/cpuset/cpuset.cpus` and it holds the information or the currently online cpus.
The each cgroup and subcgroup has its own cpuset.cpus. *Cgroup parent* is a cgroup that contains another cgroup. Subcgroups can have only a subset or the entire subset of their cgroup parent cpuset.

The docker daemon has an additional option `--exec-opt native.cgroupdriver`. The default value is *cgroupdriver* and the name structure looks like these in the following example.
However, the cgroup manage could be handles also by *systemd* using this option. The name cgroup strcture is different from that used by the *cgroupdriver*.
For a first prototype we just consider the default option. Hence, the cgroup are managed by the *cgroupdriver*.

The default cgroup parent for docker is called *docker* and the its path is `/sys/fs/cgroup/cpuset/docker/cpuset.cpus`. Each container has a subfolder in `/sys/fs/cgroup/cpuset/docker/` that holds its own cpuset.

Additionally, a container can create another cgroup parent with the option `--cgroup-parent string`. The cgroup-parent can be an entire path and each folder of the path has its own cpuset.

Example
```sh
docker run -td s390x/ubuntu bash
docker run -td  --cgroup-parent level1/level2 s390x/ubuntu bash
```
The cgroup structure looks like:

```sh
                /sys/fs/cgroup/cpuset/
 DEFAULT                              cpuset.cpus
    ____________  / ___________              \
   |             /             |              \
   |    /docker                |                /level1
   |            cpuset.cpus    |                        cpuset.cpus
   |        /                  |                              \
   |       /                   |                               \
   |  /7e2..a12                |                             /level2
   |            cpuset.cpus    |                                    cpuset.cpus
   |                           |                                        \
   |___________________________|                                         \
                                                                        /2f9..a45
                                                                                  cpuset.cpus
```
