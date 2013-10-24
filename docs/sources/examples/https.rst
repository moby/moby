:title: Docker HTTPS Setup
:description: How to setup docker with https
:keywords: docker, example, https, daemon

.. _running_docker_https:

Running docker with https
=========================

Normally docker runs via http on ``/var/run/docker.sock``

.. code-block:: bash

   sudo docker -d &


If you wish to run docker via https you first need to generate a certificate
and a private key file. How to do this securely is beyond the scope of this
example, however the following command will generate an example one.

.. code-block:: bash

    openssl dsaparam -out dsaparam.pem 2048
    openssl gendsa -out privkey.pem dsaparam.pem
    openssl req -new -x509 -key privkey.pem -out cacert.pem -days 1095


Docker can then run using these certificates. Most commonly you will want to
run docker on a different port that the default unix socket when in https mode.

.. code-block:: bash

    sudo docker -d -sslkey=privkey.pem -sslcert=cacert.pem -H=tcp://0.0.0.0 -H unix:///var/run/docker.sock
