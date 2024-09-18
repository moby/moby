Branches and tags
=================

> Note: details of the release process of the project are documented in [PATCH-RELEASES.md](PATCH-RELEASES.md).

# Branches

`master` serves as the development branch for future releases of the project.
All changes should be made to the `master` branch, and changes to release branches should only be made in the form of cherry-picked commits, if possible.
The sponsoring maintainers of a release branch serve as the primary point of contact, and are available to provide guidance on contributing changes to their respective branches.

Keep in mind that release branches only accept bug and security fixes; new features will generally not be considered for backport to release branches.

Currently (and previously) maintained release branches are documented in the table below:

| Branch Name                       | Sponsoring Maintainer(s)                                | Contribution Status   | Expected End of Maintenance  | Known Distributors                  |
|-----------------------------------|---------------------------------------------------------|-----------------------|------------------------------|-------------------------------------|
| master (development branch)       | The Moby Project [MAINTAINERS](../MAINTAINERS)          | N/A                   | -                            | N/A                                 |
| 27.x                              | The Moby Project [MAINTAINERS](../MAINTAINERS)          | Maintained            | After 28.x                   | [Docker, Inc.](https://docker.com)  |
| 26.1                              | @cpuguy83                                               | Maintained (security) | TBD                          | [Microsoft](https://microsoft.com)  |
| 25.0                              | @corhere                                                | Maintained            | TBD                          | [Amazon][al2023]                    |
| 23.0                              | @corhere                                                | Maintained            | [2025-05-19][mcr23-maint]    | [Mirantis](https://mirantis.com)    |

[al2023]: https://docs.aws.amazon.com/linux/
[mcr23-maint]: https://docs.mirantis.com/mcr/23.0/compat-matrix/maintenance-lifecycle.html

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
