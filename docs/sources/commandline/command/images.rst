:title: Images Command
:description: List images
:keywords: images, docker, container, documentation

=========================
``images`` -- List images
=========================

::

    Usage: docker images [OPTIONS] [NAME]

    List images

      -a=false: show all images
      -q=false: only show numeric IDs
      -viz=false: output in graphviz format

Displaying images visually
--------------------------

::

    sudo docker images -viz | dot -Tpng -o docker.png

.. image:: https://docs.docker.io/en/latest/_static/docker_images.gif
