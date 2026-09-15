# Engine API

[![PkgGoDev](https://pkg.go.dev/badge/github.com/moby/moby/api)](https://pkg.go.dev/github.com/moby/moby/api)
![GitHub License](https://img.shields.io/github/license/moby/moby)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/moby/moby/badge)](https://scorecard.dev/viewer/?uri=github.com/moby/moby)
[![OpenSSF Best Practices](https://www.bestpractices.dev/projects/10989/badge)](https://www.bestpractices.dev/projects/10989)


The Engine API is an HTTP API used by the command-line client to communicate with the daemon. It can also be used by third-party software to control the daemon.

It consists of various components in this repository:

- `api/openapi.yaml` An OpenAPI 3.2.0 definition of the API.
- `api/swagger.yaml` The generated Swagger 2.0 compatibility definition of the same API.
- `api/types/` Types shared by both the client and server, representing various objects, options, responses, etc. Most are written manually, but some are automatically generated from the API definition. See [#27919](https://github.com/moby/moby/issues/27919) for progress on this.
- `client/` The Go client used by the command-line client. It can also be used by third-party Go programs.
- `daemon/` The daemon, which serves the API.

## OpenAPI definition

The API is defined by the [OpenAPI 3.2.0](https://spec.openapis.org/oas/v3.2.0) definition in `api/openapi.yaml`. This definition can be used to:

1. Automatically generate documentation.
2. Automatically generate the Go server and client. (A work-in-progress.)
3. Provide a machine readable version of the API for introspecting what it can do, automatically generating clients for other languages, etc.

## Updating the API documentation

The API documentation is generated entirely from `api/openapi.yaml`. If you make updates to the API, edit this file to represent the change in the documentation.
Documentation for each API version can be found in the [docs directory](docs/README.md), which also provides a [CHANGELOG.md](docs/CHANGELOG.md). 

The file is split into two main sections:

- `components.schemas`, which defines re-usable objects used in requests and responses
- `paths`, which defines the API endpoints (and some inline objects which don't need to be reusable)

To make an edit, first look for the endpoint you want to edit under `paths`, then make the required edits. Endpoints may reference reusable objects with `$ref: "#/components/schemas/Name"`.

Request bodies are defined by `requestBody.content`, and response schemas and examples by `responses.<status>.content`, keyed by media type. Non-body parameters use `schema`. Array query parameters use `style: form` with `explode: true` for repeated keys and `explode: false` for comma-separated values; do not rely on the OpenAPI default, which differs from Swagger 2.0.

The relative server URL retains the version prefix without choosing a daemon hostname. `x-schemes` records the supported HTTP and HTTPS transports. The root `x-consumes` and `x-produces` retain the legacy Swagger defaults for compatibility generation; OpenAPI consumers use each operation's explicit `content` instead. Streaming media types and connection-upgrade descriptions are part of the contract; an empty media object means the response schema is unspecified, not that there is no stream. Explicit `consumes` declarations on operations without a request body are retained as `x-consumes`.

Schemas use the default OpenAPI 3.1 schema dialect (JSON Schema 2020-12), declared by `jsonSchemaDialect`. Keep the document within what the Docker documentation pipeline and the Go model projection accept: do not use the removed OpenAPI 3.0 `nullable` keyword, `type` unions, `anyOf`, `oneOf`, or the 3.2-only `itemSchema` and tag hierarchy fields until the published documentation renders them. Whether a field may be JSON `null` on the wire is recorded only by the `x-nullable` generator extension; it reflects the Go representation and is not yet a reviewed statement about the API contract.

Run `make -C api swagger-spec` from the repository root after editing `openapi.yaml`, and commit the generated `swagger.yaml` too. Do not edit `swagger.yaml` directly. `scripts/swagger_spec.py` converts the complete specification, including endpoints, parameters, bodies, headers, examples, and schemas. When the existing Swagger matches the projected content and schema property order, generation retains its text unchanged, including comments and formatting. Content or property-order changes produce fresh YAML; formatting may then change. Generation also works without an existing Swagger file.

The generated Swagger follows the OpenAPI documentation, including repeated query parameters, bodyless HEAD responses, and omission of the invalid `RemoteManagers` null default. Swagger 2.0 cannot express different media types for individual responses, so each operation's `produces` lists their union. OpenAPI remains authoritative for which response uses which media type. No endpoint-specific compatibility overrides preserve the previous documentation errors.

Run `make -C api validate-swagger` to check YAML style, validate OpenAPI 3.2 (including `default` values against their schemas), validate Swagger 2.0, and test the converter. This compares the checked-in Swagger with a fresh projection, including schema property order, and checks that regeneration leaves the file byte-for-byte unchanged. YAML response-key types and parameter placement do not affect the comparison. The pinned validators and YAML parser are installed by `api/Dockerfile`. Historical `api/docs/v*.yaml` specifications are not migrated.

## Generating Go models

Run `make swagger-gen` from the repository root, or `make swagger-gen` in this directory, as before. This first regenerates `api/swagger.yaml` from `api/openapi.yaml`, then feeds that full Swagger 2.0 file to go-swagger, pinned in `api/Dockerfile`. The converter preserves property order (including `--keep-spec-order`), custom formats, examples, and generator extensions.

JSON Schema 2020-12 evaluates keywords placed beside `$ref`, so a referenced property carries its `description`, `x-nullable`, and `x-omitempty` directly, exactly as go-swagger reads them. The projection keeps original `allOf` compositions and Go pointer/omitempty behavior. Schema constructs that go-swagger cannot represent cause the projection to fail rather than silently lose information.

Run `make -C api validate-swagger-gen` to regenerate into a temporary module and compare the results with the checked-in Go models. Do not change generated models just to accommodate a specification-format change.

## Viewing the API documentation

When you make edits to `openapi.yaml`, you may want to check the generated API documentation to ensure it renders correctly.

Run `make swagger-docs` and a preview will be running at `http://localhost:9000`. Some of the styling may be incorrect, but you'll be able to ensure that it is generating the correct documentation.

The production documentation is generated by vendoring the versioned `docs/v*.yaml` copies of `openapi.yaml` into [docker/docs](https://github.com/docker/docs).
