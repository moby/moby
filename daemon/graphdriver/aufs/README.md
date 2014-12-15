## aufs - a storage backend based on AUFS

The aufs graph driver uses AUFS (advanced multi layered unification filesystem)
to implement COW layers. For each graph location ( typically stored at
/var/lib/docker/aufs ), three directories are created:

- layers: contains the metadata files for each layers. Each metatada file
  contains its layer's parent ids.

- diff: contains content of the layers. A layer's content will be saved into a
  directory with the layer's id as the directory name.

- mnt: mount points of the rw layers.

### options

The aufs backend supports some options that you can specify when starting the
docker daemon using the `--storage-opt` flags.
This uses the `aufs` prefix and would be used something like `docker -d --storage-opt aufs.foo=bar`

Here is the list of supported options:

 *  `aufs.mountopt`

    Specifies extra mount options used when mounting the aufs layers.
