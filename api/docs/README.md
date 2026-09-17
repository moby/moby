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
It uses [OpenAPI 3.2.0](https://spec.openapis.org/oas/v3.2.0). This format
migration does not change the Engine API contract or generated Go models.
A generated [Swagger 2.0 compatibility specification](../swagger.yaml) is
also available and is used by the Go model generator. Edit `openapi.yaml`
and regenerate it rather than editing `swagger.yaml` directly.
See the [module README](../README.md) for validation and model-generation
instructions.

For API version v1.24, documentation is only available in markdown
format. Historical `v*.yaml` files in this directory retain their original
[Swagger (OpenAPI) v2.0](https://swagger.io/specification/v2/) format; they
are not converted along with the latest specification. The Moby project itself
primarily uses these specification files to produce the API documentation;
while we attempt to make these files match the actual implementation,
the specification formats have limitations that prevent us from
expressing all options provided. There may be discrepancies (for which
we welcome contributions). If you find bugs, or discrepancies, please
open a ticket (or pull request).


