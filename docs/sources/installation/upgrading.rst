.. _upgrading:

Upgrading
============

   We assume you are upgrading from within the operating system which runs your docker daemon.


Get the latest docker binary:

::

  wget http://get.docker.io/builds/$(uname -s)/$(uname -m)/docker-master.tgz



Unpack it to your current dir

::

   tar -xf docker-master.tgz


Stop your current daemon. How you stop your daemon depends on how you started it.

- If you started the daemon manually (``sudo docker -d``), you can just kill the process: ``killall docker``
- If the process was started using upstart (the ubuntu startup daemon), you may need to use that to stop it


Start docker in daemon mode (-d) and disconnect (&) starting ./docker will start the version in your current dir rather
than the one in your PATH.

Now start the daemon

::

   sudo ./docker -d &


Alternatively you can replace the docker binary in ``/usr/local/bin``