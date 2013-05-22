=================================
Docker Index Environment Variable
=================================

Variable
--------

.. code-block:: sh

    DOCKER_INDEX_URL

Setting this environment variable on the docker server will change the URL docker index.
This address is used in commands such as ``docker login``, ``docker push`` and ``docker pull``.
The docker daemon doesn't need to be restarted for this parameter to take effect.

Example
-------

.. code-block:: sh

    docker -d &
    export DOCKER_INDEX_URL="https://index.docker.io"

