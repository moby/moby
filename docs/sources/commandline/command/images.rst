:title: Images Command
:description: List images
:keywords: images, docker, container, documentation

=========================
``images`` -- List images
=========================

::

   Usage: docker images [-ahq] [--no-trunc] [--viz] [NAME]

   List images

    -a, --all       show all images
    -h, --help      Display this help
        --no-trunc  Don't truncate output
    -q, --quiet     only show numeric IDs
        --viz       output graph in graphviz format


Displaying images visually
--------------------------

::

    sudo docker images --viz | dot -Tpng -o docker.png

.. image:: images/docker_images.gif
