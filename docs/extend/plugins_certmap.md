<!--[metadata]>
+++
title = "Certificate mapping plugins"
description = "How to map client TLS certificates to authenticated users"
keywords = ["Examples, Usage, certificates, docker, plugin, api"]
[menu.main]
parent = "mn_extend"
+++
<![end-metadata]-->

# Write a certificate mapping plugin

Docker certificate mapping plugins enable Docker deployments to authenticate
clients by reading the client identity from a TLS client certificate, or by
consulting an outside source.  See the [plugin documentation](plugins.md) for
more information.

# Command-line changes

A certificate mapping plugin is enabled by using the `-a` flag and the
`--authn-opt certmap` option with the `docker daemon` command.

    $ docker daemon -a --authn-opt certmap=sss

# Authentication protocol

If a plugin registers itself as a `ClientCertificateMapper` plugin when
activated, then it is expected to provide a
`ClientCertificateMapper.MapClientCertificateToUser` handler.

**Request**:
```
{
    "Certificate": ""
    "Options": []
}
```
#### The `Certificate` string contains a base64-encoded copy of the client
certificate.
#### The `Options` array contains authentication options which were passed to
the daemon at startup-time.

**Response**:
```
{
    "AuthedUser": {},
}
```
If authentication succeeded, the `AuthedUser` map should contain at least these items:
#### Name: a string naming the client.  Can be empty if a UID is set.
#### HaveUID: a boolean indicating whether or not a `UID` value is present.
#### UID: an optional number containing the UID of the client.
#### Groups: an optional array of names of groups of which the client is a member.
#### Scheme: a string naming the HTTP authentication scheme.  If not set, the
name of the plugin will be used, which is probably not what you want.
