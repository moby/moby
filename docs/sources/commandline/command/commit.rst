:title: Commit Command
:description: Create a new image from a container's changes
:keywords: commit, docker, container, documentation

===========================================================
``commit`` -- Create a new image from a container's changes
===========================================================

::

    Usage: docker commit [OPTIONS] CONTAINER [REPOSITORY [TAG]]

    Create a new image from a container's changes

      -m="": Commit message
      -author="": Author (eg. "John Hannibal Smith <hannibal@a-team.com>"
      -run="": Config automatically applied when the image is
       run. "+`(ex: {"Cmd": ["cat", "/world"], "PortSpecs": ["22"]}')

Full -run example::

    {"Hostname": "",
     "User": "",
     "CpuShares": 0,
     "Memory": 0,
     "MemorySwap": 0,
     "PortSpecs": ["22", "80", "443"],
     "Tty": true,
     "OpenStdin": true,
     "StdinOnce": true,
     "Env": ["FOO=BAR", "FOO2=BAR2"],
     "Cmd": ["cat", "-e", "/etc/resolv.conf"],
     "Dns": ["8.8.8.8", "8.8.4.4"]}
