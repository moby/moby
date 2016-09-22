Creates a writeable container layer over the specified image and prepares it for
running the specified command. The container ID is then printed to STDOUT. This
is similar to **docker run -d** except the container is never started. You can 
then use the **docker start <container_id>** command to start the container at
any point.

The initial status of the container created with **docker create** is 'created'.

# OPTIONS 

The `CONTAINER-DIR` must be an absolute path such as `/src/docs`. The `HOST-DIR`
can be an absolute path or a `name` value. A `name` value must start with an
alphanumeric character, followed by `a-z0-9`, `_` (underscore), `.` (period) or
`-` (hyphen). An absolute path starts with a `/` (forward slash).

If you supply a `HOST-DIR` that is an absolute path,  Docker bind-mounts to the
path you specify. If you supply a `name`, Docker creates a named volume by that
`name`. For example, you can specify either `/foo` or `foo` for a `HOST-DIR`
value. If you supply the `/foo` value, Docker creates a bind-mount. If you
supply the `foo` specification, Docker creates a named volume.

You can specify multiple  **-v** options to mount one or more mounts to a
container. To use these same mounts in other containers, specify the
**--volumes-from** option also.

You can add `:ro` or `:rw` suffix to a volume to mount it  read-only or
read-write mode, respectively. By default, the volumes are mounted read-write.
See examples.

Labeling systems like SELinux require that proper labels are placed on volume
content mounted into a container. Without a label, the security system might
prevent the processes running inside the container from using the content. By
default, Docker does not change the labels set by the OS.

To change a label in the container context, you can add either of two suffixes
`:z` or `:Z` to the volume mount. These suffixes tell Docker to relabel file
objects on the shared volumes. The `z` option tells Docker that two containers
share the volume content. As a result, Docker labels the content with a shared
content label. Shared volume labels allow all containers to read/write content.
The `Z` option tells Docker to label the content with a private unshared label.
Only the current container can use a private volume.

By default bind mounted volumes are `private`. That means any mounts done
inside container will not be visible on host and vice-a-versa. One can change
this behavior by specifying a volume mount propagation property. Making a
volume `shared` mounts done under that volume inside container will be
visible on host and vice-a-versa. Making a volume `slave` enables only one
way mount propagation and that is mounts done on host under that volume
will be visible inside container but not the other way around.

To control mount propagation property of volume one can use `:[r]shared`,
`:[r]slave` or `:[r]private` propagation flag. Propagation property can
be specified only for bind mounted volumes and not for internal volumes or
named volumes. For mount propagation to work source mount point (mount point
where source dir is mounted on) has to have right propagation properties. For
shared volumes, source mount point has to be shared. And for slave volumes,
source mount has to be either shared or slave.

Use `df <source-dir>` to figure out the source mount and then use
`findmnt -o TARGET,PROPAGATION <source-mount-dir>` to figure out propagation
properties of source mount. If `findmnt` utility is not available, then one
can look at mount entry for source mount point in `/proc/self/mountinfo`. Look
at `optional fields` and see if any propagaion properties are specified.
`shared:X` means mount is `shared`, `master:X` means mount is `slave` and if
nothing is there that means mount is `private`.

To change propagation properties of a mount point use `mount` command. For
example, if one wants to bind mount source directory `/foo` one can do
`mount --bind /foo /foo` and `mount --make-private --make-shared /foo`. This
will convert /foo into a `shared` mount point. Alternatively one can directly
change propagation properties of source mount. Say `/` is source mount for
`/foo`, then use `mount --make-shared /` to convert `/` into a `shared` mount.

> **Note**:
> When using systemd to manage the Docker daemon's start and stop, in the systemd
> unit file there is an option to control mount propagation for the Docker daemon
> itself, called `MountFlags`. The value of this setting may cause Docker to not
> see mount propagation changes made on the mount point. For example, if this value
> is `slave`, you may not be able to use the `shared` or `rshared` propagation on
> a volume.


To disable automatic copying of data from the container path to the volume, use
the `nocopy` flag. The `nocopy` flag can be set on bind mounts and named volumes.

# EXAMPLES

## Specify isolation technology for container (--isolation)

This option is useful in situations where you are running Docker containers on
Windows. The `--isolation=<value>` option sets a container's isolation
technology. On Linux, the only supported is the `default` option which uses
Linux namespaces. On Microsoft Windows, you can specify these values:

* `default`: Use the value specified by the Docker daemon's `--exec-opt` . If the `daemon` does not specify an isolation technology, Microsoft Windows uses `process` as its default value.
* `process`: Namespace isolation only.
* `hyperv`: Hyper-V hypervisor partition-based isolation.

Specifying the `--isolation` flag without a value is the same as setting `--isolation="default"`.
