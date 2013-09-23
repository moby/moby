:title: Login Command
:description: Register or Login to the docker registry server
:keywords: login, docker, documentation

============================================================
``login`` -- Register or Login to the docker registry server
============================================================

::

   Usage: docker login [-h] [-e email] [-p password] [-u username] [SERVER]

   Register or Login to the docker registry server

    -e, --email=email
    -h, --help         Display this help
    -p, --password=password
    -u, --username=username

    If you want to login to a private registry you can
    specify this by adding the server name.

    example:
    docker login localhost:8080

