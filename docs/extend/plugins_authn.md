<!--[metadata]>
+++
title = "Authentication plugins"
description = "How to authenticate clients with external authentication plugins"
keywords = ["Examples, Usage, authn, docker, plugin, api"]
[menu.main]
parent = "mn_extend"
+++
<![end-metadata]-->

# Write an authentication plugin

Docker authentication plugins enable Docker deployments to authenticate clients
using HTTP authentication schemes which are not implemented by the Docker
daemon itself, and to implement password checks for Basic authentication in
different ways.  See the [plugin documentation](plugins.md) for more
information.

# Command-line changes

An authentication plugin is enabled by using the `-a` flag and the
`--authn-opt plugins` option with the `docker daemon` command.

    $ docker daemon -a --authn-opt plugins=sss

# Authentication protocol

If a plugin registers itself as an `Authentication` plugin when activated, then
it is expected to provide both `Authentication.GetChallenge` and
`Authentication.CheckResponse` handlers.  Both handlers accept the same request
type and return the same response type.

**Request**:
```
{
    "Method": "GET"/"POST",
    "URL": {},
    "Host": "",
    "Header": {},
    "Options": []
}
```
#### The `Method` string is taken from the client request.
#### The `URL` map contains `Scheme`, `Path`, `RawQuery`, and `Fragment`
strings representing portions of the URL reconstructed from the request as
received by the daemon.
#### The `Header` map contains arrays of header values indexed by their names,
taken from the request as received by the daemon.
#### The `Options` array contains authentication options which were passed to
the daemon at startup-time.
Plugins which implement the "Basic" authentication scheme should take note of
the "realm" setting, if one is set, in their `GetChallenge` handlers.

**Response**:
```
{
    "AuthedUser": {},
    "Header": {}
}
```
If authentication succeeded, the `AuthedUser` map should contain at least these items:
#### Name: a string naming the client.  Can be empty if a UID is set.
#### HaveUID: a boolean indicating whether or not a `UID` value is present.
#### UID: an optional number containing the UID of the client.
#### Groups: an optional array of names of groups of which the client is a member.
#### Scheme: a string naming the HTTP authentication scheme.  If not set, the
name of the plugin will be used, which is probably not what you want.

### /Authentication.GetChallenge

Produces challenge headers to be returned to the client along with a
StatusUnauthorized error and returns them as elements in the `Header` map.  The
`AuthedUser` field is ignored.

### /Authentication.CheckResponse

Checks client authentication information passed in through the `Header` map,
either returning a populated `AuthedUser` field or a `Headers` map containing
headers to be returned to the client along with a StatusUnauthorized error.
