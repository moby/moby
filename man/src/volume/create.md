Creates a new volume that containers can consume and store data in. If a name
is not specified, Docker generates a random name. You create a volume and then
configure the container to use it, for example:

    $ docker volume create hello
    hello
    $ docker run -d -v hello:/world busybox ls /world

The mount is created inside the container's `/src` directory. Docker doesn't
not support relative paths for mount points inside the container.

Multiple containers can use the same volume in the same time period. This is
useful if two containers need access to shared data. For example, if one
container writes and the other reads the data.

## Driver specific options

Some volume drivers may take options to customize the volume creation. Use the
`-o` or `--opt` flags to pass driver options:

    $ docker volume create --driver fake --opt tardis=blue --opt timey=wimey

These options are passed directly to the volume driver. Options for different
volume drivers may do different things (or nothing at all).

The built-in `local` driver on Windows does not support any options.

The built-in `local` driver on Linux accepts options similar to the linux
`mount` command:

    $ docker volume create --driver local --opt type=tmpfs --opt device=tmpfs --opt o=size=100m,uid=1000

Another example:

    $ docker volume create --driver local --opt type=btrfs --opt device=/dev/sda2
