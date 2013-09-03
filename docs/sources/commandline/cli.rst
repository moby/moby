:title: Command Line Interface
:description: Docker's CLI command description and usage
:keywords: Docker, Docker documentation, CLI, command line

.. _cli:

Overview
======================

Docker Usage
~~~~~~~~~~~~

To list available commands, either run ``docker`` with no parameters or execute
``docker help``::

  $ sudo docker
    Usage: docker [-D] [-H value] COMMAND [parameters...]
      -D, --debug       Debug mode
      -H, --host=value  tcp://host:port to bind/connect to or unix://path/to/socket to use

    A self-sufficient runtime for linux containers.

    ...

Available Commands
~~~~~~~~~~~~~~~~~~

.. toctree::
   :maxdepth: 2

   command/attach
   command/build
   command/commit
   command/cp
   command/diff
   command/events
   command/export
   command/history
   command/images
   command/import
   command/info
   command/insert
   command/inspect
   command/kill
   command/login
   command/logs
   command/port
   command/ps
   command/pull
   command/push
   command/restart
   command/rm
   command/rmi
   command/run
   command/search
   command/start
   command/stop
   command/tag
   command/top
   command/version
   command/wait
