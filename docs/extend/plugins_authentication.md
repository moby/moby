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

Docker authentication plugins enable Docker daemons to authenticate clients
using data included in the client HTTP request.  See the [plugin
documentation](plugins.md) for information about implementing plugins that is
not specific to authentication plugins.

# Command-line changes

An authentication plugin is enabled by using the `--authn` flag and the
`--authn-opt plugins` option with the `dockerd` command.

    $ dockerd --authn --authn-opt plugins=htpasswd

# Authentication protocol

If a plugin registers itself as an `Authentication` plugin when activated, then
it is expected to provide `Authentication.SetOptions` and
`Authentication.Authenticate` handlers.

### /Authentication.SetOptions

**Request**:
```
{
    "Options": {}
}
```
#### The `Options` map contains the authentication options which were passed
to the daemon at startup-time.

**Response**:
```
{
}
```

#### Implementation

Authentication options which were provided to the daemon will be passed to the
plugin using this endpoint.  Plugins which implement the "Basic" authentication
scheme should take note of the "realm" option, if one is set.

### /Authentication.Authenticate

**Request**:
```
{
    "Method": "GET"/"POST",
    "URL": "",
    "Host": "",
    "Header": {},
    "Certificate": ""
}
```
#### The `Method` string is taken from the client request.
#### The `URL` string is taken from the client request.
#### The `Host` string is taken from the client request.
#### The `Header` map contains arrays of header values indexed by their names,
taken from the request as received by the daemon.
#### The `Certificate` string is the client's TLS certificate, if one was
supplied, in binary form, base64-encoded.

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
#### UID: an optional unsigned 32-bit number containing the UID of the client.
#### Groups: an optional array of names of groups of which the client is a member.
#### Scheme: a string naming the HTTP authentication scheme.  If not set, the
name of the plugin will be used, which is probably not what you want.

If authentication succeeded, headers returned by the succeeding plugin (and
only that plugin) will be included in the response which is sent to the client.

If authentication failed, headers returned by all plugins will be included in
the response which is sent to the client.

#### Implementation

The plugin should first check if the client request included an `Authorization`
header which matches a scheme that it implements.  If `Authorization` headers
are present, but for a different scheme, the plugin should simply return an
empty response.

If a suitable `Authorization` header is present, the plugin should attempt to
use it to authenticate the user.

If no `Authorization` header is present, the plugin can attempt to authenticate
the client without it.

If authentication succeeds, the plugin should populate the `AuthedUser` field
in its response.  If not, it may provide a `WWW-Authenticate` header in the
`Header` field to be sent to the client along with a 401 response.
