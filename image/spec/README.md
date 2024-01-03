# Docker Image Specification v1.

This directory contains documents about Docker Image Specification v1.X.

The Docker Image Specification is the image specification as used by the
Docker Engine, and was used as foundation of the OCI image specification.

The Docker Image Specification provides a superset of the OCI Image specification;
it is OCI-compatible, but some extensions that are specific to the Docker
Engine implementation.

Refer to [spec.md](spec.md) for the current version of the Docker Image
Specification, and the [OCI Image specification](https://github.com/opencontainers/image-spec/)
for an in-depth specification of the OCI Image specs.

The v1 file layout and manifests are no longer used in Moby and Docker, except in `docker save` and `docker load`.

However, v1 Image JSON (`application/vnd.docker.container.image.v1+json`) has been still widely
used and officially adopted in [V2 manifest](https://github.com/distribution/distribution/blob/main/docs/content/spec/manifest-v2-2.md)
and in [OCI Image Format Specification](https://github.com/opencontainers/image-spec).

## v1.X rough Changelog

All 1.X versions are compatible with older ones.

### [v1.3](spec.md)

* Implemented in Docker v25.0

Changes:

* `StartInterval` was added to the `Healthcheck` struct in the Image JSON

### [v1.2](https://github.com/moby/moby/blob/daa4618da826fb1de4fc2478d88196edbba49b2f/image/spec/v1.2.md)

* Implemented in Docker v1.12 (July, 2016)
* The official spec document was written in August 2016 ([#25750](https://github.com/moby/moby/pull/25750))

Changes:

* `Healthcheck` struct was added to Image JSON

### [v1.1](https://github.com/moby/moby/blob/daa4618da826fb1de4fc2478d88196edbba49b2f/image/spec/v1.1.md)

* Implemented in Docker v1.10 (February, 2016)
* The official spec document was written in April 2016 ([#22264](https://github.com/moby/moby/pull/22264))

Changes:

* IDs were made into SHA256 digest values rather than random values
* Layer directory names were made into deterministic values rather than random ID values
* `manifest.json` was added 

### [v1](https://github.com/moby/moby/blob/daa4618da826fb1de4fc2478d88196edbba49b2f/image/spec/v1.md)

* The initial revision
* The official spec document was written in late 2014 ([#9560](https://github.com/moby/moby/pull/9560)), but actual implementations had existed even earlier


## Related specifications

* [Open Containers Initiative (OCI) Image Format Specification v1.0.0](https://github.com/opencontainers/image-spec/tree/v1.0.0)
* [Docker Image Manifest Version 2, Schema 2](https://github.com/distribution/distribution/blob/main/docs/content/spec/manifest-v2-2.md)
* [Docker Image Manifest Version 2, Schema 1](https://github.com/distribution/distribution/blob/main/docs/content/spec/deprecated-schema-v1.md) (*DEPRECATED*)
* [Docker Registry HTTP API V2](https://docs.docker.com/registry/spec/api/)
