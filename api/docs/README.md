# API Documentation

This directory contains versioned documents for each version of the API
specification supported by this module. While this module provides support
for older API versions, support should be considered "best-effort", especially
for very old versions. Users are recommended to use the latest API versions,
and only rely on older API versions for compatibility with older clients.

Newer API versions tend to be backward-compatible with older versions,
with some exceptions where features were deprecated. For an overview
of changes for each version, refer to [CHANGELOG.md](CHANGELOG.md).

The latest version of the API specification can be found [at the root directory
of this module](../openapi.yaml) which may contain unreleased changes.

For API version v1.24, documentation is only available in markdown
format, for later versions [OpenAPI v3](https://www.openapis.org/)
specifications can be found in this directory. The Moby project itself
primarily uses these OpenAPI files to produce the API documentation;
while we attempt to make these files match the actual implementation,
the OpenAPI 3 specification has limitations that prevent us from
expressing all options provided. There may be discrepancies (for which
we welcome contributions). If you find bugs, or discrepancies, please
open a ticket (or pull request).


