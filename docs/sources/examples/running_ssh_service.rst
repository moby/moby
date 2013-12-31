:title: Running an SSH service
:description: A screencast of installing and running an sshd service
:keywords: docker, example, package installation, networking

.. _running_ssh_service:

SSH Daemon Service
==================

.. include:: example_header.inc


**Video:**

I've created a little screencast to show how to create an SSHd service
and connect to it. It is something like 11 minutes and not entirely
smooth, but it gives you a good idea.

.. note::
   This screencast was created before Docker version 0.5.2, so the
   daemon is unprotected and available via a TCP port. When you run
   through the same steps in a newer version of Docker, you will
   need to add ``sudo`` in front of each ``docker`` command in order
   to reach the daemon over its protected Unix socket.

.. raw:: html

    <div style="margin-top:10px;">
      <iframe width="800" height="400" src="http://ascii.io/a/2637/raw" frameborder="0"></iframe>
    </div>
        
You can also get this sshd container by using:

.. code-block:: bash

    sudo docker pull dhrp/sshd


The password is ``screencast``.

**Video's Transcription:**

.. code-block:: bash

         # Hello! We are going to try and install openssh on a container and run it as a service
         # let's pull ubuntu to get a base ubuntu image. 
         $ docker pull ubuntu
         # I had it so it was quick
         # now let's connect using -i for interactive and with -t for terminal 
         # we execute /bin/bash to get a prompt.
         $ docker run -i -t ubuntu /bin/bash
         # yes! we are in!
         # now lets install openssh
         $ apt-get update
         $ apt-get install openssh-server
         # ok. lets see if we can run it.
         $ which sshd
         # we need to create privilege separation directory
         $ mkdir /var/run/sshd
         $ /usr/sbin/sshd
         $ exit
         # now let's commit it 
         # which container was it?
         $ docker ps -a |more
         $ docker commit a30a3a2f2b130749995f5902f079dc6ad31ea0621fac595128ec59c6da07feea dhrp/sshd 
         # I gave the name dhrp/sshd for the container
         # now we can run it again 
         $ docker run -d dhrp/sshd /usr/sbin/sshd -D # D for daemon mode 
         # is it running?
         $ docker ps
         # yes!
         # let's stop it 
         $ docker stop 0ebf7cec294755399d063f4b1627980d4cbff7d999f0bc82b59c300f8536a562
         $ docker ps
         # and reconnect, but now open a port to it
         $ docker run -d -p 22 dhrp/sshd /usr/sbin/sshd -D 
         $ docker port b2b407cf22cf8e7fa3736fa8852713571074536b1d31def3fdfcd9fa4fd8c8c5 22
         # it has now given us a port to connect to
         # we have to connect using a public ip of our host
         $ hostname
         # *ifconfig* is deprecated, better use *ip addr show* now
         $ ifconfig
         $ ssh root@192.168.33.10 -p 49153
         # Ah! forgot to set root passwd
         $ docker commit b2b407cf22cf8e7fa3736fa8852713571074536b1d31def3fdfcd9fa4fd8c8c5 dhrp/sshd 
         $ docker ps -a
         $ docker run -i -t dhrp/sshd /bin/bash
         $ passwd
         $ exit
         $ docker commit 9e863f0ca0af31c8b951048ba87641d67c382d08d655c2e4879c51410e0fedc1 dhrp/sshd
         $ docker run -d -p 22 dhrp/sshd /usr/sbin/sshd -D
         $ docker port a0aaa9558c90cf5c7782648df904a82365ebacce523e4acc085ac1213bfe2206 22
         # *ifconfig* is deprecated, better use *ip addr show* now
         $ ifconfig
         $ ssh root@192.168.33.10 -p 49154
         # Thanks for watching, Thatcher thatcher@dotcloud.com
         # For Ubuntu 13.10 using stackbrew/ubuntu, I had do these additional steps:
         # change /etc/pam.d/sshd, pam_loginuid line 'required' to 'optional'
         # echo LANG=\"en_US.UTF-8\" > /etc/default/locale


