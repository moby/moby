:title: docker.io Accounts API
:description: API Documentation for docker.io accounts.
:keywords: API, Docker, accounts, REST, documentation


======================
docker.io Accounts API
======================


1. Endpoints
============


1.1 Get a single user
^^^^^^^^^^^^^^^^^^^^^

.. http:get:: /api/v1.1/users/:username/

    Get profile info for the specified user.

    :param username: username of the user whose profile info is being requested.

    :reqheader Authorization: required authentication credentials of either type HTTP Basic or OAuth Bearer Token.

    :statuscode 200: success, user data returned.
    :statuscode 401: authentication error.
    :statuscode 403: permission error, authenticated user must be the user whose data is being requested, OAuth access tokens must have ``profile_read`` scope.
    :statuscode 404: the specified username does not exist.

    **Example request**:

    .. sourcecode:: http

        GET /api/v1.1/users/janedoe/ HTTP/1.1
        Host: www.docker.io
        Accept: application/json
        Authorization: Basic dXNlcm5hbWU6cGFzc3dvcmQ=

    **Example response**:

    .. sourcecode:: http

        HTTP/1.1 200 OK
        Content-Type: application/json

        {
            "id": 2,
            "username": "janedoe",
            "url": "https://www.docker.io/api/v1.1/users/janedoe/",
            "date_joined": "2014-02-12T17:58:01.431312Z",
            "type": "User",
            "full_name": "Jane Doe",
            "location": "San Francisco, CA",
            "company": "Success, Inc.",
            "profile_url": "https://docker.io/",
            "gravatar_url": "https://secure.gravatar.com/avatar/0212b397124be4acd4e7dea9aa357.jpg?s=80&r=g&d=mm"
            "email": "jane.doe@example.com",
            "is_active": true
        }


1.2 Update a single user
^^^^^^^^^^^^^^^^^^^^^^^^

.. http:patch:: /api/v1.1/users/:username/

    Update profile info for the specified user.

    :param username: username of the user whose profile info is being updated.

    :jsonparam string full_name: (optional) the new name of the user.
    :jsonparam string location: (optional) the new location.
    :jsonparam string company: (optional) the new company of the user.
    :jsonparam string profile_url: (optional) the new profile url.
    :jsonparam string gravatar_email: (optional) the new Gravatar email address.

    :reqheader Authorization: required authentication credentials of either type HTTP Basic or OAuth Bearer Token.
    :reqheader Content-Type: MIME Type of post data. JSON, url-encoded form data, etc.

    :statuscode 200: success, user data updated.
    :statuscode 400: post data validation error.
    :statuscode 401: authentication error.
    :statuscode 403: permission error, authenticated user must be the user whose data is being updated, OAuth access tokens must have ``profile_write`` scope.
    :statuscode 404: the specified username does not exist.

    **Example request**:

    .. sourcecode:: http

        PATCH /api/v1.1/users/janedoe/ HTTP/1.1
        Host: www.docker.io
        Accept: application/json
        Authorization: Basic dXNlcm5hbWU6cGFzc3dvcmQ=

        {
            "location": "Private Island",
            "profile_url": "http://janedoe.com/",
            "company": "Retired",
        }

    **Example response**:

    .. sourcecode:: http

        HTTP/1.1 200 OK
        Content-Type: application/json

        {
            "id": 2,
            "username": "janedoe",
            "url": "https://www.docker.io/api/v1.1/users/janedoe/",
            "date_joined": "2014-02-12T17:58:01.431312Z",
            "type": "User",
            "full_name": "Jane Doe",
            "location": "Private Island",
            "company": "Retired",
            "profile_url": "http://janedoe.com/",
            "gravatar_url": "https://secure.gravatar.com/avatar/0212b397124be4acd4e7dea9aa357.jpg?s=80&r=g&d=mm"
            "email": "jane.doe@example.com",
            "is_active": true
        }


1.3 List email addresses for a user
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

.. http:get:: /api/v1.1/users/:username/emails/

    List email info for the specified user.

    :param username: username of the user whose profile info is being updated.

    :reqheader Authorization: required authentication credentials of either type HTTP Basic or OAuth Bearer Token

    :statuscode 200: success, user data updated.
    :statuscode 401: authentication error.
    :statuscode 403: permission error, authenticated user must be the user whose data is being requested, OAuth access tokens must have ``email_read`` scope.
    :statuscode 404: the specified username does not exist.

    **Example request**:

    .. sourcecode:: http

        GET /api/v1.1/users/janedoe/emails/ HTTP/1.1
        Host: www.docker.io
        Accept: application/json
        Authorization: Bearer zAy0BxC1wDv2EuF3tGs4HrI6qJp6KoL7nM

    **Example response**:

    .. sourcecode:: http

        HTTP/1.1 200 OK
        Content-Type: application/json

        [
            {
                "email": "jane.doe@example.com",
                "verified": true,
                "primary": true
            }
        ]


1.4 Add email address for a user
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

.. http:post:: /api/v1.1/users/:username/emails/

    Add a new email address to the specified user's account. The email address
    must be verified separately, a confirmation email is not automatically sent.

    :jsonparam string email: email address to be added.

    :reqheader Authorization: required authentication credentials of either type HTTP Basic or OAuth Bearer Token.
    :reqheader Content-Type: MIME Type of post data. JSON, url-encoded form data, etc.

    :statuscode 201: success, new email added.
    :statuscode 400: data validation error.
    :statuscode 401: authentication error.
    :statuscode 403: permission error, authenticated user must be the user whose data is being requested, OAuth access tokens must have ``email_write`` scope.
    :statuscode 404: the specified username does not exist.

    **Example request**:

    .. sourcecode:: http

        POST /api/v1.1/users/janedoe/emails/ HTTP/1.1
        Host: www.docker.io
        Accept: application/json
        Content-Type: application/json
        Authorization: Bearer zAy0BxC1wDv2EuF3tGs4HrI6qJp6KoL7nM

        {
            "email": "jane.doe+other@example.com"
        }

    **Example response**:

    .. sourcecode:: http

        HTTP/1.1 201 Created
        Content-Type: application/json

        {
            "email": "jane.doe+other@example.com",
            "verified": false,
            "primary": false
        }


1.5 Update an email address for a user
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

.. http:patch:: /api/v1.1/users/:username/emails/

    Update an email address for the specified user to either verify an email
    address or set it as the primary email for the user. You cannot use this
    endpoint to un-verify an email address. You cannot use this endpoint to
    unset the primary email, only set another as the primary.

    :param username: username of the user whose email info is being updated.

    :jsonparam string email: the email address to be updated.
    :jsonparam boolean verified: (optional) whether the email address is verified, must be ``true`` or absent.
    :jsonparam boolean primary: (optional) whether to set the email address as the primary email, must be ``true`` or absent.

    :reqheader Authorization: required authentication credentials of either type HTTP Basic or OAuth Bearer Token.
    :reqheader Content-Type: MIME Type of post data. JSON, url-encoded form data, etc.

    :statuscode 200: success, user's email updated.
    :statuscode 400: data validation error.
    :statuscode 401: authentication error.
    :statuscode 403: permission error, authenticated user must be the user whose data is being updated, OAuth access tokens must have ``email_write`` scope.
    :statuscode 404: the specified username or email address does not exist.

    **Example request**:

    Once you have independently verified an email address.

    .. sourcecode:: http

        PATCH /api/v1.1/users/janedoe/emails/ HTTP/1.1
        Host: www.docker.io
        Accept: application/json
        Authorization: Basic dXNlcm5hbWU6cGFzc3dvcmQ=

        {
            "email": "jane.doe+other@example.com",
            "verified": true,
        }

    **Example response**:

    .. sourcecode:: http

        HTTP/1.1 200 OK
        Content-Type: application/json

        {
            "email": "jane.doe+other@example.com",
            "verified": true,
            "primary": false
        }


1.6 Delete email address for a user
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

.. http:delete:: /api/v1.1/users/:username/emails/

    Delete an email address from the specified user's account. You cannot
    delete a user's primary email address.

    :jsonparam string email: email address to be deleted.

    :reqheader Authorization: required authentication credentials of either type HTTP Basic or OAuth Bearer Token.
    :reqheader Content-Type: MIME Type of post data. JSON, url-encoded form data, etc.

    :statuscode 204: success, email address removed.
    :statuscode 400: validation error.
    :statuscode 401: authentication error.
    :statuscode 403: permission error, authenticated user must be the user whose data is being requested, OAuth access tokens must have ``email_write`` scope.
    :statuscode 404: the specified username or email address does not exist.

    **Example request**:

    .. sourcecode:: http

        DELETE /api/v1.1/users/janedoe/emails/ HTTP/1.1
        Host: www.docker.io
        Accept: application/json
        Content-Type: application/json
        Authorization: Bearer zAy0BxC1wDv2EuF3tGs4HrI6qJp6KoL7nM

        {
            "email": "jane.doe+other@example.com"
        }

    **Example response**:

    .. sourcecode:: http

        HTTP/1.1 204 NO CONTENT
        Content-Length: 0
