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

Request bodies use `requestBody.content`, and responses use `responses.<status>.content`, keyed by media type.
Non-body parameters use `schema`.
For array query parameters, specify `style: form` and `explode: true` for repeated keys or `explode: false` for comma-separated values.
An empty media object leaves the response schema unspecified; it does not mean there is no body.

Use only schema features supported by the Swagger converter; unions, `anyOf`, `oneOf`, `nullable`, and `itemSchema` are not supported.
Keep `x-nullable` and other Go generator extensions; they control the generated Go representation, not wire-level validation.

Run `make -C api swagger-spec` from the repository root after editing `openapi.yaml`, and commit the generated `swagger.yaml` too.
Do not edit `swagger.yaml` directly.
Regeneration preserves an unchanged file and multiline JSON examples; other formatting may change when the content changes.

Swagger's operation-wide `produces` lists the union of response media types; OpenAPI remains authoritative for individual responses.
Legacy `consumes` declarations on bodyless operations use `x-consumes`.

Run `make -C api validate-swagger` to check YAML style, validate both specifications, and test the converter against the checked-in Swagger.

## Generating Go models

Run `make swagger-gen` from the repository root to regenerate `api/swagger.yaml` and generate Go models with go-swagger.
Run `make -C api validate-swagger-gen` to compare regenerated models with the checked-in files.

## Viewing the API documentation

When you make edits to `openapi.yaml`, you may want to check the generated API documentation to ensure it renders correctly.

Run `make swagger-docs` and a preview will be running at `http://localhost:9000`. Some of the styling may be incorrect, but you'll be able to ensure that it is generating the correct documentation.

The production documentation vendors the versioned `docs/v*.yaml` specifications into [docker/docs](https://github.com/docker/docs).
