<!--[metadata]>
+++
title = "cp"
description = "The cp command description and usage"
keywords = ["copy, container, files, folders"]
[menu.main]
parent = "smn_cli"
+++
<![end-metadata]-->

# cp

    Usage: docker cp [OPTIONS] CONTAINER:SRC_PATH DEST_PATH | -
           docker cp [OPTIONS] SRC_PATH | - CONTAINER:DEST_PATH

    Copy files/folders between a container and the local filesystem

      -L, --follow-link=false    Always follow symbol link in SRC_PATH
      --help=false               Print usage

The `docker cp` utility copies the contents of `SRC_PATH` to the `DEST_PATH`.
You can copy from the container's file system to the local machine or the
reverse, from the local filesystem to the container. If `-` is specified for
either the `SRC_PATH` or `DEST_PATH`, you can also stream a tar archive from
`STDIN` or to `STDOUT`. The `CONTAINER` can be a running or stopped container.
The `SRC_PATH` or `DEST_PATH` be a file or directory.

The `docker cp` command assumes container paths are relative to the container's 
`/` (root) directory. This means supplying the initial forward slash is optional;
The command sees `compassionate_darwin:/tmp/foo/myfile.txt` and
`compassionate_darwin:tmp/foo/myfile.txt` as identical. Local machine paths can
be an absolute or relative value. The command interprets a local machine's
relative paths as relative to the current working directory where `docker cp` is
run.

The `cp` command behaves like the Unix `cp -a` command in that directories are
copied recursively with permissions preserved if possible. Ownership is set to
the user and primary group at the destination. For example, files copied to a
container are created with `UID:GID` of the root user. Files copied to the local
machine are created with the `UID:GID` of the user which invoked the `docker cp`
command.  If you specify the `-L` option, `docker cp` follows any symbolic link
in the `SRC_PATH`.

Assuming a path separator of `/`, a first argument of `SRC_PATH` and second
argument of `DEST_PATH`, the behavior is as follows:

- `SRC_PATH` specifies a file
    - `DEST_PATH` does not exist
        - the file is saved to a file created at `DEST_PATH`
    - `DEST_PATH` does not exist and ends with `/`
        - Error condition: the destination directory must exist.
    - `DEST_PATH` exists and is a file
        - the destination is overwritten with the source file's contents
    - `DEST_PATH` exists and is a directory
        - the file is copied into this directory using the basename from
          `SRC_PATH`
- `SRC_PATH` specifies a directory
    - `DEST_PATH` does not exist
        - `DEST_PATH` is created as a directory and the *contents* of the source
           directory are copied into this directory
    - `DEST_PATH` exists and is a file
        - Error condition: cannot copy a directory to a file
    - `DEST_PATH` exists and is a directory
        - `SRC_PATH` does not end with `/.`
            - the source directory is copied into this directory
        - `SRC_PATH` does end with `/.`
            - the *content* of the source directory is copied into this
              directory

The command requires `SRC_PATH` and `DEST_PATH` to exist according to the above
rules. If `SRC_PATH` is local and is a symbolic link, the symbolic link, not
the target, is copied by default. To copy the link target and not the link, specify 
the `-L` option.

A colon (`:`) is used as a delimiter between `CONTAINER` and its path. You can
also use `:` when specifying paths to a `SRC_PATH` or `DEST_PATH` on a local
machine, for example  `file:name.txt`. If you use a `:` in a local machine path,
you must be explicit with a relative or absolute path, for example:

    `/path/to/file:name.txt` or `./file:name.txt`

It is not possible to copy certain system files such as resources under
`/proc`, `/sys`, `/dev`, and mounts created by the user in the container.

Using `-` as the `SRC_PATH` streams the contents of `STDIN` as a tar archive.
The command extracts the content of the tar to the `DEST_PATH` in container's
filesystem. In this case, `DEST_PATH` must specify a directory. Using `-` as
`DEST_PATH` streams the contents of the resource as a tar archive to `STDOUT`.
