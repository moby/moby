:title: Index API
:description: API Documentation for Docker Index
:keywords: API, Docker, index, REST, documentation

=================
Docker Index API
=================

1. Brief introduction
=====================

- This is the REST API for the Docker index
- Authorization is done with basic auth over SSL
- Not all commands require authentication, only those noted as such.

2. Endpoints
============

2.1 Repository
^^^^^^^^^^^^^^

Repositories
*************

User Repo
~~~~~~~~~

.. http:put:: /v1/repositories/(namespace)/(repo_name)/

    Create a user repository with the given ``namespace`` and ``repo_name``.

    **Example Request**:

    .. sourcecode:: http

        PUT /v1/repositories/foo/bar/ HTTP/1.1
        Host: index.docker.io
        Accept: application/json
        Content-Type: application/json
        Authorization: Basic akmklmasadalkm==
        X-Docker-Token: true

        [{"id": "9e89cc6f0bc3c38722009fe6857087b486531f9a779a0c17e3ed29dae8f12c4f"}]

    :parameter namespace: the namespace for the repo
    :parameter repo_name: the name for the repo

    **Example Response**:

    .. sourcecode:: http

        HTTP/1.1 200
        Vary: Accept
        Content-Type: application/json
        WWW-Authenticate: Token signature=123abc,repository="foo/bar",access=write
        X-Docker-Token: signature=123abc,repository="foo/bar",access=write
        X-Docker-Endpoints: registry-1.docker.io [, registry-2.docker.io]

        ""

    :statuscode 200: Created
    :statuscode 400: Errors (invalid json, missing or invalid fields, etc)
    :statuscode 401: Unauthorized
    :statuscode 403: Account is not Active


.. http:delete:: /v1/repositories/(namespace)/(repo_name)/

    Delete a user repository with the given ``namespace`` and ``repo_name``.

    **Example Request**:

    .. sourcecode:: http

        DELETE /v1/repositories/foo/bar/ HTTP/1.1
        Host: index.docker.io
        Accept: application/json
        Content-Type: application/json
        Authorization: Basic akmklmasadalkm==
        X-Docker-Token: true

        ""

    :parameter namespace: the namespace for the repo
    :parameter repo_name: the name for the repo

    **Example Response**:

    .. sourcecode:: http

        HTTP/1.1 202
        Vary: Accept
        Content-Type: application/json
        WWW-Authenticate: Token signature=123abc,repository="foo/bar",access=delete
        X-Docker-Token: signature=123abc,repository="foo/bar",access=delete
        X-Docker-Endpoints: registry-1.docker.io [, registry-2.docker.io]

        ""

    :statuscode 200: Deleted
    :statuscode 202: Accepted
    :statuscode 400: Errors (invalid json, missing or invalid fields, etc)
    :statuscode 401: Unauthorized
    :statuscode 403: Account is not Active

Library Repo
~~~~~~~~~~~~

.. http:put:: /v1/repositories/(repo_name)/

    Create a library repository with the given ``repo_name``.
    This is a restricted feature only available to docker admins.
    
    When namespace is missing, it is assumed to be ``library``

    **Example Request**:

    .. sourcecode:: http

        PUT /v1/repositories/foobar/ HTTP/1.1
        Host: index.docker.io
        Accept: application/json
        Content-Type: application/json
        Authorization: Basic akmklmasadalkm==
        X-Docker-Token: true

        [{"id": "9e89cc6f0bc3c38722009fe6857087b486531f9a779a0c17e3ed29dae8f12c4f"}]

    :parameter repo_name:  the library name for the repo

    **Example Response**:

    .. sourcecode:: http

        HTTP/1.1 200
        Vary: Accept
        Content-Type: application/json
        WWW-Authenticate: Token signature=123abc,repository="library/foobar",access=write
        X-Docker-Token: signature=123abc,repository="foo/bar",access=write
        X-Docker-Endpoints: registry-1.docker.io [, registry-2.docker.io]

        ""

    :statuscode 200: Created
    :statuscode 400: Errors (invalid json, missing or invalid fields, etc)
    :statuscode 401: Unauthorized
    :statuscode 403: Account is not Active

.. http:delete:: /v1/repositories/(repo_name)/

    Delete a library repository with the given ``repo_name``.
    This is a restricted feature only available to docker admins.
    
    When namespace is missing, it is assumed to be ``library``

    **Example Request**:

    .. sourcecode:: http

        DELETE /v1/repositories/foobar/ HTTP/1.1
        Host: index.docker.io
        Accept: application/json
        Content-Type: application/json
        Authorization: Basic akmklmasadalkm==
        X-Docker-Token: true

        ""

    :parameter repo_name:  the library name for the repo

    **Example Response**:

    .. sourcecode:: http

        HTTP/1.1 202
        Vary: Accept
        Content-Type: application/json
        WWW-Authenticate: Token signature=123abc,repository="library/foobar",access=delete
        X-Docker-Token: signature=123abc,repository="foo/bar",access=delete
        X-Docker-Endpoints: registry-1.docker.io [, registry-2.docker.io]

        ""

    :statuscode 200: Deleted
    :statuscode 202: Accepted
    :statuscode 400: Errors (invalid json, missing or invalid fields, etc)
    :statuscode 401: Unauthorized
    :statuscode 403: Account is not Active

Repository Images
*****************

User Repo Images
~~~~~~~~~~~~~~~~

.. http:put:: /v1/repositories/(namespace)/(repo_name)/images

    Update the images for a user repo.

    **Example Request**:

    .. sourcecode:: http

        PUT /v1/repositories/foo/bar/images HTTP/1.1
        Host: index.docker.io
        Accept: application/json
        Content-Type: application/json
        Authorization: Basic akmklmasadalkm==

        [{"id": "9e89cc6f0bc3c38722009fe6857087b486531f9a779a0c17e3ed29dae8f12c4f",
        "checksum": "b486531f9a779a0c17e3ed29dae8f12c4f9e89cc6f0bc3c38722009fe6857087"}]

    :parameter namespace: the namespace for the repo
    :parameter repo_name: the name for the repo

    **Example Response**:

    .. sourcecode:: http

        HTTP/1.1 204
        Vary: Accept
        Content-Type: application/json

        ""

    :statuscode 204: Created
    :statuscode 400: Errors (invalid json, missing or invalid fields, etc)
    :statuscode 401: Unauthorized
    :statuscode 403: Account is not Active or permission denied


.. http:get:: /v1/repositories/(namespace)/(repo_name)/images

    get the images for a user repo.

    **Example Request**:

    .. sourcecode:: http

        GET /v1/repositories/foo/bar/images HTTP/1.1
        Host: index.docker.io
        Accept: application/json

    :parameter namespace: the namespace for the repo
    :parameter repo_name: the name for the repo

    **Example Response**:

    .. sourcecode:: http

        HTTP/1.1 200
        Vary: Accept
        Content-Type: application/json

        [{"id": "9e89cc6f0bc3c38722009fe6857087b486531f9a779a0c17e3ed29dae8f12c4f",
        "checksum": "b486531f9a779a0c17e3ed29dae8f12c4f9e89cc6f0bc3c38722009fe6857087"},
        {"id": "ertwetewtwe38722009fe6857087b486531f9a779a0c1dfddgfgsdgdsgds",
        "checksum": "34t23f23fc17e3ed29dae8f12c4f9e89cc6f0bsdfgfsdgdsgdsgerwgew"}]

    :statuscode 200: OK
    :statuscode 404: Not found

Library Repo Images
~~~~~~~~~~~~~~~~~~~

.. http:put:: /v1/repositories/(repo_name)/images

    Update the images for a library repo.

    **Example Request**:

    .. sourcecode:: http

        PUT /v1/repositories/foobar/images HTTP/1.1
        Host: index.docker.io
        Accept: application/json
        Content-Type: application/json
        Authorization: Basic akmklmasadalkm==

        [{"id": "9e89cc6f0bc3c38722009fe6857087b486531f9a779a0c17e3ed29dae8f12c4f",
        "checksum": "b486531f9a779a0c17e3ed29dae8f12c4f9e89cc6f0bc3c38722009fe6857087"}]

    :parameter repo_name: the library name for the repo

    **Example Response**:

    .. sourcecode:: http

        HTTP/1.1 204
        Vary: Accept
        Content-Type: application/json

        ""

    :statuscode 204: Created
    :statuscode 400: Errors (invalid json, missing or invalid fields, etc)
    :statuscode 401: Unauthorized
    :statuscode 403: Account is not Active or permission denied


.. http:get:: /v1/repositories/(repo_name)/images

    get the images for a library repo.

    **Example Request**:

    .. sourcecode:: http

        GET /v1/repositories/foobar/images HTTP/1.1
        Host: index.docker.io
        Accept: application/json

    :parameter repo_name: the library name for the repo

    **Example Response**:

    .. sourcecode:: http

        HTTP/1.1 200
        Vary: Accept
        Content-Type: application/json

        [{"id": "9e89cc6f0bc3c38722009fe6857087b486531f9a779a0c17e3ed29dae8f12c4f",
        "checksum": "b486531f9a779a0c17e3ed29dae8f12c4f9e89cc6f0bc3c38722009fe6857087"},
        {"id": "ertwetewtwe38722009fe6857087b486531f9a779a0c1dfddgfgsdgdsgds",
        "checksum": "34t23f23fc17e3ed29dae8f12c4f9e89cc6f0bsdfgfsdgdsgdsgerwgew"}]

    :statuscode 200: OK
    :statuscode 404: Not found


Repository Authorization
************************

Library Repo
~~~~~~~~~~~~

.. http:put:: /v1/repositories/(repo_name)/auth

    authorize a token for a library repo

    **Example Request**:

    .. sourcecode:: http

        PUT /v1/repositories/foobar/auth HTTP/1.1
        Host: index.docker.io
        Accept: application/json
        Authorization: Token signature=123abc,repository="library/foobar",access=write

    :parameter repo_name: the library name for the repo

    **Example Response**:

    .. sourcecode:: http

        HTTP/1.1 200
        Vary: Accept
        Content-Type: application/json

        "OK"

    :statuscode 200: OK
    :statuscode 403: Permission denied
    :statuscode 404: Not found


User Repo
~~~~~~~~~

.. http:put:: /v1/repositories/(namespace)/(repo_name)/auth

    authorize a token for a user repo

    **Example Request**:

    .. sourcecode:: http

        PUT /v1/repositories/foo/bar/auth HTTP/1.1
        Host: index.docker.io
        Accept: application/json
        Authorization: Token signature=123abc,repository="foo/bar",access=write

    :parameter namespace: the namespace for the repo
    :parameter repo_name: the name for the repo

    **Example Response**:

    .. sourcecode:: http

        HTTP/1.1 200
        Vary: Accept
        Content-Type: application/json

        "OK"

    :statuscode 200: OK
    :statuscode 403: Permission denied
    :statuscode 404: Not found


2.2 Users
^^^^^^^^^

User Login
**********

.. http:get:: /v1/users

    If you want to check your login, you can try this endpoint
    
    **Example Request**:
    
    .. sourcecode:: http
    
        GET /v1/users HTTP/1.1
        Host: index.docker.io
        Accept: application/json
        Authorization: Basic akmklmasadalkm==

    **Example Response**:

    .. sourcecode:: http

        HTTP/1.1 200 OK
        Vary: Accept
        Content-Type: application/json

        OK

    :statuscode 200: no error
    :statuscode 401: Unauthorized
    :statuscode 403: Account is not Active


User Register
*************

.. http:post:: /v1/users

    Registering a new account.

    **Example request**:

    .. sourcecode:: http

        POST /v1/users HTTP/1.1
        Host: index.docker.io
        Accept: application/json
        Content-Type: application/json

        {"email": "sam@dotcloud.com",
         "password": "toto42",
         "username": "foobar"'}

    :jsonparameter email: valid email address, that needs to be confirmed
    :jsonparameter username: min 4 character, max 30 characters, must match the regular expression [a-z0-9\_].
    :jsonparameter password: min 5 characters

    **Example Response**:

    .. sourcecode:: http

        HTTP/1.1 201 OK
        Vary: Accept
        Content-Type: application/json

        "User Created"

    :statuscode 201: User Created
    :statuscode 400: Errors (invalid json, missing or invalid fields, etc)

Update User
***********

.. http:put:: /v1/users/(username)/

    Change a password or email address for given user. If you pass in an email,
    it will add it to your account, it will not remove the old one. Passwords will
    be updated.

    It is up to the client to verify that that password that is sent is the one that
    they want. Common approach is to have them type it twice.

    **Example Request**:

    .. sourcecode:: http

        PUT /v1/users/fakeuser/ HTTP/1.1
        Host: index.docker.io
        Accept: application/json
        Content-Type: application/json
        Authorization: Basic akmklmasadalkm==

        {"email": "sam@dotcloud.com",
         "password": "toto42"}

    :parameter username: username for the person you want to update

    **Example Response**:

    .. sourcecode:: http

        HTTP/1.1 204
        Vary: Accept
        Content-Type: application/json

        ""

    :statuscode 204: User Updated
    :statuscode 400: Errors (invalid json, missing or invalid fields, etc)
    :statuscode 401: Unauthorized
    :statuscode 403: Account is not Active
    :statuscode 404: User not found


2.3 Search
^^^^^^^^^^
If you need to search the index, this is the endpoint you would use.

Search
******

.. http:get:: /v1/search

   Search the Index given a search term. It accepts :http:method:`get` only.

   **Example request**:

   .. sourcecode:: http

      GET /v1/search?q=search_term HTTP/1.1
      Host: example.com
      Accept: application/json


   **Example response**:

   .. sourcecode:: http

      HTTP/1.1 200 OK
      Vary: Accept
      Content-Type: application/json

      {"query":"search_term",
        "num_results": 3,
        "results" : [
           {"name": "ubuntu", "description": "An ubuntu image..."},
           {"name": "centos", "description": "A centos image..."},
           {"name": "fedora", "description": "A fedora image..."}
         ]
       }

   :query q: what you want to search for
   :statuscode 200: no error
   :statuscode 500: server error
