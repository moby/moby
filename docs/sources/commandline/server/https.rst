:title: HTTPS Docker Flags
:description: Flags to run docker with HTTPS
:keywords: docker, documentation, server, daemon, https

=======================================================
``-sslkey, -sslcert`` -- Run a docker daemon with https
=======================================================

::

    -sslcert="": path to SSL certificate file
    -sslkey="": path to SSL key file


Examples
--------

.. code-block:: bash

    sudo docker -d -H=tcp://0.0.0.0 -sslcert=cert.pem -sslkey=privkey.pem

This will run docker as a daemon listening for https
connections. This command uses go's built in https server so it only supports
a subset of the full TLS protocol. Currently it only supports TLSv1.0 but when
go 1.2 comes out it will have support for TLSv1.2.
