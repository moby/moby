:title: Login Command
:description: Register or Login to the docker registry server
:keywords: login, docker, documentation

============================================================
``login`` -- Register or Login to the docker registry server
============================================================

::

    Usage: docker login [OPTIONS] [SERVER]

    Register or Login to the docker registry server

    -e="": email
    -p="": password
    -u="": username

    If you want to login to a private registry you can
    specify this by adding the server name.

    example:
    docker login localhost:8080

