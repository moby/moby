# APIs

Your programs and scripts can access Docker’s functionality via these
interfaces:

-   [Registry & Index Spec](registry_index_spec/)
    -   [1. The 3 roles](registry_index_spec/#the-3-roles)
        -   [1.1 Index](registry_index_spec/#index)
        -   [1.2 Registry](registry_index_spec/#registry)
        -   [1.3 Docker](registry_index_spec/#docker)

    -   [2. Workflow](registry_index_spec/#workflow)
        -   [2.1 Pull](registry_index_spec/#pull)
        -   [2.2 Push](registry_index_spec/#push)
        -   [2.3 Delete](registry_index_spec/#delete)

    -   [3. How to use the Registry in standalone
        mode](registry_index_spec/#how-to-use-the-registry-in-standalone-mode)
        -   [3.1 Without an
            Index](registry_index_spec/#without-an-index)
        -   [3.2 With an Index](registry_index_spec/#with-an-index)

    -   [4. The API](registry_index_spec/#the-api)
        -   [4.1 Images](registry_index_spec/#images)
        -   [4.2 Users](registry_index_spec/#users)
        -   [4.3 Tags (Registry)](registry_index_spec/#tags-registry)
        -   [4.4 Images (Index)](registry_index_spec/#images-index)
        -   [4.5 Repositories](registry_index_spec/#repositories)

    -   [5. Chaining
        Registries](registry_index_spec/#chaining-registries)
    -   [6. Authentication &
        Authorization](registry_index_spec/#authentication-authorization)
        -   [6.1 On the Index](registry_index_spec/#on-the-index)
        -   [6.2 On the Registry](registry_index_spec/#on-the-registry)

    -   [7 Document Version](registry_index_spec/#document-version)

-   [Docker Registry API](registry_api/)
    -   [1. Brief introduction](registry_api/#brief-introduction)
    -   [2. Endpoints](registry_api/#endpoints)
        -   [2.1 Images](registry_api/#images)
        -   [2.2 Tags](registry_api/#tags)
        -   [2.3 Repositories](registry_api/#repositories)
        -   [2.4 Status](registry_api/#status)

    -   [3 Authorization](registry_api/#authorization)

-   [Docker Index API](index_api/)
    -   [1. Brief introduction](index_api/#brief-introduction)
    -   [2. Endpoints](index_api/#endpoints)
        -   [2.1 Repository](index_api/#repository)
        -   [2.2 Users](index_api/#users)
        -   [2.3 Search](index_api/#search)

-   [Docker Remote API](docker_remote_api/)
    -   [1. Brief introduction](docker_remote_api/#brief-introduction)
    -   [2. Versions](docker_remote_api/#versions)
        -   [v1.11](docker_remote_api/#v1-11)
        -   [v1.10](docker_remote_api/#v1-10)
        -   [v1.9](docker_remote_api/#v1-9)
        -   [v1.8](docker_remote_api/#v1-8)
        -   [v1.7](docker_remote_api/#v1-7)
        -   [v1.6](docker_remote_api/#v1-6)
        -   [v1.5](docker_remote_api/#v1-5)
        -   [v1.4](docker_remote_api/#v1-4)
        -   [v1.3](docker_remote_api/#v1-3)
        -   [v1.2](docker_remote_api/#v1-2)
        -   [v1.1](docker_remote_api/#v1-1)
        -   [v1.0](docker_remote_api/#v1-0)

-   [Docker Remote API Client Libraries](remote_api_client_libraries/)
-   [docker.io OAuth API](docker_io_oauth_api/)
    -   [1. Brief introduction](docker_io_oauth_api/#brief-introduction)
    -   [2. Register Your
        Application](docker_io_oauth_api/#register-your-application)
    -   [3. Endpoints](docker_io_oauth_api/#endpoints)
        -   [3.1 Get an Authorization
            Code](docker_io_oauth_api/#get-an-authorization-code)
        -   [3.2 Get an Access
            Token](docker_io_oauth_api/#get-an-access-token)
        -   [3.3 Refresh a Token](docker_io_oauth_api/#refresh-a-token)

    -   [4. Use an Access Token with the
        API](docker_io_oauth_api/#use-an-access-token-with-the-api)

-   [docker.io Accounts API](docker_io_accounts_api/)
    -   [1. Endpoints](docker_io_accounts_api/#endpoints)
        -   [1.1 Get a single
            user](docker_io_accounts_api/#get-a-single-user)
        -   [1.2 Update a single
            user](docker_io_accounts_api/#update-a-single-user)
        -   [1.3 List email addresses for a
            user](docker_io_accounts_api/#list-email-addresses-for-a-user)
        -   [1.4 Add email address for a
            user](docker_io_accounts_api/#add-email-address-for-a-user)
        -   [1.5 Update an email address for a
            user](docker_io_accounts_api/#update-an-email-address-for-a-user)
        -   [1.6 Delete email address for a
            user](docker_io_accounts_api/#delete-email-address-for-a-user)