title
:   Container

description
:   Definitions of a container

keywords
:   containers, lxc, concepts, explanation, image, container

Container
=========

![image](images/docker-filesystems-busyboxrw.png)

Once you start a process in Docker from an image\_def, Docker fetches
the image and its parent\_image\_def, and repeats the process until it
reaches the base\_image\_def. Then the ufs\_def adds a read-write layer
on top. That read-write layer, plus the information about its
parent\_image\_def and some additional information like its unique id,
networking configuration, and resource limits is called a **container**.

Container State
---------------

Containers can change, and so they have state. A container may be
**running** or **exited**.

When a container is running, the idea of a "container" also includes a
tree of processes running on the CPU, isolated from the other processes
running on the host.

When the container is exited, the state of the file system and its exit
value is preserved. You can start, stop, and restart a container. The
processes restart from scratch (their memory state is **not** preserved
in a container), but the file system is just as it was when the
container was stopped.

You can promote a container to an image\_def with `docker commit`. Once
a container is an image, you can use it as a parent for new containers.

Container IDs
-------------

All containers are identified by a 64 hexadecimal digit string
(internally a 256bit value). To simplify their use, a short ID of the
first 12 characters can be used on the commandline. There is a small
possibility of short id collisions, so the docker server will always
return the long ID.
