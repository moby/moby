% DOCKER(1) Docker User Manuals
% Docker Community
% JUNE 2014
# NAME
docker-cp - Copy files or folders from a container's PATH to a LOCALPATH
or to STDOUT.

# SYNOPSIS
**docker cp**
[**--pause**]
[**--help**]
CONTAINER:PATH|LOCALPATH|- CONTAINER:PATH|LOCALPATH|-

# DESCRIPTION

Copy files or folders from a `CONTAINER:PATH` to the `LOCALPATH` or to `STDOUT`. 
The `CONTAINER:PATH` is relative to the root of the container's filesystem. You
can copy from either a running or stopped container. 

The `PATH` can be a file or directory. The `docker cp` command assumes all
`PATH` values start at the `/` (root) directory. This means supplying the
initial forward slash is optional; The command sees
`compassionate_darwin:/tmp/foo/myfile.txt` and
`compassionate_darwin:tmp/foo/myfile.txt` as identical.

The `LOCALPATH` refers to a directory on the host. If you do not specify an
absolute path for your `LOCALPATH` value, Docker creates the directory relative to
where you run the `docker cp` command. For example, suppose you want to copy the
`/tmp/foo` directory from a container to the `/tmp` directory on your host. If
you run `docker cp` in your `~` (home) directory on the host:

		$ docker cp compassionate_darwin:tmp/foo /tmp

Docker creates a `/tmp/foo` directory on your host. Alternatively, you can omit
the leading slash in the command. If you execute this command from your home directory:

		$ docker cp compassionate_darwin:tmp/foo tmp

Docker creates a `~/tmp/foo` subdirectory.  

When copying files to an existing `LOCALPATH`, the `cp` command adds the new files to
the directory. For example, this command:

		$ docker cp sharp_ptolemy:/tmp/foo/myfile.txt /tmp

Creates a `/tmp/foo` directory on the host containing the `myfile.txt` file. If
you repeat the command but change the filename:

		$ docker cp sharp_ptolemy:/tmp/foo/secondfile.txt /tmp

Your `/tmp/foo` directory will contain both files:

		$ ls /tmp/foo
		myfile.txt secondfile.txt

You can also copy a local file or directory into a container:

  $ mkdir /tmp/foo && touch /tmp/foo/bar
  $ docker cp /tmp/foo sharp_ptolemy:/tmp

This will copy `/tmp/foo` into `/tmp/` of the container

It is recommended when using `docker cp` to use the `--pause` flag to ensure no
files are being written to or read from while copying.


# OPTIONS
**--help**
  Print usage statement

# EXAMPLES
An important shell script file, created in a bash shell, is copied from
the exited container to the current dir on the host:

    # docker cp c071f3c3ee81:setup.sh .

# HISTORY
April 2014, Originally compiled by William Henry (whenry at redhat dot com)
based on docker.com source material and internal work.
June 2014, updated by Sven Dowideit <SvenDowideit@home.org.au>
May 2015, updated by Brian Goff <cpuguy83@gmail.com>
