title
:   docker.io OAuth API

description
:   API Documentation for docker.io's OAuth flow.

keywords
:   API, Docker, oauth, REST, documentation

docker.io OAuth API
===================

1. Brief introduction
---------------------

Some docker.io API requests will require an access token to
authenticate. To get an access token for a user, that user must first
grant your application access to their docker.io account. In order for
them to grant your application access you must first register your
application.

Before continuing, we encourage you to familiarize yourself with [The
OAuth 2.0 Authorization Framework](http://tools.ietf.org/html/rfc6749).

*Also note that all OAuth interactions must take place over https
connections*

2. Register Your Application
----------------------------

You will need to register your application with docker.io before users
will be able to grant your application access to their account
information. We are currently only allowing applications selectively. To
request registration of your application send an email to

support-accounts at docker dot com

with the following information:

-   The name of your application
-   A description of your application and the service it will provide to
    docker.io users.
-   A callback URI that we will use for redirecting authorization
    requests to your application. These are used in the step of getting
    an Authorization Code. The domain name of the callback URI will be
    visible to the user when they are requested to authorize your
    application.

When your application is approved you will receive a response from the
docker.io team with your `client_id` and `client_secret` which your
application will use in the steps of getting an Authorization Code and
getting an Access Token.

3. Endpoints
------------

### 3.1 Get an Authorization Code

Once You have registered you are ready to start integrating docker.io
accounts into your application! The process is usually started by a user
following a link in your application to an OAuth Authorization endpoint.

### 3.2 Get an Access Token

Once the user has authorized your application, a request will be made to
your application's specified `redirect_uri` which includes a `code`
parameter that you must then use to get an Access Token.

### 3.3 Refresh a Token

Once the Access Token expires you can use your `refresh_token` to have
docker.io issue your application a new Access Token, if the user has not
revoked access from your application.

4. Use an Access Token with the API
-----------------------------------

Many of the docker.io API requests will require a Authorization request
header field. Simply ensure you add this header with "Bearer
\<`access_token`\>":
