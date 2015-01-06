% DOCKER(1) Docker User Manuals
% Docker Community
% JANUARY 2015
# NAME
**docker-cp** -- copy files between containers or a local path

# SYNOPSIS
**docker cp**
[**--help**]
[SRC_CONTAINER:]SRC_PATH [DST_CONTAINER:]DST_PATH

# DESCRIPTION
Copy files between a container and your local host or between any two
containers. If not absolute, a container path is considered relative to the
container's working directory. Files can be copied to and from a running or
stopped container.

The behavior is similar to the common Unix utility `cp -a` in that directories
are copied recursively and file mode, permission, and ownership are preserved
if possible.

Assuming a path separator of `/`, the behavior is as follows:

- `SRC_PATH` specifies a file
    - `DST_PATH` does not exist
        - the file is saved to a file created at `DST_PATH`
    - `DST_PATH` does not exist and ends with `/`
        - Error condition: the destination directory must exist.
    - `DST_PATH` exists and is a file
        - the destination is overwritten with the contents of the source file
    - `DST_PATH` exists and is a directory
        - the file is copied into this directory using the basename from
          `SRC_PATH`
- `SRC_PATH` specifies a directory
    - `DST_PATH` does not exist
        - `DST_PATH` is created as a directory and the *contents* of the source
           directory are copied into this directory
    - `DST_PATH` exists and is a file
        - Error condition: cannot copy a directory to a file
    - `DST_PATH` exists and is a directory
        - `SRC_PATH` does not end with `/.`
            - the source directory is copied into this directory
        - `SRC_PAPTH` does end with `/.`
            - the *content* of the source directory is copied into this
              directory

The command will always fail if the `SRC_PATH` resource does not exist or if
the parent directory of `DST_PATH` does not exist.

It is not possible to copy certain system files such as resources under
`/proc`, `/sys`, `/dev`, and mounts created by the user in the container.

# OPTIONS
**--help**
  Print usage statement

# EXAMPLES
An important shell script file, created in a bash shell, is copied from
the exited container to the current dir on the host:

    # docker cp c071f3c3ee81:setup.sh .

A configuration file is copied into a stopped container and the container is
restarted:

    # docker cp config.yml 8cce319429b2:/etc/my-app.d
    # docker start 8cce319429b2

Directory contents from one container are copied to another container's
working directory:

	# docker cp 8cce319429b2:/crunched_data/. c071f3c3ee81:.

# HISTORY
April 2014, Originally compiled by William Henry (whenry at redhat dot com)
based on docker.com source material and internal work.
June 2014, updated by Sven Dowideit <SvenDowideit@home.org.au>
January 2015, updated by Josh Hawn <josh.hawn@docker.com>
