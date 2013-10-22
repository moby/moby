:title: Images Command
:description: List images
:keywords: images, docker, container, documentation

=========================
``images`` -- List images
=========================

::

    Usage: docker images [OPTIONS] [NAME]

    List images

      -a=false: show all images (by default filter out the intermediate images used to build)
      -notrunc=false: Don't truncate output
      -q=false: only show numeric IDs
      -viz=false: output graph in graphviz format

Examples
--------

Listing the most recently created images
........................................

.. code-block:: bash

	$ docker images | head
	REPOSITORY                    TAG                 ID                  CREATED             SIZE
	<none>                        <none>              77af4d6b9913        19 hours ago        30.53 MB (virtual 1.089 GB)
	committest                    latest              b6fa739cedf5        19 hours ago        30.53 MB (virtual 1.089 GB)
	<none>                        <none>              78a85c484f71        19 hours ago        30.53 MB (virtual 1.089 GB)
	docker                        latest              30557a29d5ab        20 hours ago        30.53 MB (virtual 1.089 GB)
	<none>                        <none>              0124422dd9f9        20 hours ago        30.53 MB (virtual 1.089 GB)
	<none>                        <none>              18ad6fad3402        22 hours ago        23.68 MB (virtual 1.082 GB)
	<none>                        <none>              f9f1e26352f0        23 hours ago        30.46 MB (virtual 1.089 GB)
	tryout                        latest              2629d1fa0b81        23 hours ago        16.4 kB (virtual 131.5 MB)
	<none>                        <none>              5ed6274db6ce        24 hours ago        30.44 MB (virtual 1.089 GB)

Listing the full length image IDs
.................................


.. code-block:: bash

	$ docker images -notrunc | head
	REPOSITORY                    TAG                 ID                                                                 CREATED             SIZE
	<none>                        <none>              77af4d6b9913e693e8d0b4b294fa62ade6054e6b2f1ffb617ac955dd63fb0182   19 hours ago        30.53 MB (virtual 1.089 GB)
	committest                    latest              b6fa739cedf5ea12a620a439402b6004d057da800f91c7524b5086a5e4749c9f   19 hours ago        30.53 MB (virtual 1.089 GB)
	<none>                        <none>              78a85c484f71509adeaace20e72e941f6bdd2b25b4c75da8693efd9f61a37921   19 hours ago        30.53 MB (virtual 1.089 GB)
	docker                        latest              30557a29d5abc51e5f1d5b472e79b7e296f595abcf19fe6b9199dbbc809c6ff4   20 hours ago        30.53 MB (virtual 1.089 GB)
	<none>                        <none>              0124422dd9f9cf7ef15c0617cda3931ee68346455441d66ab8bdc5b05e9fdce5   20 hours ago        30.53 MB (virtual 1.089 GB)
	<none>                        <none>              18ad6fad340262ac2a636efd98a6d1f0ea775ae3d45240d3418466495a19a81b   22 hours ago        23.68 MB (virtual 1.082 GB)
	<none>                        <none>              f9f1e26352f0a3ba6a0ff68167559f64f3e21ff7ada60366e2d44a04befd1d3a   23 hours ago        30.46 MB (virtual 1.089 GB)
	tryout                        latest              2629d1fa0b81b222fca63371ca16cbf6a0772d07759ff80e8d1369b926940074   23 hours ago        16.4 kB (virtual 131.5 MB)
	<none>                        <none>              5ed6274db6ceb2397844896966ea239290555e74ef307030ebb01ff91b1914df   24 hours ago        30.44 MB (virtual 1.089 GB)

Displaying images visually
..........................

.. code-block:: bash

    sudo docker images -viz | dot -Tpng -o docker.png

.. image:: https://docs.docker.io/en/latest/_static/docker_images.gif
