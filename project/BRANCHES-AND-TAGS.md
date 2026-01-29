Branches and tags
=================

> Note: details of the release process of the project are documented in [PATCH-RELEASES.md](PATCH-RELEASES.md).

# Overview

The Moby Project repository contains multiple components with their own versioning:

- **Moby** - the core library, versioned as `v2.x` (currently in beta)
- **Docker Engine** - built from this repository, versioned independently
- **API** (`api/`) - a separate Go module containing the materialization of the API specification, follows API versioning
- **Client** (`client/`) - a separate Go module of the client library

# Branches

`master` serves as the development branch for future releases of the project.
All changes should be made to the `master` branch.
When possible, Moby/Docker Engine releases are cut directly from `master`.
Release branches are created when ongoing development on `master` would conflict with patch releases.
The sponsoring maintainers of a release branch serve as the primary point of contact, and are available to provide guidance on contributing changes to their respective branches.

A single release branch can be used for the whole release cycle of a major version or a specific minor version line.
For example, the `docker-29.x` branch is used for the release cycle of the `29` major version and all minor versions within it.
A historical example of the latter approach are the `26.0` and `26.1` branches, which were used for separate minor versions within the `26` major version.

It is up to the sponsoring maintainers to decide whether to use a single release branch for the whole release cycle of a major version or a specific minor version line.

## Docker Engine Branches

Docker Engine release branches use the format `docker-MAJOR.x` (e.g., `docker-29.x`).

Currently (and previously) maintained release branches are documented in the table below:

| Branch Name                 | Sponsoring Maintainer(s)                       | Contribution Status   | Expected End of Maintenance | Known Distributors                        |
|-----------------------------|------------------------------------------------|-----------------------|-----------------------------|-------------------------------------------|
| docker-29.x                 | The Moby Project [MAINTAINERS](../MAINTAINERS) | Maintained            | After docker-30.x           | [Docker, Inc.][docker], [Microsoft][msft] |
| docker-28.x                 | @cpuguy83                                      | Maintained            | TBD                         | [Microsoft][msft]                         |
| 27.x                        |                                                | Unmaintained          |                             |                                           |
| 26.1                        |                                                | Unmaintained          |                             |                                           |
| 26.0                        |                                                | Unmaintained          |                             |                                           |
| 25.0                        | @corhere                                       | Maintained            | [2026-12-04][mcr25-maint]   | [Amazon][al2023], [Mirantis][mcr]         |
| 24.0                        |                                                | Unmaintained          |                             |                                           |
| 23.0                        |                                                | Unmaintained          | [2025-05-19][mcr23-maint]   |                                           |
| Older than 23.0             |                                                | Unmaintained          |                             |                                           |

[al2023]: https://docs.aws.amazon.com/linux/
[docker]: https://docker.com
[mcr23-maint]: https://docs.mirantis.com/mcr/23.0/compat-matrix/maintenance-lifecycle.html
[mcr25-maint]: https://docs.mirantis.com/mcr/25.0/compat-matrix/maintenance-lifecycle.html#mirantis-container-runtime-mcr
[mcr]: https://www.mirantis.com/software/mirantis-container-runtime/
[msft]: https://microsoft.com

> Note: The Moby Project provides source code releases. Binary distributions are available from multiple contributing parties, and known distributions can be discovered in [PACKAGERS.md](PACKAGERS.md).

## Contribution Status

The contribution status of a branch is meant to set contributor expectations for acceptance of changes into a branch, as well as document what level of contribution or maintenance the sponsoring maintainers expect to perform. This status is informational and not binding.

- **Maintained** - actively developed by project maintainers; accepting contributions and backports; in-scope for security advisories 
- **Maintained (security)** - no longer actively developed; may accept contributions and backports for critical security issues; in-scope for security advisories
- **Unmaintained** - no longer actively developed; not accepting contributions; out-of-scope for security advisories

# Tags

All releases of The Moby Project should have a corresponding tag in the repository.
The project generally attempts to adhere to [Semantic Versioning](https://semver.org) whenever possible.

The general format of a tag is `vX.Y.Z[-suffix[N]]`:

- All of `X`, `Y`, `Z` must be specified (example: `v1.0.0`)
- First release candidate for version `1.8.0` should be tagged `v1.8.0-rc1`
- Second alpha release of a product should be tagged `v1.0.0-alpha1`

## Tag Prefixes

Different components use different tag prefixes:

| Component     | Tag Format              | Example           |
|---------------|-------------------------|-------------------|
| Moby          | `vX.Y.Z`                | `v2.0.0-beta.5`   |
| Docker Engine | `docker-vX.Y.Z`         | `docker-v29.1.2`  |
| API           | `api/vX.Y.Z`            | `api/v1.52.0`     |
| Client        | `client/vX.Y.Z`         | `client/v1.52.0`  |

For the current release state, see [GitHub Releases](https://github.com/moby/moby/releases).
