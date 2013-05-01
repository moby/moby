==============
Docker Builder
==============

.. contents:: Table of Contents

1. Format
=========

The Docker builder format is quite simple:

    ``instruction arguments``

The first instruction must be `FROM`

All instruction are to be placed in a file named `Dockerfile`

In order to place comments within a Dockerfile, simply prefix the line with "`#`"

2. Instructions
===============

Docker builder comes with a set of instructions:

1. FROM: Set from what image to build
2. RUN: Execute a command
3. INSERT: Insert a remote file (http) into the image

2.1 FROM
--------
    ``FROM <image>``

The `FROM` instruction must be the first one in order for Builder to know from where to run commands.

`FROM` can also be used in order to build multiple images within a single Dockerfile

2.2 RUN
-------
    ``RUN <command>``

The `RUN` instruction is the main one, it allows you to execute any commands on the `FROM` image and to save the results.
You can use as many `RUN` as you want within a Dockerfile, the commands will be executed on the result of the previous command.

2.3 INSERT
----------

    ``INSERT <file url> <path>``

The `INSERT` instruction will download the file at the given url and place it within the image at the given path.

.. note::
    The path must include the file name.

3. Dockerfile Examples
======================

::

    # Nginx
    #
    # VERSION               0.0.1
    # DOCKER-VERSION        0.2
    
    from ubuntu

    # make sure the package repository is up to date
    run echo "deb http://archive.ubuntu.com/ubuntu precise main universe" > /etc/apt/sources.list
    run apt-get update
    
    run apt-get install -y inotify-tools nginx apache openssh-server
    insert https://raw.github.com/creack/docker-vps/master/nginx-wrapper.sh /usr/sbin/nginx-wrapper

::

    # Firefox over VNC
    #
    # VERSION               0.3
    # DOCKER-VERSION        0.2
    
    from ubuntu
    # make sure the package repository is up to date
    run echo "deb http://archive.ubuntu.com/ubuntu precise main universe" > /etc/apt/sources.list
    run apt-get update
    
    # Install vnc, xvfb in order to create a 'fake' display and firefox
    run apt-get install -y x11vnc xvfb firefox
    run mkdir /.vnc
    # Setup a password
    run x11vnc -storepasswd 1234 ~/.vnc/passwd
    # Autostart firefox (might not be the best way to do it, but it does the trick)
    run bash -c 'echo "firefox" >> /.bashrc'
