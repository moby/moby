:title: Events Command
:description: Get real time events from the server
:keywords: events, docker, documentation

=================================================================
``events`` -- Get real time events from the server
=================================================================

::

   Usage: docker events [-h] [--since timestamp]

   Get real time events from the server

    -h, --help  Display this help
        --since=timestamp
                Show events previously created (used for polling).

Examples
--------

Starting and stopping a container
.................................

.. code-block:: bash

    $ sudo docker start 4386fb97867d
    $ sudo docker stop 4386fb97867d

In another shell

.. code-block:: bash
    
    $ sudo docker events
    [2013-09-03 15:49:26 +0200 CEST] 4386fb97867d: (from 12de384bfb10) start
    [2013-09-03 15:49:29 +0200 CEST] 4386fb97867d: (from 12de384bfb10) die
    [2013-09-03 15:49:29 +0200 CEST] 4386fb97867d: (from 12de384bfb10) stop

