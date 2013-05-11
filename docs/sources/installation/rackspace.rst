===============
Rackspace Cloud
===============

.. contents:: Table of Contents

Ubuntu 12.04
------------

1. Build an Ubuntu 12.04 server using the "Next generation cloud servers", with your desired size. It will give you the password, keep that you will need it later.
2. When the server is up and running ssh into the server.

    .. code-block:: bash

        $ ssh root@<server-ip>

3. Once you are logged in you should check what kernel version you are running.

    .. code-block:: bash

        $ uname -a
        Linux docker-12-04 3.2.0-38-virtual #61-Ubuntu SMP Tue Feb 19 12:37:47 UTC 2013 x86_64 x86_64 x86_64 GNU/Linux

4. Let's update the server package list

    .. code-block:: bash

        $ apt-get update

5. Now lets install Docker and it's dependencies. To keep things simple, we will use the Docker install script. It will take a couple of minutes.

    .. code-block:: bash

        $ curl get.docker.io | sudo sh -x

6. Docker runs best with a new kernel, so lets use 3.8.x

    .. code-block:: bash
        
        # add the ppa to get the right kernel package
        $ echo deb http://ppa.launchpad.net/ubuntu-x-swat/r-lts-backport/ubuntu precise main > /etc/apt/sources.list.d/xswat.list
        
        # add the key for the ppa
        $ sudo apt-key adv --keyserver keyserver.ubuntu.com --recv-keys 3B22AB97AF1CDFA9
        
        # update packages again
        $ apt-get update
        
        # install the new kernel
        $ apt-get install linux-image-3.8.0-19-generic
        
        # update grub so it will use the new kernel after we reboot
        $ update-grub
        
        # update-grub doesn't always work so lets make sure. ``/boot/grub/menu.lst`` was updated.
        $ grep 3.8.0- /boot/grub/menu.lst
        
        # nope it wasn't lets manually update ``/boot/grub/menu.lst``  (make sure you are searching for correct kernel version, look at initial uname -a results.)
        $ sed -i s/3.2.0-38-virtual/3.8.0-19-generic/ /boot/grub/menu.lst
        
        # once again lets make sure it worked.
        $ grep 3.8.0- /boot/grub/menu.lst
        title          Ubuntu 12.04.2 LTS, kernel 3.8.0-19-generic
        kernel          /boot/vmlinuz-3.8.0-19-generic root=/dev/xvda1 ro quiet splash console=hvc0
        initrd          /boot/initrd.img-3.8.0-19-generic
        title          Ubuntu 12.04.2 LTS, kernel 3.8.0-19-generic (recovery mode)
        kernel          /boot/vmlinuz-3.8.0-19-generic root=/dev/xvda1 ro quiet splash  single
        initrd          /boot/initrd.img-3.8.0-19-generic
        
        # much better.

7. Reboot server (either via command line or console)
8. login again and check to make sure the kernel was updated

    .. code-block:: bash
        
        $ ssh root@<server_ip>
        $ uname -a
        Linux docker-12-04 3.8.0-19-generic #30~precise1-Ubuntu SMP Wed May 1 22:26:36 UTC 2013 x86_64 x86_64 x86_64 GNU/Linux
        
        # nice 3.8.

9. Make sure docker is running and test it out.

    .. code-block:: bash
        
        $ start dockerd
        $ docker pull busybox
        $ docker run busybox /bin/echo hello world
        hello world

Ubuntu 12.10
------------

1. Build an Ubuntu 12.10 server using the "Next generation cloud servers", with your desired size. It will give you the password, keep that you will need it later.
2. When the server is up and running ssh into the server.

    .. code-block:: bash

        $ ssh root@<server-ip>

3. Once you are logged in you should check what kernel version you are running.

    .. code-block:: bash

        $ uname -a
        Linux docker-12-10 3.5.0-25-generic #39-Ubuntu SMP Mon Feb 25 18:26:58 UTC 2013 x86_64 x86_64 x86_64 GNU/Linux

4. Let's update the server package list

    .. code-block:: bash

        $ apt-get update

5. Now lets install Docker and it's dependencies. To keep things simple, we will use the Docker install script. It will take a couple of minutes.

    .. code-block:: bash

        $ curl get.docker.io | sudo sh -x

6. Docker runs best with a new kernel, so lets use 3.8.x

    .. code-block:: bash
        
        # add the ppa to get the right kernel package
        $ echo deb http://ppa.launchpad.net/ubuntu-x-swat/q-lts-backport/ubuntu quantal main > /etc/apt/sources.list.d/xswat.list
        
        # add the key for the ppa
        $ sudo apt-key adv --keyserver keyserver.ubuntu.com --recv-keys 3B22AB97AF1CDFA9
        
        # update packages again
        $ apt-get update
        
        # install the new kernel
        $ apt-get install linux-image-3.8.0-19-generic

        # make sure grub has been updated.
        $ grep 3.8.0- /boot/grub/menu.lst
        title   Ubuntu 12.10, kernel 3.8.0-19-generic
        kernel  /boot/vmlinuz-3.8.0-19-generic root=/dev/xvda1 ro quiet splash console=hvc0
        initrd  /boot/initrd.img-3.8.0-19-generic
        title   Ubuntu 12.10, kernel 3.8.0-19-generic (recovery mode)
        kernel  /boot/vmlinuz-3.8.0-19-generic root=/dev/xvda1 ro quiet splash  single
        initrd  /boot/initrd.img-3.8.0-19-generic
        
        # looks good. If it doesn't work for you, look at the notes for 12.04 to fix.

7. Reboot server (either via command line or console)
8. login again and check to make sure the kernel was updated

    .. code-block:: bash
        
        $ ssh root@<server_ip>
        $ uname -a
        Linux docker-12-10 3.8.0-19-generic #29~precise2-Ubuntu SMP Fri Apr 19 16:15:35 UTC 2013 x86_64 x86_64 x86_64 GNU/Linux
        
        # nice 3.8.

9. Make sure docker is running and test it out.

    .. code-block:: bash
        
        $ start dockerd
        $ docker pull busybox
        $ docker run busybox /bin/echo hello world
        hello world

Ubuntu 13.04
------------

1. Build an Ubuntu 13.04 server using the "Next generation cloud servers", with your desired size. It will give you the password, keep that you will need it later.
2. When the server is up and running ssh into the server.

    .. code-block:: bash

        $ ssh root@<server-ip>

3. Once you are logged in you should check what kernel version you are running.

    .. code-block:: bash

        $ uname -a
        Linux docker-1304 3.8.0-19-generic #29-Ubuntu SMP Wed Apr 17 18:16:28 UTC 2013 x86_64 x86_64 x86_64 GNU/Linux

4. Let's update the server package list

    .. code-block:: bash

        $ apt-get update

5. Now lets install Docker and it's dependencies. To keep things simple, we will use the Docker install script. It will take a couple of minutes.

    .. code-block:: bash

        $ curl get.docker.io | sudo sh -x

6. Make sure docker is running and test it out.

    .. code-block:: bash
        
        $ start dockerd
        $ docker pull busybox
        $ docker run busybox /bin/echo hello world
        hello world
 