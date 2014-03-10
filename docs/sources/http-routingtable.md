
# HTTP Routing Table

[**/api**](#cap-/api) | [**/auth**](#cap-/auth) |
[**/build**](#cap-/build) | [**/commit**](#cap-/commit) |
[**/containers**](#cap-/containers) | [**/events**](#cap-/events) |
[**/events:**](#cap-/events:) | [**/images**](#cap-/images) |
[**/info**](#cap-/info) | [**/v1**](#cap-/v1) |
[**/version**](#cap-/version)

  -- -------------------------------------------------------------------------------------------------------------------------------------------------------------------- ----
                                                                                                                                                                          
     **/api**                                                                                                                                                             
     [`GET /api/v1.1/o/authorize/`{.xref}](../reference/api/docker_io_oauth_api/#get--api-v1.1-o-authorize-)                                                              **
     [`POST /api/v1.1/o/token/`{.xref}](../reference/api/docker_io_oauth_api/#post--api-v1.1-o-token-)                                                                    **
     [`GET /api/v1.1/users/:username/`{.xref}](../reference/api/docker_io_accounts_api/#get--api-v1.1-users--username-)                                                   **
     [`PATCH /api/v1.1/users/:username/`{.xref}](../reference/api/docker_io_accounts_api/#patch--api-v1.1-users--username-)                                               **
     [`GET /api/v1.1/users/:username/emails/`{.xref}](../reference/api/docker_io_accounts_api/#get--api-v1.1-users--username-emails-)                                     **
     [`PATCH /api/v1.1/users/:username/emails/`{.xref}](../reference/api/docker_io_accounts_api/#patch--api-v1.1-users--username-emails-)                                 **
     [`POST /api/v1.1/users/:username/emails/`{.xref}](../reference/api/docker_io_accounts_api/#post--api-v1.1-users--username-emails-)                                   **
     [`DELETE /api/v1.1/users/:username/emails/`{.xref}](../reference/api/docker_io_accounts_api/#delete--api-v1.1-users--username-emails-)                               **
                                                                                                                                                                          
     **/auth**                                                                                                                                                            
     [`GET /auth`{.xref}](../reference/api/docker_remote_api_v1.1/#get--auth)                                                                                             **
     [`POST /auth`{.xref}](../reference/api/docker_remote_api_v1.9/#post--auth)                                                                                           **
                                                                                                                                                                          
     **/build**                                                                                                                                                           
     [`POST /build`{.xref}](../reference/api/docker_remote_api_v1.9/#post--build)                                                                                         **
                                                                                                                                                                          
     **/commit**                                                                                                                                                          
     [`POST /commit`{.xref}](../reference/api/docker_remote_api_v1.9/#post--commit)                                                                                       **
                                                                                                                                                                          
     **/containers**                                                                                                                                                      
     [`DELETE /containers/(id)`{.xref}](../reference/api/docker_remote_api_v1.9/#delete--containers-(id))                                                                 **
     [`POST /containers/(id)/attach`{.xref}](../reference/api/docker_remote_api_v1.9/#post--containers-(id)-attach)                                                       **
     [`GET /containers/(id)/changes`{.xref}](../reference/api/docker_remote_api_v1.9/#get--containers-(id)-changes)                                                       **
     [`POST /containers/(id)/copy`{.xref}](../reference/api/docker_remote_api_v1.9/#post--containers-(id)-copy)                                                           **
     [`GET /containers/(id)/export`{.xref}](../reference/api/docker_remote_api_v1.9/#get--containers-(id)-export)                                                         **
     [`GET /containers/(id)/json`{.xref}](../reference/api/docker_remote_api_v1.9/#get--containers-(id)-json)                                                             **
     [`POST /containers/(id)/kill`{.xref}](../reference/api/docker_remote_api_v1.9/#post--containers-(id)-kill)                                                           **
     [`POST /containers/(id)/restart`{.xref}](../reference/api/docker_remote_api_v1.9/#post--containers-(id)-restart)                                                     **
     [`POST /containers/(id)/start`{.xref}](../reference/api/docker_remote_api_v1.9/#post--containers-(id)-start)                                                         **
     [`POST /containers/(id)/stop`{.xref}](../reference/api/docker_remote_api_v1.9/#post--containers-(id)-stop)                                                           **
     [`GET /containers/(id)/top`{.xref}](../reference/api/docker_remote_api_v1.9/#get--containers-(id)-top)                                                               **
     [`POST /containers/(id)/wait`{.xref}](../reference/api/docker_remote_api_v1.9/#post--containers-(id)-wait)                                                           **
     [`POST /containers/create`{.xref}](../reference/api/docker_remote_api_v1.9/#post--containers-create)                                                                 **
     [`GET /containers/json`{.xref}](../reference/api/docker_remote_api_v1.9/#get--containers-json)                                                                       **
                                                                                                                                                                          
     **/events**                                                                                                                                                          
     [`GET /events`{.xref}](../reference/api/docker_remote_api_v1.9/#get--events)                                                                                         **
                                                                                                                                                                          
     **/events:**                                                                                                                                                         
     [`GET /events:`{.xref}](../reference/api/docker_remote_api/#get--events-)                                                                                            **
                                                                                                                                                                          
     **/images**                                                                                                                                                          
     [`GET /images/(format)`{.xref}](../reference/api/docker_remote_api_v1.6/#get--images-(format))                                                                       **
     [`DELETE /images/(name)`{.xref}](../reference/api/docker_remote_api_v1.9/#delete--images-(name))                                                                     **
     [`GET /images/(name)/get`{.xref}](../reference/api/docker_remote_api_v1.9/#get--images-(name)-get)                                                                   **
     [`GET /images/(name)/history`{.xref}](../reference/api/docker_remote_api_v1.9/#get--images-(name)-history)                                                           **
     [`POST /images/(name)/insert`{.xref}](../reference/api/docker_remote_api_v1.9/#post--images-(name)-insert)                                                           **
     [`GET /images/(name)/json`{.xref}](../reference/api/docker_remote_api_v1.9/#get--images-(name)-json)                                                                 **
     [`POST /images/(name)/push`{.xref}](../reference/api/docker_remote_api_v1.9/#post--images-(name)-push)                                                               **
     [`POST /images/(name)/tag`{.xref}](../reference/api/docker_remote_api_v1.9/#post--images-(name)-tag)                                                                 **
     [`POST /images/<name>/delete`{.xref}](../reference/api/docker_remote_api/#post--images--name--delete)                                                                **
     [`POST /images/create`{.xref}](../reference/api/docker_remote_api_v1.9/#post--images-create)                                                                         **
     [`GET /images/json`{.xref}](../reference/api/docker_remote_api_v1.9/#get--images-json)                                                                               **
     [`POST /images/load`{.xref}](../reference/api/docker_remote_api_v1.9/#post--images-load)                                                                             **
     [`GET /images/search`{.xref}](../reference/api/docker_remote_api_v1.9/#get--images-search)                                                                           **
     [`GET /images/viz`{.xref}](../reference/api/docker_remote_api/#get--images-viz)                                                                                      **
                                                                                                                                                                          
     **/info**                                                                                                                                                            
     [`GET /info`{.xref}](../reference/api/docker_remote_api_v1.9/#get--info)                                                                                             **
                                                                                                                                                                          
     **/v1**                                                                                                                                                              
     [`GET /v1/_ping`{.xref}](../reference/api/registry_api/#get--v1-_ping)                                                                                               **
     [`GET /v1/images/(image_id)/ancestry`{.xref}](../reference/api/registry_api/#get--v1-images-(image_id)-ancestry)                                                     **
     [`GET /v1/images/(image_id)/json`{.xref}](../reference/api/registry_api/#get--v1-images-(image_id)-json)                                                             **
     [`PUT /v1/images/(image_id)/json`{.xref}](../reference/api/registry_api/#put--v1-images-(image_id)-json)                                                             **
     [`GET /v1/images/(image_id)/layer`{.xref}](../reference/api/registry_api/#get--v1-images-(image_id)-layer)                                                           **
     [`PUT /v1/images/(image_id)/layer`{.xref}](../reference/api/registry_api/#put--v1-images-(image_id)-layer)                                                           **
     [`PUT /v1/repositories/(namespace)/(repo_name)/`{.xref}](../reference/api/index_api/#put--v1-repositories-(namespace)-(repo_name)-)                                  **
     [`DELETE /v1/repositories/(namespace)/(repo_name)/`{.xref}](../reference/api/index_api/#delete--v1-repositories-(namespace)-(repo_name)-)                            **
     [`PUT /v1/repositories/(namespace)/(repo_name)/auth`{.xref}](../reference/api/index_api/#put--v1-repositories-(namespace)-(repo_name)-auth)                          **
     [`GET /v1/repositories/(namespace)/(repo_name)/images`{.xref}](../reference/api/index_api/#get--v1-repositories-(namespace)-(repo_name)-images)                      **
     [`PUT /v1/repositories/(namespace)/(repo_name)/images`{.xref}](../reference/api/index_api/#put--v1-repositories-(namespace)-(repo_name)-images)                      **
     [`DELETE /v1/repositories/(namespace)/(repository)/`{.xref}](../reference/api/registry_api/#delete--v1-repositories-(namespace)-(repository)-)                       **
     [`GET /v1/repositories/(namespace)/(repository)/tags`{.xref}](../reference/api/registry_api/#get--v1-repositories-(namespace)-(repository)-tags)                     **
     [`GET /v1/repositories/(namespace)/(repository)/tags/(tag)`{.xref}](../reference/api/registry_api/#get--v1-repositories-(namespace)-(repository)-tags-(tag))         **
     [`PUT /v1/repositories/(namespace)/(repository)/tags/(tag)`{.xref}](../reference/api/registry_api/#put--v1-repositories-(namespace)-(repository)-tags-(tag))         **
     [`DELETE /v1/repositories/(namespace)/(repository)/tags/(tag)`{.xref}](../reference/api/registry_api/#delete--v1-repositories-(namespace)-(repository)-tags-(tag))   **
     [`PUT /v1/repositories/(repo_name)/`{.xref}](../reference/api/index_api/#put--v1-repositories-(repo_name)-)                                                          **
     [`DELETE /v1/repositories/(repo_name)/`{.xref}](../reference/api/index_api/#delete--v1-repositories-(repo_name)-)                                                    **
     [`PUT /v1/repositories/(repo_name)/auth`{.xref}](../reference/api/index_api/#put--v1-repositories-(repo_name)-auth)                                                  **
     [`GET /v1/repositories/(repo_name)/images`{.xref}](../reference/api/index_api/#get--v1-repositories-(repo_name)-images)                                              **
     [`PUT /v1/repositories/(repo_name)/images`{.xref}](../reference/api/index_api/#put--v1-repositories-(repo_name)-images)                                              **
     [`GET /v1/search`{.xref}](../reference/api/index_api/#get--v1-search)                                                                                                **
     [`GET /v1/users`{.xref}](../reference/api/index_api/#get--v1-users)                                                                                                  **
     [`POST /v1/users`{.xref}](../reference/api/index_api/#post--v1-users)                                                                                                **
     [`PUT /v1/users/(username)/`{.xref}](../reference/api/index_api/#put--v1-users-(username)-)                                                                          **
                                                                                                                                                                          
     **/version**                                                                                                                                                         
     [`GET /version`{.xref}](../reference/api/docker_remote_api_v1.9/#get--version)                                                                                       **
  -- -------------------------------------------------------------------------------------------------------------------------------------------------------------------- ----


