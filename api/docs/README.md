# API Documentation

This directory contains versioned documents for each version of the API
specification supported by this module. While this module provides support
for older API versions, support should be considered "best-effort", especially
for very old versions. Users are recommended to use the latest API versions,
and only rely on older API versions for compatibility with older clients.

Newer API versions tend to be backward-compatible with older versions,
with some exceptions where features were deprecated. For an overview
of changes for each version, refer to [CHANGELOG.md](CHANGELOG.md).

The latest specification is [openapi.yaml](../openapi.yaml), in [OpenAPI 3.2.0](https://spec.openapis.org/oas/v3.2.0) format, and may contain unreleased changes.
The generated [Swagger 2.0 specification](../swagger.yaml) supports Go model generation.
See the [module README](../README.md) for editing and validation instructions.

For API version v1.24, documentation is only available in markdown format.
Historical `v*.yaml` files retain their [Swagger (OpenAPI) v2.0](https://swagger.io/specification/v2/) format.
The Moby project primarily uses these specifications to produce API documentation.
The formats cannot express all implementation details; please report discrepancies in an issue or pull request.


