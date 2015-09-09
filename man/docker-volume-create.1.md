% DOCKER(1) Docker User Manuals
% Docker Community
% JULY 2015
# NAME
docker-volume-create - Create a new volume

# SYNOPSIS
**docker volume create**
[**-d**|**--driver**[=*DRIVER*]]
[**--help**]
[**--name**[=*NAME*]]
[**-o**|**--opt**[=*[]*]]

# DESCRIPTION

Creates a new volume that containers can consume and store data in. If a name is not specified, Docker generates a random name. You create a volume and then configure the container to use it, for example:

  ```
  $ docker volume create --name hello
  hello
  $ docker run -d -v hello:/world busybox ls /world
  ```

The mount is created inside the container's `/src` directory. Docker doesn't not support relative paths for mount points inside the container. 

Multiple containers can use the same volume in the same time period. This is useful if two containers need access to shared data. For example, if one container writes and the other reads the data.

## Driver specific options

Some volume drivers may take options to customize the volume creation. Use the `-o` or `--opt` flags to pass driver options:

  ```
  $ docker volume create --driver fake --opt tardis=blue --opt timey=wimey
  ```

These options are passed directly to the volume driver. Options for
different volume drivers may do different things (or nothing at all).

*Note*: The built-in `local` volume driver does not currently accept any options.

# OPTIONS
**-d**, **--driver**="local"
  Specify volume driver name

**--help**
  Print usage statement

**--name**=""
  Specify volume name

**-o**, **--opt**=[]
  Set driver specific options

# HISTORY
July 2015, created by Brian Goff <cpuguy83@gmail.com>
