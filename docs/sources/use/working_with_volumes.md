page_title: Share Directories via Volumes
page_description: How to create and share volumes
page_keywords: Examples, Usage, volume, docker, documentation, examples

# Share Directories via Volumes

## Introduction

A *data volume* is a specially-designated directory within one or more
containers that bypasses the [*Union File
System*](../../terms/layer/#ufs-def) to provide several useful features
for persistent or shared data:

- **Data volumes can be shared and reused between containers:**  
  This is the feature that makes data volumes so powerful. You can
  use it for anything from hot database upgrades to custom backup or
  replication tools. See the example below.
- **Changes to a data volume are made directly:**  
  Without the overhead of a copy-on-write mechanism. This is good for
  very large files.
- **Changes to a data volume will not be included at the next commit:**  
  Because they are not recorded as regular filesystem changes in the
  top layer of the [*Union File System*](../../terms/layer/#ufs-def)
- **Volumes persist until no containers use them:**  
  As they are a reference counted resource. The container does not need to be
  running to share its volumes, but running it can help protect it
  against accidental removal via `docker rm`.

Each container can have zero or more data volumes.

New in version v0.3.0.

## Getting Started

Using data volumes is as simple as adding a `-v`
parameter to the `docker run` command. The
`-v` parameter can be used more than once in order
to create more volumes within the new container. To create a new
container with two new volumes:

    $ docker run -v /var/volume1 -v /var/volume2 busybox true

This command will create the new container with two new volumes that
exits instantly (`true` is pretty much the smallest,
simplest program that you can run). Once created you can mount its
volumes in any other container using the `--volumes-from`
option; irrespective of whether the container is running or
not.

Or, you can use the VOLUME instruction in a Dockerfile to add one or
more new volumes to any container created from that image:

    # BUILD-USING:        docker build -t data .
    # RUN-USING:          docker run -name DATA data
    FROM          busybox
    VOLUME        ["/var/volume1", "/var/volume2"]
    CMD           ["/bin/true"]

### Creating and mounting a Data Volume Container

If you have some persistent data that you want to share between
containers, or want to use from non-persistent containers, its best to
create a named Data Volume Container, and then to mount the data from
it.

Create a named container with volumes to share (`/var/volume1`
and `/var/volume2`):

    $ docker run -v /var/volume1 -v /var/volume2 -name DATA busybox true

Then mount those data volumes into your application containers:

    $ docker run -t -i -rm -volumes-from DATA -name client1 ubuntu bash

You can use multiple `-volumes-from` parameters to
bring together multiple data volumes from multiple containers.

Interestingly, you can mount the volumes that came from the
`DATA` container in yet another container via the
`client1` middleman container:

    $ docker run -t -i -rm -volumes-from client1 -name client2 ubuntu bash

This allows you to abstract the actual data source from users of that
data, similar to
[*ambassador\_pattern\_linking*](../ambassador_pattern_linking/#ambassador-pattern-linking).

If you remove containers that mount volumes, including the initial DATA
container, or the middleman, the volumes will not be deleted until there
are no containers still referencing those volumes. This allows you to
upgrade, or effectively migrate data volumes between containers.

### Mount a Host Directory as a Container Volume:

    -v=[]: Create a bind mount with: [host-dir]:[container-dir]:[rw|ro].

You must specify an absolute path for `host-dir`. If
`host-dir` is missing from the command, then docker
creates a new volume. If `host-dir` is present but
points to a non-existent directory on the host, Docker will
automatically create this directory and use it as the source of the
bind-mount.

Note that this is not available from a Dockerfile due the portability
and sharing purpose of it. The `host-dir` volumes
are entirely host-dependent and might not work on any other machine.

For example:

    sudo docker run -t -i -v /var/logs:/var/host_logs:ro ubuntu bash

The command above mounts the host directory `/var/logs`
into the container with read only permissions as
`/var/host_logs`.

New in version v0.5.0.

### Note for OS/X users and remote daemon users:

OS/X users run `boot2docker` to create a minimalist
virtual machine running the docker daemon. That virtual machine then
launches docker commands on behalf of the OS/X command line. The means
that `host directories` refer to directories in the
`boot2docker` virtual machine, not the OS/X
filesystem.

Similarly, anytime when the docker daemon is on a remote machine, the
`host directories` always refer to directories on
the daemon’s machine.

### Backup, restore, or migrate data volumes

You cannot back up volumes using `docker export`,
`docker save` and `docker cp`
because they are external to images. Instead you can use
`--volumes-from` to start a new container that can
access the data-container’s volume. For example:

    $ sudo docker run -rm --volumes-from DATA -v $(pwd):/backup busybox tar cvf /backup/backup.tar /data

-   `-rm` - remove the container when it exits
-   `--volumes-from DATA` - attach to the volumes
    shared by the `DATA` container
-   `-v $(pwd):/backup` - bind mount the current
    directory into the container; to write the tar file to
-   `busybox` - a small simpler image - good for
    quick maintenance
-   `tar cvf /backup/backup.tar /data` - creates an
    uncompressed tar file of all the files in the `/data`
 directory

Then to restore to the same container, or another that you’ve made
elsewhere:

    # create a new data container
    $ sudo docker run -v /data -name DATA2 busybox true
    # untar the backup files into the new container's data volume
    $ sudo docker run -rm --volumes-from DATA2 -v $(pwd):/backup busybox tar xvf /backup/backup.tar
    data/
    data/sven.txt
    # compare to the original container
    $ sudo docker run -rm --volumes-from DATA -v `pwd`:/backup busybox ls /data
    sven.txt

You can use the basic techniques above to automate backup, migration and
restore testing using your preferred tools.

## Known Issues

-   [Issue 2702](https://github.com/dotcloud/docker/issues/2702):
    "lxc-start: Permission denied - failed to mount" could indicate a
    permissions problem with AppArmor. Please see the issue for a
    workaround.
-   [Issue 2528](https://github.com/dotcloud/docker/issues/2528): the
    busybox container is used to make the resulting container as small
    and simple as possible - whenever you need to interact with the data
    in the volume you mount it into another container.

