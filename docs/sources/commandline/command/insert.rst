:title: Insert Command
:description: Insert a file in an image
:keywords: insert, image, docker, documentation

==========================================================================
``insert`` -- Insert a file in an image
==========================================================================

::

    Usage: docker insert IMAGE URL PATH

    Use the specified IMAGE as the parent for a new image, and Insert a file 
    from URL into that new image at PATH

    ie. this does not modify the specified image.

Examples
--------

Insert file from github
.......................

.. code-block:: bash

    $ sudo docker insert 8283e18b24bc https://raw.github.com/metalivedev/django/master/postinstall /tmp/postinstall.sh

or

.. code-block:: bash

    $ sudo docker insert testme:commit https://raw.github.com/metalivedev/django/master/postinstall /tmp/postinstall.sh
    ba7952adfd26a6902ad6a4055f0055bcc0af4f5b2ad6ff717b4756b60be7acf1
    ba7952adfd26: 
    $sudo docker images | head
    REPOSITORY                                             TAG                 ID                  CREATED              SIZE
    <none>                                                 <none>              ba7952adfd26        5 seconds ago        16.58 kB (virtual 131.5 MB)
    testme                                                 commit              8ab3a789f868        About a minute ago   16.39 kB (virtual 131.5 MB)
