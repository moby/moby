:title: Command Line Interface
:description: Docker's CLI command description and usage
:keywords: Docker, Docker documentation, CLI, command line

.. _cli:

Overview
======================

Docker Usage
~~~~~~~~~~~~~~~~~~

To list available commands, either run ``docker`` with no parameters or execute
``docker help``::

  $ sudo docker
    Usage: docker [OPTIONS] COMMAND [arg...]
      -H=[unix:///var/run/docker.sock]: tcp://host:port to bind/connect to or unix://path/to/socket to use

    A self-sufficient runtime for linux containers.

    ...



Available Commands
~~~~~~~~~~~~~~~~~~

.. include:: command/attach.rst

.. include:: command/build.rst

.. include:: command/commit.rst

.. include:: command/cp.rst

.. include:: command/diff.rst

.. include:: command/events.rst

.. include:: command/export.rst

.. include:: command/history.rst

.. include:: command/images.rst

.. include:: command/import.rst

.. include:: command/info.rst

.. include:: command/insert.rst

.. include:: command/inspect.rst

.. include:: command/kill.rst

.. include:: command/login.rst

.. include:: command/logs.rst

.. include:: command/port.rst

.. include:: command/ps.rst

.. include:: command/pull.rst

.. include:: command/push.rst

.. include:: command/restart.rst

.. include:: command/rm.rst

.. include:: command/rmi.rst

.. include:: command/run.rst

.. include:: command/search.rst

.. include:: command/start.rst

.. include:: command/stop.rst

.. include:: command/tag.rst

.. include:: command/top.rst

.. include:: command/version.rst

.. include:: command/wait.rst


