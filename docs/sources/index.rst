:title: Docker Documentation
:description: An overview of the Docker Documentation
:keywords: containers, lxc, concepts, explanation

.. image:: https://www.docker.io/static/img/linked/dockerlogo-horizontal.png

Introduction
------------

``docker``, the Linux Container Runtime, runs Unix processes with
strong guarantees of isolation across servers. Your software runs
repeatably everywhere because its :ref:`container_def` includes any
dependencies.

``docker`` runs three ways:

* as a daemon to manage LXC containers on your :ref:`Linux host
  <kernel>` (``sudo docker -d``)
* as a :ref:`CLI <cli>` which talks to the daemon's `REST API
  <api/docker_remote_api>`_ (``docker run ...``)
* as a client of :ref:`Repositories <working_with_the_repository>`
  that let you share what you've built (``docker pull, docker
  commit``).

Each use of ``docker`` is documented here. The features of Docker are
currently in active development, so this documentation will change
frequently.

For an overview of Docker, please see the `Introduction
<http://www.docker.io>`_. When you're ready to start working with
Docker, we have a `quick start <http://www.docker.io/gettingstarted>`_
and a more in-depth guide to :ref:`ubuntu_linux` and other
:ref:`installation_list` paths including prebuilt binaries,
Vagrant-created VMs, Rackspace and Amazon instances.

Enough reading! :ref:`Try it out! <running_examples>`
