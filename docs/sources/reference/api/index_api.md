Docker Index API[¶](#docker-index-api "Permalink to this headline")
===================================================================

1. Brief introduction[¶](#brief-introduction "Permalink to this headline")
--------------------------------------------------------------------------

-   This is the REST API for the Docker index
-   Authorization is done with basic auth over SSL
-   Not all commands require authentication, only those noted as such.

2. Endpoints[¶](#endpoints "Permalink to this headline")
--------------------------------------------------------

### 2.1 Repository[¶](#repository "Permalink to this headline")

#### Repositories[¶](#repositories "Permalink to this headline")

##### User Repo[¶](#user-repo "Permalink to this headline")

 `PUT `{.descname}`/v1/repositories/`{.descname}(*namespace*)`/`{.descname}(*repo\_name*)`/`{.descname}[¶](#put--v1-repositories-(namespace)-(repo_name)- "Permalink to this definition")
:   Create a user repository with the given `namespace`{.docutils
    .literal} and `repo_name`{.docutils .literal}.

    **Example Request**:

        PUT /v1/repositories/foo/bar/ HTTP/1.1
        Host: index.docker.io
        Accept: application/json
        Content-Type: application/json
        Authorization: Basic akmklmasadalkm==
        X-Docker-Token: true

        [{"id": "9e89cc6f0bc3c38722009fe6857087b486531f9a779a0c17e3ed29dae8f12c4f"}]

    Parameters:

    -   **namespace** – the namespace for the repo
    -   **repo\_name** – the name for the repo

    **Example Response**:

        HTTP/1.1 200
        Vary: Accept
        Content-Type: application/json
        WWW-Authenticate: Token signature=123abc,repository="foo/bar",access=write
        X-Docker-Token: signature=123abc,repository="foo/bar",access=write
        X-Docker-Endpoints: registry-1.docker.io [, registry-2.docker.io]

        ""

    Status Codes:

    -   **200** – Created
    -   **400** – Errors (invalid json, missing or invalid fields, etc)
    -   **401** – Unauthorized
    -   **403** – Account is not Active

 `DELETE `{.descname}`/v1/repositories/`{.descname}(*namespace*)`/`{.descname}(*repo\_name*)`/`{.descname}[¶](#delete--v1-repositories-(namespace)-(repo_name)- "Permalink to this definition")
:   Delete a user repository with the given `namespace`{.docutils
    .literal} and `repo_name`{.docutils .literal}.

    **Example Request**:

        DELETE /v1/repositories/foo/bar/ HTTP/1.1
        Host: index.docker.io
        Accept: application/json
        Content-Type: application/json
        Authorization: Basic akmklmasadalkm==
        X-Docker-Token: true

        ""

    Parameters:

    -   **namespace** – the namespace for the repo
    -   **repo\_name** – the name for the repo

    **Example Response**:

        HTTP/1.1 202
        Vary: Accept
        Content-Type: application/json
        WWW-Authenticate: Token signature=123abc,repository="foo/bar",access=delete
        X-Docker-Token: signature=123abc,repository="foo/bar",access=delete
        X-Docker-Endpoints: registry-1.docker.io [, registry-2.docker.io]

        ""

    Status Codes:

    -   **200** – Deleted
    -   **202** – Accepted
    -   **400** – Errors (invalid json, missing or invalid fields, etc)
    -   **401** – Unauthorized
    -   **403** – Account is not Active

##### Library Repo[¶](#library-repo "Permalink to this headline")

 `PUT `{.descname}`/v1/repositories/`{.descname}(*repo\_name*)`/`{.descname}[¶](#put--v1-repositories-(repo_name)- "Permalink to this definition")
:   Create a library repository with the given `repo_name`{.docutils
    .literal}. This is a restricted feature only available to docker
    admins.

    When namespace is missing, it is assumed to be `library`{.docutils
    .literal}

    **Example Request**:

        PUT /v1/repositories/foobar/ HTTP/1.1
        Host: index.docker.io
        Accept: application/json
        Content-Type: application/json
        Authorization: Basic akmklmasadalkm==
        X-Docker-Token: true

        [{"id": "9e89cc6f0bc3c38722009fe6857087b486531f9a779a0c17e3ed29dae8f12c4f"}]

    Parameters:

    -   **repo\_name** – the library name for the repo

    **Example Response**:

        HTTP/1.1 200
        Vary: Accept
        Content-Type: application/json
        WWW-Authenticate: Token signature=123abc,repository="library/foobar",access=write
        X-Docker-Token: signature=123abc,repository="foo/bar",access=write
        X-Docker-Endpoints: registry-1.docker.io [, registry-2.docker.io]

        ""

    Status Codes:

    -   **200** – Created
    -   **400** – Errors (invalid json, missing or invalid fields, etc)
    -   **401** – Unauthorized
    -   **403** – Account is not Active

 `DELETE `{.descname}`/v1/repositories/`{.descname}(*repo\_name*)`/`{.descname}[¶](#delete--v1-repositories-(repo_name)- "Permalink to this definition")
:   Delete a library repository with the given `repo_name`{.docutils
    .literal}. This is a restricted feature only available to docker
    admins.

    When namespace is missing, it is assumed to be `library`{.docutils
    .literal}

    **Example Request**:

        DELETE /v1/repositories/foobar/ HTTP/1.1
        Host: index.docker.io
        Accept: application/json
        Content-Type: application/json
        Authorization: Basic akmklmasadalkm==
        X-Docker-Token: true

        ""

    Parameters:

    -   **repo\_name** – the library name for the repo

    **Example Response**:

        HTTP/1.1 202
        Vary: Accept
        Content-Type: application/json
        WWW-Authenticate: Token signature=123abc,repository="library/foobar",access=delete
        X-Docker-Token: signature=123abc,repository="foo/bar",access=delete
        X-Docker-Endpoints: registry-1.docker.io [, registry-2.docker.io]

        ""

    Status Codes:

    -   **200** – Deleted
    -   **202** – Accepted
    -   **400** – Errors (invalid json, missing or invalid fields, etc)
    -   **401** – Unauthorized
    -   **403** – Account is not Active

#### Repository Images[¶](#repository-images "Permalink to this headline")

##### User Repo Images[¶](#user-repo-images "Permalink to this headline")

 `PUT `{.descname}`/v1/repositories/`{.descname}(*namespace*)`/`{.descname}(*repo\_name*)`/images`{.descname}[¶](#put--v1-repositories-(namespace)-(repo_name)-images "Permalink to this definition")
:   Update the images for a user repo.

    **Example Request**:

        PUT /v1/repositories/foo/bar/images HTTP/1.1
        Host: index.docker.io
        Accept: application/json
        Content-Type: application/json
        Authorization: Basic akmklmasadalkm==

        [{"id": "9e89cc6f0bc3c38722009fe6857087b486531f9a779a0c17e3ed29dae8f12c4f",
        "checksum": "b486531f9a779a0c17e3ed29dae8f12c4f9e89cc6f0bc3c38722009fe6857087"}]

    Parameters:

    -   **namespace** – the namespace for the repo
    -   **repo\_name** – the name for the repo

    **Example Response**:

        HTTP/1.1 204
        Vary: Accept
        Content-Type: application/json

        ""

    Status Codes:

    -   **204** – Created
    -   **400** – Errors (invalid json, missing or invalid fields, etc)
    -   **401** – Unauthorized
    -   **403** – Account is not Active or permission denied

 `GET `{.descname}`/v1/repositories/`{.descname}(*namespace*)`/`{.descname}(*repo\_name*)`/images`{.descname}[¶](#get--v1-repositories-(namespace)-(repo_name)-images "Permalink to this definition")
:   get the images for a user repo.

    **Example Request**:

        GET /v1/repositories/foo/bar/images HTTP/1.1
        Host: index.docker.io
        Accept: application/json

    Parameters:

    -   **namespace** – the namespace for the repo
    -   **repo\_name** – the name for the repo

    **Example Response**:

        HTTP/1.1 200
        Vary: Accept
        Content-Type: application/json

        [{"id": "9e89cc6f0bc3c38722009fe6857087b486531f9a779a0c17e3ed29dae8f12c4f",
        "checksum": "b486531f9a779a0c17e3ed29dae8f12c4f9e89cc6f0bc3c38722009fe6857087"},
        {"id": "ertwetewtwe38722009fe6857087b486531f9a779a0c1dfddgfgsdgdsgds",
        "checksum": "34t23f23fc17e3ed29dae8f12c4f9e89cc6f0bsdfgfsdgdsgdsgerwgew"}]

    Status Codes:

    -   **200** – OK
    -   **404** – Not found

##### Library Repo Images[¶](#library-repo-images "Permalink to this headline")

 `PUT `{.descname}`/v1/repositories/`{.descname}(*repo\_name*)`/images`{.descname}[¶](#put--v1-repositories-(repo_name)-images "Permalink to this definition")
:   Update the images for a library repo.

    **Example Request**:

        PUT /v1/repositories/foobar/images HTTP/1.1
        Host: index.docker.io
        Accept: application/json
        Content-Type: application/json
        Authorization: Basic akmklmasadalkm==

        [{"id": "9e89cc6f0bc3c38722009fe6857087b486531f9a779a0c17e3ed29dae8f12c4f",
        "checksum": "b486531f9a779a0c17e3ed29dae8f12c4f9e89cc6f0bc3c38722009fe6857087"}]

    Parameters:

    -   **repo\_name** – the library name for the repo

    **Example Response**:

        HTTP/1.1 204
        Vary: Accept
        Content-Type: application/json

        ""

    Status Codes:

    -   **204** – Created
    -   **400** – Errors (invalid json, missing or invalid fields, etc)
    -   **401** – Unauthorized
    -   **403** – Account is not Active or permission denied

 `GET `{.descname}`/v1/repositories/`{.descname}(*repo\_name*)`/images`{.descname}[¶](#get--v1-repositories-(repo_name)-images "Permalink to this definition")
:   get the images for a library repo.

    **Example Request**:

        GET /v1/repositories/foobar/images HTTP/1.1
        Host: index.docker.io
        Accept: application/json

    Parameters:

    -   **repo\_name** – the library name for the repo

    **Example Response**:

        HTTP/1.1 200
        Vary: Accept
        Content-Type: application/json

        [{"id": "9e89cc6f0bc3c38722009fe6857087b486531f9a779a0c17e3ed29dae8f12c4f",
        "checksum": "b486531f9a779a0c17e3ed29dae8f12c4f9e89cc6f0bc3c38722009fe6857087"},
        {"id": "ertwetewtwe38722009fe6857087b486531f9a779a0c1dfddgfgsdgdsgds",
        "checksum": "34t23f23fc17e3ed29dae8f12c4f9e89cc6f0bsdfgfsdgdsgdsgerwgew"}]

    Status Codes:

    -   **200** – OK
    -   **404** – Not found

#### Repository Authorization[¶](#repository-authorization "Permalink to this headline")

##### Library Repo[¶](#id1 "Permalink to this headline")

 `PUT `{.descname}`/v1/repositories/`{.descname}(*repo\_name*)`/auth`{.descname}[¶](#put--v1-repositories-(repo_name)-auth "Permalink to this definition")
:   authorize a token for a library repo

    **Example Request**:

        PUT /v1/repositories/foobar/auth HTTP/1.1
        Host: index.docker.io
        Accept: application/json
        Authorization: Token signature=123abc,repository="library/foobar",access=write

    Parameters:

    -   **repo\_name** – the library name for the repo

    **Example Response**:

        HTTP/1.1 200
        Vary: Accept
        Content-Type: application/json

        "OK"

    Status Codes:

    -   **200** – OK
    -   **403** – Permission denied
    -   **404** – Not found

##### User Repo[¶](#id2 "Permalink to this headline")

 `PUT `{.descname}`/v1/repositories/`{.descname}(*namespace*)`/`{.descname}(*repo\_name*)`/auth`{.descname}[¶](#put--v1-repositories-(namespace)-(repo_name)-auth "Permalink to this definition")
:   authorize a token for a user repo

    **Example Request**:

        PUT /v1/repositories/foo/bar/auth HTTP/1.1
        Host: index.docker.io
        Accept: application/json
        Authorization: Token signature=123abc,repository="foo/bar",access=write

    Parameters:

    -   **namespace** – the namespace for the repo
    -   **repo\_name** – the name for the repo

    **Example Response**:

        HTTP/1.1 200
        Vary: Accept
        Content-Type: application/json

        "OK"

    Status Codes:

    -   **200** – OK
    -   **403** – Permission denied
    -   **404** – Not found

### 2.2 Users[¶](#users "Permalink to this headline")

#### User Login[¶](#user-login "Permalink to this headline")

 `GET `{.descname}`/v1/users`{.descname}[¶](#get--v1-users "Permalink to this definition")
:   If you want to check your login, you can try this endpoint

    **Example Request**:

        GET /v1/users HTTP/1.1
        Host: index.docker.io
        Accept: application/json
        Authorization: Basic akmklmasadalkm==

    **Example Response**:

        HTTP/1.1 200 OK
        Vary: Accept
        Content-Type: application/json

        OK

    Status Codes:

    -   **200** – no error
    -   **401** – Unauthorized
    -   **403** – Account is not Active

#### User Register[¶](#user-register "Permalink to this headline")

 `POST `{.descname}`/v1/users`{.descname}[¶](#post--v1-users "Permalink to this definition")
:   Registering a new account.

    **Example request**:

        POST /v1/users HTTP/1.1
        Host: index.docker.io
        Accept: application/json
        Content-Type: application/json

        {"email": "sam@dotcloud.com",
         "password": "toto42",
         "username": "foobar"'}

    Json Parameters:

     

    -   **email** – valid email address, that needs to be confirmed
    -   **username** – min 4 character, max 30 characters, must match
        the regular expression [a-z0-9\_].
    -   **password** – min 5 characters

    **Example Response**:

        HTTP/1.1 201 OK
        Vary: Accept
        Content-Type: application/json

        "User Created"

    Status Codes:

    -   **201** – User Created
    -   **400** – Errors (invalid json, missing or invalid fields, etc)

#### Update User[¶](#update-user "Permalink to this headline")

 `PUT `{.descname}`/v1/users/`{.descname}(*username*)`/`{.descname}[¶](#put--v1-users-(username)- "Permalink to this definition")
:   Change a password or email address for given user. If you pass in an
    email, it will add it to your account, it will not remove the old
    one. Passwords will be updated.

    It is up to the client to verify that that password that is sent is
    the one that they want. Common approach is to have them type it
    twice.

    **Example Request**:

        PUT /v1/users/fakeuser/ HTTP/1.1
        Host: index.docker.io
        Accept: application/json
        Content-Type: application/json
        Authorization: Basic akmklmasadalkm==

        {"email": "sam@dotcloud.com",
         "password": "toto42"}

    Parameters:

    -   **username** – username for the person you want to update

    **Example Response**:

        HTTP/1.1 204
        Vary: Accept
        Content-Type: application/json

        ""

    Status Codes:

    -   **204** – User Updated
    -   **400** – Errors (invalid json, missing or invalid fields, etc)
    -   **401** – Unauthorized
    -   **403** – Account is not Active
    -   **404** – User not found

### 2.3 Search[¶](#search "Permalink to this headline")

If you need to search the index, this is the endpoint you would use.

#### Search[¶](#id3 "Permalink to this headline")

 `GET `{.descname}`/v1/search`{.descname}[¶](#get--v1-search "Permalink to this definition")
:   Search the Index given a search term. It accepts
    [GET](http://www.w3.org/Protocols/rfc2616/rfc2616-sec9.html#sec9.3)
    only.

    **Example request**:

        GET /v1/search?q=search_term HTTP/1.1
        Host: example.com
        Accept: application/json

    **Example response**:

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

    Query Parameters:

     

    -   **q** – what you want to search for

    Status Codes:

    -   **200** – no error
    -   **500** – server error


