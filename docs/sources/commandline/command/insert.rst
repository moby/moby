:title: Insert Command
:description: Insert a file in an image
:keywords: insert, image, docker, documentation

==========================================================================
``insert`` -- Insert a file in an image
==========================================================================

::

    Usage: docker insert IMAGE URL PATH

    Insert a file from URL in the IMAGE at PATH

Examples
--------

Insert file from github
.......................

.. code-block:: bash

    $ sudo docker insert 8283e18b24bc https://raw.github.com/metalivedev/django/master/postinstall /tmp/postinstall.sh
