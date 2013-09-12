:title: Host Integration
:description: How to generate scripts for upstart, systemd, etc.
:keywords: systemd, upstart, supervisor, docker, documentation, host integration

Introduction
============

When you have finished setting up your image and are happy with your running
container, you may want to use a process manager like `upstart` or `systemd`.

In order to do so, we provide a simple image: creack/manger:min.

This image takes the container ID as parameter. We also can specify the kind of
process manager and meta datas like Author and Description.

If no process manager is specified, then `upstart` is assumed.

Note: The result will be an output to stdout.

Usage
=====
Usage: docker run creack/manager:min [OPTIONS] <container id>

  -a="<none>": Author of the image
  -d="<none>": Description of the image
  -t="upstart": Type of manager requested

Development
===========

The image creack/manager:min is a `busybox` base with the binary as entrypoint.
It is meant to be light and fast to download.

Now, if you want/need to change or add things, you can download the full
creack/manager repository that contains creack/manager:min and
creack/manager:dev.

The Dockerfiles and the sources are available in `/contrib/host_integration`.


Upstart
=======

Upstart is the default process manager. The generated script will start the
container after docker daemon. If the container dies, it will respawn.
Start/Restart/Stop/Reload are supported. Reload will send a SIGHUP to the container.

Example:
`CID=$(docker run -d creack/firefo-vnc)`
`docker run creack/manager:min -a 'Guillaume J. Charmes <guillaume@dotcloud.com>' -d 'Awesome Firefox in VLC' $CID > /etc/init/firefoxvnc.conf`

You can now do `start firefoxvnc` or `stop firefoxvnc` and if the container
dies for some reason, upstart will restart it.

Systemd
=======

In order to generate a systemd script, we need to -t option. The generated
script will start the container after docker daemon. If the container dies, it
will respawn.
Start/Restart/Reload/Stop are supported.

Example (fedora):
`CID=$(docker run -d creack/firefo-vnc)`
`docker run creack/manager:min -t systemd -a 'Guillaume J. Charmes <guillaume@dotcloud.com>' -d 'Awesome Firefox in VLC' $CID > /usr/lib/systemd/system/firefoxvnc.service`

You can now do `systemctl start firefoxvnc` or `systemctl stop firefoxvnc`
and if the container dies for some reason, systemd will restart it.
