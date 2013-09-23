:title: Commit Command
:description: Create a new image from a container's changes
:keywords: commit, docker, container, documentation

===========================================================
``commit`` -- Create a new image from a container's changes
===========================================================

::

   Usage: docker commit [-h] [-a name] [-m mes] [-r config] CONTAINER [REPOSITORY [TAG]]

   Create a new image from a container's changes

    -a, --author=name  Author (eg. "John Hannibal Smith <hannibal@a-team.com>")
    -h, --help         Display this help
    -m, --message=mes  Commit message
    -r, --run=config   Config automatically applied when the image is run.
                       (ex: {"Cmd": ["cat", "/world"], "PortSpecs": ["22"]}')

Full --run example::

{
      "Entrypoint" : null,
      "Privileged" : false,
      "User" : "",
      "VolumesFrom" : "",
      "Cmd" : ["cat", "-e", "/etc/resolv.conf"],
      "Dns" : ["8.8.8.8", "8.8.4.4"],
      "MemorySwap" : 0,
      "AttachStdin" : false,
      "AttachStderr" : false,
      "CpuShares" : 0,
      "OpenStdin" : false,
      "Volumes" : null,
      "Hostname" : "122612f45831",
      "PortSpecs" : ["22", "80", "443"],
      "Image" : "b750fe79269d2ec9a3c593ef05b4332b1d1a02a62b4accb2c21d589ff2f5f2dc",
      "Tty" : false,
      "Env" : [
         "HOME=/",
         "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
      ],
      "StdinOnce" : false,
      "Domainname" : "",
      "WorkingDir" : "/",
      "NetworkDisabled" : false,
      "Memory" : 0,
      "AttachStdout" : false
}
